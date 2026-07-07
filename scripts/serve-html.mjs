import { createServer } from "node:http";
import { access, readFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const defaultHost = "127.0.0.1";
const defaultPort = 8000;

export async function resolveTarget(filePath, host = defaultHost, port = defaultPort) {
  if (!filePath) {
    throw new Error("usage: node scripts/serve-html.mjs <html-file>");
  }

  const absolutePath = path.resolve(filePath);
  await access(absolutePath);

  return {
    absolutePath,
    directory: path.dirname(absolutePath),
    fileName: path.basename(absolutePath),
    host,
    port,
    url: `http://${host}:${port}/${encodeURIComponent(path.basename(absolutePath))}`,
  };
}

export function createRequestHandler(rootDirectory, defaultFileName) {
  return async (request, response) => {
    const requestPath = request.url === "/" ? `/${defaultFileName}` : request.url || `/${defaultFileName}`;
    const decodedPath = decodeURIComponent(requestPath.split("?")[0]);
    const relativePath = decodedPath.replace(/^\/+/, "");
    const filePath = path.resolve(rootDirectory, relativePath);

    if (!filePath.startsWith(path.resolve(rootDirectory) + path.sep) && filePath !== path.resolve(rootDirectory, defaultFileName)) {
      response.writeHead(403, { "Content-Type": "text/plain; charset=utf-8" });
      response.end("forbidden");
      return;
    }

    try {
      const content = await readFile(filePath);
      response.writeHead(200, {
        "Content-Type": contentTypeFor(filePath),
        "Cache-Control": "no-store",
      });
      response.end(content);
    } catch {
      response.writeHead(404, { "Content-Type": "text/plain; charset=utf-8" });
      response.end("not found");
    }
  };
}

function contentTypeFor(filePath) {
  if (filePath.endsWith(".html")) {
    return "text/html; charset=utf-8";
  }

  if (filePath.endsWith(".css")) {
    return "text/css; charset=utf-8";
  }

  if (filePath.endsWith(".js")) {
    return "text/javascript; charset=utf-8";
  }

  if (filePath.endsWith(".json")) {
    return "application/json; charset=utf-8";
  }

  if (filePath.endsWith(".svg")) {
    return "image/svg+xml";
  }

  return "text/plain; charset=utf-8";
}

async function main() {
  const port = Number(process.env.PORT || defaultPort);
  const target = await resolveTarget(process.argv[2], defaultHost, port);
  const server = createServer(createRequestHandler(target.directory, target.fileName));

  server.listen(target.port, target.host, () => {
    console.log(target.url);
  });
}

const entrypointPath = process.argv[1] ? path.resolve(process.argv[1]) : "";
if (entrypointPath === fileURLToPath(import.meta.url)) {
  main().catch((error) => {
    console.error(error.message);
    process.exit(1);
  });
}
