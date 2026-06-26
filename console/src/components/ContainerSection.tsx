import { alpha } from "@mui/material/styles";
import ArrowBackIcon from "@mui/icons-material/ArrowBack";
import CheckCircleIcon from "@mui/icons-material/CheckCircle";
import CloudUploadOutlinedIcon from "@mui/icons-material/CloudUploadOutlined";
import ErrorOutlinedIcon from "@mui/icons-material/ErrorOutlined";
import GitHubIcon from "@mui/icons-material/GitHub";
import DeleteOutlinedIcon from "@mui/icons-material/DeleteOutlined";
import { Box, Button, Card, CardContent, Chip, CircularProgress, IconButton, Paper, TextField, Tooltip, Typography } from "@mui/material";
import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import { Link as RouterLink } from "react-router-dom";
import type { DeployedService, UpdateForm } from "../types";
import { EnvVarEditor } from "./EnvVarEditor";
import { ContainerLogsViewer } from "./ContainerLogsViewer";
import { actionLinkButtonSx } from "../theme";
import { formatServiceStatus, formatServiceTimestamp, getServiceStatus } from "../utils";

type ContainerSectionProps = {
  loading: boolean;
  deletingServiceName: string;
  updatingServiceName: string;
  onBackToList: () => void;
  onDeployClick: () => void;
  onDeleteService: (name: string) => void;
  onOpenService: (name: string) => void;
  onRepoConnectClick: () => void;
  onSetDomain: (name: string, domain: string) => Promise<void>;
  onUpdateService: (name: string, form: UpdateForm) => Promise<void>;
  selectedService: DeployedService | null;
  selectedStatus: ReturnType<typeof getServiceStatus> | null;
  containers: DeployedService[];
  activeProjectId: string;
};

