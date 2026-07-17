import AddIcon from "@mui/icons-material/Add";
import DeleteOutlinedIcon from "@mui/icons-material/DeleteOutlined";
import { Box, Button, CircularProgress, IconButton, Radio, Tooltip, Typography } from "@mui/material";
import type { Project } from "../types";
import { PageHeader, DataTable } from "./primitives";
import type { Column } from "./primitives";

type HomeSectionProps = {
  activeProjectId: string;
  deletingProjectId: string;
  onRequestDeleteProject: (projectId: string, projectName: string) => void;
  onSelectProject: (projectId: string) => void;
  onOpenProjectCreate: () => void;
  projects: Project[];
};

export function HomeSection({
  activeProjectId, deletingProjectId,
  onRequestDeleteProject, onSelectProject, onOpenProjectCreate, projects,
}: HomeSectionProps) {

  const columns: Column<Project>[] = [
    {
      key: "select", header: "", width: 44,
      render: (p) => {
        const isDeleting = p.deleting === true || deletingProjectId === p.id;
        return (
          <Radio
            size="small"
            checked={p.id === activeProjectId}
            disabled={isDeleting}
            onChange={() => onSelectProject(p.id)}
            value={p.id} name="project-select"
          />
        );
      }
    },
    {
      key: "name", header: "名前",
      render: (p) => {
        const isDeleting = p.deleting === true || deletingProjectId === p.id;
        return (
          <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
            <Typography variant="body2" sx={{ fontWeight: 500 }}>{p.name}</Typography>
            {isDeleting && <Typography variant="caption" color="error.main" sx={{ fontWeight: 500 }}>削除中</Typography>}
          </Box>
        );
      }
    },
    {
      key: "id", header: "ID",
      render: (p) => (
        <Typography variant="caption" color="text.secondary" sx={{ fontFamily: "monospace", wordBreak: "break-all" }}>
          {p.id}
        </Typography>
      )
    },
    {
      key: "actions", header: "", width: 60, align: "right",
      render: (p) => {
        const isDeleting = p.deleting === true || deletingProjectId === p.id;
        return (
          <Tooltip title={isDeleting ? "削除中" : "削除"}>
            <span>
              <IconButton
                size="small" color="error"
                disabled={isDeleting}
                onClick={() => onRequestDeleteProject(p.id, p.name)}
              >
                {isDeleting ? <CircularProgress size={14} /> : <DeleteOutlinedIcon fontSize="small" />}
              </IconButton>
            </span>
          </Tooltip>
        );
      }
    }
  ];

  return (
    <Box>
      <PageHeader
        title="プロジェクト"
        subtitle="操作対象のプロジェクトを選択・作成・削除します"
        actions={
          <Button variant="contained" startIcon={<AddIcon />} onClick={onOpenProjectCreate}>
            作成
          </Button>
        }
      />
      <DataTable
        columns={columns}
        rows={projects}
        rowKey={(p) => p.id}
        emptyMessage="まだプロジェクトがありません。「作成」から始めましょう。"
      />
    </Box>
  );
}
