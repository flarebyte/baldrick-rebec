import type { E2EContext } from './types';
import {
  assertStep,
  blackboardListJSON,
  dbCountJSON,
  dbCountPerRole,
  experimentList,
  listWithRole,
  runRbc,
  runShell,
  stickieFind,
  stickieGetJSON,
  stickieList,
  stickieListByBlackboard,
  stickieListJSON,
  stickieRelGet,
  stickieRelList,
} from './cli-helper';
import { validateStickieListContract } from './contract-helper';

export async function runListingSyncImport(ctx: E2EContext) {
  const bb1 = ctx.state.bb1 || '';
  const st1 = ctx.state.st1 || '';
  const st2 = ctx.state.st2 || '';

  ctx.nextStep('Listing entities and counts (per-role and JSON)');
  await listWithRole('workflow', ctx.TEST_ROLE_USER, 50);
  await listWithRole('task', ctx.TEST_ROLE_USER, 50);
  await listWithRole('conversation', ctx.TEST_ROLE_USER, 50);
  await experimentList(50);
  await listWithRole('message', ctx.TEST_ROLE_USER, 50);
  await listWithRole('project', ctx.TEST_ROLE_USER, 50);
  await listWithRole('workspace', ctx.TEST_ROLE_USER, 50);
  await listWithRole('script', ctx.TEST_ROLE_USER, 50);
  await listWithRole('blackboard', ctx.TEST_ROLE_USER, 50);
  await stickieList(50);
  await stickieListByBlackboard({ blackboard: bb1, limit: 50 });

  {
    const stList = await stickieListJSON({ blackboard: bb1 });
    validateStickieListContract(stList);
    const byId = (id: string) => (stList || []).find((x: any) => x && (x.id === id || x.ID === id));
    const s1json = byId(st1);
    const s2json = byId(st2);
    const f1 = await stickieFind({ name: 'Onboarding Refresh', blackboard: bb1 });
    await assertStep(
      'stickies validated',
      s1json && s1json.name === 'Onboarding Refresh' && s2json && s2json.name === 'DevOps Caching' && f1 && f1.id === st1,
      'stickies list/find validation failed',
    );
    await assertStep(
      'stickie list includes note/code',
      typeof s2json?.note === 'string' && s2json.note.length > 0 && typeof s2json?.code === 'string' && s2json.code.length > 0,
      'expected stickie list json to include note and code',
    );
  }

  ctx.nextStep('Exporting blackboard to temp/blackboard-test');
  try {
    await runShell('rm -rf temp/blackboard-test');
  } catch {}
  await runRbc('blackboard', 'sync', `id:${bb1}`, 'folder:temp/blackboard-test');
  try {
    await runRbc('blackboard', 'sync', 'id:_', 'folder:temp/blackboard-test', '--dry-run');
    await assertStep('sync id:_ (id->folder) ok', true);
  } catch {
    await assertStep('sync id:_ (id->folder) ok', false, 'expected id:_ shortcut to resolve from folder');
  }

  const bbYaml = await runShell('test -f temp/blackboard-test/blackboard.yaml && echo OK || echo MISSING');
  const st1Yaml = await runShell('test -f temp/blackboard-test/about-onboarding-refresh.stickie.yaml && echo OK || echo MISSING');
  const st2Yaml = await runShell('test -f temp/blackboard-test/about-devops-caching.stickie.yaml && echo OK || echo MISSING');
  await assertStep(
    'blackboard synced to folder',
    String(bbYaml.stdout || '').includes('OK') && String(st1Yaml.stdout || '').includes('OK') && String(st2Yaml.stdout || '').includes('OK'),
    'expected exported YAML files missing for blackboard/stickies',
  );

  ctx.nextStep('Diff: unchanged right after export');
  const diffUnchanged = await runRbc('blackboard', 'diff', `id:${bb1}`, 'folder:temp/blackboard-test');
  await assertStep(
    'diff unchanged concise',
    String(diffUnchanged.stdout || '').includes('= blackboard id=') && String(diffUnchanged.stdout || '').includes('= stickie id='),
    'expected concise diff to show unchanged blackboard and at least one stickie',
    diffUnchanged.stdout || diffUnchanged.stderr || '',
  );
  const diffUnchangedAlias = await runRbc('blackboard', 'diff', 'id:_', 'folder:temp/blackboard-test');
  await assertStep(
    'diff id:_ shortcut works',
    String(diffUnchangedAlias.stdout || '').includes('= blackboard id=') && String(diffUnchangedAlias.stdout || '').includes('= stickie id='),
    'expected diff id:_ to resolve id from folder',
    diffUnchangedAlias.stdout || diffUnchangedAlias.stderr || '',
  );

  ctx.nextStep('Exporting blackboard with --clear-ids');
  try {
    await runShell('rm -rf temp/blackboard-noids');
  } catch {}
  await runRbc('blackboard', 'sync', `id:${bb1}`, 'folder:temp/blackboard-noids', '--clear-ids');
  const st1NoId = await runShell('grep -q "^id:" temp/blackboard-noids/about-onboarding-refresh.stickie.yaml && echo HAS_ID || echo NO_ID');
  const st2NoId = await runShell('grep -q "^id:" temp/blackboard-noids/about-devops-caching.stickie.yaml && echo HAS_ID || echo NO_ID');
  await assertStep(
    'clear-ids omitted id field',
    String(st1NoId.stdout || '').includes('NO_ID') && String(st2NoId.stdout || '').includes('NO_ID'),
    'expected no id field in stickie YAML when using --clear-ids',
  );

  ctx.nextStep('Folder->ID: updating existing stickie (by id)');
  const s1before = await stickieGetJSON({ id: st1 });
  const prevUpdated = s1before?.updated ? String(s1before.updated) : '';
  await runShell(`cat > temp/blackboard-test/about-onboarding-refresh.stickie.yaml <<EOF\nid: ${st1}\nnote: Updated via folder->id sync\nEOF`);

  ctx.nextStep('Diff: detect local change before import');
  const diffChanged = await runRbc('blackboard', 'diff', `id:${bb1}`, 'folder:temp/blackboard-test');
  await assertStep(
    'diff detects stickie change (concise)',
    String(diffChanged.stdout || '').includes(`~ stickie id=${st1}`),
    'expected diff to show changed stickie for st1',
    diffChanged.stdout || diffChanged.stderr || '',
  );
  const diffChangedDet = await runRbc('blackboard', 'diff', `id:${bb1}`, 'folder:temp/blackboard-test', '--detailed');
  await assertStep(
    'diff detailed shows field info',
    String(diffChangedDet.stdout || '').includes(`~ stickie id=${st1}`) && String(diffChangedDet.stdout || '').includes('note['),
    'expected detailed diff to include note[...] details',
    diffChangedDet.stdout || diffChangedDet.stderr || '',
  );

  await runRbc('blackboard', 'sync', 'folder:temp/blackboard-test', `id:${bb1}`);
  try {
    await runRbc('blackboard', 'sync', 'folder:temp/blackboard-test', 'id:_', '--dry-run');
    await assertStep('sync id:_ (folder->id) ok', true);
  } catch {
    await assertStep('sync id:_ (folder->id) ok', false, 'expected id:_ shortcut to resolve from folder');
  }
  {
    const s1after = await stickieGetJSON({ id: st1 });
    await assertStep(
      'folder->id updated existing stickie',
      s1after && s1after.id === st1 && s1after.note === 'Updated via folder->id sync' && (prevUpdated ? String(s1after.updated) !== prevUpdated : true),
      'expected note to be updated and timestamp changed',
    );
  }

  ctx.nextStep('Folder->ID: creating new stickie (no id)');
  await runShell('cat > temp/blackboard-test/new-sync.stickie.yaml <<EOF\nname: Created by folder sync\nnote: This was created via folder->id\nlabels: [sync,created]\nEOF');
  await runRbc('blackboard', 'sync', 'folder:temp/blackboard-test', `id:${bb1}`);
  {
    const created = await stickieFind({ name: 'Created by folder sync', blackboard: bb1 });
    await assertStep('folder->id created new stickie', created?.id && created.blackboard_id === bb1, 'expected new stickie to be created with assigned UUID');
  }

  ctx.nextStep('Folder->ID: security guard on foreign/nonexistent id');
  await runShell('cat > temp/blackboard-test/bad.stickie.yaml <<EOF\nid: 123e4567-e89b-12d3-a456-426614174000\nnote: Should fail due to foreign id\nEOF');
  let failed = false;
  try {
    await runShell(`go run main.go blackboard sync folder:temp/blackboard-test id:${bb1} 2>/dev/null`);
  } catch {
    failed = true;
  }
  await assertStep(
    'folder->id rejects unknown/foreign id',
    failed,
    'expected sync to fail when a .stickie.yaml contains an id not on destination blackboard',
  );
  try {
    await runShell('rm -f temp/blackboard-test/bad.stickie.yaml');
  } catch {}

  await stickieRelList({ id: st1, direction: 'out' });
  await stickieRelGet({ from: st1, to: st2, type: 'uses', ignoreMissing: true });
  await listWithRole('tag', ctx.TEST_ROLE_USER, 50);
  await dbCountPerRole();
  await dbCountJSON();

  ctx.nextStep('Import: creating new blackboard from folder with preserved IDs');
  try {
    await runShell('rm -rf temp/blackboard-import');
  } catch {}
  await runShell('mkdir -p temp/blackboard-import');
  await runShell('cp temp/blackboard-test/blackboard.yaml temp/blackboard-import/blackboard.yaml');
  await runShell('cp temp/blackboard-test/*.stickie.yaml temp/blackboard-import/');
  await runShell('for f in temp/blackboard-import/*.stickie.yaml; do grep -q "^id:" "$f" || rm -f "$f"; done');

  const BB_IMPORT = String((await runShell('uuidgen | tr "[:upper:]" "[:lower:]"')).stdout || '').trim();
  const ST_IMPORT_1 = String((await runShell('uuidgen | tr "[:upper:]" "[:lower:]"')).stdout || '').trim();
  const ST_IMPORT_2 = String((await runShell('uuidgen | tr "[:upper:]" "[:lower:]"')).stdout || '').trim();

  await runShell(`sed -i'' -e 's/^id:.*/id: ${BB_IMPORT}/' temp/blackboard-import/blackboard.yaml`);
  await runShell(`sed -i'' -e 's/^id:.*/id: ${ST_IMPORT_1}/' temp/blackboard-import/about-onboarding-refresh.stickie.yaml`);
  await runShell(`sed -i'' -e 's/^id:.*/id: ${ST_IMPORT_2}/' temp/blackboard-import/about-devops-caching.stickie.yaml`);

  await runRbc('blackboard', 'import', 'temp/blackboard-import', '--detailed');
  {
    const bbl = await blackboardListJSON({ role: ctx.TEST_ROLE_USER, limit: 200 });
    const got = (bbl || []).find((b: any) => b && (b.id === BB_IMPORT || b.ID === BB_IMPORT));
    await assertStep('import: blackboard inserted', !!got, 'expected imported blackboard in list');

    const lst = await stickieListJSON({ blackboard: BB_IMPORT });
    const has1 = (lst || []).find((x: any) => x && (x.id === ST_IMPORT_1 || x.ID === ST_IMPORT_1));
    const has2 = (lst || []).find((x: any) => x && (x.id === ST_IMPORT_2 || x.ID === ST_IMPORT_2));
    await assertStep('import: stickies inserted', !!has1 && !!has2, 'expected both imported stickies');
  }

  {
    let failedDup = false;
    let details = '';
    try {
      await runRbc('blackboard', 'import', 'temp/blackboard-import');
    } catch (e) {
      failedDup = true;
      details = (e as any)?.stderr || (e as any)?.stdout || String(e || '');
    }
    await assertStep('import: duplicates rejected', failedDup, 'expected import to fail when ids already exist', details);
  }
}
