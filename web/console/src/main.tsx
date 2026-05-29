import React from "react";
import { createRoot } from "react-dom/client";
import "./styles.css";

const services = [
  { name: "Control Plane", status: "稼働準備", color: "cyan" },
  { name: "Web Console", status: "開発中", color: "pink" },
  { name: "CloudRun", status: "API追加済み", color: "green" }
];

function App() {
  return (
    <main className="page-shell">
      <section className="hero-card">
        <p className="eyebrow">Distributed Cloud Platform</p>
        <h1>k8s の上に、やわらかく扱えるクラウドを作る。</h1>
        <p className="lead">
          dcp は Kubernetes 上に GCP のような開発者体験を構築する OSS プロジェクトです。
          control-plane、console、CloudRun API、Helm chart をOCIコンテナとして育てます。
        </p>
        <div className="actions">
          <a className="pill primary" href="/api/v1/platform">API を確認</a>
          <a className="pill tertiary" href="/api/v1/cloudrun/services">CloudRun API</a>
          <a className="pill secondary" href="https://github.com/">GitHub Actions</a>
        </div>
      </section>

      <section className="service-grid" aria-label="services">
        {services.map((service) => (
          <article className={`service-card ${service.color}`} key={service.name}>
            <span className="status">{service.status}</span>
            <h2>{service.name}</h2>
            <p>OCI コンテナとしてビルドし、Helm で Kubernetes へ配備します。</p>
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
