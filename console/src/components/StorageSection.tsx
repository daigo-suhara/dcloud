import AddIcon from "@mui/icons-material/Add";
import CloudUploadIcon from "@mui/icons-material/CloudUpload";
import ContentCopyIcon from "@mui/icons-material/ContentCopy";
import DeleteOutlinedIcon from "@mui/icons-material/DeleteOutlined";
import DownloadIcon from "@mui/icons-material/Download";
import FolderOpenIcon from "@mui/icons-material/FolderOpen";
import FolderIcon from "@mui/icons-material/Folder";
import InsertDriveFileIcon from "@mui/icons-material/InsertDriveFile";
import KeyIcon from "@mui/icons-material/Key";
import NavigateNextIcon from "@mui/icons-material/NavigateNext";
import {
  Box, Breadcrumbs, Button, CircularProgress, Collapse, Dialog, DialogContent, DialogTitle,
  Divider, IconButton, Link, Paper, TextField, Tooltip, Typography
} from "@mui/material";
import { useRef, useState } from "react";
import type { Bucket, BucketCreateForm } from "../types";
import { formatComputeTimestamp } from "../utils";
import { monoFontFamily } from "../theme";
import { PageHeader, DataTable, StatusBadge, FormDialog } from "./primitives";
import type { Column, StatusVariant } from "./primitives";

type S3Object = { key: string; size: number; lastModified: string };
type Creds = { endpoint: string; bucketName: string; accessKeyId: string; secretAccessKey: string };

