import React, { useEffect, useState } from "react";
import { createRoot } from "react-dom/client";
import { HiOutlineHome, HiOutlineCloud } from "react-icons/hi2";
import { HiOutlineServerStack } from "react-icons/hi2";
import { LuContainer } from "react-icons/lu";
import { SiGithub, SiDocker } from "react-icons/si";
import "./styles.css";

type PlatformResponse = {
  namespace: string;
  user: string;
  projectId: string;
  services: DeployedService[];
};

type ProjectsResponse = {
  user: string;
  projects: Project[];
  defaultProjectId: string;
};

type AuthUser = {
  id: string;
  username: string;
  email?: string;
  name?: string;
};

type Project = {
  id: string;
  name: string;
  owner: string;
  createdAt: string;
};

type DeployedService = {
  name: string;
  image: string;
  url?: string;
  ready: boolean;
  reason?: string;
  createdAt?: string;
  updatedAt?: string;
  namespace: string;
  projectId?: string;
  generation?: number;
};

type DeployForm = {
  name: string;
  image: string;
  port: string;
  minScale: string;
  maxScale: string;
};

const initialForm: DeployForm = {
  name: "",
  image: "",
  port: "",
  minScale: "0",
  maxScale: "1"
};

const navItems = [
  { id: "home", label: "ホーム" },
  { id: "container", label: "コンテナ" },
  { id: "deploy", label: "仮想マシン" }
] as const;

type RouteState = {
  section: (typeof navItems)[number]["id"];
  selectedServiceName: string | null;
};

