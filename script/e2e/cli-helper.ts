import { createConnectGrpcJsonClient } from '../grpc-json-client-connect.mjs';

type RunOpts = {
  cwd?: string;
  env?: Record<string, string | undefined>;
  stdin?: string;
  allowFailure?: boolean;
};

type RunResult = {
  stdout: string;
  stderr: string;
  exitCode: number;
};

async function runCmd(cmd: string[], opts: RunOpts = {}): Promise<RunResult> {
  const proc = Bun.spawn(cmd, {
    cwd: opts.cwd,
    env: opts.env,
    stdin: opts.stdin !== undefined ? 'pipe' : 'ignore',
    stdout: 'pipe',
    stderr: 'pipe',
  });

  if (opts.stdin !== undefined) {
    const stdinAny = proc.stdin as any;
    if (typeof stdinAny?.write === 'function') {
      stdinAny.write(opts.stdin);
      if (typeof stdinAny?.end === 'function') stdinAny.end();
    } else if (typeof stdinAny?.getWriter === 'function') {
      const writer = stdinAny.getWriter();
      await writer.write(new TextEncoder().encode(opts.stdin));
      await writer.close();
    } else {
      throw new Error(
        `stdin piping not supported for command: ${cmd.join(' ')}`,
      );
    }
  }

  const [exitCode, stdout, stderr] = await Promise.all([
    proc.exited,
    new Response(proc.stdout).text(),
    new Response(proc.stderr).text(),
  ]);

  if (exitCode !== 0 && !opts.allowFailure) {
    const err = new Error(
      `Command failed (${exitCode}): ${cmd.join(' ')}`,
    ) as Error & {
      stdout?: string;
      stderr?: string;
      exitCode?: number;
    };
    err.stdout = stdout;
    err.stderr = stderr;
    err.exitCode = exitCode;
    throw err;
  }

  return { stdout, stderr, exitCode };
}

export async function runShell(
  script: string,
  opts: Omit<RunOpts, 'stdin'> & { stdin?: string } = {},
) {
  return await runCmd(['bash', '-lc', script], opts);
}

export async function runRbc(...args: string[]) {
  return await runCmd(['go', 'run', 'main.go', ...args]);
}

export async function runRbcJSON(...args: string[]) {
  const p = await runRbc(...args);
  try {
    return JSON.parse(p.stdout || 'null');
  } catch (err) {
    console.error('Failed to parse JSON from:', args.join(' '));
    console.error(p.stdout);
    throw err;
  }
}

export function idFrom(obj: unknown) {
  if (!obj || typeof obj !== 'object') return '';
  const o = obj as Record<string, unknown>;
  return String(o.id || o.ID || '');
}

export function logStep(i: number, total: number, msg: string) {
  console.log(`[${i}/${total}] ${msg}`);
}

export function assert(cond: unknown, msg: string) {
  if (!cond) {
    throw new Error(`Assertion failed: ${msg}`);
  }
}

function sleep(ms: number) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

let __assertExperimentId = '';
let __assertConnectEnabled = false;

async function ensureAssertExperiment() {
  if (__assertExperimentId) return __assertExperimentId;
  const conv = await conversationSet({
    title: 'Test-All Assert Steps',
    role: 'rbctest-user',
  });
  const exp = await experimentCreate({ conversation: idFrom(conv) });
  __assertExperimentId = idFrom(exp);
  return __assertExperimentId;
}

