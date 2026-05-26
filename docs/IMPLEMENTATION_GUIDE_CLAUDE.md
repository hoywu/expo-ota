# Expo OTA 自建服务实现指导文档

> 技术栈：**Go-Zero + PostgreSQL + Vue 3 + 对象存储**
> 目标：实现一个符合 [Expo Updates v1 协议](https://docs.expo.dev/technical-specs/expo-updates-1/) 的自建 OTA 服务端，并提供一个能够管理项目/分支/发布/回滚/灰度的 Dashboard。
>
> 本文档基于：
> - `expo/packages/expo-updates` 客户端 README
> - Expo Updates v1 SPEC（官方协议）
> - `custom-expo-updates-server`（Next.js 官方示例实现）
> - EAS Update 的工作机制（branch / channel / runtimeVersion / platform 四元组）

---

## 1. 背景与核心概念

理解协议前，必须先对齐以下名词，它们直接决定数据库结构和 API 设计：

| 概念 | 说明 | 由谁设置 |
|---|---|---|
| **Runtime Version** | 描述原生层 JS 接口的版本字符串。原生代码变更（升级 Expo SDK、加原生模块等）必须升级，否则客户端崩溃 | 由 app.json/Info.plist 在 **构建时** 写入 |
| **Platform** | `ios` 或 `android` | 客户端请求时携带 |
| **Channel** | 一个构建（binary）绑定的"通道"，如 `production`、`staging`。多个构建可共享同一个 channel | 构建时写入 binary |
| **Branch** | 服务端的一条更新序列，包含按时间排序的多次 update（类比 Git 分支） | 服务端 |
| **Channel → Branch 映射** | 一个 channel 可以被路由（rollout）到一个或多个 branch | Dashboard 配置 |
| **Update / Manifest** | 一次发布产物，包含 launchAsset（JS bundle）+ assets（图片/字体等） + 元数据 | 由 `expo export` 生成，CLI 上传 |
| **Asset** | 不可变文件（按 sha256 寻址），客户端按 URL 拉取，必须**永不修改** | 服务端永久存储 |
| **Directive** | 服务端对客户端的"指令"，目前主要有 `noUpdateAvailable` 与 `rollBackToEmbedded` | 服务端 |

客户端在每次冷启动（默认）时会带 `expo-platform`、`expo-runtime-version`、`expo-channel-name`（由 server-defined-headers 控制）、`expo-current-update-id`、`expo-embedded-update-id`、可选的 `expo-expect-signature` 等头部请求 `/api/manifest`，服务端要返回 `multipart/mixed` 响应，包含 manifest 或 directive。

---

## 2. 服务端必须实现的完整功能列表

下面列表中 **【协议必需】** 表示由 Expo Updates v1 SPEC 强制要求，**【强烈推荐】** 是自建系统的工程实践，**【可选】** 是进阶能力。

### 2.1 协议端点（面向 Expo 客户端，公网/CDN）

| 功能 | 端点 | 等级 | 关键要求 |
|---|---|---|---|
| Manifest 协商 | `GET /api/manifest`（路径可自定义） | 协议必需 | 校验 `expo-platform` / `expo-runtime-version` / `expo-protocol-version`；按 runtimeVersion + platform + channel→branch 选最新 update；支持 `multipart/mixed`，含 `manifest` + `extensions` + 可选 `directive` part；写入 `expo-protocol-version: 1`、`expo-sfv-version: 0`、`expo-manifest-filters`、`expo-server-defined-headers`、`cache-control: private, max-age=0` |
| Asset 下载 | `GET /api/assets?asset=...&runtimeVersion=...&platform=...` | 协议必需 | 资源内容**永不变更**；正确的 `content-type`；支持 `accept-encoding: br, gzip`；建议 `cache-control: public, max-age=31536000, immutable`；推荐由 CDN/对象存储直出 |
| 协议版本 0/1 兼容 | 同上 | 强烈推荐 | 老客户端用 `protocolVersion=0`（不支持 directive），新客户端 `=1` |
| `noUpdateAvailable` directive | manifest 端点的一种返回 | 协议必需（v1） | 当客户端 `expo-current-update-id` 已是最新时返回 |
| `rollBackToEmbedded` directive | manifest 端点的一种返回 | 协议必需（v1） | 当 branch 配置了"回滚到内嵌"时返回，客户端会丢弃所有下载更新 |
| 代码签名（Code Signing） | manifest/directive 的 `expo-signature` | 强烈推荐 | RSA-SHA256；按 `expo-expect-signature` 头决定是否签名；支持多 keyid 切换 |
| 自定义 directive | 同上 | 可选 | 实现自定义指令（如 `forceUpdate`、`showMessage`） |
| 健康检查 | `GET /healthz`、`/readyz` | 强烈推荐 | K8s/LB 探活 |

### 2.2 管理 API（面向 Dashboard 与 CLI，鉴权）

| 模块 | 说明 |
|---|---|
| **认证授权** | 登录（账号密码 / OIDC / GitHub OAuth）、JWT、RBAC（owner/admin/developer/viewer）、API Token（CI 使用）、双因素（可选） |
| **组织 / 项目管理** | Organization、Project、Member、邀请；项目隔离 |
| **Runtime Version 管理** | 列出、查看每个 runtime 关联的 update / binary 指标 |
| **Branch 管理** | 创建、重命名、删除、列表，查看 branch 下的 update 列表 |
| **Channel 管理** | 创建/删除 channel；channel→branch 的映射（含 rollout 灰度比例：A 80% / B 20%） |
| **Update 发布** | 接收 CLI 上传的 update bundle（multipart）：metadata.json、launchAsset、assets、expoConfig.json；服务端按 sha256 去重存储；写入数据库；可选预签名 |
| **Update 列表 / 详情** | 查看历史，附带客户端激活量、错误率、大小 |
| **Promote / Republish** | 将一个 branch 上的某 update "复制 / 提升" 到另一个 branch（无需重新打包） |
| **回滚控制** | 单次回滚到特定 update；标记 branch 进入 `rollBackToEmbedded` 状态 |
| **灰度发布 (Rollout)** | 按百分比、按 deviceId 哈希、按用户标签（meta filter）控制发布范围 |
| **Code Signing Key 管理** | 上传公钥/私钥、轮换、按 keyid 选择 |
| **Manifest filter 配置** | 配置每个 channel/branch 的 `expo-manifest-filters` 与 `expo-server-defined-headers` |
| **资源管理** | 列出 / 清理孤儿 asset；按 runtimeVersion 触发 GC |
| **审计日志** | 谁在何时做了发布 / 回滚 / channel 切换 |
| **Webhook / 通知** | 发布完成、错误率突增、回滚事件触发 Slack/钉钉/邮件/HTTP webhook |
| **客户端事件回报（推荐）** | 端侧上报 update 下载成功/失败、应用 crash、launch 时长，用于灰度健康度判断 |
| **统计查询** | 按 channel / branch / runtime / platform 聚合的 DAU、激活率、下载失败率 |

### 2.3 资源（Asset）存储

- 协议要求 asset URL 永久不变。**实现要求：以 `sha256(content)` 作为存储 key**（content-addressable），不同 update 引用同一 asset 时自动去重。
- 推荐使用 S3 兼容的对象存储（MinIO/OSS/COS/R2）+ CDN；服务端可生成预签名直链 URL，避免代理流量。
- 对 launchAsset（JS bundle）做 Brotli + Gzip 预压缩存储两份，按 `accept-encoding` 协商返回。

---

## 3. Dashboard 推荐功能（Vue 3）

### 3.1 必备页面

1. **登录 / 组织切换**
2. **总览 Dashboard**：当前组织下所有项目的发布频率、下载量、错误率热力图、最近一周 release 时间轴
3. **项目主页**
   - 概览卡片：今日 manifest 请求数、活跃设备数、最新 update 覆盖率
   - Channel 卡片矩阵：每个 channel 当前指向的 branch、当前 active update、覆盖率
4. **Branch 详情页**
   - 历史 update 时间线（类似 Git 提交图）
   - 每条 update：ID、commit message、提交者、平台/runtime、大小、激活数、错误数
   - 操作：Promote 到其他 branch、Rollback、回滚到内嵌、复制为新 branch
5. **Update 详情页**
   - manifest JSON 预览
   - 资源清单（点击可直接下载 asset）
   - 客户端激活漏斗：拉 manifest → 下载完成 → 启动成功
   - Crash / JS error 列表（如对接了 Sentry）
6. **发布向导**：上传 zip → 服务器解析 → 选择 branch → 确认发布；支持 dry-run
7. **灰度发布配置**：拖动滑块设置百分比，预览受影响设备数
8. **Channel 路由编辑**：图形化设置 channel → branch 映射 + rollout 比例
9. **Runtime Version 管理**：查看每个 runtime 下的 binary 版本分布、update 列表，兼容性提醒
10. **代码签名**：密钥列表、轮换、为 channel 启用/禁用签名
11. **设置**
    - 团队成员与权限
    - API Token
    - Webhook
    - 项目级签名策略
12. **审计日志页**：时间轴 + 过滤 + 导出
13. **可观测性页**：内嵌 Grafana 面板或自绘 ECharts

### 3.2 推荐技术栈

- Vue 3 + TypeScript + Vite
- 状态管理：Pinia
- UI 库：Ant Design Vue / Element Plus / Naive UI（任选其一，文档体验最好）
- 图表：ECharts / VueUse + Apache ECharts
- 请求：Axios + 自动生成的 OpenAPI client（go-zero 可由 `goctl` 产生 swagger）
- 国际化：vue-i18n
- 表单：vee-validate / FormKit
- Monorepo：pnpm workspace（与 server 共享 OpenAPI 类型）

---

## 4. 推荐的可观测性指标

### 4.1 协议端（Prometheus 指标，强烈建议）

| 指标 | 类型 | 标签 |
|---|---|---|
| `ota_manifest_requests_total` | Counter | `project`, `channel`, `branch`, `runtime`, `platform`, `protocol_version`, `result`(update/no_update/rollback/error) |
| `ota_manifest_request_duration_seconds` | Histogram | 同上 |
| `ota_asset_requests_total` | Counter | `project`, `runtime`, `platform`, `asset_type`(launch/static), `cache`(hit/miss) |
| `ota_asset_bytes_sent_total` | Counter | 同上 |
| `ota_signing_failures_total` | Counter | `keyid` |
| `ota_active_devices` | Gauge（由后台聚合） | `project`, `channel`, `update_id` |
| `ota_update_adoption_ratio` | Gauge | `project`, `branch`, `update_id`（新 update 覆盖率） |
| `ota_publish_total` | Counter | `project`, `branch`, `result` |
| `ota_rollback_total` | Counter | `project`, `branch`, `type`(directive/promote) |
| `ota_client_event_total` | Counter | `event_type`(download_ok/download_fail/launch_ok/launch_crash) |
| `ota_db_query_duration_seconds` | Histogram | `query` |
| `ota_storage_op_duration_seconds` | Histogram | `op`(put/get/head/delete) |

### 4.2 日志（结构化 JSON，建议接入 ELK / Loki）

- 每个 manifest 请求：requestId、clientIp、ua、deviceId(若有)、所有 expo-* 头、命中的 update_id、是否签名、耗时
- 发布事件：actor、project、branch、update_id、size、duration
- 错误日志：堆栈 + traceId

### 4.3 链路追踪

- OpenTelemetry SDK（go-zero 内置 trace 支持），关键 span：`manifest.resolve`、`db.select_latest_update`、`storage.sign_url`、`signing.rsa_sha256`
- 透传 traceId 到客户端事件回报，端到端关联

### 4.4 业务大盘（Grafana 面板）

- 当前在线 update 分布（饼图：哪些 update_id 还在被用）
- 新 update 24h 内的覆盖率曲线
- 不同 runtimeVersion 下的活跃比例（用于判断老 binary 是否可下线）
- 每分钟 manifest QPS + p99 延迟
- 资源缓存命中率 / 带宽消耗
- 灰度阶段错误率对比（A 组 vs B 组）

---

## 5. 推荐项目架构

### 5.1 仓库结构（Monorepo，pnpm + Go modules）

```
expo-ota/
├── server/                        # Go-Zero 后端
│   ├── api/                       # 对外 HTTP API（含协议端 + 管理 API）
│   │   ├── protocol/              # Expo Updates 协议端（公开，无鉴权）
│   │   │   ├── etc/protocol.yaml
│   │   │   ├── protocol.api       # goctl api 定义
│   │   │   ├── internal/
│   │   │   │   ├── handler/       # manifest, asset
│   │   │   │   ├── logic/         # manifest 协商核心逻辑
│   │   │   │   ├── svc/
│   │   │   │   └── types/
│   │   │   └── protocol.go
│   │   └── admin/                 # 管理 API（鉴权 + RBAC）
│   │       ├── admin.api
│   │       └── ...
│   ├── rpc/                       # 可选：内部 zRPC（如多服务拆分）
│   │   ├── publisher/             # 处理上传 / 解析 bundle
│   │   └── metrics/               # 异步聚合活跃设备 / 覆盖率
│   ├── internal/
│   │   ├── domain/                # 实体：Project, Branch, Update, Asset...
│   │   ├── repository/            # Postgres 仓储（建议 sqlc / gorm / ent 二选一）
│   │   ├── service/               # 业务服务：PublishService, RolloutService, SigningService
│   │   ├── manifest/              # 协议组装（multipart、SFV header、signing）
│   │   ├── storage/               # 对象存储抽象（S3, MinIO, 本地 fs）
│   │   ├── signing/               # RSA-SHA256、密钥管理
│   │   ├── auth/                  # JWT、OIDC、RBAC
│   │   ├── events/                # 客户端事件收集（kafka/nats 可选）
│   │   ├── metrics/               # Prom 指标
│   │   └── pkg/                   # 通用工具
│   ├── migrations/                # SQL migrations（推荐 goose / atlas）
│   ├── deploy/                    # Dockerfile, helm chart, k8s yaml
│   └── go.mod
├── dashboard/                     # Vue 3 前端
│   ├── src/
│   │   ├── api/                   # 自动生成的 OpenAPI client
│   │   ├── views/
│   │   ├── components/
│   │   ├── stores/
│   │   ├── router/
│   │   └── i18n/
│   ├── vite.config.ts
│   └── package.json
├── cli/                           # 上传/管理 CLI（Go，复用 server 模块）
│   └── ota/ (cobra)
└── docs/
```

### 5.2 服务分层与部署拓扑

```
┌──────────────────────────────────────────────────────────────────────┐
│                          CDN (CloudFront/CDN)                         │
│  /api/assets/*  长缓存                /api/manifest/*  no-cache       │
└──────────────────┬───────────────────────────────┬──────────────────-┘
                   │                               │
         ┌─────────▼─────────┐           ┌─────────▼─────────┐
         │  Protocol Service │           │   Admin Service   │
         │  (Go-Zero api)    │           │   (Go-Zero api)   │
         │  无状态，水平扩展    │           │   鉴权 + RBAC      │
         └─────────┬─────────┘           └─────────┬─────────┘
                   │                               │
                   ├───────────────────────────────┤
                   ▼                               ▼
         ┌──────────────────┐             ┌──────────────────┐
         │   PostgreSQL     │             │  Object Storage  │
         │  (主从 / 读写分离) │             │   (S3 / MinIO)   │
         └──────────────────┘             └──────────────────┘
                   ▲
                   │
         ┌─────────┴────────┐
         │ Redis (限流/缓存) │
         └──────────────────┘

旁路：
  - Kafka / NATS：客户端事件 → 聚合 worker → Postgres / ClickHouse
  - Prometheus + Grafana + Loki + Tempo（可观测性栈）
```

**关键设计决策：**

1. **协议端与管理端拆分**：协议端无鉴权、纯读、QPS 高；管理端写多、需鉴权。拆分便于独立扩缩容与限流。
2. **Manifest 计算结果缓存到 Redis**：以 `(channel, branch, runtime, platform, protocol_version)` 为 key，TTL 短（5-30s），命中后只需替换签名（如果客户端要签名）即可。
3. **Asset 永不可变**：URL 包含 sha256，可设极长 CDN 缓存。
4. **多租户**：所有 SQL 都带 `project_id`，由网关层注入。
5. **客户端事件回报走独立轻量端点**：批量、异步、降级时直接丢弃，不影响主链路。

---

## 6. 推荐数据库结构（PostgreSQL）

> 命名约定：snake_case；所有主键用 `uuid`（pgcrypto / uuid-ossp）；时间字段统一 `timestamptz`；逻辑删除字段 `deleted_at`。

### 6.1 ER 概览

```
organizations 1─┬─* projects 1─┬─* runtime_versions 1─* updates *─1 branches
                │              ├─* branches
                │              ├─* channels *─* channel_branch_mappings *─1 branches
                │              ├─* code_signing_keys
                │              └─* api_tokens
                └─* memberships *─1 users

updates 1─* update_assets *─1 assets   (多对多，asset 全局去重)
updates 1─* client_events
```

### 6.2 表定义（核心）

```sql
-- 组织 / 用户 / 权限
CREATE TABLE organizations (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  slug         text UNIQUE NOT NULL,
  name         text NOT NULL,
  created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE users (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  email         citext UNIQUE NOT NULL,
  password_hash text,            -- 仅密码登录时使用
  display_name  text,
  created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE memberships (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id uuid REFERENCES organizations(id) ON DELETE CASCADE,
  user_id         uuid REFERENCES users(id) ON DELETE CASCADE,
  role            text NOT NULL CHECK (role IN ('owner','admin','developer','viewer')),
  UNIQUE (organization_id, user_id)
);

-- 项目
CREATE TABLE projects (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  slug            text NOT NULL,
  name            text NOT NULL,
  default_signing_key_id uuid,        -- FK to code_signing_keys
  manifest_extra  jsonb NOT NULL DEFAULT '{}'::jsonb,   -- 全局 extra（如 EAS projectId）
  created_at      timestamptz NOT NULL DEFAULT now(),
  deleted_at      timestamptz,
  UNIQUE (organization_id, slug)
);

CREATE TABLE api_tokens (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id  uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  name        text NOT NULL,
  token_hash  text NOT NULL,        -- sha256 后存储
  scopes      text[] NOT NULL,      -- e.g. {'publish','read'}
  expires_at  timestamptz,
  created_by  uuid REFERENCES users(id),
  created_at  timestamptz NOT NULL DEFAULT now(),
  revoked_at  timestamptz
);

-- Runtime / Branch / Channel
CREATE TABLE runtime_versions (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id  uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  version     text NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now(),
  UNIQUE (project_id, version)
);

CREATE TABLE branches (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id  uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  name        text NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now(),
  archived_at timestamptz,
  UNIQUE (project_id, name)
);

CREATE TABLE channels (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id  uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  name        text NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now(),
  UNIQUE (project_id, name)
);

-- Channel 与 Branch 的灰度映射（一个 channel 可指向多个 branch，按权重）
CREATE TABLE channel_branch_mappings (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  channel_id  uuid NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
  branch_id   uuid NOT NULL REFERENCES branches(id) ON DELETE CASCADE,
  weight      int  NOT NULL DEFAULT 100 CHECK (weight BETWEEN 0 AND 100),
  filter      jsonb NOT NULL DEFAULT '{}'::jsonb, -- 高级筛选：os 版本、设备标签
  created_at  timestamptz NOT NULL DEFAULT now(),
  UNIQUE (channel_id, branch_id)
);

-- 资源（按内容寻址，去重）
CREATE TABLE assets (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  sha256          bytea NOT NULL UNIQUE,             -- 32 bytes
  size_bytes      bigint NOT NULL,
  content_type    text NOT NULL,
  storage_bucket  text NOT NULL,
  storage_key     text NOT NULL,                     -- 在对象存储中的 key
  has_br          boolean NOT NULL DEFAULT false,
  has_gz          boolean NOT NULL DEFAULT false,
  created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX assets_sha256_idx ON assets USING hash (sha256);

-- 一次发布
CREATE TABLE updates (
  id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id          uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  branch_id           uuid NOT NULL REFERENCES branches(id) ON DELETE CASCADE,
  runtime_version_id  uuid NOT NULL REFERENCES runtime_versions(id),
  platform            text NOT NULL CHECK (platform IN ('ios','android')),
  manifest_uuid       uuid NOT NULL,                 -- 协议字段 manifest.id
  launch_asset_id     uuid NOT NULL REFERENCES assets(id),
  metadata            jsonb NOT NULL DEFAULT '{}'::jsonb,  -- 协议 manifest.metadata
  extra               jsonb NOT NULL DEFAULT '{}'::jsonb,
  expo_config         jsonb,                          -- expoConfig.json 整体
  message             text,                           -- commit message
  git_commit_hash     text,
  published_by        uuid REFERENCES users(id),
  publish_source      text NOT NULL DEFAULT 'cli',    -- cli / dashboard / api
  created_at          timestamptz NOT NULL DEFAULT now(),
  deleted_at          timestamptz,
  UNIQUE (branch_id, platform, manifest_uuid)
);

CREATE INDEX updates_lookup_idx
  ON updates (branch_id, platform, runtime_version_id, created_at DESC)
  WHERE deleted_at IS NULL;

-- update <-> asset 多对多 (asset_key 是客户端用来定位资源的 key)
CREATE TABLE update_assets (
  update_id   uuid NOT NULL REFERENCES updates(id) ON DELETE CASCADE,
  asset_id    uuid NOT NULL REFERENCES assets(id),
  asset_key   text NOT NULL,             -- manifest.assets[].key (md5 hex)
  file_ext    text,                      -- manifest.assets[].fileExtension（含 .）
  PRIMARY KEY (update_id, asset_key)
);

-- 分支级回滚指令（一旦设置，对应 branch 后续 manifest 请求会返回 rollBackToEmbedded）
CREATE TABLE branch_directives (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  branch_id    uuid NOT NULL REFERENCES branches(id) ON DELETE CASCADE,
  platform     text NOT NULL CHECK (platform IN ('ios','android','all')),
  runtime_version_id uuid REFERENCES runtime_versions(id),
  type         text NOT NULL,             -- 'rollBackToEmbedded' 等
  parameters   jsonb NOT NULL DEFAULT '{}'::jsonb,
  active       boolean NOT NULL DEFAULT true,
  created_by   uuid REFERENCES users(id),
  created_at   timestamptz NOT NULL DEFAULT now()
);

-- 代码签名
CREATE TABLE code_signing_keys (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id    uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  key_id        text NOT NULL,            -- 客户端用的 keyid（如 'main'）
  algorithm     text NOT NULL DEFAULT 'rsa-v1_5-sha256',
  public_key_pem  text NOT NULL,
  private_key_enc bytea NOT NULL,         -- 用 KMS / age 加密后存储
  created_at    timestamptz NOT NULL DEFAULT now(),
  revoked_at    timestamptz,
  UNIQUE (project_id, key_id)
);

-- 客户端事件（推荐独立写入 ClickHouse；若用 PG 则按月分区）
CREATE TABLE client_events (
  id           bigserial PRIMARY KEY,
  project_id   uuid NOT NULL,
  occurred_at  timestamptz NOT NULL,
  event_type   text NOT NULL,             -- manifest_request / download_ok / download_fail / launch_ok / launch_fail / crash
  update_id    uuid,
  runtime      text,
  platform     text,
  channel      text,
  device_id    text,
  app_version  text,
  os_version   text,
  duration_ms  int,
  error_code   text,
  payload      jsonb
) PARTITION BY RANGE (occurred_at);

-- 审计日志
CREATE TABLE audit_logs (
  id           bigserial PRIMARY KEY,
  organization_id uuid NOT NULL,
  actor_user_id   uuid,
  actor_token_id  uuid,
  action          text NOT NULL,         -- publish_update / promote / set_channel / rollback / revoke_token ...
  target_type     text,
  target_id       text,
  payload         jsonb NOT NULL DEFAULT '{}'::jsonb,
  ip              inet,
  occurred_at     timestamptz NOT NULL DEFAULT now()
);

-- Webhook
CREATE TABLE webhooks (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id  uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  url         text NOT NULL,
  secret      text NOT NULL,
  events      text[] NOT NULL,
  active      boolean NOT NULL DEFAULT true,
  created_at  timestamptz NOT NULL DEFAULT now()
);
```

### 6.3 索引与性能要点

- **manifest 查询热路径**：`SELECT ... FROM updates WHERE branch_id=$1 AND platform=$2 AND runtime_version_id=$3 AND deleted_at IS NULL ORDER BY created_at DESC LIMIT 1`，已被 `updates_lookup_idx` 覆盖。
- 加 Redis 缓存：key = `manifest:{branch_id}:{platform}:{runtime_id}`，记录 update_id 与 manifest JSON 摘要；写路径（发布、回滚、channel 重定向）立即失效。
- `client_events` 按月分区 + 异步聚合到 `update_daily_stats` 物化表用于报表。
- 大字段（`expo_config`、`payload`）使用 jsonb 并按需 `->`/`->>` 索引。

---

## 7. 协议关键实现细节（避免踩坑）

下面这些点是阅读 SPEC 与示例代码后必须在 Go 端严格遵守的：

1. **`multipart/mixed`** 而非 multipart/form-data；boundary 自由，但每个 part 的 `Content-Disposition: form-data; name="manifest|extensions|directive"` 必须正确。
2. **manifest.id 必须是 UUID 格式**：示例代码用 `sha256(metadata.json)` 取前 32 hex 字符按 8-4-4-4-12 切片成 UUID。Go 中等价实现：
   ```go
   sum := sha256.Sum256(metaBytes)
   hex := hex.EncodeToString(sum[:16])
   id  := fmt.Sprintf("%s-%s-%s-%s-%s", hex[0:8], hex[8:12], hex[12:16], hex[16:20], hex[20:32])
   ```
3. **`launchAsset.fileExtension` 必须省略或被客户端忽略**，且 `contentType` 必须是 `application/javascript`。
4. **asset.key** = `md5(file_bytes)` hex；**asset.hash** = base64url(sha256(file_bytes))，**注意是 base64url（无填充）**。
5. **`expo-signature`** 是 SFV Dictionary：`sig=:base64sig:, keyid="main", alg="rsa-v1_5-sha256"`。签的是 manifest（或 directive）的 **JSON 字节序列**，所以发送前必须缓存这串 JSON 而不是重新序列化。Go 推荐用 `crypto/rsa` + PKCS1v15。
6. **`expo-current-update-id` 判等**：当客户端传的 ID 与服务端最新 manifest UUID 相同，且 protocolVersion=1，应返回 `noUpdateAvailable` directive；protocolVersion=0 则直接重新返回最新 manifest。
7. **`expo-embedded-update-id`** 用于决定 `rollBackToEmbedded` 是否还需要发送（若客户端已是 embedded，则返回 noUpdate）。
8. **`cache-control`**：manifest 必须 `private, max-age=0`；asset 应 `public, max-age=31536000, immutable`。
9. **协议版本 0 不支持任何 directive**，必须降级为返回最新 manifest 或 404。
10. **`expo-manifest-filters` / `expo-server-defined-headers`** 是 SFV Dictionary。常见用法是给设备打灰度标签：服务端通过 `expo-server-defined-headers: expo-channel-name="prod-gray"` 让客户端下次请求带回这个 header，从而完成"将一个设备锚定到一个分组"。

---

## 8. 开发路线图

### Milestone 0 · 准备（0.5 周）
- 初始化 monorepo，搭建 CI、容器化、本地 docker-compose（PG + MinIO + Redis）
- 选定 ORM（建议 `sqlc` + `pgx`，性能与类型安全俱佳）与 migration 工具（`goose`）
- goctl 生成 protocol.api 与 admin.api 骨架

### Milestone 1 · 协议最小可用（1.5 周）
**目标：能让一台真机收到一个手工导入的 update。**
- [ ] 实现 `assets` 表 + 对象存储抽象（先支持本地 fs + S3）
- [ ] 实现"导入 update"的 CLI：读取 `expo export` 产物 → 解析 `metadata.json` → 上传 asset 去重 → 写 `updates` 表
- [ ] 实现 `GET /api/manifest`：仅支持 protocol v1、normal update、不签名、固定 channel="default"
- [ ] 实现 `GET /api/assets`：从对象存储直读或返回预签名 URL
- [ ] 用 `custom-expo-updates-server` 同款 client 验证端到端流程
- [ ] 单测 + manifest 协议契约测试（构造典型请求头，断言响应结构）

### Milestone 2 · 完整协议能力（2 周）
- [ ] `noUpdateAvailable` directive
- [ ] `rollBackToEmbedded` directive（通过 `branch_directives` 表驱动）
- [ ] Code Signing（RSA-SHA256 + SFV header）+ 密钥管理 API
- [ ] `expo-manifest-filters` / `expo-server-defined-headers` 支持
- [ ] Brotli/Gzip 预压缩 launchAsset
- [ ] 协议 v0 兼容

### Milestone 3 · 多租户与管理 API（2 周）
- [ ] 组织 / 用户 / RBAC / JWT / API Token
- [ ] 项目、Runtime、Branch、Channel CRUD
- [ ] Channel → Branch 路由（基础 100% 模式）
- [ ] 发布上传 API（替代手工 CLI）
- [ ] 审计日志 + Webhook

### Milestone 4 · Dashboard MVP（2 周）
- [ ] 登录、组织/项目切换、成员管理
- [ ] 项目主页、Branch 详情、Update 详情
- [ ] 发布向导（拖入 zip）
- [ ] Channel 路由编辑
- [ ] 签名密钥管理

### Milestone 5 · 灰度与高级发布（1.5 周）
- [ ] `channel_branch_mappings.weight` 按 deviceId 哈希分组
- [ ] Promote / Rollback 一键操作
- [ ] 基于 metadata filter 的灰度（如按 OS 版本）

### Milestone 6 · 可观测性（1.5 周）
- [ ] Prometheus 指标全量埋点
- [ ] 客户端事件回报端点 `POST /api/events`（批量、限流）
- [ ] OpenTelemetry traces / structured logs
- [ ] Grafana 仪表盘 JSON
- [ ] Dashboard 内嵌统计页：覆盖率、错误率、激活漏斗

### Milestone 7 · 生产化（1 周）
- [ ] CDN 接入文档 + Cache-Control 策略验证
- [ ] 资源 GC（清理未被任何 update 引用的 asset）
- [ ] 备份与恢复（PG + 对象存储）
- [ ] 限流、IP 黑白名单、abuse 检测
- [ ] Helm chart / k8s 部署 manifest
- [ ] 安全审计：密钥加密存储（KMS / age）、JWT 安全、CSRF

### Milestone 8 · 进阶（按需）
- [ ] 客户端事件接入 ClickHouse 做大盘
- [ ] 自定义 directive（强制升级、弹窗通知等）
- [ ] GitHub Action 集成（push 触发 publish）
- [ ] CLI 二进制发布 + 自动补全
- [ ] 多区域部署（多 region 对象存储 + GeoDNS）

---

## 9. 风险与注意事项

| 风险 | 缓解 |
|---|---|
| Asset URL 一旦发布不能变更 | 强制以 sha256 命名 + 不可删除 + 通过 CDN 直出 |
| Runtime version 不匹配会导致 app crash | Dashboard 上明确高亮、发布时强校验 |
| 签名密钥泄露 | 私钥必须 KMS 加密；支持密钥轮换；操作要审计 |
| 客户端缓存过期可能引起服务端"重发" | `cache-control: private, max-age=0` 强制；用 `expo-current-update-id` 防重复下载 |
| 灰度策略写错导致全员升级 | Dashboard 在保存前展示"预计受影响设备数"二次确认 |
| 上传 update 中途失败造成孤儿 asset | 事务化写入：先入库 update（pending），全部 asset 落库后置 active；异步 GC pending |
| 协议 v0/v1 兼容 | 单测覆盖所有头组合；旧客户端按 v0 行为兜底 |

---

## 10. 附录：推荐第三方库

**Go 端**

- 框架：`github.com/zeromicro/go-zero`
- DB：`github.com/jackc/pgx/v5` + `sqlc-dev/sqlc`
- Migration：`pressly/goose`
- 对象存储：`aws/aws-sdk-go-v2` + 兼容 MinIO
- JWT：`golang-jwt/jwt/v5`
- 结构化头：自实现 SFV（参考 IETF RFC 8941）或 `github.com/dunglas/httpsfv`
- 签名：`crypto/rsa`、`crypto/sha256`（标准库）
- Multipart：`mime/multipart`
- 指标：`github.com/prometheus/client_golang`
- Tracing：`go.opentelemetry.io/otel`
- 测试：`stretchr/testify` + `testcontainers-go`

**Vue Dashboard 端**

- 框架：Vue 3 + TS + Vite
- UI：Naive UI / Ant Design Vue
- 图表：ECharts + vue-echarts
- 状态：Pinia
- 路由：Vue Router
- 请求：Axios + OpenAPI 生成
- 表单校验：vee-validate + zod
- 国际化：vue-i18n
- 测试：Vitest + Playwright

---

## 11. 验证清单（上线前必过）

- [ ] 用未签名的 release build 能拉到 update 并启动
- [ ] 用 signed release build 能拉到带 `expo-signature` 的 manifest 并验签通过
- [ ] 同一个 update 第二次冷启动返回 `noUpdateAvailable`，客户端不重复下载
- [ ] `rollBackToEmbedded` 能让客户端回到内嵌版本
- [ ] runtime version 不匹配时返回 404 / 无 update
- [ ] iOS / Android 双端通过
- [ ] Asset URL 用浏览器直接打开能下载，且 CDN 命中
- [ ] 5k QPS 压测下 manifest p99 < 100ms（开启 Redis 缓存）
- [ ] 灰度 50% 时设备分布稳定（同 deviceId 多次请求结果一致）
- [ ] 撤销 API Token 后 60s 内拒绝请求
- [ ] 全链路 trace 在 Grafana 可见

---

**完成以上路线图后，你将得到一个功能与 EAS Update 主流场景对齐、可独立部署、可观测、可灰度的自托管 Expo OTA 服务。**
