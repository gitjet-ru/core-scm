#!/usr/bin/env node
import {readFile} from 'node:fs/promises';

const allowedLicensePatterns = [
  /apache license/i,
  /\bmit license\b/i,
  /\bbsd\b/i,
  /\bisc license\b/i,
];

const forbiddenLicensePatterns = [
  /\bgnu (lesser )?general public license\b/i,
  /\blgpl\b/i,
  /\bgpl\b/i,
  /\bmozilla public license\b/i,
  /\bmpl\b/i,
  /\bcreative commons\b/i,
  /\bcc0\b/i,
  /\bpublic domain\b/i,
  /\beula\b/i,
];

const goDeniedModules = new Set([
  'github.com/hashicorp/go-version',
  'github.com/hashicorp/go-retryablehttp',
  'github.com/go-sql-driver/mysql',
  'github.com/mattn/go-sqlite3',
  'modernc.org/sqlite',
  'github.com/xi2/xz',
  'github.com/zeebo/blake3',
]);

const jsDeniedPackages = new Set([
  '@resvg/resvg-wasm',
  'eslint-plugin-sonarjs',
]);

async function readExceptions() {
  const raw = await readFile(new URL('../docs/compliance/license-exceptions.json', import.meta.url), 'utf8');
  const parsed = JSON.parse(raw);
  return {
    go: new Set(parsed.go ?? []),
    js: new Set(parsed.js ?? []),
  };
}

function isAllowedLicense(text) {
  return allowedLicensePatterns.some((p) => p.test(text));
}

function isForbiddenLicense(text) {
  return forbiddenLicensePatterns.some((p) => p.test(text));
}

async function checkGo(exceptions) {
  const raw = await readFile(new URL('../assets/go-licenses.json', import.meta.url), 'utf8');
  const licenses = JSON.parse(raw);
  const issues = [];

  for (const item of licenses) {
    const name = item.name ?? '';
    if (!name || name.startsWith('github.com/gitjet-ru/core-scm')) {
      continue;
    }

    if (exceptions.go.has(name)) {
      continue;
    }

    if (goDeniedModules.has(name)) {
      issues.push(`Go denied module: ${name}`);
      continue;
    }

    const text = (item.licenseText ?? '').toLowerCase();
    if (isForbiddenLicense(text)) {
      issues.push(`Go forbidden license: ${name}`);
      continue;
    }
    // "unknown" handling is done via legal review inventory, not hard-fail.
  }

  return issues;
}

async function checkJS(exceptions) {
  const raw = await readFile(new URL('../package.json', import.meta.url), 'utf8');
  const pkg = JSON.parse(raw);
  const all = {
    ...(pkg.dependencies ?? {}),
    ...(pkg.devDependencies ?? {}),
  };
  const issues = [];
  for (const name of Object.keys(all)) {
    if (jsDeniedPackages.has(name)) {
      if (exceptions.js.has(name)) {
        continue;
      }
      issues.push(`JS denied package: ${name}`);
    }
  }
  return issues;
}

async function main() {
  const exceptions = await readExceptions();
  const issues = [
    ...(await checkGo(exceptions)),
    ...(await checkJS(exceptions)),
  ];
  if (issues.length > 0) {
    console.error('License gate failed:');
    for (const issue of issues) {
      console.error(` - ${issue}`);
    }
    process.exit(1);
  }
  console.log('License gate passed.');
}

await main();
