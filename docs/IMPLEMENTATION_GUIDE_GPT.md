# Expo Updates 自建服务实现指导文档

技术栈：go-zero + PostgreSQL + Vue 3 + 对象存储 + Redis + Prometheus/Grafana

目标：实现一个符合 Expo Updates v1 协议、可生产部署、可灰度发布、可观测、可审计的自托管 Expo OTA 服务端，并提供 Vue dashboard 管理面板。

参考资料：

- `expo/packages/expo-updates/README.md`
- Expo Updates v1 SPEC：`https://docs.expo.dev/technical-specs/expo-updates-1/`
- 官方示例服务端：`custom-expo-updates-server/`
- EAS Update 文档中的 channel、branch、runtimeVersion、platform、rollback、debugging 工作模型

## 1. 核心结论

Expo Updates 服务端不是简单的静态文件服务器。服务端必须基于客户端请求头选择一个与当前 binary 兼容的最新 update，并返回符合协议的 manifest 或 directive。资产 URL 必须不可变，manifest 必须 no-cache，代码签名、channel 到 branch 路由、runtimeVersion 兼容性和 rollback 是生产化实现的关键。

建议采用以下产品模型：

| 概念 | 说明 |
| --- | --- |
| Organization | 多租户组织边界 |
| Project | 一个 Expo/RN 应用 |
| Runtime Version | binary 原生层兼容版本，只有相同 runtime 的 update 才能发给该 binary |
| Platform | `ios` 或 `android` |
| Channel | binary 构建时写入的通道，如 `production`、`staging` |
| Branch | 服务端 update 序列，类似 Git 分支 |
| Channel Routing | channel 指向一个或多个 branch，可做百分比灰度 |
| Update | 一次发布，包含 manifest、launch asset、普通 assets、metadata、extra |
| Asset | 内容寻址的不可变文件，建议以 sha256 存对象存储 |
| Directive | 服务端指令，如 `noUpdateAvailable`、`rollBackToEmbedded` |

推荐优先实现 Expo/EAS 的核心模型：`project + channel + branch + runtimeVersion + platform + update`。不要只按 `runtimeVersion` 查最新 update，否则无法安全支持 staging、production、灰度和多项目。

## 2. 服务端完整功能列表

### 2.1 Expo Updates 协议端点

这些端点面向移动端客户端，应无状态、低延迟、易水平扩容。

| 功能 | 推荐端点 | 必要性 | 实现要求 |
| --- | --- | --- | --- |
| Manifest 查询 | `GET /api/updates/:projectId/manifest` | 必须 | 校验 `expo-protocol-version`、`expo-platform`、`expo-runtime-version`、`accept`；读取 `expo-channel-name` 或 query 调试参数；按 channel 路由、runtime、platform 选择最新兼容 update |
| JSON manifest 响应 | 同上 | 必须支持其一 | 当 `Accept` 偏好 `application/expo+json` 或 `application/json` 且结果是 normal update 时返回 JSON body |
| Multipart 响应 | 同上 | 强烈推荐 | 支持 `multipart/mixed`，可返回 `manifest`、`extensions`、`directive` part；这是 directive 和 asset request headers 的主要承载形式 |
| No update directive | 同上 | 必须 | protocol v1 且 `expo-current-update-id` 等于最新 update 时返回 `noUpdateAvailable` directive 或 204 no content |
| Rollback directive | 同上 | 必须 | branch/runtime/platform 配置回滚时返回 `rollBackToEmbedded`；如果 `expo-embedded-update-id` 已是当前状态则返回 no update |
| Asset 下载 | `GET /api/assets/:assetKey` 或对象存储 CDN URL | 必须 | URL 必须不可变，不能指向“当前最新 update”；正确返回 `Content-Type`、`Cache-Control: public, max-age=31536000, immutable` |
| 压缩资产 | 同上 | 强烈推荐 | 对 JS bundle 预生成 `.br`、`.gz`，按 `Accept-Encoding` 返回，设置 `Content-Encoding` 和 `Vary: Accept-Encoding` |
| Code Signing | manifest/directive 响应头或 part 头 | 强烈推荐 | 解析 `expo-expect-signature`；按 `keyid` 选择私钥；签名实际发送的 JSON 字节；返回 SFV 格式 `expo-signature` |
| Manifest filters | 响应头 | 必须 | 返回 `expo-manifest-filters`，用于客户端本地过滤已下载 updates |
| Server defined headers | 响应头 | 强烈推荐 | 返回 `expo-server-defined-headers`，例如写入稳定灰度 bucket、channel 锚定信息 |
| Extensions | multipart part | 推荐 | 返回 `assetRequestHeaders`，用于资产请求附加认证头；无需求时返回空对象或省略 |
| 健康检查 | `GET /healthz`、`GET /readyz` | 必须 | readiness 检查 DB、Redis、对象存储关键依赖 |
| Metrics | `GET /metrics` | 必须 | Prometheus 文本格式 |

### 2.2 Manifest 选择算法

推荐热路径逻辑：

