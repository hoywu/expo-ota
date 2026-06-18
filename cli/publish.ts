#!/usr/bin/env bun
/**
 * Expo OTA publish CLI — plan → COS direct upload → finalize (pending draft).
 *
 * Usage:
 *   OTA_API=https://ota.example.com \
 *   OTA_TOKEN=ota_pat_xxx \
 *   OTA_APP_SLUG=my-app \
 *   OTA_PLATFORM=ios \
 *   OTA_DIST_DIR=./dist \
 *   OTA_RUNTIME_VERSION=1.0.0 \
 *   bun run cli/publish.ts
 *
 * Optional:
 *   OTA_MESSAGE="fix checkout"   (defaults to latest git commit message, or "" outside a git repo)
 *   OTA_GIT_COMMIT_HASH=abc123   (defaults to `git rev-parse HEAD` when in a git repo)
 *   OTA_UPLOAD_CONCURRENCY=4     (parallel PUT uploads; default 4)
 *
 * Publish the pending draft from the dashboard after finalize completes.
 */

import { $ } from "bun";
import logUpdate from "log-update";
import mime from "mime";
import { createHash } from "node:crypto";
import { readFile } from "node:fs/promises";
import { join } from "node:path";

/** Max parallel PUT uploads. Override with OTA_UPLOAD_CONCURRENCY. */
const UPLOAD_CONCURRENCY = Math.max(
  1,
  Number.parseInt(process.env.OTA_UPLOAD_CONCURRENCY ?? "4", 10) || 4,
);

type Platform = "ios" | "android";

interface PlanAssetItem {
  key: string;
  sha256: string;
  size: number;
  contentType: string;
  fileExt?: string;
}

interface PlanMissingItem {
  key: string;
  putUrl: string;
  putHeaders: Record<string, string>;
}

interface PlanResp {
  missing: PlanMissingItem[];
  reuse: { key: string; finalUrl: string }[];
}

interface FinalizeResp {
  updateId: string;
  manifestUuid: string;
  status: string;
  createdAt: string;
}

interface UploadPayload {
  runtimeVersion: string;
  platform: Platform;
  manifestMetadata: Record<string, unknown>;
  expoConfig: Record<string, unknown>;
  message: string;
  gitCommitHash?: string;
  assets: PlanAssetItem[];
}

function requireEnv(name: string): string {
  const value = process.env[name]?.trim();
  if (!value) {
    throw new Error(`Missing required env: ${name}`);
  }
  return value;
}

function parsePlatform(raw: string): Platform {
  if (raw === "ios" || raw === "android") return raw;
  throw new Error(
    `OTA_PLATFORM must be "ios" or "android", got ${JSON.stringify(raw)}`,
  );
}

function md5(buf: Buffer): string {
  return createHash("md5").update(buf).digest("hex");
}

function sha256B64url(buf: Buffer): string {
  return createHash("sha256").update(buf).digest("base64url");
}

interface MetadataAssetEntry {
  path: string;
  ext: string;
}

interface PlatformMetadata {
  bundle: string;
  assets: MetadataAssetEntry[];
}

function normalizeExt(ext: string): string {
  return ext.startsWith(".") ? ext : `.${ext}`;
}

function contentTypeFromExt(ext: string): string {
  const dotExt = normalizeExt(ext);
  if (dotExt === ".hbc" || dotExt === ".js") return "application/javascript";

  const type = mime.getType(dotExt);
  if (type === "text/javascript") return "application/javascript";
  return type ?? "application/octet-stream";
}

