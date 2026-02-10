MAKEFILE guide — Tiny, predictable, AI‑friendly

Purpose

- Keep the Makefile tiny, readable, and deterministic.
- Real logic lives in scripts; Make just wires simple commands together.

Core conventions

- Use explicit .PHONY declarations for all public targets.
- No shell logic in Make: avoid conditionals, pattern rules, loops.
- One‑liners per target; call out to scripts for anything non‑trivial.
- Prefer cross‑platform tools (node + zx, or tiny bash that stays POSIX).
- Keep variables simple, stable constants (no computed Make vars).
- Document targets via a help target that prints a static list.

Existing helpers and variables

- ZX runner: `ZX := npx zx`
- Run CLI quickly: `RBC := go run main.go`

Standard targets (present today)

- `lint` — run project linters (delegates to script/biome‑check.sh)
- `format` — gofmt + Biome formatting for scripts
- `test` — end‑to‑end test script + a couple CLI smoke calls
- `gen` — generate artifacts (protobuf/Connect), via NPM scripts
- `clean` — remove generated artifacts (kept small and explicit)
- `build` — build rbc binaries via `build-go.mjs` (injects version/date)
- `release` — build multi‑OS binaries and create a GitHub release from VERSION
- `help` — print the static list of targets and what they do

Versioning and releases

- Source of truth for version is the root `VERSION` file (single line, e.g., 1.2.3).
- `build-go.mjs` injects `cli.Version` and `cli.Date` via Go `-ldflags`.
- `make build` produces binaries under `build/` and a `checksums.txt` file.
- `make release` clears `build/`, rebuilds, then runs `gh release create v$(cat VERSION) ./build/* --generate-notes`.

When adding a new target

- Keep it to a single, explicit shell command when possible.
- If it needs logic (arguments, loops, conditionals), create a script in `script/` and call it from the Makefile.
- Add a brief line to the help target. Keep descriptions short and action‑oriented.

Example pattern

- Make target (simple wrapper):
  mytool:
  	$(ZX) script/mytool.mjs

- Script (logic lives here): `script/mytool.mjs`
  - Parse args, perform checks, call programs, handle output.
  - Keep cross‑platform where feasible.

Do / Don’t

- Do: keep everything explicit, deterministic, and minimal.
- Do: reuse ZX, bash scripts, or NPM scripts for real work.
- Don’t: introduce dynamic Make variables, auto‑discovery, or hidden deps.
- Don’t: add pattern rules or multi‑line shell logic into Make targets.

Help target template

- Update the `help` target whenever you add/remove targets.
- Use consistent wording and alignment; list most used first.