1. 解析 `projectId`，定位项目。
2. 校验 `expo-protocol-version`，默认可兼容缺失时按 v0 或返回 400，生产建议只支持 v1 并明确报错。
3. 校验 `expo-platform in ('ios','android')`。
4. 读取 `expo-runtime-version`。
5. 从 `expo-channel-name`、`Expo-Extra-Params` 或调试 query 中确定 channel。
6. 根据 channel routing 和稳定分桶选择 branch。
7. 检查 branch 是否有 active directive，如 `rollBackToEmbedded`。
8. 在 `updates` 中查找 `branch + runtimeVersion + platform + status='published'` 的最新 update。
9. 若客户端 `expo-current-update-id` 等于最新 update 的 `manifest_uuid`，返回 `noUpdateAvailable`。
10. 组装 manifest，按 `Accept` 决定 JSON 或 multipart。
11. 如请求包含 `expo-expect-signature`，对 manifest/directive 的实际 JSON bytes 签名。
12. 记录结构化日志、指标和异步 client event。

伪代码：

```go
func ResolveManifest(req Request) (Response, error) {
    project := loadProject(req.ProjectID)
    platform := validatePlatform(req.Header("expo-platform"))
    runtime := require(req.Header("expo-runtime-version"))
    channel := resolveChannel(req)
    branch := rolloutRouter.SelectBranch(project.ID, channel, req.StableDeviceKey())

    if directive := repo.ActiveDirective(branch.ID, runtime, platform); directive != nil {
        return buildDirectiveResponse(req, directive)
    }

    update := repo.LatestPublishedUpdate(branch.ID, runtime, platform)
    if update == nil {
        return noUpdateOr404(req)
    }
    if req.Header("expo-current-update-id") == update.ManifestUUID.String() {
        return buildNoUpdateResponse(req)
    }
    return buildManifestResponse(req, update)
}
```

### 2.3 管理 API

这些端点面向 dashboard、CLI 和 CI/CD，必须鉴权、授权、审计。

| 模块 | 功能 |
| --- | --- |
| 认证 | 密码登录、OIDC/GitHub OAuth、JWT access/refresh、API token、token 撤销 |
| RBAC | `owner`、`admin`、`developer`、`viewer`；CI token 支持 scope，如 `publish`、`read`、`rollback` |
| 组织管理 | organization CRUD、成员邀请、成员角色调整 |
| 项目管理 | project CRUD、项目配置、默认签名 key、对象存储 namespace |
| Runtime 管理 | runtimeVersion 列表、兼容 update、活跃 binary 分布 |
| Branch 管理 | 创建、删除、归档、查看 update 时间线 |
| Channel 管理 | 创建、删除、channel 到 branch 映射、灰度权重调整 |
| 发布上传 | 上传 `expo export` 产物或 zip；解析 `metadata.json`、`expoConfig.json`；校验资产存在性；写入 pending，再原子切 published |
| 预签名上传 | 大文件直接上传对象存储，完成后调用 finalize |
| Update 管理 | 列表、详情、manifest 预览、资产清单、republish、promote、禁用 |
| Rollback | republish previous update；或发 `rollBackToEmbedded` directive |
| Code signing key | key 生成、导入、启用、禁用、轮换、keyid 映射 |
| 灰度发布 | 按百分比、设备 hash、用户标签、runtime/platform 过滤 |
| Webhook | 发布完成、回滚、灰度推进、错误率超阈值通知 |
| 审计日志 | 记录所有管理写操作，含 actor、IP、payload、traceId |
| 客户端事件 | 批量接收下载、启动、失败、crash、日志上报 |

### 2.4 发布 CLI

Dashboard 上传适合人工操作，但生产 CI 更适合 CLI。

推荐提供 `ota` CLI：

```sh
ota login
ota publish --project app --branch staging --platform all --message "fix checkout" --dist ./dist
ota promote --project app --from staging --to production --update <id>
ota rollback --project app --channel production --to-update <id>
ota rollback-embedded --project app --branch production --runtime 1.2.0
ota channel edit production --branch release-2026-05 --weight 100
```

CLI 发布流程：

1. 执行或接收 `npx expo export --platform ios|android|all` 产物。
2. 读取 `metadata.json` 和 `expoConfig.json`。
3. 计算每个文件的 `sha256`、`md5`、size、contentType。
4. 调用服务端创建 upload session。
5. 服务端返回缺失资产的预签名 URL。
6. CLI 并发上传缺失资产。
7. 调用 finalize，服务端事务写入 update、update_assets。
8. 服务端异步预热 manifest cache，并发 webhook。

## 3. 协议关键实现细节

### 3.1 请求头

客户端必须或可能发送：

| Header | 含义 |
| --- | --- |
| `expo-protocol-version` | v1 协议应为 `1` |
| `accept` | 可包含 `application/expo+json`、`application/json`、`multipart/mixed` |
| `expo-platform` | `ios` 或 `android` |
| `expo-runtime-version` | native runtime 兼容版本 |
| `expo-expect-signature` | 客户端要求 code signing，SFV dictionary |
| `expo-current-update-id` | 当前运行 update UUID，用于 no update 判断 |
| `expo-embedded-update-id` | embedded update UUID，用于 rollback 判断 |
| `expo-channel-name` | 构建时写入的 channel，通常在 `updates.requestHeaders` 配置 |
| `Expo-Extra-Params` | 通过 `Updates.setExtraParamAsync` 设置的额外参数，SFV dictionary |

### 3.2 响应头

normal manifest、directive、multipart 都应携带：

| Header | 推荐值 |
| --- | --- |
| `expo-protocol-version` | `1` |
| `expo-sfv-version` | `0` |
| `expo-manifest-filters` | 如 `branch="production"` 或 `rollout="bucket-12"` |
| `expo-server-defined-headers` | 如 `expo-channel-name="production"`，无则 `{}` 对应空 SFV |
| `cache-control` | manifest 用 `private, max-age=0` |
| `content-type` | `application/expo+json`、`application/json` 或 `multipart/mixed; boundary=...` |
| `expo-signature` | 请求签名时返回，JSON 响应放 response header，multipart 放 part header |

