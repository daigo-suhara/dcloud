import ArrowBackIcon from "@mui/icons-material/ArrowBack";
import DeleteOutlinedIcon from "@mui/icons-material/DeleteOutlined";
import { alpha } from "@mui/material/styles";
import { Alert, Box, Button, CircularProgress, Paper, Typography } from "@mui/material";
import { useEffect, useMemo, useRef, useState } from "react";
import { Terminal } from "xterm";
import "xterm/css/xterm.css";
import type { ComputeMachine } from "../types";
import { formatComputeStatus, formatComputeTimestamp } from "../utils";
import { monoFontFamily } from "../theme";
import { PageHeader, StatusBadge } from "./primitives";
import type { StatusVariant } from "./primitives";

type ComputeDetailSectionProps = {
  machine: ComputeMachine | null;
  machineName: string;
  loading: boolean;
  projectId: string;
  deletingMachineName: string;
  onBack: () => void;
  onDeleteMachine: (name: string) => void;
};

function detailStatus(machine: ComputeMachine | null, isDeleting: boolean, loading: boolean): { v: StatusVariant; label: string; spin: boolean } {
  if (isDeleting) return { v: "error", label: "削除中", spin: true };
  if (!machine) return { v: "unknown", label: loading ? "読み込み中" : "未検出", spin: loading };
  if (machine.ready) return { v: "ready", label: formatComputeStatus(machine), spin: false };
  return { v: "progress", label: formatComputeStatus(machine), spin: true };
}

