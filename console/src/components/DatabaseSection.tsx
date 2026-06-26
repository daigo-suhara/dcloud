import CheckCircleIcon from "@mui/icons-material/CheckCircle";
import ContentCopyIcon from "@mui/icons-material/ContentCopy";
import DeleteOutlinedIcon from "@mui/icons-material/DeleteOutlined";
import LinkIcon from "@mui/icons-material/Link";
import { alpha } from "@mui/material/styles";
import {
  Box, Button, Card, CardContent, CircularProgress, Collapse, Dialog, DialogContent,
  DialogTitle, IconButton, MenuItem, Paper, TextField, Tooltip, Typography
} from "@mui/material";
import { useState } from "react";
import type { DatabaseCreateForm, DatabaseInstance } from "../types";
import { formatComputeTimestamp } from "../utils";

const DB_TYPES = [
  { value: "postgres", label: "PostgreSQL" },
  { value: "mysql", label: "MySQL" },
  { value: "redis", label: "Redis" },
];

const DEFAULT_VERSIONS: Record<string, string> = {
  postgres: "postgresql-16.4.0",
  mysql: "mysql-8.4.2",
  redis: "redis-7.2.4",
};

const initialForm: DatabaseCreateForm = {
  name: "",
  type: "postgres",
  version: "",
  cpu: "500m",
  memory: "1Gi",
  storage: "1Gi",
};

type ConnectionInfo = {
  connectionString: string;
  host: string;
  port: string;
  username: string;
  password: string;
  databaseName: string;
};

type DatabaseSectionProps = {
  loading: boolean;
  databases: DatabaseInstance[];
  deletingDatabaseName: string;
  onDeleteDatabase: (name: string) => void;
  onCreateDatabase: (form: DatabaseCreateForm) => Promise<void>;
  onOpenDatabase: (name: string) => void;
  activeProjectId: string;
};

