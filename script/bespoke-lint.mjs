#!/usr/bin/env zx

/**
 * Run semgrep with a Go pattern and return JSON results
 *
 * @param {string} pattern - Semgrep pattern (e.g. "$X == nil")
 * @param {string} [targetPath='.'] - Directory or file to scan
 * @returns {Promise<object>} Parsed Semgrep JSON output
 */
export async function runSemgrep(pattern, targetPath = '.') {
  try {
    // Run semgrep with strict JSON output and no extraneous logs
    const result =
      await $`semgrep --lang go --pattern ${pattern} --json --quiet --metrics=off ${targetPath}`;

    // semgrep prints JSON to stdout
    const json = JSON.parse(result.stdout);
    return json;
  } catch (err) {
    console.error('Semgrep execution failed');
    if (err.stdout) {
      console.error(err.stdout);
    }
    if (err.stderr) {
      console.error(err.stderr);
    }
    throw err;
  }
}

// Example usage: search repo for Go struct type declarations
const results = await runSemgrep('type $STRUCT struct {...}', '.');
console.log(results);
