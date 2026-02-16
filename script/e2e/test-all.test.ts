import { describe, expect, test } from 'bun:test';
import {
  createContext,
  runAllPhases,
  runUntilBlackboard,
  runUntilCollab,
  runUntilListing,
  runUntilProject,
} from './context';
import { runBootstrap } from './bootstrap';

const TEST_TIMEOUT_MS = 30 * 60 * 1000;
const TEST_CONTEXT_OPTS = { skipSnapshot: true, showSteps: false } as const;

let serialChain = Promise.resolve();

async function runSerial(fn: () => Promise<void>) {
  const previous = serialChain;
  let release: (() => void) | undefined;
  serialChain = new Promise<void>((resolve) => {
    release = resolve;
  });
  await previous;
  try {
    await fn();
  } finally {
    release?.();
  }
}

describe('E2E Integration', () => {
  test(
    'bootstrap phase',
    async () => {
      await runSerial(async () => {
        const ctx = createContext(TEST_CONTEXT_OPTS);
        await runBootstrap(ctx);
        expect(ctx.state.sidUnit).toBeTruthy();
        expect(ctx.state.tUnit).toBeTruthy();
      });
    },
    TEST_TIMEOUT_MS,
  );

  test(
    'project phase',
    async () => {
      await runSerial(async () => {
        const ctx = createContext(TEST_CONTEXT_OPTS);
        await runUntilProject(ctx);
        expect(ctx.step).toBeGreaterThan(0);
        expect(await Bun.file('main.project.yaml').exists()).toBe(true);
      });
    },
    TEST_TIMEOUT_MS,
  );

  test(
    'blackboard phase',
    async () => {
      await runSerial(async () => {
        const ctx = createContext(TEST_CONTEXT_OPTS);
        await runUntilBlackboard(ctx);
        expect(ctx.state.bb1).toBeTruthy();
        expect(ctx.state.st1).toBeTruthy();
      });
    },
    TEST_TIMEOUT_MS,
  );

  test(
    'collaboration phase',
    async () => {
      await runSerial(async () => {
        const ctx = createContext(TEST_CONTEXT_OPTS);
        await runUntilCollab(ctx);
        expect(ctx.state.convID).toBeTruthy();
        expect(ctx.state.expID).toBeTruthy();
      });
    },
    TEST_TIMEOUT_MS,
  );

  test(
    'listing, sync and import phase',
    async () => {
      await runSerial(async () => {
        const ctx = createContext(TEST_CONTEXT_OPTS);
        await runUntilListing(ctx);
        expect(await Bun.file('temp/blackboard-test/blackboard.yaml').exists()).toBe(true);
      });
    },
    TEST_TIMEOUT_MS,
  );

  test(
    'full pipeline',
    async () => {
      await runSerial(async () => {
        const includeSnapshot = process.env.E2E_INCLUDE_SNAPSHOT === '1';
        const ctx = createContext({ skipSnapshot: !includeSnapshot, showSteps: false });
        await runAllPhases(ctx);
        expect(ctx.step).toBeGreaterThan(0);
      });
    },
    TEST_TIMEOUT_MS,
  );
});
