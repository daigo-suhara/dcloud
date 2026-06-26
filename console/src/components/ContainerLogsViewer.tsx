import { Box, Button, Chip, IconButton, Tooltip, Typography } from "@mui/material";
import RefreshIcon from "@mui/icons-material/Refresh";
import DeleteSweepOutlinedIcon from "@mui/icons-material/DeleteSweepOutlined";
import { useEffect, useRef, useState } from "react";

type LogLine = { text: string; timestamp: string };

type ContainerLogsViewerProps = {
  serviceName: string;
  projectId: string;
  enabled: boolean;
  tail?: number;
};

const MAX_LINES = 1000;

export function ContainerLogsViewer({ serviceName, projectId, enabled, tail = 200 }: ContainerLogsViewerProps) {
  const [lines, setLines] = useState<LogLine[]>([]);
  const [status, setStatus] = useState<"idle" | "connecting" | "open" | "error" | "closed">("idle");
  const [errorMessage, setErrorMessage] = useState("");
  const [autoscroll, setAutoscroll] = useState(true);
  const [generation, setGeneration] = useState(0);
  const containerRef = useRef<HTMLDivElement | null>(null);
  const sourceRef = useRef<EventSource | null>(null);

  useEffect(() => {
    if (!enabled || !serviceName) {
      sourceRef.current?.close();
      sourceRef.current = null;
      setStatus("idle");
      return;
    }
    setLines([]);
    setErrorMessage("");
    setStatus("connecting");

    const url = `/api/v1/container/${encodeURIComponent(serviceName)}/logs?tail=${tail}&follow=1&project=${encodeURIComponent(projectId)}`;
    const es = new EventSource(url, { withCredentials: true });
    sourceRef.current = es;

    es.onopen = () => setStatus("open");
    es.onmessage = (event) => {
      try {
        const line = JSON.parse(event.data) as LogLine;
        setLines((prev) => {
          const next = prev.concat(line);
          if (next.length > MAX_LINES) next.splice(0, next.length - MAX_LINES);
          return next;
        });
      } catch {
        setLines((prev) => prev.concat({ text: event.data, timestamp: "" }));
      }
    };
    es.addEventListener("error", (event: MessageEvent) => {
      if (event.data) {
        try {
          const parsed = JSON.parse(event.data) as { detail?: string };
          setErrorMessage(parsed.detail || "ログ取得に失敗しました");
        } catch {
          setErrorMessage(event.data);
        }
      }
    });
    es.onerror = () => {
      setStatus((cur) => (cur === "open" ? "closed" : "error"));
      es.close();
    };

    return () => {
      es.close();
      sourceRef.current = null;
    };
  }, [enabled, serviceName, projectId, tail, generation]);

  useEffect(() => {
    if (!autoscroll || !containerRef.current) return;
    containerRef.current.scrollTop = containerRef.current.scrollHeight;
  }, [lines, autoscroll]);

  function handleScroll() {
    const el = containerRef.current;
    if (!el) return;
    const nearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 40;
    setAutoscroll(nearBottom);
  }

  const statusLabel: Record<typeof status, string> = {
    idle: "停止中",
    connecting: "接続中…",
    open: "ストリーミング中",
    error: "切断",
    closed: "ストリーム終了",
  };
  const statusColor: Record<typeof status, "default" | "primary" | "success" | "error"> = {
    idle: "default",
    connecting: "primary",
    open: "success",
    error: "error",
    closed: "default",
  };

  return (
    <Box sx={{ display: "grid", gap: 1 }}>
      <Box sx={{ display: "flex", justifyContent: "space-between", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          <Typography variant="subtitle2" sx={{ fontWeight: 700 }}>ログ</Typography>
          <Chip label={statusLabel[status]} color={statusColor[status]} size="small" variant="outlined" />
          {!autoscroll && status === "open" && (
            <Chip label="自動スクロール停止中" size="small" variant="outlined" onClick={() => setAutoscroll(true)} />
          )}
        </Box>
        <Box sx={{ display: "flex", gap: 0.5 }}>
          <Tooltip title="再接続">
            <span>
              <IconButton size="small" onClick={() => setGeneration((g) => g + 1)} disabled={!enabled}>
                <RefreshIcon fontSize="small" />
              </IconButton>
            </span>
          </Tooltip>
          <Tooltip title="クリア">
            <IconButton size="small" onClick={() => setLines([])}>
              <DeleteSweepOutlinedIcon fontSize="small" />
            </IconButton>
          </Tooltip>
        </Box>
      </Box>
      {errorMessage && (
        <Typography variant="caption" color="error">{errorMessage}</Typography>
      )}
      <Box
        ref={containerRef}
        onScroll={handleScroll}
        sx={{
          fontFamily: '"SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace',
          fontSize: 12,
          lineHeight: 1.5,
          bgcolor: "#0b1020",
          color: "#dbe4ff",
          borderRadius: 1.5,
          p: 1.5,
          maxHeight: 360,
          minHeight: 200,
          overflow: "auto",
          whiteSpace: "pre-wrap",
          wordBreak: "break-all",
        }}
      >
        {!enabled ? (
          <Typography variant="caption" sx={{ color: "rgba(219,228,255,0.6)" }}>サービスが準備できるとログが表示されます。</Typography>
        ) : lines.length === 0 ? (
          <Typography variant="caption" sx={{ color: "rgba(219,228,255,0.6)" }}>
            {status === "connecting" ? "接続しています…" : "新しいログを待っています…"}
          </Typography>
        ) : (
          lines.map((line, i) => (
            <Box key={i} sx={{ display: "block" }}>
              {line.timestamp && (
                <Box component="span" sx={{ color: "rgba(124,147,246,0.85)", mr: 1 }}>
                  {line.timestamp}
                </Box>
              )}
              {line.text}
            </Box>
          ))
        )}
      </Box>
      {!enabled && status !== "idle" && (
        <Button size="small" onClick={() => setGeneration((g) => g + 1)}>再接続</Button>
      )}
    </Box>
  );
}