export function ContainerSection({
  loading,
  deletingServiceName,
  updatingServiceName,
  onBackToList,
  onDeployClick,
  onDeleteService,
  onOpenService,
  onRepoConnectClick,
  onSetDomain,
  onUpdateService,
  selectedService,
  selectedStatus,
  containers,
  activeProjectId
}: ContainerSectionProps) {
  const [domainInput, setDomainInput] = useState("");
  const [savingDomain, setSavingDomain] = useState(false);
  const [updateForm, setUpdateForm] = useState<UpdateForm>({ image: "", port: "8080", minScale: "0", maxScale: "20", startupScript: "", env: [] });

  useEffect(() => {
    if (selectedService) {
      setUpdateForm({
        image: selectedService.image,
        port: String(selectedService.port ?? 8080),
        minScale: String(selectedService.minScale ?? 0),
        maxScale: String(selectedService.maxScale ?? 20),
        startupScript: selectedService.startupScript ?? "",
        env: selectedService.env ?? []
      });
    }
  }, [selectedService?.name]);
  const selectedStatusIcon =
    selectedStatus === "ready" ? (
      <CheckCircleIcon fontSize="small" />
    ) : selectedStatus === "loading" ? (
      <CircularProgress size={16} thickness={5} sx={{ color: "inherit" }} />
    ) : (
      <ErrorOutlinedIcon fontSize="small" />
    );

  return (
    <Box sx={{ display: "grid", gridTemplateColumns: { xs: "1fr", lg: "minmax(0, 1fr) 360px" }, gap: 3, alignItems: "start" }}>
      <Box>
        {selectedService ? (
          <Card variant="outlined" sx={{ borderRadius: 2 }}>
            <CardContent sx={{ p: 3, display: "grid", gap: 2 }}>
              <Box sx={{ display: "flex", justifyContent: "space-between", alignItems: "center", gap: 2, flexWrap: "wrap" }}>
                <Box sx={{ display: "grid", gap: 0.75, minWidth: 0 }}>
                  <Typography variant="overline" color="primary">
                    サービス詳細
                  </Typography>
                  <Typography variant="h5" sx={{ fontWeight: 700, wordBreak: "break-word" }}>
                    {selectedService.name}
                  </Typography>
                </Box>
                <Button startIcon={<ArrowBackIcon />} onClick={onBackToList}>
                  一覧に戻る
                </Button>
              </Box>

              <Box sx={{ display: "grid", gridTemplateColumns: { xs: "1fr", sm: "1fr 1fr" }, gap: 1.5 }}>
                <Paper variant="outlined" sx={{ p: 2, borderRadius: 2, bgcolor: "grey.50" }}>
                  <Typography variant="caption" color="text.secondary">
                    状態
                  </Typography>
                  <Box sx={{ mt: 0.75, display: "flex", alignItems: "center", gap: 1, color: selectedStatus === "ready" ? "success.main" : "text.secondary" }}>
                    <Box sx={{ width: 22, height: 22, display: "grid", placeItems: "center", borderRadius: "999px", bgcolor: selectedStatus === "ready" ? "transparent" : selectedStatus === "loading" ? alpha("#2563eb", 0.12) : alpha("#dc2626", 0.12), color: selectedStatus === "ready" ? "success.main" : selectedStatus === "loading" ? "primary.main" : "error.main" }}>
                      {selectedStatusIcon}
                    </Box>
                    <Typography sx={{ fontWeight: 700 }}>{formatServiceStatus(selectedService)}</Typography>
                  </Box>
                </Paper>
                <Paper variant="outlined" sx={{ p: 2, borderRadius: 2, bgcolor: "grey.50" }}>
                  <Typography variant="caption" color="text.secondary">
                    イメージ
                  </Typography>
                  <Typography sx={{ mt: 0.5, fontWeight: 600, wordBreak: "break-all" }}>{selectedService.image}</Typography>
                </Paper>
                <Paper variant="outlined" sx={{ p: 2, borderRadius: 2, bgcolor: "grey.50" }}>
                  <Typography variant="caption" color="text.secondary">
                    公開URL
                  </Typography>
                  <Typography sx={{ mt: 0.5, fontWeight: 600, wordBreak: "break-all" }}>
                    {selectedService.url ? (
                      <Button
                        component="a"
                        href={selectedService.url}
                        target="_blank"
                        rel="noreferrer"
                        variant="text"
                        size="small"
                        sx={actionLinkButtonSx}
                      >
                        {selectedService.url}
                      </Button>
                    ) : (
                      "-"
                    )}
                  </Typography>
                </Paper>
                <Paper variant="outlined" sx={{ p: 2, borderRadius: 2, bgcolor: "grey.50" }}>
                  <Typography variant="caption" color="text.secondary">
                    作成時刻
                  </Typography>
                  <Typography sx={{ mt: 0.5, fontWeight: 600 }}>{selectedService.createdAt ?? "-"}</Typography>
                </Paper>
              </Box>

              <Box sx={{ display: "grid", gap: 1.5 }}>
                <Typography variant="subtitle2" sx={{ fontWeight: 700 }}>
                  イメージを更新
                </Typography>
                <Box
                  component="form"
                  onSubmit={async (event: FormEvent<HTMLFormElement>) => {
                    event.preventDefault();
                    await onUpdateService(selectedService.name, updateForm);
                  }}
                  sx={{ display: "grid", gap: 1.5 }}
                >
                  <TextField
                    size="small"
                    label="コンテナイメージ"
                    value={updateForm.image}
                    onChange={(e) => setUpdateForm((f) => ({ ...f, image: e.target.value }))}
                    disabled={updatingServiceName === selectedService.name}
                    placeholder="ghcr.io/org/app:tag"
                    fullWidth
                    slotProps={{ htmlInput: { autoComplete: "off", autoCorrect: "off", autoCapitalize: "none", spellCheck: false } }}
                  />
                  <Box sx={{ display: "grid", gap: 1.5, gridTemplateColumns: { xs: "1fr", sm: "repeat(3, 1fr)" } }}>
                    <TextField
                      size="small"
                      label="Port"
                      type="number"
                      slotProps={{ htmlInput: { min: 1, max: 65535 } }}
                      value={updateForm.port}
                      onChange={(e) => setUpdateForm((f) => ({ ...f, port: e.target.value }))}
                      disabled={updatingServiceName === selectedService.name}
                    />
                    <TextField
                      size="small"
                      label="最小スケール"
                      type="number"
                      slotProps={{ htmlInput: { min: 0, max: 20 } }}
                      value={updateForm.minScale}
                      onChange={(e) => setUpdateForm((f) => ({ ...f, minScale: e.target.value }))}
                      disabled={updatingServiceName === selectedService.name}
                    />
                    <TextField
                      size="small"
                      label="最大スケール"
                      type="number"
                      slotProps={{ htmlInput: { min: 1, max: 20 } }}
                      value={updateForm.maxScale}
                      onChange={(e) => setUpdateForm((f) => ({ ...f, maxScale: e.target.value }))}
                      disabled={updatingServiceName === selectedService.name}
                    />
                  </Box>
                  <EnvVarEditor
                    value={updateForm.env}
                    onChange={(env) => setUpdateForm((f) => ({ ...f, env }))}
                    disabled={updatingServiceName === selectedService.name}
                    size="small"
                  />
                  <TextField
                    size="small"
                    label="起動スクリプト（任意）"
                    value={updateForm.startupScript}
                    onChange={(e) => setUpdateForm((f) => ({ ...f, startupScript: e.target.value }))}
                    disabled={updatingServiceName === selectedService.name}
                    placeholder={"#!/bin/sh\nexec code-server --bind-addr 0.0.0.0:8080 --auth none ."}
                    multiline
                    minRows={3}
                    fullWidth
                    slotProps={{
                      htmlInput: {
                        autoComplete: "off",
                        autoCorrect: "off",
                        autoCapitalize: "none",
                        spellCheck: false,
                        style: { fontFamily: "monospace", fontSize: "0.85rem" }
                      }
                    }}
                  />
                  <Box sx={{ display: "flex", justifyContent: "flex-end" }}>
                    <Button
                      type="submit"
                      variant="contained"
                      disabled={updatingServiceName === selectedService.name || !updateForm.image.trim()}
                      startIcon={updatingServiceName === selectedService.name ? <CircularProgress size={16} thickness={5} sx={{ color: "inherit" }} /> : undefined}
                    >
                      {updatingServiceName === selectedService.name ? "更新中..." : "更新"}
                    </Button>
                  </Box>
                </Box>
              </Box>

              <Box sx={{ display: "grid", gap: 1.5 }}>
                <Typography variant="subtitle2" sx={{ fontWeight: 700 }}>
                  カスタムドメイン
                </Typography>
                {selectedService.customDomain ? (
                  <Paper variant="outlined" sx={{ p: 2, borderRadius: 2, display: "grid", gap: 1.5 }}>
                    <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 2, flexWrap: "wrap" }}>
                      <Button
                        component="a"
                        href={`https://${selectedService.customDomain}`}
                        target="_blank"
                        rel="noreferrer"
                        variant="text"
                        size="small"
                        sx={{ ...actionLinkButtonSx, fontWeight: 700 }}
                      >
                        {selectedService.customDomain}
                      </Button>
                      <Button
                        variant="outlined"
                        color="error"
                        size="small"
                        disabled={savingDomain}
                        onClick={async () => {
                          setSavingDomain(true);
                          try { await onSetDomain(selectedService.name, ""); } finally { setSavingDomain(false); }
                        }}
                      >
                        {savingDomain ? <CircularProgress size={14} thickness={5} sx={{ color: "inherit" }} /> : "削除"}
                      </Button>
                    </Box>
                    {selectedService.domainStatus === "ready" && (
                      <Chip label="有効" color="success" size="small" icon={<CheckCircleIcon />} sx={{ width: "fit-content" }} />
                    )}
                    {selectedService.domainStatus === "pending" && (
                      <Tooltip title={selectedService.domainStatusReason ?? "DNS または TLS の設定を待機中"}>
                        <Chip
                          label="DNS 待機中"
                          size="small"
                          icon={<CircularProgress size={12} thickness={5} sx={{ color: "inherit !important" }} />}
                          sx={{ width: "fit-content", bgcolor: alpha("#f59e0b", 0.12), color: "warning.dark", "& .MuiChip-icon": { color: "warning.dark" } }}
                        />
                      </Tooltip>
                    )}
                    {selectedService.domainStatus === "error" && (
                      <Tooltip title={selectedService.domainStatusReason ?? ""}>
                        <Chip label="エラー" color="error" size="small" icon={<ErrorOutlinedIcon />} sx={{ width: "fit-content" }} />
                      </Tooltip>
                    )}
                    {selectedService.domainStatus === "pending" && selectedService.domainCnameTarget && (
                      <Typography variant="caption" color="text.secondary">
                        CNAME レコードを <Box component="code" sx={{ bgcolor: "grey.100", px: 0.5, borderRadius: 0.5 }}>{selectedService.domainCnameTarget}</Box> に向けてください
                      </Typography>
                    )}
                  </Paper>
                ) : (
                  <Box sx={{ display: "flex", gap: 1, alignItems: "flex-start" }}>
                    <TextField
                      size="small"
                      placeholder="example.com"
                      value={domainInput}
                      onChange={(e) => setDomainInput(e.target.value)}
                      disabled={savingDomain}
                      sx={{ flex: 1 }}
                    />
                    <Button
                      variant="contained"
                      size="small"
                      disabled={savingDomain || !domainInput.trim()}
                      onClick={async () => {
                        setSavingDomain(true);
                        try { await onSetDomain(selectedService.name, domainInput); setDomainInput(""); } finally { setSavingDomain(false); }
                      }}
                      sx={{ whiteSpace: "nowrap", height: 40 }}
                    >
                      {savingDomain ? <CircularProgress size={14} thickness={5} sx={{ color: "inherit" }} /> : "設定"}
                    </Button>
                  </Box>
                )}
              </Box>

              <ContainerLogsViewer
                serviceName={selectedService.name}
                projectId={activeProjectId}
                enabled={selectedService.ready}
              />

              <Box sx={{ display: "flex", justifyContent: "flex-end" }}>
                <Button
                  variant="contained"
                  color="error"
                  startIcon={deletingServiceName === selectedService.name ? <CircularProgress size={16} thickness={5} sx={{ color: "inherit" }} /> : <DeleteOutlinedIcon />}
                  onClick={() => onDeleteService(selectedService.name)}
                  disabled={deletingServiceName === selectedService.name}
                >
                  {deletingServiceName === selectedService.name ? "削除中..." : "削除"}
                </Button>
              </Box>
            </CardContent>
          </Card>
        ) : (
          <Card variant="outlined" sx={{ borderRadius: 2 }}>
            <CardContent sx={{ p: 3, display: "grid", gap: 2 }}>
              <Box sx={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 2, flexWrap: "wrap" }}>
                <Typography variant="h6" sx={{ fontWeight: 700 }}>
                  サービス
                </Typography>
              </Box>

              <Box sx={{ display: "grid", gap: 0 }}>
                <Box
                  sx={{
                    display: { xs: "none", sm: "grid" },
                    gridTemplateColumns: "42px minmax(0, 1fr) 44px",
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
                  <Box sx={{ display: "grid", gridTemplateColumns: "minmax(120px, max-content) 150px", columnGap: 3 }}>
                    <Box>名前</Box>
                    <Box>更新日時</Box>
                  </Box>
                  <Box sx={{ textAlign: "right" }}>操作</Box>
                </Box>

                <Box sx={{ borderTop: "1px solid rgba(148, 163, 184, 0.18)" }}>
                  {containers.length > 0 ? (
                    containers.map((service) => {
                      const isDeleting = deletingServiceName === service.name;
                      const status = getServiceStatus(service);
                      const statusIcon = isDeleting ? (
                        <CircularProgress size={14} thickness={5.5} sx={{ color: "inherit" }} />
                      ) : status === "ready" ? (
                        <CheckCircleIcon fontSize="small" />
                      ) : status === "loading" ? (
                        <CircularProgress size={14} thickness={5.5} sx={{ color: "inherit" }} />
                      ) : (
                        <ErrorOutlinedIcon fontSize="small" />
                      );
                      const statusColor = isDeleting ? alpha("#dc2626", 0.12) : status === "ready" ? "transparent" : status === "loading" ? alpha("#2563eb", 0.12) : alpha("#dc2626", 0.12);
                      const statusTextColor = isDeleting ? "error.main" : status === "ready" ? "success.main" : status === "loading" ? "primary.main" : "error.main";
                      return (
                        <Paper
                          key={service.name}
                          variant="outlined"
                          sx={{
                            display: "grid",
                            gridTemplateColumns: { xs: "42px minmax(0, 1fr) 44px", sm: "42px minmax(0, 1fr) 44px" },
                            gap: { xs: 0, sm: 0 },
                            alignItems: "center",
                            minHeight: { xs: 40, sm: 44 },
                            p: { xs: 1, sm: 0 },
                            borderRadius: 0,
                            borderLeft: 0,
                            borderRight: 0,
                            borderTop: 0
                          }}
                          >
                          <Box sx={{ display: "grid", placeItems: "center" }}>
                            <Box sx={{ width: 22, height: 22, display: "grid", placeItems: "center", borderRadius: "999px", bgcolor: statusColor, color: statusTextColor }}>
                              {statusIcon}
                            </Box>
                          </Box>
                          <Box sx={{ display: "grid", gridTemplateColumns: { xs: "1fr", sm: "minmax(120px, max-content) 150px" }, columnGap: 3, rowGap: 0.5, alignItems: "center", minWidth: 0 }}>
                            <Button component={RouterLink} to={`/container/${encodeURIComponent(service.name)}`} onClick={() => onOpenService(service.name)} sx={{ justifyContent: "flex-start", textAlign: "left", color: "inherit", px: 0, minWidth: 0 }}>
                              <Typography sx={{ fontWeight: 700, wordBreak: "break-all" }}>{service.name}</Typography>
                            </Button>
                            <Typography variant="body2" color="text.secondary" sx={{ display: { xs: "none", sm: "block" }, whiteSpace: { xs: "normal", sm: "nowrap" } }}>
                              {service.updatedAt || service.createdAt ? formatServiceTimestamp(service.updatedAt || service.createdAt || "") : "-"}
                            </Typography>
                          </Box>
                          <Box sx={{ display: "flex", justifyContent: "flex-end", width: 44, minWidth: 44 }}>
                            <Tooltip title="削除">
                              <span>
                                <IconButton
                                  color="error"
                                  disabled={isDeleting}
                                  onClick={() => onDeleteService(service.name)}
                                  size="small"
                                  sx={{
                                    border: "1px solid",
                                    borderColor: "error.main",
                                    bgcolor: "error.main",
                                    color: "common.white",
                                    "&:hover": {
                                      bgcolor: "error.dark",
                                      borderColor: "error.dark"
                                    },
                                    "&.Mui-disabled": {
                                      bgcolor: "rgba(220, 38, 38, 0.08)",
                                      color: "error.main",
                                      borderColor: "rgba(220, 38, 38, 0.2)"
                                    }
                                  }}
                                >
                                  <DeleteOutlinedIcon fontSize="small" />
                                </IconButton>
                              </span>
                            </Tooltip>
                          </Box>
                        </Paper>
                      );
                    })
                  ) : (
                    <Paper variant="outlined" sx={{ mt: 1.5, p: 2, borderRadius: 2, borderStyle: "dashed", bgcolor: alpha("#ffffff", 0.7) }}>
                      <Typography color="text.secondary">{loading ? "読み込み中..." : "まだサービスはありません。"}</Typography>
                    </Paper>
                  )}
                </Box>
              </Box>
            </CardContent>
          </Card>
        )}
      </Box>

      {!selectedService ? (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
          <Card variant="outlined" sx={{ borderRadius: 2 }}>
            <CardContent sx={{ p: 3, display: "grid", gap: 2 }}>
              <Typography variant="h6" sx={{ fontWeight: 700 }}>
                サービスのデプロイ
              </Typography>
              <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
                <Button component={RouterLink} to="/container/deploy" variant="contained" startIcon={<CloudUploadOutlinedIcon />} fullWidth onClick={onDeployClick}>
                  コンテナのデプロイ
                </Button>
                <Button component={RouterLink} to="/container/repository" variant="outlined" startIcon={<GitHubIcon />} fullWidth onClick={onRepoConnectClick}>
                  リポジトリの接続
                </Button>
              </Box>
            </CardContent>
          </Card>
        </Box>
      ) : null}
    </Box>
  );
}