注意：官方示例为了演示在 extensions 中给每个 asset 加了 `test-header`。生产实现不要保留这种示例 header。

### 3.3 Manifest 字段

```ts
type Manifest = {
  id: string;
  createdAt: string;
  runtimeVersion: string;
  launchAsset: Asset;
  assets: Asset[];
  metadata: { [key: string]: string };
  extra: { [key: string]: any };
};
```

实现要求：

- `id` 必须是 UUID，建议上传 finalize 时生成并持久化，不要每次请求重新算。
- `createdAt` 必须是 update 真实创建时间，不能用请求当前时间，也不能用会漂移的对象存储 `LastModified`。
- `runtimeVersion` 必须与请求匹配。
- `launchAsset.contentType` 使用 `application/javascript`。
- `launchAsset.fileExtension` 建议省略。
- `assets[].hash` 是 base64url 无 padding 的 SHA-256。
- `assets[].key` 推荐使用 md5 hex 或稳定资产 key。
- `assets[].url` 必须不可变，推荐包含 sha256 或 asset id，不要包含“latest”。
- `metadata` 必须是 string-valued dictionary，建议至少包含 `branch`、`channel`、`runtimeVersion`、`platform`、`rollout`。
- `extra.expoClient` 可放 `expoConfig.json`，很多 Expo 模块依赖该配置。

### 3.4 Asset URL 的不可变性

这是最容易踩坑的地方。协议要求：一个 asset URL 对应的内容不能改变或删除，因为客户端可能在任意时间拉取旧 manifest 中的资产。

错误设计：

```text
/api/assets?asset=bundles/android.js&runtimeVersion=1.0.0&platform=android
```

如果服务端按 runtime 查“当前最新 update”，客户端拿到旧 manifest 后遇到新发布，就可能下载到新 update 的资产，导致 hash 校验失败。

正确设计：

```text
https://cdn.example.com/assets/sha256/4nGjshgRoD62YxnJAnZBWllEzCBrQR2zQ_2ei8glL6s
https://ota.example.com/api/assets/6f3...uuid?hash=4nGj...
```

推荐对象存储 key：

```text
projects/{project_id}/assets/sha256/{base64url_sha256}
projects/{project_id}/assets/sha256/{base64url_sha256}.br
projects/{project_id}/assets/sha256/{base64url_sha256}.gz
```

### 3.5 Code Signing

实现要点：

1. 解析 `expo-expect-signature`，至少支持 `sig`、`keyid`、`alg`。
2. 选择与 `keyid` 匹配且未 revoked 的私钥。
3. 若 `alg` 不支持，返回 400 或 406。
4. 对实际发送的 manifest/directive JSON bytes 做 RSA-SHA256 PKCS#1 v1.5 签名。
5. `expo-signature` 使用 Expo SFV dictionary 格式。

推荐返回：

```text
expo-signature: sig=:BASE64_SIGNATURE:, keyid="main", alg="rsa-v1_5-sha256"
```

不要先签一个对象，后续再重新 `json.Marshal` 写响应。应先生成 `body []byte`，签 `body`，再写同一份 `body`。

### 3.6 Rollback 的两种语义

| 操作 | 协议行为 | 适用场景 |
| --- | --- | --- |
| Republish previous | 把旧 update 复制为当前最新 update，客户端下载旧 JS/资产 | “撤回坏版本”最常用，不依赖 embedded 版本 |
| `rollBackToEmbedded` directive | 指示客户端回到 binary 内嵌 update | 当前所有 OTA 都不可用或需强制回到 app store 包内版本 |

Dashboard 上应避免都叫“回滚”。建议命名：

- “重新发布旧版本”：republish previous update。
- “回滚到内嵌版本”：roll back to embedded。

## 4. 推荐项目架构

### 4.1 Monorepo 结构

```text
expo-ota/
├── server/
│   ├── api/
│   │   ├── protocol/
│   │   │   ├── protocol.api
│   │   │   ├── etc/protocol.yaml
│   │   │   ├── internal/handler/
│   │   │   ├── internal/logic/
│   │   │   └── internal/svc/
│   │   └── admin/
│   │       ├── admin.api
│   │       ├── etc/admin.yaml
│   │       └── internal/
│   ├── internal/
│   │   ├── domain/
│   │   ├── repository/
│   │   ├── service/
│   │   ├── manifest/
│   │   ├── storage/
│   │   ├── signing/
│   │   ├── rollout/
│   │   ├── auth/
│   │   ├── events/
│   │   ├── metrics/
│   │   └── sfv/
│   ├── model/
│   ├── migrations/
│   ├── jobs/
│   └── go.mod
├── dashboard/
│   ├── src/api/
│   ├── src/views/
│   ├── src/components/
│   ├── src/stores/
│   ├── src/router/
│   └── package.json
├── cli/
│   └── cmd/ota/
├── deploy/
│   ├── docker-compose.yml
│   ├── helm/
│   └── grafana/
└── docs/
```

### 4.2 go-zero 服务拆分

推荐初期两个 HTTP API，后续按压力拆分 RPC：

| 服务 | 类型 | 职责 |
| --- | --- | --- |
| protocol-api | go-zero api | manifest、asset redirect、events、health、metrics |
| admin-api | go-zero api | dashboard、CLI、发布、RBAC、项目管理 |
| publisher-worker | queue worker | 解析上传、预压缩、去重、finalize、webhook |
| stats-worker | queue worker | 聚合 client events、更新覆盖率、错误率 |

