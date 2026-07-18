import ArrowBackIcon from "@mui/icons-material/ArrowBack";
import AddIcon from "@mui/icons-material/Add";
import ContentCopyIcon from "@mui/icons-material/ContentCopy";
import DeleteOutlinedIcon from "@mui/icons-material/DeleteOutlined";
import {
  Alert, Box, Button, CircularProgress, Collapse, IconButton, Paper, TextField, Tooltip, Typography
} from "@mui/material";
import { useCallback, useEffect, useState } from "react";
import type { DatabaseConnectionInfo, DatabaseInstance, DatabaseSchema } from "../types";
import { formatComputeTimestamp, formatStatus } from "../utils";
import { monoFontFamily } from "../theme";
import { PageHeader, DataTable, StatusBadge, FormDialog } from "./primitives";
import type { Column, StatusVariant } from "./primitives";

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

function detailStatus(db: DatabaseInstance | null, loading: boolean): { v: StatusVariant; label: string; spin: boolean } {
  if (!db) return { v: "unknown", label: loading ? "読み込み中" : "未検出", spin: loading };
  if (db.ready) return { v: "ready", label: formatStatus(db.status, "実行中"), spin: false };
  return { v: "progress", label: formatStatus(db.status, "準備中"), spin: true };
}

export function DatabaseDetailSection({
  database, databaseName, loading, activeProjectId, onBack,
}: DatabaseDetailSectionProps) {
  const [schemas, setSchemas] = useState<DatabaseSchema[]>([]);
  const [schemasLoading, setSchemasLoading] = useState(false);
  const [schemasError, setSchemasError] = useState("");
  const [createOpen, setCreateOpen] = useState(false);
  const [newSchemaName, setNewSchemaName] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [createError, setCreateError] = useState("");
  const [deleting, setDeleting] = useState("");
  const [connInfo, setConnInfo] = useState<Record<string, DatabaseConnectionInfo>>({});
  const [openedConn, setOpenedConn] = useState("");
  const [connLoading, setConnLoading] = useState("");

  const isManagedType = database?.type === "mysql" || database?.type === "postgres";
  const isReady = database?.ready ?? false;
  const s = detailStatus(database, loading);

  const loadSchemas = useCallback(async () => {
    if (!databaseName || !isManagedType || !isReady) return;
    setSchemasLoading(true);
    setSchemasError("");
    try {
      const response = await fetch(`/api/v1/database/${encodeURIComponent(databaseName)}/schemas`, {
        credentials: "include", headers: { "X-DCP-Project": activeProjectId }
      });
      if (!response.ok) throw new Error(await response.text());
      const data = await response.json() as { schemas: DatabaseSchema[] };
      setSchemas(data.schemas ?? []);
    } catch (err) {
      setSchemasError(err instanceof Error ? err.message : "スキーマの取得に失敗しました");
    } finally { setSchemasLoading(false); }
  }, [databaseName, activeProjectId, isManagedType, isReady]);

  useEffect(() => { void loadSchemas(); }, [loadSchemas]);

  async function handleCreate() {
    const name = newSchemaName.trim();
    if (!name) return;
    setSubmitting(true); setCreateError("");
    try {
      const response = await fetch(`/api/v1/database/${encodeURIComponent(databaseName)}/schemas`, {
        method: "POST", credentials: "include",
        headers: { "Content-Type": "application/json", "X-DCP-Project": activeProjectId },
        body: JSON.stringify({ schemaName: name })
      });
      if (!response.ok) throw new Error(await response.text() || "作成に失敗しました");
      setNewSchemaName(""); setCreateOpen(false);
      await loadSchemas();
    } catch (err) {
      setCreateError(err instanceof Error ? err.message : "作成に失敗しました");
    } finally { setSubmitting(false); }
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
    } finally { setDeleting(""); }
  }

  async function handleToggleConnection(schemaName: string) {
    if (openedConn === schemaName) { setOpenedConn(""); return; }
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
      } finally { setConnLoading(""); }
    }
    setOpenedConn(schemaName);
  }

  function copyToClipboard(text: string) { void navigator.clipboard.writeText(text); }

  const columns: Column<DatabaseSchema>[] = [
    {
      key: "name", header: "名前",
      render: (schema) => (
        <Typography variant="body2" sx={{ fontFamily: monoFontFamily, wordBreak: "break-all" }}>
          {schema.name}
        </Typography>
      )
    },
    {
      key: "actions", header: "", width: 180, align: "right",
      render: (schema) => {
        const isDeleting = deleting === schema.name;
        return (
          <Box sx={{ display: "flex", justifyContent: "flex-end", gap: 0.5 }}>
            <Button
              size="small" variant="outlined"
              disabled={connLoading === schema.name}
              onClick={() => void handleToggleConnection(schema.name)}
            >
              {openedConn === schema.name ? "隠す" : "接続情報"}
            </Button>
            <Tooltip title="削除">
              <span>
                <IconButton size="small" color="error"
                  disabled={isDeleting}
                  onClick={() => void handleDelete(schema.name)}>
                  {isDeleting ? <CircularProgress size={14} /> : <DeleteOutlinedIcon fontSize="small" />}
                </IconButton>
              </span>
            </Tooltip>
          </Box>
        );
      }
    }
  ];

  return (
    <Box>
      <PageHeader
        title={databaseName}
        actions={<Button startIcon={<ArrowBackIcon />} onClick={onBack}>一覧に戻る</Button>}
      />

      {/* Summary */}
      <Box sx={{ display: "grid", gap: 1, gridTemplateColumns: { xs: "1fr 1fr", md: "repeat(5, minmax(0, 1fr))" }, mb: 3 }}>
        <Paper variant="outlined" sx={{ p: 1.5 }}>
          <Typography variant="caption" color="text.secondary">ステータス</Typography>
          <Box sx={{ mt: 0.5 }}>
            <StatusBadge variant={s.v} label={s.label} showSpinner={s.spin} />
          </Box>
        </Paper>
        <Paper variant="outlined" sx={{ p: 1.5 }}>
          <Typography variant="caption" color="text.secondary">種別</Typography>
          <Typography variant="body2" sx={{ mt: 0.5, fontWeight: 500 }}>
            {database ? (DB_TYPE_LABELS[database.type] ?? database.type) : "-"}
          </Typography>
        </Paper>
        <Paper variant="outlined" sx={{ p: 1.5 }}>
          <Typography variant="caption" color="text.secondary">バージョン</Typography>
          <Typography variant="body2" sx={{ mt: 0.5 }}>{database?.version ?? "-"}</Typography>
        </Paper>
        <Paper variant="outlined" sx={{ p: 1.5 }}>
          <Typography variant="caption" color="text.secondary">リソース</Typography>
          <Typography variant="body2" sx={{ mt: 0.5 }}>
            CPU {database?.cpu ?? "-"} / Mem {database?.memory ?? "-"}
          </Typography>
        </Paper>
        <Paper variant="outlined" sx={{ p: 1.5 }}>
          <Typography variant="caption" color="text.secondary">作成日時</Typography>
          <Typography variant="body2" sx={{ mt: 0.5 }}>{formatComputeTimestamp(database?.createdAt)}</Typography>
        </Paper>
      </Box>

      {/* Schemas */}
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", mb: 2 }}>
        <Typography variant="h6">スキーマ</Typography>
        <Button
          variant="contained" size="small" startIcon={<AddIcon />}
          disabled={!isManagedType || !isReady}
          onClick={() => setCreateOpen(true)}
        >
          追加
        </Button>
      </Box>

      {!isManagedType && (
        <Alert severity="info" sx={{ mb: 2 }}>この種別ではスキーマ管理は対応していません。</Alert>
      )}
      {isManagedType && !isReady && (
        <Alert severity="info" sx={{ mb: 2 }}>インスタンスが準備中です。Running になると操作できます。</Alert>
      )}
      {schemasError && (
        <Alert severity="error" onClose={() => setSchemasError("")} sx={{ mb: 2 }}>{schemasError}</Alert>
      )}

      {isManagedType && isReady && (
        <>
          <DataTable
            columns={columns}
            rows={schemas}
            rowKey={(schema) => schema.name}
            loading={schemasLoading}
            emptyMessage="まだスキーマはありません"
          />
          {schemas.map(schema => (
            <Collapse in={openedConn === schema.name && !!connInfo[schema.name]} key={`conn-${schema.name}`}>
              <Paper variant="outlined" sx={{ mt: 1, p: 2, bgcolor: "#fafafa" }}>
                <Typography variant="caption" color="text.secondary" sx={{ display: "block", mb: 1, fontWeight: 500 }}>
                  {schema.name} の接続情報
                </Typography>
                {connInfo[schema.name] && (
                  <Box sx={{ display: "grid", gap: 0.75 }}>
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
                        <Typography variant="caption" sx={{ fontFamily: monoFontFamily, wordBreak: "break-all", flex: 1 }}>{value}</Typography>
                        <IconButton size="small" onClick={() => copyToClipboard(value)}>
                          <ContentCopyIcon sx={{ fontSize: 14 }} />
                        </IconButton>
                      </Box>
                    ))}
                  </Box>
                )}
              </Paper>
            </Collapse>
          ))}
        </>
      )}

      <FormDialog
        open={createOpen}
        title="スキーマを追加"
        onClose={() => setCreateOpen(false)}
        onSubmit={handleCreate}
        submitting={submitting}
        submitDisabled={!newSchemaName.trim()}
        error={createError}
      >
        <TextField
          label="名前"
          value={newSchemaName}
          onChange={(e) => setNewSchemaName(e.target.value)}
          fullWidth autoFocus disabled={submitting}
          helperText="英数字とアンダースコアのみ・最大64文字"
        />
      </FormDialog>
    </Box>
  );
}
