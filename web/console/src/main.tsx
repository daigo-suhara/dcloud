import React, { useEffect, useState } from "react";
import { createRoot } from "react-dom/client";
import "./styles.css";

type PlatformResponse = {
  namespace: string;
  services: DeployedService[];
};

type DeployedService = {
  name: string;
  image: string;
  url?: string;
  ready: boolean;
  reason?: string;
  createdAt?: string;
  namespace: string;
  generation?: number;
};

type DeployForm = {
  name: string;
  image: string;
  port: string;
};

const initialForm: DeployForm = {
  name: "",
  image: "",
  port: ""
};

const navItems = [
  { id: "home", label: "コンテナ" },
  { id: "deploy", label: "VM" },
  { id: "services", label: "ネットワーク" }
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
  if (section === "services" && rest.length > 0) {
    return {
      section: "services",
      selectedServiceName: decodeURIComponent(rest.join("/"))
    };
  }

  if (navItems.some((item) => item.id === section)) {
    return { section: section as RouteState["section"], selectedServiceName: null };
  }

  return { section: "home", selectedServiceName: null };
}

function App() {
  const [namespace, setNamespace] = useState("dcp-system");
  const [services, setServices] = useState<DeployedService[]>([]);
  const [route, setRoute] = useState<RouteState>(() => parseRoute(window.location.hash));
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [deletingName, setDeletingName] = useState("");
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [form, setForm] = useState(initialForm);
  const selectedService = services.find((service) => service.name === route.selectedServiceName) ?? null;

  useEffect(() => {
    const handleHashChange = () => {
      setRoute(parseRoute(window.location.hash));
    };

    window.addEventListener("hashchange", handleHashChange);
    void loadServices();
    const timer = window.setInterval(() => {
      void loadServices({ quiet: true });
    }, 5000);

    return () => {
      window.clearInterval(timer);
      window.removeEventListener("hashchange", handleHashChange);
    };
  }, []);

  async function loadServices(options?: { quiet?: boolean }) {
    if (!options?.quiet) {
      setLoading(true);
      setError("");
    }
    try {
      const response = await fetch("/api/v1/services");
      const data = (await response.json()) as PlatformResponse | { error?: string };
      if (!response.ok) {
        throw new Error("error" in data && data.error ? data.error : "failed to load services");
      }
      if ("namespace" in data) {
        setNamespace(data.namespace);
        setServices(data.services ?? []);
      }
    } catch (loadError) {
      if (!options?.quiet) {
        setError(loadError instanceof Error ? loadError.message : "failed to load services");
      }
    } finally {
      if (!options?.quiet) {
        setLoading(false);
      }
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
        headers: {
          "Content-Type": "application/json"
        },
        body: JSON.stringify({
          name: form.name.trim(),
          image: form.image.trim(),
          port: Number(form.port || "8080")
        })
      });

      const data = (await response.json()) as DeployedService | { error?: string };
      if (!response.ok) {
        throw new Error("error" in data && data.error ? data.error : "failed to deploy service");
      }

      if ("name" in data && "namespace" in data) {
        setMessage(`${data.name} を作成しました`);
      }
      setForm((current) => ({ ...current, name: "hello-dcp" }));
      await loadServices();
    } catch (deployError) {
      setError(deployError instanceof Error ? deployError.message : "failed to deploy service");
    } finally {
      setSubmitting(false);
    }
  }

  async function handleDelete(name: string) {
    if (!window.confirm(`${name} を削除しますか？`)) {
      return;
    }

    setDeletingName(name);
    setError("");
    setMessage("");

    try {
      const response = await fetch(`/api/v1/services/${name}`, {
        method: "DELETE"
      });

      if (!response.ok && response.status !== 204) {
        const data = (await response.json()) as { error?: string };
        throw new Error(data.error ?? "failed to delete service");
      }

      setMessage(`${name} を削除しました`);
      if (route.selectedServiceName === name) {
        window.location.hash = "#services";
      }
      await loadServices();
    } catch (deleteError) {
      setError(deleteError instanceof Error ? deleteError.message : "failed to delete service");
    } finally {
      setDeletingName("");
    }
  }

  return (
    <main className="app-shell">
      <aside className="sidebar" aria-label="navigation">
        <div className="sidebar-brand">dcp</div>
        <div className="sidebar-divider" aria-hidden="true" />
        <nav className="sidebar-nav">
          {navItems.map((item) => (
            <a
              key={item.id}
              className={`nav-item ${route.section === item.id ? "active" : ""}`}
              href={`#${item.id}`}
            >
              <span className="nav-icon" aria-hidden="true">
                {item.id === "home" ? <ContainerIcon /> : item.id === "deploy" ? <VmIcon /> : <NetworkIcon />}
              </span>
              <span className="nav-copy">
                <strong>{item.label}</strong>
              </span>
            </a>
          ))}
        </nav>
      </aside>

      <section className="content">
        <section className="dashboard-grid" aria-label="deployment-console">
          <form className="deploy-card" id="deploy" onSubmit={handleSubmit}>
            <div className="panel-header">
              <div>
                <p className="panel-kicker">サービスの作成</p>
                <h2>コンテナを作成</h2>
              </div>
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
            </div>

            <div className="actions">
              <button className="pill primary button" type="submit" disabled={submitting}>
                {submitting ? "Deploying..." : "作成"}
              </button>
            </div>

            {message ? <p className="status-banner success">{message}</p> : null}
            {error ? <p className="status-banner error">{error}</p> : null}
          </form>

          <section className="service-card panel" id="services">
            <div className="panel-header">
              <div>
                <p className="panel-kicker">サービス</p>
                <h2>デプロイ済みサービス</h2>
              </div>
              {selectedService ? (
                <a className="detail-back" href="#services">
                  一覧に戻る
                </a>
              ) : null}
            </div>

            {selectedService ? (
              <section className="service-detail" aria-label={`${selectedService.name} details`}>
                <div className="service-detail-head">
                  <div className="service-detail-title">
                    <span className={`status-icon ${getServiceStatus(selectedService)}`} aria-hidden="true">
                      {getServiceStatus(selectedService) === "ready" ? (
                        <CheckIcon />
                      ) : getServiceStatus(selectedService) === "loading" ? (
                        <LoadingIcon />
                      ) : (
                        <ErrorIcon />
                      )}
                    </span>
                    <div>
                      <p className="panel-kicker">サービス詳細</p>
                      <h3>{selectedService.name}</h3>
                    </div>
                  </div>
                  <button
                    className="pill danger button"
                    type="button"
                    onClick={() => handleDelete(selectedService.name)}
                    disabled={deletingName === selectedService.name}
                  >
                    {deletingName === selectedService.name ? "削除中..." : "削除"}
                  </button>
                </div>

                <dl className="detail-grid">
                  <div>
                    <dt>状態</dt>
                    <dd>{formatServiceStatus(selectedService)}</dd>
                  </div>
                  <div>
                    <dt>コンテナイメージ</dt>
                    <dd>{selectedService.image}</dd>
                  </div>
                  <div>
                    <dt>Namespace</dt>
                    <dd>{selectedService.namespace}</dd>
                  </div>
                  <div>
                    <dt>作成時刻</dt>
                    <dd>{selectedService.createdAt ?? "just now"}</dd>
                  </div>
                  <div>
                    <dt>URL</dt>
                    <dd>{selectedService.url ? <a href={selectedService.url}>{selectedService.url}</a> : "none"}</dd>
                  </div>
                  {selectedService.reason ? (
                    <div>
                      <dt>理由</dt>
                      <dd>{selectedService.reason}</dd>
                    </div>
                  ) : null}
                </dl>
              </section>
            ) : (
              <div className="service-list">
                {services.length > 0 ? (
                  services.map((service) => {
                    const status = getServiceStatus(service);
                    return (
                      <article className="service-row" key={service.name}>
                        <span className={`status-icon ${status}`} aria-hidden="true">
                          {status === "ready" ? <CheckIcon /> : status === "loading" ? <LoadingIcon /> : <ErrorIcon />}
                        </span>
                        <a className="service-name-link" href={`#services/${encodeURIComponent(service.name)}`}>
                          {service.name}
                        </a>
                      </article>
                    );
                  })
                ) : (
                  <div className="empty-state">
                    <p>{loading ? "Loading..." : "まだサービスはありません。"}</p>
                  </div>
                )}
              </div>
            )}
          </section>
        </section>
      </section>
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
    return "Ready";
  }

  return service.reason ?? "Pending";
}

function ContainerIcon() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true">
      <rect x="4" y="6" width="16" height="12" rx="3" />
      <path d="M8 6v12" />
      <path d="M16 6v12" />
      <path d="M4 12h16" />
    </svg>
  );
}

function VmIcon() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true">
      <rect x="5" y="5" width="14" height="14" rx="3" />
      <path d="M8 9h8" />
      <path d="M8 12h5" />
      <path d="M8 15h6" />
    </svg>
  );
}

function NetworkIcon() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true">
      <path d="M12 5v14" />
      <path d="M6 9h12" />
      <path d="M7.5 15.5 12 20l4.5-4.5" />
      <circle cx="12" cy="5" r="1.5" />
      <circle cx="6" cy="9" r="1.5" />
      <circle cx="18" cy="9" r="1.5" />
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

createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
