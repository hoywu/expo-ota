# 实现文档

> 状态：已定稿 (accepted)

## 0. 术语速查

| 术语                 | 含义                                                                      |
| -------------------- | ------------------------------------------------------------------------- |
| **App**              | 一个 Expo/RN 应用（一个 dashboard 上的"项目"），由 `appSlug` 全局唯一标识 |
| **Runtime Version**  | 原生层兼容版本字符串，由 `expo export` 产物中的 `metadata.json` 给出      |
| **Update**           | 一次发布产物，包含 manifest、launch asset、普通 assets、metadata          |
| **Asset**            | 内容寻址的不可变文件（按 sha256 寻址）                                    |
| **Manifest**         | 描述一次 update 内容的 JSON 响应体                                        |
| **Client Event**     | 客户端主动上报的 `update_succeeded` / `update_failed` 事件                |
| **Manifest Request** | 服务端在 manifest 端点观察到的请求行                                      |

> 完整词汇表见 `server/CONTEXT.md`。

---

## 1. 范围与目标

### 1.1 In-Scope

- 完整实现 [Expo Updates v1 协议](https://docs.expo.dev/technical-specs/expo-updates-1/)（manifest 端点、directive、code signing、immutable asset URL）
- 多 App 支持，按 `appSlug` 在 URL 中隔离
- 同 App 内按 `runtimeVersion + platform` 路由最新 update
- 两阶段发布：plan → 直传 COS → finalize
- 历史 update 复制实现"republish previous"语义
- 落后 3 个版本后允许删除，附带孤儿 asset GC
- 客户端事件上报 + 服务端 manifest 请求观察，构成 dashboard 可观测性
- 管理员后台（Vue 3 + Nuxt UI）：App / Update / Token / Signing Key / User / Audit 管理
- 公网部署：HTTPS、登录限速、API 限速、协议端限速

### 1.2 Out-of-Scope（明确不做）

- **多租户 / Organization / RBAC**：内部使用，单一信任域
- **Channel / Branch / 灰度路由**：runtimeVersion 已是天然路由维度
- **协议 v0 兼容**：只支持 v1
- **资产预压缩**（gzip / brotli）：MVP 砍，后期再补
- **Webhook / 通知**：用户未要求
- **自定义 directive**：只实现 `noUpdateAvailable` 和 `rollBackToEmbedded`
- **多区域 / 跨地域部署**：单 region COS + 单实例 PostgreSQL
- **KMS 加密私钥**：私钥 AES-256-GCM 加密，密钥从 env 读
- **多 active key 并行轮换**：MVP 只保留每 App 一个 active key

---

## 2. 概念模型

```text
                          ┌──────────────────┐
                          │       App        │
                          │  (slug 不可改)   │
                          └────────┬─────────┘
                                   │ 1—*
                          ┌────────▼────────┐
                          │  RuntimeVersion  │
                          │ (version 字符串) │
                          └────────┬─────────┘
                                   │ 1—*
                          ┌────────▼────────┐
                          │     Update       │ ◀── 状态: pending / published
                          │                  │      软删: deleted_at
                          │                  │      回滚源: rolled_back_from
                          └────────┬─────────┘
                                   │ *—*
                          ┌────────▼────────┐
                          │     Asset        │  全局去重: (app_id, sha256)
                          │  (sha256 寻址)   │  存储: COS [prefix/]apps/{slug}/assets/{sha256}
                          └──────────────────┘

独立实体:
  User        - 管理员账号 (无角色, 全等价)
  ApiToken    - CLI/CI 发布用, 关联 app_id
  CodeSignKey - 每 App 仅 1 个 active key, 可保留 disabled 历史 key
  ManifestReq - 服务端观察的 manifest 请求行
  ClientEvent - 客户端上报的 success/fail
  AuditLog    - 所有管理写操作
```

### 2.1 关键不变量

- **Asset URL 不可变**：`manifest.assets[].url` 永远是 `https://<cos-domain>/[prefix/]apps/{appSlug}/assets/{sha256_base64url}`，其中 `prefix` 来自可选 `COS_KEY_PREFIX`
- **App 内 update 排序**：(app, runtimeVersion, platform) 流内只看 `status='published'` 且 `deleted_at IS NULL` 的 update，并按 `published_at DESC` 取最新一条为"当前"
- **代码签名**：`Updates.codeSigningCertificate` 配了的服务端必须签名；不配不签
- **Rollback = Republish**：复制历史 update 为新行，资产复用

---

## 3. 系统架构

```text
┌─────────────────────────────────────────────────────────────┐
│                    Nginx (反代 + HTTPS)                     │
│         TLS 终结 / gzip / 限速 / 路径路由                   │
└──────┬─────────────────┬──────────────────┬─────────────────┘
       │ /api/protocol/* │ /api/admin/*      │ /  (static SPA)
       │ /api/apps/*/    │                   │
       │   manifest      │                   │
       │ /api/apps/*/    │                   │
       │   events        │                   │
       ▼                ▼                  ▼
┌──────────────┐  ┌──────────────┐  ┌──────────────────┐
│ protocol-api │  │  admin-api   │  │  /var/www/       │
│  Go-Zero     │  │  Go-Zero     │  │   dashboard/     │
│  :8080       │  │  :8081       │  │  (CI rsync 推送) │
└──────┬───────┘  └──────┬───────┘  └────────┬─────────┘
       │                 │                   │ Nginx 直出
       └────────┬────────┘                   │
                │                            │
        ┌───────▼───────┐                   │
        │  PostgreSQL 18 │   单实例          │
        │  端口 5432     │                   │
        └────────────────┘                   │
                                             │
外部依赖:
  - 腾讯云 COS (对象存储, 整桶 private, `[prefix/]apps/*/assets/*` 路径级公有读)
  - (无 Redis, 见 ADR-0001 决议)
```

详细 Nginx 配置见 §12。

---

## 4. 数据库 Schema

### 4.1 完整表清单

```text
apps                  -- App 主表
runtime_versions      -- 预登记的 runtime version (可选, 也可懒创建)
updates               -- 一次发布
assets                -- 资产 (内容寻址, 全局去重)
update_assets         -- update ↔ asset 多对多
api_tokens            -- CLI/CI 发布 token
users                 -- 管理员账号
code_signing_keys     -- 每 App 一个签名 key
manifest_requests     -- 服务端观察的 manifest 请求
client_events         -- 客户端上报的成功/失败
audit_logs            -- 管理写操作审计
```

### 4.2 Schema (PostgreSQL 18, 命名 snake_case, UUID 主键, timestamptz 时间)

以下内容以 `server/db/migrations/00001_init.sql` 为准。相比此前设计稿，Schema 做了几处有意取舍，以便直接配合 `goctl model pg datasource` 生成并降低实现成本：

- 主键默认值统一改为 `uuidv7()`
- 去掉 `citext`、分区表、复合主键等会增加生成/维护成本的设计
- `update_assets`、`manifest_requests`、`client_events`、`audit_logs` 统一使用单列主键/identity 主键
- `apps` 不再反向保存 `code_signing_key_id`，active key 直接从 `code_signing_keys` 按 `app_id` 查询

```sql
CREATE TABLE users (
  id            uuid PRIMARY KEY DEFAULT uuidv7(),
  username      text UNIQUE NOT NULL,
  password_hash text NOT NULL,
  created_at    timestamptz NOT NULL DEFAULT now(),
  last_login_at timestamptz,
  disabled_at   timestamptz
);

CREATE TABLE apps (
  id                  uuid PRIMARY KEY DEFAULT uuidv7(),
  app_slug            text NOT NULL UNIQUE
                      CHECK (app_slug ~ '^[a-z0-9][a-z0-9-]{1,30}[a-z0-9]$'),
  name                text NOT NULL,
  description         text,
  created_at          timestamptz NOT NULL DEFAULT now(),
  deleted_at          timestamptz
);

CREATE TABLE code_signing_keys (
  id                    uuid PRIMARY KEY DEFAULT uuidv7(),
  app_id                uuid NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
  key_id                text NOT NULL,
  algorithm             text NOT NULL DEFAULT 'rsa-v1_5-sha256',
  public_key_pem        text NOT NULL,
  encrypted_private_key bytea NOT NULL,
  encryption_key_id     text NOT NULL,
  enabled               boolean NOT NULL DEFAULT true,
  created_at            timestamptz NOT NULL DEFAULT now(),
  disabled_at           timestamptz,
  UNIQUE (app_id, key_id)
);
CREATE INDEX code_signing_keys_app_id_idx ON code_signing_keys(app_id);

CREATE TABLE runtime_versions (
  id         uuid PRIMARY KEY DEFAULT uuidv7(),
  app_id     uuid NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
  version    text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (app_id, version)
);
CREATE INDEX runtime_versions_app_idx ON runtime_versions (app_id);

CREATE TABLE assets (
  id            uuid PRIMARY KEY DEFAULT uuidv7(),
  app_id        uuid NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
  sha256        bytea NOT NULL CHECK (octet_length(sha256) = 32),
  sha256_b64url text NOT NULL,
  size_bytes    bigint NOT NULL CHECK (size_bytes >= 0),
  content_type  text NOT NULL,
  file_ext      text,
  storage_key   text NOT NULL,
  created_at    timestamptz NOT NULL DEFAULT now(),
  UNIQUE (app_id, sha256)
);
CREATE INDEX assets_app_sha_b64url_idx ON assets (app_id, sha256_b64url);

CREATE TABLE updates (
  id                 uuid PRIMARY KEY DEFAULT uuidv7(),
  app_id             uuid NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
  runtime_version_id uuid NOT NULL REFERENCES runtime_versions(id),
  platform           text NOT NULL CHECK (platform IN ('ios','android')),
  manifest_uuid      uuid NOT NULL,
  launch_asset_id    uuid NOT NULL REFERENCES assets(id),
  status             text NOT NULL DEFAULT 'pending'
                     CHECK (status IN ('pending','published')),
  message            text,
  git_commit_hash    text,
  manifest_metadata  jsonb NOT NULL DEFAULT '{}',
  extra              jsonb NOT NULL DEFAULT '{}',
  expo_config        jsonb,
  manifest_snapshot  jsonb NOT NULL,
  rolled_back_from   uuid REFERENCES updates(id),
  created_at         timestamptz NOT NULL DEFAULT now(),
  published_at       timestamptz,
  deleted_at         timestamptz,
  CHECK (
    (status = 'pending' AND published_at IS NULL) OR
    (status = 'published' AND published_at IS NOT NULL)
  ),
  UNIQUE (app_id, manifest_uuid)
);
CREATE INDEX updates_latest_idx
  ON updates (app_id, runtime_version_id, platform, published_at DESC)
  WHERE deleted_at IS NULL AND status = 'published';
CREATE INDEX updates_app_created_idx
  ON updates (app_id, created_at DESC)
  WHERE deleted_at IS NULL;

CREATE TABLE update_assets (
  id         uuid PRIMARY KEY DEFAULT uuidv7(),
  update_id  uuid NOT NULL REFERENCES updates(id) ON DELETE CASCADE,
  asset_id   uuid NOT NULL REFERENCES assets(id),
  asset_key  text NOT NULL,
  file_ext   text,
  sort_order int NOT NULL DEFAULT 0,
  UNIQUE (update_id, asset_key)
);
CREATE INDEX update_assets_asset_idx ON update_assets (asset_id);

CREATE TABLE api_tokens (
  id           uuid PRIMARY KEY DEFAULT uuidv7(),
  app_id       uuid NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
  created_by   uuid NOT NULL REFERENCES users(id),
  name         text NOT NULL,
  token_hash   bytea NOT NULL UNIQUE,
  scopes       text[] NOT NULL DEFAULT ARRAY['publish']::text[],
  last_used_at timestamptz,
  expires_at   timestamptz,
  created_at   timestamptz NOT NULL DEFAULT now(),
  revoked_at   timestamptz
);
CREATE INDEX api_tokens_app_active_idx
  ON api_tokens (app_id) WHERE revoked_at IS NULL;

CREATE TABLE manifest_requests (
  id               bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  app_id           uuid NOT NULL,
  occurred_at      timestamptz NOT NULL DEFAULT now(),
  runtime_version  text NOT NULL,
  platform         text NOT NULL,
  device_id        text,
  served_update_id uuid,
  result           text NOT NULL
                   CHECK (result IN ('update','no_update','rollback','not_found','bad_request','not_acceptable','error')),
  http_status      smallint NOT NULL
);

CREATE TABLE client_events (
  id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  app_id          uuid NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
  occurred_at     timestamptz NOT NULL,
  received_at     timestamptz NOT NULL DEFAULT now(),
  event_id        uuid NOT NULL,
  event_type      text NOT NULL
                  CHECK (event_type IN ('update_succeeded','update_failed')),
  update_id       uuid,
  manifest_uuid   uuid,
  runtime_version text,
  platform        text,
  device_id       text NOT NULL,
  app_version     text,
  os_version      text,
  duration_ms     int,
  error_code      text,
  error_message   text
);
CREATE UNIQUE INDEX client_events_app_event_idx
  ON client_events (app_id, event_id);

CREATE TABLE audit_logs (
  id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  app_id        uuid REFERENCES apps(id) ON DELETE SET NULL,
  actor_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
  action        text NOT NULL,
  target_type   text,
  target_id     text,
  request_id    text,
  ip            text,
  user_agent    text,
  payload       jsonb NOT NULL DEFAULT '{}',
  occurred_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_logs_app_time_idx ON audit_logs (app_id, occurred_at DESC);
CREATE INDEX audit_logs_actor_time_idx ON audit_logs (actor_user_id, occurred_at DESC);
CREATE INDEX audit_logs_action_time_idx ON audit_logs (action, occurred_at DESC);
```

### 4.3 生成友好取舍

- `users.username` 改为普通 `text UNIQUE`，不依赖 `citext` 扩展；如需大小写不敏感，服务端在创建/登录时统一做规范化
- `update_assets` 改为独立 `id` 主键，并保留 `UNIQUE (update_id, asset_key)` 约束，便于 `goctl` 直接生成模型与通用 CRUD
- `manifest_requests` 与 `client_events` 改为普通表，主键分别为 identity 列；不默认启用月分区，也不需要 `partition-gc`
- migration 里保留了两张表的分区示例注释；如果未来数据量需要，再把主键改回带时间列的复合主键后启用 RANGE PARTITION
- `apps` 不再保存 `code_signing_key_id` 外键，避免双向依赖；签名时按 `app_id` 查询当前 active key
- `manifest_requests.app_id` / `served_update_id` 目前不做外键，保持协议写路径低耦合，避免观测写入被业务 FK 拖住
- `published_at` 与 `status` 成对约束：pending 时必须为空，published 时必须非空；这样才能支持 finalize 草稿和准时发布

---

## 5. 协议实现

### 5.1 Manifest 端点

`GET /api/apps/{appSlug}/manifest`

**请求头**（客户端必须带）：

```text
expo-protocol-version: 1
expo-platform: ios | android
expo-runtime-version: <string>
expo-current-update-id: <uuid>     -- 可选, 用于 noUpdateAvailable 判定
expo-embedded-update-id: <uuid>    -- 可选, 用于 rollback 判定
expo-expect-signature: sig, keyid="<keyid>", alg="rsa-v1_5-sha256"  -- 可选
accept: application/expo+json, application/json, multipart/mixed
accept-encoding: br, gzip          -- MVP 暂不响应压缩
```

**响应头**（所有响应都带）：

```text
expo-protocol-version: 1
expo-sfv-version: 0
expo-manifest-filters: branch="default"     -- MVP 固定 default
expo-server-defined-headers: {}             -- MVP 固定空
cache-control: private, max-age=0
content-type: application/expo+json | application/json | multipart/mixed; boundary=...
expo-signature: sig=:b64:, keyid="<keyid>", alg="rsa-v1_5-sha256"  -- 仅启用签名时
```

**响应体**：

- `accept` 命中 `multipart/mixed`：返回 `manifest` part（JSON） + 可选 `directive` part
- `accept` 命中 `application/expo+json` / `application/json`：返回单 JSON body；如结果是 directive，返回 204 No Content
- 都不命中：返回 406

**Manifest JSON 字段映射**：

| Manifest 字段      | 数据源                                                     |
| ------------------ | ---------------------------------------------------------- |
| `id`               | `updates.manifest_uuid`                                    |
| `createdAt`        | `updates.created_at` 格式化为 ISO 8601                     |
| `runtimeVersion`   | `runtime_versions.version`                                 |
| `launchAsset`      | `update_assets` JOIN `assets`，asset_key 指向 launch asset |
| `assets[]`         | 同上，join 出所有 assets                                   |
| `metadata`         | `updates.manifest_metadata`                                |
| `extra.expoClient` | `updates.expo_config`                                      |

**Manifest 协商算法**：

```text
1. 校验 expo-protocol-version == 1 (否则 400)
2. 校验 expo-platform in ('ios','android') (否则 400)
3. 校验 appSlug 存在且未 deleted (否则 404)
4. 查 runtime_version (懒创建, 第一次见)
5. 查 latest update:
     SELECT id, manifest_uuid, ... FROM updates
     WHERE app_id = $1
       AND runtime_version_id = $2
       AND platform = $3
       AND status = 'published'
       AND deleted_at IS NULL
     ORDER BY published_at DESC LIMIT 1
6. 若无 update:
     - 若 expo-current-update-id 存在, 返回 204 No Content
     - 否则返回 200 + multipart 空 body (零 part 视为 no-op)
7. 若 expo-current-update-id == latest.manifest_uuid:
     - protocol v1: 返回 noUpdateAvailable directive
8. 构造 manifest JSON
9. 若该 App 存在 enabled 且未 disabled 的 signing key:
     - 签名 JSON bytes (RSA-SHA256 PKCS#1 v1.5)
     - 写 expo-signature header
10. 写一行 manifest_requests (occurred_at, result, served_update_id)
11. 返回响应
```

### 5.2 Directives

#### noUpdateAvailable

```json
{
  "type": "noUpdateAvailable"
}
```

#### rollBackToEmbedded

**MVP 不通过服务端 API 触发**，因为我们没有 channel/branch 概念。如未来需要"全量强制回到 embedded"功能，可加一张 `app_directives` 表（schema 留空）。

**当前不实现**。如果需要，单独提需求。

### 5.3 资产响应

**不走本服务**。`manifest.assets[].url` 直接是 COS 公开读 URL：

```text
https://<cos-domain>/[prefix/]apps/{appSlug}/assets/{sha256_b64url}
```

**响应头**（由 COS 端配置）：

- `cache-control: public, max-age=31536000, immutable`
- `content-type: <manifest 中声明>`

**客户端管理**：

- 客户端发 manifest 请求，下载所有 `assets[].url`
- 校验每个 asset 的 base64url(sha256) 与 manifest 中 `hash` 字段
- 验证失败的资产会触发整次 update 失败

### 5.4 资产预压缩

**MVP 不做**。CLI 不上传 `.br` / `.gz`，服务端不返回 `content-encoding`，客户端收到未压缩 bundle。

**加回来时**（不在 MVP 范围）：

- CLI 在 plan 阶段同时算 `.br` / `.gz` 的 sha256，列在 plan 请求里
- 缺失时返回两组 pre-signed PUT URL
- Manifest 的 `assets[].url` 不变（不带后缀），服务端在响应时按 `accept-encoding` 返回
- COS 路径：`[prefix/]apps/{slug}/assets/{sha256}.br` 与 `[prefix/]apps/{slug}/assets/{sha256}.gz`

### 5.5 Code Signing

- 私钥：2048-bit RSA，存 DB 的 `encrypted_private_key` 是 AES-256-GCM 加密后的密文
- 加密密钥：32 字节，从 env `SIGNING_KEY_ENCRYPTION_KEY`（base64）读
- 签名：RSA-SHA256 PKCS#1 v1.5，签 manifest JSON 实际发送字节
- 算法：固定 `rsa-v1_5-sha256`
- 每 App 仅允许一个 active key；旧 key 可 disable 保留，不做多 active key 并行轮换
- 删除 key 前必须先 disable 24h（保护在线客户端）

签名实现要点：

```text
1. manifest JSON marshal 为 []byte
2. 若按 `app_id` 查到 enabled 且未 disabled 的 signing key:
   a. 取私钥: decrypt(encrypted_private_key) -> PEM
   b. sign(PEM, manifestBytes) -> sig
   c. base64.StdEncoding.EncodeToString(sig) -> sigB64
3. 写 manifest 响应:
   - multipart: 写 manifest part body = manifestBytes, part header expo-signature
   - JSON: 写 body = manifestBytes, response header expo-signature
4. 写完不再二次 marshal
```

### 5.6 客户端事件上报

`POST /api/apps/{appSlug}/events`

**请求体**（单条）：

```json
{
  "eventId": "<uuid>",
  "eventType": "update_succeeded" | "update_failed",
  "occurredAt": "2026-06-04T10:00:00.000Z",
  "updateId": "<uuid>",
  "manifestUuid": "<uuid>",
  "runtimeVersion": "1.0.0",
  "platform": "ios",
  "deviceId": "<stable uuid>",
  "appVersion": "1.0.0",
  "osVersion": "17.4",
  "durationMs": 4321,
  "errorCode": "ASSET_HASH_MISMATCH",
  "errorMessage": "..."
}
```

**响应**：

- 200 OK，body 空
- 400 参数错误（含 SQL 校验失败）
- 429 限速命中

**幂等**：当前 schema 已直接对 `(app_id, event_id)` 建唯一索引，可直接用数据库保证幂等：

```sql
INSERT INTO client_events (...)
VALUES (...)
ON CONFLICT (app_id, event_id) DO NOTHING;
```

客户端重试 = 服务端查到已存在行 = 跳过 = 不双计数。

`received_at` 由服务端 `now()` 写入，不接受客户端值。

**限速**：60 req/min/device（device_id 来自 body），兜底 60 req/min/IP。

---

## 6. 管理 API 列表

所有 `/api/admin/*` 端点需 `Authorization: Bearer <jwt-or-token>`，且属于以下两种之一：

- 管理员 JWT（从 `POST /api/admin/login` 获取，HS256，24h 过期）
- API Token（从 dashboard 创建，`token_hash` 存 DB；只允许访问所属 App 的发布链路，见 §6.5）

### 6.1 鉴权

| Method | Path                | 说明                                                                                 |
| ------ | ------------------- | ------------------------------------------------------------------------------------ |
| POST   | `/api/admin/login`  | `{username, password}` → `{accessToken, refreshToken, expiresIn}`                    |
| POST   | `/api/admin/refresh`| `{refreshToken}` → `{accessToken, refreshToken, expiresIn}`，无状态轮换，旧 token 到期前仍有效 |
| POST   | `/api/admin/logout` | no-op（前端丢 token），保留对称                                                      |
| GET    | `/api/admin/me`     | 探活 + 返回当前 user 信息                                                            |

### 6.2 App

| Method | Path                        | 说明                                          |
| ------ | --------------------------- | --------------------------------------------- |
| GET    | `/api/admin/apps`           | 列出（仅未删除）                              |
| POST   | `/api/admin/apps`           | 创建 `{appSlug, name, description}`           |
| GET    | `/api/admin/apps/{appSlug}` | 详情                                          |
| PATCH  | `/api/admin/apps/{appSlug}` | 更新 `name`, `description`（**slug 不可改**） |
| DELETE | `/api/admin/apps/{appSlug}` | 软删                                          |

### 6.3 Update

| Method | Path                                                    | 说明                                                     |
| ------ | ------------------------------------------------------- | -------------------------------------------------------- |
| GET    | `/api/admin/apps/{appSlug}/updates`                     | 列表，支持 `?platform=ios&runtimeVersion=1.0.0&limit=20` |
| GET    | `/api/admin/apps/{appSlug}/updates/{updateId}`          | 详情（含 manifest 预览 + 统计）                          |
| DELETE | `/api/admin/apps/{appSlug}/updates/{updateId}`          | 软删 + 异步 GC 孤儿 asset                                |
| POST   | `/api/admin/apps/{appSlug}/updates/{updateId}/publish`  | 将 pending update 发布为 published，并写 `published_at`  |
| POST   | `/api/admin/apps/{appSlug}/updates/{updateId}/rollback` | 复制为新 update，标记 `rolled_back_from`                 |
| POST   | `/api/admin/apps/{appSlug}/updates/cleanup`             | body `{keepLatestN: 3}`，批量软删 + GC                   |

### 6.4 上传

| Method | Path                                         | 说明  |
| ------ | -------------------------------------------- | ----- |
| POST   | `/api/admin/apps/{appSlug}/uploads/plan`     | 见 §7 |
| POST   | `/api/admin/apps/{appSlug}/uploads/finalize` | 见 §7 |

### 6.5 API Token

API Token 是给 CLI/CI 发布使用的 App 级长期凭据，不等价于管理员 JWT。当前只支持 `publish` scope：

- token 形态：`ota_pat_<32 chars base62>`，DB 只保存 `sha256(token)`。
- token 绑定一个 `app_id`；请求 URL 中的 `{appSlug}` 必须解析到同一个 App。
- token 必须未撤销、未过期，且 `scopes` 包含 `publish`。
- token 只允许访问发布链路端点：
  - `POST /api/admin/apps/{appSlug}/uploads/plan`
  - `POST /api/admin/apps/{appSlug}/uploads/finalize`
  - `POST /api/admin/apps/{appSlug}/updates/{updateId}/publish`
- token 不能访问 App/User/Signing Key/Audit/Token 管理、删除、rollback、cleanup 等管理员操作；这些操作必须使用管理员 JWT。
- 服务端验证通过后会临时桥接为短期 access JWT 进入现有 go-zero JWT 链路，但桥接前已完成 App + scope + endpoint 授权。

| Method | Path                                        | 说明                                    |
| ------ | ------------------------------------------- | --------------------------------------- |
| GET    | `/api/admin/apps/{appSlug}/api-tokens`      | 列表（不返回明文）                      |
| POST   | `/api/admin/apps/{appSlug}/api-tokens`      | 创建，返回明文一次 `{name, expiresAt?}` |
| DELETE | `/api/admin/apps/{appSlug}/api-tokens/{id}` | 撤销（revoked_at = now）                |

### 6.6 Code Signing

| Method | Path                                             | 说明                               |
| ------ | ------------------------------------------------ | ---------------------------------- |
| GET    | `/api/admin/apps/{appSlug}/signing-key`          | 详情（含公钥）                     |
| POST   | `/api/admin/apps/{appSlug}/signing-key/generate` | 服务端生成 keypair                 |
| POST   | `/api/admin/apps/{appSlug}/signing-key/import`   | 导入完整 keypair（公钥 + 加密后的私钥） |
| PATCH  | `/api/admin/apps/{appSlug}/signing-key`          | `{enabled: true/false}`            |
| DELETE | `/api/admin/apps/{appSlug}/signing-key`          | 真删（必须先 disable 24h）         |

### 6.7 用户

| Method | Path                                 | 说明                                        |
| ------ | ------------------------------------ | ------------------------------------------- |
| GET    | `/api/admin/users`                   | 列表                                        |
| POST   | `/api/admin/users`                   | 创建 `{username, password}`（密码强度校验） |
| PATCH  | `/api/admin/users/{userId}/password` | 改密（admin 可改任意用户）                  |
| POST   | `/api/admin/users/{userId}/disable`  | 禁用                                        |
| POST   | `/api/admin/users/{userId}/enable`   | 启用                                        |

### 6.8 Audit Log

| Method | Path                                   | 说明                                          |
| ------ | -------------------------------------- | --------------------------------------------- |
| GET    | `/api/admin/apps/{appSlug}/audit-logs` | 列表，支持 `?action=&actor=&from=&to=&limit=` |

### 6.9 健康检查

| Method | Path       | 说明                  |
| ------ | ---------- | --------------------- |
| GET    | `/healthz` | 进程存活              |
| GET    | `/readyz`  | DB ping OK 才返回 200 |

---

## 7. 发布流程

### 7.1 plan

`POST /api/admin/apps/{appSlug}/uploads/plan`

**请求体**：

```json
{
  "runtimeVersion": "1.0.0",
  "platform": "ios",
  "manifestMetadata": { "...metadata.json content..." },
  "expoConfig": { "...expoConfig.json content..." },
  "message": "fix checkout",
  "gitCommitHash": "abc123",
  "assets": [
    {
      "key": "bunch of hex md5",
      "sha256": "<base64url, 43 chars>",
      "size": 12345,
      "contentType": "image/png",
      "fileExt": ".png"
    }
  ]
}
```

**服务端处理**：

1. 校验 appSlug / runtimeVersion / platform
2. 对每个 asset 查 `assets` 表：
   - 已存在 (app_id, sha256) → 加入 `reuse[]`
   - 不存在 → 生成 COS pre-signed PUT URL（15 min 过期），加入 `missing[]`
3. 不写任何持久化状态

**响应**：

```json
{
  "missing": [
    {
      "key": "...",
      "sha256": "...",
      "size": 12345,
      "contentType": "image/png",
      "putUrl": "https://cos.example.com/...?sign=...",
      "putHeaders": { "Content-MD5": "...", "x-cos-content-sha256": "..." },
      "finalUrl": "https://cos.example.com/[prefix/]apps/{slug}/assets/4nGjshg..."
    }
  ],
  "reuse": [{ "key": "...", "sha256": "...", "finalUrl": "..." }]
}
```

### 7.2 直传 COS

CLI 端用 `cos-nodejs-sdk-v5`（Bun/Node）或 AWS SDK 兼容模式，对每个 missing 项发 PUT：

```ts
await fetch(missing[i].putUrl, {
  method: "PUT",
  headers: missing[i].putHeaders,
  body: fileContent,
});
```

**不走本服务**。失败的资源下次重传。

### 7.3 finalize

`POST /api/admin/apps/{appSlug}/uploads/finalize`

**请求体**：与 plan 相同（带 metadata + assets 列表），不依赖 plan 的响应 id。

**服务端处理**（一个 PostgreSQL 事务）：

1. 对 plan/missing 中的每个 asset：
   - HEAD COS 对象 → 必须存在 + Content-Length == size + 自算 sha256 一致
   - 失败 → 400，body 列出不一致的 key
2. INSERT assets（ON CONFLICT DO NOTHING）
3. 懒创建 runtime_versions 行（若不存在）
4. 算 manifest 的 `manifest_uuid`（见 §7.4）
5. 构造 manifest JSON 并 marshal 一次（缓存到 `manifest_snapshot`）
6. INSERT update（status='pending'）
7. INSERT update_assets
8. 写 audit_log（`action='finalize_update'`）
9. 返回 pending update；**不自动 publish**

**响应**：

```json
{
  "updateId": "<uuid>",
  "manifestUuid": "<uuid>",
  "status": "pending",
  "createdAt": "2026-06-04T10:00:00.000Z"
}
```

### 7.4 publish

`POST /api/admin/apps/{appSlug}/updates/{updateId}/publish`

**用途**：管理员在 dashboard 点击"发布"后，手动将一个已 finalize 的 `pending` update 切到 `published`。

**服务端处理**：

1. 校验 update 属于当前 app，且 `status='pending'`
2. 校验其 manifest / assets / launch asset 均已完整落库
3. `UPDATE updates SET status='published', published_at=now() WHERE id=$1 AND status='pending'`
4. 写 audit_log（`action='publish_update'`）
5. 失效相关缓存（Q5 暂未启用，POSTHOLE）

**响应**：

```json
{
  "updateId": "<uuid>",
  "manifestUuid": "<uuid>",
  "status": "published",
  "publishedAt": "2026-06-04T10:30:00.000Z"
}
```

### 7.5 manifest_uuid 生成

`manifest_uuid` 在 finalize 时**一次性计算并持久化**（不再每次请求重算）。算法：

```text
sha256(canonicalManifestJSON)
  -> take first 16 bytes
  -> format as UUID v4 layout (8-4-4-4-12)
```

`canonicalManifestJSON` = 对 manifest 字段按固定 key 序序列化。Go 用 `encoding/json` 配 sort.Slice + map[string]interface{} 转换，或手写 marshal。

### 7.6 发布脚本（cli/publish.ts）

```ts
#!/usr/bin/env bun
// Usage:
//   OTA_API=https://ota.example.com \
//   OTA_TOKEN=ota_pat_xxx \
//   OTA_APP_SLUG=my-app \
//   OTA_PLATFORM=ios \
//   OTA_DIST_DIR=./dist \
//   OTA_RUNTIME_VERSION=1.0.0 \
//   OTA_MESSAGE="fix checkout" \
//   bun run cli/publish.ts

import { readdir, readFile } from "node:fs/promises";
import { join, relative, extname } from "node:path";
import { createHash } from "node:crypto";

const API = process.env.OTA_API!;
const TOKEN = process.env.OTA_TOKEN!;
const SLUG = process.env.OTA_APP_SLUG!;
const PLATFORM = process.env.OTA_PLATFORM as "ios" | "android";
const DIST = process.env.OTA_DIST_DIR!;
const RV = process.env.OTA_RUNTIME_VERSION!;
const MSG = process.env.OTA_MESSAGE ?? "";

async function walk(dir: string): Promise<string[]> {
  const out: string[] = [];
  for (const e of await readdir(dir, { withFileTypes: true })) {
    const p = join(dir, e.name);
    if (e.isDirectory()) out.push(...(await walk(p)));
    else out.push(p);
  }
  return out;
}

function md5(buf: Buffer): string {
  return createHash("md5").update(buf).digest("hex");
}

function sha256B64url(buf: Buffer): string {
  return createHash("sha256").update(buf).digest("base64url");
}

function mime(p: string): string {
  const ext = extname(p).toLowerCase();
  const m: Record<string, string> = {
    ".js": "application/javascript",
    ".hbc": "application/javascript",
    ".png": "image/png",
    ".jpg": "image/jpeg",
    ".jpeg": "image/jpeg",
    ".webp": "image/webp",
    ".ttf": "font/ttf",
    ".otf": "font/otf",
    ".json": "application/json",
  };
  return m[ext] ?? "application/octet-stream";
}

async function main() {
  // 1. 读 metadata + expoConfig
  const meta = JSON.parse(await readFile(join(DIST, "metadata.json"), "utf8"));
  const expo = JSON.parse(
    await readFile(join(DIST, "expoConfig.json"), "utf8"),
  );

  // 2. 收集 assets
  const files = (await walk(DIST))
    .map((p) => relative(DIST, p))
    .filter((p) => p !== "metadata.json" && p !== "expoConfig.json");

  const assets = await Promise.all(
    files.map(async (f) => {
      const buf = await readFile(join(DIST, f));
      return {
        key: md5(buf),
        sha256: sha256B64url(buf),
        size: buf.length,
        contentType: mime(f),
        fileExt: extname(f),
      };
    }),
  );

  // 3. plan
  const planRes = await fetch(`${API}/api/admin/apps/${SLUG}/uploads/plan`, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${TOKEN}`,
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      runtimeVersion: RV,
      platform: PLATFORM,
      manifestMetadata: meta,
      expoConfig: expo,
      message: MSG,
      assets,
    }),
  });
  if (!planRes.ok)
    throw new Error(`plan failed: ${planRes.status} ${await planRes.text()}`);
  const plan = (await planRes.json()) as {
    missing: Array<{
      key: string;
      putUrl: string;
      putHeaders: Record<string, string>;
    }>;
    reuse: Array<{ key: string; finalUrl: string }>;
  };

  console.log(
    `Plan: ${plan.missing.length} to upload, ${plan.reuse.length} to reuse`,
  );

  // 4. 直传 missing
  await Promise.all(
    plan.missing.map(async (m) => {
      // 通过 key 反查原文件路径 (key 是 md5, 需另存 map)
      // 简化: 重传时 client 端按 key 匹配 file
      const file = files.find(
        (f) => md5(await readFile(join(DIST, f))) === m.key,
      );
      if (!file) throw new Error(`local file not found for key ${m.key}`);
      const buf = await readFile(join(DIST, file));
      const res = await fetch(m.putUrl, {
        method: "PUT",
        headers: m.putHeaders,
        body: buf,
      });
      if (!res.ok) throw new Error(`upload ${m.key} failed: ${res.status}`);
    }),
  );

  // 5. finalize（只生成 pending draft，不自动发布）
  const finRes = await fetch(`${API}/api/admin/apps/${SLUG}/uploads/finalize`, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${TOKEN}`,
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      runtimeVersion: RV,
      platform: PLATFORM,
      manifestMetadata: meta,
      expoConfig: expo,
      message: MSG,
      assets,
    }),
  });
  if (!finRes.ok)
    throw new Error(`finalize failed: ${finRes.status} ${await finRes.text()}`);
  const fin = (await finRes.json()) as {
    updateId: string;
    manifestUuid: string;
    status: "pending";
  };
  console.log(
    `Finalized draft ${fin.updateId} (${fin.status}) (manifest ${fin.manifestUuid})`,
  );
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
```

---

## 8. 客户端集成指引

应用端（iOS/Android）的 `expo-updates` 客户端需要配置/打点的内容：

### 8.1 app.json 配置

```json
{
  "expo": {
    "updates": {
      "url": "https://ota.example.com/api/apps/{your-appSlug}/manifest",
      "channel": "default"
    },
    "runtimeVersion": {
      "policy": "sdkVersion"
    }
  }
}
```

### 8.2 deviceId 生成

```ts
import AsyncStorage from "@react-native-async-storage/async-storage";

const DEVICE_ID_KEY = "@expo_ota_device_id_v1";

export async function getDeviceId(): Promise<string> {
  let id = await AsyncStorage.getItem(DEVICE_ID_KEY);
  if (!id) {
    id = crypto.randomUUID();
    await AsyncStorage.setItem(DEVICE_ID_KEY, id);
  }
  return id;
}
```

### 8.3 事件上报代码

```ts
import * as Updates from "expo-updates";
import { getDeviceId } from "./deviceId";

const API = "https://ota.example.com";

type ReportEvent = {
  eventId: string;
  eventType: "update_succeeded" | "update_failed";
  occurredAt: string;
  updateId?: string;
  manifestUuid?: string;
  runtimeVersion: string;
  platform: "ios" | "android";
  deviceId: string;
  appVersion: string;
  osVersion: string;
  durationMs: number;
  errorCode?: string;
  errorMessage?: string;
};

async function report(ev: ReportEvent): Promise<void> {
  for (let i = 0; i < 3; i++) {
    try {
      await fetch(`${API}/api/apps/${APP_SLUG}/events`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(ev),
        keepalive: true,
      });
      return;
    } catch (_) {
      await new Promise((r) => setTimeout(r, 500 * 2 ** i));
    }
  }
  // 静默失败
}

export async function checkAndReport(): Promise<void> {
  const start = Date.now();
  const deviceId = await getDeviceId();
  const platform = Platform.OS as "ios" | "android";
  const appVersion = (await Application.nativeApplicationVersion) ?? "";
  const osVersion = String(Platform.Version);
  const runtimeVersion = Updates.runtimeVersion;

  try {
    const check = await Updates.checkForUpdateAsync();
    if (!check.isAvailable) return;

    await Updates.fetchUpdateAsync();
    const updateId = Updates.updateId;
    const manifestUuid = Updates.manifest?.id;

    await report({
      eventId: crypto.randomUUID(),
      eventType: "update_succeeded",
      occurredAt: new Date().toISOString(),
      updateId: updateId ?? undefined,
      manifestUuid: manifestUuid ?? undefined,
      runtimeVersion,
      platform,
      deviceId,
      appVersion,
      osVersion,
      durationMs: Date.now() - start,
    });
  } catch (err: any) {
    await report({
      eventId: crypto.randomUUID(),
      eventType: "update_failed",
      occurredAt: new Date().toISOString(),
      runtimeVersion,
      platform,
      deviceId,
      appVersion,
      osVersion,
      durationMs: Date.now() - start,
      errorCode: err?.code ?? "UNKNOWN",
      errorMessage: String(err?.message ?? err),
    });
  }
}
```

调用点：放在 `App.tsx` 顶层 + `useEffect(() => { checkAndReport() }, [])` 即可。

### 8.4 Code Signing 客户端配置

1. 从 dashboard `/apps/{slug}/signing-key` 下载公钥 PEM
2. 用 `npx expo-build-helpers generate-code-signing-cert` 转成 cert 格式（或用 `expo-updates` 提供的工具）
3. 配置 `app.json`：
   ```json
   {
     "expo": {
       "updates": {
         "codeSigningCertificate": "-----BEGIN CERTIFICATE-----\n...",
         "codeSigningMetadata": {
           "keyid": "main",
           "alg": "rsa-v1_5-sha256"
         }
       }
     }
   }
   ```
4. 重新 build / archive

---

## 9. 回滚与删除

### 9.1 Rollback（复制旧 update 为最新）

`POST /api/admin/apps/{appSlug}/updates/{updateId}/rollback`

**行为**：

1. 读源 update
2. 在 `updates` 插新行：
   - 所有字段从源 update 复制
   - `manifest_uuid` 重新计算（按 §7.4）
   - `launch_asset_id` 复用源
   - `manifest_snapshot` 重新构造
   - `rolled_back_from = <源 update id>`
   - `created_at = now()`
3. 复制 `update_assets` 关联（同 asset_id）
4. 新行默认 `status='pending'`、`published_at=NULL`，需管理员再次点击发布
5. 写 audit_log

### 9.2 单条删除

`DELETE /api/admin/apps/{appSlug}/updates/{updateId}`

**前置校验**：该 update 在其 stream 内 `rnk > 3`（即至少落后 3 个版本），否则 400。

**行为**：

1. `UPDATE updates SET deleted_at = now() WHERE id = $1`
2. 异步队列：找出孤儿 asset（不再被任何未删 update 引用），逐个删 COS 对象 + 删 assets 行

### 9.3 批量清理

`POST /api/admin/apps/{appSlug}/updates/cleanup` body `{keepLatestN: 3}`

**行为**：

1. 用 CTE 给每个 stream 内已发布 update 按 `published_at DESC` 算 rank
2. `rnk > keepLatestN` 的全部软删
3. 收集全部孤儿 asset，异步 GC

### 9.4 流定义

"流" = `(app_id, runtime_version, platform)`。每个流独立保留最新 N 条已发布 update；pending draft 不参与线上 latest 选择。

---

## 10. 可观测性

### 10.1 数据流

```text
客户端                                服务端
  │                                     │
  │  POST /api/apps/{slug}/events       │
  │  { eventType: update_succeeded,     │ → INSERT client_events
  │    durationMs: 4321, ... }          │
  │                                     │
  │                                     │
  │  GET /api/apps/{slug}/manifest      │
  │  expo-runtime-version: 1.0.0        │ → INSERT manifest_requests
  │  expo-platform: ios                 │   (served_update_id, result)
  │                                     │
  ▼                                    ▼
```

### 10.2 Dashboard 统计查询

**Update 详情页 "统计卡"**（时间段 [t1, t2]）：

```sql
WITH req AS (
  SELECT COUNT(DISTINCT device_id) AS requested_devices
  FROM manifest_requests
  WHERE app_id = $1
    AND served_update_id = $2
    AND occurred_at BETWEEN $3 AND $4
),
ev AS (
  SELECT
    event_type,
    COUNT(DISTINCT device_id) AS devices,
    MIN(duration_ms) AS min_ms,
    MAX(duration_ms) AS max_ms,
    AVG(duration_ms)::int AS avg_ms
  FROM client_events
  WHERE app_id = $1
    AND update_id = $2
    AND received_at BETWEEN $3 AND $4
  GROUP BY event_type
)
SELECT
  COALESCE(req.requested_devices, 0) AS requested_devices,
  COALESCE(ev_succeeded.devices, 0)  AS succeeded_devices,
  COALESCE(ev_failed.devices, 0)     AS failed_devices,
  ev_succeeded.min_ms, ev_succeeded.max_ms, ev_succeeded.avg_ms
FROM (SELECT 0) z
LEFT JOIN req ON true
LEFT JOIN ev ev_succeeded ON ev_succeeded.event_type = 'update_succeeded'
LEFT JOIN ev ev_failed    ON ev_failed.event_type    = 'update_failed';
```

6 个数字卡：请求设备数 / 成功设备数 / 失败设备数 / 最快 ms / 最慢 ms / 平均 ms。

时间范围选择器：最近 1h / 24h / 7d / 30d / 自定义。

### 10.3 监控告警

MVP 不接 Prometheus/Grafana。日志走 stdout JSON 格式，由部署环境的日志收集（k8s/Loki）接：

```json
{
  "ts": "2026-06-04T10:00:00.000Z",
  "level": "info",
  "msg": "manifest served",
  "request_id": "...",
  "app_slug": "my-app",
  "runtime_version": "1.0.0",
  "platform": "ios",
  "result": "update",
  "served_update_id": "...",
  "duration_ms": 18,
  "signed": true
}
```

---

## 11. 鉴权与安全

### 11.1 鉴权三层

1. **公网端**（manifest / events）：无需鉴权，靠 Nginx 限速
2. **管理 API**：管理员 JWT（完整管理权限）或 API Token（Bearer，App 级发布权限）
3. **首次部署**：env 注入 `INITIAL_ADMIN_USERNAME` / `INITIAL_ADMIN_PASSWORD`，服务首次启动时若 users 表为空则创建

API Token 的权限边界见 §6.5。它只覆盖 CI 发布所需的 plan / finalize / publish，不允许执行用户管理、签名 key 管理、回滚、删除、清理或跨 App 操作。

### 11.2 密码策略

- bcrypt cost = 12
- 改密 / 创建时强校验：≥10 字符 + 字母 + 数字 + 非纯字典
- 不存明文，不在日志输出

### 11.3 限速

| 端点                            | 限速       | Key                                |
| ------------------------------- | ---------- | ---------------------------------- |
| `POST /api/admin/login`         | 5 req/min  | IP                                 |
| `GET /api/apps/{slug}/manifest` | 30 req/min | `expo-device-id` header（兜底 IP） |
| `POST /api/apps/{slug}/events`  | 60 req/min | body.deviceId（兜底 IP）           |
| 其他 `/api/admin/*`             | 60 req/min | user_id / token_id                 |

### 11.4 安全响应头（Nginx 层）

- `Strict-Transport-Security: max-age=31536000; includeSubDomains`
- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`
- `Referrer-Policy: no-referrer`

### 11.5 失败审计

`audit_logs` 表记录所有管理写操作（finalize_update / publish_update / rollback_update / delete_update / cleanup_updates / create_app / create_user / change_password / ...），含 actor_user_id、IP、UA、payload、occurred_at。

`POST /api/admin/login` 失败时也写一条 audit_log（`action='login_failed'`，无 actor_user_id）。

### 11.6 CORS

不需要。客户端（native）不受 CORS 限制，dashboard 与 admin-api 同源（Nginx 同 server_name 反代）。

---

## 12. 部署清单

### 12.1 进程与目录

```text
expo-ota/
├── server/
│   ├── api/protocol/         # go-zero, 端口 8080
│   ├── api/admin/            # go-zero, 端口 8081
│   ├── internal/             # 业务代码 (待实现)
│   ├── db/migrations/        # goose SQL
│   ├── cmd/asset-gc/         # 孤儿 asset GC cron
│   └── deploy/
│       ├── docker-compose.yml
│       ├── nginx.conf
│       ├── dashboard/        # CI rsync 推送静态文件到这里
│       └── .env (不入 git)
├── cli/
│   └── publish.ts            # Bun 发布脚本
├── dashboard/                # Vue 3 + Nuxt UI 源码 (待建)
├── docs/
│   ├── IMPLEMENTATION.md     # 本文件
│   ├── CONTEXT-MAP.md        # 词汇表索引
│   └── adr/                  # ADR
└── server/
    └── CONTEXT.md            # server 上下文词汇表
```

### 12.2 docker-compose.yml

```yaml
services:
  db:
    image: postgres:18-alpine
    restart: unless-stopped
    shm_size: 256mb
    environment:
      POSTGRES_USER: ${PG_USER:-admin}
      POSTGRES_PASSWORD: ${PG_PASSWORD:?required}
      POSTGRES_DB: ${PG_DB:-expo_ota}
      TZ: Asia/Shanghai
    ports: ["127.0.0.1:5432:5432"]
    volumes: ["pg_data:/var/lib/postgresql"]
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U $$POSTGRES_USER -d $$POSTGRES_DB"]
      interval: 5s
      timeout: 5s
      retries: 5

  protocol-api:
    build: { context: ../, dockerfile: server/deploy/Dockerfile.api }
    restart: unless-stopped
    environment:
      LISTEN_PORT: 8080
    env_file: .env
    depends_on:
      db: { condition: service_healthy }
    expose: ["8080"]

  admin-api:
    build: { context: ../, dockerfile: server/deploy/Dockerfile.api }
    restart: unless-stopped
    command: ["/app/admin-api", "-f", "/app/etc/admin-api.yaml"]
    environment:
      LISTEN_PORT: 8081
    env_file: .env
    depends_on:
      db: { condition: service_healthy }
    expose: ["8081"]

  nginx:
    image: nginx:1.27-alpine
    restart: unless-stopped
    ports: ["80:80", "443:443"]
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf:ro
      - ./certs:/etc/nginx/certs:ro
      - ./dashboard:/var/www/dashboard:ro # CI rsync 推送的静态文件
    depends_on: [protocol-api, admin-api]

volumes:
  pg_data:
    name: expo_ota_pg_data
    driver: local
```

### 12.3 环境变量

`server/deploy/.env`（不入 git）：

```bash
# —— 通用 ——
SERVER_ENV=production
LOG_LEVEL=info
LISTEN_HOST=0.0.0.0

# —— PostgreSQL ——
PG_USER=admin
PG_PASSWORD=ChangeMeSecurely123!
PG_DB=expo_ota
DB_URL=postgresql://${PG_USER}:${PG_PASSWORD}@db:5432/${PG_DB}

# —— 腾讯云 COS ——
COS_SECRET_ID=AKIDxxxxxxxxxxxxxxxx
COS_SECRET_KEY=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
COS_REGION=ap-guangzhou
COS_BUCKET=expo-ota-12345-1250000000
COS_DOMAIN=                                 # 可选, 自定义 COS 源站域名
COS_KEY_PREFIX=                             # 可选, 对象 key 前缀, 如 expo-ota

# —— JWT (HS256, 32+ 字节 base64) ——
JWT_SECRET=<base64-of-32-random-bytes>

# —— 签名私钥加密密钥 (AES-256, 32 字节 base64) ——
SIGNING_KEY_ENCRYPTION_KEY=<base64-of-32-random-bytes>

# —— 首次启动兜底管理员 ——
INITIAL_ADMIN_USERNAME=admin
INITIAL_ADMIN_PASSWORD=ChangeMeNow123!

# —— 限速阈值 (req/min) ——
RATE_LIMIT_MANIFEST_PER_DEVICE=30
RATE_LIMIT_EVENTS_PER_DEVICE=60
RATE_LIMIT_ADMIN_LOGIN_PER_IP=5
RATE_LIMIT_ADMIN_API_PER_USER=60

```

**部署后立刻**：

- 改 `INITIAL_ADMIN_PASSWORD`
- 把 secrets 移到密钥管理（Vaul、AWS SM 等），不要让 `.env` 长期存在

### 12.4 首次部署步骤

```bash
# 1. 准备 .env
cp server/deploy/.env.example server/deploy/.env
# 编辑填入真实值, 生成 JWT_SECRET / SIGNING_KEY_ENCRYPTION_KEY:
openssl rand -base64 32   # 各跑两次

# 2. 创建 COS 桶
#    - 整桶 private
#    - 在 COS 控制台配置路径 [COS_KEY_PREFIX/]apps/*/assets/* 为公有读
#    - 不开 CORS, 不开 lifecycle

# 3. 启 DB
docker compose -f server/deploy/docker-compose.yml up -d db

# 4. 跑 migration
make migrate

# 5. 启全部
docker compose -f server/deploy/docker-compose.yml up -d

# 6. 验证
curl https://ota.example.com/healthz
curl https://ota.example.com/readyz

# 7. 改密码
# 浏览器打开 https://ota.example.com/login
# 用 INITIAL_ADMIN_USERNAME / INITIAL_ADMIN_PASSWORD 登录
# 进入 /admin/users 改密

# 8. 推送 dashboard 静态文件 (CI 流程, 后续由 CI 自动)
cd dashboard
bun install
bun run build                    # vite build -> dist/
rsync -avz --delete dist/ server:/opt/expo-ota/server/deploy/dashboard/
# (nginx 容器挂载了该目录, 重新加载配置无需重启容器)
docker exec expo-ota-nginx-1 nginx -s reload

# 9. 第一次发布
#   a) dashboard /apps/new 创建 app, 拿到 appSlug
#   b) dashboard /apps/{slug}/tokens 创建 API token, 拷贝明文
#   c) 在 CI 配置 OTA_TOKEN env
#   d) 跑 cli/publish.ts 验证
```

### 12.5 Nginx 配置

完整配置见 [`server/deploy/nginx.conf`](../../server/deploy/nginx.conf)，按 §5/§6 路径路由 + §11.4 安全头 + §11.3 限速。TLS 证书用 certbot 或公司内部 CA。

---

## 13. 开发路线图

按依赖关系排序：

| Milestone               | 目标                                                                                        | 估时 |
| ----------------------- | ------------------------------------------------------------------------------------------- | ---- |
| **M0 基础设施**         | docker-compose 起 PG / 写好 .env / 建 COS 桶 / 配好 Nginx / 写好 `server/deploy/nginx.conf` | 0.5d |
| **M1 Schema**           | 全部 DDL 写完 goose migration + 跑通                                                        | 1d   |
| **M2 服务骨架**         | go-zero admin-api 跑通 login + users CRUD + 鉴权 middleware                                 | 2d   |
| **M3 App / Token 管理** | app CRUD + api token 创建/撤销 + audit log 接入                                             | 1.5d |
| **M4 Manifest 端点**    | protocol-api 读 DB 组装 manifest, 处理 noUpdate / 404 / 406, 写 manifest_requests           | 2d   |
| **M5 Code Signing**     | keypair 生成 / 导入 / 启用 / 签名 manifest                                                  | 1.5d |
| **M6 发布流**           | plan / COS pre-sign / 直传 / finalize draft / admin publish + cli/publish.ts                | 2.5d |
| **M7 事件 + 可观测**    | events 端点 + dashboard 统计卡                                                               | 1.5d |
| **M8 回滚 + 删除**      | rollback 复制 / 单条删除 / 批量清理 + 孤儿 asset GC                                         | 1.5d |
| **M9 Dashboard MVP**    | 9 个页面 + Nuxt UI + 调 Go API + CI 构建脚本 (vite build → rsync 静态文件到服务器)          | 4d   |
| **M10 生产加固**        | 限速中间件 / 安全响应头 / 健康检查 / nginx 限速 / 兜底 asset GC                              | 1d   |
| **M11 端到端验证**      | 真机 E2E: 不签名 → 签名 → rollback → 清理 全流程跑通                                        | 1.5d |

**总估时**：~20 工作日。

---

## 14. 风险与未决项

### 14.1 已识别风险

| 风险                               | 缓解                                                                                 |
| ---------------------------------- | ------------------------------------------------------------------------------------ |
| 客户端 manifest 拉到后被中间人篡改 | code signing（强制开启）                                                             |
| Admin 密码泄露                     | bcrypt cost=12 + 改密可立即吊销 JWT 24h 窗口 + audit log 全量                        |
| 大量客户端同时上报事件             | 限速 60/min/device + 不阻塞 manifest 主链路                                          |
| 孤儿 asset 累积                    | 同步 GC（删除/清理时）+ cron GC（兜底）                                              |
| PostgreSQL 单点故障                | pg_dump 每日 cron + COS 归档；内部 1-2 App RPO 24h 可接受                            |
| 大 update 包含几千个 asset         | plan/finalize 一次请求体过大（10MB+）→ 后期拆 batch；MVP 假设单次 update < 500 asset |
| COS 桶误操作                       | 整桶 private + 路径级公有读 + 部署文档明确操作流程                                   |

### 14.2 未决项（明确说"以后再说"）

- **多副本部署**：单 protocol-api + 单 admin-api 假设；如扩多副本，限速改为 Redis 共享 / DB 行锁
- **跨 region 部署**：单 region COS 假设
- **Protocol v0 兼容**：客户端必须升级到 v1
- **资产预压缩**（brotli/gzip）：MVP 不做，加法增量小
- **Channel / Branch / 灰度**：明确不做
- **自定义 directive**：明确不做
- **Webhooks / 通知**：用户未要求
- **客户端事件原始日志查询 UI**：先用 SQL 直查 `client_events` 表，量大时再补 UI

### 14.3 后续可演进方向

- KMS 加密私钥（替换 env AES 密钥）
- ClickHouse 接 client_events 做大盘
- Webhook 发布完成通知（钉钉/飞书/Slack）
- 自定义 directive（强制升级、公告）
- 多 keyid 轮换
- Sentry / Crashlytics 集成
- 蓝绿发布（虽然是冗余的：runtimeVersion 已是天然隔离）

---

## 附录 A：参考资料

- [Expo Updates v1 规范](https://docs.expo.dev/technical-specs/expo-updates-1/)
- `expo/packages/expo-updates` 客户端 README
- 官方参考实现 `custom-expo-updates-server`（Next.js）
- EAS Update 文档（channel / branch / runtimeVersion / platform 概念参考）
- `docs/IMPLEMENTATION_GUIDE_CLAUDE.md`（调研稿，作分析参考）
- `docs/IMPLEMENTATION_GUIDE_GPT.md`（调研稿，作分析参考）
- `docs/xavia-ota-analysis.md`（开源方案分析）
- `docs/expo-open-ota-analysis.md`（开源方案分析）

## 附录 B：ADR 索引

- [ADR-0001 砍掉 channel/branch 概念](./adr/0001-drop-channel-branch.md)
- [ADR-0002 腾讯云 COS 路径级公有读](./adr/0002-tencent-cos-with-path-public-read.md)
- [ADR-0003 两阶段发布 + 无状态 finalize](./adr/0003-stateless-two-phase-upload.md)
