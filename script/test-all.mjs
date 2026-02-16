#!/usr/bin/env node

import { spawnSync } from 'node:child_process';

const res = spawnSync('bun', ['run', 'script/e2e/test-all.ts', ...process.argv.slice(2)], {
  stdio: 'inherit',
});

process.exit(res.status ?? 1);
