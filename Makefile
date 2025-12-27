# AI-FRIENDLY VERSION
# Purpose: Provide simple, deterministic Biome tasks for scripts.
# Notes for AI: Keep targets tiny. Do NOT add logic or complex Make features.
# - Real logic must live in external scripts, but these commands are simple tool invocations.
# - Avoid variables that compute values; keep only stable constants.
# - Do not add pattern rules, arguments, or conditionals.

.PHONY: lint format test gen clean help

ZX := npx zx

# Generic lint (abstract across languages): delegate to project script
lint:
	bash script/biome-check.sh

# Generic format: keep it simple and fast
format:
	gofmt -w .
	npx @biomejs/biome format script --write
	npx @biomejs/biome check script --write

# Generic test: end-to-end script
test: gen
	$(ZX) script/test-all.mjs

# Generate artifacts (e.g., client stubs)
gen:
	cd script && npm run gen

# Clean generated artifacts
clean:
	cd script && npm run gen:clean

terms:
	osascript -l JavaScript script/terminals.js

termsc:
	osascript -l JavaScript script/terminals-conversation.js

# HUMAN: Print a clear list of available Make targets and what they do.
# AI: Keep this static and explicit; do not auto-parse or add shell logic.
help:
	@printf "Make targets (generic):\n"
	@printf "  lint    Run project linters (fast, generic).\n"
	@printf "  format  Apply basic formatting.\n"
	@printf "  test    Run end-to-end tests.\n"
	@printf "  gen     Generate artifacts (e.g., client stubs).\n"
	@printf "  clean   Clean generated artifacts.\n"

# --- HUMAN VERSION BELOW ---
# Goal:
# Keep the Makefile tiny, predictable, and easy for humans to use.
# AI maintains it but avoids complexity; real logic belongs in scripts.
#
# Targets:
# - biome-check  : Runs Biome twice via script (rdjson for AI, colored for humans)
# - biome-format : Applies formatting with Biome to script/*.mjs
# - test-all     : Runs the ZX end-to-end test script (script/test-all.mjs)
#
# Usage:
#   make biome-check
#   make biome-format
#
# Why so simple:
# - Biome config (biome.json) defines the scope (script/*.mjs). Calling the tool directly is sufficient.
# - No shell logic in Makefile, no arguments or conditionals, no pattern rules.
