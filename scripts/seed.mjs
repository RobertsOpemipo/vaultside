import { readFileSync, existsSync } from "node:fs";
import { spawnSync } from "node:child_process";
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

const files = [
  "scripts/seed.sql",
  "scripts/seed-content.sql",
  "scripts/seed-phase3.sql",
  "scripts/seed-phase4.sql",
  "scripts/seed-gamification.sql",
  "scripts/seed-quests.sql",
];

for (const f of files) {
  console.log(`--- ${f} ---`);
  const sql = readFileSync(path.resolve(f));
  const r = spawnSync("docker", [
    "compose", "exec", "-T", "postgres",
    "psql", "-U", "vaultside", "-d", "vaultside",
    "-v", "ON_ERROR_STOP=1",
    "-f", "/dev/stdin"
  ], {
    input: sql,
    stdio: ["pipe", "inherit", "inherit"],
    shell: process.platform === "win32",
  });
  if ((r.status ?? 1) !== 0) process.exit(r.status ?? 1);
}