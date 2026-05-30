import React from "react";
import { createRoot } from "react-dom/client";

function App() {
  return (
    <main className="page">
      <section className="card">
        <p className="eyebrow">CloudRun sample</p>
        <h1>Hello, world!</h1>
        <p className="lead">
          これは `dcp` の CloudRun でデプロイするための、React ベースのサンプルコンテナです。
        </p>
        <div className="badge-row">
          <span className="badge">React</span>
          <span className="badge">OCI</span>
          <span className="badge">8080</span>
        </div>
      </section>
    </main>
  );
}

createRoot(document.getElementById("root")).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
