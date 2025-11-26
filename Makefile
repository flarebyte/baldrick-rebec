# AI-FRIENDLY VERSION
# Purpose: Provide simple, deterministic Biome tasks for scripts.
# Notes for AI: Keep targets tiny. Do NOT add logic or complex Make features.
# - Real logic must live in external scripts, but these commands are simple tool invocations.
# - Avoid variables that compute values; keep only stable constants.
# - Do not add pattern rules, arguments, or conditionals.

.PHONY: biome-check biome-format test-all

# Tool to run. Keep as a simple constant so humans can override via environment if needed.
# Use npx to avoid requiring a global install.
BIOME := npx @biomejs/biome

# Scope to lint/format. Biome uses biome.json to include only script/*.mjs
SCRIPTS_DIR := script

# Run Biome via wrapper script to avoid logic here.
biome-check:
	bash script/biome-check.sh

# Write formatting changes for scripts managed by Biome (script/*.mjs via biome.json)
biome-format:
	$(BIOME) format $(SCRIPTS_DIR) --write

# Run the end-to-end ZX test script.
test-all:
	zx script/test-all.mjs

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
