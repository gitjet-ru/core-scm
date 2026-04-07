#!/usr/bin/env node
import {readFile} from 'node:fs/promises';
import {resolve} from 'node:path';

const root = process.cwd();

async function readText(path) {
  return readFile(resolve(root, path), 'utf8');
}

function hasAny(text, needles) {
  return needles.filter((n) => text.includes(n));
}

async function main() {
  const policy = JSON.parse(await readText('compliance/license-policy.json'));
  const goMod = await readText('go.mod');
  const goSum = await readText('go.sum');
  const packageJson = await readText('package.json');
  const pnpmLock = await readText('pnpm-lock.yaml');

  const failures = [];

  const blockedGo = [
    ...hasAny(goMod, policy.go.blockedModules),
    ...hasAny(goSum, policy.go.blockedModules),
  ];
  if (blockedGo.length) {
    failures.push(`Blocked Go modules found: ${[...new Set(blockedGo)].join(', ')}`);
  }

  const blockedNpm = [
    ...hasAny(packageJson, policy.npm.blockedPackages),
    ...hasAny(pnpmLock, policy.npm.blockedPackages),
  ];
  if (blockedNpm.length) {
    failures.push(`Blocked npm packages found: ${[...new Set(blockedNpm)].join(', ')}`);
  }

  if (failures.length) {
    console.error('[license-gate] FAILED');
    for (const f of failures) console.error(`- ${f}`);
    process.exit(1);
  }

  console.log('[license-gate] OK');
}

await main();