go-zero 适配建议：

- `protocol.api` 尽量只定义普通 JSON 管理端点；manifest/multipart 这类特殊响应在 handler 中直接写 `http.ResponseWriter`。
- 使用 go-zero middleware 做 request id、panic recover、CORS、rate limit、JWT。
- PostgreSQL 推荐 `pgx/v5 + sqlc`，go-zero 自带 model 生成更偏 MySQL，可只复用框架能力。
- Redis 使用 go-zero `redis` 或 `go-redis`，缓存 channel routing、manifest skeleton、token introspection。
- Trace 使用 go-zero 内置链路追踪并接 OpenTelemetry collector。

### 4.3 部署拓扑

```text
Mobile App
   |
   v
CDN / WAF
   |-----------------------> Object Storage for immutable assets
   |
   v
protocol-api pods  ---- Redis
   |                    |
   v                    v
PostgreSQL <------ admin-api pods <------ Vue Dashboard / CLI / CI
   |
   v
workers ---> webhook / metrics / event aggregation
```

关键策略：

- Manifest 走源站，`Cache-Control: private, max-age=0`。
- Asset 走 CDN/对象存储，长缓存 immutable。
- protocol-api 只读为主，可按 QPS 独立扩容。
- admin-api 写路径必须审计和限流。
- client events 可降级丢弃，不能影响 manifest 主链路。

## 5. 推荐数据库结构

约定：PostgreSQL，主键 UUID，时间 `timestamptz`，JSON 用 `jsonb`，邮箱用 `citext`，UUID 使用 `pgcrypto` 的 `gen_random_uuid()`。

### 5.1 核心 ER

```text
organizations 1--* projects 1--* branches 1--* updates
organizations 1--* memberships *--1 users
projects 1--* channels 1--* channel_routes *--1 branches
projects 1--* runtime_versions 1--* updates
updates *--* assets via update_assets
projects 1--* code_signing_keys
projects 1--* api_tokens
projects 1--* client_events
```

### 5.2 SQL 草案

