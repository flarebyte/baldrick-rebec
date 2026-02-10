#!/usr/bin/env zx

function getBritishDate() {
  const now = new Date();
  const options = { year: 'numeric', month: 'long', day: 'numeric' };
  return new Intl.DateTimeFormat('en-GB', options).format(now);
}

async function readFileSafe(p) {
  try { return await fs.promises.readFile(p, 'utf8'); } catch { return ''; }
}

async function ensureDir(p) {
  await fs.promises.mkdir(p, { recursive: true });
}

// use system shasum via zx rather than Node crypto to keep script tiny

// Resolve version or default
const version = (process.env.VERSION ?? (await readFileSafe('VERSION'))).trim() || '0.0.0';

const currentDirectory = process.cwd();
const folderName = path.basename(currentDirectory);
const projectName = `github.com/flarebyte/${folderName}`;
const currentDate = getBritishDate().replaceAll(' ', '-');

// ldflags for cli version/date
const ldflags = `-X ${projectName}/cli.Version=${version} -X ${projectName}/cli.Date=${currentDate}`;
const platforms = [
  { label: 'Linux (amd64)', os: 'linux', arch: 'amd64' },
  { label: 'Linux (arm64)', os: 'linux', arch: 'arm64' },
  { label: 'macOS (Apple Silicon)', os: 'darwin', arch: 'arm64' },
];

await ensureDir('build');
for (const p of platforms) {
  console.log(p.label);
  const env = { ...process.env, GOOS: p.os, GOARCH: p.arch };
  if (p.os === 'darwin') {
    const macArch = p.arch === 'amd64' ? 'x86_64' : 'arm64';
    env.CGO_ENABLED = '1';
    env.CC = 'clang';
    env.CGO_CFLAGS = `-arch ${macArch}`;
    env.CGO_LDFLAGS = `-arch ${macArch}`;
    env.MACOSX_DEPLOYMENT_TARGET = env.MACOSX_DEPLOYMENT_TARGET || '11.0';
  }
  const out = path.join('build', `${folderName}-${p.os}-${p.arch}`);
  await $({ env })`go build -o ${out} -ldflags ${ldflags}`;
}

// checksums
const checksum = await $`shasum -a 256 build/${folderName}-*`;
await fs.promises.writeFile('build/checksums.txt', checksum.stdout, 'utf8');
