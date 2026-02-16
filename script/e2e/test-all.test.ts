import { test } from 'bun:test';
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

let testChain = Promise.resolve();

function serialTest(name: string, fn: () => Promise<void>) {
  test(
    name,
    async () => {
      const previous = testChain;
      let release: (() => void) | undefined;
      testChain = new Promise<void>((resolve) => {
        release = resolve;
      });

      await previous;
      try {
        await fn();
      } finally {
        release?.();
      }
    },
    TEST_TIMEOUT_MS,
  );
}

serialTest('e2e bootstrap', async () => {
  const ctx = createContext({ skipSnapshot: true });
  await runBootstrap(ctx);
});

serialTest('e2e project', async () => {
  const ctx = createContext({ skipSnapshot: true });
  await runUntilProject(ctx);
});

serialTest('e2e blackboard', async () => {
  const ctx = createContext({ skipSnapshot: true });
  await runUntilBlackboard(ctx);
});

serialTest('e2e collab', async () => {
  const ctx = createContext({ skipSnapshot: true });
  await runUntilCollab(ctx);
});

serialTest('e2e listing-sync-import', async () => {
  const ctx = createContext({ skipSnapshot: true });
  await runUntilListing(ctx);
});

serialTest('e2e full pipeline', async () => {
  const includeSnapshot = process.env.E2E_INCLUDE_SNAPSHOT === '1';
  const ctx = createContext({ skipSnapshot: !includeSnapshot });
  await runAllPhases(ctx);
});