```sql
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE organizations (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  slug text NOT NULL UNIQUE,
  name text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE users (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  email citext NOT NULL UNIQUE,
  password_hash text,
  display_name text,
  created_at timestamptz NOT NULL DEFAULT now(),
  disabled_at timestamptz
);

CREATE TABLE memberships (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role text NOT NULL CHECK (role IN ('owner','admin','developer','viewer')),
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (organization_id, user_id)
);

CREATE TABLE projects (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  slug text NOT NULL,
  name text NOT NULL,
  public_id text NOT NULL UNIQUE,
  manifest_extra jsonb NOT NULL DEFAULT '{}'::jsonb,
  default_signing_key_id uuid,
  created_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  UNIQUE (organization_id, slug)
);

CREATE TABLE api_tokens (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  name text NOT NULL,
  token_hash bytea NOT NULL UNIQUE,
  scopes text[] NOT NULL,
  created_by uuid REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz,
  revoked_at timestamptz
);

CREATE TABLE runtime_versions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  version text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (project_id, version)
);

CREATE TABLE branches (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  name text NOT NULL,
  description text,
  created_at timestamptz NOT NULL DEFAULT now(),
  archived_at timestamptz,
  UNIQUE (project_id, name)
);

CREATE TABLE channels (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  name text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  archived_at timestamptz,
  UNIQUE (project_id, name)
);

CREATE TABLE channel_routes (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  channel_id uuid NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
  branch_id uuid NOT NULL REFERENCES branches(id) ON DELETE CASCADE,
  weight int NOT NULL CHECK (weight BETWEEN 0 AND 10000),
  priority int NOT NULL DEFAULT 0,
  filter jsonb NOT NULL DEFAULT '{}'::jsonb,
  rollout_key text NOT NULL DEFAULT 'device',
  active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (channel_id, branch_id)
);

CREATE TABLE assets (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  sha256 bytea NOT NULL,
  sha256_base64url text NOT NULL,
  md5_hex text NOT NULL,
  size_bytes bigint NOT NULL CHECK (size_bytes >= 0),
  content_type text NOT NULL,
  file_ext text,
  storage_bucket text NOT NULL,
  storage_key text NOT NULL,
  br_storage_key text,
  gz_storage_key text,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (project_id, sha256)
);

CREATE INDEX assets_project_sha256_b64_idx ON assets(project_id, sha256_base64url);

CREATE TABLE updates (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  branch_id uuid NOT NULL REFERENCES branches(id) ON DELETE CASCADE,
  runtime_version_id uuid NOT NULL REFERENCES runtime_versions(id),
  platform text NOT NULL CHECK (platform IN ('ios','android')),
  manifest_uuid uuid NOT NULL,
  launch_asset_id uuid NOT NULL REFERENCES assets(id),
  status text NOT NULL CHECK (status IN ('pending','published','disabled','failed')),
  message text,
  git_commit_hash text,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  extra jsonb NOT NULL DEFAULT '{}'::jsonb,
  expo_config jsonb,
  manifest_json jsonb,
  published_by uuid REFERENCES users(id),
  published_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  disabled_at timestamptz,
  UNIQUE (project_id, manifest_uuid),
  UNIQUE (branch_id, runtime_version_id, platform, manifest_uuid)
);

CREATE INDEX updates_latest_idx
  ON updates(branch_id, runtime_version_id, platform, published_at DESC)
  WHERE status = 'published';

CREATE TABLE update_assets (
  update_id uuid NOT NULL REFERENCES updates(id) ON DELETE CASCADE,
  asset_id uuid NOT NULL REFERENCES assets(id),
  asset_key text NOT NULL,
  file_ext text,
  sort_order int NOT NULL DEFAULT 0,
  PRIMARY KEY (update_id, asset_key)
);

CREATE TABLE branch_directives (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  branch_id uuid NOT NULL REFERENCES branches(id) ON DELETE CASCADE,
  runtime_version_id uuid REFERENCES runtime_versions(id),
  platform text NOT NULL CHECK (platform IN ('ios','android','all')),
  type text NOT NULL CHECK (type IN ('rollBackToEmbedded','noUpdateAvailable')),
  parameters jsonb NOT NULL DEFAULT '{}'::jsonb,
  active boolean NOT NULL DEFAULT true,
  created_by uuid REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  disabled_at timestamptz
);

CREATE INDEX branch_directives_active_idx
  ON branch_directives(branch_id, runtime_version_id, platform)
  WHERE active = true;

CREATE TABLE code_signing_keys (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  key_id text NOT NULL,
  algorithm text NOT NULL DEFAULT 'rsa-v1_5-sha256',
  public_key_pem text NOT NULL,
  private_key_ciphertext bytea NOT NULL,
  kms_key_id text,
  created_at timestamptz NOT NULL DEFAULT now(),
  activated_at timestamptz,
  revoked_at timestamptz,
  UNIQUE (project_id, key_id)
);

ALTER TABLE projects
  ADD CONSTRAINT projects_default_signing_key_fk
  FOREIGN KEY (default_signing_key_id) REFERENCES code_signing_keys(id);

CREATE TABLE upload_sessions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  branch_id uuid NOT NULL REFERENCES branches(id),
  runtime_version text NOT NULL,
  platform text NOT NULL CHECK (platform IN ('ios','android','all')),
  status text NOT NULL CHECK (status IN ('created','uploading','finalized','expired','failed')),
  manifest_metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_by uuid REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL,
  finalized_at timestamptz
);

CREATE TABLE client_events (
  id bigserial,
  project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  occurred_at timestamptz NOT NULL,
  event_type text NOT NULL,
  update_id uuid,
  manifest_uuid uuid,
  channel text,
  branch text,
  runtime_version text,
  platform text,
  device_id_hash text,
  app_version text,
  os_version text,
  duration_ms int,
  error_code text,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  PRIMARY KEY (occurred_at, id)
) PARTITION BY RANGE (occurred_at);

CREATE TABLE update_daily_stats (
  day date NOT NULL,
  project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  update_id uuid,
  channel text,
  runtime_version text,
  platform text,
  manifest_requests bigint NOT NULL DEFAULT 0,
  unique_devices bigint NOT NULL DEFAULT 0,
  download_success bigint NOT NULL DEFAULT 0,
  download_failure bigint NOT NULL DEFAULT 0,
  launch_success bigint NOT NULL DEFAULT 0,
  launch_failure bigint NOT NULL DEFAULT 0,
  PRIMARY KEY (day, project_id, update_id, channel, runtime_version, platform)
);

CREATE TABLE audit_logs (
  id bigserial PRIMARY KEY,
  organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  project_id uuid REFERENCES projects(id) ON DELETE SET NULL,
  actor_user_id uuid REFERENCES users(id),
  actor_token_id uuid REFERENCES api_tokens(id),
  action text NOT NULL,
  target_type text,
  target_id text,
  request_id text,
  ip inet,
  user_agent text,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  occurred_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE webhooks (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  url text NOT NULL,
  secret_ciphertext bytea NOT NULL,
  events text[] NOT NULL,
  active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  disabled_at timestamptz
);
```

### 5.3 热路径查询

最新 update 查询：

```sql
SELECT *
FROM updates
WHERE branch_id = $1
  AND runtime_version_id = $2
  AND platform = $3
  AND status = 'published'
ORDER BY published_at DESC
LIMIT 1;
```

Channel routing 查询应缓存：

```sql
SELECT cr.*, b.name AS branch_name
FROM channel_routes cr
JOIN channels c ON c.id = cr.channel_id
JOIN branches b ON b.id = cr.branch_id
WHERE c.project_id = $1
  AND c.name = $2
  AND cr.active = true
ORDER BY cr.priority DESC, cr.created_at DESC;
```

建议 Redis key：

| Key | TTL | 失效时机 |
| --- | --- | --- |
| `project:{public_id}` | 5m | project 更新 |
| `channel_routes:{project_id}:{channel}` | 30s-5m | channel route 更新 |
| `latest_update:{branch_id}:{runtime_id}:{platform}` | 10s-60s | publish、disable、directive 更新 |
| `manifest_skeleton:{update_id}` | 长 TTL | update immutable，服务版本升级时换 key prefix |
| `api_token:{hash}` | 60s | token revoke |

## 6. Dashboard 推荐功能

### 6.1 信息架构

