#!/usr/bin/env node
import {createReadStream} from 'node:fs';
import {mkdir, readFile} from 'node:fs/promises';
import {spawn} from 'node:child_process';
import {createHash} from 'node:crypto';

const registryBase = process.env.REGISTRY_BASE || 'REGISTRY_BASE';
const lockFile = 'compliance/base-images.lock';
const artifactChecksumFile = 'compliance/base-image-artifacts.sha256';

const images = [
  {name: 'alpine-3.23', dockerfile: 'docker/base-images/alpine-3.23.Dockerfile', imageKey: 'ALPINE_3_23_IMAGE'},
  {name: 'alpine-3.19', dockerfile: 'docker/base-images/alpine-3.19.Dockerfile', imageKey: 'ALPINE_3_19_IMAGE'},
  {name: 'golang-1.26-alpine3.23', dockerfile: 'docker/base-images/golang-1.26-alpine3.23.Dockerfile', imageKey: 'GOLANG_1_26_ALPINE_3_23_IMAGE'},
];

const requiredArtifacts = [
  'artifacts/rootfs/alpine-minirootfs-3.23.0-x86_64.tar.gz',
  'artifacts/rootfs/alpine-minirootfs-3.19.0-x86_64.tar.gz',
  'artifacts/go/go1.26.1.linux-amd64.tar.gz',
];

function run(cmd, args, opts = {}) {
  return new Promise((resolve, reject) => {
    const child = spawn(cmd, args, {stdio: 'inherit', ...opts});
    child.on('exit', (code) => (code === 0 ? resolve() : reject(new Error(`${cmd} exited with code ${code}`))));
  });
}

async function commandExists(command) {
  try {
    await run('sh', ['-c', `command -v ${command} >/dev/null 2>&1`], {stdio: 'ignore'});
    return true;
  } catch {
    return false;
  }
}

async function readLock() {
  const raw = await readFile(lockFile, 'utf8');
  const result = {};
  for (const line of raw.split('\n')) {
    const cleaned = line.trim();
    if (!cleaned || cleaned.startsWith('#')) continue;
    const index = cleaned.indexOf('=');
    if (index === -1) continue;
    const key = cleaned.slice(0, index);
    const value = cleaned.slice(index + 1).replaceAll('REGISTRY_BASE', registryBase);
    result[key] = value;
  }
  return result;
}

async function sha256File(path) {
  return new Promise((resolve, reject) => {
    const hash = createHash('sha256');
    const stream = createReadStream(path);
    stream.on('error', reject);
    stream.on('data', (chunk) => hash.update(chunk));
    stream.on('end', () => resolve(hash.digest('hex')));
  });
}

async function verifyArtifacts() {
  const checksumRaw = await readFile(artifactChecksumFile, 'utf8');
  const checksumMap = new Map();
  for (const line of checksumRaw.split('\n')) {
    const cleaned = line.trim();
    if (!cleaned || cleaned.startsWith('#')) continue;
    const parts = cleaned.split(/\s+/);
    if (parts.length < 2) continue;
    checksumMap.set(parts[1], parts[0]);
  }

  for (const artifact of requiredArtifacts) {
    const expected = checksumMap.get(artifact);
    if (!expected) throw new Error(`Missing checksum for ${artifact} in ${artifactChecksumFile}`);
    if (expected.startsWith('REPLACE_')) {
      throw new Error(`Checksum placeholder is not replaced for ${artifact} in ${artifactChecksumFile}`);
    }
    const actual = await sha256File(artifact);
    if (actual !== expected) {
      throw new Error(`Checksum mismatch for ${artifact}: expected ${expected}, got ${actual}`);
    }
  }
}

async function build() {
  await verifyArtifacts();
  const lock = await readLock();
  for (const item of images) {
    const imageRef = lock[item.imageKey];
    if (!imageRef) throw new Error(`Missing ${item.imageKey} in ${lockFile}`);
    await run('docker', ['build', '-f', item.dockerfile, '-t', imageRef, '.']);
  }
}

async function push() {
  const lock = await readLock();
  for (const item of images) {
    const imageRef = lock[item.imageKey];
    if (!imageRef) throw new Error(`Missing ${item.imageKey} in ${lockFile}`);
    await run('docker', ['push', imageRef]);
  }
}

async function sbom() {
  const hasSyft = await commandExists('syft');
  if (!hasSyft) {
    throw new Error('syft is required for SBOM generation');
  }

  const lock = await readLock();
  await mkdir('compliance/sbom', {recursive: true});
  for (const item of images) {
    const imageRef = lock[item.imageKey];
    if (!imageRef) throw new Error(`Missing ${item.imageKey} in ${lockFile}`);
    const output = `compliance/sbom/${item.name}.spdx.json`;
    await run('syft', [imageRef, '-o', `spdx-json=${output}`]);
  }
}

async function sign() {
  const hasCosign = await commandExists('cosign');
  if (!hasCosign) {
    throw new Error('cosign is required for image signing');
  }

  const lock = await readLock();
  for (const item of images) {
    const refKey = `${item.imageKey.replace(/_IMAGE$/, '')}_REF`;
    const imageRef = lock[refKey];
    if (!imageRef) throw new Error(`Missing ${refKey} in ${lockFile}`);
    await run('cosign', ['sign', imageRef]);
  }
}

const action = process.argv[2];
if (!action) {
  console.error('Usage: node tools/base-images.mjs <build|push|sbom|sign|verify-artifacts>');
  process.exit(1);
}

const actions = {build, push, sbom, sign, 'verify-artifacts': verifyArtifacts};
const fn = actions[action];
if (!fn) {
  console.error(`Unknown action: ${action}`);
  process.exit(1);
}

await fn();