function parsePlatformMetadata(
  metadata: Record<string, unknown>,
  platform: Platform,
): PlatformMetadata {
  const fileMetadata = metadata.fileMetadata;
  if (
    !fileMetadata ||
    typeof fileMetadata !== "object" ||
    Array.isArray(fileMetadata)
  ) {
    throw new Error("metadata.json: missing or invalid fileMetadata");
  }

  const raw = (fileMetadata as Record<string, unknown>)[platform];
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) {
    throw new Error(`metadata.json: missing fileMetadata.${platform}`);
  }

  const platformMeta = raw as Record<string, unknown>;
  const bundle = platformMeta.bundle;
  if (typeof bundle !== "string" || !bundle.trim()) {
    throw new Error(`metadata.json: missing fileMetadata.${platform}.bundle`);
  }

  const rawAssets = platformMeta.assets;
  if (rawAssets === undefined) {
    return { bundle, assets: [] };
  }
  if (!Array.isArray(rawAssets)) {
    throw new Error(
      `metadata.json: fileMetadata.${platform}.assets must be an array`,
    );
  }

  const assets: MetadataAssetEntry[] = [];
  for (const [i, entry] of rawAssets.entries()) {
    if (!entry || typeof entry !== "object" || Array.isArray(entry)) {
      throw new Error(
        `metadata.json: fileMetadata.${platform}.assets[${i}] must be an object`,
      );
    }
    const item = entry as Record<string, unknown>;
    if (typeof item.path !== "string" || !item.path.trim()) {
      throw new Error(
        `metadata.json: fileMetadata.${platform}.assets[${i}].path must be a non-empty string`,
      );
    }
    if (typeof item.ext !== "string" || !item.ext.trim()) {
      throw new Error(
        `metadata.json: fileMetadata.${platform}.assets[${i}].ext must be a non-empty string`,
      );
    }
    assets.push({ path: item.path, ext: item.ext });
  }

  return { bundle, assets };
}

async function readJson(path: string): Promise<Record<string, unknown>> {
  const raw = await readFile(path, "utf8");
  const parsed: unknown = JSON.parse(raw);
  if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
    throw new Error(`${path} must contain a JSON object`);
  }
  return parsed as Record<string, unknown>;
}

async function resolveGitCommitHash(): Promise<string | undefined> {
  const fromEnv = process.env.OTA_GIT_COMMIT_HASH?.trim();
  if (fromEnv) return fromEnv;

  const result = await $`git rev-parse HEAD`.nothrow().quiet();
  if (result.exitCode !== 0) return undefined;
  const hash = result.stdout.toString().trim();
  return hash || undefined;
}

async function resolveMessage(): Promise<string> {
  if (process.env.OTA_MESSAGE !== undefined) {
    return process.env.OTA_MESSAGE.trim();
  }

  const result = await $`git log -1 --pretty=%B`.nothrow().quiet();
  if (result.exitCode !== 0) return "";
  const [firstLine = ""] = result.stdout.toString().trim().split("\n");
  return firstLine.trim();
}

