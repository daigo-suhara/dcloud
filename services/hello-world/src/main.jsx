import React from "react";
import { createRoot } from "react-dom/client";
import celebrationSvg from "../.generated/celebrationSvg.js";

function App() {
  return (
    <main className="relative grid min-h-screen place-items-center overflow-hidden px-4 py-8 sm:px-6">
      <div
        className="pointer-events-none absolute left-[8%] top-[8%] h-56 w-56 rounded-full bg-[rgba(139,208,221,0.18)] blur-xl"
        aria-hidden="true"
      />
      <div
        className="pointer-events-none absolute bottom-[8%] right-[8%] h-72 w-72 rounded-full bg-[rgba(227,131,168,0.14)] blur-xl"
        aria-hidden="true"
      />

      <section className="relative z-10 grid w-full max-w-3xl justify-items-center gap-5 rounded-[32px] border border-[rgba(119,119,119,0.08)] bg-white/90 px-5 py-8 text-center shadow-soft backdrop-blur-md sm:px-8 sm:py-10">
        <img
          className="w-full max-w-[260px] select-none"
          src={celebrationSvg}
          alt="Celebration illustration"
        />

        <div className="grid w-full max-w-2xl justify-items-center gap-3">
          <h1 className="m-0 text-[clamp(22px,4vw,34px)] font-extrabold leading-[1.2] tracking-[0.08em] text-text sm:tracking-[0.1em]">
            正常に動作しています
          </h1>
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
