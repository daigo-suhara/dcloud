import { Alert, Box, Button, Card, CardContent, Container, Divider, TextField, Typography } from "@mui/material";
import CloudQueueOutlinedIcon from "@mui/icons-material/CloudQueueOutlined";
import type { FormEvent } from "react";
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
  return (
    <Box sx={{ minHeight: "100vh" }}>
      <Container maxWidth="sm" sx={{ minHeight: "100vh", display: "grid", placeItems: "center", py: 4 }}>
        <Card variant="outlined" sx={{ width: "100%", overflow: "hidden", boxShadow: "0 18px 36px rgba(15, 23, 42, 0.10)" }}>
          <CardContent sx={{ p: { xs: 3, sm: 4 } }}>
            <Box sx={{ display: "grid", gap: 2.5 }}>
              <Box sx={{ display: "flex", alignItems: "center", gap: 1.25 }}>
                <CloudQueueOutlinedIcon sx={{ fontSize: 28, color: "primary.main" }} />
                <Typography variant="h6" sx={{ fontWeight: 700, lineHeight: 1 }}>
                  DCloud Console
                </Typography>
              </Box>

              <Box sx={{ display: "grid", gap: 0.75 }}>
                <Typography variant="h4" sx={{ fontWeight: 700, lineHeight: 1.15 }}>
                  ログイン
                </Typography>
                <Typography color="text.secondary">
                  自前の identity で認証します。
                </Typography>
              </Box>

              <Box component="form" onSubmit={onLogin} sx={{ display: "grid", gap: 1.5 }}>
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

                <Box sx={{ display: "flex", flexDirection: { xs: "column", sm: "row" }, gap: 1.5, pt: 0.5 }}>
                  <Button type="submit" variant="contained" size="large" disabled={loading} fullWidth>
                    ログイン
                  </Button>
                  <Button type="button" variant="outlined" size="large" onClick={onRegister} disabled={loading} fullWidth>
                    アカウント作成
                  </Button>
                </Box>
              </Box>

              <Divider />

              {error ? <Alert severity="error">{error}</Alert> : null}
            </Box>
          </CardContent>
        </Card>
      </Container>
    </Box>
  );
}