| 页面 | 功能 |
| --- | --- |
| 登录页 | 密码、OIDC、GitHub OAuth、MFA 可选 |
| 组织/项目选择 | 多组织、多项目切换 |
| 全局总览 | 项目列表、近 24h manifest QPS、发布次数、错误率、活跃设备 |
| 项目首页 | channel 状态卡片、最新 update、runtime 分布、覆盖率曲线 |
| Deployments | 类似 EAS Deployments，按 channel 展示当前 branch、runtime、platform、update |
| Channels | channel 列表、路由编辑、灰度权重、稳定分桶预览 |
| Branches | branch 列表、update 时间线、promote、republish |
| Update 详情 | manifest JSON、资产清单、大小、下载链接、签名状态、覆盖率、错误率 |
| 发布向导 | 上传 zip/dist、解析预览、选择 branch、dry-run、确认发布 |
| Rollback 控制台 | 重新发布旧版本、回滚到 embedded、二次确认、影响面预估 |
| Runtime 管理 | runtime 版本、平台、活跃设备、兼容 updates、老版本下线建议 |
| Code Signing | key 列表、生成/导入、激活、轮换、撤销、客户端配置示例 |
| Client Events | 下载失败、启动失败、crash、日志检索 |
| Observability | 内嵌 Grafana 或自绘 ECharts 面板 |
| Members | 成员、角色、邀请 |
| API Tokens | CI token、scope、过期、撤销 |
| Audit Logs | 操作审计、过滤、导出 |
| Webhooks | 通知配置、投递日志、重试 |

### 6.2 关键交互建议

- Channel 路由编辑必须显示“预计影响设备数”和“当前 production 指向”。
- 灰度调整采用滑块，但保存前展示 diff：`production: main 100% -> main 90%, candidate 10%`。
- Rollback 操作必须二次确认，并要求输入 branch/channel 名称。
- Update 详情应突出 `runtimeVersion` 和 `platform`，避免把 iOS update 发给 Android 或反向。
- 发布向导先 dry-run：列出新增资产、复用资产、总大小、manifest id、签名 key。
- Code signing key revoke 前提示仍在使用该 keyid 的客户端版本。
- 所有写操作显示 audit log 链接。

### 6.3 前端技术建议

| 类别 | 推荐 |
| --- | --- |
| 框架 | Vue 3 + TypeScript + Vite |
| UI | Naive UI 或 Ant Design Vue |
| 状态 | Pinia |
| 路由 | Vue Router |
| 图表 | ECharts / vue-echarts |
| 请求 | Axios + OpenAPI 生成 client |
| 表单 | vee-validate + zod |
| 表格 | UI 库表格或 TanStack Table |
| 测试 | Vitest + Playwright |
| 权限 | route meta + permission directive |

## 7. 推荐可观测性指标

### 7.1 Prometheus 指标

| 指标 | 类型 | 标签 |
| --- | --- | --- |
| `ota_manifest_requests_total` | Counter | `project`, `channel`, `branch`, `runtime`, `platform`, `protocol_version`, `result` |
| `ota_manifest_request_duration_seconds` | Histogram | `project`, `channel`, `runtime`, `platform`, `result` |
| `ota_manifest_response_bytes` | Histogram | `project`, `result` |
| `ota_manifest_cache_hits_total` | Counter | `project`, `cache` |
| `ota_asset_requests_total` | Counter | `project`, `asset_type`, `encoding`, `result` |
| `ota_asset_bytes_sent_total` | Counter | `project`, `asset_type`, `encoding` |
| `ota_asset_origin_redirects_total` | Counter | `project`, `storage`, `cdn` |
| `ota_publish_total` | Counter | `project`, `branch`, `platform`, `result` |
| `ota_publish_duration_seconds` | Histogram | `project`, `platform`, `result` |
| `ota_publish_asset_dedup_total` | Counter | `project`, `result`(`new`/`reused`) |
| `ota_rollback_total` | Counter | `project`, `branch`, `type` |
| `ota_code_sign_requests_total` | Counter | `project`, `keyid`, `result` |
| `ota_code_sign_duration_seconds` | Histogram | `project`, `keyid` |
| `ota_client_events_total` | Counter | `project`, `event_type`, `platform` |
| `ota_update_adoption_ratio` | Gauge | `project`, `channel`, `update_id`, `platform` |
| `ota_active_devices` | Gauge | `project`, `channel`, `runtime`, `platform` |
| `ota_db_query_duration_seconds` | Histogram | `query`, `result` |
| `ota_storage_operation_duration_seconds` | Histogram | `op`, `storage`, `result` |
| `ota_webhook_deliveries_total` | Counter | `project`, `event`, `result` |

`result` 建议枚举：`update`、`no_update`、`rollback`、`not_found`、`bad_request`、`not_acceptable`、`error`。

### 7.2 结构化日志

每个 manifest 请求记录：

```json
{
  "level": "info",
  "msg": "manifest resolved",
  "request_id": "...",
  "project_id": "...",
  "channel": "production",
  "branch": "main",
  "runtime_version": "1.0.0",
  "platform": "ios",
  "protocol_version": "1",
  "current_update_id": "...",
  "served_update_id": "...",
  "result": "update",
  "signed": true,
  "duration_ms": 18
}
```

要求：

- 不记录 authorization token、cookie、私钥、预签名 URL 完整 query。
- request id 贯穿 admin、protocol、worker、webhook。
- 错误日志包含 stack、trace id、user id 或 token id。

### 7.3 Tracing

关键 span：

- `http.manifest`
- `manifest.resolve_project`
- `manifest.resolve_channel_route`
- `manifest.select_latest_update`
- `manifest.build_json`
- `manifest.sign`
- `storage.presign_asset`
- `publish.parse_metadata`
- `publish.upload_asset`
- `publish.finalize_tx`
- `webhook.deliver`

### 7.4 Dashboard/Grafana 面板

推荐面板：