export function DatabaseSection({
  loading,
  databases,
  deletingDatabaseName,
  onDeleteDatabase,
  onCreateDatabase,
  onOpenDatabase,
  activeProjectId
}: DatabaseSectionProps) {
  const [createOpen, setCreateOpen] = useState(false);
  const [form, setForm] = useState<DatabaseCreateForm>(initialForm);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");
  const [connOpen, setConnOpen] = useState<string | null>(null);
  const [connInfo, setConnInfo] = useState<Record<string, ConnectionInfo>>({});
  const [connLoading, setConnLoading] = useState(false);

  async function handleCreate() {
    if (!form.name.trim() || !form.type) return;
    setSubmitting(true);
    setError("");
    try {
      await onCreateDatabase(form);
      setForm(initialForm);
      setCreateOpen(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : "作成に失敗しました");
    } finally {
      setSubmitting(false);
    }
  }

  async function handleShowConnection(name: string) {
    if (connInfo[name]) {
      setConnOpen(connOpen === name ? null : name);
      return;
    }
    setConnLoading(true);
    setConnOpen(name);
    try {
      const response = await fetch(`/api/v1/database/${encodeURIComponent(name)}/connection`, {
        credentials: "include",
        headers: { "X-DCP-Project": activeProjectId }
      });
      if (!response.ok) throw new Error("接続情報の取得に失敗しました");
      const data = await response.json() as ConnectionInfo;
      setConnInfo(prev => ({ ...prev, [name]: data }));
    } catch {
      setConnOpen(null);
    } finally {
      setConnLoading(false);
    }
  }

  function copyToClipboard(text: string) {
    void navigator.clipboard.writeText(text);
  }

  function dbTypeLabel(type: string) {
    return DB_TYPES.find(d => d.value === type)?.label ?? type;
  }

  return (
    <Box sx={{ display: "grid", gap: 3 }}>
      <Card variant="outlined" sx={{ borderRadius: 2 }}>
        <CardContent sx={{ p: 3, display: "grid", gap: 2 }}>
          <Box sx={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", gap: 2, flexWrap: "wrap" }}>
            <Box sx={{ display: "grid", gap: 0.75 }}>
              <Typography variant="h5" sx={{ fontWeight: 700 }}>
                データベース
              </Typography>
            </Box>
            <Button variant="contained" onClick={() => setCreateOpen(true)}>
              データベースを作成
            </Button>
          </Box>

          <Box sx={{ display: "grid", gap: 0 }}>
            <Box
              sx={{
                display: { xs: "none", sm: "grid" },
                gridTemplateColumns: "42px minmax(0, 1fr) 100px 100px 160px 88px",
                alignItems: "center",
                minHeight: 36,
                px: 1,
                color: "text.secondary",
                fontSize: 11,
                fontWeight: 700,
                borderBottom: "1px solid rgba(148, 163, 184, 0.18)"
              }}
            >
              <Box />
              <Box>名前</Box>
              <Box>種別</Box>
              <Box>ステータス</Box>
              <Box>作成日時</Box>
              <Box sx={{ textAlign: "right" }}>操作</Box>
            </Box>

            <Box sx={{ borderTop: "1px solid rgba(148, 163, 184, 0.18)" }}>
              {databases.length > 0 ? (
                databases.map((db) => {
                  const isDeleting = deletingDatabaseName === db.name;
                  const isReady = db.ready;
                  const statusIcon = isDeleting || !isReady
                    ? <CircularProgress size={14} thickness={5.5} sx={{ color: "inherit" }} />
                    : <CheckCircleIcon fontSize="small" />;
                  const statusBgColor = isDeleting ? alpha("#dc2626", 0.12) : isReady ? "transparent" : alpha("#2563eb", 0.12);
                  const statusTextColor = isDeleting ? "error.main" : isReady ? "success.main" : "primary.main";

                  return (
                    <Box key={db.name}>
                      <Paper
                        variant="outlined"
                        sx={{
                          display: "grid",
                          gridTemplateColumns: "42px minmax(0, 1fr) 100px 100px 160px 88px",
                          gap: 0,
                          alignItems: "center",
                          minHeight: 44,
                          borderRadius: 0,
                          borderLeft: 0,
                          borderRight: 0,
                          borderTop: 0
                        }}
                      >
                        <Box sx={{ display: "grid", placeItems: "center" }}>
                          <Box sx={{ width: 22, height: 22, display: "grid", placeItems: "center", borderRadius: "999px", bgcolor: statusBgColor, color: statusTextColor }}>
                            {statusIcon}
                          </Box>
                        </Box>
                        <Box sx={{ minWidth: 0 }}>
                          <Typography
                            variant="body2"
                            sx={{ fontWeight: 700, wordBreak: "break-all", cursor: "pointer", "&:hover": { textDecoration: "underline" } }}
                            onClick={() => onOpenDatabase(db.name)}
                          >
                            {db.name}
                          </Typography>
                          <Typography variant="caption" color="text.secondary">{db.version}</Typography>
                        </Box>
                        <Box>
                          <Typography variant="caption" sx={{ fontWeight: 600 }}>{dbTypeLabel(db.type)}</Typography>
                        </Box>
                        <Box>
                          <Typography variant="caption" color="text.secondary">{db.status || (isReady ? "Running" : "Creating")}</Typography>
                        </Box>
                        <Box>
                          <Typography variant="caption" color="text.secondary" sx={{ whiteSpace: "nowrap" }}>
                            {formatComputeTimestamp(db.createdAt)}
                          </Typography>
                        </Box>
                        <Box sx={{ display: "flex", justifyContent: "flex-end", gap: 0.5, pr: 0.5 }}>
                          <Tooltip title="接続情報">
                            <IconButton
                              size="small"
                              disabled={!isReady}
                              onClick={() => void handleShowConnection(db.name)}
                              sx={{ border: "1px solid", borderColor: "divider" }}
                            >
                              <LinkIcon fontSize="small" />
                            </IconButton>
                          </Tooltip>
                          <Tooltip title="削除">
                            <span>
                              <IconButton
                                color="error"
                                disabled={isDeleting}
                                onClick={() => onDeleteDatabase(db.name)}
                                size="small"
                                sx={{
                                  border: "1px solid",
                                  borderColor: "error.main",
                                  bgcolor: "error.main",
                                  color: "common.white",
                                  "&:hover": { bgcolor: "error.dark", borderColor: "error.dark" },
                                  "&.Mui-disabled": { bgcolor: alpha("#dc2626", 0.08), color: "error.main", borderColor: alpha("#dc2626", 0.2) }
                                }}
                              >
                                <DeleteOutlinedIcon fontSize="small" />
                              </IconButton>
                            </span>
                          </Tooltip>
                        </Box>
                      </Paper>

                      <Collapse in={connOpen === db.name}>
                        <Box sx={{ px: 2, py: 1.5, bgcolor: alpha("#1e293b", 0.04), borderBottom: "1px solid rgba(148,163,184,0.18)" }}>
                          {connLoading && !connInfo[db.name] ? (
                            <CircularProgress size={16} />
                          ) : connInfo[db.name] ? (
                            <Box sx={{ display: "grid", gap: 1 }}>
                              {[
                                { label: "接続文字列", value: connInfo[db.name].connectionString },
                                { label: "Host", value: connInfo[db.name].host },
                                { label: "Port", value: connInfo[db.name].port },
                                ...(connInfo[db.name].username ? [{ label: "Username", value: connInfo[db.name].username }] : []),
                                { label: "Password", value: connInfo[db.name].password },
                              ].map(({ label, value }) => (
                                <Box key={label} sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                                  <Typography variant="caption" color="text.secondary" sx={{ minWidth: 100 }}>{label}</Typography>
                                  <Typography variant="caption" sx={{ fontFamily: "monospace", wordBreak: "break-all", flex: 1 }}>{value}</Typography>
                                  <IconButton size="small" onClick={() => copyToClipboard(value)}>
                                    <ContentCopyIcon sx={{ fontSize: 14 }} />
                                  </IconButton>
                                </Box>
                              ))}
                            </Box>
                          ) : null}
                        </Box>
                      </Collapse>
                    </Box>
                  );
                })
              ) : (
                <Paper variant="outlined" sx={{ mt: 1.5, p: 2, borderRadius: 2, borderStyle: "dashed", bgcolor: alpha("#ffffff", 0.7) }}>
                  <Typography color="text.secondary">{loading ? "読み込み中..." : "まだデータベースはありません。"}</Typography>
                </Paper>
              )}
            </Box>
          </Box>
        </CardContent>
      </Card>

      <Dialog open={createOpen} onClose={() => !submitting && setCreateOpen(false)} fullWidth maxWidth="xs">
        <DialogTitle>データベースを作成</DialogTitle>
        <DialogContent sx={{ display: "grid", gap: 2, pt: "8px !important" }}>
          {error && <Typography color="error" variant="body2">{error}</Typography>}
          <TextField
            label="名前"
            value={form.name}
            onChange={(e) => setForm(f => ({ ...f, name: e.target.value }))}
            size="small"
            fullWidth
            helperText="小文字・数字・ハイフンのみ"
            disabled={submitting}
          />
          <TextField
            select
            label="種別"
            value={form.type}
            onChange={(e) => setForm(f => ({ ...f, type: e.target.value, version: DEFAULT_VERSIONS[e.target.value] ?? "" }))}
            size="small"
            fullWidth
            disabled={submitting}
          >
            {DB_TYPES.map(opt => (
              <MenuItem key={opt.value} value={opt.value}>{opt.label}</MenuItem>
            ))}
          </TextField>
          <TextField
            label="バージョン"
            value={form.version || DEFAULT_VERSIONS[form.type] || ""}
            onChange={(e) => setForm(f => ({ ...f, version: e.target.value }))}
            size="small"
            fullWidth
            disabled={submitting}
          />
          <Box sx={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 1 }}>
            <TextField label="CPU" value={form.cpu} onChange={(e) => setForm(f => ({ ...f, cpu: e.target.value }))} size="small" disabled={submitting} />
            <TextField label="メモリ" value={form.memory} onChange={(e) => setForm(f => ({ ...f, memory: e.target.value }))} size="small" disabled={submitting} />
          </Box>
          <TextField
            label="ストレージ"
            value={form.storage}
            onChange={(e) => setForm(f => ({ ...f, storage: e.target.value }))}
            size="small"
            fullWidth
            disabled={submitting}
          />
          <Box sx={{ display: "flex", gap: 1, justifyContent: "flex-end" }}>
            <Button onClick={() => setCreateOpen(false)} disabled={submitting}>キャンセル</Button>
            <Button variant="contained" onClick={() => void handleCreate()} disabled={submitting || !form.name.trim()}>
              {submitting ? <CircularProgress size={18} /> : "作成"}
            </Button>
          </Box>
        </DialogContent>
      </Dialog>
    </Box>
  );
}
