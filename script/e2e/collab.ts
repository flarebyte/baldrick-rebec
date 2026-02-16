import { createConnectGrpcJsonClient } from '../grpc-json-client-connect.mjs';
import {
  assert,
  assertStep,
  conversationGetJSON,
  conversationListJSON,
  conversationSet,
  enableAssertConnect,
  experimentCreate,
  idFrom,
  messageListJSON,
  messageSet,
  packageSet,
  queueAdd,
  queuePeek,
  queueSize,
  queueTake,
  runRbc,
  testcaseCreate,
  testcaseListJSON,
  workspaceSet,
} from './cli-helper';
import {
  validateConversationListContract,
  validateMessageListContract,
} from './contract-helper';
import type { E2EContext } from './types';

export async function runCollab(ctx: E2EContext) {
  ctx.nextStep('Creating workspaces and packages');
  await workspaceSet({
    role: ctx.TEST_ROLE_USER,
    project: 'acme/build-system',
    description: 'Local build-system workspace',
    tags: 'status=active',
  });
  await workspaceSet({
    role: ctx.TEST_ROLE_USER,
    project: 'acme/product',
    description: 'Local product workspace',
    tags: 'status=active',
  });
  await packageSet({ role: ctx.TEST_ROLE_USER, variant: 'unit/go' });
  await packageSet({ role: ctx.TEST_ROLE_QA, variant: 'integration' });
  await packageSet({ role: ctx.TEST_ROLE_USER, variant: 'lint/go' });

  ctx.nextStep('Creating conversation, experiment, messages and queue');
  const convMeta = await conversationSet({
    title: 'Test Conversation',
    role: ctx.TEST_ROLE_USER,
  });
  ctx.state.convID = idFrom(convMeta);
  const expMeta = await experimentCreate({
    conversation: ctx.state.convID || '',
  });
  ctx.state.expID = idFrom(expMeta);

  await messageSet({
    text: 'Hello from user12',
    experiment: ctx.state.expID,
    title: 'Greeting',
    tags: 'hello',
    role: ctx.TEST_ROLE_USER,
  });
  await messageSet({
    text: 'Build started',
    experiment: ctx.state.expID,
    title: 'BuildStart',
    tags: 'build',
    role: ctx.TEST_ROLE_USER,
  });
  await messageSet({
    text: 'Onboarding checklist updated',
    experiment: ctx.state.expID,
    title: 'DocsUpdate',
    tags: 'docs,update',
    role: ctx.TEST_ROLE_USER,
  });

  const convMeta2 = await conversationSet({
    title: 'QA Discussion',
    role: ctx.TEST_ROLE_QA,
    description: 'Quality assurance planning and triage',
    project: 'acme/quality',
    tags: 'area=qa,priority=high,triage',
    notes: 'Weekly QA sync notes',
  });
  const convID2 = idFrom(convMeta2);
  {
    const c2 = await conversationGetJSON({ id: convID2 });
    const okConv2 =
      !!c2 &&
      c2.id === convID2 &&
      c2.title === 'QA Discussion' &&
      c2.description === 'Quality assurance planning and triage' &&
      c2.project === 'acme/quality' &&
      c2.notes === 'Weekly QA sync notes' &&
      c2.tags &&
      typeof c2.tags === 'object' &&
      c2.tags.area === 'qa';
    await assertStep(
      'conversation 2 validated',
      okConv2,
      'conv2 fields mismatch',
    );
  }

  ctx.nextStep('Creating testcases and verifying listing');
  await testcaseCreate({
    title: 'Unit: go vet',
    role: ctx.TEST_ROLE_USER,
    experiment: ctx.state.expID,
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
    role: ctx.TEST_ROLE_USER,
    experiment: ctx.state.expID,
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
    role: ctx.TEST_ROLE_USER,
    experiment: ctx.state.expID,
    status: 'KO',
    level: 'h3',
    name: 'misspell',
    pkg: 'acme/build',
    classname: 'lint.Misspell',
    error: 'found "teh" in README.md',
    file: 'README.md',
    line: 3,
    executionTime: 0.33,
  });
  await testcaseCreate({
    title: 'Integration: DB connect smoke',
    role: ctx.TEST_ROLE_USER,
    experiment: ctx.state.expID,
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
    role: ctx.TEST_ROLE_USER,
    experiment: ctx.state.expID,
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
      role: ctx.TEST_ROLE_USER,
      experiment: ctx.state.expID || '',
      limit: 50,
    });
    const gotVet = tcs.find(
      (x: any) => x?.title === 'Unit: go vet' && x?.status === 'OK',
    );
    const gotMisspell = tcs.find(
      (x: any) => x?.title === 'Lint: misspell' && x?.status === 'KO',
    );
    const gotTodo = tcs.find(
      (x: any) =>
        x?.title === 'Integration: DB connect smoke' &&
        x?.status?.toUpperCase() === 'TODO',
    );
    await assertStep(
      'testcases created',
      Array.isArray(tcs) &&
        tcs.length >= 5 &&
        !!gotVet &&
        !!gotMisspell &&
        !!gotTodo,
      'expected testcases missing in first experiment',
    );
  }

  const expMeta2 = await experimentCreate({
    conversation: ctx.state.convID || '',
  });
  const expID2 = idFrom(expMeta2);
  await testcaseCreate({
    title: 'Unit: string utils',
    role: ctx.TEST_ROLE_USER,
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
    role: ctx.TEST_ROLE_USER,
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
    role: ctx.TEST_ROLE_USER,
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
      role: ctx.TEST_ROLE_USER,
      experiment: expID2,
      limit: 50,
    });
    const gotAPI = tcs2.find(
      (x: any) => x?.title === 'Integration: API smoke' && x?.status === 'KO',
    );
    const gotStrings = tcs2.find(
      (x: any) => x?.title === 'Unit: string utils' && x?.status === 'OK',
    );
    await assertStep(
      'testcases exp2 created',
      Array.isArray(tcs2) && tcs2.length >= 3 && !!gotAPI && !!gotStrings,
      'expected testcases missing in second experiment',
    );
  }

  ctx.nextStep('Testcases via Connect JSON service');
  try {
    await enableAssertConnect();
    const client = createConnectGrpcJsonClient({
      baseUrl: 'http://127.0.0.1:53051',
    });
    const created = await client.testcase.Create({
      title: 'GRPC: smoke',
      role: ctx.TEST_ROLE_USER,
      experiment: ctx.state.expID,
      status: 'OK',
      file: 'grpc.json',
      line: 1,
    });
    assert(created?.id, 'grpc testcase create missing id');
    const listed = await client.testcase.List({
      role: ctx.TEST_ROLE_USER,
      experiment: ctx.state.expID,
      limit: 10,
      offset: 0,
    });
    assert(
      listed && Array.isArray(listed.items),
      'grpc testcase list missing items',
    );
    const found = (listed?.items ?? []).find((x: any) => x?.id === created.id);
    assert(!!found, 'grpc testcase not found in list');
    const del = await client.testcase.Delete({ id: created.id });
    assert(
      del && (del.deleted === 1 || del.deleted === '1'),
      'grpc delete did not report 1',
    );
  } catch (e) {
    const msg = (e as Error)?.message ?? String(e);
    if (msg?.includes('404')) {
      console.error('grpc testcase step skipped:', msg);
    } else {
      console.error('grpc testcase step failed:', msg);
      throw e;
    }
  } finally {
    try {
      await runRbc('server', 'stop');
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
      role: ctx.TEST_ROLE_USER,
      limit: 50,
    });
    validateConversationListContract(convs);
    const msgs = await messageListJSON({ role: ctx.TEST_ROLE_USER, limit: 50 });
    validateMessageListContract(msgs);
  }
}
