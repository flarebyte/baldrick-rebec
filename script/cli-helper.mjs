// Common CLI helpers for ZX-based admin scripts
// Note: Keep ZX idioms only; do not import fs/path. Ensure $ and argv are available.
import 'zx/globals';

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

