import ArrowBackIcon from "@mui/icons-material/ArrowBack";
import { Box, Button, Paper, TextField } from "@mui/material";
import type { FormEvent } from "react";
import { PageHeader } from "./primitives";

type ProjectCreateSectionProps = {
  creatingProject: boolean;
  hasProjects: boolean;
  onBack: () => void;
  onCreateProject: (event: FormEvent<HTMLFormElement>) => void;
  onProjectNameChange: (value: string) => void;
  projectName: string;
};

export function ProjectCreateSection({
  creatingProject, hasProjects, onBack, onCreateProject, onProjectNameChange, projectName
}: ProjectCreateSectionProps) {
  const normalizedName = projectName.trim();
  const isValidName = /^[a-z0-9-]+$/.test(normalizedName) && !normalizedName.startsWith("-") && !normalizedName.endsWith("-");
  const showError = normalizedName.length > 0 && !isValidName;

  return (
    <Box sx={{ maxWidth: 720, mx: "auto" }}>
      <PageHeader
        title="プロジェクトを作成"
        actions={hasProjects ? (
          <Button startIcon={<ArrowBackIcon />} onClick={onBack}>戻る</Button>
        ) : undefined}
      />

      <Paper variant="outlined" sx={{ p: 3 }}>
        <Box component="form" onSubmit={onCreateProject} sx={{ display: "grid", gap: 2 }}>
          <TextField
            autoFocus
            label="プロジェクト名"
            value={projectName}
            onChange={(event) => onProjectNameChange(event.target.value)}
            placeholder="新しいプロジェクト"
            error={showError}
            helperText={showError ? "英小文字・数字・ハイフンのみ" : "英小文字・数字・ハイフンで指定してください"}
            fullWidth
          />
          <Box sx={{ display: "flex", justifyContent: "flex-end" }}>
            <Button
              type="submit" variant="contained"
              disabled={creatingProject || !normalizedName || !isValidName}
            >
              {creatingProject ? "作成中..." : "作成"}
            </Button>
          </Box>
        </Box>
      </Paper>
    </Box>
  );
}
