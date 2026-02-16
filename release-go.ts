#!/usr/bin/env bun

// Release helper for Go binaries using Bun/TypeScript
// - Reads version from main.project.yaml (tags.version)
// - Builds multi-platform binaries via build-go.ts
// - Creates a GitHub release v<version> with generated artifacts

import { promises as fs } from 'node:fs';
import path from 'node:path';

async function readFileSafe(p: string): Promise<string> {
  try {
    return await fs.readFile(p, 'utf8');
  } catch {
    return '';
  }
}

async function readVersionFromProjectYAML(p = 'main.project.yaml'): Promise<string> {
  const raw = await readFileSafe(p);
  if (!raw) return '';
  const lines = raw.split(/\r?\n/);
  let inTags = false;
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    const trimmed = line.trim();
    if (!inTags) {
      if (/^tags\s*:\s*$/.test(trimmed)) inTags = true;
      continue;
    }
    if (/^\S/.test(line)) break; // next top-level key
    const m = line.match(/^\s*version\s*:\s*(.+)\s*$/);
    if (m) {
      let v = m[1].trim();
      if ((v.startsWith('"') && v.endsWith('"')) || (v.startsWith("'") && v.endsWith("'"))) v = v.slice(1, -1);
      return v;
    }
  }
  return '';
}

async function runChecked(cmd: string[], opts: { cwd?: string; env?: Record<string, string | undefined> } = {}) {
  const proc = Bun.spawn(cmd, { cwd: opts.cwd, env: opts.env, stdout: 'inherit', stderr: 'inherit' });
  const code = await proc.exited;
  if (code !== 0) throw new Error(`Command failed (${code}): ${cmd.join(' ')}`);
}

async function ensureGhAvailable() {
  const which = Bun.which('gh');
  if (!which) throw new Error('gh (GitHub CLI) is required');
}

async function listBuildArtifacts(dir = 'build'): Promise<string[]> {
  try {
    const items = await fs.readdir(dir);
    return items.map((f) => path.join(dir, f));
  } catch {
    return [];
  }
}

async function main() {
  const args = process.argv.slice(2);
  const dryRun = args.includes('--dry-run');

  if (!dryRun) {
    await ensureGhAvailable();
  }
  const version = (await readVersionFromProjectYAML()).trim();
  if (!version) throw new Error('version not found in main.project.yaml (tags.version)');

  if (dryRun) {
    console.log(`[dry-run] Would build binaries via: bun run build-go.ts`);
    console.log(`[dry-run] Would create GitHub release v${version} from ./build/*`);
    return;
  }

  // fresh build
  try {
    await fs.rm('build', { recursive: true });
  } catch {}
  await runChecked(['bun', 'run', 'build-go.ts']);

  const artifacts = await listBuildArtifacts('build');
  if (!artifacts.length) throw new Error('no build artifacts found');

  console.log(`Creating GitHub release v${version}`);
  await runChecked(['gh', 'release', 'create', `v${version}`, ...artifacts, '--generate-notes']);
}

main().catch((err) => {
  console.error(err?.message || err);
  process.exitCode = 1;
});
