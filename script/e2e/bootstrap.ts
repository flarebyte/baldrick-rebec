import {
  createScript,
  dbReset,
  dbScaffoldAll,
  enableAssertConnect,
  idFrom,
  roleGetJSON,
  roleListJSON,
  runSetRole,
  runSetTask,
  runSetWorkflow,
  scriptFind,
  scriptListJSON,
  tagSet,
  taskListJSON,
  taskScriptAdd,
  taskSetReplacement,
  workflowListJSON,
} from './cli-helper';
import {
  validateRoleContract,
  validateRoleListContract,
  validateScriptListContract,
  validateTaskListContract,
  validateWorkflowListContract,
} from './contract-helper';
import type { E2EContext } from './types';

export async function runBootstrap(ctx: E2EContext) {
  ctx.nextStep(
    ctx.SKIP_RESET
      ? 'Skipping reset (--skip-reset)'
      : 'Resetting database (destructive)',
  );
  if (!ctx.SKIP_RESET) {
    await dbReset({ dropAppRole: false });
  }

  ctx.nextStep(
    'Scaffolding roles, database, privileges, schema, content index, backup grants',
  );
  await dbScaffoldAll();
  await enableAssertConnect();
  await ctx.checkStep('db scaffolded', true, 'db scaffold should succeed');

  ctx.nextStep('Ensuring roles for test users (FK for packages)');
  await runSetRole({ name: ctx.TEST_ROLE_USER, title: 'RBCTest User' });
  await runSetRole({ name: ctx.TEST_ROLE_QA, title: 'RBCTest QA' });
  await runSetRole({ name: 'dev', title: 'Software Engineer' });
  {
    const rUser = await roleGetJSON({ name: ctx.TEST_ROLE_USER });
    validateRoleContract(rUser, { allowEmptyTitle: false });
    const rQA = await roleGetJSON({ name: ctx.TEST_ROLE_QA });
    validateRoleContract(rQA, { allowEmptyTitle: false });
    const rList = await roleListJSON({ limit: 200 });
    const parsed = validateRoleListContract(rList, { allowEmptyTitle: false });
    await ctx.checkStep(
      'roles seeded',
      parsed.length >= 3,
      'expected at least the 3 test roles in role list',
    );
    const rDev = await roleGetJSON({ name: 'dev' });
    validateRoleContract(rDev, { allowEmptyTitle: false });
  }

  ctx.nextStep('Creating workflows');
  await runSetWorkflow({
    name: 'ci-test',
    title: 'Continuous Integration: Test Suite',
    description: 'Runs unit and integration tests.',
    notes: 'CI test workflow',
    role: ctx.TEST_ROLE_USER,
  });
  await runSetWorkflow({
    name: 'ci-lint',
    title: 'Continuous Integration: Lint & Format',
    description: 'Lints and vets the codebase.',
    notes: 'CI lint workflow',
    role: ctx.TEST_ROLE_USER,
  });
  {
    const wfList = await workflowListJSON({
      role: ctx.TEST_ROLE_USER,
      limit: 50,
    });
    validateWorkflowListContract(wfList, { allowEmptyTitle: false });
  }
  await ctx.checkStep(
    'workflows created',
    true,
    'workflows were created and listed',
  );

  ctx.nextStep('Creating scripts and capturing ids');
  ctx.state.sidUnit = await createScript(
    ctx.TEST_ROLE_USER,
    'Unit: go test',
    'Run unit tests',
    '#!/usr/bin/env bash\nset -euo pipefail\ngo test ./...\n',
  );
  ctx.state.sidInteg = await createScript(
    ctx.TEST_ROLE_USER,
    'Integration: compose+test',
    'Run integration tests',
    '#!/usr/bin/env bash\nset -euo pipefail\ndocker compose up -d && go test -tags=integration ./...\n',
  );
  ctx.state.sidLint = await createScript(
    ctx.TEST_ROLE_USER,
    'Lint & Vet',
    'Runs vet and lints',
    '#!/usr/bin/env bash\nset -euo pipefail\ngo vet ./... && echo linting...\n',
  );
  ctx.state.sidLs = await createScript(
    ctx.TEST_ROLE_USER,
    'List directory',
    'Demo: ls -la',
    '#!/usr/bin/env bash\nset -euo pipefail\nls -la\n',
  );
  ctx.state.sidLsAll = await createScript(
    ctx.TEST_ROLE_USER,
    'List all files',
    'Demo: ls -la (all files)',
    '#!/usr/bin/env bash\nset -euo pipefail\nls -la\n',
  );
  ctx.state.sidLsDirs = await createScript(
    ctx.TEST_ROLE_USER,
    'List directories only',
    'Demo: ls -la | grep ^d',
    '#!/usr/bin/env bash\nset -euo pipefail\nls -la | grep ^d || true\n',
  );
  {
    const listJSON = await scriptListJSON({ role: ctx.TEST_ROLE_USER });
    const parsedScripts = validateScriptListContract(listJSON, {
      allowEmptyTitle: false,
    });
    const byId = (id: string) => parsedScripts.find((x) => x.id === id);
    const ju = byId(ctx.state.sidUnit || '');
    ctx.check(ju, 'script list json missing unit script');
    ctx.check(
      ju.name === 'Unit: go test',
      'unit script name mismatch in list json',
    );
    ctx.check(
      (ju.variant ?? '') === '',
      'unit script variant should be empty in list json',
    );

    const ji = byId(ctx.state.sidInteg || '');
    ctx.check(
      ji && ji.name === 'Integration: compose+test',
      'integration script not present or name mismatch',
    );

    const jl = byId(ctx.state.sidLint || '');
    ctx.check(
      jl && jl.name === 'Lint & Vet',
      'lint script not present or name mismatch',
    );

    const foundUnit = await scriptFind({
      name: 'Unit: go test',
      variant: '',
      role: ctx.TEST_ROLE_USER,
    });
    ctx.check(
      foundUnit && foundUnit.id === ctx.state.sidUnit,
      'script find did not resolve unit by complex name',
    );
  }
  await ctx.checkStep(
    'scripts created',
    true,
    'scripts were created and validated',
  );

  ctx.nextStep('Creating tasks');
  ctx.state.tUnit = idFrom(
    await runSetTask({
      workflow: 'ci-test',
      command: 'unit',
      variant: 'go',
      role: ctx.TEST_ROLE_USER,
      title: 'Run Unit Tests',
      description: 'Executes unit tests.',
      shell: 'bash',
      timeout: '10 minutes',
      tags: 'unit,fast',
      level: 'h2',
    }),
  );
  {
    const tList = await taskListJSON({ role: ctx.TEST_ROLE_USER, limit: 50 });
    validateTaskListContract(tList, { allowEmptyTitle: true });
  }
  await runSetTask({
    workflow: 'ci-test',
    command: 'integration',
    variant: '',
    role: ctx.TEST_ROLE_USER,
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
    role: ctx.TEST_ROLE_USER,
    title: 'Lint & Vet',
    description: 'Runs vet and lints.',
    shell: 'bash',
    timeout: '5 minutes',
    tags: 'lint,style',
    level: 'h2',
  });

  ctx.state.tList = idFrom(
    await runSetTask({
      workflow: 'ci-test',
      command: 'lsdemo',
      variant: 'lsdemo/go',
      role: ctx.TEST_ROLE_USER,
      title: 'List workspace',
      description: 'Runs ls -la',
      shell: 'bash',
      timeout: '30 seconds',
      tags: 'demo,ls',
      level: 'h3',
    }),
  );

  await taskScriptAdd({
    task: ctx.state.tList || '',
    script: ctx.state.sidLs || '',
    name: 'list',
  });
  await taskScriptAdd({
    task: ctx.state.tList || '',
    script: ctx.state.sidLsAll || '',
    name: 'list-all',
  });
  await taskScriptAdd({
    task: ctx.state.tList || '',
    script: ctx.state.sidLsDirs || '',
    name: 'list-dirs',
  });

  await taskSetReplacement({
    workflow: 'ci-test',
    command: 'unit',
    variant: 'go-patch1',
    role: ctx.TEST_ROLE_USER,
    title: 'Run Unit Tests (Quick)',
    description: 'Patch: run quick subset',
    shell: 'bash',
    replaces: ctx.state.tUnit,
    replaceLevel: 'patch',
    replaceComment: 'Flaky test workaround',
  });
  await taskSetReplacement({
    workflow: 'ci-test',
    command: 'unit',
    variant: 'go-minor1',
    role: ctx.TEST_ROLE_USER,
    title: 'Run Unit Tests (Race)',
    description: 'Minor: enable race detector',
    shell: 'bash',
    replaces: ctx.state.tUnit,
    replaceLevel: 'minor',
    replaceComment: 'Add -race',
  });

  ctx.nextStep('Creating tags');
  await tagSet({
    name: 'priority-high',
    title: 'High Priority',
    role: ctx.TEST_ROLE_USER,
  });
}