function parseRoute(hash: string): RouteState {
  const route = hash.replace(/^#/, "");

  if (!route) {
    return { section: "home", selectedServiceName: null };
  }

  const [section, ...rest] = route.split("/");
  const normalizedSection = section === "services" ? "container" : section;

  if (normalizedSection === "container" && rest.length > 0) {
    return {
      section: "container",
      selectedServiceName: decodeURIComponent(rest.join("/"))
    };
  }

  if (navItems.some((item) => item.id === normalizedSection)) {
    return { section: normalizedSection as RouteState["section"], selectedServiceName: null };
  }

  return { section: "home", selectedServiceName: null };
}

function App() {
  const [currentUser, setCurrentUser] = useState<AuthUser | null>(null);
  const [services, setServices] = useState<DeployedService[]>([]);
  const [projects, setProjects] = useState<Project[]>([]);
  const [activeProjectId, setActiveProjectId] = useState("");
  const [projectName, setProjectName] = useState("");
  const [creatingProject, setCreatingProject] = useState(false);
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
  const selectedService =
    route.section === "container" && route.selectedServiceName
      ? services.find((service) => service.name === route.selectedServiceName)
      : null;
  const selectedStatus = selectedService ? getServiceStatus(selectedService) : null;

  useEffect(() => {
    if (!message) {
      return;
    }

    const timer = window.setTimeout(() => {
      setMessage("");
    }, 3500);

    return () => {
      window.clearTimeout(timer);
    };
  }, [message]);

  useEffect(() => {
    const handleHashChange = () => {
      setRoute(parseRoute(window.location.hash));
    };

    window.addEventListener("hashchange", handleHashChange);
    void loadCurrentUser();

    return () => {
      window.removeEventListener("hashchange", handleHashChange);
    };
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

    return () => {
      window.clearInterval(timer);
    };
  }, [activeProjectId]);

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

  async function loadServices(options?: { quiet?: boolean }) {
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

  async function handleCreateProject(event: React.FormEvent<HTMLFormElement>) {
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
        setMessage(`${data.name} を作成しました`);
      }
    } catch (projectError) {
      setError(projectError instanceof Error ? projectError.message : "プロジェクトの作成に失敗しました");
    } finally {
      setCreatingProject(false);
    }
  }

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
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
      }
      setForm((current) => ({ ...current, name: "hello-dcp", minScale: "0", maxScale: "1" }));
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

  if (authLoading) {
    return (
      <main className="app-shell">
        <section className="auth-shell">
          <div className="auth-card panel">
            <p className="panel-kicker">D Cloud</p>
            <h1>読み込み中</h1>
          </div>
        </section>
      </main>
    );
  }

  if (!currentUser) {
    return (
      <main className="app-shell auth-page">
        <section className="auth-shell">
          <div className="auth-card panel">
            <div className="panel-header">
              <div>
                <p className="panel-kicker">D Cloud</p>
                <h1>Keycloak</h1>
              </div>
            </div>
            <div className="auth-actions">
              <button className="pill primary button" type="button" onClick={startLogin}>
                ログイン
              </button>
              <button className="pill button" type="button" onClick={startRegister}>
                ユーザー登録
              </button>
            </div>
            {error ? <p className="status-banner error">{error}</p> : null}
          </div>
        </section>
      </main>
    );
  }

  return (
    <main className={`app-shell ${sidebarOpen ? "sidebar-open" : "sidebar-collapsed"}`}>
      {message ? (
        <div className="toast toast-success" role="status" aria-live="polite">
          {message}
        </div>
      ) : null}
      <header className="app-header">
        <button
          className="header-toggle"
          type="button"
          aria-expanded={sidebarOpen}
          aria-controls="sidebar-navigation"
          onClick={() => setSidebarOpen((current) => !current)}
        >
          <span className="header-toggle-icon" aria-hidden="true">
            <HamburgerIcon />
          </span>
        </button>
        <div className="brand-slot">
          <BrandLogo />
        </div>
        <button className="project-create-button logout-button header-logout" type="button" onClick={startLogout}>
          ログアウト
        </button>
      </header>
      {sidebarOpen ? <div className="sidebar-backdrop" role="presentation" onClick={() => setSidebarOpen(false)} /> : null}
      <aside className={`sidebar ${sidebarOpen ? "open" : ""}`} aria-label="navigation">
        <div className="sidebar-top">
          <div className="brand-slot">
            <BrandLogo />
          </div>
        </div>
        <button
          className="sidebar-close-button"
          type="button"
          aria-label="サイドバーを閉じる"
          onClick={() => setSidebarOpen(false)}
        >
          <span className="sidebar-close-icon" aria-hidden="true">
            <CloseIcon />
          </span>
        </button>
        <div className="sidebar-divider" aria-hidden="true" />
        <nav className="sidebar-nav" id="sidebar-navigation">
          {navItems.map((item) => (
            <a
              key={item.id}
              className={`nav-item ${route.section === item.id ? "active" : ""}`}
              href={`#${item.id}`}
              onClick={() => {
                if (window.matchMedia("(max-width: 760px)").matches) {
                  setSidebarOpen(false);
                }
              }}
            >
              <span className="nav-icon" aria-hidden="true">
                {item.id === "home" ? (
                  <HiOutlineHome />
                ) : item.id === "container" ? (
                  <LuContainer />
                ) : (
                  <HiOutlineServerStack />
                )}
              </span>
              <span className="nav-copy">
                <strong>{item.label}</strong>
              </span>
            </a>
          ))}
        </nav>
      </aside>

      <section className="content">
        {route.section === "home" ? (
          <section className="home-stack">
            <section className="panel project-panel" id="projects" aria-label="project-management">
              <div className="panel-header">
                <div>
                  <p className="panel-kicker">プロジェクト</p>
                  <h2>切り替えと管理</h2>
                </div>
              </div>

              <div className="project-panel-body">
                <label className="field project-select-field">
                  <span className="field-label">プロジェクトを切り替え</span>
                  <select
                    className="project-select project-select-inline"
                    value={activeProjectId}
                    onChange={(event) => handleProjectSelect(event.target.value)}
                  >
                    {projects.map((project) => (
                      <option key={project.id} value={project.id}>
                        {project.name}
                      </option>
                    ))}
                  </select>
                </label>

                <form className="project-create project-create-inline" onSubmit={handleCreateProject}>
                  <label className="field project-create-field">
                    <span className="field-label">プロジェクトを作成</span>
                    <input
                      className="text-input project-input"
                      value={projectName}
                      onChange={(event) => setProjectName(event.target.value)}
                      placeholder="新しいプロジェクト"
                      aria-label="新しいプロジェクト"
                    />
                  </label>
                  <div className="project-create-actions">
                    <button className="project-create-button" type="submit" disabled={creatingProject || !projectName.trim()}>
                      作成
                    </button>
                  </div>
                </form>
              </div>

              <div className="project-list">
                {projects.map((project) => {
                  const isActive = project.id === activeProjectId;
                  const canDelete = project.name !== "default";
                  return (
                    <article className={`project-row ${isActive ? "active" : ""}`} key={project.id}>
                      <button
                        className="project-row-main"
                        type="button"
                        onClick={() => handleProjectSelect(project.id)}
                        aria-pressed={isActive}
                      >
                        <span className="project-row-title">
                          <strong>{project.name}</strong>
                          {isActive ? <span className="project-badge">現在使用中</span> : null}
                        </span>
                        <span className="project-row-meta">{formatServiceTimestamp(project.createdAt)}</span>
                      </button>
                      <button
                        className="project-row-delete"
                        type="button"
                        disabled={!canDelete || deletingProjectId === project.id}
                        onClick={() => requestDeleteProject(project.id)}
                      >
                        {deletingProjectId === project.id ? "削除中..." : "削除"}
                      </button>
                    </article>
                  );
                })}
              </div>
            </section>

          </section>
        ) : route.section === "container" ? (
          <section className="service-rail" aria-label="container-services">
            {route.selectedServiceName ? (
              <section className="service-card panel service-detail-card" aria-label="service-detail">
                {selectedService ? (
                  <>
                    <div className="service-detail-head">
                      <div className="service-detail-title">
                        <span className={`status-icon ${selectedStatus}`} aria-hidden="true">
                          {selectedStatus === "ready" ? (
                            <CheckIcon />
                          ) : selectedStatus === "loading" ? (
                            <LoadingIcon />
                          ) : (
                            <ErrorIcon />
                          )}
                        </span>
                        <h3>{selectedService.name}</h3>
                      </div>
                      <a className="detail-back" href="#container">
                        一覧に戻る
                      </a>
                    </div>

                    <div className="detail-grid">
                      <div>
                        <dt>状態</dt>
                        <dd>{formatServiceStatus(selectedService)}</dd>
                      </div>
                      <div>
                        <dt>イメージ</dt>
                        <dd>{selectedService.image}</dd>
                      </div>
                      <div>
                        <dt>URL</dt>
                        <dd>
                          {selectedService.url ? (
                            <a href={selectedService.url} target="_blank" rel="noreferrer">
                              {selectedService.url}
                            </a>
                          ) : (
                            "-"
                          )}
                        </dd>
                      </div>
                      <div>
                        <dt>作成時刻</dt>
                        <dd>{selectedService.createdAt ?? "-"}</dd>
                      </div>
                    </div>

                    <div className="delete-actions detail-actions">
                      <button className="pill danger button" type="button" onClick={() => requestDelete(selectedService.name)}>
                        削除
                      </button>
                    </div>
                  </>
                ) : (
                  <>
                    <div className="service-detail-head">
                      <div className="service-detail-title">
                        <h3>サービスが見つかりません</h3>
                      </div>
                      <a className="detail-back" href="#container">
                        一覧に戻る
                      </a>
                    </div>
                    <p className="service-detail-empty">削除されたか、まだ同期されていません。</p>
                  </>
                )}
              </section>
            ) : (
              <section className="service-card panel service-list-card" id="container">
                <div className="panel-header">
                  <div>
                    <p className="panel-kicker">サービス</p>
                    <h2>デプロイ済みサービス</h2>
                  </div>
                </div>

                <div className="service-list-table">
                  <div className="service-list-head" aria-hidden="true">
                    <span className="service-list-head-status" />
                    <span className="service-list-head-main">
                      <span className="service-list-head-name">名前</span>
                      <span className="service-list-head-updated">更新日時</span>
                    </span>
                  </div>

                  <div className="service-list">
                    {services.length > 0 ? (
                      services.map((service) => {
                        const status = getServiceStatus(service);
                        return (
                          <article className="service-row" key={service.name}>
                            <span className="service-cell service-cell-status" aria-hidden="true">
                              <span className={`status-icon ${status}`}>
                                {status === "ready" ? <CheckIcon /> : status === "loading" ? <LoadingIcon /> : <ErrorIcon />}
                              </span>
                            </span>
                            <span className="service-cell service-cell-name">
                              <a className="service-name-link" href={`#container/${encodeURIComponent(service.name)}`}>
                                <span className="service-name-text">{service.name}</span>
                              </a>
                              <span className="service-updated-inline">
                                {service.updatedAt || service.createdAt
                                  ? formatServiceTimestamp(service.updatedAt || service.createdAt || "")
                                  : "-"}
                              </span>
                            </span>
                          </article>
                        );
                      })
                    ) : (
                      <div className="empty-state">
                        <p>{loading ? "読み込み中..." : "まだサービスはありません。"}</p>
                      </div>
                    )}
                  </div>
                </div>
              </section>
            )}

            <section className="service-card panel deploy-panel" aria-label="サービスのデプロイ">
              <div className="panel-header">
                <div>
                  <p className="panel-kicker">作成</p>
                  <h2>サービスのデプロイ</h2>
                </div>
              </div>

              <div className="deploy-panel-grid">
                <a className="deploy-launch-card" href="#deploy" aria-label="コンテナのデプロイ">
                  <span className="deploy-launch-icon" aria-hidden="true">
                    <SiDocker />
                  </span>
                  <span className="deploy-launch-label">コンテナのデプロイ</span>
                </a>

                <a className="deploy-launch-card deploy-launch-secondary" href="#container" aria-label="リポジトリの接続">
                  <span className="deploy-launch-icon deploy-launch-glyph" aria-hidden="true">
                    <SiGithub />
                  </span>
                  <span className="deploy-launch-label">リポジトリの接続</span>
                </a>
              </div>
            </section>
          </section>
        ) : route.section === "deploy" ? (
          <form className="deploy-card" id="deploy" onSubmit={handleSubmit}>
            <div className="panel-header">
              <div>
                <p className="panel-kicker">サービスの作成</p>
                <h2>コンテナを作成</h2>
              </div>
              <a className="detail-back" href="#home">
                一覧に戻る
              </a>
            </div>

            <div className="field-grid">
              <label className="field">
                <span className="field-label">サービス名</span>
                <input
                  className="text-input"
                  value={form.name}
                  onChange={(event) => setForm((current) => ({ ...current, name: event.target.value }))}
                  placeholder="service-name"
                  autoComplete="off"
                />
              </label>

              <label className="field">
                <span className="field-label">コンテナイメージのURL</span>
                <input
                  className="text-input"
                  value={form.image}
                  onChange={(event) => setForm((current) => ({ ...current, image: event.target.value }))}
                  placeholder="ghcr.io/org/app:tag"
                  autoComplete="off"
                />
              </label>

              <label className="field">
                <span className="field-label">Port</span>
                <input
                  className="text-input"
                  type="number"
                  min={1}
                  max={65535}
                  value={form.port}
                  onChange={(event) => setForm((current) => ({ ...current, port: event.target.value }))}
                  placeholder="8080"
                />
              </label>

              <label className="field">
                <span className="field-label">最小スケール数</span>
                <input
                  className="text-input"
                  type="number"
                  min={0}
                  max={20}
                  value={form.minScale}
                  onChange={(event) => setForm((current) => ({ ...current, minScale: event.target.value }))}
                  placeholder="0"
                />
              </label>

              <label className="field">
                <span className="field-label">最大スケール数</span>
                <input
                  className="text-input"
                  type="number"
                  min={1}
                  max={20}
                  value={form.maxScale}
                  onChange={(event) => setForm((current) => ({ ...current, maxScale: event.target.value }))}
                  placeholder="1"
                />
              </label>

            </div>

            <div className="actions">
              <button className="pill primary button" type="submit" disabled={submitting}>
                {submitting ? "作成中..." : "作成"}
              </button>
            </div>

            {error ? <p className="status-banner error">{error}</p> : null}
          </form>
        ) : null}
      </section>

      {pendingDeleteName ? (
        <div className="delete-overlay" role="presentation" onClick={cancelDelete}>
          <section
            className="delete-modal"
            role="dialog"
            aria-modal="true"
            aria-label="削除の確認"
            onClick={(event) => event.stopPropagation()}
          >
            <p className="panel-kicker">削除の確認</p>
            <h3>{pendingDeleteName}</h3>
            <p>このサービスを削除しますか？</p>
            <div className="delete-actions">
              <button className="pill button delete-cancel" type="button" onClick={cancelDelete}>
                キャンセル
              </button>
              <button
                className="pill danger button"
                type="button"
                onClick={() => confirmDelete(pendingDeleteName)}
                disabled={deletingName === pendingDeleteName}
              >
                {deletingName === pendingDeleteName ? "削除中..." : "削除"}
              </button>
            </div>
          </section>
        </div>
      ) : null}

      {pendingProjectDeleteId ? (
        <div className="delete-overlay" role="presentation" onClick={cancelProjectDelete}>
          <section
            className="delete-modal"
            role="dialog"
            aria-modal="true"
            aria-label="削除の確認"
            onClick={(event) => event.stopPropagation()}
          >
            <p className="panel-kicker">削除の確認</p>
            <h3>プロジェクトの削除</h3>
            <p>このプロジェクトを削除しますか？</p>
            <div className="delete-actions">
              <button className="pill button delete-cancel" type="button" onClick={cancelProjectDelete}>
                キャンセル
              </button>
              <button
                className="pill danger button"
                type="button"
                onClick={() => confirmDeleteProject(pendingProjectDeleteId)}
                disabled={deletingProjectId === pendingProjectDeleteId}
              >
                {deletingProjectId === pendingProjectDeleteId ? "削除中..." : "削除"}
              </button>
            </div>
          </section>
        </div>
      ) : null}
    </main>
  );
}

