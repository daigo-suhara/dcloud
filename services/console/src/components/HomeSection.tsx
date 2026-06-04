import { alpha } from "@mui/material/styles";
import AddOutlinedIcon from "@mui/icons-material/AddOutlined";
import DeleteOutlinedIcon from "@mui/icons-material/DeleteOutlined";
import { Box, Button, Card, CardContent, Paper, Radio, Table, TableBody, TableCell, TableHead, TableRow, TextField, Typography } from "@mui/material";
import type { FormEvent } from "react";
import type { Project } from "../types";

type HomeSectionProps = {
  activeProjectId: string;
  creatingProject: boolean;
  deletingProjectId: string;
  onCreateProject: (event: FormEvent<HTMLFormElement>) => void;
  onProjectNameChange: (value: string) => void;
  onRequestDeleteProject: (projectId: string) => void;
  onSelectProject: (projectId: string) => void;
  onToggleCreateForm: () => void;
  projectName: string;
  projects: Project[];
  showProjectCreateForm: boolean;
};

export function HomeSection({
  activeProjectId,
  creatingProject,
  deletingProjectId,
  onCreateProject,
  onProjectNameChange,
  onRequestDeleteProject,
  onSelectProject,
  onToggleCreateForm,
  projectName,
  projects,
  showProjectCreateForm
}: HomeSectionProps) {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Card variant="outlined" sx={{ borderRadius: 2 }}>
        <CardContent sx={{ p: 3, display: "grid", gap: 2 }}>
          <Box sx={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", gap: 2, flexWrap: "wrap" }}>
            <Typography variant="h5" sx={{ fontWeight: 700 }}>
              プロジェクト管理
            </Typography>
            <Button variant="contained" startIcon={<AddOutlinedIcon />} onClick={onToggleCreateForm}>
              プロジェクトを作成
            </Button>
          </Box>

          {showProjectCreateForm ? (
            <Box component="form" onSubmit={onCreateProject} sx={{ display: "grid", gap: 2, maxWidth: 560 }}>
              <TextField
                label="プロジェクト名"
                value={projectName}
                onChange={(event) => onProjectNameChange(event.target.value)}
                placeholder="新しいプロジェクト"
                fullWidth
              />
              <Box sx={{ display: "flex", justifyContent: "flex-end" }}>
                <Button type="submit" variant="contained" disabled={creatingProject || !projectName.trim()}>
                  作成
                </Button>
              </Box>
            </Box>
          ) : null}

          <Box sx={{ display: "grid", gap: 1 }}>
            <Table size="small" sx={{ "& .MuiTableCell-root": { borderBottomColor: "rgba(148, 163, 184, 0.18)" } }}>
              <TableHead>
                <TableRow>
                  <TableCell sx={{ width: { xs: 44, sm: 56 } }} />
                  <TableCell sx={{ fontSize: 12, fontWeight: 700, color: "text.secondary" }}>名前</TableCell>
                  <TableCell sx={{ display: { xs: "none", sm: "table-cell" }, fontSize: 12, fontWeight: 700, color: "text.secondary" }}>ID</TableCell>
                  <TableCell sx={{ width: { xs: 92, sm: 120 } }} />
                </TableRow>
              </TableHead>
              <TableBody>
                {projects.map((project) => {
                  const isActive = project.id === activeProjectId;
                  const canDelete = project.name !== "default";
                  return (
                    <TableRow
                      key={project.id}
                      hover
                      selected={isActive}
                      sx={{
                        minHeight: { xs: 40, sm: "auto" },
                        "& td": {
                          bgcolor: isActive ? alpha("#2563eb", 0.04) : "background.paper"
                        }
                      }}
                    >
                      <TableCell sx={{ py: { xs: 0.5, sm: 1.25 }, pl: { xs: 0.25, sm: 1 } }}>
                        <Radio checked={isActive} onChange={() => onSelectProject(project.id)} value={project.id} name="project-select" size="small" />
                      </TableCell>
                      <TableCell sx={{ py: { xs: 0.5, sm: 1.25 }, pl: { xs: 0, sm: 1.5 } }}>
                        <Typography sx={{ fontWeight: 700 }}>{project.name}</Typography>
                      </TableCell>
                      <TableCell sx={{ display: { xs: "none", sm: "table-cell" }, py: 1.25 }}>
                        <Typography variant="body2" color="text.secondary" sx={{ wordBreak: "break-all" }}>
                          {project.id}
                        </Typography>
                      </TableCell>
                      <TableCell sx={{ py: { xs: 0.5, sm: 1.25 }, pr: { xs: 0.5, sm: 1 } }}>
                        <Button
                          variant="outlined"
                          color="inherit"
                          startIcon={<DeleteOutlinedIcon />}
                          disabled={!canDelete || deletingProjectId === project.id}
                          onClick={() => onRequestDeleteProject(project.id)}
                          fullWidth
                          size="small"
                          sx={{ whiteSpace: "nowrap" }}
                        >
                          {deletingProjectId === project.id ? "削除中..." : "削除"}
                        </Button>
                      </TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          </Box>
        </CardContent>
      </Card>
    </Box>
  );
}
