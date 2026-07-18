import AddIcon from "@mui/icons-material/Add";
import DeleteOutlinedIcon from "@mui/icons-material/DeleteOutlined";
import RefreshIcon from "@mui/icons-material/Refresh";
import { Alert, Box, Button, CircularProgress, IconButton, Tooltip, Typography } from "@mui/material";
import { useCallback, useEffect, useState } from "react";
import { formatComputeTimestamp, formatStatus } from "../utils";
import { DataTable, StatusBadge } from "./primitives";
import type { Column, StatusVariant } from "./primitives";

type Backup = {
  name: string;
  status: string;
  method: string;
  totalSize?: string;
  createdAt?: string;
  completedAt?: string;
};

type Props = {
  dbName: string;
  projectId: string;
  enabled: boolean;
};

function statusVariant(phase: string): StatusVariant {
  const p = phase.toLowerCase();
  if (p === "completed") return "ready";
  if (p === "failed") return "error";
  return "progress";
}

export function BackupsPanel({ dbName, projectId, enabled }: Props) {
  const [backups, setBackups] = useState<Backup[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [creating, setCreating] = useState(false);
  const [deleting, setDeleting] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const res = await fetch(`/api/v1/database/${encodeURIComponent(dbName)}/backups`, {
        credentials: "include", headers: { "X-DCP-Project": projectId }
      });
      if (!res.ok) throw new Error(await res.text());
      const data = await res.json() as { backups: Backup[] };
      setBackups(data.backups ?? []);
    } catch (err) {
      setError(err instanceof Error ? err.message : "バックアップを取得できませんでした");
    } finally { setLoading(false); }
  }, [dbName, projectId]);

  useEffect(() => {
    if (!enabled) return;
    void load();
    const t = setInterval(() => void load(), 10000);
    return () => clearInterval(t);
  }, [enabled, load]);

  async function handleCreate() {
    setCreating(true); setError("");
    try {
      const res = await fetch(`/api/v1/database/${encodeURIComponent(dbName)}/backups`, {
        method: "POST", credentials: "include", headers: { "X-DCP-Project": projectId }
      });
      if (!res.ok) throw new Error(await res.text());
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "バックアップに失敗しました");
    } finally { setCreating(false); }
  }

  async function handleDelete(name: string) {
    setDeleting(name);
    try {
      const res = await fetch(`/api/v1/database/${encodeURIComponent(dbName)}/backups/${encodeURIComponent(name)}`, {
        method: "DELETE", credentials: "include", headers: { "X-DCP-Project": projectId }
      });
      if (!res.ok) throw new Error(await res.text());
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "削除に失敗しました");
    } finally { setDeleting(""); }
  }

  const columns: Column<Backup>[] = [
    {
      key: "name", header: "名前",
      render: (b) => <Typography variant="body2" sx={{ fontFamily: "monospace", wordBreak: "break-all" }}>{b.name}</Typography>
    },
    {
      key: "status", header: "ステータス", width: 140,
      render: (b) => <StatusBadge variant={statusVariant(b.status)} label={formatStatus(b.status, "処理中")} showSpinner={statusVariant(b.status) === "progress"} />
    },
    { key: "size", header: "サイズ", width: 100, render: (b) => <Typography variant="caption" color="text.secondary">{b.totalSize || "-"}</Typography> },
    { key: "createdAt", header: "作成日時", width: 180, render: (b) => <Typography variant="caption" color="text.secondary">{formatComputeTimestamp(b.createdAt)}</Typography> },
    {
      key: "actions", header: "", width: 60, align: "right",
      render: (b) => (
        <Tooltip title="削除">
          <span>
            <IconButton size="small" color="error"
              disabled={deleting === b.name}
              onClick={() => void handleDelete(b.name)}>
              {deleting === b.name ? <CircularProgress size={14} /> : <DeleteOutlinedIcon fontSize="small" />}
            </IconButton>
          </span>
        </Tooltip>
      )
    }
  ];

  return (
    <Box sx={{ mt: 4 }}>
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", mb: 2 }}>
        <Typography variant="h6">バックアップ</Typography>
        <Box sx={{ display: "flex", gap: 0.5 }}>
          <Tooltip title="更新">
            <IconButton size="small" onClick={() => void load()}>
              <RefreshIcon fontSize="small" />
            </IconButton>
          </Tooltip>
          <Button
            variant="contained" size="small" startIcon={<AddIcon />}
            disabled={creating || !enabled}
            onClick={() => void handleCreate()}
          >
            {creating ? "作成中..." : "バックアップ"}
          </Button>
        </Box>
      </Box>

      {!enabled && <Alert severity="info" sx={{ mb: 2 }}>この種別ではバックアップに対応していません。</Alert>}
      {error && <Alert severity="error" onClose={() => setError("")} sx={{ mb: 2 }}>{error}</Alert>}

      <DataTable
        columns={columns}
        rows={backups}
        rowKey={(b) => b.name}
        loading={loading}
        emptyMessage="バックアップはまだありません"
      />
    </Box>
  );
}
