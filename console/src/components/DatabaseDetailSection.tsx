import ArrowBackIcon from "@mui/icons-material/ArrowBack";
import CheckCircleIcon from "@mui/icons-material/CheckCircle";
import ContentCopyIcon from "@mui/icons-material/ContentCopy";
import DeleteOutlinedIcon from "@mui/icons-material/DeleteOutlined";
import { alpha } from "@mui/material/styles";
import {
  Alert, Box, Button, Card, CardContent, CircularProgress, Dialog, DialogActions,
  DialogContent, DialogTitle, IconButton, Paper, TextField, Tooltip, Typography
} from "@mui/material";
import { useCallback, useEffect, useState } from "react";
import type { DatabaseConnectionInfo, DatabaseInstance, DatabaseSchema } from "../types";
import { formatComputeTimestamp } from "../utils";

const DB_TYPE_LABELS: Record<string, string> = {
  postgres: "PostgreSQL",
  mysql: "MySQL",
  redis: "Redis",
};

type DatabaseDetailSectionProps = {
  database: DatabaseInstance | null;
  databaseName: string;
  loading: boolean;
  activeProjectId: string;
  onBack: () => void;
};

export function DatabaseDetailSection({
  database,
  databaseName,
  loading,
  activeProjectId,
  onBack,
}: DatabaseDetailSectionProps) {
  const [schemas, setSchemas] = useState<DatabaseSchema[]>([]);
  const [schemasLoading, setSchemasLoading] = useState(false);
  const [schemasError, setSchemasError] = useState("");
  const [createOpen, setCreateOpen] = useState(false);
  const [newSchemaName, setNewSchemaName] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [createError, setCreateError] = useState("");
  const [deleting, setDeleting] = useState<string>("");
  const [connInfo, setConnInfo] = useState<Record<string, DatabaseConnectionInfo>>({});
  const [openedConn, setOpenedConn] = useState<string>("");
  const [connLoading, setConnLoading] = useState<string>("");
  const isManagedType = database?.type === "mysql" || database?.type === "postgres";
  const isReady = database?.ready ?? false;

  const loadSchemas = useCallback(async () => {
    if (!databaseName || !isManagedType || !isReady) return;
    setSchemasLoading(true);
    setSchemasError("");
    try {
      const response = await fetch(`/api/v1/database/${encodeURIComponent(databaseName)}/schemas`, {
        credentials: "include",
        headers: { "X-DCP-Project": activeProjectId }
      });
      if (!response.ok) throw new Error(await response.text());
      const data = await response.json() as { schemas: DatabaseSchema[] };
      setSchemas(data.schemas ?? []);
    } catch (err) {
      setSchemasError(err instanceof Error ? err.message : "スキーマの取得に失敗しました");
    } finally {
      setSchemasLoading(false);
    }
  }, [databaseName, activeProjectId, isManagedType, isReady]);

  useEffect(() => { void loadSchemas(); }, [loadSchemas]);

  async function handleCreate() {
    const name = newSchemaName.trim();
    if (!name) return;
    setSubmitting(true);
    setCreateError("");
    try {
      const response = await fetch(`/api/v1/database/${encodeURIComponent(databaseName)}/schemas`, {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json", "X-DCP-Project": activeProjectId },
        body: JSON.stringify({ schemaName: name })
      });
      if (!response.ok) {
        const text = await response.text();
        throw new Error(text || "作成に失敗しました");
      }
      setNewSchemaName("");
      setCreateOpen(false);
      await loadSchemas();
    } catch (err) {
      setCreateError(err instanceof Error ? err.message : "作成に失敗しました");
    } finally {
      setSubmitting(false);
    }
  }

  async function handleDelete(name: string) {
    setDeleting(name);
    try {
      const response = await fetch(
        `/api/v1/database/${encodeURIComponent(databaseName)}/schemas/${encodeURIComponent(name)}`,
        { method: "DELETE", credentials: "include", headers: { "X-DCP-Project": activeProjectId } }
      );
      if (!response.ok) throw new Error("削除に失敗しました");
      await loadSchemas();
    } catch (err) {
      setSchemasError(err instanceof Error ? err.message : "削除に失敗しました");
    } finally {
      setDeleting("");
    }
  }

  async function handleToggleConnection(schemaName: string) {
    if (openedConn === schemaName) {
      setOpenedConn("");
      return;
    }
    if (!connInfo[schemaName]) {
      setConnLoading(schemaName);
      try {
        const url = `/api/v1/database/${encodeURIComponent(databaseName)}/connection?schema=${encodeURIComponent(schemaName)}`;
        const response = await fetch(url, { credentials: "include", headers: { "X-DCP-Project": activeProjectId } });
        if (!response.ok) throw new Error("接続情報の取得に失敗しました");
        const data = await response.json() as DatabaseConnectionInfo;
        setConnInfo(prev => ({ ...prev, [schemaName]: data }));
      } catch (err) {
        setSchemasError(err instanceof Error ? err.message : "接続情報の取得に失敗しました");
        return;
      } finally {
        setConnLoading("");
      }
    }
    setOpenedConn(schemaName);
  }

  function copyToClipboard(text: string) {
    void navigator.clipboard.writeText(text);
  }

  const headerStatus = loading
    ? "読み込み中"
    : !database
      ? "未検出"
      : isReady
        ? "正常"
        : database.status || "準備中";

  return (
    <Box sx={{ display: "grid", gap: 3 }}>
      <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
        <Button startIcon={<ArrowBackIcon />} onClick={onBack} size="small">一覧に戻る</Button>
      </Box>

      <Card variant="outlined" sx={{ borderRadius: 2 }}>
        <CardContent sx={{ p: 3, display: "grid", gap: 1.5 }}>
          <Box sx={{ display: "flex", alignItems: "center", gap: 2, flexWrap: "wrap" }}>
            <Typography variant="h5" sx={{ fontWeight: 700, wordBreak: "break-all" }}>{databaseName}</Typography>
            <Box sx={{ display: "flex", alignItems: "center", gap: 0.5, color: isReady ? "success.main" : "text.secondary" }}>
              {isReady ? <CheckCircleIcon fontSize="small" /> : <CircularProgress size={14} thickness={5.5} />}
              <Typography variant="caption" sx={{ fontWeight: 600 }}>{headerStatus}</Typography>
            </Box>
          </Box>
          {database && (
            <Box sx={{ display: "grid", gridTemplateColumns: { xs: "1fr 1fr", sm: "repeat(4, minmax(0, 1fr))" }, gap: 2 }}>
              <Box>
                <Typography variant="caption" color="text.secondary">種別</Typography>
                <Typography variant="body2" sx={{ fontWeight: 600 }}>{DB_TYPE_LABELS[database.type] ?? database.type}</Typography>
              </Box>
              <Box>
                <Typography variant="caption" color="text.secondary">バージョン</Typography>
                <Typography variant="body2">{database.version}</Typography>
              </Box>
              <Box>
                <Typography variant="caption" color="text.secondary">リソース</Typography>
                <Typography variant="body2">CPU {database.cpu} / Mem {database.memory}</Typography>
              </Box>
              <Box>
                <Typography variant="caption" color="text.secondary">作成日時</Typography>
                <Typography variant="body2">{formatComputeTimestamp(database.createdAt)}</Typography>
              </Box>
            </Box>
          )}
        </CardContent>
      </Card>

      <Card variant="outlined" sx={{ borderRadius: 2 }}>
        <CardContent sx={{ p: 3, display: "grid", gap: 2 }}>
          <Box sx={{ display: "flex", justifyContent: "space-between", alignItems: "center", gap: 2, flexWrap: "wrap" }}>
            <Typography variant="h6" sx={{ fontWeight: 700 }}>データベース (スキーマ)</Typography>
            <Button
              variant="contained"
              size="small"
              disabled={!isManagedType || !isReady}
              onClick={() => setCreateOpen(true)}
            >
              データベースを追加
            </Button>
          </Box>
          {!isManagedType && (
            <Alert severity="info">この種別ではスキーマ管理は対応していません。</Alert>
          )}
          {isManagedType && !isReady && (
            <Alert severity="info">インスタンスが準備中です。Running になると操作できます。</Alert>
          )}
          {schemasError && <Alert severity="error" onClose={() => setSchemasError("")}>{schemasError}</Alert>}

          {isManagedType && isReady && (
            <Box sx={{ display: "grid", gap: 0 }}>
              <Box
                sx={{
                  display: { xs: "none", sm: "grid" },
                  gridTemplateColumns: "minmax(0, 1fr) 220px",
                  alignItems: "center",
                  minHeight: 36,
                  px: 1,
                  color: "text.secondary",
                  fontSize: 11,
                  fontWeight: 700,
                  borderBottom: "1px solid rgba(148, 163, 184, 0.18)"
                }}
              >
                <Box>名前</Box>
                <Box sx={{ textAlign: "right" }}>操作</Box>
              </Box>
              {schemasLoading ? (
                <Box sx={{ p: 2 }}><CircularProgress size={18} /></Box>
              ) : schemas.length === 0 ? (
                <Paper variant="outlined" sx={{ mt: 1.5, p: 2, borderRadius: 2, borderStyle: "dashed" }}>
                  <Typography color="text.secondary">まだデータベース(スキーマ)はありません。</Typography>
                </Paper>
              ) : schemas.map(schema => {
                const isDeleting = deleting === schema.name;
                return (
                  <Box key={schema.name}>
                    <Paper
                      variant="outlined"
                      sx={{
                        display: "grid",
                        gridTemplateColumns: "minmax(0, 1fr) 220px",
                        alignItems: "center",
                        minHeight: 44,
                        borderRadius: 0,
                        borderLeft: 0,
                        borderRight: 0,
                        borderTop: 0
                      }}
                    >
                      <Box sx={{ minWidth: 0, px: 1 }}>
                        <Typography variant="body2" sx={{ fontWeight: 600, fontFamily: "monospace", wordBreak: "break-all" }}>
                          {schema.name}
                        </Typography>
                      </Box>
                      <Box sx={{ display: "flex", justifyContent: "flex-end", gap: 0.5, pr: 0.5 }}>
                        <Button
                          size="small"
                          variant="outlined"
                          disabled={connLoading === schema.name}
                          onClick={() => void handleToggleConnection(schema.name)}
                        >
                          {openedConn === schema.name ? "隠す" : "接続情報"}
                        </Button>
                        <Tooltip title="削除">
                          <span>
                            <IconButton
                              color="error"
                              disabled={isDeleting}
                              onClick={() => void handleDelete(schema.name)}
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
                              {isDeleting ? <CircularProgress size={14} sx={{ color: "inherit" }} /> : <DeleteOutlinedIcon fontSize="small" />}
                            </IconButton>
                          </span>
                        </Tooltip>
                      </Box>
                    </Paper>
                    {openedConn === schema.name && connInfo[schema.name] && (
                      <Box sx={{ px: 2, py: 1.5, bgcolor: alpha("#1e293b", 0.04), borderBottom: "1px solid rgba(148,163,184,0.18)" }}>
                        <Box sx={{ display: "grid", gap: 1 }}>
                          {[
                            { label: "接続文字列", value: connInfo[schema.name].connectionString },
                            { label: "Host", value: connInfo[schema.name].host },
                            { label: "Port", value: connInfo[schema.name].port },
                            ...(connInfo[schema.name].username ? [{ label: "Username", value: connInfo[schema.name].username }] : []),
                            { label: "Password", value: connInfo[schema.name].password },
                            { label: "Database", value: connInfo[schema.name].databaseName },
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
                      </Box>
                    )}
                  </Box>
                );
              })}
            </Box>
          )}
        </CardContent>
      </Card>

      <Dialog open={createOpen} onClose={() => !submitting && setCreateOpen(false)} fullWidth maxWidth="xs">
        <DialogTitle>データベースを追加</DialogTitle>
        <DialogContent sx={{ display: "grid", gap: 2, pt: "8px !important" }}>
          {createError && <Typography color="error" variant="body2">{createError}</Typography>}
          <TextField
            label="名前"
            value={newSchemaName}
            onChange={(e) => setNewSchemaName(e.target.value)}
            size="small"
            fullWidth
            autoFocus
            disabled={submitting}
            helperText="英数字とアンダースコアのみ・最大64文字"
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setCreateOpen(false)} disabled={submitting}>キャンセル</Button>
          <Button variant="contained" onClick={() => void handleCreate()} disabled={submitting || !newSchemaName.trim()}>
            {submitting ? <CircularProgress size={16} /> : "作成"}
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}
