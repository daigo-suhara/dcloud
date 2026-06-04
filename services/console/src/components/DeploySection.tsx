import ArrowBackIcon from "@mui/icons-material/ArrowBack";
import InfoOutlinedIcon from "@mui/icons-material/InfoOutlined";
import { Alert, Box, Button, Card, CardContent, Divider, Paper, Stack, TextField, Typography } from "@mui/material";
import type { FormEvent } from "react";
import type { DeployForm } from "../types";

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
  return (
    <Card variant="outlined" sx={{ borderRadius: 2, maxWidth: 1120, width: "100%" }}>
      <CardContent sx={{ p: { xs: 2.5, sm: 3 }, display: "grid", gap: 2.5 }}>
        <Box sx={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 2, flexWrap: "wrap" }}>
          <Box sx={{ display: "grid", gap: 0.75 }}>
            <Typography variant="h5" sx={{ fontWeight: 700, lineHeight: 1.1 }}>
              コンテナを作成
            </Typography>
            <Typography color="text.secondary">
              イメージ、ポート、スケール範囲をまとめて指定してデプロイします。
            </Typography>
          </Box>
          <Button startIcon={<ArrowBackIcon />} onClick={onBack} variant="outlined">
            一覧に戻る
          </Button>
        </Box>

        <Box sx={{ display: "grid", gap: 2.5, gridTemplateColumns: { xs: "1fr", lg: "minmax(0, 1.3fr) minmax(280px, 0.7fr)" }, alignItems: "start" }}>
          <Card variant="outlined" sx={{ borderRadius: 2 }}>
            <CardContent sx={{ p: { xs: 2.5, sm: 3 }, display: "grid", gap: 2.25 }}>
              <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 2, flexWrap: "wrap" }}>
                <Typography variant="h6" sx={{ fontWeight: 700 }}>
                  デプロイ設定
                </Typography>
                <Typography variant="body2" color="text.secondary">
                  必須項目から順に入力
                </Typography>
              </Box>

              <Divider />

              <Box component="form" onSubmit={onSubmit} sx={{ display: "grid", gap: 2.25 }}>
                <TextField
                  label="サービス名"
                  value={form.name}
                  onChange={(event) => onChange({ name: event.target.value })}
                  placeholder="service-name"
                  error={serviceNameError}
                  helperText={serviceNameError ? "英小文字・数字・ハイフンのみで入力してください" : "DNS 名や一覧に表示される名前です"}
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
                  helperText="レジストリの完全修飾イメージ名を入力します"
                  fullWidth
                />

                <Box sx={{ display: "grid", gap: 2, gridTemplateColumns: { xs: "1fr", md: "repeat(3, minmax(0, 1fr))" } }}>
                  <TextField
                    label="Port"
                    type="number"
                    slotProps={{ htmlInput: { min: 1, max: 65535 } }}
                    value={form.port}
                    onChange={(event) => onChange({ port: event.target.value })}
                    placeholder="8080"
                    helperText="コンテナが待ち受けるポート"
                    fullWidth
                  />
                  <TextField
                    label="最小スケール"
                    type="number"
                    slotProps={{ htmlInput: { min: 0, max: 20 } }}
                    value={form.minScale}
                    onChange={(event) => onChange({ minScale: event.target.value })}
                    placeholder="0"
                    helperText="未使用時の待機数"
                    fullWidth
                  />
                  <TextField
                    label="最大スケール"
                    type="number"
                    slotProps={{ htmlInput: { min: 1, max: 20 } }}
                    value={form.maxScale}
                    onChange={(event) => onChange({ maxScale: event.target.value })}
                    placeholder="1"
                    helperText="必要に応じて増やす上限"
                    fullWidth
                  />
                </Box>

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

          <Paper variant="outlined" sx={{ borderRadius: 2, p: { xs: 2.25, sm: 2.5 }, bgcolor: "grey.50" }}>
            <Stack spacing={2.25}>
              <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                <InfoOutlinedIcon color="primary" fontSize="small" />
                <Typography variant="h6" sx={{ fontWeight: 700 }}>
                  入力ガイド
                </Typography>
              </Box>

              <Typography variant="body2" color="text.secondary">
                まずはイメージ名とポートだけ入れれば動かせます。スケールは初期値のままでも構いません。
              </Typography>

              <Box sx={{ display: "grid", gap: 1.25 }}>
                <Box sx={{ p: 1.5, borderRadius: 2, border: "1px solid rgba(148, 163, 184, 0.24)", bgcolor: "background.paper" }}>
                  <Typography variant="caption" color="text.secondary">
                    推奨イメージ
                  </Typography>
                  <Typography sx={{ fontWeight: 700, mt: 0.25 }}>ghcr.io/org/app:tag</Typography>
                </Box>
                <Box sx={{ p: 1.5, borderRadius: 2, border: "1px solid rgba(148, 163, 184, 0.24)", bgcolor: "background.paper" }}>
                  <Typography variant="caption" color="text.secondary">
                    推奨ポート
                  </Typography>
                  <Typography sx={{ fontWeight: 700, mt: 0.25 }}>8080</Typography>
                </Box>
                <Box sx={{ p: 1.5, borderRadius: 2, border: "1px solid rgba(148, 163, 184, 0.24)", bgcolor: "background.paper" }}>
                  <Typography variant="caption" color="text.secondary">
                    初期スケール
                  </Typography>
                  <Typography sx={{ fontWeight: 700, mt: 0.25 }}>min 0 / max 1</Typography>
                </Box>
              </Box>
            </Stack>
          </Paper>
        </Box>
      </CardContent>
    </Card>
  );
}
