import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { createServer } from "node:http";

import { createRequestHandler, resolveTarget } from "./serve-html.mjs";

test("resolveTarget returns directory and direct URL for file", async () => {
  const root = await mkdtemp(path.join(tmpdir(), "serve-html-"));
  const filePath = path.join(root, "report.html");
  await writeFile(filePath, "<h1>report</h1>");

  try {
    const target = await resolveTarget(filePath, "127.0.0.1", 9000);
    assert.equal(target.directory, root);
    assert.equal(target.fileName, "report.html");
    assert.equal(target.url, "http://127.0.0.1:9000/report.html");
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});

test("request handler serves target file from root path", async () => {
  const root = await mkdtemp(path.join(tmpdir(), "serve-html-"));
  const filePath = path.join(root, "report.html");
  await writeFile(filePath, "<h1>report</h1>");

  const server = createServer(createRequestHandler(root, "report.html"));
  await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));

  try {
    const address = server.address();
    assert.ok(address && typeof address === "object");
    const response = await fetch(`http://127.0.0.1:${address.port}/`);
    assert.equal(response.status, 200);
    assert.equal(await response.text(), await readFile(filePath, "utf8"));
  } finally {
    await new Promise((resolve, reject) => server.close((error) => (error ? reject(error) : resolve())));
    await rm(root, { recursive: true, force: true });
  }
});
