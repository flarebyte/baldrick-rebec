import { createConnectGrpcJsonClient } from '../grpc-json-client-connect.mjs';
import {
  assert,
  assertStep,
  projectGetJSON,
  projectListJSON,
  projectSet,
  promptRunJSON,
  runRbc,
  runShell,
  toolGetJSON,
  toolListJSON,
  toolSet,
} from './cli-helper';
import { validateProjectListContract } from './contract-helper';
import type { E2EContext } from './types';

function sleep(ms: number) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

export async function runProjectToolPrompt(ctx: E2EContext) {
  ctx.nextStep('Creating projects');
  await projectSet({
    name: 'acme/build-system',
    role: ctx.TEST_ROLE_USER,
    description: 'Build system and CI pipeline',
    notes: 'Notes: CI with Go and Docker',
    tags: 'status=active,type=ci',
  });
  await projectSet({
    name: 'acme/product',
    role: ctx.TEST_ROLE_USER,
    description: 'Main product',
    notes: 'Notes: App repo',
    tags: 'status=active,type=app',
  });
  await projectSet({
    name: 'acme/complete',
    role: ctx.TEST_ROLE_USER,
    description: 'Complete metadata project',
    notes: 'Project notes filled',
    tags: 'area=complete,stage=alpha',
  });
  await projectSet({
    name: 'github/flarebyte/baldrick-rebec',
    role: 'dev',
    description: 'Autonomous build automation tool and task runner',
    notes: 'Project for dev role',
    tags: 'source=github,org=flarebyte,version=0.6.0',
  });

  ctx.nextStep('Syncing baldrick-rebec project YAML to repo root');
  try {
    await runShell(
      'rm -f ./main.project.yaml ./github-flarebyte-baldrick-rebec.project.yaml',
    );
  } catch {}
  await runRbc(
    'project',
    'sync',
    'name:github/flarebyte/baldrick-rebec',
    'folder:.',
    '--role',
    'dev',
  );
  await runShell(
    'if [ -f ./github-flarebyte-baldrick-rebec.project.yaml ]; then mv ./github-flarebyte-baldrick-rebec.project.yaml ./main.project.yaml; fi',
  );
  const repoPrj = await runShell(
    'test -f ./main.project.yaml && echo OK || echo MISSING',
  );
  await assertStep(
    'repo project yaml exists',
    String(repoPrj.stdout || '').includes('OK'),
    'expected main.project.yaml at repo root',
  );

  {
    const pj = await projectGetJSON({
      name: 'acme/complete',
      role: ctx.TEST_ROLE_USER,
    });
    const okProject =
      !!pj &&
      pj.name === 'acme/complete' &&
      pj.description === 'Complete metadata project' &&
      pj.notes === 'Project notes filled';
    await assertStep(
      'project complete validated',
      okProject,
      'project complete: fields mismatch or missing',
    );
  }
  {
    const prj = await projectListJSON({ role: ctx.TEST_ROLE_USER, limit: 50 });
    validateProjectListContract(prj);
  }

  ctx.nextStep('Exporting project to temp/project-test');
  try {
    await runShell('rm -rf temp/project-test');
  } catch {}
  await runRbc(
    'project',
    'sync',
    'name:acme/complete',
    'folder:temp/project-test',
    '--role',
    ctx.TEST_ROLE_USER,
  );
  const prjYaml = await runShell(
    'test -f temp/project-test/acme-complete.project.yaml && echo OK || echo MISSING',
  );
  const content = await runShell(
    'cat temp/project-test/acme-complete.project.yaml || true',
  );
  const hasName = String(content.stdout || '').includes('name: acme/complete');
  const hasRole = String(content.stdout || '').includes(
    `role: ${ctx.TEST_ROLE_USER}`,
  );
  await assertStep(
    'project synced to folder',
    String(prjYaml.stdout || '').includes('OK') && hasName && hasRole,
    'expected exported project YAML missing or missing required fields (name, role)',
  );
  try {
    await runRbc(
      'project',
      'sync',
      'name:acme/complete',
      'folder:temp/project-test',
      '--role',
      ctx.TEST_ROLE_USER,
      '--dry-run',
    );
    await assertStep('project sync dry-run ok', true);
  } catch {
    await assertStep(
      'project sync dry-run ok',
      false,
      'expected project sync dry-run to succeed',
    );
  }

  ctx.nextStep('Importing project from temp/project-import (folder->name)');
  try {
    await runShell('rm -rf temp/project-import');
  } catch {}
  await runShell('mkdir -p temp/project-import');
  await runShell(
    `cat > temp/project-import/acme-complete.project.yaml <<EOF\nname: acme/complete\nrole: ${ctx.TEST_ROLE_USER}\ndescription: Updated via import\nnotes: Updated via import\ntags:\n  imported: true\nEOF`,
  );
  await runRbc(
    'project',
    'sync',
    'folder:temp/project-import',
    'name:acme/complete',
  );
  {
    const pj2 = await projectGetJSON({
      name: 'acme/complete',
      role: ctx.TEST_ROLE_USER,
    });
    const ok2 =
      !!pj2 &&
      pj2.description === 'Updated via import' &&
      pj2.notes === 'Updated via import';
    await assertStep(
      'project imported and updated',
      ok2,
      'project import did not update fields as expected',
    );
  }

  ctx.nextStep('Creating tools and verifying CRUD');
  await toolSet({
    name: 'acme-linter',
    title: 'Acme Linter',
    role: ctx.TEST_ROLE_USER,
    description: 'Lints code with custom rules',
    tags: 'lang=go,scope=lint',
    settings: JSON.stringify({ severity: 'strict', autofix: true }),
    type: 'linter',
  });
  await toolSet({
    name: 'acme-formatter',
    title: 'Acme Formatter',
    role: ctx.TEST_ROLE_USER,
    description: 'Formats code',
    tags: 'lang=go,scope=format',
    settings: JSON.stringify({ style: 'gofmt' }),
    type: 'formatter',
  });
  {
    const tList = await toolListJSON({ role: ctx.TEST_ROLE_USER, limit: 50 });
    const hasLinter = tList.find(
      (x: any) => x && x.name === 'acme-linter' && x.title === 'Acme Linter',
    );
    const hasFmt = tList.find(
      (x: any) =>
        x && x.name === 'acme-formatter' && x.title === 'Acme Formatter',
    );
    await assertStep(
      'tools listed',
      Array.isArray(tList) && tList.length >= 2 && !!hasLinter && !!hasFmt,
      'tools list missing expected entries',
    );
    const t1 = await toolGetJSON({ name: 'acme-linter' });
    await assertStep(
      'tool get validated',
      t1 &&
        t1.name === 'acme-linter' &&
        t1.role === ctx.TEST_ROLE_USER &&
        t1.settings &&
        t1.settings.autofix === true,
      'tool get validation failed for acme-linter',
    );
  }

  ctx.nextStep('Prompt run via Ollama (gemma3:1b), if available');
  try {
    const OLLAMA_BASE_URL =
      process.env.OLLAMA_BASE_URL || 'http://127.0.0.1:11434';
    await toolSet({
      name: 'ollama-gemma',
      title: 'Ollama Gemma 1B',
      role: ctx.TEST_ROLE_USER,
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
    if (out) {
      assert(out.object === 'response', 'prompt: expected response object');
      assert(typeof out.model === 'string', 'prompt: model string');
      assert(Array.isArray(out.output), 'prompt: output array');
    }
  } catch (e) {
    console.error(
      'prompt (ollama) skipped:',
      (e as Error)?.message || String(e),
    );
  }

  ctx.nextStep('Prompt via Connect client (if server available)');
  try {
    await runRbc('server', 'start', '--detach');
    try {
      for (let i = 0; i < 50; i++) {
        try {
          const r = await fetch('http://127.0.0.1:53051/health');
          if (r?.ok) break;
        } catch {}
        await sleep(100);
      }
    } catch {}

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
      await runRbc('server', 'stop');
    } catch {}
  }
}