function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / (1024 * 1024)).toFixed(2)} MB`;
}

function progressBar(ratio: number, width = 28): string {
  const clamped = Math.min(1, Math.max(0, ratio));
  const filled = Math.round(clamped * width);
  return `[${"=".repeat(filled)}${" ".repeat(width - filled)}] ${(clamped * 100).toFixed(1)}%`;
}

function formatHeaders(headers: Record<string, string>): string {
  return Object.entries(headers)
    .map(([k, v]) => `    ${k}: ${v}`)
    .join("\n");
}

interface ActiveUpload {
  key: string;
  label: string;
  putUrl: string;
  putHeaders: Record<string, string>;
  loaded: number;
  total: number;
}

interface UploadProgressState {
  totalFiles: number;
  completedFiles: number;
  completedBytes: number;
  totalBytes: number;
  active: Map<string, ActiveUpload>;
}

function uploadedBytes(state: UploadProgressState): number {
  let activeBytes = 0;
  for (const upload of state.active.values()) {
    activeBytes += upload.loaded;
  }
  return state.completedBytes + activeBytes;
}

function renderUploadProgress(state: UploadProgressState): string {
  const lines: string[] = [];

  const fileRatio =
    state.totalFiles > 0 ? state.completedFiles / state.totalFiles : 1;
  const byteRatio =
    state.totalBytes > 0 ? uploadedBytes(state) / state.totalBytes : 1;

  lines.push(
    `Uploading ${state.completedFiles}/${state.totalFiles} files · ${formatBytes(uploadedBytes(state))}/${formatBytes(state.totalBytes)}`,
  );
  lines.push(
    `Total  ${progressBar(byteRatio)}  (files ${progressBar(fileRatio, 20)})`,
  );
  lines.push("");

  if (state.active.size === 0 && state.completedFiles < state.totalFiles) {
    lines.push("Starting uploads…");
  } else {
    let slot = 1;
    for (const upload of state.active.values()) {
      const fileRatio = upload.total > 0 ? upload.loaded / upload.total : 1;
      lines.push(`[${slot}] ${upload.label}`);
      lines.push(`    key: ${upload.key}`);
      lines.push(`    PUT ${upload.putUrl}`);
      lines.push(formatHeaders(upload.putHeaders));
      lines.push(
        `    ${progressBar(fileRatio)}  ${formatBytes(upload.loaded)}/${formatBytes(upload.total)}`,
      );
      if (slot < state.active.size) lines.push("");
      slot++;
    }
  }

  return lines.join("\n");
}

function createProgressBody(
  buf: Buffer,
  onProgress: (loaded: number) => void,
): ReadableStream<Uint8Array> {
  let offset = 0;
  const chunkSize = 64 * 1024;
  return new ReadableStream({
    pull(controller) {
      if (offset >= buf.length) {
        controller.close();
        return;
      }
      const end = Math.min(offset + chunkSize, buf.length);
      controller.enqueue(buf.subarray(offset, end));
      offset = end;
      onProgress(offset);
    },
  });
}

async function collectAssets(
  dist: string,
  platform: Platform,
  metadata: Record<string, unknown>,
): Promise<{
  assets: PlanAssetItem[];
  buffersByKey: Map<string, Buffer>;
  pathsByKey: Map<string, string>;
}> {
  const { bundle, assets: metadataAssets } = parsePlatformMetadata(
    metadata,
    platform,
  );

  const specs: {
    relPath: string;
    contentType: string;
    fileExt: string;
  }[] = [
    {
      relPath: bundle,
      contentType: "application/javascript",
      fileExt: ".bundle",
    },
    ...metadataAssets.map((asset) => ({
      relPath: asset.path,
      contentType: contentTypeFromExt(asset.ext),
      fileExt: normalizeExt(asset.ext),
    })),
  ];

  const seen = new Set<string>();
  const uniqueSpecs = specs.filter((spec) => {
    if (seen.has(spec.relPath)) return false;
    seen.add(spec.relPath);
    return true;
  });

  const assets: PlanAssetItem[] = [];
  const buffersByKey = new Map<string, Buffer>();
  const pathsByKey = new Map<string, string>();

  for (const spec of uniqueSpecs) {
    const absPath = join(dist, spec.relPath);
    let buf: Buffer;
    try {
      buf = await readFile(absPath);
    } catch {
      throw new Error(
        `Asset not found: ${spec.relPath} (metadata.json fileMetadata.${platform})`,
      );
    }

    const key = md5(buf);
    assets.push({
      key,
      sha256: sha256B64url(buf),
      size: buf.length,
      contentType: spec.contentType,
      fileExt: spec.fileExt,
    });
    buffersByKey.set(key, buf);
    pathsByKey.set(key, spec.relPath);
  }

  return { assets, buffersByKey, pathsByKey };
}

async function adminFetch<T>(
  api: string,
  token: string,
  path: string,
  init: RequestInit = {},
): Promise<T> {
  const headers = new Headers(init.headers);
  headers.set("Authorization", `Bearer ${token}`);
  if (init.body !== undefined) {
    headers.set("Content-Type", "application/json");
  }

  const res = await fetch(`${api}/api/admin${path}`, { ...init, headers });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(
      `${init.method ?? "GET"} ${path} failed: ${res.status} ${text}`,
    );
  }
  return (await res.json()) as T;
}

async function uploadOne(
  item: PlanMissingItem,
  buf: Buffer,
  label: string,
  state: UploadProgressState,
  redraw: () => void,
): Promise<void> {
  const active: ActiveUpload = {
    key: item.key,
    label,
    putUrl: item.putUrl,
    putHeaders: item.putHeaders,
    loaded: 0,
    total: buf.length,
  };
  state.active.set(item.key, active);
  redraw();

  try {
    const res = await fetch(item.putUrl, {
      method: "PUT",
      headers: item.putHeaders,
      body: createProgressBody(buf, (loaded) => {
        active.loaded = loaded;
        redraw();
      }),
    });

    if (!res.ok) {
      const text = await res.text();
      throw new Error(`upload ${item.key} failed: ${res.status} ${text}`);
    }

    state.completedFiles += 1;
    state.completedBytes += buf.length;
  } finally {
    state.active.delete(item.key);
    redraw();
  }
}

async function uploadMissing(
  missing: PlanMissingItem[],
  buffersByKey: Map<string, Buffer>,
  pathsByKey: Map<string, string>,
): Promise<void> {
  if (missing.length === 0) return;

  const state: UploadProgressState = {
    totalFiles: missing.length,
    completedFiles: 0,
    completedBytes: 0,
    totalBytes: missing.reduce(
      (sum, item) => sum + (buffersByKey.get(item.key)?.length ?? 0),
      0,
    ),
    active: new Map(),
  };

  const redraw = () => logUpdate(renderUploadProgress(state));

  let nextIndex = 0;
  const worker = async (): Promise<void> => {
    while (true) {
      const index = nextIndex++;
      if (index >= missing.length) return;

      const item = missing[index]!;
      const buf = buffersByKey.get(item.key);
      if (!buf) {
        throw new Error(`local file not found for asset key ${item.key}`);
      }

      const label = pathsByKey.get(item.key) ?? item.key;
      await uploadOne(item, buf, label, state, redraw);
    }
  };

  const concurrency = Math.min(UPLOAD_CONCURRENCY, missing.length);
  try {
    redraw();
    await Promise.all(Array.from({ length: concurrency }, () => worker()));
    logUpdate(
      `Uploaded ${missing.length} asset(s) to object storage (${formatBytes(state.totalBytes)})`,
    );
    logUpdate.done();
  } catch (err) {
    logUpdate.done();
    throw err;
  }
}

async function main(): Promise<void> {
  const api = requireEnv("OTA_API").replace(/\/$/, "");
  const token = requireEnv("OTA_TOKEN");
  const slug = requireEnv("OTA_APP_SLUG");
  const platform = parsePlatform(requireEnv("OTA_PLATFORM"));
  const dist = requireEnv("OTA_DIST_DIR");
  const runtimeVersion = requireEnv("OTA_RUNTIME_VERSION");
  const [manifestMetadata, expoConfig, gitCommitHash, message] =
    await Promise.all([
      readJson(join(dist, "metadata.json")),
      readJson(join(dist, "expoConfig.json")),
      resolveGitCommitHash(),
      resolveMessage(),
    ]);

  const collected = await collectAssets(dist, platform, manifestMetadata);

  const payload: UploadPayload = {
    runtimeVersion,
    platform,
    manifestMetadata,
    expoConfig,
    message,
    assets: collected.assets,
  };
  if (gitCommitHash) {
    payload.gitCommitHash = gitCommitHash;
  }

  const plan = await adminFetch<PlanResp>(
    api,
    token,
    `/apps/${encodeURIComponent(slug)}/uploads/plan`,
    { method: "POST", body: JSON.stringify(payload) },
  );

  console.log(
    `Plan: ${plan.missing.length} to upload, ${plan.reuse.length} to reuse`,
  );

  if (plan.missing.length > 0) {
    await uploadMissing(
      plan.missing,
      collected.buffersByKey,
      collected.pathsByKey,
    );
  }

  const finalized = await adminFetch<FinalizeResp>(
    api,
    token,
    `/apps/${encodeURIComponent(slug)}/uploads/finalize`,
    { method: "POST", body: JSON.stringify(payload) },
  );

  console.log(
    `Finalized draft ${finalized.updateId} (${finalized.status}) manifest ${finalized.manifestUuid}`,
  );
  console.log("Draft is pending — publish it from the dashboard.");
}

export const publish = main;

if (import.meta.path === Bun.main) {
  main().catch((err: unknown) => {
    console.error(err instanceof Error ? err.message : err);
    process.exit(1);
  });
}
