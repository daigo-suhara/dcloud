import { Alert, Box, Button, Container, Paper, Tab, Tabs, TextField, Typography } from "@mui/material";
import CloudQueueOutlinedIcon from "@mui/icons-material/CloudQueueOutlined";
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
    <Box sx={{ minHeight: "100vh", display: "grid", placeItems: "center", bgcolor: "background.default" }}>
      <Container maxWidth="xs">
        <Box sx={{ display: "flex", alignItems: "center", justifyContent: "center", gap: 1, mb: 3 }}>
          <CloudQueueOutlinedIcon sx={{ fontSize: 28, color: "primary.main" }} />
          <Typography variant="h4" sx={{ letterSpacing: "-0.02em" }}>
            dcloud
          </Typography>
        </Box>

        <Paper variant="outlined" sx={{ p: 3 }}>
          <Tabs
            value={mode}
            onChange={(_, value: "login" | "register") => setMode(value)}
            variant="fullWidth"
            sx={{ mb: 2, borderBottom: 1, borderColor: "divider" }}
          >
            <Tab value="login" label="ログイン" />
            <Tab value="register" label="アカウント作成" />
          </Tabs>

          <Box
            component="form"
            onSubmit={mode === "login" ? onLogin : handleRegisterSubmit}
            sx={{ display: "grid", gap: 2 }}
          >
            <TextField
              label="メールアドレス" type="email" value={form.email}
              onChange={(e) => onChange({ email: e.target.value })}
              autoComplete="email" fullWidth autoFocus
            />
            <TextField
              label="パスワード" type="password" value={form.password}
              onChange={(e) => onChange({ password: e.target.value })}
              autoComplete={mode === "login" ? "current-password" : "new-password"} fullWidth
            />
            {error && <Alert severity="error">{error}</Alert>}
            <Button type="submit" variant="contained" size="large" disabled={loading}>
              {mode === "login" ? (loading ? "ログイン中..." : "ログイン") : (loading ? "作成中..." : "アカウント作成")}
            </Button>
          </Box>
        </Paper>
      </Container>
    </Box>
  );
}
