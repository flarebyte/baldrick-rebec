// Contract validation helpers using Zod. These are for test-only assertions.
// ZX runner will auto-install zod when run with `zx --install`.
import { z } from 'zod';

function roleSchemaFactory({ allowEmptyTitle = false } = {}) {
  const titleSchema = allowEmptyTitle ? z.string() : z.string().min(1);
  return z.object({
    name: z.string().min(1),
    title: titleSchema,
    description: z.string().optional(),
    notes: z.string().optional(),
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

