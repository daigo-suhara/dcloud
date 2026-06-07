import DeleteOutlinedIcon from "@mui/icons-material/DeleteOutlined";
import CheckCircleIcon from "@mui/icons-material/CheckCircle";
import { alpha } from "@mui/material/styles";
import { Alert, Box, Button, Card, CardContent, CircularProgress, Paper, TextField, Typography } from "@mui/material";
import type { FormEvent } from "react";
import type { ComputeForm, ComputeMachine } from "../types";
import { actionLinkButtonSx } from "../theme";
import { formatComputeStatus, formatComputeTimestamp } from "../utils";

function isDnsLabel(value: string) {
  return /^[a-z0-9]([a-z0-9-]*[a-z0-9])?$/.test(value) && value.length <= 63;
}

type ComputeSectionProps = {
  error: string;
  form: ComputeForm;
  loading: boolean;
  deletingMachineName: string;
  machines: ComputeMachine[];
  onChange: (patch: Partial<ComputeForm>) => void;
  onDeleteMachine: (name: string) => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
  submitting: boolean;
};

export function ComputeSection({
  error,
  form,
  loading,
  deletingMachineName,
  machines,
  onChange,
  onDeleteMachine,
  onSubmit,
  submitting
}: ComputeSectionProps) {
  const machineName = form.name.trim();
  const machineNameError = machineName.length > 0 && !isDnsLabel(machineName);

  function fillTestImage() {
    onChange({
      image: "quay.io/containerdisks/fedora:latest",
      cpu: "1",
      memory: "1Gi"
    });
  }

  return (
    <Box sx={{ display: "grid", gridTemplateColumns: { xs: "1fr", lg: "minmax(0, 1fr) 360px" }, gap: 3, alignItems: "start" }}>
      <Box>
        <Card variant="outlined" sx={{ borderRadius: 2 }}>
          <CardContent sx={{ p: 3, display: "grid", gap: 2 }}>
            <Box sx={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", gap: 2, flexWrap: "wrap" }}>
              <Box sx={{ display: "grid", gap: 0.75 }}>
                <Typography variant="h5" sx={{ fontWeight: 700 }}>
                  仮想マシン
                </Typography>
                <Typography variant="body2" color="text.secondary">
                  KubeVirt で作成された VM の一覧です。
                </Typography>
              </Box>
            </Box>

            <Box sx={{ display: "grid", gap: 0 }}>
              <Box
                sx={{
                  display: { xs: "none", sm: "grid" },
                  gridTemplateColumns: "42px minmax(0, 1fr) minmax(220px, max-content)",
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
                {machines.length > 0 ? (
                  machines.map((machine) => {
                    const status = formatComputeStatus(machine);
                    const isReady = machine.ready;
                    const statusIcon =
                      isReady ? (
                        <CheckCircleIcon fontSize="small" />
                      ) : (
                        <CircularProgress size={14} thickness={5.5} sx={{ color: "inherit" }} />
                      );

                    return (
                      <Paper
                        key={machine.name}
                        variant="outlined"
                        sx={{
                          display: "grid",
                          gridTemplateColumns: { xs: "42px minmax(0, 1fr)", sm: "42px minmax(0, 1fr) minmax(220px, max-content)" },
                          gap: 0,
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
                          <Box
                            sx={{
                              width: 22,
                              height: 22,
                              display: "grid",
                              placeItems: "center",
                              borderRadius: "999px",
                              bgcolor: isReady ? "transparent" : alpha("#2563eb", 0.12),
                              color: isReady ? "success.main" : "primary.main"
                            }}
                          >
                            {statusIcon}
                          </Box>
                        </Box>
                        <Box sx={{ display: "grid", gridTemplateColumns: { xs: "1fr", sm: "minmax(120px, 1fr) max-content" }, columnGap: 3, rowGap: 0.5, alignItems: "center", minWidth: 0 }}>
                          <Box sx={{ display: "grid", gap: 0.25, minWidth: 0 }}>
                            <Typography sx={{ fontWeight: 700, wordBreak: "break-all" }}>{machine.name}</Typography>
                            <Typography variant="body2" color="text.secondary" sx={{ wordBreak: "break-all" }}>
                              {machine.image}
                            </Typography>
                            <Typography variant="caption" color="text.secondary">
                              CPU {machine.cpu} / MEM {machine.memory}
                            </Typography>
                          </Box>
                          <Box sx={{ display: "flex", alignItems: "center", justifyContent: "flex-end", gap: 1.5, minWidth: 0, flexWrap: "wrap" }}>
                            <Box sx={{ display: "grid", gap: 0.25, minWidth: 0, textAlign: { xs: "left", sm: "right" } }}>
                              <Typography variant="body2" color={isReady ? "success.main" : "text.secondary"} sx={{ fontWeight: 600 }}>
                                {status}
                              </Typography>
                              <Typography variant="caption" color="text.secondary" sx={{ whiteSpace: "nowrap" }}>
                                {formatComputeTimestamp(machine.updatedAt || machine.createdAt)}
                              </Typography>
                            </Box>
                            <Button
                              variant="text"
                              size="small"
                              color="error"
                              startIcon={
                                deletingMachineName === machine.name ? <CircularProgress size={14} thickness={5} sx={{ color: "inherit" }} /> : <DeleteOutlinedIcon fontSize="small" />
                              }
                              onClick={() => onDeleteMachine(machine.name)}
                              disabled={deletingMachineName === machine.name}
                              sx={{ minWidth: 0, whiteSpace: "nowrap" }}
                            >
                              {deletingMachineName === machine.name ? "削除中..." : "削除"}
                            </Button>
                          </Box>
                        </Box>
                      </Paper>
                    );
                  })
                ) : (
                  <Paper variant="outlined" sx={{ mt: 1.5, p: 2, borderRadius: 2, borderStyle: "dashed", bgcolor: alpha("#ffffff", 0.7) }}>
                    <Typography color="text.secondary">{loading ? "読み込み中..." : "まだ仮想マシンはありません。"}</Typography>
                  </Paper>
                )}
              </Box>
            </Box>
          </CardContent>
        </Card>
      </Box>

      <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
        <Card variant="outlined" sx={{ borderRadius: 2 }}>
          <CardContent sx={{ p: { xs: 2.5, sm: 3 }, display: "grid", gap: 2.25 }}>
            <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 2, flexWrap: "wrap" }}>
              <Typography variant="h6" sx={{ fontWeight: 700 }}>
                仮想マシンを作成
              </Typography>
              <Button type="button" variant="text" size="small" onClick={fillTestImage} sx={actionLinkButtonSx}>
                サンプルVMを使用
              </Button>
            </Box>

            <Box component="form" onSubmit={onSubmit} sx={{ display: "grid", gap: 2.25 }}>
              <TextField
                label="VM名"
                value={form.name}
                onChange={(event) => onChange({ name: event.target.value })}
                placeholder="vm-name"
                error={machineNameError}
                helperText={machineNameError ? "英小文字・数字・ハイフンのみ" : ""}
                slotProps={{
                  htmlInput: {
                    autoCapitalize: "none",
                    autoComplete: "off",
                    autoCorrect: "off",
                    inputMode: "text",
                    maxLength: 63,
                    pattern: "[a-z0-9]([a-z0-9-]*[a-z0-9])?"
                  }
                }}
                fullWidth
              />

              <TextField
                label="イメージ"
                value={form.image}
                onChange={(event) => onChange({ image: event.target.value })}
                placeholder="quay.io/containerdisks/fedora:latest"
                fullWidth
                slotProps={{
                  htmlInput: {
                    autoComplete: "off",
                    autoCorrect: "off",
                    autoCapitalize: "none",
                    spellCheck: false
                  }
                }}
              />

              <Box sx={{ display: "grid", gap: 2, gridTemplateColumns: { xs: "1fr", md: "repeat(2, minmax(0, 1fr))" } }}>
                <TextField
                  label="CPU"
                  value={form.cpu}
                  onChange={(event) => onChange({ cpu: event.target.value })}
                  placeholder="1"
                  fullWidth
                />
                <TextField
                  label="メモリ"
                  value={form.memory}
                  onChange={(event) => onChange({ memory: event.target.value })}
                  placeholder="1Gi"
                  fullWidth
                />
              </Box>

              <Box sx={{ display: "flex", flexWrap: "wrap", justifyContent: "flex-end", gap: 1.25, pt: 0.5 }}>
                <Button type="submit" variant="contained" disabled={submitting || machineNameError}>
                  {submitting ? "作成中..." : "作成"}
                </Button>
              </Box>
            </Box>

            {error ? <Alert severity="error">{error}</Alert> : null}
          </CardContent>
        </Card>
      </Box>
    </Box>
  );
}
