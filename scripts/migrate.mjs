import { spawnSync } from "node:child_process";
import { readFileSync, existsSync } from "node:fs";
import path from "node:path";

const envPath = path.resolve(".env");
if (existsSync(envPath)) {
  for (const line of readFileSync(envPath, "utf8").split(/\r?\n/)) {
    const m = line.match(/^\s*([A-Z_][A-Z0-9_]*)\s*=\s*(.*?)\s*$/i);
    if (m && !process.env[m[1]]) {
      process.env[m[1]] = m[2].replace(/^["']|["']$/g, "");
    }
  }
}

const action = process.argv[2] ?? "up";
if (!["up", "down", "status", "version"].includes(action)) {
  console.error(`Unknown migration action: ${action}`);
  process.exit(1);
}

// Migrations run as the schema owner, not as app_rw.
const url = process.env.DATABASE_URL_ADMIN || process.env.DATABASE_URL;
if (!url) {
  console.error("DATABASE_URL_ADMIN (or DATABASE_URL) missing from .env");
  process.exit(1);
}

const dir = path.resolve("apps/api/db/migrations");
const r = spawnSync(
  "goose",
  ["-dir", dir, "postgres", url, action],
  { stdio: "inherit", shell: process.platform === "win32" }
);
process.exit(r.status ?? 1);