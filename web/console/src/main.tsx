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
  name: "hello-dcp",
  image: "ghcr.io/daigo-suhara/hello-dcp:latest",
  port: "8080"
};

const navItems = [
  { id: "home", label: "コンテナ" },
  { id: "deploy", label: "VM" },
  { id: "services", label: "ネットワーク" }
] as const;

function App() {
  const [namespace, setNamespace] = useState("dcp-system");
  const [services, setServices] = useState<DeployedService[]>([]);
  const [activeSection, setActiveSection] = useState<(typeof navItems)[number]["id"]>("home");
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [deletingName, setDeletingName] = useState("");
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [form, setForm] = useState(initialForm);
  const readyCount = services.filter((service) => service.ready).length;

  useEffect(() => {
    void loadServices();
    const timer = window.setInterval(() => {
      void loadServices({ quiet: true });
    }, 5000);

    return () => window.clearInterval(timer);
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
        setMessage(`Deployed ${data.name} to ${data.namespace}`);
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
    if (!window.confirm(`Delete ${name}?`)) {
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

      setMessage(`Deleted ${name}`);
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
        <div className="sidebar-brand">
          <p className="eyebrow">dcp</p>
          <h2>Console</h2>
        </div>

        <nav className="sidebar-nav">
          {navItems.map((item) => (
            <a
              key={item.id}
              className={`nav-item ${activeSection === item.id ? "active" : ""}`}
              href={`#${item.id}`}
              onClick={() => setActiveSection(item.id)}
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
        <section className="hero-card" id="home">
          <p className="eyebrow">Distributed Cloud Platform</p>
          <h1>コンテナを送ると、Knative Service まで一気に展開する。</h1>
          <p className="lead">
            dcp は Kubernetes 上で、console から image を指定して Knative Service を作成する
            開発者向けプラットフォームです。デプロイ先は namespace <code>{namespace}</code> に揃えています。
          </p>
          <div className="actions">
            <a className="pill primary" href="/api/v1/platform">
              Platform API
            </a>
            <a className="pill tertiary" href="/api/v1/services">
              Deploy API
            </a>
            <a className="pill secondary" href="https://github.com/daigo-suhara/dcp/actions">
              GitHub Actions
            </a>
          </div>
        </section>

        <section className="overview-grid" aria-label="platform-overview">
          <article className="overview-card cyan">
            <p className="panel-kicker">Platform</p>
            <h2>Namespace</h2>
            <strong>{namespace}</strong>
            <span>デプロイ先はここに集約します。</span>
          </article>

          <article className="overview-card pink">
            <p className="panel-kicker">Status</p>
            <h2>Ready / Total</h2>
            <strong>
              {readyCount} / {services.length}
            </strong>
            <span>Knative Service の状態を毎回更新します。</span>
          </article>

          <article className="overview-card green">
            <p className="panel-kicker">Stack</p>
            <h2>Components</h2>
            <strong>Control Plane</strong>
            <span>Console から CloudRun を一気に展開します。</span>
          </article>
        </section>

        <section className="dashboard-grid" aria-label="deployment-console">
          <form className="deploy-card" id="deploy" onSubmit={handleSubmit}>
            <div className="panel-header">
              <div>
                <p className="panel-kicker">Deploy</p>
                <h2>コンテナをデプロイ</h2>
              </div>
              <span className="pill mini">namespace: {namespace}</span>
            </div>

            <div className="field-grid">
              <label className="field">
                <span className="field-label">Service Name</span>
                <input
                  className="text-input"
                  value={form.name}
                  onChange={(event) => setForm((current) => ({ ...current, name: event.target.value }))}
                  placeholder="hello-dcp"
                  autoComplete="off"
                />
              </label>

              <label className="field">
                <span className="field-label">Image</span>
                <input
                  className="text-input"
                  value={form.image}
                  onChange={(event) => setForm((current) => ({ ...current, image: event.target.value }))}
                  placeholder="ghcr.io/org/app:latest"
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
                />
              </label>
            </div>

            <p className="helper">
              名前は DNS label 形式、image は OCI イメージを指定します。Knative Service として作成されます。
            </p>

            <div className="actions">
              <button className="pill primary button" type="submit" disabled={submitting}>
                {submitting ? "Deploying..." : "Deploy container"}
              </button>
            </div>

            {message ? <p className="status-banner success">{message}</p> : null}
            {error ? <p className="status-banner error">{error}</p> : null}
          </form>

          <section className="service-card panel" id="services">
            <div className="panel-header">
              <div>
                <p className="panel-kicker">Services</p>
                <h2>デプロイ済みサービス</h2>
              </div>
              <span className="pill mini">{loading ? "Loading..." : `${services.length} items`}</span>
            </div>

            <div className="service-list">
              {services.length > 0 ? (
                services.map((service) => (
                  <article className="service-row" key={service.name}>
                    <div className="service-row-top">
                      <div>
                        <h3>{service.name}</h3>
                        <p>{service.image}</p>
                      </div>
                      <span className={`status ${service.ready ? "ready" : "pending"}`}>
                        {service.ready ? "Ready" : service.reason ?? "Pending"}
                      </span>
                    </div>
                    <div className="service-meta">
                      <span>{service.namespace}</span>
                      <span>{service.createdAt ?? "just now"}</span>
                      {service.url ? <a href={service.url}>{service.url}</a> : null}
                    </div>
                    <div className="service-actions">
                      <button
                        className="pill danger button"
                        type="button"
                        onClick={() => handleDelete(service.name)}
                        disabled={deletingName === service.name}
                      >
                        {deletingName === service.name ? "Deleting..." : "Delete"}
                      </button>
                    </div>
                  </article>
                ))
              ) : (
                <div className="empty-state">
                  <p>{loading ? "Loading services..." : "まだサービスはありません。"}</p>
                </div>
              )}
            </div>
          </section>
        </section>
      </section>
    </main>
  );
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

createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
