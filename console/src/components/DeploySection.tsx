import ArrowBackIcon from "@mui/icons-material/ArrowBack";
import { Alert, Box, Button, Paper, TextField, Typography } from "@mui/material";
import type { FormEvent } from "react";
import type { DeployForm } from "../types";
import { actionLinkButtonSx, monoFontFamily } from "../theme";
import { EnvVarEditor } from "./EnvVarEditor";
import { BucketVolumesEditor } from "./BucketVolumesEditor";
import { PageHeader, FormRow } from "./primitives";

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
  activeProjectId: string;
};

export function DeploySection({ error, form, onBack, onChange, onSubmit, submitting, activeProjectId }: DeploySectionProps) {
  const serviceName = form.name.trim();
  const serviceNameError = serviceName.length > 0 && !isDnsLabel(serviceName);

  function fillTestImage() {
    onChange({ name: "hello", image: "gcr.io/knative-samples/helloworld-go:latest" });
  }

  return (
    <Box>
      <PageHeader
        title="コンテナをデプロイ"
        actions={
          <>
            <Button type="button" size="small" onClick={fillTestImage} sx={actionLinkButtonSx}>
              サンプル入力
            </Button>
            <Button startIcon={<ArrowBackIcon />} onClick={onBack}>一覧に戻る</Button>
          </>
        }
      />

      <Paper variant="outlined" component="form" onSubmit={onSubmit}>
        <FormRow label="サービス名">
          <TextField
            value={form.name}
            onChange={(e) => onChange({ name: e.target.value })}
            placeholder="service-name"
            error={serviceNameError}
            helperText={serviceNameError ? "英小文字・数字・ハイフンのみ" : " "}
            slotProps={{ htmlInput: { autoCapitalize: "none", autoComplete: "off", autoCorrect: "off", inputMode: "text", maxLength: 63, pattern: "[a-z0-9]([a-z0-9-]*[a-z0-9])?" } }}
            fullWidth
          />
        </FormRow>

        <FormRow label="コンテナイメージ">
          <TextField
            value={form.image}
            onChange={(e) => onChange({ image: e.target.value })}
            placeholder="ghcr.io/org/app:tag" fullWidth
            slotProps={{ htmlInput: { autoComplete: "off", autoCorrect: "off", autoCapitalize: "none", spellCheck: false } }}
          />
        </FormRow>

        <FormRow label="ポート">
          <TextField type="number"
            slotProps={{ htmlInput: { min: 1, max: 65535 } }}
            value={form.port} onChange={(e) => onChange({ port: e.target.value })}
            placeholder="8080" sx={{ maxWidth: 160 }} />
        </FormRow>

        <FormRow label="スケール">
          <Box sx={{ display: "flex", gap: 1, alignItems: "center" }}>
            <TextField label="最小" type="number"
              slotProps={{ htmlInput: { min: 0, max: 20 } }}
              value={form.minScale} onChange={(e) => onChange({ minScale: e.target.value })}
              sx={{ width: 140 }} />
            <Typography color="text.secondary" variant="body2">〜</Typography>
            <TextField label="最大" type="number"
              slotProps={{ htmlInput: { min: 1, max: 20 } }}
              value={form.maxScale} onChange={(e) => onChange({ maxScale: e.target.value })}
              sx={{ width: 140 }} />
          </Box>
        </FormRow>

        <FormRow label="環境変数">
          <EnvVarEditor value={form.env} onChange={(env) => onChange({ env })} size="small" />
        </FormRow>

        <FormRow label="ボリューム">
          <BucketVolumesEditor
            value={form.bucketVolumes}
            onChange={(bucketVolumes) => onChange({ bucketVolumes })}
            projectId={activeProjectId}
          />
        </FormRow>

        <FormRow label="起動スクリプト">
          <TextField
            value={form.startupScript}
            onChange={(e) => onChange({ startupScript: e.target.value })}
            placeholder={"#!/bin/sh\nexec code-server --bind-addr 0.0.0.0:8080 --auth none ."}
            multiline minRows={3} fullWidth
            slotProps={{ htmlInput: { autoComplete: "off", autoCorrect: "off", autoCapitalize: "none", spellCheck: false, style: { fontFamily: monoFontFamily, fontSize: 13 } } }}
          />
        </FormRow>

        {error && <Box sx={{ px: 3, pt: 2 }}><Alert severity="error">{error}</Alert></Box>}

        <Box sx={{ display: "flex", justifyContent: "flex-end", gap: 1, borderTop: 1, borderColor: "divider", p: 2 }}>
          <Button variant="outlined" onClick={onBack}>キャンセル</Button>
          <Button type="submit" variant="contained" disabled={submitting || serviceNameError || !serviceName || !form.image.trim()}>
            {submitting ? "作成中..." : "作成"}
          </Button>
        </Box>
      </Paper>
    </Box>
  );
}
