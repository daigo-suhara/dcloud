import * as esbuild from "esbuild";
import { copyFile, mkdir, readFile, writeFile } from "node:fs/promises";
import { execFileSync } from "node:child_process";

await mkdir("dist", { recursive: true });
await mkdir(".generated", { recursive: true });
const celebrationSvg = await readFile("assets/celebration.svg", "utf8");
await writeFile(
  ".generated/celebrationSvg.js",
  `export default "data:image/svg+xml;base64,${Buffer.from(celebrationSvg).toString("base64")}";\n`
);

await esbuild.build({
  entryPoints: ["src/main.jsx"],
  bundle: true,
  outfile: "dist/bundle.js",
  format: "iife",
  loader: {
    ".js": "jsx",
    ".jsx": "jsx"
  },
  define: {
    "process.env.NODE_ENV": '"production"'
  },
  minify: true,
  target: ["es2020"]
});

execFileSync(
  "./node_modules/.bin/tailwindcss",
  ["-i", "src/styles.css", "-o", "dist/styles.css", "--minify"],
  { stdio: "inherit" }
);

await copyFile("index.html", "dist/index.html");
