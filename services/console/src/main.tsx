import React from "react";
import { createRoot } from "react-dom/client";
import { Box, CssBaseline, ThemeProvider } from "@mui/material";
import "./styles.css";
import { AppShell } from "./components/AppShell";
import { AuthScreen } from "./components/AuthScreen";
import { ContainerSection } from "./components/ContainerSection";
import { DeploySection } from "./components/DeploySection";
import { LoadingScreen } from "./components/LoadingScreen";
import { HomeSection } from "./components/HomeSection";
import { theme } from "./theme";
import { useConsoleController } from "./hooks/useConsoleController";

function App() {
  const controller = useConsoleController();

  if (controller.authLoading) {
    return (
      <ThemeProvider theme={theme}>
        <CssBaseline />
        <Box sx={{ minHeight: "100vh" }}>
          <LoadingScreen />
        </Box>
      </ThemeProvider>
    );
  }

  if (!controller.currentUser) {
    return (
      <ThemeProvider theme={theme}>
        <CssBaseline />
        <Box sx={{ minHeight: "100vh" }}>
          <AuthScreen error={controller.error} onLogin={controller.startLogin} onRegister={controller.startRegister} />
        </Box>
      </ThemeProvider>
    );
  }

  return (
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <AppShell
        activeProjectId={controller.activeProjectId}
        deletingName={controller.deletingName}
        deletingProjectId={controller.deletingProjectId}
        hasProjects={controller.projects.length > 0}
        message={controller.message}
        onCancelDelete={controller.cancelDelete}
        onCancelProjectDelete={controller.cancelProjectDelete}
        onClearMessage={() => controller.setMessage("")}
        onCloseSidebar={() => controller.setSidebarOpen(false)}
        onConfirmDelete={controller.confirmDelete}
        onConfirmDeleteProject={controller.confirmDeleteProject}
        onNavigate={(section) => (window.location.hash = `#${section}`)}
        onProjectSelect={controller.handleProjectSelect}
        onToggleSidebar={() => controller.setSidebarOpen(true)}
        onLogout={controller.startLogout}
        pendingDeleteName={controller.pendingDeleteName}
        pendingProjectDeleteId={controller.pendingProjectDeleteId}
        projects={controller.projects}
        route={controller.route}
        sidebarOpen={controller.sidebarOpen}
      >
        {controller.route.section === "home" ? (
          <HomeSection
            activeProjectId={controller.activeProjectId}
            creatingProject={controller.creatingProject}
            deletingProjectId={controller.deletingProjectId}
            onCreateProject={controller.handleCreateProject}
            onProjectNameChange={controller.setProjectName}
            onRequestDeleteProject={controller.requestDeleteProject}
            onSelectProject={controller.handleProjectSelect}
            onToggleCreateForm={() => controller.setShowProjectCreateForm((current) => !current)}
            projectName={controller.projectName}
            projects={controller.projects}
            showProjectCreateForm={controller.showProjectCreateForm}
          />
        ) : controller.route.section === "container" ? (
          <ContainerSection
            loading={controller.loading}
            onBackToList={() => (window.location.hash = "#container")}
            onDeployClick={() => (window.location.hash = "#deploy")}
            onDeleteService={controller.requestDelete}
            onOpenService={controller.handleOpenService}
            onRepoConnectClick={() => (window.location.hash = "#container")}
            onReloadLogs={() => {
              if (controller.selectedService) {
                void controller.loadServiceLogs(controller.selectedService.name);
              }
            }}
            selectedService={controller.selectedService}
            selectedStatus={controller.selectedStatus}
            serviceLogs={controller.serviceLogs}
            serviceLogsError={controller.serviceLogsError}
            serviceLogsLoading={controller.serviceLogsLoading}
            services={controller.services}
          />
        ) : controller.route.section === "deploy" ? (
          <DeploySection
            error={controller.error}
            form={controller.form}
            onBack={() => (window.location.hash = "#container")}
            onChange={controller.handleFormChange}
            onSubmit={controller.handleSubmit}
            submitting={controller.submitting}
          />
        ) : null}
      </AppShell>
    </ThemeProvider>
  );
}

createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
