import ArrowBackIcon from "@mui/icons-material/ArrowBack";
import LinkIcon from "@mui/icons-material/Link";
import { Alert, Box, Button, Paper, TextField, Typography } from "@mui/material";
import type { FormEvent } from "react";
import { PageHeader } from "./primitives";

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
  error, loading, saving, form, config, onBack, onChange, onSubmit
}: RepositorySectionProps) {
  return (
    <Box sx={{ maxWidth: 880, mx: "auto" }}>
      <PageHeader
        title="リポジトリを接続"
        subtitle="プロジェクトに GitHub リポジトリを紐付けます"
        actions={<Button startIcon={<ArrowBackIcon />} onClick={onBack}>戻る</Button>}
      />

      <Paper variant="outlined" sx={{ p: 2.5 }}>
        {config && (
          <Alert severity="success" icon={<LinkIcon fontSize="inherit" />} sx={{ mb: 2 }}>
            {`${config.repositoryOwner}/${config.repositoryName} (${config.repositoryBranch}) が接続済みです`}
          </Alert>
        )}

        <Box component="form" onSubmit={onSubmit} sx={{ display: "grid", gap: 2 }}>
          <Box sx={{ display: "grid", gap: 2, gridTemplateColumns: { xs: "1fr", md: "repeat(2, minmax(0, 1fr))" } }}>
            <TextField
              label="リポジトリオーナー" value={form.repositoryOwner}
              onChange={(e) => onChange({ repositoryOwner: e.target.value })}
              placeholder="octocat" fullWidth
            />
            <TextField
              label="リポジトリ名" value={form.repositoryName}
              onChange={(e) => onChange({ repositoryName: e.target.value })}
              placeholder="my-app" fullWidth
            />
          </Box>
          <TextField
            label="ブランチ" value={form.repositoryBranch}
            onChange={(e) => onChange({ repositoryBranch: e.target.value })}
            placeholder="main" fullWidth
          />

          {config && (
            <Box sx={{ display: "grid", gap: 0.5 }}>
              <Typography variant="caption" color="text.secondary">
                接続時刻: {loading ? "読み込み中..." : config.connectedAt}
              </Typography>
              <Typography variant="caption" color="text.secondary">
                更新時刻: {loading ? "読み込み中..." : config.updatedAt}
              </Typography>
            </Box>
          )}

          {error && <Alert severity="error">{error}</Alert>}

          <Box sx={{ display: "flex", justifyContent: "flex-end", gap: 1, pt: 0.5 }}>
            <Button variant="outlined" onClick={onBack}>キャンセル</Button>
            <Button type="submit" variant="contained" disabled={saving}>
              {saving ? "保存中..." : "接続を保存"}
            </Button>
          </Box>
        </Box>
      </Paper>
    </Box>
  );
}
