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
export async function runSetRole({ name, title, description = '', notes = '' }) {
  return await runRbc(
    'admin',
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
  return await runRbcJSON('admin', 'role', 'get', '--name', name);
}

export async function roleListJSON({ limit = 100, offset = 0 } = {}) {
  return await runRbcJSON('admin', 'role', 'list', '--output', 'json', '--limit', String(limit), '--offset', String(offset));
}

export async function runSetWorkflow({ name, title, description = '', role = 'user', notes = '' }) {
  return await runRbc(
    'admin',
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
  const args = ['admin', 'script', 'set', '--role', role, '--title', title];
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
  return await runRbcJSON('admin', 'script', 'list', '--role', role, '--output', 'json', '--limit', String(limit), '--offset', String(offset));
}

export async function scriptFind({ name, variant = '', archived = false, role = '' }) {
  const args = ['admin', 'script', 'find', '--name', name, '--variant', variant];
  if (archived) args.push('--archived');
  if (role) args.push('--role', role);
  return await runRbcJSON(...args);
}

export async function runSetTask({ workflow, command, variant = '', role = 'user', title = '', description = '', shell = '', timeout = '', tags = '', level = '' }) {
  const args = ['admin', 'task', 'set', '--workflow', workflow, '--command', command, '--variant', variant, '--role', role];
  if (title) args.push('--title', title);
  if (description) args.push('--description', description);
  if (shell) args.push('--shell', shell);
  if (timeout) args.push('--timeout', timeout);
  if (tags) args.push('--tags', tags);
  if (level) args.push('--level', level);
  return await runRbcJSON(...args);
}

export async function storeGet({ name, role = 'user' }) {
  return await runRbcJSON('admin', 'store', 'get', '--name', name, '--role', role);
}

export async function blackboardSet({ role = 'user', storeId, project = '', background = '', guidelines = '' }) {
  const args = ['admin', 'blackboard', 'set', '--role', role, '--store-id', storeId];
  if (project) args.push('--project', project);
  if (background) args.push('--background', background);
  if (guidelines) args.push('--guidelines', guidelines);
  return await runRbcJSON(...args);
}

export async function conversationSet({ title, role = 'user' }) {
  return await runRbcJSON('admin', 'conversation', 'set', '--title', title, '--role', role);
}

export async function experimentCreate({ conversation }) {
  return await runRbcJSON('admin', 'experiment', 'create', '--conversation', conversation);
}

export async function queueAdd({ description, status = '', why = '', tags = '' }) {
  const args = ['admin', 'queue', 'add', '--description', description];
  if (status) args.push('--status', status);
  if (why) args.push('--why', why);
  if (tags) args.push('--tags', tags);
  return await runRbcJSON(...args);
}

export async function stickieSet({ id = '', blackboard, topicName = '', topicRole = '', note = '', labels = [], createdByTask = '', priority = '', name = '', variant = '', archived = false }) {
  const args = ['admin', 'stickie', 'set'];
  if (id) args.push('--id', id);
  if (blackboard) args.push('--blackboard', blackboard);
  if (topicName) args.push('--topic-name', topicName);
  if (topicRole) args.push('--topic-role', topicRole);
  if (note) args.push('--note', note);
  if (labels && labels.length) args.push('--labels', labels.join(','));
  if (createdByTask) args.push('--created-by-task', createdByTask);
  if (priority) args.push('--priority', priority);
  if (name !== undefined) args.push('--name', name);
  if (variant !== undefined) args.push('--variant', variant);
  if (archived) args.push('--archived');
  return await runRbcJSON(...args);
}

export async function stickieListJSON({ blackboard = '', topicName = '', topicRole = '', limit = 100, offset = 0 }) {
  const args = ['admin', 'stickie', 'list', '--output', 'json'];
  if (blackboard) args.push('--blackboard', blackboard);
  if (topicName) args.push('--topic-name', topicName);
  if (topicRole) args.push('--topic-role', topicRole);
  args.push('--limit', String(limit), '--offset', String(offset));
  return await runRbcJSON(...args);
}

export async function stickieFind({ name, variant = '', archived = false, blackboard = '' }) {
  const args = ['admin', 'stickie', 'find', '--name', name, '--variant', variant];
  if (archived) args.push('--archived');
  if (blackboard) args.push('--blackboard', blackboard);
  return await runRbcJSON(...args);
}

// Messages (stdin body)
export async function messageSet({ text = '', experiment = '', title = '', tags = '', role = 'user' }) {
  const args = ['admin', 'message', 'set'];
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
  const args = ['admin', 'stickie-rel', 'set', '--from', from, '--to', to, '--type', type];
  if (labels) args.push('--labels', labels);
  return await runRbc(...args);
}

export async function stickieRelList({ id, direction = 'out' }) {
  return await runRbc('admin', 'stickie-rel', 'list', '--id', id, '--direction', direction);
}

export async function stickieRelGet({ from, to, type, ignoreMissing = false }) {
  const args = ['admin', 'stickie-rel', 'get', '--from', from, '--to', to, '--type', type];
  if (ignoreMissing) args.push('--ignore-missing');
  return await runRbc(...args);
}

// Non-JSON convenience wrappers
export async function dbReset({ dropAppRole = false } = {}) {
  return await runRbc('admin', 'db', 'reset', '--force', `--drop-app-role=${dropAppRole ? 'true' : 'false'}`);
}

export async function dbScaffoldAll() {
  return await runRbc('admin', 'db', 'scaffold', '--all', '--yes');
}

export async function taskSetReplacement({ workflow, command, variant = '', role = 'user', title = '', description = '', shell = '', replaces = '', replaceLevel = '', replaceComment = '' }) {
  const args = ['admin', 'task', 'set', '--workflow', workflow, '--command', command, '--variant', variant, '--role', role];
  if (title) args.push('--title', title);
  if (description) args.push('--description', description);
  if (shell) args.push('--shell', shell);
  if (replaces) args.push('--replaces', replaces);
  if (replaceLevel) args.push('--replace-level', replaceLevel);
  if (replaceComment) args.push('--replace-comment', replaceComment);
  return await runRbc(...args);
}

export async function tagSet({ name, title, role = 'user' }) {
  return await runRbc('admin', 'tag', 'set', '--name', name, '--title', title, '--role', role);
}

export async function topicSet({ name, role = 'user', title, description = '', tags = '' }) {
  const args = ['admin', 'topic', 'set', '--name', name, '--role', role, '--title', title];
  if (description) args.push('--description', description);
  if (tags) args.push('--tags', tags);
  return await runRbc(...args);
}

export async function projectSet({ name, role = 'user', description = '', tags = '' }) {
  const args = ['admin', 'project', 'set', '--name', name, '--role', role];
  if (description) args.push('--description', description);
  if (tags) args.push('--tags', tags);
  return await runRbc(...args);
}

export async function storeSet({ name, role = 'user', title = '', description = '', type = '', scope = '', lifecycle = '', tags = '' }) {
  const args = ['admin', 'store', 'set', '--name', name, '--role', role];
  if (title) args.push('--title', title);
  if (description) args.push('--description', description);
  if (type) args.push('--type', type);
  if (scope) args.push('--scope', scope);
  if (lifecycle) args.push('--lifecycle', lifecycle);
  if (tags) args.push('--tags', tags);
  return await runRbc(...args);
}

export async function workspaceSet({ role = 'user', project = '', description = '', tags = '' }) {
  const args = ['admin', 'workspace', 'set', '--role', role];
  if (project) args.push('--project', project);
  if (description) args.push('--description', description);
  if (tags) args.push('--tags', tags);
  return await runRbc(...args);
}

export async function packageSet({ role = 'user', variant }) {
  return await runRbc('admin', 'package', 'set', '--role', role, '--variant', variant);
}

export async function queuePeek({ limit = 2 } = {}) { return await runRbc('admin', 'queue', 'peek', '--limit', String(limit)); }
export async function queueSize() { return await runRbc('admin', 'queue', 'size'); }
export async function queueTake({ id }) { return await runRbc('admin', 'queue', 'take', '--id', id); }

export async function listWithRole(cmd, role, limit = 50) {
  return await runRbc('admin', cmd, 'list', '--role', role, '--limit', String(limit));
}
export async function experimentList(limit = 50) { return await runRbc('admin', 'experiment', 'list', '--limit', String(limit)); }
export async function stickieList(limit = 50) { return await runRbc('admin', 'stickie', 'list', '--limit', String(limit)); }
export async function stickieListByBlackboard({ blackboard, limit = 50 }) { return await runRbc('admin', 'stickie', 'list', '--blackboard', blackboard, '--limit', String(limit)); }
export async function stickieListByTopic({ topicName, topicRole, limit = 50 }) { return await runRbc('admin', 'stickie', 'list', '--topic-name', topicName, '--topic-role', topicRole, '--limit', String(limit)); }

export async function dbCountPerRole() { return await runRbc('admin', 'db', 'count', '--per-role'); }
export async function dbCountJSON() { return await runRbc('admin', 'db', 'count', '--json'); }

export async function snapshotBackupJSON({ description, who }) {
  return await runRbcJSON('admin', 'snapshot', 'backup', '--description', description, '--who', who, '--json');
}
export async function snapshotList({ limit = 5 } = {}) { return await runRbc('admin', 'snapshot', 'list', '--limit', String(limit)); }
export async function snapshotShow({ id }) { return await runRbc('admin', 'snapshot', 'show', id); }
export async function snapshotRestoreDry({ id, mode = 'append' }) { return await runRbc('admin', 'snapshot', 'restore', id, '--mode', mode, '--dry-run'); }
export async function snapshotDelete({ id }) { return await runRbc('admin', 'snapshot', 'delete', id, '--force'); }

export async function snapshotVerifyJSON({ id, schema = 'backup' }) {
  return await runRbcJSON('admin', 'snapshot', 'verify', id, '--schema', schema, '--json');
}

export async function snapshotPrunePreviewJSON({ olderThan = '90d', schema = 'backup' }) {
  return await runRbcJSON('admin', 'snapshot', 'prune', '--older-than', olderThan, '--schema', schema, '--json');
}

export async function snapshotPruneYesJSON({ olderThan = '90d', schema = 'backup' }) {
  return await runRbcJSON('admin', 'snapshot', 'prune', '--older-than', olderThan, '--schema', schema, '--yes', '--json');
}
