#!/usr/bin/env bun

import { logStep } from './cli-helper';
import type { E2EContext } from './types';
import { runBlackboardStickie } from './blackboard-stickie';
import { runBootstrap } from './bootstrap';
import { runCollab } from './collab';
import { runListingSyncImport } from './listing-sync-import';
import { runProjectToolPrompt } from './project-tool-prompt';
import { runSnapshotVault } from './snapshot-vault';

const TEST_ROLE_USER = 'rbctest-user';
const TEST_ROLE_QA = 'rbctest-qa';

const args = new Set(Bun.argv.slice(2));
const SKIP_RESET = args.has('--skip-reset');
const SKIP_SNAPSHOT = args.has('--skip-snapshot');

const TOTAL = 33;

const ctx: E2EContext = {
  TEST_ROLE_USER,
  TEST_ROLE_QA,
  SKIP_RESET,
  SKIP_SNAPSHOT,
  total: TOTAL,
  step: 0,
  state: {},
  nextStep(msg: string) {
    this.step += 1;
    logStep(this.step, this.total, msg);
  },
};

async function main() {
  await runBootstrap(ctx);
  await runProjectToolPrompt(ctx);
  await runBlackboardStickie(ctx);
  await runCollab(ctx);
  await runListingSyncImport(ctx);
  await runSnapshotVault(ctx);

  ctx.nextStep('Done.');
}

main().catch((err) => {
  const e = err as any;
  const msg = e && (e.stack || e.stderr || e.message || String(e));
  const extra =
    e &&
    (e.stdout ? `\nstdout:\n${e.stdout}` : '') +
      (e.stderr ? `\nstderr:\n${e.stderr}` : '');
  console.error('Test-all failed:', msg, extra);
  process.exit(1);
});