- Manifest QPS、错误率、p95/p99 延迟。
- 按 channel/branch/runtime/platform 的 update 命中分布。
- 新 update 发布后 1h/24h 覆盖率曲线。
- 下载成功率、资产失败 Top N。
- JS crash 或 launch failure 趋势。
- 对象存储流量、CDN 命中率、资产带宽。
- Code signing 失败次数。
- 发布耗时、资产去重率。
- 老 runtime 活跃设备趋势。

## 8. 安全设计

### 8.1 必须做

- 管理 API 全量鉴权，禁止只做前端登录态。
- API token 只存 hash，显示一次后不可再取回。
- Token 支持 scope、过期、撤销。
- 私钥用 KMS 或 envelope encryption 加密存储。
- 所有发布、回滚、路由变更写 audit log。
- 上传 zip/dist 必须校验路径，禁止 `..`、绝对路径、控制字符。
- 限制上传文件大小、文件数量、总大小。
- 对 `runtimeVersion`、`channel`、`branch`、`platform` 做白名单校验。
- CSRF 防护：cookie 登录时使用 SameSite + CSRF token；纯 Bearer token 可不启用。
- 管理端和协议端分域名或路径隔离，WAF/rate limit 策略不同。

### 8.2 发布安全

OTA 本质上是在终端执行远程 JS。发布权限等价于对客户端代码执行权限，必须按高危权限处理。

建议：

- production 发布需要双人审批，可作为高级功能。
- CI token 只能发布指定 project/branch。
- 禁止 viewer/developer 修改 production channel route。
- Rollback 到 embedded 需要 admin 以上角色。
- 所有发布包记录 git commit、CI build URL、actor。

## 9. 开发路线图

### Milestone 0：技术预研与骨架（3-5 天）

- 初始化 monorepo。
- 搭建 `docker-compose`：PostgreSQL、Redis、MinIO、Prometheus、Grafana。
- 创建 go-zero `protocol-api` 和 `admin-api`。
- 选定 `pgx + sqlc + goose` 或 `pgx + ent`。
- 建立 OpenAPI 生成链路给 dashboard。
- 写 Expo Updates v1 协议契约测试样例。

验收：本地能启动空服务，`/healthz`、`/metrics` 可访问，migration 可执行。

### Milestone 1：协议最小闭环（1-2 周）

目标：真实 iOS/Android release build 能从自建服务拉取 update 并启动。

- 实现 organizations/projects 基础表，可先内置单项目。
- 实现 branches、channels、runtime_versions、updates、assets 表。
- 实现本地文件或 MinIO asset storage。
- 实现 CLI 手工导入 `expo export` 产物。
- 实现 manifest endpoint：multipart v1、normal update、不签名。
- 实现 asset endpoint 或 CDN URL。
- 实现 `extra.expoClient` 注入。
- 用官方示例 client 或自有 Expo app 端到端验证。

验收：发布一个 update 后，客户端二次冷启动能加载新 JS；资产 hash 校验通过。

### Milestone 2：协议完整性（1-2 周）

- 支持 `Accept` 协商，JSON 和 multipart。
- 实现 `noUpdateAvailable`。
- 实现 `rollBackToEmbedded`。
- 实现 `expo-manifest-filters` 和 `expo-server-defined-headers`。
- 实现 code signing，多 keyid，签名契约测试。
- 资产 gzip/brotli 预压缩，`Vary: Accept-Encoding`。
- 增加 protocol v0 兼容策略或明确拒绝。

验收：覆盖正常更新、无更新、rollback、签名、压缩资产的自动化测试。

### Milestone 3：发布系统与管理 API（2 周）

- 实现 JWT 登录、API token、RBAC。
- 实现 project/branch/channel CRUD。
- 实现 upload session、预签名上传、finalize。
- 实现 asset 去重和 pending/published 状态机。
- 实现 republish、promote。
- 实现 audit logs。
- 实现 webhook 基础能力。

验收：CI 可通过 CLI 发布到 staging，管理员可 promote 到 production。

### Milestone 4：Vue Dashboard MVP（2 周）

- 登录、项目切换。
- 项目首页和 deployment 总览。
- Branch/update 详情。
- 发布向导。
- Channel route 编辑。
- Rollback/republish 操作。
- API token 管理。
- Audit log 查看。

验收：不使用 CLI 也能完成发布、promote、rollback 的核心流程。

### Milestone 5：灰度发布（1-2 周）

- 实现 `channel_routes.weight`。
- 设计稳定分桶 key：优先设备安装 ID 或客户端传入 device id hash，否则退化为 embedded/current update id、IP+UA hash。
- 支持按 `Expo-Extra-Params`、app version、OS version 的过滤条件。
- Dashboard 显示灰度影响面和健康指标。
- 灰度自动暂停：错误率超过阈值时停止推进。

验收：同一设备多次请求稳定命中同一 branch；10% 灰度分布接近预期。

### Milestone 6：可观测性与客户端事件（1-2 周）

- 实现 Prometheus 指标。
- 实现 `POST /api/events` 批量事件上报。
- stats-worker 聚合 `update_daily_stats`。
- Grafana dashboard JSON。
- Dashboard 展示覆盖率、错误率、下载漏斗。
- 接入 OpenTelemetry traces 和 Loki 日志。

验收：一次发布后能在 24h 曲线中看到 adoption，manifest p99 和错误率可告警。

### Milestone 7：生产化（1-2 周）

