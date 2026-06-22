import CheckCircleIcon from "@mui/icons-material/CheckCircle";
import CloudUploadIcon from "@mui/icons-material/CloudUpload";
import ContentCopyIcon from "@mui/icons-material/ContentCopy";
import DeleteOutlinedIcon from "@mui/icons-material/DeleteOutlined";
import DownloadIcon from "@mui/icons-material/Download";
import FolderOpenIcon from "@mui/icons-material/FolderOpen";
import FolderIcon from "@mui/icons-material/Folder";
import InsertDriveFileIcon from "@mui/icons-material/InsertDriveFile";
import KeyIcon from "@mui/icons-material/Key";
import NavigateNextIcon from "@mui/icons-material/NavigateNext";
import { alpha } from "@mui/material/styles";
import {
  Box, Breadcrumbs, Button, Card, CardContent, CircularProgress, Collapse, Dialog,
  DialogContent, DialogTitle, Divider, IconButton, Link, Paper, TextField, Tooltip, Typography
} from "@mui/material";
import { useRef, useState } from "react";
import type { Bucket, BucketCreateForm } from "../types";
import { formatComputeTimestamp } from "../utils";

type S3Object = {
  key: string;
  size: number;
  lastModified: string;
};

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

export function StorageSection({
  loading,
  buckets,
  deletingBucketName,
  onDeleteBucket,
  onCreateBucket,
  activeProjectId
}: StorageSectionProps) {
  const [createOpen, setCreateOpen] = useState(false);
  const [form, setForm] = useState<BucketCreateForm>({ name: "" });
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");
  const [credsOpen, setCredsOpen] = useState<string | null>(null);
  const [creds, setCreds] = useState<Record<string, { endpoint: string; bucketName: string; accessKeyId: string; secretAccessKey: string }>>({});
  const [credsLoading, setCredsLoading] = useState(false);

  // file browser state
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
    if (creds[name]) {
      setCredsOpen(name);
      return;
    }
    setCredsLoading(true);
    setCredsOpen(name);
    try {
      const response = await fetch(`/api/v1/storage/${encodeURIComponent(name)}/credentials`, {
        credentials: "include",
        headers: { "X-DCP-Project": activeProjectId }
      });
      if (!response.ok) throw new Error("認証情報の取得に失敗しました");
      const data = (await response.json()) as { endpoint: string; bucketName: string; accessKeyId: string; secretAccessKey: string };
      setCreds(prev => ({ ...prev, [name]: data }));
    } catch {
      setCredsOpen(null);
    } finally {
      setCredsLoading(false);
    }
  }

  async function loadObjects(bucketName: string, prefix: string) {
    setObjectsLoading(true);
    try {
      const res = await fetch(`/api/v1/storage/${encodeURIComponent(bucketName)}/objects?prefix=${encodeURIComponent(prefix)}`, {
        credentials: "include",
        headers: { "X-DCP-Project": activeProjectId }
      });
      if (!res.ok) throw new Error();
      const data = (await res.json()) as { objects: S3Object[]; prefixes: string[] };
      setObjects(data.objects ?? []);
      setPrefixes(data.prefixes ?? []);
    } catch {
      setObjects([]);
      setPrefixes([]);
    } finally {
      setObjectsLoading(false);
    }
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
    setBrowseOpen(null);
    setBrowsePrefix("");
    setObjects([]);
    setPrefixes([]);
    setUploadError("");
  }

  async function handleUpload(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file || !browseOpen) return;
    setUploading(true);
    setUploadError("");
    const formData = new FormData();
    formData.append("file", file);
    try {
      const res = await fetch(`/api/v1/storage/${encodeURIComponent(browseOpen)}/objects?prefix=${encodeURIComponent(browsePrefix)}`, {
        method: "POST",
        credentials: "include",
        headers: { "X-DCP-Project": activeProjectId },
        body: formData,
      });
      if (!res.ok) {
        const data = (await res.json().catch(() => ({}))) as { detail?: string };
        throw new Error(data.detail ?? "アップロードに失敗しました");
      }
      void loadObjects(browseOpen, browsePrefix);
    } catch (err) {
      setUploadError(err instanceof Error ? err.message : "アップロードに失敗しました");
    } finally {
      setUploading(false);
      e.target.value = "";
    }
  }

  async function handleDeleteObject(key: string) {
    if (!browseOpen) return;
    setDeletingKey(key);
    try {
      await fetch(`/api/v1/storage/${encodeURIComponent(browseOpen)}/objects?key=${encodeURIComponent(key)}`, {
        method: "DELETE",
        credentials: "include",
        headers: { "X-DCP-Project": activeProjectId }
      });
      void loadObjects(browseOpen, browsePrefix);
    } finally {
      setDeletingKey("");
    }
  }

  function handleDownload(key: string) {
    if (!browseOpen) return;
    const filename = key.split("/").pop() ?? "download";
    const url = `/api/v1/storage/${encodeURIComponent(browseOpen)}/download?key=${encodeURIComponent(key)}&project=${encodeURIComponent(activeProjectId)}`;
    const a = document.createElement("a");
    a.href = url;
    a.download = filename;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
  }

  function copyToClipboard(text: string) {
    void navigator.clipboard.writeText(text);
  }

  const breadcrumbParts = browsePrefix ? browsePrefix.split("/").filter(Boolean) : [];

  return (
    <Box sx={{ display: "grid", gap: 3 }}>
      <Card variant="outlined" sx={{ borderRadius: 2 }}>
        <CardContent sx={{ p: 3, display: "grid", gap: 2 }}>
          <Box sx={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", gap: 2, flexWrap: "wrap" }}>
            <Box sx={{ display: "grid", gap: 0.75 }}>
              <Typography variant="h5" sx={{ fontWeight: 700 }}>
                オブジェクトストレージ
              </Typography>
              <Typography variant="body2" color="text.secondary">
                Ceph (Rook-Ceph) によるS3互換バケット
              </Typography>
            </Box>
            <Button variant="contained" onClick={() => setCreateOpen(true)}>
              バケットを作成
            </Button>
          </Box>

          <Box sx={{ display: "grid", gap: 0 }}>
            <Box
              sx={{
                display: { xs: "none", sm: "grid" },
                gridTemplateColumns: "42px minmax(0, 1fr) 100px 160px 120px",
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
              <Box>ステータス</Box>
              <Box>作成日時</Box>
              <Box sx={{ textAlign: "right" }}>操作</Box>
            </Box>

            <Box sx={{ borderTop: "1px solid rgba(148, 163, 184, 0.18)" }}>
              {buckets.length > 0 ? (
                buckets.map((bucket) => {
                  const isDeleting = deletingBucketName === bucket.name;
                  const isReady = bucket.ready;
                  const statusIcon = isDeleting || !isReady
                    ? <CircularProgress size={14} thickness={5.5} sx={{ color: "inherit" }} />
                    : <CheckCircleIcon fontSize="small" />;
                  const statusBgColor = isDeleting ? alpha("#dc2626", 0.12) : isReady ? "transparent" : alpha("#2563eb", 0.12);
                  const statusTextColor = isDeleting ? "error.main" : isReady ? "success.main" : "primary.main";

                  return (
                    <Box key={bucket.name}>
                      <Paper
                        variant="outlined"
                        sx={{
                          display: "grid",
                          gridTemplateColumns: "42px minmax(0, 1fr) 100px 160px 120px",
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
                          <Typography variant="body2" sx={{ fontWeight: 700, wordBreak: "break-all" }}>
                            {bucket.name}
                          </Typography>
                        </Box>
                        <Box>
                          <Typography variant="caption" color="text.secondary">
                            {bucket.status || (isReady ? "Bound" : "Pending")}
                          </Typography>
                        </Box>
                        <Box>
                          <Typography variant="caption" color="text.secondary" sx={{ whiteSpace: "nowrap" }}>
                            {formatComputeTimestamp(bucket.createdAt)}
                          </Typography>
                        </Box>
                        <Box sx={{ display: "flex", justifyContent: "flex-end", gap: 0.5, pr: 0.5 }}>
                          <Tooltip title="ファイル">
                            <span>
                              <IconButton
                                size="small"
                                disabled={!isReady}
                                onClick={() => handleOpenBrowse(bucket.name)}
                                sx={{ border: "1px solid", borderColor: "divider" }}
                              >
                                <FolderOpenIcon fontSize="small" />
                              </IconButton>
                            </span>
                          </Tooltip>
                          <Tooltip title="認証情報">
                            <span>
                              <IconButton
                                size="small"
                                disabled={!isReady}
                                onClick={() => void handleShowCreds(bucket.name)}
                                sx={{ border: "1px solid", borderColor: "divider" }}
                              >
                                <KeyIcon fontSize="small" />
                              </IconButton>
                            </span>
                          </Tooltip>
                          <Tooltip title="削除">
                            <span>
                              <IconButton
                                color="error"
                                disabled={isDeleting}
                                onClick={() => onDeleteBucket(bucket.name)}
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

                      <Collapse in={credsOpen === bucket.name}>
                        <Box sx={{ px: 2, py: 1.5, bgcolor: alpha("#1e293b", 0.04), borderBottom: "1px solid rgba(148,163,184,0.18)" }}>
                          {credsLoading && !creds[bucket.name] ? (
                            <CircularProgress size={16} />
                          ) : creds[bucket.name] ? (
                            <Box sx={{ display: "grid", gap: 1 }}>
                              {[
                                { label: "Endpoint", value: creds[bucket.name].endpoint },
                                { label: "Bucket Name", value: creds[bucket.name].bucketName },
                                { label: "Access Key ID", value: creds[bucket.name].accessKeyId },
                                { label: "Secret Access Key", value: creds[bucket.name].secretAccessKey },
                              ].map(({ label, value }) => (
                                <Box key={label} sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                                  <Typography variant="caption" color="text.secondary" sx={{ minWidth: 140 }}>{label}</Typography>
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
                  <Typography color="text.secondary">{loading ? "読み込み中..." : "まだバケットはありません。"}</Typography>
                </Paper>
              )}
            </Box>
          </Box>
        </CardContent>
      </Card>

      {/* Create Dialog */}
      <Dialog open={createOpen} onClose={() => !submitting && setCreateOpen(false)} fullWidth maxWidth="xs">
        <DialogTitle>バケットを作成</DialogTitle>
        <DialogContent sx={{ display: "grid", gap: 2, pt: "8px !important" }}>
          {error && <Typography color="error" variant="body2">{error}</Typography>}
          <TextField
            label="バケット名"
            value={form.name}
            onChange={(e) => setForm({ name: e.target.value })}
            size="small"
            fullWidth
            helperText="小文字・数字・ハイフンのみ (最大63文字)"
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

      {/* File Browser Dialog */}
      <Dialog open={browseOpen !== null} onClose={handleCloseBrowse} fullWidth maxWidth="md" slotProps={{ paper: { sx: { height: "80vh" } } }}>
        <DialogTitle sx={{ pb: 1 }}>
          <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
            <FolderOpenIcon color="primary" />
            <Typography variant="h6" sx={{ fontWeight: 700 }}>{browseOpen}</Typography>
          </Box>
        </DialogTitle>
        <Divider />
        <Box sx={{ px: 2, py: 1, display: "flex", alignItems: "center", justifyContent: "space-between", gap: 1 }}>
          <Breadcrumbs separator={<NavigateNextIcon fontSize="small" />} sx={{ flex: 1 }}>
            <Link
              component="button"
              underline="hover"
              color={browsePrefix === "" ? "text.primary" : "inherit"}
              sx={{ cursor: "pointer", fontSize: 13 }}
              onClick={() => handleNavigate("")}
            >
              /
            </Link>
            {breadcrumbParts.map((part, i) => {
              const fullPrefix = breadcrumbParts.slice(0, i + 1).join("/") + "/";
              const isLast = i === breadcrumbParts.length - 1;
              return (
                <Link
                  key={fullPrefix}
                  component="button"
                  underline="hover"
                  color={isLast ? "text.primary" : "inherit"}
                  sx={{ cursor: "pointer", fontSize: 13 }}
                  onClick={() => handleNavigate(fullPrefix)}
                >
                  {part}
                </Link>
              );
            })}
          </Breadcrumbs>
          <Box sx={{ display: "flex", gap: 1 }}>
            <input ref={fileInputRef} type="file" style={{ display: "none" }} onChange={(e) => void handleUpload(e)} />
            <Button
              variant="contained"
              size="small"
              startIcon={uploading ? <CircularProgress size={14} sx={{ color: "inherit" }} /> : <CloudUploadIcon />}
              disabled={uploading}
              onClick={() => fileInputRef.current?.click()}
            >
              アップロード
            </Button>
          </Box>
        </Box>
        {uploadError && (
          <Typography color="error" variant="caption" sx={{ px: 2 }}>{uploadError}</Typography>
        )}
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
              {/* Header */}
              <Box sx={{ display: "grid", gridTemplateColumns: "36px minmax(0,1fr) 80px 140px 76px", alignItems: "center", minHeight: 36, px: 1, bgcolor: alpha("#0f172a", 0.03), borderBottom: "1px solid rgba(148,163,184,0.18)", color: "text.secondary", fontSize: 11, fontWeight: 700 }}>
                <Box />
                <Box>名前</Box>
                <Box>サイズ</Box>
                <Box>更新日時</Box>
                <Box sx={{ textAlign: "right" }}>操作</Box>
              </Box>
              {/* Folders */}
              {prefixes.map((p) => {
                const folderName = p.slice(browsePrefix.length);
                return (
                  <Box
                    key={p}
                    onClick={() => handleNavigate(p)}
                    sx={{
                      display: "grid",
                      gridTemplateColumns: "36px minmax(0,1fr) 80px 140px 76px",
                      alignItems: "center",
                      minHeight: 40,
                      px: 1,
                      cursor: "pointer",
                      borderBottom: "1px solid rgba(148,163,184,0.1)",
                      "&:hover": { bgcolor: alpha("#2563eb", 0.04) }
                    }}
                  >
                    <Box sx={{ display: "grid", placeItems: "center" }}>
                      <FolderIcon sx={{ fontSize: 18, color: "primary.main" }} />
                    </Box>
                    <Typography variant="body2" sx={{ fontWeight: 600 }}>{folderName}</Typography>
                    <Box />
                    <Box />
                    <Box />
                  </Box>
                );
              })}
              {/* Files */}
              {objects.map((obj) => {
                const filename = obj.key.slice(browsePrefix.length);
                const isDeleting = deletingKey === obj.key;
                return (
                  <Box
                    key={obj.key}
                    sx={{
                      display: "grid",
                      gridTemplateColumns: "36px minmax(0,1fr) 80px 140px 76px",
                      alignItems: "center",
                      minHeight: 40,
                      px: 1,
                      borderBottom: "1px solid rgba(148,163,184,0.1)",
                      "&:hover": { bgcolor: alpha("#0f172a", 0.02) }
                    }}
                  >
                    <Box sx={{ display: "grid", placeItems: "center" }}>
                      <InsertDriveFileIcon sx={{ fontSize: 18, color: "text.disabled" }} />
                    </Box>
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
