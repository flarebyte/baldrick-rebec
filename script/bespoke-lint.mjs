#!/usr/bin/env zx

/**
 * Run semgrep with a Go pattern and return JSON results
 *
 * @param {string} pattern - Semgrep pattern (e.g. "$X == nil")
 * @param {string} targetPath - Directory or file to scan
 * @returns {Promise<object>} Parsed Semgrep JSON output
 */
export async function runSemgrep(pattern) {
  try {
    // Run semgrep with JSON output
    const result = await $`semgrep -l go -e 'type $STRU struct {...}'`;

    // semgrep prints JSON to stdout
    const json = JSON.parse(result.stdout);
    return json;
  } catch (err) {
    console.error("Semgrep execution failed");
    if (err.stdout) {
      console.error(err.stdout);
    }
    if (err.stderr) {
      console.error(err.stderr);
    }
    throw err;
  }
}

const results = await runSemgrep('type $STRU struct {...}');
console.log(results)
