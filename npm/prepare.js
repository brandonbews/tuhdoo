#!/usr/bin/env node
'use strict';
// Assembles the publishable npm packages from prebuilt tuhdoo binaries.
//
//   node npm/prepare.js <version> <dist-dir> <out-dir>
//
//   <version>   bare semver, no leading v (e.g. 0.1.0 for tag v0.1.0)
//   <dist-dir>  contains <goos>_<goarch>/tuhdoo prebuilt binaries, the
//               layout the release workflow's cross-compile step produces
//   <out-dir>   gets one ready-to-`npm publish` directory per package:
//               four @tuhdoo/<os>-<cpu> platform packages plus the main
//               tuhdoo launcher package (copied from npm/tuhdoo with the
//               version stamped in)
//
// Dependency-free on purpose; runs on the bare node of a CI runner.

const fs = require('fs');
const path = require('path');

// Go arch names on the left of the mapping, npm cpu names on the right.
const PLATFORMS = [
  { goos: 'darwin', goarch: 'arm64', os: 'darwin', cpu: 'arm64' },
  { goos: 'darwin', goarch: 'amd64', os: 'darwin', cpu: 'x64' },
  { goos: 'linux', goarch: 'arm64', os: 'linux', cpu: 'arm64' },
  { goos: 'linux', goarch: 'amd64', os: 'linux', cpu: 'x64' },
];

const REPOSITORY = {
  type: 'git',
  url: 'git+https://github.com/brandonbews/tuhdoo.git',
};

function fail(msg) {
  console.error('prepare.js: ' + msg);
  process.exit(1);
}

const [version, distDir, outDir] = process.argv.slice(2);
if (!version || !distDir || !outDir) {
  fail('usage: node npm/prepare.js <version> <dist-dir> <out-dir>');
}
if (!/^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$/.test(version)) {
  fail('version must be bare semver without the leading v, got: ' + version);
}

const templateDir = path.join(__dirname, 'tuhdoo');

for (const p of PLATFORMS) {
  const binSrc = path.join(distDir, p.goos + '_' + p.goarch, 'tuhdoo');
  if (!fs.existsSync(binSrc)) {
    fail('missing prebuilt binary: ' + binSrc);
  }
  const name = p.os + '-' + p.cpu;
  const pkgDir = path.join(outDir, name);
  fs.mkdirSync(path.join(pkgDir, 'bin'), { recursive: true });
  fs.copyFileSync(binSrc, path.join(pkgDir, 'bin', 'tuhdoo'));
  fs.chmodSync(path.join(pkgDir, 'bin', 'tuhdoo'), 0o755);
  fs.writeFileSync(
    path.join(pkgDir, 'package.json'),
    JSON.stringify(
      {
        name: '@tuhdoo/' + name,
        version: version,
        description: 'tuhdoo prebuilt binary for ' + p.os + '/' + p.cpu,
        repository: REPOSITORY,
        os: [p.os],
        cpu: [p.cpu],
      },
      null,
      2
    ) + '\n'
  );
  fs.writeFileSync(
    path.join(pkgDir, 'README.md'),
    '# @tuhdoo/' + name + '\n\nThe tuhdoo binary for ' + p.os + '/' + p.cpu +
      '. Install [tuhdoo](https://www.npmjs.com/package/tuhdoo) instead of ' +
      'this package directly.\n'
  );
}

// Main launcher package: copy the checked-in template, stamp the version
// into both the package itself and its optionalDependencies (exact-pinned
// so a given tuhdoo version always pulls its own binaries).
const mainDir = path.join(outDir, 'tuhdoo');
fs.cpSync(templateDir, mainDir, { recursive: true });
const pkgPath = path.join(mainDir, 'package.json');
const pkg = JSON.parse(fs.readFileSync(pkgPath, 'utf8'));
pkg.version = version;
for (const dep of Object.keys(pkg.optionalDependencies)) {
  pkg.optionalDependencies[dep] = version;
}
fs.writeFileSync(pkgPath, JSON.stringify(pkg, null, 2) + '\n');

console.log('assembled ' + (PLATFORMS.length + 1) + ' packages in ' + outDir);
