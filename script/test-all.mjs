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
  conversationGetJSON,
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
  projectGetJSON,
  projectListJSON,
  projectSet,
  promptRunJSON,
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
  stickieGetJSON,
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
  taskScriptAdd,
  taskSetReplacement,
  testcaseCreate,
  testcaseListJSON,
  toolGetJSON,
  toolListJSON,
  toolSet,
  topicListJSON,
  topicSet,
  vaultBackendCurrent,
  vaultDoctor,
  vaultList,
  vaultShow,
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
  validateVaultListContract,
  validateVaultShowContract,
  validateWorkflowListContract,
} from './contract-helper.mjs';

// Connect client imported dynamically in the step to avoid hard failure

// Note: sleep helper removed until needed; ZX provides sleep() globally.

// -----------------------------
// Flow
// -----------------------------
const TOTAL = 19;
let step = 0;

try {
  // 1) Reset
  step++;
  if (!SKIP_RESET) {
    logStep(step, TOTAL, 'Resetting database (destructive)');
    await dbReset({ dropAppRole: false });
  } else {
    logStep(step, TOTAL, 'Skipping reset (--skip-reset)');
    // Create a blackboard anchored to the complete-store to surface joined Store fields in the TUI
    const storeIdFull = sfull.id || sfull.ID || '';
    if (storeIdFull) {
      await blackboardSet({
        role: TEST_ROLE_USER,
        storeId: storeIdFull,
        project: 'acme/complete',
        background: 'Board for complete-store demo',
        guidelines: 'Keep entries consistent; test UI fields',
      });
    }
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
  // Simple demo script: list files
  const sidLs = await createScript(
    TEST_ROLE_USER,
    'List directory',
    'Demo: ls -la',
    '#!/usr/bin/env bash\nset -euo pipefail\nls -la\n',
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
  // Demo task using the ls script
  const tList = idFrom(
    await runSetTask({
      workflow: 'ci-test',
      command: 'lsdemo',
      variant: '',
      role: TEST_ROLE_USER,
      title: 'List workspace',
      description: 'Runs ls -la',
      shell: 'bash',
      timeout: '30 seconds',
      tags: 'demo,ls',
      level: 'h3',
    }),
  );
  // Attach the ls script to the lsdemo task
  await taskScriptAdd({ task: tList, script: sidLs, name: 'list' });

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
    notes: 'Notes: CI with Go and Docker',
    tags: 'status=active,type=ci',
  });
  await projectSet({
    name: 'acme/product',
    role: TEST_ROLE_USER,
    description: 'Main product',
    notes: 'Notes: App repo',
    tags: 'status=active,type=app',
  });
  // Ensure one project has all fields populated
  await projectSet({
    name: 'acme/complete',
    role: TEST_ROLE_USER,
    description: 'Complete metadata project',
    notes: 'Project notes filled',
    tags: 'area=complete,stage=alpha',
  });
  {
    const pj = await projectGetJSON({
      name: 'acme/complete',
      role: TEST_ROLE_USER,
    });
    assert(
      pj && pj.name === 'acme/complete',
      'project complete: name mismatch',
    );
    assert(
      pj.description === 'Complete metadata project',
      'project complete: description missing',
    );
    assert(
      pj.notes === 'Project notes filled',
      'project complete: notes missing',
    );
  }
  {
    const prj = await projectListJSON({ role: TEST_ROLE_USER, limit: 50 });
    validateProjectListContract(prj);
  }

  // 7.5) Tools
  step++;
  logStep(step, TOTAL, 'Creating tools and verifying CRUD');
  await toolSet({
    name: 'acme-linter',
    title: 'Acme Linter',
    role: TEST_ROLE_USER,
    description: 'Lints code with custom rules',
    tags: 'lang=go,scope=lint',
    settings: JSON.stringify({ severity: 'strict', autofix: true }),
    type: 'linter',
  });
  await toolSet({
    name: 'acme-formatter',
    title: 'Acme Formatter',
    role: TEST_ROLE_USER,
    description: 'Formats code',
    tags: 'lang=go,scope=format',
    settings: JSON.stringify({ style: 'gofmt' }),
    type: 'formatter',
  });
  {
    const tList = await toolListJSON({ role: TEST_ROLE_USER, limit: 50 });
    assert(
      Array.isArray(tList) && tList.length >= 2,
      'expected at least 2 tools in list',
    );
    const hasLinter = tList.find(
      (x) => x && x.name === 'acme-linter' && x.title === 'Acme Linter',
    );
    const hasFmt = tList.find(
      (x) => x && x.name === 'acme-formatter' && x.title === 'Acme Formatter',
    );
    assert(!!hasLinter, 'tool list missing acme-linter');
    assert(!!hasFmt, 'tool list missing acme-formatter');
    const t1 = await toolGetJSON({ name: 'acme-linter' });
    assert(
      t1 && t1.name === 'acme-linter' && t1.role === TEST_ROLE_USER,
      'tool get failed for acme-linter',
    );
    assert(
      t1.settings && t1.settings.autofix === true,
      'tool get missing settings for acme-linter',
    );
  }

  // 7.6) Prompt (Ollama gemma3:1b) — optional
  step++;
  logStep(step, TOTAL, 'Prompt run via Ollama (gemma3:1b), if available');
  try {
    const OLLAMA_BASE_URL =
      process.env.OLLAMA_BASE_URL || 'http://127.0.0.1:11434';
    await toolSet({
      name: 'ollama-gemma',
      title: 'Ollama Gemma 1B',
      role: TEST_ROLE_USER,
      description: 'Local small model for tests',
      settings: JSON.stringify({
        provider: 'ollama',
        model: 'gemma3:1b',
        base_url: OLLAMA_BASE_URL,
      }),
      type: 'llm',
    });
    const out = await promptRunJSON({
      toolName: 'ollama-gemma',
      input: 'Say "hello" in one short line.',
      maxOutputTokens: 64,
    });
    // Basic shape assertions when available
    if (out) {
      assert(out.object === 'response', 'prompt: expected response object');
      assert(typeof out.model === 'string', 'prompt: model string');
      assert(Array.isArray(out.output), 'prompt: output array');
    }
  } catch (e) {
    console.error('prompt (ollama) skipped:', e?.message || String(e));
  }

  // 7.7) Connect client (optional)
  step++;
  logStep(step, TOTAL, 'Prompt via Connect client (if server available)');
  try {
    // Start server in background if not running
    await $`go run main.go admin server start --detach`;
    // Wait for health endpoint instead of fixed sleep to avoid race conditions
    try {
      await (async function waitHealth() {
        for (let i = 0; i < 50; i++) {
          try {
            const r = await fetch('http://127.0.0.1:53051/health');
            if (r?.ok) return;
          } catch {}
          await sleep(100);
        }
        throw new Error('health timeout');
      })();
    } catch {}
    // Import connect client; if dependency missing, install script deps and retry
    let createConnectGrpcJsonClient;
    try {
      ({ createConnectGrpcJsonClient } = await import(
        './grpc-json-client-connect.mjs'
      ));
    } catch (e) {
      const msg = e?.message || String(e || '');
      if (msg.includes("'@connectrpc/connect-node'")) {
        // Install dependencies declared in script/package.json
        await $`npm --prefix script install --silent`;
        ({ createConnectGrpcJsonClient } = await import(
          './grpc-json-client-connect.mjs'
        ));
      } else {
        throw e;
      }
    }
    const client = createConnectGrpcJsonClient({
      baseUrl: 'http://127.0.0.1:53051',
    });
    const out = await client.Run({
      tool_name: 'ollama-gemma',
      input: 'Say "hello" in one short line.',
      max_output_tokens: 64,
    });
    if (out) {
      assert(
        out.object === 'response',
        'connect client: expected response object',
      );
      assert(Array.isArray(out.output), 'connect client: output array');
    }
  } finally {
    try {
      await $`go run main.go admin server stop`;
    } catch {}
  }

  // 8) Stores & Blackboards
  step++;
  logStep(step, TOTAL, 'Creating stores and blackboards');
  await storeSet({
    name: 'ideas-acme-build',
    role: TEST_ROLE_USER,
    title: 'Ideas for acme/build-system',
    description: 'Idea backlog',
    motivation: 'Capture and prioritize improvement ideas',
    security: 'Internal only',
    privacy: 'No PII expected',
    notes: 'Markdown allowed: keep entries concise',
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
      score: 0.42,
    }),
  );
  const st2 = idFrom(
    await stickieSet({
      blackboard: bb1,
      topicName: 'devops',
      topicRole: TEST_ROLE_USER,
      note: 'Evaluate GitHub Actions caching for go build',
      code: 'name: CI\n\non: [push]\n\njobs:\n  build:\n    runs-on: ubuntu-latest\n    steps:\n      - uses: actions/checkout@v4\n      - uses: actions/setup-go@v5\n      - run: go build ./...\n',
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
      code: 'package main\n\nimport "fmt"\n\nfunc main() {\n  fmt.Println("Hello, RBCTest!")\n}\n',
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

  // Validate score set on create and update-after-create
  {
    const g1 = await stickieGetJSON({ id: st1 });
    assert(
      typeof g1.score === 'number' && Math.abs(g1.score - 0.42) < 1e-9,
      'stickie st1 score should be 0.42 after create',
    );
    // Update stickie 2 with a score
    await stickieSet({ id: st2, score: 0.99 });
    const g2 = await stickieGetJSON({ id: st2 });
    assert(
      typeof g2.score === 'number' && Math.abs(g2.score - 0.99) < 1e-9,
      'stickie st2 score should be 0.99 after update',
    );
  }
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

  // Ensure one store has all fields populated
  await storeSet({
    name: 'complete-store',
    role: TEST_ROLE_USER,
    title: 'Complete Store',
    description: 'Store with all fields',
    motivation: 'Centralize artifacts',
    security: 'Internal only',
    privacy: 'No PII',
    notes: 'Markdown: some details here',
    type: 'journal',
    scope: 'shared',
    lifecycle: 'weekly',
    tags: 'env=dev,owner=qa',
  });
  {
    const sfull = await storeGet({
      name: 'complete-store',
      role: TEST_ROLE_USER,
    });
    assert(
      sfull && (sfull.id || sfull.ID || sfull.name === 'complete-store'),
      'store complete: not found',
    );
    assert(sfull.title === 'Complete Store', 'store complete: title mismatch');
    assert(
      sfull.description === 'Store with all fields',
      'store complete: description missing',
    );
    assert(
      sfull.motivation === 'Centralize artifacts',
      'store complete: motivation missing',
    );
    assert(
      sfull.security === 'Internal only',
      'store complete: security missing',
    );
    assert(sfull.privacy === 'No PII', 'store complete: privacy missing');
    assert(
      sfull.notes === 'Markdown: some details here',
      'store complete: notes missing',
    );
    assert(sfull.type === 'journal', 'store complete: type missing');
    assert(sfull.scope === 'shared', 'store complete: scope missing');
    assert(sfull.lifecycle === 'weekly', 'store complete: lifecycle missing');
  }

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

  // Create a second conversation with populated metadata fields
  const convMeta2 = await conversationSet({
    title: 'QA Discussion',
    role: TEST_ROLE_QA,
    description: 'Quality assurance planning and triage',
    project: 'acme/quality',
    tags: 'area=qa,priority=high,triage',
    notes: 'Weekly QA sync notes',
  });
  const convID2 = idFrom(convMeta2);
  {
    const c2 = await conversationGetJSON({ id: convID2 });
    assert(c2 && c2.id === convID2, 'conv2: id mismatch');
    assert(c2.title === 'QA Discussion', 'conv2: title mismatch');
    assert(
      c2.description === 'Quality assurance planning and triage',
      'conv2: description missing',
    );
    assert(c2.project === 'acme/quality', 'conv2: project missing');
    assert(c2.notes === 'Weekly QA sync notes', 'conv2: notes missing');
    assert(c2.tags && typeof c2.tags === 'object', 'conv2: tags missing');
    assert(c2.tags.area === 'qa', 'conv2: tag area=qa missing');
  }

  // 11.5) Testcases
  step++;
  logStep(step, TOTAL, 'Creating testcases and verifying listing');
  await testcaseCreate({
    title: 'Unit: go vet',
    role: TEST_ROLE_USER,
    experiment: expID,
    status: 'OK',
    level: 'h1',
    name: 'vet-basic',
    pkg: 'acme/build',
    classname: 'lint.Vet',
    file: 'main.go',
    line: 12,
    executionTime: 1.23,
  });
  await testcaseCreate({
    title: 'Unit: go fmt',
    role: TEST_ROLE_USER,
    experiment: expID,
    status: 'OK',
    level: 'h2',
    name: 'fmt-style',
    pkg: 'acme/build',
    classname: 'format.Fmt',
    file: 'util.go',
    line: 7,
    executionTime: 0.42,
  });
  await testcaseCreate({
    title: 'Lint: misspell',
    role: TEST_ROLE_USER,
    experiment: expID,
    status: 'KO',
    level: 'h3',
    name: 'misspell',
    pkg: 'acme/build',
    classname: 'lint.Misspell',
    error: 'found “teh” in README.md',
    file: 'README.md',
    line: 3,
    executionTime: 0.33,
  });
  // Additional cases including TODO and mixed levels
  await testcaseCreate({
    title: 'Integration: DB connect smoke',
    role: TEST_ROLE_USER,
    experiment: expID,
    status: 'TODO',
    level: 'h1',
    name: 'db-connect',
    pkg: 'acme/integration',
    classname: 'integration.DB',
    file: 'db_test.go',
    line: 5,
  });
  await testcaseCreate({
    title: 'Unit: edge cases',
    role: TEST_ROLE_USER,
    experiment: expID,
    status: 'OK',
    level: 'h3',
    name: 'edge-cases',
    pkg: 'acme/build',
    classname: 'unit.Edge',
    file: 'edge_test.go',
    line: 21,
    executionTime: 0.05,
  });
  {
    const tcs = await testcaseListJSON({
      role: TEST_ROLE_USER,
      experiment: expID,
      limit: 50,
    });
    assert(
      Array.isArray(tcs) && tcs.length >= 5,
      'expected at least 5 testcases',
    );
    const gotVet = tcs.find(
      (x) => x?.title === 'Unit: go vet' && x?.status === 'OK',
    );
    const gotMisspell = tcs.find(
      (x) => x?.title === 'Lint: misspell' && x?.status === 'KO',
    );
    const gotTodo = tcs.find(
      (x) =>
        x?.title === 'Integration: DB connect smoke' &&
        x?.status?.toUpperCase() === 'TODO',
    );
    assert(!!gotVet, 'missing testcase: go vet');
    assert(!!gotMisspell, 'missing testcase: misspell');
    assert(!!gotTodo, 'missing testcase: integration DB connect (TODO)');
  }

  // Create a second experiment and attach a different set of testcases
  logStep(step, TOTAL, 'Creating additional testcases under a new experiment');
  const expMeta2 = await experimentCreate({ conversation: convID });
  const expID2 = idFrom(expMeta2);
  await testcaseCreate({
    title: 'Unit: string utils',
    role: TEST_ROLE_USER,
    experiment: expID2,
    status: 'OK',
    level: 'h2',
    name: 'string-utils',
    pkg: 'acme/build',
    classname: 'unit.Strings',
    file: 'strings_test.go',
    line: 15,
    executionTime: 0.11,
  });
  await testcaseCreate({
    title: 'Integration: API smoke',
    role: TEST_ROLE_USER,
    experiment: expID2,
    status: 'KO',
    level: 'h2',
    name: 'api-smoke',
    pkg: 'acme/integration',
    classname: 'integration.API',
    error: 'timeout contacting service',
    file: 'api_test.go',
    line: 27,
    executionTime: 2.5,
  });
  await testcaseCreate({
    title: 'Unit: parsing basics',
    role: TEST_ROLE_USER,
    experiment: expID2,
    status: 'TODO',
    level: 'h1',
    name: 'parsing-basics',
    pkg: 'acme/build',
    classname: 'unit.Parser',
    file: 'parse_test.go',
    line: 3,
  });
  {
    const tcs2 = await testcaseListJSON({
      role: TEST_ROLE_USER,
      experiment: expID2,
      limit: 50,
    });
    assert(
      Array.isArray(tcs2) && tcs2.length >= 3,
      'expected at least 3 testcases in second experiment',
    );
    const gotAPI = tcs2.find(
      (x) => x?.title === 'Integration: API smoke' && x?.status === 'KO',
    );
    const gotStrings = tcs2.find(
      (x) => x?.title === 'Unit: string utils' && x?.status === 'OK',
    );
    assert(!!gotAPI, 'missing testcase in exp2: API smoke');
    assert(!!gotStrings, 'missing testcase in exp2: string utils');
  }

  // 11.6) Testcases via Connect JSON (start server, create+list+delete one)
  step++;
  logStep(step, TOTAL, 'Testcases via Connect JSON service');
  try {
    await $`go run main.go admin server start --detach`;
    await sleep(1500);
    // Use fetch against Connect JSON endpoints
    const endpoint = (m) =>
      `http://127.0.0.1:53051/testcase.v1.TestcaseService/${m}`;
    const post = async (m, body) => {
      const res = await fetch(endpoint(m), {
        method: 'POST',
        headers: { 'content-type': 'application/connect+json' },
        body: JSON.stringify(body || {}),
      });
      const txt = await res.text();
      try {
        return JSON.parse(txt || 'null');
      } catch {
        throw new Error(`invalid json: ${txt}`);
      }
    };
    // Create a temporary testcase via Connect JSON
    const created = await post('Create', {
      title: 'GRPC: smoke',
      role: TEST_ROLE_USER,
      experiment: expID,
      status: 'OK',
      file: 'grpc.json',
      line: 1,
    });
    assert(created?.id, 'grpc testcase create missing id');
    // List and assert presence
    const listed = await post('List', {
      role: TEST_ROLE_USER,
      experiment: expID,
      limit: 10,
      offset: 0,
    });
    assert(
      listed && Array.isArray(listed.items),
      'grpc testcase list missing items',
    );
    const found = (listed?.items ?? []).find((x) => x?.id === created.id);
    assert(!!found, 'grpc testcase not found in list');
    // Delete
    const del = await post('Delete', { id: created.id });
    assert(
      del && (del.deleted === 1 || del.deleted === '1'),
      'grpc delete did not report 1',
    );
  } catch (e) {
    const msg = e?.message ?? String(e);
    if (msg?.includes('404')) {
      console.error('grpc testcase step skipped:', msg);
    } else {
      console.error('grpc testcase step failed:', msg);
      throw e;
    }
  } finally {
    try {
      await $`go run main.go admin server stop`;
    } catch {}
  }

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

  // 15) Vault (read-only; conditional on presence of rbctest-key)
  step++;
  logStep(step, TOTAL, 'Vault read-only checks (if configured)');
  try {
    const items = await vaultList();
    validateVaultListContract(items);
    const exists = items.find(
      (x) => x.name === 'rbctest-key' && x.status === 'set',
    );
    if (exists) {
      const md = await vaultShow('rbctest-key');
      validateVaultShowContract(md);
      assert(md.name === 'rbctest-key', 'vault.show name matches');
      assert(md.status === 'set', 'vault.show status is set');
      const backend = await vaultBackendCurrent();
      assert(backend === 'keychain', 'vault backend current is keychain');
      await vaultDoctor();
    } else {
      console.error('vault: rbctest-key not set; skipping deep checks');
    }
  } catch (e) {
    console.error('vault: checks skipped due to error:', e?.message || e);
  }

  // 16) Done
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
