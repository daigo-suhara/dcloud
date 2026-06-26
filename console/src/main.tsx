import React from "react";
import { createRoot } from "react-dom/client";
import { Box, CssBaseline, ThemeProvider } from "@mui/material";
import { BrowserRouter, useLocation, useNavigate } from "react-router-dom";
import "./styles.css";
import { AuthScreen } from "./components/AuthScreen";
import { AppShell } from "./components/AppShell";
import { ContainerSection } from "./components/ContainerSection";
import { ComputeCreateSection } from "./components/ComputeCreateSection";
import { ComputeDetailSection } from "./components/ComputeDetailSection";
import { ComputeSection } from "./components/ComputeSection";
import { DeploySection } from "./components/DeploySection";
import { RepositorySection } from "./components/RepositorySection";
import { ProjectCreateSection } from "./components/ProjectCreateSection";
import { LoadingScreen } from "./components/LoadingScreen";
import { HomeSection } from "./components/HomeSection";
import { StorageSection } from "./components/StorageSection";
import { DatabaseSection } from "./components/DatabaseSection";
import { DatabaseDetailSection } from "./components/DatabaseDetailSection";
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
  const selectedComputeMachine = controller.route.selectedComputeMachineName
    ? controller.computeMachines.find((machine) => machine.name === controller.route.selectedComputeMachineName) ?? null
    : null;
  const selectedDatabase = controller.route.selectedDatabaseName
    ? controller.databases.find((db) => db.name === controller.route.selectedDatabaseName) ?? null
    : null;

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
        deletingMachineName={controller.deletingMachineName}
        deletingName={controller.deletingName}
        deletingProjectId={controller.deletingProjectId}
        hasProjects={controller.projects.length > 0}
        message={controller.message}
        onCancelDelete={controller.cancelDelete}
        onCancelDeleteMachine={controller.cancelDeleteMachine}
        onCancelProjectDelete={controller.cancelProjectDelete}
        onClearMessage={() => controller.setMessage("")}
        onCloseSidebar={() => controller.setSidebarOpen(false)}
        onConfirmDelete={controller.confirmDelete}
        onConfirmDeleteMachine={controller.confirmDeleteMachine}
        onConfirmDeleteProject={controller.confirmDeleteProject}
        onProjectSelect={controller.handleProjectSelect}
        onToggleSidebar={() => controller.setSidebarOpen(true)}
        onLogout={controller.startLogout}
        pendingDeleteMachineName={controller.pendingDeleteMachineName}
        pendingDeleteName={controller.pendingDeleteName}
        pendingProjectDeleteId={controller.pendingProjectDeleteId}
        pendingProjectDeleteName={controller.pendingProjectDeleteName}
        projects={controller.projects}
        route={controller.route}
        sidebarOpen={controller.sidebarOpen}
        onNavigate={(section) =>
          navigate(
            section === "compute"
              ? "/compute"
              : section === "compute-create"
                ? "/compute/new"
                : section === "deploy"
                  ? "/container/deploy"
                  : `/${section}`
          )
        }
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
            updatingServiceName={controller.updatingName}
            onBackToList={() => navigate("/container")}
            onDeployClick={() => navigate("/container/deploy")}
            onDeleteService={controller.requestDelete}
            onOpenService={controller.handleOpenService}
            onRepoConnectClick={controller.handleOpenRepository}
            onSetDomain={controller.handleSetContainerDomain}
            onUpdateService={controller.handleUpdateContainer}
            selectedService={controller.selectedService}
            selectedStatus={controller.selectedStatus}
            containers={controller.containers}
            activeProjectId={controller.activeProjectId}
          />
        ) : visibleSection === "compute" && controller.route.selectedComputeMachineName ? (
          <ComputeDetailSection
            machine={selectedComputeMachine}
            machineName={controller.route.selectedComputeMachineName}
            loading={controller.computeLoading}
            projectId={controller.activeProjectId}
            deletingMachineName={controller.deletingMachineName}
            onBack={() => navigate("/compute")}
            onDeleteMachine={controller.requestDeleteMachine}
          />
        ) : visibleSection === "compute-create" ? (
          <ComputeCreateSection
            error={controller.error}
            form={controller.computeForm}
            onBack={() => navigate("/compute")}
            onChange={controller.handleComputeFormChange}
            onSubmit={controller.handleComputeSubmit}
            submitting={controller.computeSubmitting}
          />
        ) : visibleSection === "compute" ? (
          <ComputeSection
            deletingMachineName={controller.deletingMachineName}
            loading={controller.computeLoading}
            machines={controller.computeMachines}
            onDeleteMachine={controller.requestDeleteMachine}
            onOpenCreate={() => navigate("/compute/new")}
          />
        ) : visibleSection === "storage" ? (
          <StorageSection
            loading={controller.storageLoading}
            buckets={controller.buckets}
            deletingBucketName={controller.deletingBucketName}
            onDeleteBucket={controller.handleDeleteBucket}
            onCreateBucket={controller.handleCreateBucket}
            activeProjectId={controller.activeProjectId}
          />
        ) : visibleSection === "database" ? (
          <DatabaseSection
            loading={controller.databaseLoading}
            databases={controller.databases}
            deletingDatabaseName={controller.deletingDatabaseName}
            onDeleteDatabase={controller.handleDeleteDatabase}
            onCreateDatabase={controller.handleCreateDatabase}
            onOpenDatabase={(name) => navigate(`/database/${encodeURIComponent(name)}`)}
            activeProjectId={controller.activeProjectId}
          />
        ) : visibleSection === "database-detail" ? (
          <DatabaseDetailSection
            database={selectedDatabase}
            databaseName={controller.route.selectedDatabaseName ?? ""}
            loading={controller.databaseLoading}
            activeProjectId={controller.activeProjectId}
            onBack={() => navigate("/database")}
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
