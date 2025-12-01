#!/usr/bin/env zx

/**
 * Purpose: End-to-end local sanity using Google ZX
 * Notes: Resets DB, scaffolds roles/DB/privs/schema, creates sample data, lists entities, runs snapshot smoke tests.
 * Agent: Keep ZX idioms (await $``; cd(); argv), avoid importing core modules (fs/path/fetch), use async-only.
 */

// -----------------------------
// Configuration
// -----------------------------
const TEST_ROLE_USER = 'rbctest-user';
const TEST_ROLE_QA = 'rbctest-qa';

// Allow partial runs via flags, e.g. --skip-reset, --skip-snapshot
const SKIP_RESET = argv['skip-reset'] ?? false;
const SKIP_SNAPSHOT = argv['skip-snapshot'] ?? false;

// -----------------------------
// Helpers (shared)
// -----------------------------
import {
  runRbc,
  runRbcJSON,
  idFrom,
  logStep,
  assert,
  createScript,
  runSetRole,
  runSetWorkflow,
  scriptListJSON,
  scriptFind,
  stickieListJSON,
  stickieFind,
  stickieSet,
} from './cli-helper.mjs';

// Note: sleep helper removed until needed; ZX provides sleep() globally.

// -----------------------------
// Flow
// -----------------------------
const TOTAL = 15;
let step = 0;