- Helm chart、Kubernetes probes、HPA。
- CDN 接入和缓存验证。
- 数据库备份恢复演练。
- 对象存储生命周期策略和孤儿资产 GC。
- 限流、WAF、IP allowlist 可选。
- 安全审计：私钥加密、token redact、权限矩阵。
- 压测：manifest QPS、发布大包、资产下载。

验收：多副本部署下发布后所有 pod 在可接受时间内看到新 update；5k QPS manifest p99 达标。

### Milestone 8：高级能力（按需）

- 多区域部署和 GeoDNS。
- ClickHouse 存储 client events。
- Sentry/Crashlytics 集成。
- GitHub Actions/GitLab CI 官方 action。
- 审批流和变更窗口。
- 自定义 directive，如强制商店升级、公告、实验开关。

## 10. 测试策略

### 10.1 单元测试

- SHA-256 base64url、md5 key 计算。
- SFV dictionary parse/serialize。
- Accept header 协商。
- Code signing 签名和验签。
- Rollout 稳定分桶。
- Manifest JSON 字段生成。

### 10.2 协议契约测试

覆盖请求：

- 缺少 platform 返回 400。
- platform 非 ios/android 返回 400。
- runtime 不存在返回 404 或 no update。
- `expo-current-update-id` 等于最新时返回 no update。
- rollback active 时返回 directive。
- `Accept` 不支持时返回 406。
- 请求签名但 key 不存在时返回 400。

覆盖响应：

- 必要 headers 存在。
- multipart part name 正确。
- manifest.id 是 UUID。
- `createdAt` 稳定。
- asset hash 与实际文件一致。
- asset URL 不随新发布改变。

### 10.3 E2E 测试

- 使用官方 custom server 示例中的测试 update fixtures。
- 构建一个 release app，配置自建 `updates.url` 和 `expo-channel-name`。
- 发布 update 后强制重启 app，验证 UI 变化。
- 启用 code signing 后验证签名通过。
- 发布坏 update 后 republish previous，验证客户端回到旧内容。
- 发 `rollBackToEmbedded`，验证客户端回 embedded。

## 11. 上线检查清单

- [ ] Manifest 响应符合 Expo Updates v1 SPEC。
- [ ] Asset URL 内容寻址且 immutable。
- [ ] Manifest no-cache，asset long-cache。
- [ ] iOS 和 Android release build 端到端通过。
- [ ] Code signing 签名和客户端验签通过。
- [ ] `noUpdateAvailable` 不触发重复下载。
- [ ] `rollBackToEmbedded` 语义正确。
- [ ] Channel 到 branch 映射变更可审计、可回滚。
- [ ] API token 撤销在 60 秒内生效。
- [ ] 私钥加密存储且日志无泄漏。
- [ ] 多副本部署下 Redis cache 失效正确。
- [ ] Prometheus、Grafana、日志、trace 可用。
- [ ] PostgreSQL 和对象存储完成备份恢复演练。
- [ ] 压测达到目标 QPS 和 p99。

## 12. 推荐第三方库

### Go

| 用途 | 推荐 |
| --- | --- |
| Web 框架 | `github.com/zeromicro/go-zero` |
| PostgreSQL | `github.com/jackc/pgx/v5` |
| SQL 生成 | `github.com/sqlc-dev/sqlc` |
| Migration | `github.com/pressly/goose` 或 Atlas |
| Redis | `github.com/redis/go-redis/v9` 或 go-zero redis |
| S3/MinIO | `github.com/aws/aws-sdk-go-v2` |
| JWT | `github.com/golang-jwt/jwt/v5` |
| Password | `golang.org/x/crypto/bcrypt` 或 argon2id |
| Metrics | `github.com/prometheus/client_golang` |
| Tracing | `go.opentelemetry.io/otel` |
| Tests | `github.com/stretchr/testify`、`testcontainers-go` |

### Vue

| 用途 | 推荐 |
| --- | --- |
| 框架 | Vue 3 + TypeScript + Vite |
| UI | Naive UI / Ant Design Vue |
| 状态 | Pinia |
| 图表 | ECharts |
| 请求 | Axios + OpenAPI generator |
| 测试 | Vitest + Playwright |

## 13. 主要风险与规避

| 风险 | 规避 |
| --- | --- |
| Runtime version 错配导致客户端 crash | 发布时强校验 runtime；Dashboard 高亮 native 兼容性 |
| Asset URL 可变导致 hash mismatch | 内容寻址 URL，禁止 latest asset URL |
| 私钥泄露 | KMS 加密、最小权限、密钥轮换、审计 |
| 灰度分桶不稳定 | 使用稳定 device key；server-defined headers 锚定 bucket |
| 上传半成品被客户端看到 | pending 状态不可被 manifest 查询；finalize 事务切 published |
| 多副本缓存不一致 | Redis 集中缓存，发布后主动失效 |
| 管理 API 被滥用 | RBAC、scope token、rate limit、audit、二次确认 |
| 指标写入影响主链路 | client events 异步队列，满载可丢弃 |

## 14. 最小可行版本范围

如果希望最快落地，MVP 只做以下功能：

- 单组织多项目。
- API token 发布。
- branch/channel/runtime/platform 模型。
- manifest multipart v1。
- asset immutable URL。
- MinIO/S3 存储。
- no update directive。
- republish previous。
- Vue dashboard 展示项目、channel、branch、update、发布向导。
- Prometheus 基础指标。

不要在 MVP 中省略 channel/branch，也不要使用可变 asset URL。这两项后期补成本很高，而且会直接影响协议正确性和生产安全。
