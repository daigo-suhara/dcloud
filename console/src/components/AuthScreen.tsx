import { Alert, Box, Button, Card, CardContent, Container, Divider, Tab, Tabs, TextField, Typography } from "@mui/material";
import CloudQueueOutlinedIcon from "@mui/icons-material/CloudQueueOutlined";
import KeyOutlinedIcon from "@mui/icons-material/KeyOutlined";
import { useState, type FormEvent } from "react";
import type { AuthForm } from "../types";

type AuthScreenProps = {
  error: string;
  loading: boolean;
  form: AuthForm;
  onChange: (patch: Partial<AuthForm>) => void;
  onLogin: (event: FormEvent<HTMLFormElement>) => void;
  onRegister: () => void;
};

export function AuthScreen({ error, loading, form, onChange, onLogin, onRegister }: AuthScreenProps) {
  const [mode, setMode] = useState<"login" | "register">("login");

  function handleRegisterSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    onRegister();
  }

  return (
    <Box className="auth-page" sx={{ minHeight: "100vh" }}>
      <Container maxWidth="lg" className="auth-shell">
        <Card variant="outlined" className="auth-card auth-hero-card" sx={{ width: "100%", overflow: "hidden" }}>
          <Box
            sx={{
              display: "grid",
              gridTemplateColumns: { xs: "1fr", md: "0.95fr 1.05fr" },
              minHeight: { md: 640 }
            }}
          >
            <Box
              sx={{
                p: { xs: 3, sm: 4, md: 5 },
                background: "linear-gradient(160deg, #0f172a 0%, #1d4ed8 72%, #2563eb 100%)",
                color: "#ffffff",
                display: "grid",
                alignContent: "space-between",
                gap: 4
              }}
            >
              <Box sx={{ display: "grid", gap: 2.25 }}>
                <Box sx={{ display: "flex", alignItems: "center", gap: 1.25 }}>
                  <CloudQueueOutlinedIcon sx={{ fontSize: 30, color: "#ffffff" }} />
                  <Typography variant="h6" sx={{ fontWeight: 800, lineHeight: 1, letterSpacing: "0.02em" }}>
                    DCloud Console
                  </Typography>
                </Box>

                <Box sx={{ display: "grid", gap: 0.7 }}>
                  <Typography variant="h3" sx={{ fontWeight: 800, lineHeight: 1.08, letterSpacing: "-0.03em" }}>
                    DCloud
                  </Typography>
                  <Typography sx={{ maxWidth: 420, color: "rgba(255,255,255,0.78)", fontSize: 15, lineHeight: 1.8 }}>
                    ローカルの identity でログインして、プロジェクトとコンテナを管理します。
                  </Typography>
                </Box>

                <Typography sx={{ color: "rgba(255,255,255,0.66)", fontSize: 12, lineHeight: 1.7 }}>
                  アカウント作成とログインは右側のタブで切り替えます。
                </Typography>
              </Box>

              <Typography sx={{ color: "rgba(255,255,255,0.62)", fontSize: 12, lineHeight: 1.7 }}>
                DCloud Console は外部 auth provider を使わず、identity を直接使う構成です。
              </Typography>
            </Box>

            <CardContent sx={{ p: { xs: 3, sm: 4, md: 5 } }}>
              <Box sx={{ display: "grid", gap: 2.5 }}>
                <Box sx={{ display: "grid", gap: 1.1 }}>
                  <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                    <KeyOutlinedIcon sx={{ fontSize: 22, color: "primary.main" }} />
                    <Typography variant="h5" sx={{ fontWeight: 800, lineHeight: 1.15 }}>
                      identity
                    </Typography>
                  </Box>
                  <Typography color="text.secondary">ログインかアカウント作成を選んでください。</Typography>
                </Box>

                <Box sx={{ borderBottom: 1, borderColor: "divider" }}>
                  <Tabs
                    value={mode}
                    onChange={(_, value: "login" | "register") => setMode(value)}
                    variant="fullWidth"
                  >
                    <Tab value="login" label="ログイン" />
                    <Tab value="register" label="アカウント作成" />
                  </Tabs>
                </Box>

                <Box
                  component="form"
                  onSubmit={mode === "login" ? onLogin : handleRegisterSubmit}
                  sx={{ display: "grid", gap: 1.5 }}
                >
                  <TextField
                    label="ユーザ名"
                    value={form.username}
                    onChange={(event) => onChange({ username: event.target.value })}
                    autoComplete="username"
                    fullWidth
                  />
                  <TextField
                    label="パスワード"
                    type="password"
                    value={form.password}
                    onChange={(event) => onChange({ password: event.target.value })}
                    autoComplete="current-password"
                    fullWidth
                  />

                  {mode === "register" ? (
                    <>
                      <Divider sx={{ my: 0.5 }} />
                      <Box sx={{ display: "grid", gap: 0.6 }}>
                        <Typography variant="subtitle2" sx={{ fontWeight: 700 }}>
                          アカウント作成の追加情報
                        </Typography>
                        <Typography variant="body2" color="text.secondary">
                          作成時だけ入力してください。ログイン時は不要です。
                        </Typography>
                      </Box>
                      <TextField
                        label="メールアドレス"
                        type="email"
                        value={form.email}
                        onChange={(event) => onChange({ email: event.target.value })}
                        autoComplete="email"
                        fullWidth
                      />
                      <TextField
                        label="表示名"
                        value={form.name}
                        onChange={(event) => onChange({ name: event.target.value })}
                        autoComplete="name"
                        fullWidth
                      />
                    </>
                  ) : null}

                  <Box sx={{ display: "flex", flexDirection: { xs: "column", sm: "row" }, gap: 1.5, pt: 0.5 }}>
                    {mode === "login" ? (
                      <Button type="submit" variant="contained" size="large" disabled={loading} fullWidth>
                        ログイン
                      </Button>
                    ) : (
                      <Button type="submit" variant="contained" size="large" disabled={loading} fullWidth>
                        アカウント作成
                      </Button>
                    )}
                  </Box>
                </Box>

                {error ? <Alert severity="error">{error}</Alert> : null}
              </Box>
            </CardContent>
          </Box>
        </Card>
      </Container>
    </Box>
  );
}
