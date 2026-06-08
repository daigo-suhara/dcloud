import ArrowBackIcon from "@mui/icons-material/ArrowBack";
import { alpha } from "@mui/material/styles";
import { Alert, Box, Button, Card, CardContent, Paper, TextField, Typography } from "@mui/material";
import type { FormEvent } from "react";
import { SiCentos, SiDebian, SiFedora, SiOpensuse, SiUbuntu } from "react-icons/si";
import type { ComputeForm } from "../types";
import { actionLinkButtonSx } from "../theme";

function isDnsLabel(value: string) {
  return /^[a-z0-9]([a-z0-9-]*[a-z0-9])?$/.test(value) && value.length <= 63;
}

type ComputeCreateSectionProps = {
  error: string;
  form: ComputeForm;
  onBack: () => void;
  onChange: (patch: Partial<ComputeForm>) => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
  submitting: boolean;
};

const imagePresets = [
  {
    label: "Fedora",
    image: "quay.io/containerdisks/fedora:latest",
    icon: SiFedora
  },
  {
    label: "Ubuntu",
    image: "quay.io/containerdisks/ubuntu:latest",
    icon: SiUbuntu
  },
  {
    label: "Debian",
    image: "quay.io/containerdisks/debian:latest",
    icon: SiDebian
  },
  {
    label: "CentOS Stream",
    image: "quay.io/containerdisks/centos-stream:latest",
    icon: SiCentos
  },
  {
    label: "openSUSE Leap",
    image: "quay.io/containerdisks/opensuse-leap:latest",
    icon: SiOpensuse
  },
  {
    label: "openSUSE Tumbleweed",
    image: "quay.io/containerdisks/opensuse-tumbleweed:latest",
    icon: SiOpensuse
  }
] as const;

export function ComputeCreateSection({ error, form, onBack, onChange, onSubmit, submitting }: ComputeCreateSectionProps) {
  const machineName = form.name.trim();
  const machineNameError = machineName.length > 0 && !isDnsLabel(machineName);

  function fillSample() {
    onChange({
      image: "quay.io/containerdisks/fedora:latest",
      cpu: "1",
      memory: "1Gi"
    });
  }

  return (
    <Box sx={{ display: "grid", gap: 3 }}>
      <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}>
        <Button type="button" variant="text" size="small" startIcon={<ArrowBackIcon fontSize="small" />} onClick={onBack} sx={actionLinkButtonSx}>
          一覧へ戻る
        </Button>
        <Typography variant="h5" sx={{ fontWeight: 800 }}>
          仮想マシンを作成
        </Typography>
      </Box>

      <Box sx={{ display: "grid", gap: 3, gridTemplateColumns: { xs: "1fr", xl: "minmax(0, 1.15fr) minmax(0, 0.85fr)" }, alignItems: "start" }}>
        <Card variant="outlined" sx={{ borderRadius: 2 }}>
          <CardContent sx={{ p: { xs: 2.5, sm: 3 }, display: "grid", gap: 2.25 }}>
            <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 2, flexWrap: "wrap" }}>
              <Box sx={{ display: "grid", gap: 0.5 }}>
                <Typography variant="h6" sx={{ fontWeight: 800 }}>
                  イメージを選ぶ
                </Typography>
                <Typography variant="body2" color="text.secondary">
                  公式の containerdisk から選択できます。必要なら右側でそのままカスタム image も入力できます。
                </Typography>
              </Box>
              <Button type="button" variant="text" size="small" onClick={fillSample} sx={actionLinkButtonSx}>
                サンプルVMを使用
              </Button>
            </Box>

            <Box
              sx={{
                display: "grid",
                gap: 1.25,
                gridTemplateColumns: {
                  xs: "1fr",
                  sm: "repeat(2, minmax(0, 1fr))",
                  lg: "repeat(3, minmax(0, 1fr))"
                }
              }}
            >
              {imagePresets.map((preset) => {
                const selected = preset.image === form.image;
                const Icon = preset.icon;
                return (
                  <Paper
                    key={preset.image}
                    component="button"
                    type="button"
                    onClick={() => onChange({ image: preset.image })}
                    variant="outlined"
                    sx={{
                      textAlign: "center",
                      width: "100%",
                      p: 1.5,
                      borderRadius: 2,
                      borderColor: selected ? "primary.main" : "rgba(148, 163, 184, 0.18)",
                      bgcolor: selected ? alpha("#2563eb", 0.08) : "background.paper",
                      cursor: "pointer",
                      display: "grid",
                      gap: 1,
                      justifyItems: "center",
                      transition: "border-color 120ms ease, background-color 120ms ease, transform 120ms ease",
                      "&:hover": {
                        transform: "translateY(-1px)",
                        borderColor: "primary.main",
                        bgcolor: alpha("#2563eb", 0.04)
                      }
                    }}
                  >
                    <Box
                      sx={{
                        width: 52,
                        height: 52,
                        display: "grid",
                        placeItems: "center",
                        borderRadius: "999px",
                        color: selected ? "primary.main" : "text.secondary",
                        bgcolor: selected ? alpha("#2563eb", 0.12) : alpha("#0f172a", 0.04)
                      }}
                    >
                      <Icon size={28} />
                    </Box>
                    <Typography sx={{ fontWeight: 800, lineHeight: 1.2 }}>{preset.label}</Typography>
                  </Paper>
                );
              })}
            </Box>
          </CardContent>
        </Card>

        <Card variant="outlined" sx={{ borderRadius: 2 }}>
          <CardContent sx={{ p: { xs: 2.5, sm: 3 }, display: "grid", gap: 2.25 }}>
            <Box sx={{ display: "grid", gap: 0.5 }}>
              <Typography variant="h6" sx={{ fontWeight: 800 }}>
                作成設定
              </Typography>
              <Typography variant="body2" color="text.secondary">
                名前、CPU、メモリを指定して VM を作成します。
              </Typography>
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
