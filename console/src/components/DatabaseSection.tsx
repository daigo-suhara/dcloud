import ContentCopyIcon from "@mui/icons-material/ContentCopy";
import DeleteOutlinedIcon from "@mui/icons-material/DeleteOutlined";
import LinkIcon from "@mui/icons-material/Link";
import AddIcon from "@mui/icons-material/Add";
import {
  Box, Button, Collapse, IconButton, MenuItem, TextField, Tooltip, Typography, Paper, CircularProgress
} from "@mui/material";
import { useState } from "react";
import type { DatabaseCreateForm, DatabaseInstance } from "../types";
import { formatComputeTimestamp, formatStatus } from "../utils";
import { monoFontFamily } from "../theme";
import { PageHeader, DataTable, StatusBadge, FormDialog } from "./primitives";
import type { Column, StatusVariant } from "./primitives";

const DB_TYPES = [
  { value: "postgres", label: "PostgreSQL" },
  { value: "mysql", label: "MySQL" },
  { value: "redis", label: "Redis" },
];

const VERSIONS_BY_TYPE: Record<string, string[]> = {
  postgres: ["18", "17", "16", "15", "14", "12"],
  mysql:    ["8.4", "8.0", "5.7"],
  redis:    ["7", "6", "5"],
};

type ResourcePreset = { label: string; cpu: string; memory: string; storage: string };
const RESOURCE_PRESETS: Record<string, ResourcePreset> = {
  plan10gb:   { label: "10GB プラン  (1 core / 2 GiB / 10 GiB)",   cpu: "1000m", memory: "2Gi",  storage: "10Gi"   },
  plan30gb:   { label: "30GB プラン  (2 core / 4 GiB / 30 GiB)",   cpu: "2000m", memory: "4Gi",  storage: "30Gi"   },
  plan90gb:   { label: "90GB プラン  (4 core / 8 GiB / 90 GiB)",   cpu: "4000m", memory: "8Gi",  storage: "90Gi"   },
  plan240gb:  { label: "240GB プラン (8 core / 16 GiB / 240 GiB)", cpu: "8000m", memory: "16Gi", storage: "240Gi"  },
  plan500gb:  { label: "500GB プラン (8 core / 16 GiB / 500 GiB)", cpu: "8000m", memory: "16Gi", storage: "500Gi"  },
  plan1tb:    { label: "1TB プラン   (8 core / 16 GiB / 1 TiB)",   cpu: "8000m", memory: "16Gi", storage: "1024Gi" },
};

