import ArrowBackIcon from "@mui/icons-material/ArrowBack";
import { Alert, Box, Button, Card, CardContent, Divider, TextField, Typography } from "@mui/material";
import type { FormEvent } from "react";
import type { DeployForm } from "../types";
import { actionLinkButtonSx } from "../theme";

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
    onChange({
      name: "hello",
      image: "ghcr.io/daigo-suhara/hello-world:latest"
    });
  }
  return (
    <Card variant="outlined" sx={{ borderRadius: 2, maxWidth: 1120, width: "100%" }}>
      <CardContent sx={{ p: { xs: 2.5, sm: 3 }, display: "grid", gap: 2.5 }}>
        <Box sx={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 2, flexWrap: "wrap" }}>
          <Box sx={{ display: "grid", gap: 0.75 }}>
            <Typography variant="h5" sx={{ fontWeight: 700, lineHeight: 1.1 }}>
              コンテナを作成
            </Typography>
          </Box>
          <Button startIcon={<ArrowBackIcon />} onClick={onBack} variant="outlined">
            一覧に戻る
          </Button>
        </Box>

        <Box sx={{ display: "grid", gap: 2.5 }}>
          <Card variant="outlined" sx={{ borderRadius: 2 }}>
            <CardContent sx={{ p: { xs: 2.5, sm: 3 }, display: "grid", gap: 2.25 }}>
              <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 2, flexWrap: "wrap" }}>
                <Typography variant="h6" sx={{ fontWeight: 700 }}>
                  デプロイ設定
                </Typography>
                <Button type="button" variant="text" size="small" onClick={fillTestImage} sx={actionLinkButtonSx}>
                  サンプルコンテナを使用
                </Button>
              </Box>

              <Divider />

              <Box component="form" onSubmit={onSubmit} sx={{ display: "grid", gap: 2.25 }}>
                <TextField
                  label="サービス名"
                  value={form.name}
                  onChange={(event) => onChange({ name: event.target.value })}
                  placeholder="service-name"
                  error={serviceNameError}
                  helperText={serviceNameError ? "英小文字・数字・ハイフンのみ" : ""}
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
                  label="コンテナイメージ"
                  value={form.image}
                  onChange={(event) => onChange({ image: event.target.value })}
                  placeholder="ghcr.io/org/app:tag"
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

                <TextField
                  label="Port"
                  type="number"
                  slotProps={{ htmlInput: { min: 1, max: 65535 } }}
                  value={form.port}
                  onChange={(event) => onChange({ port: event.target.value })}
                  placeholder="8080"
                  helperText=""
                  fullWidth
                />

                <Box sx={{ display: "grid", gap: 2, gridTemplateColumns: { xs: "1fr", md: "repeat(2, minmax(0, 1fr))" } }}>
                  <TextField
                    label="最小スケール"
                    type="number"
                    slotProps={{ htmlInput: { min: 0, max: 20 } }}
                    value={form.minScale}
                    onChange={(event) => onChange({ minScale: event.target.value })}
                    placeholder="0"
                    helperText=""
                    fullWidth
                  />
                  <TextField
                    label="最大スケール"
                    type="number"
                    slotProps={{ htmlInput: { min: 1, max: 20 } }}
                    value={form.maxScale}
                    onChange={(event) => onChange({ maxScale: event.target.value })}
                    placeholder="1"
                    helperText=""
                    fullWidth
                  />
                </Box>

                <TextField
                  label="起動スクリプト（任意）"
                  value={form.startupScript}
                  onChange={(event) => onChange({ startupScript: event.target.value })}
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

                <Box sx={{ display: "flex", flexWrap: "wrap", justifyContent: "flex-end", gap: 1.25, pt: 0.5 }}>
                  <Button variant="outlined" onClick={onBack}>
                    キャンセル
                  </Button>
                  <Button type="submit" variant="contained" disabled={submitting || serviceNameError}>
                    {submitting ? "作成中..." : "作成"}
                  </Button>
                </Box>
              </Box>

              {error ? <Alert severity="error">{error}</Alert> : null}
            </CardContent>
          </Card>
        </Box>
      </CardContent>
    </Card>
  );
}
