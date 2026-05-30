import { mkdir, readFile, rm, writeFile } from "node:fs/promises";

await rm("dist", { recursive: true, force: true });
await mkdir("dist", { recursive: true });

const celebrationSvg = await readFile("assets/celebration.svg", "utf8");
const celebrationDataUri = `data:image/svg+xml;base64,${Buffer.from(celebrationSvg).toString("base64")}`;
const html = await readFile("index.html", "utf8");

await writeFile(
  "dist/index.html",
  html.replaceAll("__CELEBRATION_DATA_URI__", celebrationDataUri)
);
