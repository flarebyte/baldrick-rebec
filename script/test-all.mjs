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
  assert,
  blackboardListJSON,
  blackboardSet,
  conversationListJSON,
  conversationSet,
  createScript,
  dbCountJSON,
  dbCountPerRole,
  dbReset,
  dbScaffoldAll,
  experimentCreate,
  experimentList,
  idFrom,
  listWithRole,
  logStep,
  messageListJSON,
  messageSet,
  packageSet,
  projectListJSON,
  projectSet,
  queueAdd,
  queuePeek,
  queueSize,
  queueTake,
  roleGetJSON,
  roleListJSON,
  runSetRole,
  runSetTask,
  runSetWorkflow,
  scriptFind,
  scriptListJSON,
  snapshotBackupJSON,
  snapshotDelete,
  snapshotList,
  snapshotPrunePreviewJSON,
  snapshotRestoreDry,
  snapshotShow,
  snapshotVerifyJSON,
  stickieFind,
  stickieList,
  stickieListByBlackboard,
  stickieListByTopic,
  stickieListJSON,
  stickieRelGet,
  stickieRelList,
  stickieRelSet,
  stickieSet,
  storeGet,
  storeListJSON,
  storeSet,
  tagSet,
  taskListJSON,
  taskSetReplacement,
  topicListJSON,
  topicSet,
  workflowListJSON,
  workspaceSet,
} from './cli-helper.mjs';
import {
  validateBlackboardListContract,
  validateConversationListContract,
  validateMessageListContract,
  validateProjectListContract,
  validateRoleContract,
  validateRoleListContract,
  validateScriptListContract,
  validateStickieListContract,
  validateStoreListContract,
  validateTaskListContract,
  validateTopicListContract,
  validateWorkflowListContract,
} from './contract-helper.mjs';

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
    await dbReset({ dropAppRole: false });
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
  await dbScaffoldAll();

  // 2.5) Ensure roles exist for package FKs
  step++;
  logStep(step, TOTAL, 'Ensuring roles for test users (FK for packages)');
  await runSetRole({ name: TEST_ROLE_USER, title: 'RBCTest User' });
  await runSetRole({ name: TEST_ROLE_QA, title: 'RBCTest QA' });
  // Contract checks: roles
  {
    const rUser = await roleGetJSON({ name: TEST_ROLE_USER });
    validateRoleContract(rUser, { allowEmptyTitle: false });
    const rQA = await roleGetJSON({ name: TEST_ROLE_QA });
    validateRoleContract(rQA, { allowEmptyTitle: false });
    const rList = await roleListJSON({ limit: 200 });
    const parsed = validateRoleListContract(rList, { allowEmptyTitle: false });
    assert(
      parsed.length >= 2,
      'expected at least the 2 test roles in role list',
    );
  }

  // 3) Workflows
  step++;
  logStep(step, TOTAL, 'Creating workflows');
  await runSetWorkflow({
    name: 'ci-test',
    title: 'Continuous Integration: Test Suite',
    description: 'Runs unit and integration tests.',
    notes: 'CI test workflow',
    role: TEST_ROLE_USER,
  });
  await runSetWorkflow({
    name: 'ci-lint',
    title: 'Continuous Integration: Lint & Format',
    description: 'Lints and vets the codebase.',
    notes: 'CI lint workflow',
    role: TEST_ROLE_USER,
  });
  {
    const wfList = await workflowListJSON({ role: TEST_ROLE_USER, limit: 50 });
    validateWorkflowListContract(wfList, { allowEmptyTitle: false });
  }

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
    const listJSON = await scriptListJSON({ role: TEST_ROLE_USER });
    validateScriptListContract(listJSON, { allowEmptyTitle: false });
    const byId = (id) =>
      (listJSON || []).find((x) => x && (x.id === id || x.ID === id));
    const ju = byId(sidUnit);
    assert(ju, 'script list json missing unit script');
    assert(
      ju.name === 'Unit: go test',
      'unit script name mismatch in list json',
    );
    assert(
      (ju.variant ?? '') === '',
      'unit script variant should be empty in list json',
    );

    const ji = byId(sidInteg);
    assert(
      ji && ji.name === 'Integration: compose+test',
      'integration script not present or name mismatch',
    );

    const jl = byId(sidLint);
    assert(
      jl && jl.name === 'Lint & Vet',
      'lint script not present or name mismatch',
    );

    const foundUnit = await scriptFind({
      name: 'Unit: go test',
      variant: '',
      role: TEST_ROLE_USER,
    });
    assert(
      foundUnit && foundUnit.id === sidUnit,
      'script find did not resolve unit by complex name',
    );
  }

  // 5) Tasks
  step++;
  logStep(step, TOTAL, 'Creating tasks');
  const tUnit = idFrom(
    await runSetTask({
      workflow: 'ci-test',
      command: 'unit',
      variant: 'go',
      role: TEST_ROLE_USER,
      title: 'Run Unit Tests',
      description: 'Executes unit tests.',
      shell: 'bash',
      timeout: '10 minutes',
      tags: 'unit,fast',
      level: 'h2',
    }),
  );
  {
    const tList = await taskListJSON({ role: TEST_ROLE_USER, limit: 50 });
    validateTaskListContract(tList, { allowEmptyTitle: true });
  }
  await runSetTask({
    workflow: 'ci-test',
    command: 'integration',
    variant: '',
    role: TEST_ROLE_USER,
    title: 'Run Integration Tests',
    description: 'Runs integration tests.',
    shell: 'bash',
    timeout: '30 minutes',
    tags: 'integration,slow',
    level: 'h2',
  });
  await runSetTask({
    workflow: 'ci-lint',
    command: 'lint',
    variant: 'go',
    role: TEST_ROLE_USER,
    title: 'Lint & Vet',
    description: 'Runs vet and lints.',
    shell: 'bash',
    timeout: '5 minutes',
    tags: 'lint,style',
    level: 'h2',
  });

  // Replacements
  await taskSetReplacement({
    workflow: 'ci-test',
    command: 'unit',
    variant: 'go-patch1',
    role: TEST_ROLE_USER,
    title: 'Run Unit Tests (Quick)',
    description: 'Patch: run quick subset',
    shell: 'bash',
    replaces: tUnit,
    replaceLevel: 'patch',
    replaceComment: 'Flaky test workaround',
  });
  await taskSetReplacement({
    workflow: 'ci-test',
    command: 'unit',
    variant: 'go-minor1',
    role: TEST_ROLE_USER,
    title: 'Run Unit Tests (Race)',
    description: 'Minor: enable race detector',
    shell: 'bash',
    replaces: tUnit,
    replaceLevel: 'minor',
    replaceComment: 'Add -race',
  });

  // 6) Tags & Topics
  step++;
  logStep(step, TOTAL, 'Creating tags and topics');
  await tagSet({
    name: 'priority-high',
    title: 'High Priority',
    role: TEST_ROLE_USER,
  });
  await topicSet({
    name: 'onboarding',
    role: TEST_ROLE_USER,
    title: 'Onboarding',
    description: 'New hires onboarding',
    tags: 'area=people,priority=med',
  });
  await topicSet({
    name: 'devops',
    role: TEST_ROLE_USER,
    title: 'DevOps',
    description: 'Build, deploy, CI/CD',
    tags: 'area=platform,priority=high',
  });
  {
    const topics = await topicListJSON({ role: TEST_ROLE_USER, limit: 50 });
    validateTopicListContract(topics);
  }

  // 7) Projects
  step++;
  logStep(step, TOTAL, 'Creating projects');
  await projectSet({
    name: 'acme/build-system',
    role: TEST_ROLE_USER,
    description: 'Build system and CI pipeline',
    tags: 'status=active,type=ci',
  });
  await projectSet({
    name: 'acme/product',
    role: TEST_ROLE_USER,
    description: 'Main product',
    tags: 'status=active,type=app',
  });
  {
    const prj = await projectListJSON({ role: TEST_ROLE_USER, limit: 50 });
    validateProjectListContract(prj);
  }

  // 8) Stores & Blackboards
  step++;
  logStep(step, TOTAL, 'Creating stores and blackboards');
  await storeSet({
    name: 'ideas-acme-build',
    role: TEST_ROLE_USER,
    title: 'Ideas for acme/build-system',
    description: 'Idea backlog',
    type: 'journal',
    scope: 'project',
    lifecycle: 'monthly',
    tags: 'topic=ideas,project=acme/build-system',
  });
  await storeSet({
    name: 'blackboard-global',
    role: TEST_ROLE_USER,
    title: 'Shared Blackboard',
    description: 'Scratch space for team',
    type: 'blackboard',
    scope: 'shared',
    lifecycle: 'weekly',
    tags: 'visibility=team',
  });
  {
    const stores = await storeListJSON({ role: TEST_ROLE_USER, limit: 50 });
    validateStoreListContract(stores);
  }

  const s1 = idFrom(
    await storeGet({ name: 'ideas-acme-build', role: TEST_ROLE_USER }),
  );
  const s2 = idFrom(
    await storeGet({ name: 'blackboard-global', role: TEST_ROLE_USER }),
  );

  const bb1 = idFrom(
    await blackboardSet({
      role: TEST_ROLE_USER,
      storeId: s1,
      project: 'acme/build-system',
      background: 'Ideas board for build system',
      guidelines: 'Keep concise; tag items with priority',
    }),
  );
  const bb2 = idFrom(
    await blackboardSet({
      role: TEST_ROLE_USER,
      storeId: s2,
      background: 'Team-wide blackboard',
      guidelines: 'Wipe weekly on Mondays',
    }),
  );
  {
    const bbs = await blackboardListJSON({ role: TEST_ROLE_USER, limit: 50 });
    validateBlackboardListContract(bbs);
  }

  // 9) Stickies and relations
  step++;
  logStep(step, TOTAL, 'Creating stickies and relations');
  const st1 = idFrom(
    await stickieSet({
      blackboard: bb1,
      topicName: 'onboarding',
      topicRole: TEST_ROLE_USER,
      note: 'Refresh onboarding guide for new hires',
      labels: ['onboarding', 'docs', 'priority:med'],
      priority: 'should',
      name: 'Onboarding Refresh',
      variant: '',
    }),
  );
  const st2 = idFrom(
    await stickieSet({
      blackboard: bb1,
      topicName: 'devops',
      topicRole: TEST_ROLE_USER,
      note: 'Evaluate GitHub Actions caching for go build',
      labels: ['idea', 'devops'],
      priority: 'could',
      name: 'DevOps Caching',
      variant: '',
    }),
  );
  const st3 = idFrom(
    await stickieSet({
      blackboard: bb2,
      note: 'Team retro every Friday',
      labels: ['team', 'ritual'],
      priority: 'must',
      name: 'Team Retro',
      variant: '',
    }),
  );

  await stickieRelSet({
    from: st1,
    to: st2,
    type: 'uses',
    labels: 'ref,dependency',
  });
  await stickieRelSet({
    from: st2,
    to: st3,
    type: 'includes',
    labels: 'backlog',
  });
  await stickieRelSet({
    from: st1,
    to: st3,
    type: 'contrasts_with',
    labels: 'tradeoff',
  });

  // 10) Workspaces, Packages
  step++;
  logStep(step, TOTAL, 'Creating workspaces and packages');
  await workspaceSet({
    role: TEST_ROLE_USER,
    project: 'acme/build-system',
    description: 'Local build-system workspace',
    tags: 'status=active',
  });
  await workspaceSet({
    role: TEST_ROLE_USER,
    project: 'acme/product',
    description: 'Local product workspace',
    tags: 'status=active',
  });

  await packageSet({ role: TEST_ROLE_USER, variant: 'unit/go' });
  await packageSet({ role: TEST_ROLE_QA, variant: 'integration' });
  await packageSet({ role: TEST_ROLE_USER, variant: 'lint/go' });

  // 11) Conversation, Experiment, Messages & Queue
  step++;
  logStep(step, TOTAL, 'Creating conversation, experiment, messages and queue');
  const convMeta = await conversationSet({
    title: 'Test Conversation',
    role: TEST_ROLE_USER,
  });
  const convID = idFrom(convMeta);
  const expMeta = await experimentCreate({ conversation: convID });
  const expID = idFrom(expMeta);
  await messageSet({
    text: 'Hello from user12',
    experiment: expID,
    title: 'Greeting',
    tags: 'hello',
    role: TEST_ROLE_USER,
  });
  await messageSet({
    text: 'Build started',
    experiment: expID,
    title: 'BuildStart',
    tags: 'build',
    role: TEST_ROLE_USER,
  });
  await messageSet({
    text: 'Onboarding checklist updated',
    experiment: expID,
    title: 'DocsUpdate',
    tags: 'docs,update',
    role: TEST_ROLE_USER,
  });

  const q1 = idFrom(
    await queueAdd({
      description: 'Run quick unit subset',
      status: 'Waiting',
      why: 'waiting for CI window',
      tags: 'kind=test,priority=low',
    }),
  );
  await queueAdd({
    description: 'Run full integration suite',
    status: 'Buildable',
    tags: 'kind=test,priority=high',
  });
  await queueAdd({
    description: 'Strict lint pass',
    status: 'Blocked',
    why: 'env not ready',
    tags: 'kind=lint',
  });

  await queuePeek({ limit: 2 });
  await queueSize();
  await queueTake({ id: q1 });
  {
    const convs = await conversationListJSON({
      role: TEST_ROLE_USER,
      limit: 50,
    });
    validateConversationListContract(convs);
    const msgs = await messageListJSON({ role: TEST_ROLE_USER, limit: 50 });
    validateMessageListContract(msgs);
  }

  // 12) Listings & counts
  step++;
  logStep(step, TOTAL, 'Listing entities and counts (per-role and JSON)');
  await listWithRole('workflow', TEST_ROLE_USER, 50);
  await listWithRole('task', TEST_ROLE_USER, 50);
  await listWithRole('conversation', TEST_ROLE_USER, 50);
  await experimentList(50);
  await listWithRole('message', TEST_ROLE_USER, 50);
  await listWithRole('project', TEST_ROLE_USER, 50);
  await listWithRole('workspace', TEST_ROLE_USER, 50);
  await listWithRole('script', TEST_ROLE_USER, 50);
  await listWithRole('store', TEST_ROLE_USER, 50);
  await listWithRole('topic', TEST_ROLE_USER, 50);
  await listWithRole('blackboard', TEST_ROLE_USER, 50);
  await stickieList(50);
  await stickieListByBlackboard({ blackboard: bb1, limit: 50 });
  // Regression: stickie list includes complex name; stickie find resolves by complex name and blackboard
  {
    const stList = await stickieListJSON({ blackboard: bb1 });
    validateStickieListContract(stList);
    const byId = (id) =>
      (stList || []).find((x) => x && (x.id === id || x.ID === id));
    const s1json = byId(st1);
    assert(
      s1json && s1json.name === 'Onboarding Refresh',
      'stickie list json missing or wrong name for st1',
    );
    const s2json = byId(st2);
    assert(
      s2json && s2json.name === 'DevOps Caching',
      'stickie list json missing or wrong name for st2',
    );
    const f1 = await stickieFind({
      name: 'Onboarding Refresh',
      variant: '',
      blackboard: bb1,
    });
    assert(
      f1 && f1.id === st1,
      'stickie find did not resolve st1 by complex name within board',
    );
  }
  await stickieListByTopic({
    topicName: 'devops',
    topicRole: TEST_ROLE_USER,
    limit: 50,
  });
  await stickieRelList({ id: st1, direction: 'out' });
  await stickieRelGet({
    from: st1,
    to: st2,
    type: 'uses',
    ignoreMissing: true,
  });
  await listWithRole('tag', TEST_ROLE_USER, 50);
  await dbCountPerRole();
  await dbCountJSON();

  // 13) Snapshot (backup/list/show/restore-dry/delete)
  step++;
  if (!SKIP_SNAPSHOT) {
    logStep(
      step,
      TOTAL,
      'Snapshot backup smoke test (backup/list/show/restore dry-run/delete)',
    );
    const bkp = await snapshotBackupJSON({
      description: 'rbctest snapshot',
      who: TEST_ROLE_USER,
    });
    const bkpID = idFrom(bkp);
    await snapshotList({ limit: 5 });
    await snapshotShow({ id: bkpID });
    const verifyRows = await snapshotVerifyJSON({ id: bkpID });
    assert(
      Array.isArray(verifyRows),
      'snapshot verify did not return JSON array',
    );
    const prunePreview = await snapshotPrunePreviewJSON({ olderThan: '0d' });
    assert(
      prunePreview &&
        typeof prunePreview.candidates === 'number' &&
        prunePreview.candidates >= 1,
      'snapshot prune preview unexpected',
    );
    await snapshotRestoreDry({ id: bkpID, mode: 'append' });
    await snapshotDelete({ id: bkpID });
  } else {
    logStep(step, TOTAL, 'Skipping snapshot (--skip-snapshot)');
  }

  // 14) Done
  step++;
  logStep(step, TOTAL, 'Done.');
} catch (err) {
  const msg = err && (err.stack || err.stderr || err.message || String(err));
  const extra =
    err &&
    (err.stdout ? `\nstdout:\n${err.stdout}` : '') +
      (err.stderr ? `\nstderr:\n${err.stderr}` : '');
  console.error('Test-all failed:', msg, extra);
  process.exit(1);
}
