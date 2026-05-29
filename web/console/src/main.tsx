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

const featureCards = [
  { name: "Control Plane", status: "稼働準備", color: "cyan" },
  { name: "Web Console", status: "デプロイ可能", color: "pink" },
  { name: "Knative", status: "サービス展開中", color: "green" }
];

const initialForm: DeployForm = {
  name: "hello-dcp",
  image: "ghcr.io/daigo-suhara/hello-dcp:latest",
  port: "8080"
};

function App() {
  const [namespace, setNamespace] = useState("dcp-system");
  const [services, setServices] = useState<DeployedService[]>([]);
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [deletingName, setDeletingName] = useState("");
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [form, setForm] = useState(initialForm);

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
    <main className="page-shell">
      <section className="hero-card">
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
          <a className="pill secondary" href="https://github.com/">
            GitHub Actions
          </a>
        </div>
      </section>

      <section className="dashboard-grid" aria-label="deployment-console">
        <form className="deploy-card" onSubmit={handleSubmit}>
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

        <section className="service-card panel">
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

      <section className="service-grid" aria-label="services">
        {featureCards.map((service) => (
          <article className={`service-card ${service.color}`} key={service.name}>
            <span className="status">{service.status}</span>
            <h2>{service.name}</h2>
            <p>OCI コンテナを Knative Service として展開し、namespace 単位で扱います。</p>
          </article>
        ))}
      </section>
    </main>
  );
}

createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