export async function assertStep(
  stepName: string,
  cond: unknown,
  msg = '',
  details = '',
) {
  const ok = !!cond;
  try {
    if (__assertConnectEnabled) {
      const client = createConnectGrpcJsonClient({
        baseUrl: 'http://127.0.0.1:53051',
      });
      const experiment = await ensureAssertExperiment();
      await client.testcase.Create({
        title: String(stepName || 'unnamed-step'),
        role: 'rbctest-user',
        experiment,
        status: ok ? 'OK' : 'KO',
        file: 'script/e2e/test-all.ts',
      });
      if (ok) return;
    }
  } catch {
    // fallback below
  }

  try {
    const experiment = await ensureAssertExperiment();
    await testcaseCreate({
      title: String(stepName || 'unnamed-step'),
      role: 'rbctest-user',
      experiment,
      status: ok ? 'OK' : 'KO',
      file: 'script/e2e/test-all.ts',
    });
  } catch (e2) {
    console.error(
      'assertStep: CLI fallback failed:',
      (e2 as Error)?.message || e2,
    );
  }

  if (!ok) {
    const detailStr = details
      ? `\nGot:\n${String(details).slice(0, 2000)}`
      : '';
    throw new Error((msg || `assertStep failed: ${stepName}`) + detailStr);
  }
}

export async function enableAssertConnect() {
  try {
    await runRbc('server', 'stop');
  } catch {}
  try {
    await runRbc('server', 'start', '--detach');
  } catch {}
  try {
    for (let i = 0; i < 50; i++) {
      try {
        const r = await fetch('http://127.0.0.1:53051/health');
        if (r?.ok) break;
      } catch {}
      await sleep(100);
    }
  } catch {}
  __assertConnectEnabled = true;
}

export async function runSetRole({
  name,
  title,
  description = '',
  notes = '',
}: {
  name: string;
  title: string;
  description?: string;
  notes?: string;
}) {
  return await runRbc(
    'role',
    'set',
    '--name',
    name,
    '--title',
    title,
    ...(description ? ['--description', description] : []),
    ...(notes ? ['--notes', notes] : []),
  );
}

export async function roleGetJSON({ name }: { name: string }) {
  return await runRbcJSON('role', 'get', '--name', name);
}

export async function roleListJSON({
  limit = 100,
  offset = 0,
}: {
  limit?: number;
  offset?: number;
} = {}) {
  return await runRbcJSON(
    'role',
    'list',
    '--output',
    'json',
    '--limit',
    String(limit),
    '--offset',
    String(offset),
  );
}

export async function runSetWorkflow({
  name,
  title,
  description = '',
  role = 'user',
  notes = '',
}: {
  name: string;
  title: string;
  description?: string;
  role?: string;
  notes?: string;
}) {
  return await runRbc(
    'workflow',
    'set',
    '--name',
    name,
    '--title',
    title,
    ...(description ? ['--description', description] : []),
    ...(notes ? ['--notes', notes] : []),
    '--role',
    role,
  );
}

export async function createScript(
  role: string,
  title: string,
  description: string,
  body: string,
  opts: { name?: string; variant?: string; archived?: boolean } = {},
) {
  const args = ['script', 'set', '--role', role, '--title', title];
  if (description) args.push('--description', description);
  if (opts.name !== undefined) args.push('--name', opts.name);
  if (opts.variant !== undefined) args.push('--variant', opts.variant);
  if (opts.archived) args.push('--archived');
  const out = await runCmd(['go', 'run', 'main.go', ...args], {
    stdin: body || '',
  });
  return JSON.parse(out.stdout).id;
}

export async function scriptListJSON({
  role,
  limit = 100,
  offset = 0,
}: {
  role: string;
  limit?: number;
  offset?: number;
}) {
  return await runRbcJSON(
    'script',
    'list',
    '--role',
    role,
    '--output',
    'json',
    '--limit',
    String(limit),
    '--offset',
    String(offset),
  );
}

export async function scriptFind({
  name,
  variant = '',
  archived = false,
  role = '',
}: {
  name: string;
  variant?: string;
  archived?: boolean;
  role?: string;
}) {
  const args = ['script', 'find', '--name', name, '--variant', variant];
  if (archived) args.push('--archived');
  if (role) args.push('--role', role);
  return await runRbcJSON(...args);
}

