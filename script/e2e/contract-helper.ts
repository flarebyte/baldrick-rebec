import { z } from 'zod';

function roleSchemaFactory({
  allowEmptyTitle = false,
}: {
  allowEmptyTitle?: boolean;
} = {}) {
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

export function validateRoleContract(
  obj: unknown,
  opts: { allowEmptyTitle?: boolean } = {},
) {
  return roleSchemaFactory(opts).parse(obj);
}

export function validateRoleListContract(
  arr: unknown,
  opts: { allowEmptyTitle?: boolean } = {},
) {
  return z.array(roleSchemaFactory(opts)).parse(arr);
}

function workflowSchemaFactory({
  allowEmptyTitle = false,
}: {
  allowEmptyTitle?: boolean;
} = {}) {
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

export function validateWorkflowListContract(
  arr: unknown,
  opts: { allowEmptyTitle?: boolean } = {},
) {
  return z.array(workflowSchemaFactory(opts)).parse(arr);
}

function scriptSchemaFactory({
  allowEmptyTitle = false,
}: {
  allowEmptyTitle?: boolean;
} = {}) {
  const titleSchema = allowEmptyTitle ? z.string() : z.string().min(1);
  return z.object({
    id: z.string().min(1),
    title: titleSchema,
    role: z.string().min(1),
    content_id: z.string().optional(),
    name: z.string().min(1).optional(),
    archived: z.boolean().optional(),
    description: z.string().min(1).optional(),
    motivation: z.string().min(1).optional(),
    notes: z.string().min(1).optional(),
    created: z.string().optional(),
    updated: z.string().optional(),
  });
}

export function validateScriptListContract(
  arr: unknown,
  opts: { allowEmptyTitle?: boolean } = {},
) {
  return z.array(scriptSchemaFactory(opts)).parse(arr);
}

function taskSchemaFactory({
  allowEmptyTitle = true,
}: {
  allowEmptyTitle?: boolean;
} = {}) {
  const titleSchema = allowEmptyTitle ? z.string() : z.string().min(1);
  return z.object({
    id: z.string().min(1),
    command: z.string().min(1),
    variant: z.string().min(1),
    workflow: z.string().min(1).optional(),
    title: titleSchema.optional(),
    level: z.string().min(1).optional(),
    archived: z.boolean().optional(),
    created: z.string().optional(),
  });
}

export function validateTaskListContract(
  arr: unknown,
  opts: { allowEmptyTitle?: boolean } = {},
) {
  return z.array(taskSchemaFactory(opts)).parse(arr);
}

function stickieListItemSchemaFactory() {
  return z.object({
    id: z.string().min(1),
    blackboard_id: z.string().min(1),
    edit_count: z.number().int().nonnegative().optional(),
    name: z.string().min(1).optional(),
    variant: z.string().optional(),
    archived: z.boolean().optional(),
    updated: z.string().optional(),
  });
}

export function validateStickieListContract(arr: unknown) {
  return z.array(stickieListItemSchemaFactory()).parse(arr);
}

function projectListItemSchemaFactory() {
  return z.object({
    name: z.string().min(1),
    role: z.string().min(1),
    description: z.string().min(1).optional(),
    notes: z.string().min(1).optional(),
    created: z.string().optional(),
    updated: z.string().optional(),
  });
}

export function validateProjectListContract(arr: unknown) {
  return z.array(projectListItemSchemaFactory()).parse(arr);
}

function blackboardListItemSchemaFactory() {
  return z.object({
    id: z.string().min(1),
    role: z.string().min(1),
    project: z.string().min(1).optional(),
    lifecycle: z.string().min(1).optional(),
    updated: z.string().optional(),
  });
}

export function validateBlackboardListContract(arr: unknown) {
  return z.array(blackboardListItemSchemaFactory()).parse(arr);
}

function conversationListItemSchemaFactory() {
  return z.object({
    id: z.string().min(1),
    title: z.string().min(1),
    project: z.string().min(1).optional(),
    tags: z.record(z.any()).optional(),
    created: z.string().optional(),
  });
}

export function validateConversationListContract(arr: unknown) {
  return z.array(conversationListItemSchemaFactory()).parse(arr);
}

function messageListItemSchemaFactory() {
  return z.object({
    id: z.string().min(1),
    content_id: z.string().min(1),
    status: z.string().min(1),
    created: z.string().min(1),
    from_task_id: z.string().min(1).optional(),
    experiment_id: z.string().min(1).optional(),
  });
}

export function validateMessageListContract(arr: unknown) {
  return z.array(messageListItemSchemaFactory()).parse(arr);
}

const VaultStatusEnum = z.enum(['set', 'unset']);

export function validateVaultListContract(arr: unknown) {
  const item = z.object({
    name: z.string().min(1),
    status: VaultStatusEnum,
    backend: z.string().min(1),
  });
  return z.array(item).parse(arr);
}

export function validateVaultShowContract(obj: unknown) {
  const schema = z.object({
    name: z.string().min(1),
    status: VaultStatusEnum,
    backend: z.string().min(1),
    updated: z.string().optional(),
  });
  return schema.parse(obj);
}