export function ComputeDetailSection({
  machine, machineName, loading, projectId, deletingMachineName, onBack, onDeleteMachine
}: ComputeDetailSectionProps) {
  const terminalContainerRef = useRef<HTMLDivElement | null>(null);
  const terminalRef = useRef<Terminal | null>(null);
  const socketRef = useRef<WebSocket | null>(null);
  const retryTimerRef = useRef<number | null>(null);
  const disposedRef = useRef(false);
  const [terminalStatus, setTerminalStatus] = useState("起動待ち");

  const isReady = machine?.ready ?? false;
  const isDeleting = deletingMachineName === machineName;
  const s = detailStatus(machine, isDeleting, loading);

  const terminalHint = useMemo(() => {
    if (!machine) return loading ? "仮想マシン情報を読み込み中です。" : "仮想マシンが見つかりません。";
    if (!machine.ready) return "仮想マシンの起動中です。コンソールは接続を試行します。";
    return "";
  }, [loading, machine]);

  useEffect(() => {
    const container = terminalContainerRef.current;
    if (!container) return;
    disposedRef.current = false;

    if (!isReady) {
      socketRef.current?.close();
      socketRef.current = null;
      if (terminalRef.current) { terminalRef.current.dispose(); terminalRef.current = null; }
      setTerminalStatus("起動待ち");
      return () => { disposedRef.current = true; };
    }

    const terminal = new Terminal({
      cursorBlink: true,
      fontFamily: monoFontFamily,
      fontSize: 13, scrollback: 1000, convertEol: true,
      theme: {
        background: "#202124", foreground: "#e8eaed",
        cursor: "#e8eaed", selectionBackground: alpha("#1a73e8", 0.35)
      }
    });
    terminalRef.current = terminal;
    terminal.open(container);
    terminal.writeln("DCloud serial console");
    terminal.writeln("");

    const resize = () => {
      if (disposedRef.current) return;
      const rect = container.getBoundingClientRect();
      const cols = Math.max(40, Math.floor(rect.width / 8.5));
      const rows = Math.max(12, Math.floor(rect.height / 17));
      terminal.resize(cols, rows);
    };
    resize();
    const observer = new ResizeObserver(() => resize());
    observer.observe(container);
    const decoder = new TextDecoder();
    const encoder = new TextEncoder();
    // Send keystrokes as BINARY frames; virt-api routes stdin from
    // BINARY frames to the VM and silently drops TEXT frames.
    const dataDisposable = terminal.onData((data) => {
      const socket = socketRef.current;
      if (socket?.readyState !== WebSocket.OPEN) return;
      socket.send(encoder.encode(data));
    });

    const clearRetryTimer = () => {
      if (retryTimerRef.current !== null) {
        window.clearTimeout(retryTimerRef.current);
        retryTimerRef.current = null;
      }
    };

    const connect = () => {
      clearRetryTimer();
      if (disposedRef.current || !machine) return;
      if (socketRef.current && socketRef.current.readyState === WebSocket.OPEN) return;

      setTerminalStatus("接続中");
      const socket = new WebSocket(`/api/v1/compute/${encodeURIComponent(machineName)}/console?projectId=${encodeURIComponent(projectId)}`);
      socketRef.current = socket;
      socket.binaryType = "arraybuffer";

      socket.onopen = () => {
        if (disposedRef.current) return;
        setTerminalStatus("接続完了");
        terminal.writeln("[connected]");
      };
      socket.onmessage = (event) => {
        if (disposedRef.current) return;
        if (typeof event.data === "string") { terminal.write(event.data); return; }
        const payload = event.data instanceof ArrayBuffer ? new Uint8Array(event.data) : new Uint8Array();
        if (payload.length === 0) return;
        terminal.write(decoder.decode(payload, { stream: true }));
      };
      socket.onerror = () => {
        if (disposedRef.current) return;
        setTerminalStatus("接続エラー");
        terminal.writeln(""); terminal.writeln("[console error]");
      };
      socket.onclose = () => {
        if (disposedRef.current) return;
        socketRef.current = null;
        terminal.write(decoder.decode());
        setTerminalStatus("再接続待ち");
        terminal.writeln(""); terminal.writeln("[disconnected: retrying]");
        retryTimerRef.current = window.setTimeout(() => { if (!disposedRef.current) connect(); }, 2000);
      };
    };
    connect();

    return () => {
      disposedRef.current = true;
      clearRetryTimer();
      observer.disconnect();
      dataDisposable.dispose();
      socketRef.current?.close();
      socketRef.current = null;
      terminal.dispose();
      terminalRef.current = null;
    };
  }, [isReady, machine?.name, loading, machineName, projectId]);

  return (
    <Box>
      <PageHeader
        title={machine?.name ?? machineName}
        actions={
          <Button startIcon={<ArrowBackIcon />} onClick={onBack}>一覧に戻る</Button>
        }
      />

      <Box sx={{ display: "grid", gap: 1, gridTemplateColumns: { xs: "1fr", md: "repeat(2, minmax(0, 1fr))" }, mb: 3 }}>
        <Paper variant="outlined" sx={{ p: 1.5 }}>
          <Typography variant="caption" color="text.secondary">ステータス</Typography>
          <Box sx={{ mt: 0.5 }}>
            <StatusBadge variant={s.v} label={s.label} showSpinner={s.spin} />
          </Box>
        </Paper>
        <Paper variant="outlined" sx={{ p: 1.5 }}>
          <Typography variant="caption" color="text.secondary">イメージ</Typography>
          <Typography variant="body2" sx={{ mt: 0.5, wordBreak: "break-all", fontFamily: "monospace" }}>
            {machine?.image ?? "-"}
          </Typography>
        </Paper>
        <Paper variant="outlined" sx={{ p: 1.5 }}>
          <Typography variant="caption" color="text.secondary">サイズ</Typography>
          <Typography variant="body2" sx={{ mt: 0.5 }}>
            CPU {machine?.cpu ?? "-"} / MEM {machine?.memory ?? "-"}
          </Typography>
        </Paper>
        <Paper variant="outlined" sx={{ p: 1.5 }}>
          <Typography variant="caption" color="text.secondary">更新日時</Typography>
          <Typography variant="body2" sx={{ mt: 0.5 }}>
            {formatComputeTimestamp(machine?.updatedAt || machine?.createdAt)}
          </Typography>
        </Paper>
      </Box>

      <Box sx={{ mb: 2 }}>
        <Typography variant="h6" sx={{ mb: 1 }}>コンソール <Typography component="span" variant="caption" color="text.secondary" sx={{ ml: 1 }}>{terminalStatus}</Typography></Typography>
        {terminalHint ? <Alert severity="info" sx={{ mb: 1 }}>{terminalHint}</Alert> : null}
        <Paper variant="outlined" sx={{ overflow: "hidden", bgcolor: "#202124", borderColor: "#dadce0" }}>
          {isReady ? (
            <Box ref={terminalContainerRef} sx={{ height: { xs: 360, md: 520 }, width: "100%" }} />
          ) : (
            <Box sx={{
              height: { xs: 360, md: 520 }, width: "100%",
              display: "grid", placeItems: "center",
              color: "#e8eaed",
              fontFamily: monoFontFamily
            }}>
              <Typography variant="body2" sx={{ color: "rgba(232, 234, 237, 0.78)" }}>
                [waiting for vm to start]
              </Typography>
            </Box>
          )}
        </Paper>
      </Box>

      <Box sx={{ display: "flex", justifyContent: "flex-end" }}>
        <Button
          variant="outlined" color="error"
          startIcon={isDeleting ? <CircularProgress size={14} sx={{ color: "inherit" }} /> : <DeleteOutlinedIcon />}
          onClick={() => onDeleteMachine(machineName)}
          disabled={isDeleting}
        >
          {isDeleting ? "削除中..." : "仮想マシンを削除"}
        </Button>
      </Box>
    </Box>
  );
}
