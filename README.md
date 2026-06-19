# Expo OTA

A self-hosted [Expo Updates v1](https://docs.expo.dev/technical-specs/expo-updates-1/) server for teams that want to ship over-the-air (OTA) JavaScript updates without relying on EAS Update.

You run the server, point your Expo app at it, publish bundles from CI with a small CLI, and control when updates go live from a web dashboard.

## What you get

- **OTA updates for Expo / React Native apps** — clients fetch manifests and download immutable assets over HTTPS.
- **A publish workflow with a safety gate** — CI uploads create a _pending_ draft; an admin explicitly _publishes_ it before any device sees the update.
- **A management dashboard** — manage apps, updates, API tokens, code-signing keys, users, and audit logs; see basic rollout stats (manifest checks, success/fail counts, download timing).
- **A Bun-based publish CLI** — `plan → upload to object storage → finalize` in one script, suitable for CI pipelines.

```
┌─────────────┐     plan / finalize      ┌──────────────┐
│  CI / CLI   │ ───────────────────────►│  Admin API   │
└─────────────┘                          └──────┬───────┘
                                                │
┌─────────────┐     manifest / events           │  PostgreSQL
│ Expo client │ ◄──────────────────────────────┤
└─────────────┘                                 │
       │                                        ▼
       └──────── asset download ──────► Object storage (COS)
```

## Strengths

- **Protocol-first asset URLs** — assets are content-addressed by SHA-256. Once published, an asset URL never changes, which matches what `expo-updates` expects and avoids a class of cross-release mismatches seen in some other self-hosted servers.
- **Deliberate release control** — uploads do not go live immediately. You can review a pending update in the dashboard before publishing.
- **Multi-app on one deployment** — each app is isolated by `appSlug` in the manifest URL (`/api/apps/{appSlug}/manifest`).
- **Sensible auth model for small teams** — dashboard users sign in with JWT; CI uses per-app API tokens (`ota_pat_…`), not a single shared upload password.
- **Operational visibility** — manifest requests are logged server-side; clients can report `update_succeeded` / `update_failed` events for per-update stats.
- **Asset deduplication** — identical files across updates are stored once per app, which keeps storage and upload time down.
- **Audit trail** — administrative write actions are recorded.

## Trade-offs and limitations

Be aware of what this project intentionally does _not_ do:

- **No channel / branch / gradual rollout** — routing is `(appSlug, runtimeVersion, platform)` only. If you need EAS-style channels, percentage rollouts, or A/B buckets, look elsewhere or plan to extend this.
- **No `rollBackToEmbedded` directive** — rolling back means _republishing a previous update_ as a new release, not sending clients back to the embedded binary via the protocol directive.
- **Tencent Cloud COS storage** — production storage is built around Tencent COS with path-level public read for assets. There is no first-class S3/GCS/local adapter today.
- **Single trust domain** — flat admin access (no roles, no organizations, no multi-tenant isolation). Designed for one team running its own instance.
- **Manual publish step** — CI finalizes a draft; a human (or a separate automation calling the admin API) must publish it.
- **No asset compression on the wire** — gzip/brotli serving is not implemented in the current MVP.
- **No bundled CDN layer** — assets are served from COS directly. You can still put a CDN in front of COS yourself, but it is not wired in.
- **Younger and opinionated** — fewer features than mature alternatives, but with a narrower scope and stronger protocol guarantees in the areas it focuses on.

## Good fit

This project is a reasonable choice if you:

- Run a **small number of Expo apps** for one organization and want a **private OTA endpoint**.
- Already use or are willing to use **Tencent Cloud COS**.
- Want **immutable, content-addressed asset URLs** and a **draft-then-publish** workflow.
- Do not need **channel-based routing** or **gradual rollout** — different binaries via `runtimeVersion` is enough for your staging vs production split.
- Prefer a **real database** (PostgreSQL) for update history, audit logs, and admin state.
- Host **multiple apps** on a single server without separate deployments per app.

## Probably not a good fit

Consider other options if you:

- Need **EAS-compatible channel / branch workflows** or **percentage rollouts**.
- Want **S3, GCS, or local filesystem** storage without adapting the codebase.
- Need **multi-tenant SaaS** with per-customer isolation and RBAC.
- Require **`rollBackToEmbedded`** to force clients back to the embedded bundle.
- Want a **batteries-included CDN, Prometheus metrics, and official GitHub Actions** out of the box (expo-open-ota is stronger here).
- Want the **quickest possible single-app prototype** and can tolerate known protocol gaps (xavia-ota is simpler to try, with caveats below).

## Comparison with other self-hosted servers

The table below compares this project with two popular open-source alternatives. It reflects documented behavior and known issues as of project design time — verify against each project's current README before deciding.

|                         | **Expo OTA** (this repo)             | **[xavia-ota](https://github.com/xavia-io/xavia-ota)** | **[expo-open-ota](https://github.com/axelmarciano/expo-open-ota)** |
| ----------------------- | ------------------------------------ | ------------------------------------------------------ | ------------------------------------------------------------------ |
| **Stack**               | Go + PostgreSQL + Tencent COS        | Next.js + TypeScript                                   | Go + S3/GCS/local (no DB)                                          |
| **Multi-app**           | Yes (`appSlug`)                      | Effectively single-app                                 | Single Expo account / owner                                        |
| **Channel / branch**    | No                                   | No                                                     | Yes (Expo channel ↔ branch)                                        |
| **Gradual rollout**     | No                                   | No                                                     | Via channel mapping                                                |
| **Asset URL stability** | SHA-256 content-addressed, immutable | Asset URLs point at "latest" — known correctness risk  | Origin URLs lack `updateId` — known race risk; CDN path is better  |
| **Upload path**         | Plan → direct COS upload → finalize  | Multipart POST through API                             | Presigned URL to bucket                                            |
| **Go-live model**       | Pending draft → admin publishes      | Live on upload                                         | Mark uploaded / checked                                            |
| **Rollback story**      | Republish previous update            | Republish copy (not protocol rollback)                 | `rollBackToEmbedded` + republish                                   |
| **Dashboard**           | Vue 3 admin UI                       | Basic web UI                                           | SPA + optional Grafana                                             |
| **CLI / CI**            | Bun `publish.ts` script              | Shell helper                                           | `eoas` CLI + GitHub Action                                         |
| **Code signing**        | Per-app RSA keys                     | Supported, limited negotiation                         | Supported; `keyid` historically hardcoded                          |
| **Auth**                | JWT + per-app API tokens             | Shared upload key; admin APIs historically weak        | Expo OAuth (owner-scoped)                                          |
| **Observability**       | Dashboard stats + audit log          | Basic tracking                                         | Prometheus metrics                                                 |
| **Maturity / scope**    | Narrow, opinionated, newer           | Small, quick to try; several protocol gaps reported    | Broadest feature set; deploy with eyes open on known issues        |

### How to read this

- Choose **Expo OTA** when you want **correct immutable assets**, **multi-app hosting**, **PostgreSQL-backed admin**, and a **publish approval gate**, and you are fine without channels/rollout.
- Choose **expo-open-ota** when you need **channel/branch parity with EAS**, **CDN integration**, **metrics**, and a **mature CLI** — and you can accept its storage model and operational complexity.
- Choose **xavia-ota** when you want a **minimal Next.js deployment** for experimentation — but treat protocol correctness and auth hardening as your responsibility before production use.

Neither alternative nor this project is a drop-in replacement for every EAS Update feature. All three require you to operate and secure your own infrastructure.

## Quick start (overview)

### 1. Point your Expo app at your server

In `app.json` (or `app.config.*`):

```json
{
  "expo": {
    "updates": {
      "url": "https://ota.example.com/api/apps/{your-appSlug}/manifest"
    },
    "runtimeVersion": {
      "policy": "appVersion"
    }
  }
}
```

Rebuild the native app after changing `updates.url` or `runtimeVersion`.

### 2. Publish an update from CI

```bash
OTA_API=https://ota.example.com \
OTA_TOKEN=ota_pat_xxx \
OTA_APP_SLUG=my-app \
OTA_PLATFORM=ios \
OTA_DIST_DIR=./dist \
OTA_RUNTIME_VERSION=1.0.0 \
bun run cli/publish.ts
```

This creates a **pending** update. Open the dashboard and click **Publish** when you are ready for devices to receive it.

### 3. Deploy the server

Backend runs via Docker Compose; the dashboard is built as static files and served by your reverse proxy (Nginx, Caddy, etc.).

See **[DEPLOY.md](./DEPLOY.md)** for production setup, environment variables, and Nginx examples.

## Repository layout

```
server/       Go backend (protocol-api + admin-api)
dashboard/    Vue 3 management UI
cli/          Bun publish script for CI
docs/         Design docs, ADRs, competitor analysis
```

## License

Copyright (c) 2026 hoywu

This project is licensed under the MIT License — see [LICENSE](LICENSE) for the full text.
