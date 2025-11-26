#!/usr/bin/env bash
# Purpose: Run Biome checks twice: AI-friendly (rdjson) then human-friendly (colored)
# Why: Keep Makefile simple (one-line recipe), yet show both outputs even if one phase fails.

set -u

# Use separate variables for command and package to avoid spaces in command name.
BIOME_NPX=${BIOME_NPX:-npx}
BIOME_PKG=${BIOME_PKG:-@biomejs/biome}
TARGET_DIR=${1:-script}

# AI-friendly report
"${BIOME_NPX}" "${BIOME_PKG}" ci "${TARGET_DIR}" --reporter=rdjson
ai_status=$?

echo '--- HUMAN VERSION BELOW ---'

# Human-friendly report with color
FORCE_COLOR=1 "${BIOME_NPX}" "${BIOME_PKG}" ci "${TARGET_DIR}"
human_status=$?

# Exit non-zero if any phase failed, but run both phases so humans see errors.
if [ "$ai_status" -ne 0 ] || [ "$human_status" -ne 0 ]; then
  exit 1
fi
