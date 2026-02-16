import {
  assert,
  blackboardListJSON,
  blackboardSet,
  idFrom,
  runShell,
  stickieGetJSON,
  stickieRelSet,
  stickieSet,
} from './cli-helper';
import { validateBlackboardListContract } from './contract-helper';
import type { E2EContext } from './types';

export async function runBlackboardStickie(ctx: E2EContext) {
  ctx.nextStep('Creating blackboards');
  ctx.state.bb1 = idFrom(
    await blackboardSet({
      role: ctx.TEST_ROLE_USER,
      project: 'acme/build-system',
      background: 'Ideas board for build system',
      guidelines: 'Keep concise; tag items with priority',
      lifecycle: 'monthly',
    }),
  );
  ctx.state.bb2 = idFrom(
    await blackboardSet({
      role: ctx.TEST_ROLE_USER,
      background: 'Team-wide blackboard',
      guidelines: 'Wipe weekly on Mondays',
      lifecycle: 'weekly',
    }),
  );
  {
    const bbs = await blackboardListJSON({
      role: ctx.TEST_ROLE_USER,
      limit: 50,
    });
    validateBlackboardListContract(bbs);
  }

  ctx.nextStep('Creating blackboard via --cli-input-yaml');
  try {
    await runShell('mkdir -p temp');
  } catch {}
  await runShell(
    `cat > temp/blackboard-input.yaml <<EOF\nrole: ${ctx.TEST_ROLE_USER}\nproject: acme/complete\nbackground: Created via YAML\nguidelines: From YAML\nlifecycle: weekly\nEOF`,
  );
  const bbYamlOut = await runShell(
    'cat temp/blackboard-input.yaml | go run main.go blackboard set --cli-input-yaml',
  );
  {
    const meta = JSON.parse(String(bbYamlOut.stdout || 'null'));
    assert(
      !!meta &&
        !!meta.id &&
        meta.role === ctx.TEST_ROLE_USER &&
        meta.lifecycle === 'weekly',
      'expected blackboard to be created via YAML with matching role and lifecycle',
    );
  }

  ctx.nextStep('Creating stickies and relations');
  ctx.state.st1 = idFrom(
    await stickieSet({
      blackboard: ctx.state.bb1,
      note: 'Refresh onboarding guide for new hires',
      labels: ['onboarding', 'docs', 'priority:med'],
      priority: 'should',
      name: 'Onboarding Refresh',
      score: 0.42,
    }),
  );
  ctx.state.st2 = idFrom(
    await stickieSet({
      blackboard: ctx.state.bb1,
      note: 'Evaluate GitHub Actions caching for go build',
      code: 'name: CI\n\non: [push]\n\njobs:\n  build:\n    runs-on: ubuntu-latest\n    steps:\n      - uses: actions/checkout@v4\n      - uses: actions/setup-go@v5\n      - run: go build ./...\n',
      labels: ['idea', 'devops'],
      priority: 'could',
      name: 'DevOps Caching',
    }),
  );
  ctx.state.st3 = idFrom(
    await stickieSet({
      blackboard: ctx.state.bb2,
      note: 'Team retro every Friday',
      code: 'package main\n\nimport "fmt"\n\nfunc main() {\n  fmt.Println("Hello, RBCTest!")\n}\n',
      labels: ['team', 'ritual'],
      priority: 'must',
      name: 'Team Retro',
    }),
  );

  await stickieRelSet({
    from: ctx.state.st1 || '',
    to: ctx.state.st2 || '',
    type: 'uses',
    labels: 'ref,dependency',
  });
  {
    const g1 = await stickieGetJSON({ id: ctx.state.st1 || '' });
    assert(
      typeof g1.score === 'number' && Math.abs(g1.score - 0.42) < 1e-9,
      'stickie st1 score should be 0.42 after create',
    );
    await stickieSet({ id: ctx.state.st2, score: 0.99 });
    const g2 = await stickieGetJSON({ id: ctx.state.st2 || '' });
    assert(
      typeof g2.score === 'number' && Math.abs(g2.score - 0.99) < 1e-9,
      'stickie st2 score should be 0.99 after update',
    );
  }
  await stickieRelSet({
    from: ctx.state.st2 || '',
    to: ctx.state.st3 || '',
    type: 'includes',
    labels: 'backlog',
  });
  await stickieRelSet({
    from: ctx.state.st1 || '',
    to: ctx.state.st3 || '',
    type: 'contrasts_with',
    labels: 'tradeoff',
  });
}
