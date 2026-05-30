import * as esbuild from "esbuild";

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
