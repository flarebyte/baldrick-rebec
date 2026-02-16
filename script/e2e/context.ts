import { logStep } from './cli-helper';
import { runBlackboardStickie } from './blackboard-stickie';
import { runBootstrap } from './bootstrap';
import { runCollab } from './collab';
import { runListingSyncImport } from './listing-sync-import';
import { runProjectToolPrompt } from './project-tool-prompt';
import { runSnapshotVault } from './snapshot-vault';
import type { E2EContext } from './types';

const DEFAULT_TOTAL = 33;

export function createContext(opts: {
  testRoleUser?: string;
  testRoleQa?: string;
  skipReset?: boolean;
  skipSnapshot?: boolean;
  total?: number;
  showSteps?: boolean;
} = {}): E2EContext {
  const showSteps = opts.showSteps ?? true;
  return {
    TEST_ROLE_USER: opts.testRoleUser ?? 'rbctest-user',
    TEST_ROLE_QA: opts.testRoleQa ?? 'rbctest-qa',
    SKIP_RESET: opts.skipReset ?? false,
    SKIP_SNAPSHOT: opts.skipSnapshot ?? false,
    total: opts.total ?? DEFAULT_TOTAL,
    step: 0,
    state: {},
    nextStep(msg: string) {
      this.step += 1;
      if (showSteps) {
        logStep(this.step, this.total, msg);
      }
    },
  };
}

export async function runAllPhases(ctx: E2EContext) {
  await runBootstrap(ctx);
  await runProjectToolPrompt(ctx);
  await runBlackboardStickie(ctx);
  await runCollab(ctx);
  await runListingSyncImport(ctx);
  await runSnapshotVault(ctx);
  ctx.nextStep('Done.');
}

export async function runUntilProject(ctx: E2EContext) {
  await runBootstrap(ctx);
  await runProjectToolPrompt(ctx);
}

export async function runUntilBlackboard(ctx: E2EContext) {
  await runUntilProject(ctx);
  await runBlackboardStickie(ctx);
}

export async function runUntilCollab(ctx: E2EContext) {
  await runUntilProject(ctx);
  await runCollab(ctx);
}

export async function runUntilListing(ctx: E2EContext) {
  await runUntilBlackboard(ctx);
  await runCollab(ctx);
  await runListingSyncImport(ctx);
}
