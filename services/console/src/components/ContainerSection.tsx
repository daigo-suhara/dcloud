import { alpha } from "@mui/material/styles";
import ArrowBackIcon from "@mui/icons-material/ArrowBack";
import CheckCircleIcon from "@mui/icons-material/CheckCircle";
import CloudUploadOutlinedIcon from "@mui/icons-material/CloudUploadOutlined";
import ErrorOutlinedIcon from "@mui/icons-material/ErrorOutlined";
import GitHubIcon from "@mui/icons-material/GitHub";
import HourglassTopIcon from "@mui/icons-material/HourglassTop";
import DeleteOutlinedIcon from "@mui/icons-material/DeleteOutlined";
import { Box, Button, Card, CardContent, Paper, Typography } from "@mui/material";
import type { DeployedService } from "../types";
import { formatServiceStatus, formatServiceTimestamp, getServiceStatus } from "../utils";

type ContainerSectionProps = {
  loading: boolean;
  onBackToList: () => void;
  onDeployClick: () => void;
  onDeleteService: (name: string) => void;
  onOpenService: (name: string) => void;
  onRepoConnectClick: () => void;
  selectedService: DeployedService | null;
  selectedStatus: ReturnType<typeof getServiceStatus> | null;
  services: DeployedService[];
};

export function ContainerSection({
  loading,
  onBackToList,
  onDeployClick,
  onDeleteService,
  onOpenService,
  onRepoConnectClick,
  selectedService,
  selectedStatus,
  services
}: ContainerSectionProps) {
  const selectedStatusIcon =
    selectedStatus === "ready" ? (
      <CheckCircleIcon fontSize="small" />
    ) : selectedStatus === "loading" ? (
      <HourglassTopIcon fontSize="small" />
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
                  <Box sx={{ display: "flex", alignItems: "center", gap: 1.25, minWidth: 0 }}>
                    <Box sx={{ width: 34, height: 34, borderRadius: "999px", display: "grid", placeItems: "center", bgcolor: selectedStatus === "ready" ? alpha("#16a34a", 0.12) : selectedStatus === "loading" ? alpha("#2563eb", 0.12) : alpha("#dc2626", 0.12), color: selectedStatus === "ready" ? "success.main" : selectedStatus === "loading" ? "primary.main" : "error.main" }}>
                      {selectedStatusIcon}
                    </Box>
                    <Typography variant="h5" sx={{ fontWeight: 700, wordBreak: "break-word" }}>
                      {selectedService.name}
                    </Typography>
                  </Box>
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
                  <Typography sx={{ mt: 0.5, fontWeight: 600 }}>{formatServiceStatus(selectedService)}</Typography>
                </Paper>
                <Paper variant="outlined" sx={{ p: 2, borderRadius: 2, bgcolor: "grey.50" }}>
                  <Typography variant="caption" color="text.secondary">
                    イメージ
                  </Typography>
                  <Typography sx={{ mt: 0.5, fontWeight: 600, wordBreak: "break-all" }}>{selectedService.image}</Typography>
                </Paper>
                <Paper variant="outlined" sx={{ p: 2, borderRadius: 2, bgcolor: "grey.50" }}>
                  <Typography variant="caption" color="text.secondary">
                    URL
                  </Typography>
                  <Typography sx={{ mt: 0.5, fontWeight: 600, wordBreak: "break-all" }}>
                    {selectedService.url ? (
                      <a href={selectedService.url} target="_blank" rel="noreferrer">
                        {selectedService.url}
                      </a>
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

              <Box sx={{ display: "flex", justifyContent: "flex-end" }}>
                <Button variant="contained" color="error" startIcon={<DeleteOutlinedIcon />} onClick={() => onDeleteService(selectedService.name)}>
                  削除
                </Button>
              </Box>
            </CardContent>
          </Card>
        ) : (
          <Card variant="outlined" sx={{ borderRadius: 2 }}>
            <CardContent sx={{ p: 3, display: "grid", gap: 2 }}>
              <Box sx={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 2, flexWrap: "wrap" }}>
                <Typography variant="h5" sx={{ fontWeight: 700 }}>
                  デプロイ済みサービス
                </Typography>
              </Box>

              <Box sx={{ display: "grid", gap: 0 }}>
                <Box sx={{ display: { xs: "none", sm: "grid" }, gridTemplateColumns: "42px minmax(0, 1fr)", alignItems: "center", minHeight: 36, px: 1, color: "text.secondary", fontSize: 11, fontWeight: 700, borderBottom: "1px solid rgba(148, 163, 184, 0.18)" }}>
                  <Box />
                  <Box sx={{ display: "grid", gridTemplateColumns: "minmax(120px, max-content) 150px", columnGap: 3 }}>
                    <Box>名前</Box>
                    <Box>更新日時</Box>
                  </Box>
                </Box>

                <Box sx={{ borderTop: "1px solid rgba(148, 163, 184, 0.18)" }}>
                  {services.length > 0 ? (
                    services.map((service) => {
                      const status = getServiceStatus(service);
                      const statusIcon = status === "ready" ? <CheckCircleIcon fontSize="small" /> : status === "loading" ? <HourglassTopIcon fontSize="small" /> : <ErrorOutlinedIcon fontSize="small" />;
                      return (
                        <Paper
                          key={service.name}
                          variant="outlined"
                          sx={{
                            display: "grid",
                            gridTemplateColumns: { xs: "1fr", sm: "42px minmax(0, 1fr)" },
                            gap: { xs: 1, sm: 0 },
                            alignItems: { xs: "start", sm: "center" },
                            minHeight: 44,
                            p: { xs: 1.25, sm: 0 },
                            borderRadius: 0,
                            borderLeft: 0,
                            borderRight: 0,
                            borderTop: 0
                          }}
                        >
                          <Box sx={{ display: "grid", placeItems: "center", alignSelf: { xs: "start", sm: "center" } }}>
                            <Box sx={{ width: 22, height: 22, display: "grid", placeItems: "center", borderRadius: "999px", bgcolor: status === "ready" ? alpha("#16a34a", 0.12) : status === "loading" ? alpha("#2563eb", 0.12) : alpha("#dc2626", 0.12), color: status === "ready" ? "success.main" : status === "loading" ? "primary.main" : "error.main" }}>
                              {statusIcon}
                            </Box>
                          </Box>
                          <Box sx={{ display: "grid", gridTemplateColumns: { xs: "1fr", sm: "minmax(120px, max-content) 150px" }, columnGap: 3, rowGap: 0.5, alignItems: "center", minWidth: 0 }}>
                            <Button component="a" href={`#container/${encodeURIComponent(service.name)}`} onClick={() => onOpenService(service.name)} sx={{ justifyContent: "flex-start", textAlign: "left", color: "inherit", px: 0, minWidth: 0 }}>
                              <Typography sx={{ fontWeight: 700, wordBreak: "break-all" }}>{service.name}</Typography>
                            </Button>
                            <Typography variant="body2" color="text.secondary" sx={{ whiteSpace: { xs: "normal", sm: "nowrap" } }}>
                              {service.updatedAt || service.createdAt ? formatServiceTimestamp(service.updatedAt || service.createdAt || "") : "-"}
                            </Typography>
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

      <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
        <Card variant="outlined" sx={{ borderRadius: 2 }}>
          <CardContent sx={{ p: 3, display: "grid", gap: 2 }}>
            <Typography variant="h6" sx={{ fontWeight: 700 }}>
              サービスのデプロイ
            </Typography>
            <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
              <Button component="a" href="#deploy" variant="contained" startIcon={<CloudUploadOutlinedIcon />} fullWidth onClick={onDeployClick}>
                コンテナのデプロイ
              </Button>
              <Button component="a" href="#container" variant="outlined" startIcon={<GitHubIcon />} fullWidth onClick={onRepoConnectClick}>
                リポジトリの接続
              </Button>
            </Box>
          </CardContent>
        </Card>
      </Box>
    </Box>
  );
}