export async function runSetTask({
  workflow,
  command,
  variant = '',
  role = 'user',
  title = '',
  description = '',
  shell = '',
  timeout = '',
  tags = '',
  level = '',
}: {
  workflow: string;
  command: string;
  variant?: string;
  role?: string;
  title?: string;
  description?: string;
  shell?: string;
  timeout?: string;
  tags?: string;
  level?: string;
}) {
  const args = [
    'task',
    'set',
    '--workflow',
    workflow,
    '--command',
    command,
    '--variant',
    variant,
    '--role',
    role,
  ];
  if (title) args.push('--title', title);
  if (description) args.push('--description', description);
  if (shell) args.push('--shell', shell);
  if (timeout) args.push('--timeout', timeout);
  if (tags) args.push('--tags', tags);
  if (level) args.push('--level', level);
  return await runRbcJSON(...args);
}

export async function taskScriptAdd({
  task,
  script,
  name,
  alias = '',
}: {
  task: string;
  script: string;
  name: string;
  alias?: string;
}) {
  const args = [
    'task',
    'script-add',
    '--task',
    task,
    '--script',
    script,
    '--name',
    name,
  ];
  if (alias) args.push('--alias', alias);
  return await runRbc(...args);
}

export async function blackboardSet({
  role = 'user',
  project = '',
  background = '',
  guidelines = '',
  lifecycle = '',
}: {
  role?: string;
  project?: string;
  background?: string;
  guidelines?: string;
  lifecycle?: string;
}) {
  const args = ['blackboard', 'set', '--role', role];
  if (project) args.push('--project', project);
  if (background) args.push('--background', background);
  if (guidelines) args.push('--guidelines', guidelines);
  if (lifecycle) args.push('--lifecycle', lifecycle);
  return await runRbcJSON(...args);
}

export async function conversationSet({
  title,
  role = 'user',
  description = '',
  project = '',
  tags = '',
  notes = '',
}: {
  title: string;
  role?: string;
  description?: string;
  project?: string;
  tags?: string;
  notes?: string;
}) {
  const args = ['conversation', 'set', '--title', title, '--role', role];
  if (description) args.push('--description', description);
  if (project) args.push('--project', project);
  if (tags) args.push('--tags', tags);
  if (notes) args.push('--notes', notes);
  return await runRbcJSON(...args);
}

export async function experimentCreate({
  conversation,
}: {
  conversation: string;
}) {
  return await runRbcJSON(
    'experiment',
    'create',
    '--conversation',
    conversation,
  );
}

export async function queueAdd({
  description,
  status = '',
  why = '',
  tags = '',
}: {
  description: string;
  status?: string;
  why?: string;
  tags?: string;
}) {
  const args = ['queue', 'add', '--description', description];
  if (status) args.push('--status', status);
  if (why) args.push('--why', why);
  if (tags) args.push('--tags', tags);
  return await runRbcJSON(...args);
}

export async function stickieSet({
  id = '',
  blackboard = '',
  topicName = '',
  topicRole = '',
  note = '',
  code = '',
  labels = [],
  createdByTask = '',
  name = '',
  archived = false,
  score = null,
  priority: _priority = '',
}: {
  id?: string;
  blackboard?: string;
  topicName?: string;
  topicRole?: string;
  note?: string;
  code?: string;
  labels?: string[];
  createdByTask?: string;
  name?: string;
  archived?: boolean;
  score?: number | null;
  priority?: string;
}) {
  const args = ['stickie', 'set'];
  if (id) args.push('--id', id);
  if (blackboard) args.push('--blackboard', blackboard);
  if (topicName) args.push('--topic-name', topicName);
  if (topicRole) args.push('--topic-role', topicRole);
  if (note) args.push('--note', note);
  if (code) args.push('--code', code);
  if (labels?.length) args.push('--labels', labels.join(','));
  if (createdByTask) args.push('--created-by-task', createdByTask);
  if (name !== undefined) args.push('--name', name);
  if (archived) args.push('--archived');
  if (score !== null && score !== undefined)
    args.push('--score', String(score));
  return await runRbcJSON(...args);
}

