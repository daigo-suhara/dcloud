import ArrowBackIcon from "@mui/icons-material/ArrowBack";
import { Alert, Box, Button, Paper, TextField, Typography } from "@mui/material";
import type { FormEvent } from "react";
import type { DeployForm } from "../types";
import { actionLinkButtonSx } from "../theme";
import { EnvVarEditor } from "./EnvVarEditor";
import { PageHeader } from "./primitives";

function isDnsLabel(value: string) {
  return /^[a-z0-9]([a-z0-9-]*[a-z0-9])?$/.test(value) && value.length <= 63;
}

type DeploySectionProps = {
  error: string;
  form: DeployForm;
  onBack: () => void;
  onChange: (patch: Partial<DeployForm>) => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
  submitting: boolean;
};

export function DeploySection({ error, form, onBack, onChange, onSubmit, submitting }: DeploySectionProps) {
  const serviceName = form.name.trim();
  const serviceNameError = serviceName.length > 0 && !isDnsLabel(serviceName);

  function fillTestImage() {
    onChange({ name: "hello", image: "gcr.io/knative-samples/helloworld-go:latest" });
  }

  return (
    <Box>
      <PageHeader
        title="コンテナをデプロイ"
        subtitle="Knative サービスを作成します"
        actions={<Button startIcon={<ArrowBackIcon />} onClick={onBack}>一覧に戻る</Button>}
      />

      <Paper variant="outlined" sx={{ p: 2.5 }}>
        <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", mb: 2, gap: 1, flexWrap: "wrap" }}>
          <Typography variant="h6">設定</Typography>
          <Button type="button" variant="text" size="small" onClick={fillTestImage} sx={actionLinkButtonSx}>
            サンプルコンテナを使用
          </Button>
        </Box>

        <Box component="form" onSubmit={onSubmit} sx={{ display: "grid", gap: 2 }}>
          <TextField
            label="サービス名" value={form.name}
            onChange={(e) => onChange({ name: e.target.value })}
            placeholder="service-name"
            error={serviceNameError}
            helperText={serviceNameError ? "英小文字・数字・ハイフンのみ" : "英小文字・数字・ハイフンで指定してください"}
            slotProps={{ htmlInput: { autoCapitalize: "none", autoComplete: "off", autoCorrect: "off", inputMode: "text", maxLength: 63, pattern: "[a-z0-9]([a-z0-9-]*[a-z0-9])?" } }}
            fullWidth
          />
          <TextField
            label="コンテナイメージ" value={form.image}
            onChange={(e) => onChange({ image: e.target.value })}
            placeholder="ghcr.io/org/app:tag" fullWidth
            slotProps={{ htmlInput: { autoComplete: "off", autoCorrect: "off", autoCapitalize: "none", spellCheck: false } }}
          />
          <Box sx={{ display: "grid", gap: 2, gridTemplateColumns: { xs: "1fr", sm: "repeat(3, minmax(0, 1fr))" } }}>
            <TextField label="Port" type="number"
              slotProps={{ htmlInput: { min: 1, max: 65535 } }}
              value={form.port} onChange={(e) => onChange({ port: e.target.value })} placeholder="8080" fullWidth />
            <TextField label="最小スケール" type="number"
              slotProps={{ htmlInput: { min: 0, max: 20 } }}
              value={form.minScale} onChange={(e) => onChange({ minScale: e.target.value })} placeholder="0" fullWidth />
            <TextField label="最大スケール" type="number"
              slotProps={{ htmlInput: { min: 1, max: 20 } }}
              value={form.maxScale} onChange={(e) => onChange({ maxScale: e.target.value })} placeholder="1" fullWidth />
          </Box>
          <EnvVarEditor value={form.env} onChange={(env) => onChange({ env })} />
          <TextField
            label="起動スクリプト（任意）" value={form.startupScript}
            onChange={(e) => onChange({ startupScript: e.target.value })}
            placeholder={"#!/bin/sh\nexec code-server --bind-addr 0.0.0.0:8080 --auth none ."}
            multiline minRows={3} fullWidth
            slotProps={{ htmlInput: { autoComplete: "off", autoCorrect: "off", autoCapitalize: "none", spellCheck: false, style: { fontFamily: "monospace", fontSize: "0.85rem" } } }}
          />
          {error && <Alert severity="error">{error}</Alert>}
          <Box sx={{ display: "flex", justifyContent: "flex-end", gap: 1, pt: 0.5 }}>
            <Button variant="outlined" onClick={onBack}>キャンセル</Button>
            <Button type="submit" variant="contained" disabled={submitting || serviceNameError || !serviceName || !form.image.trim()}>
              {submitting ? "作成中..." : "作成"}
            </Button>
          </Box>
        </Box>
      </Paper>
    </Box>
  );
}
