import ArrowBackIcon from "@mui/icons-material/ArrowBack";
import LinkIcon from "@mui/icons-material/Link";
import { Alert, Box, Button, Card, CardContent, Divider, TextField, Typography } from "@mui/material";
import type { FormEvent } from "react";

type RepositoryConfig = {
  projectId: string;
  userId: string;
  repositoryOwner: string;
  repositoryName: string;
  repositoryBranch: string;
  connectedAt: string;
  updatedAt: string;
};

type RepositoryForm = {
  repositoryOwner: string;
  repositoryName: string;
  repositoryBranch: string;
};

type RepositorySectionProps = {
  error: string;
  loading: boolean;
  saving: boolean;
  form: RepositoryForm;
  config: RepositoryConfig | null;
  onBack: () => void;
  onChange: (patch: Partial<RepositoryForm>) => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
};

export function RepositorySection({
  error,
  loading,
  saving,
  form,
  config,
  onBack,
  onChange,
  onSubmit
}: RepositorySectionProps) {
  return (
    <Box sx={{ display: "flex", justifyContent: "center" }}>
      <Card variant="outlined" sx={{ width: "100%", maxWidth: 880, borderRadius: 2 }}>
        <CardContent sx={{ p: { xs: 2.5, sm: 3 }, display: "grid", gap: 2.5 }}>
          <Box sx={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 2, flexWrap: "wrap" }}>
            <Box sx={{ display: "grid", gap: 0.5 }}>
              <Typography variant="h5" sx={{ fontWeight: 700 }}>
                リポジトリを接続
              </Typography>
              <Typography color="text.secondary">
                プロジェクトに GitHub リポジトリ設定を保存します。
              </Typography>
            </Box>
            <Button startIcon={<ArrowBackIcon />} onClick={onBack} variant="outlined">
              戻る
            </Button>
          </Box>

          <Divider />

          {config ? (
            <Alert severity="success" icon={<LinkIcon fontSize="inherit" />}>
              {`${config.repositoryOwner}/${config.repositoryName} (${config.repositoryBranch}) が接続済みです`}
            </Alert>
          ) : null}

          <Box component="form" onSubmit={onSubmit} sx={{ display: "grid", gap: 2.25 }}>
            <Box sx={{ display: "grid", gap: 2, gridTemplateColumns: { xs: "1fr", md: "repeat(2, minmax(0, 1fr))" } }}>
              <TextField
                label="リポジトリオーナー"
                value={form.repositoryOwner}
                onChange={(event) => onChange({ repositoryOwner: event.target.value })}
                placeholder="octocat"
                fullWidth
              />
              <TextField
                label="リポジトリ名"
                value={form.repositoryName}
                onChange={(event) => onChange({ repositoryName: event.target.value })}
                placeholder="my-app"
                fullWidth
              />
            </Box>
            <TextField
              label="ブランチ"
              value={form.repositoryBranch}
              onChange={(event) => onChange({ repositoryBranch: event.target.value })}
              placeholder="main"
              fullWidth
            />

            {config ? (
              <Box sx={{ display: "grid", gap: 0.5, color: "text.secondary", fontSize: 14 }}>
                <Box>接続時刻: {loading ? "読み込み中..." : config.connectedAt}</Box>
                <Box>更新時刻: {loading ? "読み込み中..." : config.updatedAt}</Box>
              </Box>
            ) : null}

            <Box sx={{ display: "flex", justifyContent: "flex-end", gap: 1.25, pt: 0.5 }}>
              <Button variant="outlined" onClick={onBack}>
                キャンセル
              </Button>
              <Button type="submit" variant="contained" disabled={saving}>
                {saving ? "保存中..." : "接続を保存"}
              </Button>
            </Box>
          </Box>

          {error ? <Alert severity="error">{error}</Alert> : null}
        </CardContent>
      </Card>
    </Box>
  );
}
