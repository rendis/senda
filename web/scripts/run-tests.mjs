#!/usr/bin/env node

import { spawnSync } from "node:child_process";
import { readdir } from "node:fs/promises";
import { dirname, join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const scriptDir = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(scriptDir, "..", "..");
const webRoot = join(repoRoot, "web");
const ignoredDirectories = new Set(["node_modules", ".next"]);

async function collectTestFiles(directory) {
  const entries = await readdir(directory, { withFileTypes: true });
  const files = [];

  for (const entry of entries) {
    const fullPath = join(directory, entry.name);

    if (entry.isDirectory()) {
      if (ignoredDirectories.has(entry.name)) {
        continue;
      }

      files.push(...(await collectTestFiles(fullPath)));
      continue;
    }

    if (!entry.isFile()) {
      continue;
    }

    if (entry.name.endsWith(".test.ts") || entry.name.endsWith(".test.mjs")) {
      files.push(relative(repoRoot, fullPath));
    }
  }

  return files;
}

const testFiles = (await collectTestFiles(webRoot)).sort((left, right) =>
  left.localeCompare(right),
);

if (testFiles.length === 0) {
  console.error("No frontend test files were found under web/");
  process.exit(1);
}

const result = spawnSync(process.execPath, ["--test", ...testFiles], {
  cwd: repoRoot,
  stdio: "inherit",
});

if (result.error) {
  console.error(result.error);
  process.exit(1);
}

process.exit(result.status ?? (result.signal ? 1 : 0));