try {
  // 1) Reset
  step++;
  if (!SKIP_RESET) {
    logStep(step, TOTAL, 'Resetting database (destructive)');
    await runRbc('admin', 'db', 'reset', '--force', '--drop-app-role=false');
  } else {
    logStep(step, TOTAL, 'Skipping reset (--skip-reset)');
  }

  // 2) Scaffold
  step++;
  logStep(
    step,
    TOTAL,
    'Scaffolding roles, database, privileges, schema, content index, backup grants',
  );
  await runRbc('admin', 'db', 'scaffold', '--all', '--yes');

  // 2.5) Ensure roles exist for package FKs
  step++;
  logStep(step, TOTAL, 'Ensuring roles for test users (FK for packages)');
  await runSetRole({ name: TEST_ROLE_USER, title: 'RBCTest User' });
  await runSetRole({ name: TEST_ROLE_QA, title: 'RBCTest QA' });

  // 3) Workflows
  step++;
  logStep(step, TOTAL, 'Creating workflows');
  await runSetWorkflow({ name: 'ci-test', title: 'Continuous Integration: Test Suite', description: 'Runs unit and integration tests.', notes: 'CI test workflow', role: TEST_ROLE_USER });
  await runSetWorkflow({ name: 'ci-lint', title: 'Continuous Integration: Lint & Format', description: 'Lints and vets the codebase.', notes: 'CI lint workflow', role: TEST_ROLE_USER });

  // 4) Scripts
  step++;
  logStep(step, TOTAL, 'Creating scripts and capturing ids');
  const sidUnit = await createScript(
    TEST_ROLE_USER,
    'Unit: go test',
    'Run unit tests',
    '#!/usr/bin/env bash\nset -euo pipefail\ngo test ./...\n',
  );
  const sidInteg = await createScript(
    TEST_ROLE_USER,
    'Integration: compose+test',
    'Run integration tests',
    '#!/usr/bin/env bash\nset -euo pipefail\ndocker compose up -d && go test -tags=integration ./...\n',
  );
  const sidLint = await createScript(
    TEST_ROLE_USER,
    'Lint & Vet',
    'Runs vet and lints',
    '#!/usr/bin/env bash\nset -euo pipefail\ngo vet ./... && echo linting...\n',
  );

  // Regression: script list includes complex name; script find resolves by complex name
  {
    const listJSON = await runRbcJSON('admin', 'script', 'list', '--role', TEST_ROLE_USER, '--output', 'json');
    const byId = (id) => (listJSON || []).find((x) => x && (x.id === id || x.ID === id));
    const ju = byId(sidUnit);
    assert(ju, 'script list json missing unit script');
    assert(ju.name === 'Unit: go test', 'unit script name mismatch in list json');
    assert((ju.variant ?? '') === '', 'unit script variant should be empty in list json');

    const ji = byId(sidInteg);
    assert(ji && ji.name === 'Integration: compose+test', 'integration script not present or name mismatch');

    const jl = byId(sidLint);
    assert(jl && jl.name === 'Lint & Vet', 'lint script not present or name mismatch');

    const foundUnit = await runRbcJSON('admin', 'script', 'find', '--name', 'Unit: go test', '--variant', '', '--role', TEST_ROLE_USER);
    assert(foundUnit && foundUnit.id === sidUnit, 'script find did not resolve unit by complex name');
  }

  // 5) Tasks
  step++;
  logStep(step, TOTAL, 'Creating tasks');
  const tUnit = idFrom(
    await runRbcJSON(
      'admin',
      'task',
      'set',
      '--workflow',
      'ci-test',
      '--command',
      'unit',
      '--variant',
      'go',
      '--role',
      TEST_ROLE_USER,
      '--title',
      'Run Unit Tests',
      '--description',
      'Executes unit tests.',
      '--shell',
      'bash',
      '--timeout',
      '10 minutes',
      '--tags',
      'unit,fast',
      '--level',
      'h2',
    ),
  );
  await runRbcJSON(
    'admin',
    'task',
    'set',
    '--workflow',
    'ci-test',
    '--command',
    'integration',
    '--variant',
    '',
    '--role',
    TEST_ROLE_USER,
    '--title',
    'Run Integration Tests',
    '--description',
    'Runs integration tests.',
    '--shell',
    'bash',
    '--timeout',
    '30 minutes',
    '--tags',
    'integration,slow',
    '--level',
    'h2',
  );
  await runRbcJSON(
    'admin',
    'task',
    'set',
    '--workflow',
    'ci-lint',
    '--command',
    'lint',
    '--variant',
    'go',
    '--role',
    TEST_ROLE_USER,
    '--title',
    'Lint & Vet',
    '--description',
    'Runs vet and lints.',
    '--shell',
    'bash',
    '--timeout',
    '5 minutes',
    '--tags',
    'lint,style',
    '--level',
    'h2',
  );

  // Replacements
  await runRbc(
    'admin',
    'task',
    'set',
    '--workflow',
    'ci-test',
    '--command',
    'unit',
    '--variant',
    'go-patch1',
    '--role',
    TEST_ROLE_USER,
    '--title',
    'Run Unit Tests (Quick)',
    '--description',
    'Patch: run quick subset',
    '--shell',
    'bash',
    '--replaces',
    tUnit,
    '--replace-level',
    'patch',
    '--replace-comment',
    'Flaky test workaround',
  );
  await runRbc(
    'admin',
    'task',
    'set',
    '--workflow',
    'ci-test',
    '--command',
    'unit',
    '--variant',
    'go-minor1',
    '--role',
    TEST_ROLE_USER,
    '--title',
    'Run Unit Tests (Race)',
    '--description',
    'Minor: enable race detector',
    '--shell',
    'bash',
    '--replaces',
    tUnit,
    '--replace-level',
    'minor',
    '--replace-comment',
    'Add -race',
  );

  // 6) Tags & Topics
  step++;
  logStep(step, TOTAL, 'Creating tags and topics');
  await runRbc(
    'admin',
    'tag',
    'set',
    '--name',
    'priority-high',
    '--title',
    'High Priority',
    '--role',
    TEST_ROLE_USER,
  );
  await runRbc(
    'admin',
    'topic',
    'set',
    '--name',
    'onboarding',
    '--role',
    TEST_ROLE_USER,
    '--title',
    'Onboarding',
    '--description',
    'New hires onboarding',
    '--tags',
    'area=people,priority=med',
  );
  await runRbc(
    'admin',
    'topic',
    'set',
    '--name',
    'devops',
    '--role',
    TEST_ROLE_USER,
    '--title',
    'DevOps',
    '--description',
    'Build, deploy, CI/CD',
    '--tags',
    'area=platform,priority=high',
  );

  // 7) Projects
  step++;
  logStep(step, TOTAL, 'Creating projects');
  await runRbc(
    'admin',
    'project',
    'set',
    '--name',
    'acme/build-system',
    '--role',
    TEST_ROLE_USER,
    '--description',
    'Build system and CI pipeline',
    '--tags',
    'status=active,type=ci',
  );
  await runRbc(
    'admin',
    'project',
    'set',
    '--name',
    'acme/product',
    '--role',
    TEST_ROLE_USER,
    '--description',
    'Main product',
    '--tags',
    'status=active,type=app',
  );

  // 8) Stores & Blackboards
  step++;
  logStep(step, TOTAL, 'Creating stores and blackboards');
  await runRbc(
    'admin',
    'store',
    'set',
    '--name',
    'ideas-acme-build',
    '--role',
    TEST_ROLE_USER,
    '--title',
    'Ideas for acme/build-system',
    '--description',
    'Idea backlog',
    '--type',
    'journal',
    '--scope',
    'project',
    '--lifecycle',
    'monthly',
    '--tags',
    'topic=ideas,project=acme/build-system',
  );
  await runRbc(
    'admin',
    'store',
    'set',
    '--name',
    'blackboard-global',
    '--role',
    TEST_ROLE_USER,
    '--title',
    'Shared Blackboard',
    '--description',
    'Scratch space for team',
    '--type',
    'blackboard',
    '--scope',
    'shared',
    '--lifecycle',
    'weekly',
    '--tags',
    'visibility=team',
  );

  const s1 = idFrom(
    await runRbcJSON(
      'admin',
      'store',
      'get',
      '--name',
      'ideas-acme-build',
      '--role',
      TEST_ROLE_USER,
    ),
  );
  const s2 = idFrom(
    await runRbcJSON(
      'admin',
      'store',
      'get',
      '--name',
      'blackboard-global',
      '--role',
      TEST_ROLE_USER,
    ),
  );

  const bb1 = idFrom(
    await runRbcJSON(
      'admin',
      'blackboard',
      'set',
      '--role',
      TEST_ROLE_USER,
      '--store-id',
      s1,
      '--project',
      'acme/build-system',
      '--background',
      'Ideas board for build system',
      '--guidelines',
      'Keep concise; tag items with priority',
    ),
  );
  const bb2 = idFrom(
    await runRbcJSON(
      'admin',
      'blackboard',
      'set',
      '--role',
      TEST_ROLE_USER,
      '--store-id',
      s2,
      '--background',
      'Team-wide blackboard',
      '--guidelines',
      'Wipe weekly on Mondays',
    ),
  );

  // 9) Stickies and relations
  step++;
  logStep(step, TOTAL, 'Creating stickies and relations');
  const st1 = idFrom(
    await runRbcJSON(
      'admin',
      'stickie',
      'set',
      '--blackboard',
      bb1,
      '--topic-name',
      'onboarding',
      '--topic-role',
      TEST_ROLE_USER,
      '--note',
      'Refresh onboarding guide for new hires',
      '--labels',
      'onboarding,docs,priority:med',
      '--priority',
      'should',
    ),
  );
  const st2 = idFrom(
    await runRbcJSON(
      'admin',
      'stickie',
      'set',
      '--blackboard',
      bb1,
      '--topic-name',
      'devops',
      '--topic-role',
      TEST_ROLE_USER,
      '--note',
      'Evaluate GitHub Actions caching for go build',
      '--labels',
      'idea,devops',
      '--priority',
      'could',
    ),
  );
  const st3 = idFrom(
    await runRbcJSON(
      'admin',
      'stickie',
      'set',
      '--blackboard',
      bb2,
      '--note',
      'Team retro every Friday',
      '--labels',
      'team,ritual',
      '--priority',
      'must',
    ),
  );

  await runRbc(
    'admin',
    'stickie-rel',
    'set',
    '--from',
    st1,
    '--to',
    st2,
    '--type',
    'uses',
    '--labels',
    'ref,dependency',
  );
  await runRbc(
    'admin',
    'stickie-rel',
    'set',
    '--from',
    st2,
    '--to',
    st3,
    '--type',
    'includes',
    '--labels',
    'backlog',
  );
  await runRbc(
    'admin',
    'stickie-rel',
    'set',
    '--from',
    st1,
    '--to',
    st3,
    '--type',
    'contrasts_with',
    '--labels',
    'tradeoff',
  );

  // 10) Workspaces, Packages
  step++;
  logStep(step, TOTAL, 'Creating workspaces and packages');
  await runRbc(
    'admin',
    'workspace',
    'set',
    '--role',
    TEST_ROLE_USER,
    '--project',
    'acme/build-system',
    '--description',
    'Local build-system workspace',
    '--tags',
    'status=active',
  );
  await runRbc(
    'admin',
    'workspace',
    'set',
    '--role',
    TEST_ROLE_USER,
    '--project',
    'acme/product',
    '--description',
    'Local product workspace',
    '--tags',
    'status=active',
  );

  await runRbc(
    'admin',
    'package',
    'set',
    '--role',
    TEST_ROLE_USER,
    '--variant',
    'unit/go',
  );
  await runRbc(
    'admin',
    'package',
    'set',
    '--role',
    TEST_ROLE_QA,
    '--variant',
    'integration',
  );
  await runRbc(
    'admin',
    'package',
    'set',
    '--role',
    TEST_ROLE_USER,
    '--variant',
    'lint/go',
  );

  // 11) Conversation, Experiment, Messages & Queue
  step++;
  logStep(step, TOTAL, 'Creating conversation, experiment, messages and queue');
  const convMeta = await runRbcJSON(
    'admin',
    'conversation',
    'set',
    '--title',
    'Test Conversation',
    '--role',
    TEST_ROLE_USER,
  );
  const convID = idFrom(convMeta);
  const expMeta = await runRbcJSON(
    'admin',
    'experiment',
    'create',
    '--conversation',
    convID,
  );
  const expID = idFrom(expMeta);
  await $`echo "Hello from user12" | go run main.go admin message set --experiment ${expID} --title Greeting --tags hello --role ${TEST_ROLE_USER}`;
  await $`echo "Build started" | go run main.go admin message set --experiment ${expID} --title BuildStart --tags build --role ${TEST_ROLE_USER}`;
  await $`echo "Onboarding checklist updated" | go run main.go admin message set --experiment ${expID} --title DocsUpdate --tags docs,update --role ${TEST_ROLE_USER}`;

  const q1 = idFrom(
    await runRbcJSON(
      'admin',
      'queue',
      'add',
      '--description',
      'Run quick unit subset',
      '--status',
      'Waiting',
      '--why',
      'waiting for CI window',
      '--tags',
      'kind=test,priority=low',
    ),
  );
  await runRbc(
    'admin',
    'queue',
    'add',
    '--description',
    'Run full integration suite',
    '--status',
    'Buildable',
    '--tags',
    'kind=test,priority=high',
  );
  await runRbc(
    'admin',
    'queue',
    'add',
    '--description',
    'Strict lint pass',
    '--status',
    'Blocked',
    '--why',
    'env not ready',
    '--tags',
    'kind=lint',
  );

  await runRbc('admin', 'queue', 'peek', '--limit', '2');
  await runRbc('admin', 'queue', 'size');
  await runRbc('admin', 'queue', 'take', '--id', q1);

  // 12) Listings & counts
  step++;
  logStep(step, TOTAL, 'Listing entities and counts (per-role and JSON)');
  await runRbc(
    'admin',
    'workflow',
    'list',
    '--role',
    TEST_ROLE_USER,
    '--limit',
    '50',
  );
  await runRbc(
    'admin',
    'task',
    'list',
    '--role',
    TEST_ROLE_USER,
    '--limit',
    '50',
  );
  await runRbc(
    'admin',
    'conversation',
    'list',
    '--role',
    TEST_ROLE_USER,
    '--limit',
    '50',
  );
  await runRbc('admin', 'experiment', 'list', '--limit', '50');
  await runRbc(
    'admin',
    'message',
    'list',
    '--role',
    TEST_ROLE_USER,
    '--limit',
    '50',
  );
  await runRbc(
    'admin',
    'project',
    'list',
    '--role',
    TEST_ROLE_USER,
    '--limit',
    '50',
  );
  await runRbc(
    'admin',
    'workspace',
    'list',
    '--role',
    TEST_ROLE_USER,
    '--limit',
    '50',
  );
  await runRbc(
    'admin',
    'script',
    'list',
    '--role',
    TEST_ROLE_USER,
    '--limit',
    '50',
  );
  await runRbc(
    'admin',
    'store',
    'list',
    '--role',
    TEST_ROLE_USER,
    '--limit',
    '50',
  );
  await runRbc(
    'admin',
    'topic',
    'list',
    '--role',
    TEST_ROLE_USER,
    '--limit',
    '50',
  );
  await runRbc(
    'admin',
    'blackboard',
    'list',
    '--role',
    TEST_ROLE_USER,
    '--limit',
    '50',
  );
  await runRbc('admin', 'stickie', 'list', '--limit', '50');
  await runRbc(
    'admin',
    'stickie',
    'list',
    '--blackboard',
    bb1,
    '--limit',
    '50',
  );
  // Regression: stickie list includes complex name; stickie find resolves by complex name and blackboard
  {
    const stList = await runRbcJSON('admin', 'stickie', 'list', '--blackboard', bb1, '--output', 'json');
    const byId = (id) => (stList || []).find((x) => x && (x.id === id || x.ID === id));
    const s1json = byId(st1);
    assert(s1json && s1json.name === 'Onboarding Refresh', 'stickie list json missing or wrong name for st1');
    const s2json = byId(st2);
    assert(s2json && s2json.name === 'DevOps Caching', 'stickie list json missing or wrong name for st2');
    const f1 = await runRbcJSON('admin', 'stickie', 'find', '--name', 'Onboarding Refresh', '--variant', '', '--blackboard', bb1);
    assert(f1 && f1.id === st1, 'stickie find did not resolve st1 by complex name within board');
  }
  await runRbc(
    'admin',
    'stickie',
    'list',
    '--topic-name',
    'devops',
    '--topic-role',
    TEST_ROLE_USER,
    '--limit',
    '50',
  );
  await runRbc(
    'admin',
    'stickie-rel',
    'list',
    '--id',
    st1,
    '--direction',
    'out',
  );
  await runRbc(
    'admin',
    'stickie-rel',
    'get',
    '--from',
    st1,
    '--to',
    st2,
    '--type',
    'uses',
    '--ignore-missing',
  );
  await runRbc(
    'admin',
    'tag',
    'list',
    '--role',
    TEST_ROLE_USER,
    '--limit',
    '50',
  );
  await runRbc('admin', 'db', 'count', '--per-role');
  await runRbc('admin', 'db', 'count', '--json');

  // 13) Snapshot (backup/list/show/restore-dry/delete)
  step++;
  if (!SKIP_SNAPSHOT) {
    logStep(
      step,
      TOTAL,
      'Snapshot backup smoke test (backup/list/show/restore dry-run/delete)',
    );
    const bkp = await runRbcJSON(
      'admin',
      'snapshot',
      'backup',
      '--description',
      'rbctest snapshot',
      '--who',
      TEST_ROLE_USER,
      '--json',
    );
    const bkpID = idFrom(bkp);
    await runRbc('admin', 'snapshot', 'list', '--limit', '5');
    await runRbc('admin', 'snapshot', 'show', bkpID);
    await runRbc(
      'admin',
      'snapshot',
      'restore',
      bkpID,
      '--mode',
      'append',
      '--dry-run',
    );
    await runRbc('admin', 'snapshot', 'delete', bkpID, '--force');
  } else {
    logStep(step, TOTAL, 'Skipping snapshot (--skip-snapshot)');
  }

  // 14) Done
  step++;
  logStep(step, TOTAL, 'Done.');
} catch (err) {
  console.error('Test-all failed:', err?.stderr || err?.message || err);
  process.exit(1);
}
