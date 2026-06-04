import { useEffect, useRef, useState } from "react";
import type { FormEvent } from "react";
import { initialForm, type AuthUser, type DeployedService, type PlatformResponse, type Project, type ProjectsResponse, type RouteState } from "../types";
import { getServiceStatus, parseRoute } from "../utils";

type LoadServicesOptions = {
  quiet?: boolean;
};

export function useConsoleController() {
  const [currentUser, setCurrentUser] = useState<AuthUser | null>(null);
  const [services, setServices] = useState<DeployedService[]>([]);
  const [projects, setProjects] = useState<Project[]>([]);
  const [activeProjectId, setActiveProjectId] = useState("");
  const [projectName, setProjectName] = useState("");
  const [creatingProject, setCreatingProject] = useState(false);
  const [showProjectCreateForm, setShowProjectCreateForm] = useState(false);
  const [deletingProjectId, setDeletingProjectId] = useState("");
  const [pendingProjectDeleteId, setPendingProjectDeleteId] = useState("");
  const [authLoading, setAuthLoading] = useState(true);
  const [route, setRoute] = useState<RouteState>(() => parseRoute(window.location.hash));
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [deletingName, setDeletingName] = useState("");
  const [pendingDeleteName, setPendingDeleteName] = useState("");
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [form, setForm] = useState(initialForm);
  const [serviceLogs, setServiceLogs] = useState("");
  const [serviceLogsLoading, setServiceLogsLoading] = useState(false);
  const [serviceLogsError, setServiceLogsError] = useState("");
  const serviceLogsAbortRef = useRef<AbortController | null>(null);

  const selectedService =
    route.section === "container" && route.selectedServiceName
      ? services.find((service) => service.name === route.selectedServiceName) ?? null
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
    const handleHashChange = () => setRoute(parseRoute(window.location.hash));
    window.addEventListener("hashchange", handleHashChange);
    void loadCurrentUser();
    return () => window.removeEventListener("hashchange", handleHashChange);
  }, []);

  useEffect(() => {
    if (!currentUser) {
      setProjects([]);
      setServices([]);
      setActiveProjectId("");
      setProjectName("");
      return;
    }

    setProjects([]);
    setServices([]);
    setActiveProjectId("");
    const savedProject = localStorage.getItem(projectStorageKey(currentUser.id));
    if (savedProject) {
      setActiveProjectId(savedProject);
    }
    void loadProjects();
  }, [currentUser]);

  useEffect(() => {
    if (!activeProjectId) {
      return;
    }

    void loadServices();
    const timer = window.setInterval(() => {
      void loadServices({ quiet: true });
    }, 5000);

    return () => window.clearInterval(timer);
  }, [activeProjectId]);

  useEffect(() => {
    if (!selectedService) {
      serviceLogsAbortRef.current?.abort();
      setServiceLogs("");
      setServiceLogsError("");
      setServiceLogsLoading(false);
      return;
    }
    void loadServiceLogs(selectedService.name);
    return () => serviceLogsAbortRef.current?.abort();
  }, [selectedService?.name]);

  function apiHeaders(extra?: HeadersInit) {
    const headers = new Headers(extra);
    if (activeProjectId) {
      headers.set("X-DCP-Project", activeProjectId);
    }
    return headers;
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
      const data = (await response.json()) as AuthUser | { error?: string };
      if (!response.ok) {
        throw new Error("error" in data && data.error ? data.error : "ログイン状態を確認できませんでした");
      }
      if ("id" in data) {
        setCurrentUser(data);
      }
    } catch (authError) {
      setError(authError instanceof Error ? authError.message : "ログイン状態を確認できませんでした");
    } finally {
      setAuthLoading(false);
    }
  }

  function startLogin() {
    window.location.href = "/api/v1/auth/login";
  }

  function startRegister() {
    window.location.href = "/api/v1/auth/register";
  }

  function startLogout() {
    window.location.href = "/api/v1/auth/logout";
  }

  function projectStorageKey(userId: string) {
    return `dcp-active-project:${userId}`;
  }

  function handleProjectSelect(projectId: string) {
    setActiveProjectId(projectId);
    if (currentUser) {
      localStorage.setItem(projectStorageKey(currentUser.id), projectId);
    }
  }

  function handleOpenService(name: string) {
    window.location.hash = `#container/${encodeURIComponent(name)}`;
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
      const data = (await response.json()) as ProjectsResponse | { error?: string };
      if (!response.ok) {
        throw new Error("error" in data && data.error ? data.error : "プロジェクト一覧を読み込めませんでした");
      }
      if ("projects" in data) {
        setProjects(data.projects);
        const saved = localStorage.getItem(projectStorageKey(currentUser.id));
        const nextProject = data.projects.find((project) => project.id === saved)?.id ?? data.defaultProjectId;
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
      const response = await fetch("/api/v1/services", {
        credentials: "include",
        headers: apiHeaders()
      });
      const data = (await response.json()) as PlatformResponse | { error?: string };
      if (!response.ok) {
        throw new Error("error" in data && data.error ? data.error : "サービス一覧を読み込めませんでした");
      }
      if ("namespace" in data) {
        setServices(data.services ?? []);
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

  async function loadServiceLogs(name: string) {
    serviceLogsAbortRef.current?.abort();
    const controller = new AbortController();
    serviceLogsAbortRef.current = controller;
    setServiceLogsLoading(true);
    setServiceLogsError("");
    try {
      const response = await fetch(`/api/v1/services/${encodeURIComponent(name)}/logs?tailLines=300`, {
        credentials: "include",
        headers: apiHeaders(),
        signal: controller.signal
      });
      const text = await response.text();
      if (controller.signal.aborted) {
        return;
      }
      if (!response.ok) {
        try {
          const data = JSON.parse(text) as { error?: string };
          throw new Error(data.error ?? (text || "ログを読み込めませんでした"));
        } catch {
          throw new Error(text || "ログを読み込めませんでした");
        }
      }
      setServiceLogs(text);
    } catch (logsError) {
      if (logsError instanceof DOMException && logsError.name === "AbortError") {
        return;
      }
      setServiceLogs("");
      setServiceLogsError(logsError instanceof Error ? logsError.message : "ログを読み込めませんでした");
    } finally {
      if (!controller.signal.aborted) {
        setServiceLogsLoading(false);
      }
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
      const data = (await response.json()) as Project | { error?: string };
      if (!response.ok) {
        throw new Error("error" in data && data.error ? data.error : "プロジェクトの作成に失敗しました");
      }
      if ("id" in data) {
        setProjects((current) => [...current, data]);
        handleProjectSelect(data.id);
        setProjectName("");
        setShowProjectCreateForm(false);
        setMessage(`${data.name} を作成しました`);
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
      const response = await fetch("/api/v1/services", {
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

      const data = (await response.json()) as DeployedService | { error?: string };
      if (!response.ok) {
        throw new Error("error" in data && data.error ? data.error : "サービスの作成に失敗しました");
      }
      if ("name" in data) {
        setMessage(`${data.name} を作成しました`);
        handleOpenService(data.name);
      }
      setForm((current) => ({ ...current, name: "hello-dcp", port: "8080", minScale: "0", maxScale: "20" }));
      await loadServices();
    } catch (deployError) {
      setError(deployError instanceof Error ? deployError.message : "サービスの作成に失敗しました");
    } finally {
      setSubmitting(false);
    }
  }

  function requestDelete(name: string) {
    setPendingDeleteName(name);
  }

  function cancelDelete() {
    setPendingDeleteName("");
  }

  function requestDeleteProject(projectId: string) {
    setPendingProjectDeleteId(projectId);
  }

  function cancelProjectDelete() {
    setPendingProjectDeleteId("");
  }

  async function confirmDelete(name: string) {
    setPendingDeleteName("");
    setDeletingName(name);
    setError("");
    setMessage("");
    try {
      const response = await fetch(`/api/v1/services/${name}`, {
        method: "DELETE",
        credentials: "include",
        headers: apiHeaders()
      });
      if (!response.ok && response.status !== 204) {
        const data = (await response.json()) as { error?: string };
        throw new Error(data.error ?? "サービスの削除に失敗しました");
      }
      setMessage(`${name} を削除しました`);
      if (route.selectedServiceName === name) {
        window.location.hash = "#container";
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
        const data = (await response.json()) as { error?: string };
        throw new Error(data.error ?? "プロジェクトの削除に失敗しました");
      }
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
    authLoading,
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
    handleProjectSelect,
    handleOpenService,
    handleSubmit,
    loadCurrentUser,
    loadServiceLogs,
    loading,
    message,
    pendingDeleteName,
    pendingProjectDeleteId,
    projectName,
    projects,
    requestDelete,
    requestDeleteProject,
    route,
    selectedService,
    selectedStatus,
    serviceLogs,
    serviceLogsError,
    serviceLogsLoading,
    setProjectName,
    setRoute,
    setSidebarOpen,
    sidebarOpen,
    services,
    setShowProjectCreateForm,
    showProjectCreateForm,
    startLogin,
    startLogout,
    startRegister,
    submitting,
    setMessage
  } as const;
}
