import { alpha } from "@mui/material/styles";
import DeleteOutlinedIcon from "@mui/icons-material/DeleteOutlined";
import { Box, Button, Card, CardContent, CircularProgress, IconButton, Paper, Radio, Table, TableBody, TableCell, TableHead, TableRow, Tooltip, Typography } from "@mui/material";
import type { Project } from "../types";

type HomeSectionProps = {
  activeProjectId: string;
  deletingProjectId: string;
  onRequestDeleteProject: (projectId: string, projectName: string) => void;
  onSelectProject: (projectId: string) => void;
  onOpenProjectCreate: () => void;
  projects: Project[];
};

export function HomeSection({
  activeProjectId,
  deletingProjectId,
  onRequestDeleteProject,
  onSelectProject,
  onOpenProjectCreate,
  projects,
}: HomeSectionProps) {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Card variant="outlined" sx={{ borderRadius: 2 }}>
        <CardContent sx={{ p: 3, display: "grid", gap: 2 }}>
          <Box sx={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", gap: 2, flexWrap: "wrap" }}>
            <Typography variant="h5" sx={{ fontWeight: 700 }}>
              プロジェクト管理
            </Typography>
            <Button variant="contained" onClick={onOpenProjectCreate}>
              プロジェクトを作成
            </Button>
          </Box>

          {projects.length === 0 ? (
            <Paper variant="outlined" sx={{ p: 2, borderRadius: 2, borderStyle: "dashed", bgcolor: alpha("#ffffff", 0.7) }}>
              <Typography color="text.secondary">まだプロジェクトがありません。上のボタンから作成画面へ進んでください。</Typography>
            </Paper>
          ) : (
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
                    const isDeleting = project.deleting === true || deletingProjectId === project.id;
                    return (
                      <TableRow
                        key={project.id}
                        hover={!isDeleting}
                        selected={isActive}
                        sx={{
                          minHeight: { xs: 40, sm: "auto" },
                          opacity: isDeleting ? 0.55 : 1,
                          "& td": {
                            bgcolor: isActive ? alpha("#2563eb", 0.04) : "background.paper"
                          }
                        }}
                      >
                        <TableCell sx={{ py: { xs: 0.5, sm: 1.25 }, pl: { xs: 0.25, sm: 1 } }}>
                          <Radio checked={isActive} disabled={isDeleting} onChange={() => onSelectProject(project.id)} value={project.id} name="project-select" size="small" />
                        </TableCell>
                        <TableCell sx={{ py: { xs: 0.5, sm: 1.25 }, pl: { xs: 0, sm: 1.5 } }}>
                          <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                            <Typography sx={{ fontWeight: 700 }}>{project.name}</Typography>
                            {isDeleting && <Typography variant="caption" color="error.main" sx={{ fontWeight: 600 }}>削除中</Typography>}
                          </Box>
                        </TableCell>
                        <TableCell sx={{ display: { xs: "none", sm: "table-cell" }, py: 1.25 }}>
                          <Typography variant="body2" color="text.secondary" sx={{ wordBreak: "break-all" }}>
                            {project.id}
                          </Typography>
                        </TableCell>
                        <TableCell sx={{ py: { xs: 0.5, sm: 1.25 }, pr: { xs: 0.5, sm: 1 } }}>
                          <Tooltip title={isDeleting ? "削除中" : "削除"}>
                            <span>
                              <IconButton
                                color="error"
                                disabled={isDeleting}
                                onClick={() => onRequestDeleteProject(project.id, project.name)}
                                size="small"
                                sx={{
                                  border: "1px solid",
                                  borderColor: "error.main",
                                  bgcolor: "error.main",
                                  color: "common.white",
                                  "&:hover": {
                                    bgcolor: "error.dark",
                                    borderColor: "error.dark"
                                  },
                                  "&.Mui-disabled": {
                                    bgcolor: "rgba(220, 38, 38, 0.08)",
                                    color: "error.main",
                                    borderColor: "rgba(220, 38, 38, 0.2)"
                                  }
                                }}
                              >
                                {isDeleting ? <CircularProgress size={14} thickness={5} sx={{ color: "inherit" }} /> : <DeleteOutlinedIcon fontSize="small" />}
                              </IconButton>
                            </span>
                          </Tooltip>
                        </TableCell>
                      </TableRow>
                    );
                  })}
                </TableBody>
              </Table>
            </Box>
          )}
        </CardContent>
      </Card>
    </Box>
  );
}