export async function stickieListJSON({
  blackboard = '',
  limit = 100,
  offset = 0,
}: {
  blackboard?: string;
  limit?: number;
  offset?: number;
}) {
  const args = ['stickie', 'list', '--output', 'json'];
  if (blackboard) args.push('--blackboard', blackboard);
  args.push('--limit', String(limit), '--offset', String(offset));
  return await runRbcJSON(...args);
}

export async function stickieGetJSON({ id }: { id: string }) {
  return await runRbcJSON('stickie', 'get', '--id', id);
}

export async function workflowListJSON({
  role,
  limit = 100,
  offset = 0,
}: {
  role: string;
  limit?: number;
  offset?: number;
}) {
  return await runRbcJSON(
    'workflow',
    'list',
    '--role',
    role,
    '--output',
    'json',
    '--limit',
    String(limit),
    '--offset',
    String(offset),
  );
}

export async function taskListJSON({
  role,
  workflow = '',
  limit = 100,
  offset = 0,
}: {
  role: string;
  workflow?: string;
  limit?: number;
  offset?: number;
}) {
  const args = [
    'task',
    'list',
    '--role',
    role,
    '--output',
    'json',
    '--limit',
    String(limit),
    '--offset',
    String(offset),
  ];
  if (workflow) args.push('--workflow', workflow);
  return await runRbcJSON(...args);
}

export async function stickieFind({
  name,
  archived = false,
  blackboard = '',
}: {
  name: string;
  archived?: boolean;
  blackboard?: string;
}) {
  const args = ['stickie', 'find', '--name', name];
  if (archived) args.push('--archived');
  if (blackboard) args.push('--blackboard', blackboard);
  return await runRbcJSON(...args);
}

export async function messageSet({
  text = '',
  experiment = '',
  title = '',
  tags = '',
  role = 'user',
}: {
  text?: string;
  experiment?: string;
  title?: string;
  tags?: string;
  role?: string;
}) {
  const args = ['message', 'set'];
  if (experiment) args.push('--experiment', experiment);
  if (title) args.push('--title', title);
  if (tags) args.push('--tags', tags);
  if (role) args.push('--role', role);
  return await runCmd(['go', 'run', 'main.go', ...args], { stdin: text || '' });
}

export async function stickieRelSet({
  from,
  to,
  type,
  labels = '',
}: {
  from: string;
  to: string;
  type: string;
  labels?: string;
}) {
  const args = [
    'stickie-rel',
    'set',
    '--from',
    from,
    '--to',
    to,
    '--type',
    type,
  ];
  if (labels) args.push('--labels', labels);
  return await runRbc(...args);
}

export async function stickieRelList({
  id,
  direction = 'out',
}: {
  id: string;
  direction?: string;
}) {
  return await runRbc(
    'stickie-rel',
    'list',
    '--id',
    id,
    '--direction',
    direction,
  );
}

export async function stickieRelGet({
  from,
  to,
  type,
  ignoreMissing = false,
}: {
  from: string;
  to: string;
  type: string;
  ignoreMissing?: boolean;
}) {
  const args = [
    'stickie-rel',
    'get',
    '--from',
    from,
    '--to',
    to,
    '--type',
    type,
  ];
  if (ignoreMissing) args.push('--ignore-missing');
  return await runRbc(...args);
}

export async function dbReset({
  dropAppRole = false,
}: {
  dropAppRole?: boolean;
} = {}) {
  return await runRbc(
    'db',
    'reset',
    '--force',
    `--drop-app-role=${dropAppRole ? 'true' : 'false'}`,
  );
}

export async function dbScaffoldAll() {
  return await runRbc('db', 'scaffold', '--all', '--yes');
}

