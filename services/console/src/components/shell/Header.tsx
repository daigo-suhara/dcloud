import LogoutIcon from "@mui/icons-material/Logout";
import MenuIcon from "@mui/icons-material/Menu";
import { Box, Button, IconButton, MenuItem, TextField, Toolbar, AppBar } from "@mui/material";
import { alpha } from "@mui/material/styles";
import type { Project } from "../../types";
import { Brand } from "./Brand";
import { shellHeaderRowSx } from "./constants";

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
      <Toolbar
        disableGutters
        sx={{
          ...shellHeaderRowSx,
          flexWrap: "nowrap",
          py: 1
        }}
      >
        <IconButton onClick={onToggleSidebar} aria-label="navigation" sx={{ width: 40, height: 40, p: 0, flex: "0 0 auto" }}>
          <MenuIcon />
        </IconButton>
        <Box sx={{ flex: "1 1 auto", minWidth: 0, display: "flex", alignItems: "center" }}>
          <Brand />
        </Box>
        <TextField
          select
          size="small"
          value={activeProjectId}
          onChange={(event) => onProjectSelect(event.target.value)}
          disabled={!hasProjects}
          sx={{
            minWidth: 0,
            width: { xs: "clamp(120px, 42vw, 180px)", sm: 220 },
            flex: "0 1 auto",
            bgcolor: "background.paper"
          }}
          slotProps={{ htmlInput: { "aria-label": "プロジェクトを切り替え" } }}
        >
          {projects.map((project) => (
            <MenuItem key={project.id} value={project.id}>
              {project.name}
            </MenuItem>
          ))}
        </TextField>
        <Button variant="outlined" startIcon={<LogoutIcon />} onClick={onLogout} sx={{ whiteSpace: "nowrap", display: { xs: "none", sm: "inline-flex" } }}>
          ログアウト
        </Button>
      </Toolbar>
    </AppBar>
  );
}
