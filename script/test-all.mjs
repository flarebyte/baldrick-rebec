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
// Helpers
// -----------------------------
function logStep(i, total, msg) {
  console.error(`[${i}/${total}] ${msg}`);
}

async function runRbc(...args) {
  // Mirrors alias: rbc='go run main.go'
  // Use template literal to avoid shell-escaping pitfalls; ZX handles args quoting.
  return await $`go run main.go ${args}`;
}

async function runRbcJSON(...args) {
  const p = await runRbc(...args);
  try {
    return JSON.parse(p.stdout || 'null');
  } catch (err) {
    console.error('Failed to parse JSON from:', args.join(' '));
    console.error(p.stdout);
    throw err;
  }
}

function idFrom(obj) {
  if (!obj || typeof obj !== 'object') return '';
  return obj.id || obj.ID || '';
}

async function createScript(role, title, description, body) {
  // Pipe body into the command (match legacy behavior)
  const cmd = `printf %s ${JSON.stringify(body)} | go run main.go admin script set --role ${role} --title ${JSON.stringify(title)} --description ${JSON.stringify(description)}`;
  const out = await $`${cmd}`;
  return JSON.parse(out.stdout).id;
}

// Note: sleep helper removed until needed; ZX provides sleep() globally.

// -----------------------------
// Flow
// -----------------------------
const TOTAL = 14;
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

  // 3) Workflows
  step++;
  logStep(step, TOTAL, 'Creating workflows');
  await runRbc(
    'admin',
    'workflow',
    'set',
    '--name',
    'ci-test',
    '--title',
    'Continuous Integration: Test Suite',
    '--description',
    'Runs unit and integration tests.',
    '--notes',
    'CI test workflow',
    '--role',
    TEST_ROLE_USER,
  );
  await runRbc(
    'admin',
    'workflow',
    'set',
    '--name',
    'ci-lint',
    '--title',
    'Continuous Integration: Lint & Format',
    '--description',
    'Lints and vets the codebase.',
    '--notes',
    'CI lint workflow',
    '--role',
    TEST_ROLE_USER,
  );

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
      '--run-script',
      sidUnit,
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
    '--run-script',
    sidInteg,
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
    '--run-script',
    sidLint,
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
    '--run-script',
    sidUnit,
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
    '--run-script',
    sidInteg,
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

  // 11) Messages & Queue
  step++;
  logStep(step, TOTAL, 'Creating messages and queue');
  await $`echo "Hello from user12" | go run main.go admin message set --experiment eid1 --title Greeting --tags hello --role ${TEST_ROLE_USER}`;
  await $`echo "Build started" | go run main.go admin message set --experiment eid1 --title BuildStart --tags build --role ${TEST_ROLE_USER}`;
  await $`echo "Onboarding checklist updated" | go run main.go admin message set --experiment eid2 --title DocsUpdate --tags docs,update --role ${TEST_ROLE_USER}`;

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
