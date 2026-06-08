import ArrowBackIcon from "@mui/icons-material/ArrowBack";
import CheckCircleIcon from "@mui/icons-material/CheckCircle";
import { alpha } from "@mui/material/styles";
import { Alert, Box, Button, Card, CardContent, CircularProgress, Paper, Typography } from "@mui/material";
import { useEffect, useMemo, useRef, useState } from "react";
import { Terminal } from "xterm";
import "xterm/css/xterm.css";
import type { ComputeMachine } from "../types";
import { formatComputeStatus, formatComputeTimestamp } from "../utils";

type ComputeDetailSectionProps = {
  machine: ComputeMachine | null;
  machineName: string;
  loading: boolean;
  projectId: string;
  onBack: () => void;
};

export function ComputeDetailSection({ machine, machineName, loading, projectId, onBack }: ComputeDetailSectionProps) {
  const terminalContainerRef = useRef<HTMLDivElement | null>(null);
  const terminalRef = useRef<Terminal | null>(null);
  const socketRef = useRef<WebSocket | null>(null);
  const retryTimerRef = useRef<number | null>(null);
  const disposedRef = useRef(false);
  const [terminalStatus, setTerminalStatus] = useState("起動待ち");

  const isReady = machine?.ready ?? false;
  const status = machine ? formatComputeStatus(machine) : loading ? "読み込み中" : "未検出";

  const terminalHint = useMemo(() => {
    if (!machine) {
      return loading ? "仮想マシン情報を読み込み中です。" : "仮想マシンが見つかりません。";
    }
    if (!machine.ready) {
      return "仮想マシンの起動中です。コンソールは接続を試行します。";
    }
    return "";
  }, [loading, machine]);

  useEffect(() => {
    const container = terminalContainerRef.current;
    if (!container) {
      return;
    }

    disposedRef.current = false;

    if (!isReady) {
      socketRef.current?.close();
      socketRef.current = null;
      if (terminalRef.current) {
        terminalRef.current.dispose();
        terminalRef.current = null;
      }
      setTerminalStatus("起動待ち");
      return () => {
        disposedRef.current = true;
      };
    }

    const terminal = new Terminal({
      cursorBlink: true,
      fontFamily: '"SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace',
      fontSize: 13,
      scrollback: 1000,
      convertEol: true,
      theme: {
        background: "#0b1020",
        foreground: "#dbe4ff",
        cursor: "#dbe4ff",
        selectionBackground: alpha("#7c93f6", 0.35)
      }
    });
    terminalRef.current = terminal;
    terminal.open(container);
    terminal.writeln("DCloud serial console");
    terminal.writeln("");

    const resize = () => {
      if (disposedRef.current) {
        return;
      }
      const rect = container.getBoundingClientRect();
      const cols = Math.max(40, Math.floor(rect.width / 8.5));
      const rows = Math.max(12, Math.floor(rect.height / 17));
      terminal.resize(cols, rows);
    };

    resize();
    const observer = new ResizeObserver(() => resize());
    observer.observe(container);
    const decoder = new TextDecoder();
    const dataDisposable = terminal.onData((data) => {
      if (socketRef.current?.readyState === WebSocket.OPEN) {
        socketRef.current.send(data);
      }
    });

    const clearRetryTimer = () => {
      if (retryTimerRef.current !== null) {
        window.clearTimeout(retryTimerRef.current);
        retryTimerRef.current = null;
      }
    };

    const connect = () => {
      clearRetryTimer();
      if (disposedRef.current || !machine) {
        return;
      }
      if (socketRef.current && socketRef.current.readyState === WebSocket.OPEN) {
        return;
      }

      setTerminalStatus("接続中");

      const socket = new WebSocket(`/api/v1/compute/${encodeURIComponent(machineName)}/console?projectId=${encodeURIComponent(projectId)}`);
      socketRef.current = socket;
      socket.binaryType = "arraybuffer";

      socket.onopen = () => {
        if (disposedRef.current) {
          return;
        }
        setTerminalStatus("接続中");
        terminal.writeln("[connected]");
      };

      socket.onmessage = (event) => {
        if (disposedRef.current) {
          return;
        }
        if (typeof event.data === "string") {
          terminal.write(event.data);
          return;
        }
        const payload = event.data instanceof ArrayBuffer ? new Uint8Array(event.data) : new Uint8Array();
        if (payload.length === 0) {
          return;
        }
        terminal.write(decoder.decode(payload, { stream: true }));
      };

      socket.onerror = () => {
        if (disposedRef.current) {
          return;
        }
        setTerminalStatus("接続エラー");
        terminal.writeln("");
        terminal.writeln("[console error]");
      };

      socket.onclose = () => {
        if (disposedRef.current) {
          return;
        }
        socketRef.current = null;
        terminal.write(decoder.decode());
        setTerminalStatus("再接続待ち");
        terminal.writeln("");
        terminal.writeln("[disconnected: retrying]");
        retryTimerRef.current = window.setTimeout(() => {
          if (!disposedRef.current) {
            connect();
          }
        }, 2000);
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
    <Box sx={{ display: "grid", gap: 3 }}>
      <Card variant="outlined" sx={{ borderRadius: 2 }}>
        <CardContent sx={{ p: { xs: 2.5, sm: 3 }, display: "grid", gap: 2.5 }}>
          <Box sx={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", gap: 2, flexWrap: "wrap" }}>
            <Box sx={{ display: "grid", gap: 0.75, minWidth: 0 }}>
              <Typography variant="overline" color="primary">
                仮想マシン詳細
              </Typography>
              <Box sx={{ display: "flex", alignItems: "center", gap: 1.25, minWidth: 0 }}>
                <Box sx={{ width: 34, height: 34, borderRadius: "999px", display: "grid", placeItems: "center", bgcolor: isReady ? "transparent" : alpha("#2563eb", 0.12), color: isReady ? "success.main" : "primary.main" }}>
                  {isReady ? <CheckCircleIcon fontSize="small" /> : <CircularProgress size={16} thickness={5} sx={{ color: "inherit" }} />}
                </Box>
                <Typography variant="h5" sx={{ fontWeight: 700, wordBreak: "break-word" }}>
                  {machine?.name ?? machineName}
                </Typography>
              </Box>
            </Box>
            <Button startIcon={<ArrowBackIcon />} onClick={onBack}>
              一覧に戻る
            </Button>
          </Box>

          <Box
            sx={{
              display: "grid",
              gap: 1.5,
              gridTemplateColumns: { xs: "1fr", md: "repeat(2, minmax(0, 1fr))" }
            }}
          >
            <Paper variant="outlined" sx={{ p: 2, borderRadius: 2 }}>
              <Typography variant="caption" color="text.secondary">
                状態
              </Typography>
              <Box sx={{ mt: 0.75, display: "flex", alignItems: "center", gap: 1, color: isReady ? "success.main" : "text.secondary" }}>
                {isReady ? <CheckCircleIcon fontSize="small" /> : <CircularProgress size={16} thickness={5} sx={{ color: "inherit" }} />}
                <Typography sx={{ fontWeight: 700 }}>{status}</Typography>
              </Box>
            </Paper>
            <Paper variant="outlined" sx={{ p: 2, borderRadius: 2 }}>
              <Typography variant="caption" color="text.secondary">
                イメージ
              </Typography>
              <Typography sx={{ fontWeight: 700, wordBreak: "break-all" }}>{machine?.image ?? "-"}</Typography>
            </Paper>
            <Paper variant="outlined" sx={{ p: 2, borderRadius: 2 }}>
              <Typography variant="caption" color="text.secondary">
                サイズ
              </Typography>
              <Typography sx={{ fontWeight: 700 }}>
                CPU {machine?.cpu ?? "-"} / MEM {machine?.memory ?? "-"}
              </Typography>
            </Paper>
            <Paper variant="outlined" sx={{ p: 2, borderRadius: 2 }}>
              <Typography variant="caption" color="text.secondary">
                更新日時
              </Typography>
              <Typography sx={{ fontWeight: 700 }}>{formatComputeTimestamp(machine?.updatedAt || machine?.createdAt)}</Typography>
            </Paper>
          </Box>

          <Box sx={{ display: "grid", gap: 1 }}>
            <Typography variant="subtitle2" sx={{ fontWeight: 800 }}>
              コンソール
            </Typography>
            {terminalHint ? <Alert severity="info">{terminalHint}</Alert> : null}
            <Paper
              variant="outlined"
              sx={{
                borderRadius: 2,
                overflow: "hidden",
                bgcolor: "#0b1020",
                borderColor: "rgba(148, 163, 184, 0.22)"
              }}
            >
              {isReady ? (
                <Box ref={terminalContainerRef} sx={{ height: { xs: 360, md: 520 }, width: "100%" }} />
              ) : (
                <Box
                  sx={{
                    height: { xs: 360, md: 520 },
                    width: "100%",
                    display: "grid",
                    placeItems: "center",
                    color: "#dbe4ff",
                    fontFamily: '"SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace'
                  }}
                >
                  <Typography variant="body2" sx={{ color: "rgba(219, 228, 255, 0.78)" }}>
                    [waiting for vm to start]
                  </Typography>
                </Box>
              )}
            </Paper>
          </Box>
        </CardContent>
      </Card>
    </Box>
  );
}
