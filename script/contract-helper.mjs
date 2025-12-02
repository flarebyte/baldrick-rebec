// Contract validation helpers using Zod. These are for test-only assertions.
// ZX runner will auto-install zod when run with `zx --install`.
import { z } from 'zod';

function roleSchemaFactory({ allowEmptyTitle = false } = {}) {
  const titleSchema = allowEmptyTitle ? z.string() : z.string().min(1);
  return z.object({
    name: z.string().min(1),
    title: titleSchema,
    description: z.string().min(1).optional(),
    notes: z.string().min(1).optional(),
    tags: z.record(z.any()).optional(),
    created: z.string().optional(),
    updated: z.string().optional(),
  });
}

export function validateRoleContract(obj, opts = {}) {
  return roleSchemaFactory(opts).parse(obj);
}

export function validateRoleListContract(arr, opts = {}) {
  const schema = z.array(roleSchemaFactory(opts));
  return schema.parse(arr);
}

// Workflows
function workflowSchemaFactory({ allowEmptyTitle = false } = {}) {
  const titleSchema = allowEmptyTitle ? z.string() : z.string().min(1);
  return z.object({
    name: z.string().min(1),
    title: titleSchema,
    description: z.string().min(1).optional(),
    notes: z.string().min(1).optional(),
    created: z.string().optional(),
    updated: z.string().optional(),
  });
}

export function validateWorkflowListContract(arr, opts = {}) {
  return z.array(workflowSchemaFactory(opts)).parse(arr);
}

// Scripts
function scriptSchemaFactory({ allowEmptyTitle = false } = {}) {
  const titleSchema = allowEmptyTitle ? z.string() : z.string().min(1);
  return z.object({
    id: z.string().min(1),
    title: titleSchema,
    role: z.string().min(1),
    content_id: z.string().optional(),
    name: z.string().min(1).optional(),
    variant: z.string().optional(),
    archived: z.boolean().optional(),
    description: z.string().min(1).optional(),
    motivation: z.string().min(1).optional(),
    notes: z.string().min(1).optional(),
    created: z.string().optional(),
    updated: z.string().optional(),
  });
}

export function validateScriptListContract(arr, opts = {}) {
  return z.array(scriptSchemaFactory(opts)).parse(arr);
}

// Tasks
function taskSchemaFactory({ allowEmptyTitle = true } = {}) {
  const titleSchema = allowEmptyTitle ? z.string() : z.string().min(1);
  return z.object({
    id: z.string().min(1),
    command: z.string().min(1),
    variant: z.string().min(1),
    // if present, ensure non-empty
    workflow: z.string().min(1).optional(),
    title: titleSchema.optional(),
    // tags intentionally omitted due to variability
    level: z.string().min(1).optional(),
    archived: z.boolean().optional(),
    created: z.string().optional(),
  });
}

export function validateTaskListContract(arr, opts = {}) {
  return z.array(taskSchemaFactory(opts)).parse(arr);
}

// Stickies (list items)
function stickieListItemSchemaFactory() {
  return z.object({
    id: z.string().min(1),
    blackboard_id: z.string().min(1),
    edit_count: z.number().int().nonnegative().optional(),
    topic_name: z.string().min(1).optional(),
    topic_role_name: z.string().min(1).optional(),
    name: z.string().min(1).optional(),
    variant: z.string().optional(),
    archived: z.boolean().optional(),
    updated: z.string().optional(),
  });
}

export function validateStickieListContract(arr) {
  return z.array(stickieListItemSchemaFactory()).parse(arr);
}
