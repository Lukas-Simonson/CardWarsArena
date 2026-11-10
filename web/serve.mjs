// Tiny static file server for local development: node serve.mjs [port]
import { createServer } from "node:http";
import { readFile } from "node:fs/promises";
import { extname, join, normalize, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const root = resolve(fileURLToPath(new URL(".", import.meta.url)));
const port = Number(process.argv[2] ?? process.env.PORT ?? 5173);
const types = { ".html": "text/html; charset=utf-8", ".js": "text/javascript; charset=utf-8", ".mjs": "text/javascript; charset=utf-8", ".css": "text/css; charset=utf-8", ".svg": "image/svg+xml", ".png": "image/png", ".json": "application/json" };
const publicFiles = /^\/(index\.html|css\/|js\/|img\/|favicon\.svg)/;

createServer(async (req, res) => {
  let path = new URL(req.url, "http://x").pathname;
  if (path === "/") path = "/index.html";
  const file = normalize(join(root, path));
  if (!publicFiles.test(path) || !file.startsWith(root + "/")) {
    res.writeHead(404).end("not found");
    return;
  }
  try {
    const body = await readFile(file);
    res.writeHead(200, { "content-type": types[extname(file)] ?? "application/octet-stream", "cache-control": "no-store" }).end(body);
  } catch {
    res.writeHead(404).end("not found");
  }
}).listen(port, "127.0.0.1", () => console.log(`Card Wars Arena web client: http://localhost:${port}/`));
