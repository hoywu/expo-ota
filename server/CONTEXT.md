# Server Context

Go-Zero 后端 + PostgreSQL + 腾讯云 COS 适配 + 协议端点实现。所有领域词汇以本文档为准。

## 领域实体

**App**:
一个 Expo/RN 应用，由 `appSlug` 全局唯一标识，创建时输入并永久只读。
_Avoid_: project, application, bundle

**RuntimeVersion**:
描述原生层 JS 接口兼容性的字符串，由 `expo export` 产物中的 `metadata.json` 给出。
_Avoid_: native version, build version, sdk version

**Update**:
一次发布产物，对应协议中的 manifest + assets 集合。状态为 `pending` 或 `published`；`pending` 不可被 manifest 查询，finalize 后仅保存草稿，管理员点击发布后切到 `published`。`published_at` 表示真正对客户端生效的时间。
_Avoid_: release, build, version

**Asset**:
按 sha256 内容寻址的不可变文件。同一 App 内跨 update 自动去重。
_Avoid_: file, resource, blob

**Manifest**:
描述一次 update 内容的 JSON 响应体。包含 `id`（UUID）、`createdAt`、`runtimeVersion`、`launchAsset`、`assets[]`、`metadata`、`extra`。
_Avoid_: payload, response body

**ClientEvent**:
客户端主动上报的 `update_succeeded` / `update_failed` 事件，用于可观测性。Dashboard 通过 `GET .../client-events` 展示最近 100 条原始行（按 `manifest_uuid` 关联 update）。
_Avoid_: telemetry, log, trace

**ManifestRequest**:
服务端在 manifest 端点观察到的请求行（不依赖客户端主动上报）。
_Avoid_: log, request record

**AuditLog**:
所有管理写操作的审计记录，含 actor、target、payload、IP、UA、时间。
_Avoid_: log, history

## 发布流

**Plan**:
发布第一阶段：CLI 提交 metadata + asset 列表，服务端返回缺失资产的 COS pre-signed PUT URL 与可复用资产列表。
_Avoid_: stage, prepare

**Finalize**:
发布第二阶段：CLI 通知服务端所有资产已上传，服务端 HEAD 校验后原子写入 `updates` 与 `update_assets` 表，并生成 `pending` 草稿 update。
_Avoid_: commit, publish

**Publish**:
管理员将已 finalize 的 `pending` update 切到 `published`，并写入 `published_at`；协议端只下发已发布 update。
_Avoid_: finalize, deploy

**RepublishPrevious** (UI 名称: Rollback):
复制历史 update 为一条新 update，资产复用，URL 不变。
_Avoid_: rollback（保留作协议 directive 名）、revert

## 协议

**AppSlug**:
App 的 URL 路径段，与 EAS 的 projectId 语义对应。`^[a-z0-9][a-z0-9-]{1,30}[a-z0-9]$`。
_Avoid_: project id, app id（DB 内部用 uuid）

**DeviceId**:
客户端首次启动生成的稳定 UUID，存 AsyncStorage，用于事件上报幂等与限速 key。
_Avoid_: installation id, device token

**RuntimeVersionID**:
`runtime_versions` 表的内部 UUID PK，对外用 `version` 字符串。
_Avoid_: native id

## 鉴权

**User**:
管理员账号，无角色区分。任意 user 等价，可访问全部 app。
_Avoid_: admin（保留 UI 文案）, operator

**ApiToken**:
CLI/CI 发布的长期凭据，形态 `ota_pat_<32 bytes base62>`，DB 存 sha256 哈希。
_Avoid_: api key, access token, pat

**CodeSigningKey**:
每 App 一对 RSA-2048 公私钥。私钥 AES-256-GCM 加密存 DB。
_Avoid_: signing key, cert key

## 不可变约束

- Asset URL 不可变：以 sha256_base64url 命名，COS 路径级公有读，禁止修改或删除
- AppSlug 创建后不可改：URL 里有"承诺"语义，修改会强制 binary 升级
- Manifest UUID 在 publish 时一次性计算并持久化：避免不同请求算出的 UUID 漂移
- PublishedAt 表示真正对客户端生效的发布时间；latest update 以 `published_at` 排序，不以 `created_at` 排序
- Update 软删后不可恢复：硬删除方向走到底