const initialForm: DatabaseCreateForm & { preset: string } = {
  name: "",
  type: "postgres",
  version: VERSIONS_BY_TYPE.postgres[0],
  preset: "plan10gb",
  cpu: RESOURCE_PRESETS.plan10gb.cpu,
  memory: RESOURCE_PRESETS.plan10gb.memory,
  storage: RESOURCE_PRESETS.plan10gb.storage,
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

function statusVariant(db: DatabaseInstance, isDeleting: boolean): { v: StatusVariant; label: string; spin: boolean } {
  if (isDeleting) return { v: "error", label: "削除中", spin: true };
  if (!db.ready) return { v: "progress", label: formatStatus(db.status, "準備中"), spin: true };
  return { v: "ready", label: formatStatus(db.status, "実行中"), spin: false };
}

function dbTypeLabel(type: string) {
  return DB_TYPES.find(d => d.value === type)?.label ?? type;
}

export function DatabaseSection({
  loading, databases, deletingDatabaseName,
  onDeleteDatabase, onCreateDatabase, onOpenDatabase, activeProjectId
}: DatabaseSectionProps) {
  const [createOpen, setCreateOpen] = useState(false);
  const [form, setForm] = useState(initialForm);
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
      const { preset: _preset, ...payload } = form;
      await onCreateDatabase(payload);
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

  const columns: Column<DatabaseInstance>[] = [
    {
      key: "name",
      header: "名前",
      render: (db) => (
        <Typography variant="body2" sx={{ fontWeight: 500, color: "primary.main" }}>{db.name}</Typography>
      )
    },
    { key: "type", header: "種別", width: 120, render: (db) => <Typography variant="body2">{dbTypeLabel(db.type)}</Typography> },
    {
      key: "version", header: "バージョン", width: 120,
      render: (db) => (
        <Typography variant="body2" color="text.secondary">
          {db.version}
        </Typography>
      )
    },
    {
      key: "status", header: "ステータス", width: 140,
      render: (db) => {
        const s = statusVariant(db, deletingDatabaseName === db.name);
        return <StatusBadge variant={s.v} label={s.label} showSpinner={s.spin} />;
      }
    },
    {
      key: "createdAt", header: "作成日時", width: 180,
      render: (db) => <Typography variant="caption" color="text.secondary">{formatComputeTimestamp(db.createdAt)}</Typography>
    },
    {
      key: "actions", header: "", width: 96, align: "right",
      render: (db) => {
        const isDeleting = deletingDatabaseName === db.name;
        return (
          <Box sx={{ display: "flex", justifyContent: "flex-end", gap: 0.5 }}>
            <Tooltip title="接続情報">
              <span>
                <IconButton size="small" disabled={!db.ready} onClick={() => void handleShowConnection(db.name)}>
                  <LinkIcon fontSize="small" />
                </IconButton>
              </span>
            </Tooltip>
            <Tooltip title="削除">
              <span>
                <IconButton size="small" color="error" disabled={isDeleting} onClick={() => onDeleteDatabase(db.name)}>
                  <DeleteOutlinedIcon fontSize="small" />
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
        title="データベース"
        actions={
          <Button variant="contained" startIcon={<AddIcon />} onClick={() => setCreateOpen(true)}>
            作成
          </Button>
        }
      />

      <DataTable
        columns={columns}
        rows={databases}
        rowKey={(db) => db.name}
        onRowClick={(db) => onOpenDatabase(db.name)}
        loading={loading}
        emptyMessage="まだデータベースはありません"
      />

      {databases.map(db => (
        <Collapse in={connOpen === db.name} key={`conn-${db.name}`}>
          <Paper variant="outlined" sx={{ mt: 1, p: 2, bgcolor: "#fafafa" }}>
            <Typography variant="caption" color="text.secondary" sx={{ display: "block", mb: 1, fontWeight: 500 }}>
              {db.name} の接続情報
            </Typography>
            {connLoading && !connInfo[db.name] ? (
              <CircularProgress size={16} />
            ) : connInfo[db.name] && (
              <Box sx={{ display: "grid", gap: 0.75 }}>
                {[
                  { label: "接続文字列", value: connInfo[db.name].connectionString },
                  { label: "Host", value: connInfo[db.name].host },
                  { label: "Port", value: connInfo[db.name].port },
                  ...(connInfo[db.name].username ? [{ label: "Username", value: connInfo[db.name].username }] : []),
                  { label: "Password", value: connInfo[db.name].password },
                ].map(({ label, value }) => (
                  <Box key={label} sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                    <Typography variant="caption" color="text.secondary" sx={{ minWidth: 88 }}>{label}</Typography>
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

      <FormDialog
        open={createOpen}
        title="データベースを作成"
        onClose={() => setCreateOpen(false)}
        onSubmit={handleCreate}
        submitLabel="作成"
        submitting={submitting}
        submitDisabled={!form.name.trim()}
        error={error}
      >
        <TextField
          label="名前"
          value={form.name}
          onChange={(e) => setForm(f => ({ ...f, name: e.target.value }))}
          fullWidth
          helperText="小文字・数字・ハイフンのみ"
          disabled={submitting}
        />
        <TextField
          select label="種別" value={form.type}
          onChange={(e) => setForm(f => ({ ...f, type: e.target.value, version: VERSIONS_BY_TYPE[e.target.value]?.[0] ?? "" }))}
          fullWidth disabled={submitting}
        >
          {DB_TYPES.map(opt => <MenuItem key={opt.value} value={opt.value}>{opt.label}</MenuItem>)}
        </TextField>
        <TextField
          select label="バージョン" value={form.version}
          onChange={(e) => setForm(f => ({ ...f, version: e.target.value }))}
          fullWidth disabled={submitting}
        >
          {(VERSIONS_BY_TYPE[form.type] ?? []).map(v => (
            <MenuItem key={v} value={v}>{v}</MenuItem>
          ))}
        </TextField>
        <TextField
          select label="リソース" value={form.preset}
          onChange={(e) => {
            const p = RESOURCE_PRESETS[e.target.value];
            setForm(f => ({ ...f, preset: e.target.value, cpu: p.cpu, memory: p.memory, storage: p.storage }));
          }}
          fullWidth disabled={submitting}
        >
          {Object.entries(RESOURCE_PRESETS).map(([k, p]) => (
            <MenuItem key={k} value={k}>{p.label}</MenuItem>
          ))}
        </TextField>
      </FormDialog>
    </Box>
  );
}
