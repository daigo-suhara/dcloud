import LogoutIcon from "@mui/icons-material/Logout";
import MenuIcon from "@mui/icons-material/Menu";
import { Box, Button, IconButton, MenuItem, TextField, Toolbar, AppBar } from "@mui/material";
import type { Project } from "../../types";
import { Brand } from "./Brand";

type HeaderProps = {
  activeProjectId: string;
  hasProjects: boolean;
  onLogout: () => void;
  onProjectSelect: (projectId: string) => void;
  onToggleSidebar: () => void;
  projects: Project[];
};

export function Header({
  activeProjectId, hasProjects, onLogout, onProjectSelect, onToggleSidebar, projects
}: HeaderProps) {
  return (
    <AppBar
      position="sticky" color="transparent" elevation={0}
      sx={{ bgcolor: "background.paper", borderBottom: 1, borderColor: "divider" }}
    >
      <Toolbar sx={{ display: "flex", alignItems: "center", gap: 1.5, minHeight: 56 }}>
        <IconButton onClick={onToggleSidebar} aria-label="navigation" size="small" sx={{ mr: 0.5 }}>
          <MenuIcon />
        </IconButton>
        <Brand />
        <Box sx={{ flex: 1 }} />
        <TextField
          select size="small" value={activeProjectId}
          onChange={(e) => onProjectSelect(e.target.value)}
          disabled={!hasProjects}
          sx={{ minWidth: 200, display: { xs: "none", sm: "inline-flex" } }}
          slotProps={{ htmlInput: { "aria-label": "プロジェクトを切り替え" } }}
        >
          {projects.map((project) => (
            <MenuItem key={project.id} value={project.id} disabled={project.deleting === true}>
              {project.name}{project.deleting ? " (削除中)" : ""}
            </MenuItem>
          ))}
        </TextField>
        <Button
          variant="outlined" size="small" startIcon={<LogoutIcon />}
          onClick={onLogout}
          sx={{ whiteSpace: "nowrap", display: { xs: "none", sm: "inline-flex" } }}
        >
          ログアウト
        </Button>
      </Toolbar>
    </AppBar>
  );
}
