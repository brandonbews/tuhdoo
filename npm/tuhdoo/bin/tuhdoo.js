#!/usr/bin/env node
'use strict';
// Dumb launcher for the real tuhdoo binary, which ships in a per-platform
// npm package (@tuhdoo/<os>-<cpu>) selected by npm via os/cpu fields on
// optionalDependencies. This file must stay dependency-free and add
// nothing to stdout/stderr on the happy path: the MCP stdio shim
// (`tuhdoo mcp`) runs through it, and any stray byte corrupts JSON-RPC.

const { spawn } = require('child_process');

const PACKAGES = {
  'darwin-arm64': '@tuhdoo/darwin-arm64',
  'darwin-x64': '@tuhdoo/darwin-x64',
  'linux-arm64': '@tuhdoo/linux-arm64',
  'linux-x64': '@tuhdoo/linux-x64',
};

const key = process.platform + '-' + process.arch;
const pkg = PACKAGES[key];
if (!pkg) {
  console.error(
    'tuhdoo: unsupported platform ' + key +
    ' (supported: ' + Object.keys(PACKAGES).join(', ') + ')'
  );
  process.exit(1);
}

let bin;
try {
  bin = require.resolve(pkg + '/bin/tuhdoo');
} catch (_) {
  console.error(
    'tuhdoo: platform package ' + pkg + ' is not installed.\n' +
    'It is an optionalDependency of tuhdoo; reinstall without ' +
    '--no-optional / --omit=optional.'
  );
  process.exit(1);
}

const child = spawn(bin, process.argv.slice(2), { stdio: 'inherit' });

// Forward termination signals; harnesses SIGTERM this pid, not the child.
const SIGNALS = ['SIGINT', 'SIGTERM', 'SIGHUP'];
for (const sig of SIGNALS) {
  process.on(sig, () => child.kill(sig));
}

child.on('error', (err) => {
  console.error('tuhdoo: failed to launch ' + bin + ': ' + err.message);
  process.exit(1);
});

child.on('exit', (code, signal) => {
  if (signal) {
    // Die by the same signal so our caller sees the child's true fate.
    // Drop our forwarding handler first or the re-raise loops forever.
    process.removeAllListeners(signal);
    process.kill(process.pid, signal);
  } else {
    process.exit(code === null ? 1 : code);
  }
});
