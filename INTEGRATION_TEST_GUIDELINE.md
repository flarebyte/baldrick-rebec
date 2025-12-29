# Integration Test Guideline (for AI Agents)

This document is a prompt for AI agents authoring and maintaining our ZX‑based integration tests. It explains what code belongs in each script under `script/`, with clear boundaries and patterns to keep the tests readable, maintainable, and reliable.

## Purpose
- Provide a single, end‑to‑end smoke/regression test (`script/test-all.mjs`) that exercises the CLI (`rbc`) across core entities.
- Centralize CLI invocation patterns in helpers (`script/cli-helper.mjs`).
- Centralize JSON contract assertions in a single place (`script/contract-helper.mjs`).

## High‑Level Principles
- Single responsibility per file: orchestrate vs. invoke vs. validate.
- Prefer composition over duplication: add wrappers once, reuse everywhere.
- Keep tests deterministic and readable: no ad‑hoc shell pipelines.
- Fail loudly with full context: always print stack, stdout, and stderr on failure.
- Avoid environment or runner coupling beyond `zx`.

## File Responsibilities

### 1) `script/test-all.mjs` (Orchestrator)
Use this file to script the end‑to‑end flow. It should:
- Import wrappers from `script/cli-helper.mjs` and validators from `script/contract-helper.mjs`.
- Orchestrate the scenario in clear, incremental steps (create, list, validate, snapshot, etc.).
- Use helper wrappers for all CLI calls (JSON and non‑JSON). Do not call `go run main.go …` directly here.
- Use contract validators for JSON outputs; use simple `assert(...)` for targeted invariants.
- Handle errors once at the bottom with a single `try/catch` that prints:
  - `err.stack` (preferred)
  - `err.stdout`/`err.stderr` when present
- Avoid importing core Node modules (`fs`, `path`, …). Use `zx` globals and helpers only.
- Avoid business logic or data shaping — push that to helpers/contracts.
- Accept control flags via `argv` (e.g., `--skip-reset`, `--skip-snapshot`).

What does NOT belong here:
- Raw `runRbc`/`runRbcJSON` implementation details.
- Zod schemas or schema decisions.
- Parsing stdout JSON manually — always use `…JSON` helpers.

### 2) `script/cli-helper.mjs` (CLI Invocation Wrappers)
This is the single place to define thin wrappers over the `rbc` CLI. It should:
- Export `runRbc` and `runRbcJSON` utility functions and structured wrappers per command (e.g., `workflowListJSON`, `taskListJSON`, `stickieSet`, `messageListJSON`, etc.).
- Accept plain parameters; do not hardcode environment or defaults beyond CLI flags.
- For commands requiring stdin, provide a wrapper that wires stdin (e.g., `messageSet`).
- Never import Zod here — no validation in helpers.
- Keep wrappers thin: no test assertions, only invocation and argument assembly.
- Prefer optional chaining and simple guards over verbose conditionals when appending flags.
- Do not mutate shared state; return either JSON (from `runRbcJSON`) or the spawned process result.

What does NOT belong here:
- Contract validation (Zod schemas), assertions, or test‑specific decisions.
- Business logic or data post‑processing beyond composing CLI arguments.

### 3) `script/contract-helper.mjs` (Zod Contracts)
This file contains only Zod schemas and exported `validate…Contract` functions. It should:
- Import Zod via ESM: `import { z } from 'zod'` (ensures a single Zod instance).
- Define one factory per entity list item and export `validate…ListContract` (and `validate…Contract` for single objects where useful).
- Validation rules:
  - Required strings must be non‑empty (`z.string().min(1)`).
  - Only truly optional fields should be marked `.optional()`.
  - If a field is optional but present in output, prefer `min(1)` constraints (e.g., optional `title` still must be non‑empty when present).
  - Omit/relax fields with shape variability across outputs (e.g., `tags` on some entities) unless the shape is stable.
- Allow parameterization for certain leniencies (e.g., `{ allowEmptyTitle: false }`).

What does NOT belong here:
- Any CLI calls or process spawning.
- Test flow orchestration or assertion logging.

## Patterns & Conventions
- Wrapper naming: `verbNounJSON` for JSON lists/reads (e.g., `projectListJSON`); `nounAction` for non‑JSON where appropriate (e.g., `queuePeek`).
- Contract naming: `validate<Noun>ListContract` for lists; `validate<Noun>Contract` for single objects.
- Error handling: centralize in `test-all.mjs`; helpers/contracts should throw and let the orchestrator print details.
- Zod instance: only `contract-helper.mjs` imports Zod directly; do not import Zod elsewhere to avoid multiple instances.

## How to Add New Coverage
1. Add a CLI wrapper in `cli-helper.mjs` for the new `rbc` command.
2. Add/extend a Zod schema and validator in `contract-helper.mjs` (keep rules above).
3. In `test-all.mjs`, call the wrapper and validate with the corresponding contract.
4. Keep the step readable and deterministic; print minimal but helpful progress.

## Examples
- Adding a new list contract:
  - Helper: `export async function packageListJSON({ role, limit = 100 }) { return await runRbcJSON('package','list','--role',role,'--output','json','--limit',String(limit)); }`
  - Contract: `function packageListItemSchemaFactory() { return z.object({ id: z.string().min(1), role: z.string().min(1), task_id: z.string().min(1).optional(), created: z.string().optional() }); } export function validatePackageListContract(a){ return z.array(packageListItemSchemaFactory()).parse(a) }`
  - Test: `const pkgs = await packageListJSON({ role: TEST_ROLE_USER }); validatePackageListContract(pkgs);`

Follow these rules to keep integration tests robust, clear, and low‑friction for future contributors and automation.