export async function taskSetReplacement({
  workflow,
  command,
  variant = '',
  role = 'user',
  title = '',
  description = '',
  shell = '',
  replaces = '',
  replaceLevel = '',
  replaceComment = '',
}: {
  workflow: string;
  command: string;
  variant?: string;
  role?: string;
  title?: string;
  description?: string;
  shell?: string;
  replaces?: string;
  replaceLevel?: string;
  replaceComment?: string;
}) {
  const args = [
    'task',
    'set',
    '--workflow',
    workflow,
    '--command',
    command,
    '--variant',
    variant,
    '--role',
    role,
  ];
  if (title) args.push('--title', title);
  if (description) args.push('--description', description);
  if (shell) args.push('--shell', shell);
  if (replaces) args.push('--replaces', replaces);
  if (replaceLevel) args.push('--replace-level', replaceLevel);
  if (replaceComment) args.push('--replace-comment', replaceComment);
  return await runRbc(...args);
}

export async function tagSet({
  name,
  title,
  role = 'user',
}: {
  name: string;
  title: string;
  role?: string;
}) {
  return await runRbc(
    'tag',
    'set',
    '--name',
    name,
    '--title',
    title,
    '--role',
    role,
  );
}

export async function projectSet({
  name,
  role = 'user',
  description = '',
  notes = '',
  tags = '',
}: {
  name: string;
  role?: string;
  description?: string;
  notes?: string;
  tags?: string;
}) {
  const args = ['project', 'set', '--name', name, '--role', role];
  if (description) args.push('--description', description);
  if (notes) args.push('--notes', notes);
  if (tags) args.push('--tags', tags);
  return await runRbc(...args);
}

export async function toolSet({
  name,
  title,
  role = 'user',
  description = '',
  notes = '',
  tags = '',
  settings = '',
  type = '',
}: {
  name: string;
  title: string;
  role?: string;
  description?: string;
  notes?: string;
  tags?: string;
  settings?: string;
  type?: string;
}) {
  const args = [
    'tool',
    'set',
    '--name',
    name,
    '--title',
    title,
    '--role',
    role,
  ];
  if (description) args.push('--description', description);
  if (notes) args.push('--notes', notes);
  if (tags) args.push('--tags', tags);
  if (settings) args.push('--settings', settings);
  if (type) args.push('--type', type);
  return await runRbc(...args);
}

export async function toolGetJSON({ name }: { name: string }) {
  return await runRbcJSON('tool', 'get', '--name', name);
}

export async function toolListJSON({
  role,
  limit = 100,
  offset = 0,
}: {
  role: string;
  limit?: number;
  offset?: number;
}) {
  return await runRbcJSON(
    'tool',
    'list',
    '--role',
    role,
    '--output',
    'json',
    '--limit',
    String(limit),
    '--offset',
    String(offset),
  );
}

export async function workspaceSet({
  role = 'user',
  project = '',
  description = '',
  tags = '',
}: {
  role?: string;
  project?: string;
  description?: string;
  tags?: string;
}) {
  const args = ['workspace', 'set', '--role', role];
  if (project) args.push('--project', project);
  if (description) args.push('--description', description);
  if (tags) args.push('--tags', tags);
  return await runRbc(...args);
}

export async function packageSet({
  role = 'user',
  variant,
}: {
  role?: string;
  variant: string;
}) {
  return await runRbc('package', 'set', '--role', role, '--variant', variant);
}

export async function queuePeek({ limit = 2 }: { limit?: number } = {}) {
  return await runRbc('queue', 'peek', '--limit', String(limit));
}

export async function queueSize() {
  return await runRbc('queue', 'size');
}

export async function queueTake({ id }: { id: string }) {
  return await runRbc('queue', 'take', '--id', id);
}

export async function listWithRole(cmd: string, role: string, limit = 50) {
  return await runRbc(cmd, 'list', '--role', role, '--limit', String(limit));
}

export async function experimentList(limit = 50) {
  return await runRbc('experiment', 'list', '--limit', String(limit));
}

export async function stickieList(limit = 50) {
  return await runRbc('stickie', 'list', '--limit', String(limit));
}

export async function stickieListByBlackboard({
  blackboard,
  limit = 50,
}: {
  blackboard: string;
  limit?: number;
}) {
  return await runRbc(
    'stickie',
    'list',
    '--blackboard',
    blackboard,
    '--limit',
    String(limit),
  );
}

