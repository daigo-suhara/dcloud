import { Alert, Box, Container, Snackbar } from "@mui/material";
import type { ReactNode } from "react";
import { type Project, type RouteState } from "../types";
import { shellBg } from "../theme";
import { Dialogs } from "./shell/Dialogs";
import { DrawerNav } from "./shell/DrawerNav";
import { Header } from "./shell/Header";

type AppShellProps = {
  activeProjectId: string;
  children: ReactNode;
  deletingName: string;
  deletingProjectId: string;
  hasProjects: boolean;
  message: string;
  onCancelDelete: () => void;
  onCancelProjectDelete: () => void;
  onCloseSidebar: () => void;
  onConfirmDelete: (name: string) => void;
  onConfirmDeleteProject: (projectId: string) => void;
  onNavigate: (section: RouteState["section"]) => void;
  onProjectSelect: (projectId: string) => void;
  onToggleSidebar: () => void;
  onLogout: () => void;
  onClearMessage: () => void;
  pendingDeleteName: string;
  pendingProjectDeleteId: string;
  projects: Project[];
  route: RouteState;
  sidebarOpen: boolean;
};

export function AppShell({
  activeProjectId,
  children,
  deletingName,
  deletingProjectId,
  hasProjects,
  message,
  onCancelDelete,
  onCancelProjectDelete,
  onCloseSidebar,
  onConfirmDelete,
  onConfirmDeleteProject,
  onNavigate,
  onProjectSelect,
  onToggleSidebar,
  onLogout,
  onClearMessage,
  pendingDeleteName,
  pendingProjectDeleteId,
  projects,
  route,
  sidebarOpen
}: AppShellProps) {
  return (
    <Box sx={{ minHeight: "100vh", ...shellBg }}>
      <Snackbar open={Boolean(message)} autoHideDuration={3500} onClose={onClearMessage} anchorOrigin={{ vertical: "top", horizontal: "right" }}>
        <Alert severity="success" variant="filled" sx={{ width: "100%" }}>
          {message}
        </Alert>
      </Snackbar>
      <Header
        activeProjectId={activeProjectId}
        hasProjects={hasProjects}
        onLogout={onLogout}
        onProjectSelect={onProjectSelect}
        onToggleSidebar={onToggleSidebar}
        projects={projects}
      />

      <DrawerNav onCloseSidebar={onCloseSidebar} onNavigate={onNavigate} route={route} sidebarOpen={sidebarOpen} />

      <Container maxWidth={false} sx={{ py: { xs: 2, md: 3 }, px: { xs: 1.5, sm: 2, md: 3 } }}>
        {children}
      </Container>
      <Dialogs
        deletingName={deletingName}
        deletingProjectId={deletingProjectId}
        onCancelDelete={onCancelDelete}
        onCancelProjectDelete={onCancelProjectDelete}
        onConfirmDelete={onConfirmDelete}
        onConfirmDeleteProject={onConfirmDeleteProject}
        pendingDeleteName={pendingDeleteName}
        pendingProjectDeleteId={pendingProjectDeleteId}
      />
    </Box>
  );
}
