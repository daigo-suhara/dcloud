import { useEffect, useMemo, useState } from "react";
import type { FormEvent } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { initialAuthForm, initialForm, type AuthForm, type AuthUser, type DeployedService, type PlatformResponse, type Project, type ProjectsResponse, type RepositoryConfig, type RepositoryForm, type RouteState } from "../types";
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
  const [submitting, setSubmitting] = useState(false);
  const [deletingName, setDeletingName] = useState("");
  const [pendingDeleteName, setPendingDeleteName] = useState("");
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [form, setForm] = useState(initialForm);
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
    void loadRepository();
    const timer = window.setInterval(() => {
      void loadServices({ quiet: true });
    }, 5000);

    return () => window.clearInterval(timer);
  }, [activeProjectId, currentUser, projectsLoaded]);

  useEffect(() => {
    if (!currentUser || !projectsLoaded || projects.length > 0 || route.section === "project-create") {
      return;
    }
    if (location.pathname !== "/project-create") {
      navigate("/project-create", { replace: true });
    }
  }, [currentUser, location.pathname, navigate, projects.length, projectsLoaded, route.section]);

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
          username: authForm.username.trim(),
          password: authForm.password,
          email: authForm.email.trim(),
          name: authForm.name.trim()
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
        if (data.projects.length === 0) {
          localStorage.removeItem(projectStorageKey(currentUser.id));
          setActiveProjectId("");
          navigate("/project-create", { replace: true });
          return;
        }

        const saved = localStorage.getItem(projectStorageKey(currentUser.id));
        const nextProject = data.projects.find((project) => project.id === saved)?.id ?? data.projects[0].id;
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
          maxScale: Number(form.maxScale || "1")
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

  function requestDelete(name: string) {
    setPendingDeleteName(name);
  }

  function cancelDelete() {
    setPendingDeleteName("");
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
      if (!response.ok && response.status !== 204) {
        const data = (await readJsonResponse(response)) as ApiErrorResponse;
        throw new Error(getApiErrorMessage(data, "サービスの削除に失敗しました"));
      }
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
      if (!response.ok && response.status !== 204) {
        const data = (await readJsonResponse(response)) as ApiErrorResponse;
        throw new Error(getApiErrorMessage(data, "プロジェクトの削除に失敗しました"));
      }
      setMessage("プロジェクトを削除しました");
      if (activeProjectId === projectId) {
        localStorage.removeItem(projectStorageKey(currentUser!.id));
        setActiveProjectId("");
      }
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
    cancelProjectDelete,
    confirmDelete,
    confirmDeleteProject,
    creatingProject,
    currentUser,
    deletingName,
    deletingProjectId,
    error,
    form,
    handleCreateProject,
    handleFormChange,
    handleAuthFormChange,
    handleOpenRepository,
    handleRepositoryFormChange,
    handleProjectSelect,
    handleOpenService,
    handleSaveRepository,
    handleSubmit,
    loading,
    message,
    projectsLoaded,
    pendingDeleteName,
    pendingProjectDeleteId,
    pendingProjectDeleteName,
    projectName,
    projects,
    repositoryConfig,
    repositoryForm,
    repositoryLoading,
    requestDelete,
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