export async function dbCountPerRole() {
  return await runRbc('db', 'count', '--per-role');
}

export async function dbCountJSON() {
  return await runRbc('db', 'count', '--json');
}

export async function snapshotBackupJSON({
  description,
  who,
}: {
  description: string;
  who: string;
}) {
  return await runRbcJSON(
    'snapshot',
    'backup',
    '--description',
    description,
    '--who',
    who,
    '--json',
  );
}

export async function snapshotList({ limit = 5 }: { limit?: number } = {}) {
  return await runRbc('snapshot', 'list', '--limit', String(limit));
}

export async function snapshotShow({ id }: { id: string }) {
  return await runRbc('snapshot', 'show', id);
}

export async function snapshotRestoreDry({
  id,
  mode = 'append',
}: {
  id: string;
  mode?: string;
}) {
  return await runRbc('snapshot', 'restore', id, '--mode', mode, '--dry-run');
}

export async function snapshotDelete({ id }: { id: string }) {
  return await runRbc('snapshot', 'delete', id, '--force');
}

export async function snapshotVerifyJSON({
  id,
  schema = 'backup',
}: {
  id: string;
  schema?: string;
}) {
  return await runRbcJSON(
    'snapshot',
    'verify',
    id,
    '--schema',
    schema,
    '--json',
  );
}

export async function snapshotPrunePreviewJSON({
  olderThan = '90d',
  schema = 'backup',
}: {
  olderThan?: string;
  schema?: string;
}) {
  return await runRbcJSON(
    'snapshot',
    'prune',
    '--older-than',
    olderThan,
    '--schema',
    schema,
    '--json',
  );
}

export async function projectListJSON({
  role,
  limit = 100,
  offset = 0,
}: {
  role: string;
  limit?: number;
  offset?: number;
}) {
  return await runRbcJSON(
    'project',
    'list',
    '--role',
    role,
    '--output',
    'json',
    '--limit',
    String(limit),
    '--offset',
    String(offset),
  );
}

export async function blackboardListJSON({
  role,
  limit = 100,
  offset = 0,
}: {
  role: string;
  limit?: number;
  offset?: number;
}) {
  return await runRbcJSON(
    'blackboard',
    'list',
    '--role',
    role,
    '--output',
    'json',
    '--limit',
    String(limit),
    '--offset',
    String(offset),
  );
}

export async function conversationListJSON({
  role,
  project = '',
  limit = 100,
  offset = 0,
}: {
  role: string;
  project?: string;
  limit?: number;
  offset?: number;
}) {
  const args = [
    'conversation',
    'list',
    '--role',
    role,
    '--output',
    'json',
    '--limit',
    String(limit),
    '--offset',
    String(offset),
  ];
  if (project) args.push('--project', project);
  return await runRbcJSON(...args);
}

export async function conversationGetJSON({ id }: { id: string }) {
  return await runRbcJSON('conversation', 'get', '--id', id);
}

export async function projectGetJSON({
  name,
  role,
}: {
  name: string;
  role: string;
}) {
  return await runRbcJSON('project', 'get', '--name', name, '--role', role);
}

export async function messageListJSON({
  role,
  experiment = '',
  task = '',
  status = '',
  limit = 100,
  offset = 0,
}: {
  role: string;
  experiment?: string;
  task?: string;
  status?: string;
  limit?: number;
  offset?: number;
}) {
  const args = [
    'message',
    'list',
    '--role',
    role,
    '--output',
    'json',
    '--limit',
    String(limit),
    '--offset',
    String(offset),
  ];
  if (experiment) args.push('--experiment', experiment);
  if (task) args.push('--task', task);
  if (status) args.push('--status', status);
  return await runRbcJSON(...args);
}