function getServiceStatus(service: DeployedService) {
  if (service.ready) {
    return "ready" as const;
  }

  const reason = service.reason?.toLowerCase() ?? "";
  if (
    reason.includes("pending") ||
    reason.includes("loading") ||
    reason.includes("progress") ||
    reason.includes("reconcil") ||
    reason.includes("revisionmissing") ||
    reason.includes("unknown")
  ) {
    return "loading" as const;
  }

  return "error" as const;
}

function formatServiceStatus(service: DeployedService) {
  if (service.ready) {
    return "正常";
  }

  return formatServiceReason(service.reason);
}

function formatServiceReason(reason?: string) {
  switch (reason) {
    case "RevisionMissing":
      return "リビジョンを準備中です";
    case "RevisionFailed":
      return "リビジョンの作成に失敗しました";
    case "ContainerMissing":
      return "コンテナが見つかりません";
    case "ContainerCreating":
      return "コンテナを作成中です";
    case "ImagePullBackOff":
      return "イメージの取得に失敗しました";
    case "ErrImagePull":
      return "イメージ取得エラーです";
    default:
      return "処理中です";
  }
}

function formatServiceTimestamp(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }

  return new Intl.DateTimeFormat("ja-JP", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit"
  }).format(date);
}

function HamburgerIcon() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true">
      <path d="M4 7h16" />
      <path d="M4 12h16" />
      <path d="M4 17h16" />
    </svg>
  );
}

function CloseIcon() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true">
      <path d="M6 6l12 12" />
      <path d="M18 6 6 18" />
    </svg>
  );
}

function CheckIcon() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true">
      <path d="m5 12 4.2 4.2L19 6.5" />
    </svg>
  );
}

function ErrorIcon() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true">
      <path d="M7 7l10 10" />
      <path d="M17 7 7 17" />
    </svg>
  );
}

function LoadingIcon() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true" className="loading-icon">
      <circle cx="12" cy="12" r="8" />
      <path d="M12 4a8 8 0 0 1 8 8" />
    </svg>
  );
}

function BrandLogo() {
  return (
    <span className="brand-logo" aria-hidden="true">
      <HiOutlineCloud />
      <span className="brand-logo-text">D Cloud</span>
    </span>
  );
}

createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