type StorageSectionProps = {
  loading: boolean;
  buckets: Bucket[];
  deletingBucketName: string;
  onDeleteBucket: (name: string) => void;
  onCreateBucket: (form: BucketCreateForm) => Promise<void>;
  activeProjectId: string;
};

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)} GB`;
}

function statusOf(b: Bucket, isDeleting: boolean): { v: StatusVariant; label: string; spin: boolean } {
  if (isDeleting) return { v: "error", label: "Deleting", spin: true };
  if (!b.ready) return { v: "progress", label: b.status || "Pending", spin: true };
  return { v: "ready", label: b.status || "Bound", spin: false };
}

export function StorageSection({
  loading, buckets, deletingBucketName, onDeleteBucket, onCreateBucket, activeProjectId
}: StorageSectionProps) {
  const [createOpen, setCreateOpen] = useState(false);
  const [form, setForm] = useState<BucketCreateForm>({ name: "" });
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");
  const [credsOpen, setCredsOpen] = useState<string | null>(null);
  const [creds, setCreds] = useState<Record<string, Creds>>({});
  const [credsLoading, setCredsLoading] = useState(false);

  const [browseOpen, setBrowseOpen] = useState<string | null>(null);
  const [browsePrefix, setBrowsePrefix] = useState("");
  const [objects, setObjects] = useState<S3Object[]>([]);
  const [prefixes, setPrefixes] = useState<string[]>([]);
  const [objectsLoading, setObjectsLoading] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [uploadError, setUploadError] = useState("");
  const [deletingKey, setDeletingKey] = useState("");
  const fileInputRef = useRef<HTMLInputElement>(null);

  async function handleCreate() {
    if (!form.name.trim()) return;
    setSubmitting(true);
    setError("");
    try {
      await onCreateBucket(form);
      setForm({ name: "" });
      setCreateOpen(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : "作成に失敗しました");
    } finally {
      setSubmitting(false);
    }
  }

  async function handleShowCreds(name: string) {
    if (creds[name]) { setCredsOpen(credsOpen === name ? null : name); return; }
    setCredsLoading(true);
    setCredsOpen(name);
    try {
      const response = await fetch(`/api/v1/storage/${encodeURIComponent(name)}/credentials`, {
        credentials: "include",
        headers: { "X-DCP-Project": activeProjectId }
      });
      if (!response.ok) throw new Error();
      const data = (await response.json()) as Creds;
      setCreds(prev => ({ ...prev, [name]: data }));
    } catch { setCredsOpen(null); }
    finally { setCredsLoading(false); }
  }

  async function loadObjects(bucketName: string, prefix: string) {
    setObjectsLoading(true);
    try {
      const res = await fetch(`/api/v1/storage/${encodeURIComponent(bucketName)}/objects?prefix=${encodeURIComponent(prefix)}`, {
        credentials: "include", headers: { "X-DCP-Project": activeProjectId }
      });
      if (!res.ok) throw new Error();
      const data = (await res.json()) as { objects: S3Object[]; prefixes: string[] };
      setObjects(data.objects ?? []);
      setPrefixes(data.prefixes ?? []);
    } catch { setObjects([]); setPrefixes([]); }
    finally { setObjectsLoading(false); }
  }

  function handleOpenBrowse(bucketName: string) {
    setBrowseOpen(bucketName);
    setBrowsePrefix("");
    setUploadError("");
    void loadObjects(bucketName, "");
  }
  function handleNavigate(prefix: string) {
    setBrowsePrefix(prefix);
    if (browseOpen) void loadObjects(browseOpen, prefix);
  }
  function handleCloseBrowse() {
    setBrowseOpen(null); setBrowsePrefix(""); setObjects([]); setPrefixes([]); setUploadError("");
  }

  async function handleUpload(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file || !browseOpen) return;
    setUploading(true); setUploadError("");
    const formData = new FormData();
    formData.append("file", file);
    try {
      const res = await fetch(`/api/v1/storage/${encodeURIComponent(browseOpen)}/objects?prefix=${encodeURIComponent(browsePrefix)}`, {
        method: "POST", credentials: "include",
        headers: { "X-DCP-Project": activeProjectId }, body: formData,
      });
      if (!res.ok) {
        const data = (await res.json().catch(() => ({}))) as { detail?: string };
        throw new Error(data.detail ?? "アップロードに失敗しました");
      }
      void loadObjects(browseOpen, browsePrefix);
    } catch (err) {
      setUploadError(err instanceof Error ? err.message : "アップロードに失敗しました");
    } finally { setUploading(false); e.target.value = ""; }
  }

  async function handleDeleteObject(key: string) {
    if (!browseOpen) return;
    setDeletingKey(key);
    try {
      await fetch(`/api/v1/storage/${encodeURIComponent(browseOpen)}/objects?key=${encodeURIComponent(key)}`, {
        method: "DELETE", credentials: "include",
        headers: { "X-DCP-Project": activeProjectId }
      });
      void loadObjects(browseOpen, browsePrefix);
    } finally { setDeletingKey(""); }
  }

  function handleDownload(key: string) {
    if (!browseOpen) return;
    const filename = key.split("/").pop() ?? "download";
    const url = `/api/v1/storage/${encodeURIComponent(browseOpen)}/download?key=${encodeURIComponent(key)}&project=${encodeURIComponent(activeProjectId)}`;
    const a = document.createElement("a");
    a.href = url; a.download = filename;
    document.body.appendChild(a); a.click(); document.body.removeChild(a);
  }

  function copyToClipboard(text: string) { void navigator.clipboard.writeText(text); }

  const breadcrumbParts = browsePrefix ? browsePrefix.split("/").filter(Boolean) : [];

  const columns: Column<Bucket>[] = [
    {
      key: "name", header: "名前",
      render: (b) => (
        <Typography variant="body2" sx={{ fontWeight: 500, color: "primary.main" }}>{b.name}</Typography>
      )
    },
    {
      key: "status", header: "ステータス", width: 140,
      render: (b) => {
        const s = statusOf(b, deletingBucketName === b.name);
        return <StatusBadge variant={s.v} label={s.label} showSpinner={s.spin} />;
      }
    },
    {
      key: "createdAt", header: "作成日時", width: 180,
      render: (b) => (
        <Typography variant="caption" color="text.secondary">{formatComputeTimestamp(b.createdAt)}</Typography>
      )
    },
    {
      key: "actions", header: "", width: 140, align: "right",
      render: (b) => {
        const isDeleting = deletingBucketName === b.name;
        return (
          <Box sx={{ display: "flex", justifyContent: "flex-end", gap: 0.5 }}>
            <Tooltip title="ファイル">
              <span>
                <IconButton size="small" disabled={!b.ready} onClick={() => handleOpenBrowse(b.name)}>
                  <FolderOpenIcon fontSize="small" />
                </IconButton>
              </span>
            </Tooltip>
            <Tooltip title="認証情報">
              <span>
                <IconButton size="small" disabled={!b.ready} onClick={() => void handleShowCreds(b.name)}>
                  <KeyIcon fontSize="small" />
                </IconButton>
              </span>
            </Tooltip>
            <Tooltip title="削除">
              <span>
                <IconButton size="small" color="error" disabled={isDeleting} onClick={() => onDeleteBucket(b.name)}>
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
        title="オブジェクトストレージ"
        actions={
          <Button variant="contained" startIcon={<AddIcon />} onClick={() => setCreateOpen(true)}>
            作成
          </Button>
        }
      />

      <DataTable
        columns={columns}
        rows={buckets}
        rowKey={(b) => b.name}
        loading={loading}
        emptyMessage="まだバケットはありません"
      />

      {buckets.map(b => (
        <Collapse in={credsOpen === b.name} key={`creds-${b.name}`}>
          <Paper variant="outlined" sx={{ mt: 1, p: 2, bgcolor: "#fafafa" }}>
            <Typography variant="caption" color="text.secondary" sx={{ display: "block", mb: 1, fontWeight: 500 }}>
              {b.name} の認証情報
            </Typography>
            {credsLoading && !creds[b.name] ? (
              <CircularProgress size={16} />
            ) : creds[b.name] && (
              <Box sx={{ display: "grid", gap: 0.75 }}>
                {[
                  { label: "Endpoint", value: creds[b.name].endpoint },
                  { label: "Bucket Name", value: creds[b.name].bucketName },
                  { label: "Access Key ID", value: creds[b.name].accessKeyId },
                  { label: "Secret Access Key", value: creds[b.name].secretAccessKey },
                ].map(({ label, value }) => (
                  <Box key={label} sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                    <Typography variant="caption" color="text.secondary" sx={{ minWidth: 140 }}>{label}</Typography>
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
        title="バケットを作成"
        onClose={() => setCreateOpen(false)}
        onSubmit={handleCreate}
        submitting={submitting}
        submitDisabled={!form.name.trim()}
        error={error}
      >
        <TextField
          label="バケット名"
          value={form.name}
          onChange={(e) => setForm({ name: e.target.value })}
          fullWidth
          helperText="小文字・数字・ハイフンのみ (最大63文字)"
          disabled={submitting}
        />
      </FormDialog>

      {/* File Browser */}
      <Dialog open={browseOpen !== null} onClose={handleCloseBrowse} fullWidth maxWidth="md" slotProps={{ paper: { sx: { height: "80vh" } } }}>
        <DialogTitle sx={{ pb: 1, display: "flex", alignItems: "center", gap: 1 }}>
          <FolderOpenIcon color="primary" fontSize="small" />
          <Typography variant="h6">{browseOpen}</Typography>
        </DialogTitle>
        <Divider />
        <Box sx={{ px: 2, py: 1, display: "flex", alignItems: "center", justifyContent: "space-between", gap: 1 }}>
          <Breadcrumbs separator={<NavigateNextIcon fontSize="small" />} sx={{ flex: 1 }}>
            <Link
              component="button" underline="hover"
              color={browsePrefix === "" ? "text.primary" : "inherit"}
              sx={{ cursor: "pointer", fontSize: 13 }}
              onClick={() => handleNavigate("")}
            >/</Link>
            {breadcrumbParts.map((part, i) => {
              const fullPrefix = breadcrumbParts.slice(0, i + 1).join("/") + "/";
              const isLast = i === breadcrumbParts.length - 1;
              return (
                <Link
                  key={fullPrefix} component="button" underline="hover"
                  color={isLast ? "text.primary" : "inherit"}
                  sx={{ cursor: "pointer", fontSize: 13 }}
                  onClick={() => handleNavigate(fullPrefix)}
                >{part}</Link>
              );
            })}
          </Breadcrumbs>
          <input ref={fileInputRef} type="file" style={{ display: "none" }} onChange={(e) => void handleUpload(e)} />
          <Button
            variant="contained" size="small"
            startIcon={uploading ? <CircularProgress size={14} sx={{ color: "inherit" }} /> : <CloudUploadIcon />}
            disabled={uploading}
            onClick={() => fileInputRef.current?.click()}
          >アップロード</Button>
        </Box>
        {uploadError && <Typography color="error" variant="caption" sx={{ px: 2 }}>{uploadError}</Typography>}
        <Divider />
        <DialogContent sx={{ p: 0, overflow: "auto" }}>
          {objectsLoading ? (
            <Box sx={{ display: "grid", placeItems: "center", height: 120 }}>
              <CircularProgress size={24} />
            </Box>
          ) : prefixes.length === 0 && objects.length === 0 ? (
            <Box sx={{ display: "grid", placeItems: "center", height: 120 }}>
              <Typography color="text.secondary" variant="body2">ファイルがありません</Typography>
            </Box>
          ) : (
            <Box>
              <Box sx={{ display: "grid", gridTemplateColumns: "36px minmax(0,1fr) 80px 140px 76px", alignItems: "center", minHeight: 32, px: 2, bgcolor: "#fafafa", borderBottom: "1px solid", borderColor: "divider", color: "text.secondary", fontSize: 12, fontWeight: 500 }}>
                <Box />
                <Box>名前</Box>
                <Box>サイズ</Box>
                <Box>更新日時</Box>
                <Box sx={{ textAlign: "right" }}>操作</Box>
              </Box>
              {prefixes.map((p) => {
                const folderName = p.slice(browsePrefix.length);
                return (
                  <Box
                    key={p}
                    onClick={() => handleNavigate(p)}
                    sx={{
                      display: "grid",
                      gridTemplateColumns: "36px minmax(0,1fr) 80px 140px 76px",
                      alignItems: "center", minHeight: 36, px: 2,
                      cursor: "pointer",
                      borderBottom: "1px solid", borderColor: "divider",
                      "&:hover": { bgcolor: "rgba(60,64,67,0.04)" }
                    }}
                  >
                    <FolderIcon sx={{ fontSize: 18, color: "primary.main" }} />
                    <Typography variant="body2" sx={{ fontWeight: 500 }}>{folderName}</Typography>
                    <Box /><Box /><Box />
                  </Box>
                );
              })}
              {objects.map((obj) => {
                const filename = obj.key.slice(browsePrefix.length);
                const isDeleting = deletingKey === obj.key;
                return (
                  <Box
                    key={obj.key}
                    sx={{
                      display: "grid",
                      gridTemplateColumns: "36px minmax(0,1fr) 80px 140px 76px",
                      alignItems: "center", minHeight: 36, px: 2,
                      borderBottom: "1px solid", borderColor: "divider",
                      "&:hover": { bgcolor: "rgba(60,64,67,0.04)" }
                    }}
                  >
                    <InsertDriveFileIcon sx={{ fontSize: 18, color: "text.disabled" }} />
                    <Typography variant="body2" sx={{ wordBreak: "break-all" }}>{filename}</Typography>
                    <Typography variant="caption" color="text.secondary">{formatFileSize(obj.size)}</Typography>
                    <Typography variant="caption" color="text.secondary">{formatComputeTimestamp(obj.lastModified)}</Typography>
                    <Box sx={{ display: "flex", justifyContent: "flex-end", gap: 0.25 }}>
                      <Tooltip title="ダウンロード">
                        <IconButton size="small" onClick={() => void handleDownload(obj.key)}>
                          <DownloadIcon sx={{ fontSize: 16 }} />
                        </IconButton>
                      </Tooltip>
                      <Tooltip title="削除">
                        <IconButton size="small" color="error" disabled={isDeleting} onClick={() => void handleDeleteObject(obj.key)}>
                          {isDeleting ? <CircularProgress size={14} /> : <DeleteOutlinedIcon sx={{ fontSize: 16 }} />}
                        </IconButton>
                      </Tooltip>
                    </Box>
                  </Box>
                );
              })}
            </Box>
          )}
        </DialogContent>
      </Dialog>
    </Box>
  );
}
