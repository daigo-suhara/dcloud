import ArrowBackIcon from "@mui/icons-material/ArrowBack";
import AddIcon from "@mui/icons-material/Add";
import { Alert, Box, Button, MenuItem, Paper, TextField, Typography } from "@mui/material";
import { useEffect, useState, type FormEvent } from "react";
import { SiCentos, SiDebian, SiFedora, SiUbuntu } from "react-icons/si";
import type { ComputeForm } from "../types";
import { PageHeader } from "./primitives";

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
  { label: "Fedora", image: "quay.io/containerdisks/fedora:latest", icon: <SiFedora size={22} color="#51A2DA" /> },
  { label: "Ubuntu", image: "quay.io/containerdisks/ubuntu:24.04",  icon: <SiUbuntu size={22} color="#E95420" /> },
  { label: "Debian", image: "quay.io/containerdisks/debian:latest",  icon: <SiDebian size={22} color="#A81D33" /> },
  { label: "CentOS Stream", image: "quay.io/containerdisks/centos-stream:latest", icon: <SiCentos size={22} color="#262577" /> },
  { label: "カスタム", image: "", icon: <AddIcon fontSize="small" /> }
] as const;

type SizePreset = { label: string; cpu: string; memory: string };
const SIZE_PRESETS: Record<string, SizePreset> = {
  small:  { label: "Small (1 CPU / 1 GiB)",  cpu: "1", memory: "1Gi" },
  medium: { label: "Medium (2 CPU / 4 GiB)", cpu: "2", memory: "4Gi" },
  large:  { label: "Large (4 CPU / 8 GiB)",  cpu: "4", memory: "8Gi" },
};

function detectPreset(form: ComputeForm): string {
  for (const [k, p] of Object.entries(SIZE_PRESETS)) {
    if (p.cpu === form.cpu && p.memory === form.memory) return k;
  }
  return "small";
}

export function ComputeCreateSection({ error, form, onBack, onChange, onSubmit, submitting }: ComputeCreateSectionProps) {
  const machineName = form.name.trim();
  const machineNameError = machineName.length > 0 && !isDnsLabel(machineName);
  const selectedPreset = imagePresets.find((preset) => preset.image === form.image) ?? null;
  const [customMode, setCustomMode] = useState(!selectedPreset);
  const [sizePreset, setSizePreset] = useState(detectPreset(form));

  useEffect(() => {
    setCustomMode((current) => current || !selectedPreset);
  }, [selectedPreset]);

  function selectImage(image: string) {
    if (image === "") { setCustomMode(true); return; }
    setCustomMode(false);
    onChange({ image });
  }

  function selectSize(k: string) {
    setSizePreset(k);
    const p = SIZE_PRESETS[k];
    onChange({ cpu: p.cpu, memory: p.memory });
  }

  return (
    <Box>
      <PageHeader
        title="仮想マシンを作成"
        subtitle="OS イメージとサイズを選んで KubeVirt VM を起動します"
        actions={<Button startIcon={<ArrowBackIcon />} onClick={onBack}>一覧に戻る</Button>}
      />

      <Box sx={{ display: "grid", gap: 3, gridTemplateColumns: { xs: "1fr", lg: "minmax(0, 1fr) minmax(0, 1fr)" } }}>
        {/* Image picker */}
        <Paper variant="outlined" sx={{ p: 2.5 }}>
          <Typography variant="h6" sx={{ mb: 1 }}>イメージ</Typography>
          <Box sx={{ display: "grid", gap: 1, gridTemplateColumns: { xs: "1fr", sm: "repeat(2, 1fr)" } }}>
            {imagePresets.map((preset) => {
              const selected = preset.image === "" ? customMode : selectedPreset?.image === preset.image && !customMode;
              return (
                <Paper
                  key={preset.image || "custom"}
                  component="button" type="button"
                  onClick={() => selectImage(preset.image)}
                  variant="outlined"
                  sx={{
                    display: "flex", alignItems: "center", gap: 1.5,
                    p: 1.5, cursor: "pointer", width: "100%", textAlign: "left",
                    borderColor: selected ? "primary.main" : "divider",
                    bgcolor: selected ? "rgba(26,115,232,0.06)" : "transparent",
                    "&:hover": { borderColor: "primary.main", bgcolor: "rgba(60,64,67,0.04)" }
                  }}
                >
                  <Box sx={{ width: 32, height: 32, display: "grid", placeItems: "center", color: selected ? "primary.main" : "text.secondary" }}>
                    {preset.icon}
                  </Box>
                  <Typography variant="body2" sx={{ fontWeight: 500 }}>{preset.label}</Typography>
                </Paper>
              );
            })}
          </Box>
          {customMode && (
            <TextField
              label="イメージ URL" value={form.image}
              onChange={(e) => onChange({ image: e.target.value })}
              placeholder="quay.io/containerdisks/custom:latest"
              fullWidth sx={{ mt: 2 }}
              slotProps={{ htmlInput: { autoCapitalize: "none", autoComplete: "off", autoCorrect: "off", spellCheck: false } }}
            />
          )}
        </Paper>

        {/* Config form */}
        <Paper variant="outlined" sx={{ p: 2.5 }}>
          <Typography variant="h6" sx={{ mb: 1 }}>設定</Typography>
          <Box component="form" onSubmit={onSubmit} sx={{ display: "grid", gap: 2 }}>
            <TextField
              label="VM 名" value={form.name}
              onChange={(e) => onChange({ name: e.target.value })}
              placeholder="vm-name"
              error={machineNameError}
              helperText={machineNameError ? "英小文字・数字・ハイフンのみ" : "英小文字・数字・ハイフンで指定してください"}
              fullWidth
              slotProps={{ htmlInput: { autoCapitalize: "none", autoComplete: "off", autoCorrect: "off", inputMode: "text", maxLength: 63, pattern: "[a-z0-9]([a-z0-9-]*[a-z0-9])?" } }}
            />
            <TextField
              select label="リソース" value={sizePreset}
              onChange={(e) => selectSize(e.target.value)} fullWidth
            >
              {Object.entries(SIZE_PRESETS).map(([k, p]) => (
                <MenuItem key={k} value={k}>{p.label}</MenuItem>
              ))}
            </TextField>
            {error && <Alert severity="error">{error}</Alert>}
            <Box sx={{ display: "flex", justifyContent: "flex-end" }}>
              <Button type="submit" variant="contained" disabled={submitting || machineNameError || !machineName || !form.image.trim()}>
                {submitting ? "作成中..." : "作成"}
              </Button>
            </Box>
          </Box>
        </Paper>
      </Box>
    </Box>
  );
}
