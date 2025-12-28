// Common CLI helpers for ZX-based admin scripts
// Note: Keep ZX idioms only; do not import fs/path. ZX globals ($, argv) are provided by the zx runner.

// Core runners
export async function runRbc(...args) {
  return await $`go run main.go ${args}`;
}

export async function runRbcJSON(...args) {
  const p = await runRbc(...args);
  try {
    return JSON.parse(p.stdout || 'null');
  } catch (err) {
    console.error('Failed to parse JSON from:', args.join(' '));
    console.error(p.stdout);
    throw err;
  }
}

// Small utilities
export function idFrom(obj) {
  if (!obj || typeof obj !== 'object') return '';
  return obj.id || obj.ID || '';
}

export function logStep(i, total, msg) {
  console.error(`[${i}/${total}] ${msg}`);
}

export function assert(cond, msg) {
  if (!cond) {
    throw new Error(`Assertion failed: ${msg}`);
  }
}

// Command helpers
export async function runSetRole({
  name,
  title,
  description = '',
  notes = '',
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

export async function roleGetJSON({ name }) {
  return await runRbcJSON('role', 'get', '--name', name);
}

export async function roleListJSON({ limit = 100, offset = 0 } = {}) {
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

export async function createScript(role, title, description, body, opts = {}) {
  const args = ['script', 'set', '--role', role, '--title', title];
  if (description) args.push('--description', description);
  if (opts.name !== undefined) args.push('--name', opts.name);
  if (opts.variant !== undefined) args.push('--variant', opts.variant);
  if (opts.archived) args.push('--archived');
  const proc = $`go run main.go ${args}`;
  proc.stdin.write(body || '');
  proc.stdin.end();
  const out = await proc;
  return JSON.parse(out.stdout).id;
}

export async function scriptListJSON({ role, limit = 100, offset = 0 }) {
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

export async function taskScriptAdd({ task, script, name, alias = '' }) {
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

export async function storeGet({ name, role = 'user' }) {
  return await runRbcJSON('store', 'get', '--name', name, '--role', role);
}

export async function blackboardSet({
  role = 'user',
  storeId,
  project = '',
  background = '',
  guidelines = '',
}) {
  const args = ['blackboard', 'set', '--role', role, '--store-id', storeId];
  if (project) args.push('--project', project);
  if (background) args.push('--background', background);
  if (guidelines) args.push('--guidelines', guidelines);
  return await runRbcJSON(...args);
}

export async function conversationSet({
  title,
  role = 'user',
  description = '',
  project = '',
  tags = '',
  notes = '',
}) {
  const args = ['conversation', 'set', '--title', title, '--role', role];
  if (description) args.push('--description', description);
  if (project) args.push('--project', project);
  if (tags) args.push('--tags', tags);
  if (notes) args.push('--notes', notes);
  return await runRbcJSON(...args);
}

export async function experimentCreate({ conversation }) {
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
}) {
  const args = ['queue', 'add', '--description', description];
  if (status) args.push('--status', status);
  if (why) args.push('--why', why);
  if (tags) args.push('--tags', tags);
  return await runRbcJSON(...args);
}

export async function stickieSet({
  id = '',
  blackboard,
  topicName = '',
  topicRole = '',
  note = '',
  code = '',
  labels = [],
  createdByTask = '',
  priority = '',
  name = '',
  variant = '',
  archived = false,
  score = null,
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
  if (priority) args.push('--priority', priority);
  if (name !== undefined) args.push('--name', name);
  if (variant !== undefined) args.push('--variant', variant);
  if (archived) args.push('--archived');
  if (score !== null && score !== undefined)
    args.push('--score', String(score));
  return await runRbcJSON(...args);
}

export async function stickieListJSON({
  blackboard = '',
  topicName = '',
  topicRole = '',
  limit = 100,
  offset = 0,
}) {
  const args = ['stickie', 'list', '--output', 'json'];
  if (blackboard) args.push('--blackboard', blackboard);
  if (topicName) args.push('--topic-name', topicName);
  if (topicRole) args.push('--topic-role', topicRole);
  args.push('--limit', String(limit), '--offset', String(offset));
  return await runRbcJSON(...args);
}

export async function stickieGetJSON({ id }) {
  return await runRbcJSON('stickie', 'get', '--id', id);
}

export async function workflowListJSON({ role, limit = 100, offset = 0 } = {}) {
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
} = {}) {
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
  variant = '',
  archived = false,
  blackboard = '',
}) {
  const args = ['stickie', 'find', '--name', name, '--variant', variant];
  if (archived) args.push('--archived');
  if (blackboard) args.push('--blackboard', blackboard);
  return await runRbcJSON(...args);
}

// Messages (stdin body)
export async function messageSet({
  text = '',
  experiment = '',
  title = '',
  tags = '',
  role = 'user',
}) {
  const args = ['message', 'set'];
  if (experiment) args.push('--experiment', experiment);
  if (title) args.push('--title', title);
  if (tags) args.push('--tags', tags);
  if (role) args.push('--role', role);
  const proc = $`go run main.go ${args}`;
  proc.stdin.write(text || '');
  proc.stdin.end();
  return await proc;
}

// Stickie relations
export async function stickieRelSet({ from, to, type, labels = '' }) {
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

export async function stickieRelList({ id, direction = 'out' }) {
  return await runRbc(
    'stickie-rel',
    'list',
    '--id',
    id,
    '--direction',
    direction,
  );
}

export async function stickieRelGet({ from, to, type, ignoreMissing = false }) {
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

// Non-JSON convenience wrappers
export async function dbReset({ dropAppRole = false } = {}) {
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

export async function tagSet({ name, title, role = 'user' }) {
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

export async function topicSet({
  name,
  role = 'user',
  title,
  description = '',
  tags = '',
}) {
  const args = [
    'topic',
    'set',
    '--name',
    name,
    '--role',
    role,
    '--title',
    title,
  ];
  if (description) args.push('--description', description);
  if (tags) args.push('--tags', tags);
  return await runRbc(...args);
}

export async function projectSet({
  name,
  role = 'user',
  description = '',
  notes = '',
  tags = '',
}) {
  const args = ['project', 'set', '--name', name, '--role', role];
  if (description) args.push('--description', description);
  if (notes) args.push('--notes', notes);
  if (tags) args.push('--tags', tags);
  return await runRbc(...args);
}

// Tools
export async function toolSet({
  name,
  title,
  role = 'user',
  description = '',
  notes = '',
  tags = '',
  settings = '', // JSON string
  type = '',
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

export async function toolGetJSON({ name }) {
  return await runRbcJSON('tool', 'get', '--name', name);
}

export async function toolListJSON({ role, limit = 100, offset = 0 }) {
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

export async function toolDelete({
  name,
  force = true,
  ignoreMissing = false,
}) {
  const args = ['tool', 'delete', '--name', name];
  if (force) args.push('--force');
  if (ignoreMissing) args.push('--ignore-missing');
  return await runRbc(...args);
}

export async function storeSet({
  name,
  role = 'user',
  title = '',
  description = '',
  motivation = '',
  security = '',
  privacy = '',
  notes = '',
  type = '',
  scope = '',
  lifecycle = '',
  tags = '',
}) {
  const args = ['store', 'set', '--name', name, '--role', role];
  if (title) args.push('--title', title);
  if (description) args.push('--description', description);
  if (motivation) args.push('--motivation', motivation);
  if (security) args.push('--security', security);
  if (privacy) args.push('--privacy', privacy);
  if (notes) args.push('--notes', notes);
  if (type) args.push('--type', type);
  if (scope) args.push('--scope', scope);
  if (lifecycle) args.push('--lifecycle', lifecycle);
  if (tags) args.push('--tags', tags);
  return await runRbc(...args);
}

export async function workspaceSet({
  role = 'user',
  project = '',
  description = '',
  tags = '',
}) {
  const args = ['workspace', 'set', '--role', role];
  if (project) args.push('--project', project);
  if (description) args.push('--description', description);
  if (tags) args.push('--tags', tags);
  return await runRbc(...args);
}

export async function packageSet({ role = 'user', variant }) {
  return await runRbc('package', 'set', '--role', role, '--variant', variant);
}

export async function queuePeek({ limit = 2 } = {}) {
  return await runRbc('queue', 'peek', '--limit', String(limit));
}
export async function queueSize() {
  return await runRbc('queue', 'size');
}
export async function queueTake({ id }) {
  return await runRbc('queue', 'take', '--id', id);
}

export async function listWithRole(cmd, role, limit = 50) {
  return await runRbc(cmd, 'list', '--role', role, '--limit', String(limit));
}
export async function experimentList(limit = 50) {
  return await runRbc('experiment', 'list', '--limit', String(limit));
}
export async function stickieList(limit = 50) {
  return await runRbc('stickie', 'list', '--limit', String(limit));
}
export async function stickieListByBlackboard({ blackboard, limit = 50 }) {
  return await runRbc(
    'stickie',
    'list',
    '--blackboard',
    blackboard,
    '--limit',
    String(limit),
  );
}
export async function stickieListByTopic({ topicName, topicRole, limit = 50 }) {
  return await runRbc(
    'stickie',
    'list',
    '--topic-name',
    topicName,
    '--topic-role',
    topicRole,
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

export async function snapshotBackupJSON({ description, who }) {
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
export async function snapshotList({ limit = 5 } = {}) {
  return await runRbc('snapshot', 'list', '--limit', String(limit));
}
export async function snapshotShow({ id }) {
  return await runRbc('snapshot', 'show', id);
}
export async function snapshotRestoreDry({ id, mode = 'append' }) {
  return await runRbc('snapshot', 'restore', id, '--mode', mode, '--dry-run');
}
export async function snapshotDelete({ id }) {
  return await runRbc('snapshot', 'delete', id, '--force');
}

export async function snapshotVerifyJSON({ id, schema = 'backup' }) {
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

export async function snapshotPruneYesJSON({
  olderThan = '90d',
  schema = 'backup',
}) {
  return await runRbcJSON(
    'snapshot',
    'prune',
    '--older-than',
    olderThan,
    '--schema',
    schema,
    '--yes',
    '--json',
  );
}

export async function projectListJSON({ role, limit = 100, offset = 0 }) {
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
export async function storeListJSON({ role, limit = 100, offset = 0 }) {
  return await runRbcJSON(
    'store',
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
export async function topicListJSON({ role, limit = 100, offset = 0 }) {
  return await runRbcJSON(
    'topic',
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
export async function blackboardListJSON({ role, limit = 100, offset = 0 }) {
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
export async function conversationGetJSON({ id }) {
  return await runRbcJSON('conversation', 'get', '--id', id);
}
export async function projectGetJSON({ name, role }) {
  return await runRbcJSON('project', 'get', '--name', name, '--role', role);
}
export async function messageListJSON({
  role,
  experiment = '',
  task = '',
  status = '',
  limit = 100,
  offset = 0,
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

// -----------------------------
// Vault helpers (read-only; never print secrets)
// -----------------------------
export async function vaultList() {
  const p = await runRbc('vault', 'list');
  const lines = (p.stdout || '')
    .split('\n')
    .map((l) => l.trim())
    .filter(Boolean);
  const items = [];
  for (const line of lines) {
    // Expected format: name\t[set|unset]\tbackend
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

export async function vaultShow(name) {
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
  // Return raw text for now; callers can inspect for 'status: OK'
  const p = await runRbc('vault', 'doctor');
  return { stdout: String(p.stdout || ''), stderr: String(p.stderr || '') };
}

// -----------------------------
// Testcase helpers
// -----------------------------
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

// -----------------------------
// Prompt helpers
// -----------------------------
export async function promptRun({
  toolName,
  input = '',
  inputFile = '',
  toolsPath = '',
  temperature = undefined,
  maxOutputTokens = undefined,
  json = false,
}) {
  const args = ['prompt', 'run', '--tool-name', toolName];
  if (input) args.push('--input', input);
  if (inputFile) args.push('--input-file', inputFile);
  if (toolsPath) args.push('--tools', toolsPath);
  if (typeof temperature === 'number')
    args.push('--temperature', String(temperature));
  if (typeof maxOutputTokens === 'number')
    args.push('--max-output-tokens', String(maxOutputTokens));
  if (json) args.push('--json');
  return await runRbc(...args);
}

export async function promptRunJSON(opts) {
  return await runRbcJSON(
    'prompt',
    'run',
    ...(() => {
      const args = [];
      if (!opts || !opts.toolName) throw new Error('toolName is required');
      args.push('--tool-name', opts.toolName);
      if (opts.input) args.push('--input', opts.input);
      if (opts.inputFile) args.push('--input-file', opts.inputFile);
      if (opts.toolsPath) args.push('--tools', opts.toolsPath);
      if (typeof opts.temperature === 'number')
        args.push('--temperature', String(opts.temperature));
      if (typeof opts.maxOutputTokens === 'number')
        args.push('--max-output-tokens', String(opts.maxOutputTokens));
      args.push('--json');
      return args;
    })(),
  );
}
