#!/usr/bin/env bun

import { createContext, runAllPhases } from './context';

const args = new Set(Bun.argv.slice(2));
const SKIP_RESET = args.has('--skip-reset');
const SKIP_SNAPSHOT = args.has('--skip-snapshot');

const ctx = createContext({
  skipReset: SKIP_RESET,
  skipSnapshot: SKIP_SNAPSHOT,
});

runAllPhases(ctx).catch((err) => {
  const e = err as StdioLikeError;
  const msg = e && (e.stack || e.stderr || e.message || String(e));
  const extra =
    e &&
    (e.stdout ? `\nstdout:\n${e.stdout}` : '') +
      (e.stderr ? `\nstderr:\n${e.stderr}` : '');
  console.error('Test-all failed:', msg, extra);
  process.exit(1);
});
type StdioLikeError = {
  stack?: string;
  stderr?: string;
  stdout?: string;
  message?: string;
};
