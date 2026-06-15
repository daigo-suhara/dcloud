import { useEffect, useMemo, useState } from "react";
import type { FormEvent } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { initialAuthForm, initialComputeForm, initialForm, type AuthForm, type AuthUser, type ComputeForm, type ComputeMachine, type DeployedService, type PlatformResponse, type Project, type ProjectsResponse, type RepositoryConfig, type RepositoryForm, type RouteState, type UpdateForm } from "../types";
import { getServiceStatus, parseRoute } from "../utils";

type LoadServicesOptions = {
  quiet?: boolean;
};

type ApiErrorResponse = {
  detail?: string;
  error?: string;
};

async function readJsonResponse(response: Response): Promise<unknown> {
  const text = await response.text();
  if (!text.trim()) {
    return null;
  }
  try {
    return JSON.parse(text) as unknown;
  } catch {
    return { detail: text };
  }
}

export function useConsoleController() {
  const [currentUser, setCurrentUser] = useState<AuthUser | null>(null);
  const [containers, setContainers] = useState<DeployedService[]>([]);
  const [computeMachines, setComputeMachines] = useState<ComputeMachine[]>([]);
  const [projects, setProjects] = useState<Project[]>([]);
  const [activeProjectId, setActiveProjectId] = useState("");
  const [projectName, setProjectName] = useState("");
  const [repositoryConfig, setRepositoryConfig] = useState<RepositoryConfig | null>(null);
  const [repositoryForm, setRepositoryForm] = useState<RepositoryForm>({
    repositoryOwner: "",
    repositoryName: "",
    repositoryBranch: "main"
  });
  const [creatingProject, setCreatingProject] = useState(false);
  const [deletingProjectId, setDeletingProjectId] = useState("");
  const [pendingProjectDeleteId, setPendingProjectDeleteId] = useState("");
  const [pendingProjectDeleteName, setPendingProjectDeleteName] = useState("");
  const [authLoading, setAuthLoading] = useState(true);
  const [authSubmitting, setAuthSubmitting] = useState(false);
  const [authForm, setAuthForm] = useState<AuthForm>(initialAuthForm);
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [loading, setLoading] = useState(true);
  const [repositoryLoading, setRepositoryLoading] = useState(true);
  const [savingRepository, setSavingRepository] = useState(false);
  const [projectsLoaded, setProjectsLoaded] = useState(false);
  const [computeLoading, setComputeLoading] = useState(true);
  const [computeSubmitting, setComputeSubmitting] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [deletingName, setDeletingName] = useState("");
  const [updatingName, setUpdatingName] = useState("");
  const [deletingMachineName, setDeletingMachineName] = useState("");
  const [pendingDeleteName, setPendingDeleteName] = useState("");
  const [pendingDeleteMachineName, setPendingDeleteMachineName] = useState("");
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [form, setForm] = useState(initialForm);
  const [computeForm, setComputeForm] = useState<ComputeForm>(initialComputeForm);
  const location = useLocation();
  const navigate = useNavigate();
  const route = useMemo<RouteState>(() => parseRoute(location.pathname), [location.pathname]);

  const selectedService =
    route.section === "container" && route.selectedServiceName
      ? containers.find((service) => service.name === route.selectedServiceName) ?? null
      : null;
  const selectedStatus = selectedService ? getServiceStatus(selectedService) : null;

  useEffect(() => {
    if (!message) {
      return;
    }
    const timer = window.setTimeout(() => setMessage(""), 3500);
    return () => window.clearTimeout(timer);
  }, [message]);

  useEffect(() => {
    void loadCurrentUser();
  }, []);

  useEffect(() => {
    if (!currentUser) {
      setProjects([]);
      setContainers([]);
      setComputeMachines([]);
      setActiveProjectId("");
      setProjectName("");
      setRepositoryConfig(null);
      setRepositoryForm({
        repositoryOwner: "",
        repositoryName: "",
        repositoryBranch: "main"
      });
      setRepositoryLoading(true);
      setProjectsLoaded(false);
      return;
    }

    setProjects([]);
    setContainers([]);
    setComputeMachines([]);
    setActiveProjectId("");
    setRepositoryConfig(null);
    setRepositoryForm({
      repositoryOwner: "",
      repositoryName: "",
      repositoryBranch: "main"
    });
    setRepositoryLoading(true);
    setProjectsLoaded(false);
    const savedProject = localStorage.getItem(projectStorageKey(currentUser.id));
    if (savedProject) {
      setActiveProjectId(savedProject);
    }
    void loadProjects();
  }, [currentUser]);

  useEffect(() => {
    if (!activeProjectId || !currentUser || !projectsLoaded) {
      return;
    }

    void loadServices();
    void loadComputeMachines();
    void loadRepository();
    const timer = window.setInterval(() => {
      void loadServices({ quiet: true });
      void loadComputeMachines({ quiet: true });
    }, 5000);

    return () => window.clearInterval(timer);
  }, [activeProjectId, currentUser, projectsLoaded]);

  useEffect(() => {
    const hasSelectable = projects.some((p) => !p.deleting);
    if (!currentUser || !projectsLoaded || hasSelectable || route.section === "project-create") {
      return;
    }
    if (location.pathname !== "/project-create") {
      navigate("/project-create", { replace: true });
    }
  }, [currentUser, location.pathname, navigate, projects, projectsLoaded, route.section]);

  function apiHeaders(extra?: HeadersInit) {
    const headers = new Headers(extra);
    if (activeProjectId) {
      headers.set("X-DCP-Project", activeProjectId);
    }
    return headers;
  }

  function getApiErrorMessage(data: unknown, fallback: string) {
    if (!data || typeof data !== "object") {
      return fallback;
    }
    const payload = data as ApiErrorResponse;
    if (typeof payload.detail === "string" && payload.detail.trim()) {
      return payload.detail;
    }
    if (typeof payload.error === "string" && payload.error.trim()) {
      return payload.error;
    }
    return fallback;
  }

  async function loadCurrentUser() {
    setAuthLoading(true);
    try {
      const response = await fetch("/api/v1/auth/me", {
        credentials: "include"
      });
      if (response.status === 401) {
        setCurrentUser(null);
        return;
      }
      const data = (await readJsonResponse(response)) as AuthUser | ApiErrorResponse;
      if (!response.ok) {
        throw new Error(getApiErrorMessage(data, "ログイン状態を確認できませんでした"));
      }
      if ("id" in data) {
        setCurrentUser(data);
      } else {
        setCurrentUser(null);
      }
    } catch (authError) {
      setError(authError instanceof Error ? authError.message : "ログイン状態を確認できませんでした");
      setCurrentUser(null);
    } finally {
      setAuthLoading(false);
    }
  }

  function handleAuthFormChange(patch: Partial<AuthForm>) {
    setAuthForm((current) => ({ ...current, ...patch }));
  }

  async function authenticate(path: string) {
    setAuthSubmitting(true);
    setError("");
    try {
      const response = await fetch(path, {
        credentials: "include",
        method: "POST",
        headers: {
          "Content-Type": "application/json"
        },
        body: JSON.stringify({
          email: authForm.email.trim(),
          password: authForm.password
        })
      });
      const data = (await readJsonResponse(response)) as { user?: AuthUser } | ApiErrorResponse;
      if (!response.ok) {
        throw new Error(getApiErrorMessage(data, path.endsWith("register") ? "アカウントを作成できませんでした" : "ログインできませんでした"));
      }
      if (data && typeof data === "object" && "user" in data && data.user) {
        setAuthForm((current) => ({ ...current, password: "" }));
        setCurrentUser(data.user);
      } else {
        await loadCurrentUser();
      }
    } catch (authError) {
      setCurrentUser(null);
      setError(authError instanceof Error ? authError.message : "認証に失敗しました");
    } finally {
      setAuthSubmitting(false);
    }
  }

  async function startLogin(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    await authenticate("/api/v1/auth/login");
  }

  async function startRegister() {
    await authenticate("/api/v1/auth/register");
  }

  async function startLogout() {
    try {
      await fetch("/api/v1/auth/logout", {
        credentials: "include",
        method: "POST"
      });
    } finally {
      setCurrentUser(null);
      setProjects([]);
      setContainers([]);
      setActiveProjectId("");
      setProjectName("");
      setRepositoryConfig(null);
      setAuthForm((current) => ({ ...current, password: "" }));
    }
  }

  function projectStorageKey(userId: string) {
    return `dcloud-active-project:${userId}`;
  }

  function handleProjectSelect(projectId: string) {
    setActiveProjectId(projectId);
    if (currentUser) {
      localStorage.setItem(projectStorageKey(currentUser.id), projectId);
    }
  }

  function handleOpenService(name: string) {
    navigate(`/container/${encodeURIComponent(name)}`);
  }

  function handleOpenRepository() {
    navigate("/container/repository");
  }

  function handleFormChange(patch: Partial<typeof form>) {
    setForm((current) => ({ ...current, ...patch }));
  }

  function handleComputeFormChange(patch: Partial<ComputeForm>) {
    setComputeForm((current) => ({ ...current, ...patch }));
  }

  async function loadProjects() {
    if (!currentUser) {
      return;
    }
    try {
      const response = await fetch("/api/v1/projects", {
        credentials: "include",
        headers: apiHeaders()
      });
      const data = (await readJsonResponse(response)) as ProjectsResponse | ApiErrorResponse;
      if (!response.ok) {
        throw new Error(getApiErrorMessage(data, "プロジェクト一覧を読み込めませんでした"));
      }
      if ("projects" in data) {
        setProjects(data.projects);
        setProjectsLoaded(true);
        const selectable = data.projects.filter((p) => !p.deleting);
        if (selectable.length === 0) {
          localStorage.removeItem(projectStorageKey(currentUser.id));
          setActiveProjectId("");
          if (data.projects.length === 0) {
            navigate("/project-create", { replace: true });
          }
          return;
        }

        const saved = localStorage.getItem(projectStorageKey(currentUser.id));
        const nextProject = selectable.find((p) => p.id === saved)?.id ?? selectable[0].id;
        handleProjectSelect(nextProject);
      }
    } catch (projectError) {
      setError(projectError instanceof Error ? projectError.message : "プロジェクト一覧を読み込めませんでした");
    }
  }

  async function loadServices(options?: LoadServicesOptions) {
    if (!activeProjectId || !currentUser) {
      return;
    }
    if (!options?.quiet) {
      setLoading(true);
      setError("");
    }
    try {
      const response = await fetch("/api/v1/container", {
        credentials: "include",
        headers: apiHeaders()
      });
      const data = (await readJsonResponse(response)) as PlatformResponse | ApiErrorResponse;
      if (!response.ok) {
        throw new Error(getApiErrorMessage(data, "サービス一覧を読み込めませんでした"));
      }
      if ("namespace" in data) {
        setContainers(data.containers ?? []);
      }
    } catch (loadError) {
      if (!options?.quiet) {
        setError(loadError instanceof Error ? loadError.message : "サービス一覧を読み込めませんでした");
      }
    } finally {
      if (!options?.quiet) {
        setLoading(false);
      }
    }
  }

  async function loadComputeMachines(options?: LoadServicesOptions) {
    if (!activeProjectId || !currentUser) {
      return;
    }
    if (!options?.quiet) {
      setComputeLoading(true);
      setError("");
    }
    try {
      const response = await fetch("/api/v1/compute", {
        credentials: "include",
        headers: apiHeaders()
      });
      const data = (await readJsonResponse(response)) as { machines?: ComputeMachine[] } | ApiErrorResponse;
      if (!response.ok) {
        throw new Error(getApiErrorMessage(data, "仮想マシン一覧を読み込めませんでした"));
      }
      if (data && typeof data === "object" && "machines" in data) {
        setComputeMachines(data.machines ?? []);
      }
    } catch (loadError) {
      if (!options?.quiet) {
        setError(loadError instanceof Error ? loadError.message : "仮想マシン一覧を読み込めませんでした");
      }
    } finally {
      if (!options?.quiet) {
        setComputeLoading(false);
      }
    }
  }

  async function loadRepository() {
    if (!activeProjectId || !currentUser) {
      return;
    }
    setRepositoryLoading(true);
    try {
      const response = await fetch(`/api/v1/projects/${encodeURIComponent(activeProjectId)}/repository`, {
        credentials: "include",
        headers: apiHeaders()
      });
      if (response.status === 404) {
        setRepositoryConfig(null);
        setRepositoryForm({
          repositoryOwner: "",
          repositoryName: "",
          repositoryBranch: "main"
        });
        return;
      }
      const data = (await readJsonResponse(response)) as RepositoryConfig | ApiErrorResponse;
      if (!response.ok) {
        throw new Error(getApiErrorMessage(data, "リポジトリ設定を読み込めませんでした"));
      }
      if ("projectId" in data) {
        setRepositoryConfig(data);
        setRepositoryForm({
          repositoryOwner: data.repositoryOwner,
          repositoryName: data.repositoryName,
          repositoryBranch: data.repositoryBranch || "main"
        });
      }
    } catch (repositoryError) {
      setError(repositoryError instanceof Error ? repositoryError.message : "リポジトリ設定を読み込めませんでした");
    } finally {
      setRepositoryLoading(false);
    }
  }

  async function handleCreateProject(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setCreatingProject(true);
    setError("");
    setMessage("");
    try {
      const response = await fetch("/api/v1/projects", {
        credentials: "include",
        method: "POST",
        headers: apiHeaders({
          "Content-Type": "application/json"
        }),
        body: JSON.stringify({ name: projectName.trim() })
      });
      const data = (await readJsonResponse(response)) as Project | ApiErrorResponse;
      if (!response.ok) {
        throw new Error(getApiErrorMessage(data, "プロジェクトの作成に失敗しました"));
      }
      if ("id" in data) {
        setProjects((current) => [...current, data]);
        handleProjectSelect(data.id);
        setProjectName("");
        setMessage(`${data.name} を作成しました`);
        navigate("/home", { replace: true });
      }
    } catch (projectError) {
      setError(projectError instanceof Error ? projectError.message : "プロジェクトの作成に失敗しました");
    } finally {
      setCreatingProject(false);
    }
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSubmitting(true);
    setError("");
    setMessage("");
    try {
      const response = await fetch("/api/v1/container", {
        method: "POST",
        credentials: "include",
        headers: apiHeaders({
          "Content-Type": "application/json"
        }),
        body: JSON.stringify({
          name: form.name.trim(),
          image: form.image.trim(),
          port: Number(form.port || "8080"),
          minScale: Number(form.minScale || "0"),
          maxScale: Number(form.maxScale || "1"),
          startupScript: form.startupScript.trim()
        })
      });

      const data = (await readJsonResponse(response)) as DeployedService | ApiErrorResponse;
      if (!response.ok) {
        throw new Error(getApiErrorMessage(data, "サービスの作成に失敗しました"));
      }
      if ("name" in data) {
        setMessage(`${data.name} を作成しました`);
        handleOpenService(data.name);
      }
      setForm(initialForm);
      await loadServices();
    } catch (deployError) {
      setError(deployError instanceof Error ? deployError.message : "サービスの作成に失敗しました");
    } finally {
      setSubmitting(false);
    }
  }

  async function handleComputeSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setComputeSubmitting(true);
    setError("");
    setMessage("");
    try {
      const response = await fetch("/api/v1/compute", {
        method: "POST",
        credentials: "include",
        headers: apiHeaders({
          "Content-Type": "application/json"
        }),
        body: JSON.stringify({
          name: computeForm.name.trim(),
          image: computeForm.image.trim(),
          cpu: computeForm.cpu.trim() || "1",
          memory: computeForm.memory.trim() || "1Gi"
        })
      });

      const data = (await readJsonResponse(response)) as ComputeMachine | ApiErrorResponse;
      if (!response.ok) {
        throw new Error(getApiErrorMessage(data, "仮想マシンの作成に失敗しました"));
      }
      if ("name" in data) {
        setMessage(`${data.name} を作成しました`);
        navigate(`/compute/${encodeURIComponent(data.name)}`);
      }
      setComputeForm((current) => ({ ...current, name: "" }));
      await loadComputeMachines();
    } catch (computeError) {
      setError(computeError instanceof Error ? computeError.message : "仮想マシンの作成に失敗しました");
    } finally {
      setComputeSubmitting(false);
    }
  }

  function handleRepositoryFormChange(patch: Partial<RepositoryForm>) {
    setRepositoryForm((current) => ({ ...current, ...patch }));
  }

  async function handleSaveRepository(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!activeProjectId) {
      setError("プロジェクトを選択してください");
      return;
    }
    setSavingRepository(true);
    setError("");
    setMessage("");
    try {
      const response = await fetch(`/api/v1/projects/${encodeURIComponent(activeProjectId)}/repository`, {
        method: "PUT",
        credentials: "include",
        headers: apiHeaders({
          "Content-Type": "application/json"
        }),
        body: JSON.stringify({
          repositoryOwner: repositoryForm.repositoryOwner.trim(),
          repositoryName: repositoryForm.repositoryName.trim(),
          repositoryBranch: repositoryForm.repositoryBranch.trim() || "main"
        })
      });
      const data = (await readJsonResponse(response)) as RepositoryConfig | ApiErrorResponse;
      if (!response.ok) {
        throw new Error(getApiErrorMessage(data, "リポジトリ設定の保存に失敗しました"));
      }
      if ("projectId" in data) {
        setRepositoryConfig(data);
        setRepositoryForm({
          repositoryOwner: data.repositoryOwner,
          repositoryName: data.repositoryName,
          repositoryBranch: data.repositoryBranch
        });
        setMessage(`${data.repositoryOwner}/${data.repositoryName} を接続しました`);
        navigate("/container");
      }
    } catch (repositoryError) {
      setError(repositoryError instanceof Error ? repositoryError.message : "リポジトリ設定の保存に失敗しました");
    } finally {
      setSavingRepository(false);
    }
  }

  async function handleUpdateContainer(name: string, form: UpdateForm) {
    setUpdatingName(name);
    setError("");
    setMessage("");
    try {
      const response = await fetch(`/api/v1/container/${encodeURIComponent(name)}`, {
        method: "PUT",
        credentials: "include",
        headers: apiHeaders({ "Content-Type": "application/json" }),
        body: JSON.stringify({
          image: form.image.trim(),
          port: Number(form.port || "8080"),
          minScale: Number(form.minScale || "0"),
          maxScale: Number(form.maxScale || "20"),
          startupScript: form.startupScript.trim()
        })
      });
      const data = (await readJsonResponse(response)) as DeployedService | ApiErrorResponse;
      if (!response.ok) {
        throw new Error(getApiErrorMessage(data, "サービスの更新に失敗しました"));
      }
      if ("name" in data) {
        setContainers((current) => current.map((c) => c.name === name ? { ...c, ...data } : c));
        setMessage(`${name} を更新しました`);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "サービスの更新に失敗しました");
    } finally {
      setUpdatingName("");
    }
  }

  async function handleSetContainerDomain(name: string, customDomain: string) {
    setError("");
    setMessage("");
    try {
      const response = await fetch(`/api/v1/container/${encodeURIComponent(name)}/domain`, {
        method: "PUT",
        credentials: "include",
        headers: apiHeaders({ "Content-Type": "application/json" }),
        body: JSON.stringify({ customDomain: customDomain.trim() })
      });
      const data = (await readJsonResponse(response)) as DeployedService | ApiErrorResponse;
      if (!response.ok) {
        throw new Error(getApiErrorMessage(data, "ドメインの設定に失敗しました"));
      }
      if ("name" in data) {
        setContainers((current) => current.map((c) => c.name === name ? { ...c, ...data } : c));
        setMessage(customDomain.trim() ? `カスタムドメインを設定しました` : "カスタムドメインを削除しました");
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "ドメインの設定に失敗しました");
    }
  }

  function requestDelete(name: string) {
    setPendingDeleteName(name);
  }

  function cancelDelete() {
    setPendingDeleteName("");
  }

  function requestDeleteMachine(name: string) {
    setPendingDeleteMachineName(name);
  }

  function cancelDeleteMachine() {
    setPendingDeleteMachineName("");
  }

  function requestDeleteProject(projectId: string, projectName: string) {
    setPendingProjectDeleteId(projectId);
    setPendingProjectDeleteName(projectName);
  }

  function cancelProjectDelete() {
    setPendingProjectDeleteId("");
    setPendingProjectDeleteName("");
  }

  async function confirmDelete(name: string) {
    setPendingDeleteName("");
    setDeletingName(name);
    setError("");
    setMessage("");
    try {
      const response = await fetch(`/api/v1/container/${name}`, {
        method: "DELETE",
        credentials: "include",
        headers: apiHeaders()
      });
      if (!response.ok) {
        const data = (await readJsonResponse(response)) as ApiErrorResponse;
        throw new Error(getApiErrorMessage(data, "サービスの削除に失敗しました"));
      }
      const { operationId } = await response.json() as { operationId: string };
      await pollOperation(operationId);
      setMessage(`${name} を削除しました`);
      if (route.selectedServiceName === name) {
        navigate("/container");
      }
      await loadServices();
    } catch (deleteError) {
      setError(deleteError instanceof Error ? deleteError.message : "サービスの削除に失敗しました");
    } finally {
      setDeletingName("");
    }
  }

  async function pollOperation(operationId: string): Promise<void> {
    for (let i = 0; i < 60; i++) {
      await new Promise(resolve => setTimeout(resolve, 2000));
      const res = await fetch(`/api/v1/operations/${operationId}`, {
        credentials: "include",
        headers: apiHeaders()
      });
      if (!res.ok) throw new Error("オペレーションの取得に失敗しました");
      const op = await res.json() as { status: string; error: string };
      if (op.status === "done") return;
      if (op.status === "error") throw new Error(op.error || "削除に失敗しました");
    }
    throw new Error("削除がタイムアウトしました");
  }

  async function confirmDeleteMachine(name: string) {
    setPendingDeleteMachineName("");
    setDeletingMachineName(name);
    setError("");
    setMessage("");
    try {
      const response = await fetch(`/api/v1/compute/${encodeURIComponent(name)}`, {
        method: "DELETE",
        credentials: "include",
        headers: apiHeaders()
      });
      if (!response.ok) {
        const data = (await readJsonResponse(response)) as ApiErrorResponse;
        throw new Error(getApiErrorMessage(data, "仮想マシンの削除に失敗しました"));
      }
      const { operationId } = await response.json() as { operationId: string };
      await pollOperation(operationId);
      setMessage(`${name} を削除しました`);
      if (route.selectedComputeMachineName === name) {
        navigate("/compute");
      }
      await loadComputeMachines();
    } catch (deleteError) {
      setError(deleteError instanceof Error ? deleteError.message : "仮想マシンの削除に失敗しました");
    } finally {
      setDeletingMachineName("");
    }
  }

  async function confirmDeleteProject(projectId: string) {
    setPendingProjectDeleteId("");
    setPendingProjectDeleteName("");
    setDeletingProjectId(projectId);
    setError("");
    setMessage("");
    try {
      const response = await fetch(`/api/v1/projects/${projectId}`, {
        method: "DELETE",
        credentials: "include",
        headers: apiHeaders()
      });
      if (!response.ok) {
        const data = (await readJsonResponse(response)) as ApiErrorResponse;
        throw new Error(getApiErrorMessage(data, "プロジェクトの削除に失敗しました"));
      }
      const { operationId } = await response.json() as { operationId: string };
      if (activeProjectId === projectId) {
        localStorage.removeItem(projectStorageKey(currentUser!.id));
        setActiveProjectId("");
        navigate("/home", { replace: true });
      }
      await pollOperation(operationId);
      setMessage("プロジェクトを削除しました");
      await loadProjects();
    } catch (projectError) {
      setError(projectError instanceof Error ? projectError.message : "プロジェクトの削除に失敗しました");
    } finally {
      setDeletingProjectId("");
    }
  }

  return {
    activeProjectId,
    authForm,
    authLoading,
    authSubmitting,
    cancelDelete,
    cancelDeleteMachine,
    cancelProjectDelete,
    confirmDelete,
    confirmDeleteMachine,
    confirmDeleteProject,
    creatingProject,
    currentUser,
    deletingName,
    updatingName,
    deletingProjectId,
    error,
    form,
    handleCreateProject,
    handleSetContainerDomain,
    handleUpdateContainer,
    handleFormChange,
    handleComputeFormChange,
    handleAuthFormChange,
    handleOpenRepository,
    handleRepositoryFormChange,
    handleProjectSelect,
    handleOpenService,
    handleSaveRepository,
    handleSubmit,
    handleComputeSubmit,
    loading,
    message,
    projectsLoaded,
    pendingDeleteMachineName,
    pendingDeleteName,
    pendingProjectDeleteId,
    pendingProjectDeleteName,
    projectName,
    projects,
    repositoryConfig,
    repositoryForm,
    repositoryLoading,
    computeLoading,
    computeSubmitting,
    computeForm,
    computeMachines,
    deletingMachineName,
    requestDelete,
    requestDeleteMachine,
    requestDeleteProject,
    route,
    selectedService,
    selectedStatus,
    setProjectName,
    setSidebarOpen,
    sidebarOpen,
    savingRepository,
    containers,
    startLogin,
    startLogout,
    startRegister,
    submitting,
    setMessage
  } as const;
}
