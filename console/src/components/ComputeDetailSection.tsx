import ArrowBackIcon from "@mui/icons-material/ArrowBack";
import CheckCircleIcon from "@mui/icons-material/CheckCircle";
import { alpha } from "@mui/material/styles";
import { Alert, Box, Button, Card, CardContent, CircularProgress, Paper, Typography } from "@mui/material";
import { useEffect, useMemo, useRef, useState } from "react";
import { Terminal } from "xterm";
import "xterm/css/xterm.css";
import type { ComputeMachine } from "../types";
import { actionLinkButtonSx } from "../theme";
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
  const [terminalStatus, setTerminalStatus] = useState("接続待ち");

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

    let disposed = false;
    const resize = () => {
      if (disposed) {
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

    const socket = machine ? new WebSocket(`/api/v1/compute/${encodeURIComponent(machineName)}/console?projectId=${encodeURIComponent(projectId)}`) : null;
    socketRef.current = socket;

    if (socket) {
      socket.binaryType = "arraybuffer";
      socket.onopen = () => {
        if (disposed) {
          return;
        }
        setTerminalStatus("接続中");
        terminal.writeln("[connected]");
      };
      socket.onmessage = (event) => {
        if (disposed) {
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
        terminal.write(new TextDecoder().decode(payload));
      };
      socket.onerror = () => {
        if (disposed) {
          return;
        }
        setTerminalStatus("接続エラー");
        terminal.writeln("");
        terminal.writeln("[console error]");
      };
      socket.onclose = () => {
        if (disposed) {
          return;
        }
        setTerminalStatus("切断");
        terminal.writeln("");
        terminal.writeln("[disconnected]");
      };

      const dataDisposable = terminal.onData((data) => {
        if (socket.readyState === WebSocket.OPEN) {
          socket.send(data);
        }
      });

      return () => {
        disposed = true;
        dataDisposable.dispose();
        observer.disconnect();
        socket.close();
        terminal.dispose();
        socketRef.current = null;
        terminalRef.current = null;
      };
    }

    const dataDisposable = terminal.onData(() => {
      // no-op until the console connection is available
    });

    return () => {
      disposed = true;
      dataDisposable.dispose();
      observer.disconnect();
      terminal.dispose();
      socketRef.current = null;
      terminalRef.current = null;
    };
  }, [isReady, machine?.name, machine?.ready, loading, machineName, projectId]);

  return (
    <Box sx={{ display: "grid", gap: 3 }}>
      <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}>
        <Button type="button" variant="text" size="small" startIcon={<ArrowBackIcon fontSize="small" />} onClick={onBack} sx={actionLinkButtonSx}>
          一覧へ戻る
        </Button>
        <Typography variant="h5" sx={{ fontWeight: 800 }}>
          仮想マシン
        </Typography>
      </Box>

      <Card variant="outlined" sx={{ borderRadius: 2 }}>
        <CardContent sx={{ p: { xs: 2.5, sm: 3 }, display: "grid", gap: 2.5 }}>
          <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 2, flexWrap: "wrap" }}>
            <Box sx={{ display: "grid", gap: 0.5 }}>
              <Typography variant="h6" sx={{ fontWeight: 800, wordBreak: "break-all" }}>
                {machine?.name ?? machineName}
              </Typography>
              <Typography variant="body2" color="text.secondary">
                {status}
              </Typography>
            </Box>
            <Box sx={{ display: "flex", alignItems: "center", gap: 1, color: isReady ? "success.main" : "text.secondary" }}>
              {isReady ? <CheckCircleIcon fontSize="small" /> : <CircularProgress size={16} thickness={5} sx={{ color: "inherit" }} />}
              <Typography variant="caption">{terminalStatus}</Typography>
            </Box>
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
            <Paper variant="outlined" sx={{ p: 2, borderRadius: 2 }}>
              <Typography variant="caption" color="text.secondary">
                名前空間
              </Typography>
              <Typography sx={{ fontWeight: 700, wordBreak: "break-all" }}>{machine?.namespace ?? "-"}</Typography>
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
              <Box ref={terminalContainerRef} sx={{ height: { xs: 360, md: 520 }, width: "100%" }} />
            </Paper>
          </Box>
        </CardContent>
      </Card>
    </Box>
  );
}
