import CheckCircleIcon from "@mui/icons-material/CheckCircle";
import ContentCopyIcon from "@mui/icons-material/ContentCopy";
import DeleteOutlinedIcon from "@mui/icons-material/DeleteOutlined";
import KeyIcon from "@mui/icons-material/Key";
import { alpha } from "@mui/material/styles";
import {
  Box, Button, Card, CardContent, CircularProgress, Collapse, Dialog, DialogContent,
  DialogTitle, IconButton, Paper, TextField, Tooltip, Typography
} from "@mui/material";
import { useState } from "react";
import type { Bucket, BucketCreateForm } from "../types";
import { formatComputeTimestamp } from "../utils";

type StorageSectionProps = {
  loading: boolean;
  buckets: Bucket[];
  deletingBucketName: string;
  onDeleteBucket: (name: string) => void;
  onCreateBucket: (form: BucketCreateForm) => Promise<void>;
  activeProjectId: string;
};

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
      const data = await response.json() as { endpoint: string; bucketName: string; accessKeyId: string; secretAccessKey: string };
      setCreds(prev => ({ ...prev, [name]: data }));
    } catch {
      setCredsOpen(null);
    } finally {
      setCredsLoading(false);
    }
  }

  function copyToClipboard(text: string) {
    void navigator.clipboard.writeText(text);
  }

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
                gridTemplateColumns: "42px minmax(0, 1fr) 100px 160px 88px",
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
                          gridTemplateColumns: "42px minmax(0, 1fr) 100px 160px 88px",
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
                          <Tooltip title="認証情報">
                            <IconButton
                              size="small"
                              disabled={!isReady}
                              onClick={() => void handleShowCreds(bucket.name)}
                              sx={{ border: "1px solid", borderColor: "divider" }}
                            >
                              <KeyIcon fontSize="small" />
                            </IconButton>
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
    </Box>
  );
}
