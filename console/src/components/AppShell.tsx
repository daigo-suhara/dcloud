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
  deletingMachineName: string;
  deletingName: string;
  deletingProjectId: string;
  hasProjects: boolean;
  message: string;
  onCancelDelete: () => void;
  onCancelDeleteMachine: () => void;
  onCancelProjectDelete: () => void;
  onCloseSidebar: () => void;
  onConfirmDelete: (name: string) => void;
  onConfirmDeleteMachine: (name: string) => void;
  onConfirmDeleteProject: (projectId: string) => void;
  onNavigate: (section: RouteState["section"]) => void;
  onProjectSelect: (projectId: string) => void;
  onToggleSidebar: () => void;
  onLogout: () => void;
  onClearMessage: () => void;
  pendingDeleteMachineName: string;
  pendingDeleteName: string;
  pendingProjectDeleteId: string;
  pendingProjectDeleteName: string;
  projects: Project[];
  route: RouteState;
  sidebarOpen: boolean;
};

export function AppShell({
  activeProjectId,
  children,
  deletingMachineName,
  deletingName,
  deletingProjectId,
  hasProjects,
  message,
  onCancelDelete,
  onCancelDeleteMachine,
  onCancelProjectDelete,
  onCloseSidebar,
  onConfirmDelete,
  onConfirmDeleteMachine,
  onConfirmDeleteProject,
  onNavigate,
  onProjectSelect,
  onToggleSidebar,
  onLogout,
  onClearMessage,
  pendingDeleteMachineName,
  pendingDeleteName,
  pendingProjectDeleteId,
  pendingProjectDeleteName,
  projects,
  route,
  sidebarOpen
}: AppShellProps) {
  return (
    <Box sx={{ minHeight: "100vh", ...shellBg }}>
      <Snackbar open={Boolean(message)} autoHideDuration={3500} onClose={onClearMessage} anchorOrigin={{ vertical: "bottom", horizontal: "center" }}>
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

      <Container maxWidth="lg" sx={{ py: { xs: 3, md: 4 }, px: { xs: 2, sm: 3 } }}>
        {children}
      </Container>
      <Dialogs
        deletingMachineName={deletingMachineName}
        deletingName={deletingName}
        deletingProjectId={deletingProjectId}
        onCancelDelete={onCancelDelete}
        onCancelDeleteMachine={onCancelDeleteMachine}
        onCancelProjectDelete={onCancelProjectDelete}
        onConfirmDelete={onConfirmDelete}
        onConfirmDeleteMachine={onConfirmDeleteMachine}
        onConfirmDeleteProject={onConfirmDeleteProject}
        pendingDeleteMachineName={pendingDeleteMachineName}
        pendingDeleteName={pendingDeleteName}
        pendingProjectDeleteId={pendingProjectDeleteId}
        pendingProjectDeleteName={pendingProjectDeleteName}
      />
    </Box>
  );
}
