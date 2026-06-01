import LogoutIcon from "@mui/icons-material/Logout";
import MenuIcon from "@mui/icons-material/Menu";
import { Box, Button, IconButton, MenuItem, TextField, Toolbar, AppBar } from "@mui/material";
import { alpha } from "@mui/material/styles";
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

export function Header({ activeProjectId, hasProjects, onLogout, onProjectSelect, onToggleSidebar, projects }: HeaderProps) {
  return (
    <AppBar
      position="sticky"
      color="transparent"
      elevation={0}
      sx={{
        backdropFilter: "blur(18px)",
        backgroundColor: alpha("#ffffff", 0.78),
        borderBottom: "1px solid rgba(148, 163, 184, 0.18)"
      }}
    >
      <Toolbar sx={{ gap: 1.5, minHeight: 64 }}>
        <IconButton edge="start" onClick={onToggleSidebar} aria-label="navigation">
          <MenuIcon />
        </IconButton>
        <Brand />
        <Box sx={{ flex: 1 }} />
        <TextField
          select
          size="small"
          value={activeProjectId}
          onChange={(event) => onProjectSelect(event.target.value)}
          disabled={!hasProjects}
          sx={{ minWidth: { xs: 120, sm: 220 }, bgcolor: "background.paper" }}
          slotProps={{ htmlInput: { "aria-label": "プロジェクトを切り替え" } }}
        >
          {projects.map((project) => (
            <MenuItem key={project.id} value={project.id}>
              {project.name}
            </MenuItem>
          ))}
        </TextField>
        <Button variant="outlined" startIcon={<LogoutIcon />} onClick={onLogout} sx={{ whiteSpace: "nowrap" }}>
          ログアウト
        </Button>
      </Toolbar>
    </AppBar>
  );
}