export async function vaultList() {
  const p = await runRbc('vault', 'list');
  const lines = (p.stdout || '')
    .split('\n')
    .map((l) => l.trim())
    .filter(Boolean);
  const items: Array<{ name: string; status: string; backend: string }> = [];
  for (const line of lines) {
    const parts = line.split('\t');
    if (parts.length >= 3) {
      const name = parts[0].trim();
      const statusRaw = parts[1].trim();
      const status = statusRaw.replace(/\[|\]/g, '');
      const backend = parts[2].trim();
      items.push({ name, status, backend });
    }
  }
  return items;
}

export async function vaultShow(name: string) {
  const p = await runRbc('vault', 'show', name);
  const out = p.stdout || '';
  const obj = { name: '', status: '', backend: '', updated: '' };
  for (const line of out.split('\n')) {
    const m = line.match(/^([^:]+):\s*(.*)$/);
    if (!m) continue;
    const k = m[1].trim().toLowerCase();
    const v = m[2].trim();
    if (k === 'name') obj.name = v;
    else if (k === 'status') obj.status = v;
    else if (k === 'backend') obj.backend = v;
    else if (k.startsWith('last updated')) obj.updated = v;
  }
  return obj;
}

export async function vaultBackendCurrent() {
  const p = await runRbc('vault', 'backend', 'current');
  return String(p.stdout || '').trim();
}

export async function vaultDoctor() {
  const p = await runRbc('vault', 'doctor');
  return { stdout: String(p.stdout || ''), stderr: String(p.stderr || '') };
}

export async function testcaseCreate({
  title,
  role = 'user',
  experiment = '',
  status = 'OK',
  name = '',
  pkg = '',
  classname = '',
  error = '',
  tags = '',
  level = '',
  file = '',
  line = 0,
  executionTime = 0,
}: {
  title: string;
  role?: string;
  experiment?: string;
  status?: string;
  name?: string;
  pkg?: string;
  classname?: string;
  error?: string;
  tags?: string;
  level?: string;
  file?: string;
  line?: number;
  executionTime?: number;
}) {
  const args = [
    'testcase',
    'create',
    '--title',
    title,
    '--role',
    role,
    '--status',
    status,
  ];
  if (experiment) args.push('--experiment', experiment);
  if (name) args.push('--name', name);
  if (pkg) args.push('--package', pkg);
  if (classname) args.push('--classname', classname);
  if (error) args.push('--error', error);
  if (tags) args.push('--tags', tags);
  if (level) args.push('--level', level);
  if (file) args.push('--file', file);
  if (line) args.push('--line', String(line));
  if (executionTime) args.push('--execution-time', String(executionTime));
  return await runRbcJSON(...args);
}

export async function testcaseListJSON({
  role,
  experiment = '',
  status = '',
  limit = 100,
  offset = 0,
}: {
  role: string;
  experiment?: string;
  status?: string;
  limit?: number;
  offset?: number;
}) {
  const args = [
    'testcase',
    'list',
    '--role',
    role,
    '--output',
    'json',
    '--limit',
    String(limit),
    '--offset',
    String(offset),
  ];
  if (experiment) args.push('--experiment', experiment);
  if (status) args.push('--status', status);
  return await runRbcJSON(...args);
}

export async function promptRunJSON(opts: {
  toolName: string;
  input?: string;
  inputFile?: string;
  toolsPath?: string;
  temperature?: number;
  maxOutputTokens?: number;
}) {
  const args: string[] = [];
  if (!opts.toolName) throw new Error('toolName is required');
  args.push('--tool-name', opts.toolName);
  if (opts.input) args.push('--input', opts.input);
  if (opts.inputFile) args.push('--input-file', opts.inputFile);
  if (opts.toolsPath) args.push('--tools', opts.toolsPath);
  if (typeof opts.temperature === 'number')
    args.push('--temperature', String(opts.temperature));
  if (typeof opts.maxOutputTokens === 'number')
    args.push('--max-output-tokens', String(opts.maxOutputTokens));
  args.push('--json');
  return await runRbcJSON('prompt', 'run', ...args);
}
