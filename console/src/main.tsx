import React from "react";
import { createRoot } from "react-dom/client";
import { Box, CssBaseline, ThemeProvider } from "@mui/material";
import { BrowserRouter, useLocation, useNavigate } from "react-router-dom";
import "./styles.css";
import { AuthScreen } from "./components/AuthScreen";
import { AppShell } from "./components/AppShell";
import { ContainerSection } from "./components/ContainerSection";
import { DeploySection } from "./components/DeploySection";
import { RepositorySection } from "./components/RepositorySection";
import { ProjectCreateSection } from "./components/ProjectCreateSection";
import { LoadingScreen } from "./components/LoadingScreen";
import { HomeSection } from "./components/HomeSection";
import { theme } from "./theme";
import { useConsoleController } from "./hooks/useConsoleController";

function App() {
  return (
    <BrowserRouter>
      <AppContent />
    </BrowserRouter>
  );
}

function AppContent() {
  const controller = useConsoleController();
  const navigate = useNavigate();
  const location = useLocation();
  const forceProjectCreate = Boolean(controller.currentUser && controller.projectsLoaded && controller.projects.length === 0);
  const visibleSection = forceProjectCreate ? "project-create" : controller.route.section;

  React.useEffect(() => {
    if (!controller.authLoading && !controller.currentUser && location.pathname !== "/login") {
      navigate("/login", { replace: true });
    }
  }, [controller.authLoading, controller.currentUser, location.pathname, navigate]);

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
          <AuthScreen
            error={controller.error}
            loading={controller.authSubmitting}
            form={controller.authForm}
            onChange={controller.handleAuthFormChange}
            onLogin={controller.startLogin}
            onRegister={controller.startRegister}
          />
        </Box>
      </ThemeProvider>
    );
  }

  if (!controller.projectsLoaded) {
    return (
      <ThemeProvider theme={theme}>
        <CssBaseline />
        <Box sx={{ minHeight: "100vh" }}>
          <LoadingScreen />
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
        onProjectSelect={controller.handleProjectSelect}
        onToggleSidebar={() => controller.setSidebarOpen(true)}
        onLogout={controller.startLogout}
        pendingDeleteName={controller.pendingDeleteName}
        pendingProjectDeleteId={controller.pendingProjectDeleteId}
        pendingProjectDeleteName={controller.pendingProjectDeleteName}
        projects={controller.projects}
        route={controller.route}
        sidebarOpen={controller.sidebarOpen}
        onNavigate={(section) => navigate(section === "deploy" ? "/container/deploy" : `/${section}`)}
      >
        {visibleSection === "home" ? (
          <HomeSection
            activeProjectId={controller.activeProjectId}
            deletingProjectId={controller.deletingProjectId}
            onOpenProjectCreate={() => navigate("/project-create")}
            onRequestDeleteProject={controller.requestDeleteProject}
            onSelectProject={controller.handleProjectSelect}
            projects={controller.projects}
          />
        ) : visibleSection === "project-create" ? (
          <ProjectCreateSection
            creatingProject={controller.creatingProject}
            hasProjects={controller.projects.length > 0}
            onBack={() => navigate("/home")}
            onCreateProject={controller.handleCreateProject}
            onProjectNameChange={controller.setProjectName}
            projectName={controller.projectName}
          />
        ) : visibleSection === "container" ? (
          <ContainerSection
            loading={controller.loading}
            deletingServiceName={controller.deletingName}
            onBackToList={() => navigate("/container")}
            onDeployClick={() => navigate("/container/deploy")}
            onDeleteService={controller.requestDelete}
            onOpenService={controller.handleOpenService}
            onRepoConnectClick={controller.handleOpenRepository}
            selectedService={controller.selectedService}
            selectedStatus={controller.selectedStatus}
            containers={controller.containers}
          />
        ) : visibleSection === "deploy" ? (
          <DeploySection
            error={controller.error}
            form={controller.form}
            onBack={() => navigate("/container")}
            onChange={controller.handleFormChange}
            onSubmit={controller.handleSubmit}
            submitting={controller.submitting}
          />
        ) : visibleSection === "repository" ? (
          <RepositorySection
            error={controller.error}
            loading={controller.repositoryLoading}
            saving={controller.savingRepository}
            form={controller.repositoryForm}
            config={controller.repositoryConfig}
            onBack={() => navigate("/container")}
            onChange={controller.handleRepositoryFormChange}
            onSubmit={controller.handleSaveRepository}
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
