# expo-open-ota 项目问题分析与汇总

> 分析基于：
> - 仓库 `axelmarciano/expo-open-ota`（本地 `expo-open-ota/`，README + 全部源码）
> - Expo Updates 协议规范 v1（<https://docs.expo.dev/technical-specs/expo-updates-1/>）
> - `expo-updates` 客户端 README（`expo/packages/expo-updates/README.md`）
> - Expo 官方示例服务端 `custom-expo-updates-server`（本地 `custom-expo-updates-server/`）
> - 已完成的 xavia-ota 分析报告 `xavia-ota-analysis.md`（作为横向对比基线）

expo-open-ota（下文简称 EOOTA）是用 Go 编写的 expo-updates 协议第三方服务端实现，提供 S3/GCS/本地三种存储后端、CloudFront/GCS-direct CDN 重定向、Redis/本地缓存、Prometheus 指标、官方 CLI（`apps/eoas`）、Admin Dashboard SPA，以及与 Expo 官方 `branch/channel` 概念对齐的元数据存取。**整体工程化程度明显高于 xavia-ota**，但仍存在若干 **协议正确性、严重安全漏洞、并发/一致性、可观测/可维护性** 问题。下面按类别汇总。

---

## 1. 协议正确性 / 与 Expo Updates v1 规范一致性

### 1.1 资产 URL 不包含 `updateId`，引发跨 update 资产错配（严重）

`internal/update/updates.go:306-319` 中 `BuildFinalManifestAssetUrlURL` 构造的资产 URL：

```go
query.Set("asset", assetFilePath)
query.Set("runtimeVersion", runtimeVersion)
query.Set("platform", platform)
query.Set("branch", branch)
```

资产 URL 中 **不携带 `updateId`**。在 `AssetsHandler → getAssetMetadata`（`internal/assets/assets.go:47`）中，服务器又用 `GetLatestUpdateBundlePathForRuntimeVersion(branch, runtimeVersion, platform)` 取 **当前最新 update** 来定位资源。

后果：

- **竞态**：客户端取到 manifest A（id=100），还没拉完资产，发布了 update B（id=101）。客户端继续按 manifest A 中那串 URL 请求资产时，服务端返回的是 update B 中同名资产的字节，可能与 A 中的 `hash`/`key` 不匹配，触发 `expo-updates` 客户端 hash 校验失败、整次更新作废。
- 同时违背了 manifest assets 的 **immutable + max-age=31536000** 语义（`internal/assets/assets.go:87,112`）——一个被声明为永久缓存的 URL，实际指向的内容会随时间变化。这会污染中间 CDN / 客户端 HTTP cache，造成长期错配。
- 当 update A 中存在某资产、B 中不存在或路径变了时，客户端会拿到 404；当 A 已被 `markUpdateAsUploaded` 判定为 "与最新一致" 而删除时（见 §3.1），所有引用 A 的客户端瞬间 404。
- CDN 路径（`CloudfrontCDN.ComputeRedirectionURLForAsset`，`internal/cdn/cloudfront.go:57`）反而带了 `updateId`——这意味着 **直连资产路径与 CDN 路径行为不一致**：开启 CDN 时是正确的 per-update，关闭 CDN 走源站时是错的 latest-update。

修复方向：在 `BuildFinalManifestAssetUrlURL` 中加入 `updateId`，并在 `AssetsHandler` 中读取该参数定位到具体 update。

### 1.2 `manifest.createdAt` 取自 S3 `LastModified`，不是 update 的真实创建时间（中等）

`internal/update/updates.go:269,279`：

```go
createdAt := file.CreatedAt  // s3 LastModified of metadata.json
...
metadata.CreatedAt = createdAt.UTC().Format("2006-01-02T15:04:05.000Z")
```

而 `bucket.GetFile` 返回的 `CreatedAt` 在 S3/GCS 中是 **LastModified**（`internal/bucket/s3Bucket.go:239`）。每次 `update-metadata.json` 被 `markUpdateAsUploaded`/`MarkUpdateAsChecked` 重写时（`internal/update/updates.go:50-77`，`StoreUpdateUUIDInMetadata` 调用 `UploadFileIntoUpdate`），LastModified 都会前移。

后果：

- `createdAt` 会在 update 已经发布后再次跳到 "当时检查时间"，并非真实创建时间。
- 由于 manifest 被缓存（`ComputeUpdataManifestCacheKey`），实际看到的 `createdAt` 取决于缓存何时被刷新，给灰度/统计带来扭曲。
- 与 xavia-ota "始终用 now"（见 xavia 报告 §1.1）相比有所好转，但仍未达到 Expo 官方参考实现使用文件 `birthtime` 的语义。

建议：将真实 `createdAt` 持久化到 `update-metadata.json`（在 `RequestUploadUrlHandler` 阶段写入 `updateId` 对应的毫秒时间戳，本身已等价创建时间，可直接 `time.UnixMilli(updateId)` 输出）。

### 1.3 `metadata` 仅包含 `branch`，未承载真正过滤维度（轻微）

`computeManifestMetadata`（`internal/update/updates.go:399-409`）只生成：

```json
{"branch": "<branchName>"}
```

并通过 `expo-manifest-filters: branch="..."` 一起下发。能覆盖最常见的 channel→branch 过滤场景。但：

- 缺少把 `runtimeVersion`/`platform`/`commitHash`/`message` 等也加进去的能力，无法支持更细的客户端过滤策略。
- 不支持自定义过滤 key——若希望按 "灰度桶" 选择 update，需要在协议层把桶 ID 加入 `manifest-filters`。

### 1.4 未发送 `expo-server-defined-headers`（轻微）

规范允许服务器声明 "客户端 MUST 在本地存储并在下次请求时回传" 的头。EOOTA 在 `manifest_handler.go:54-67 writeResponse` 中仅设置：

```
expo-protocol-version, expo-sfv-version, cache-control, content-type, expo-manifest-filters
```

未实现 `expo-server-defined-headers`。当前实现不致命，但失去了服务端跨请求维护客户端状态（灰度桶、A/B 分组）的能力。

### 1.5 `expo-signature` 硬编码 `keyid="main"`（中等）

`internal/handlers/manifest_handler.go:82`：

```go
headers["expo-signature"] = []string{fmt.Sprintf("sig=\"%s\", keyid=\"main\"", signedHash)}
```

`keyid` 永远是字面量 `"main"`，未读取配置；用户即使在 `keyStore` 中按 keyid 注册多个公钥也无法选择。同时 **未声明 `alg`** 参数（规范允许默认 `rsa-v1_5-sha256`，但显式更稳）。

后果：

- 与客户端 `expo-updates` 通过 `Updates.codeSigningCertificate` 配置的 `keyid` 校验不上时（多数模板使用 generated keyid 而非 `main`），会被丢弃整个 manifest，更新静默失败。
- 无法做 key rotation：旧客户端验签需要旧 keyid，新发布想换 keyid 时没有出路。

### 1.6 `signDirectiveOrManifest` 二次序列化导致签名漂移风险（低）

`internal/handlers/manifest_handler.go:38-52`：

```go
contentJSON, err := json.Marshal(content)
signedHash, err := crypto.SignRSASHA256(string(contentJSON), privateKey)
```

`putResponse` 后续又 `json.Marshal(content)` 一次写入 multipart 主体（`createMultipartResponse`），两次 Marshal 在 Go 中通常产出相同字节，但因为 `manifest.Metadata` 与 `manifest.Extra.ExpoClient` 是 `json.RawMessage`，而 Go map field 序列化是确定的 → 实际可重复。**不致命但很脆弱**：一旦其中任何字段改成 `map[string]interface{}`，键序不固定，签名/响应字节就会发散。

建议：先序列化一次得到 `body`，再 `crypto.Sign(body)`，再把 `body` 写入响应体。

---

## 2. 安全 / 鉴权

### 2.1 `/rollback/{BRANCH}` 与 `/republish/{BRANCH}` 任何 Expo 用户都能调用（严重，可远程篡改生产更新）

`internal/handlers/rollback_handler.go:31-41` 和 `internal/handlers/republish_handler.go:32-42` 都只调用：

```go
expoAccount, err := services.FetchExpoUserAccountInformations(expoAuth)
```

——只确认 token 能在 Expo API 解出某个账号，**没有调用 `services.ValidateExpoAuth`**。后者额外做了：

```go
if selfExpoUsername != expoAccount.Username {
    return nil, errors.New("expo account does not match self expo username")
}
```

而 `selfExpoUsername` 取自服务自身配置的 `EXPO_ACCESS_TOKEN`（`services.FetchSelfExpoUsername`），用来确认调用者就是 owner。

后果：

- 任何在 expo.dev 注册账户的攻击者，只要拿到自己的 Expo 个人 token / session（或随便注册一个新账号去申一份），向这台服务器 POST `/rollback/<branch>?platform=ios&runtimeVersion=...` 即可 **立刻把生产分支回退到 embedded 版本**；
- 同理可 `/republish` 把某个旧 update 重新发布为新 update，永久卡住任何后续静默升级链路；
- 由于路由 **没有任何 IP/CORS/鉴权前置过滤**，公网可达的实例都暴露在该漏洞下。

这是本项目最严重的问题。对比 `/markUpdateAsUploaded` 与 `/requestUploadUrl` 已正确使用 `ValidateExpoAuth`，可见是开发者漏改。修复：把两个 handler 替换为 `services.ValidateExpoAuth`。

### 2.2 EOAS CLI 与服务端共享同一 Expo 账号→单租户绑死（架构性）

`ValidateExpoAuth` 要求调用者 username 等于服务自身配置的 `EXPO_ACCESS_TOKEN` 解出的 username。这隐含：

- 服务端只能服务 **同一个 Expo 账号** 拥有的 app；多团队/多公司无法共享一套部署。
- 如果该 owner token 泄露/轮换，整条发布链路同步中断。
- 没有更细的角色/scope 控制：能发布即能 rollback 即能 republish。

设计建议：引入显式的 "uploader" 列表（基于 Expo userId 或 app collaborators role）替代 "唯一 owner" 检查。

### 2.3 Dashboard JWT：`fmt.Errorf` 当成日志使用，错误被吞（低，但暴露代码质量）

`internal/auth/auth.go:21-28`：

```go
func isPasswordValid(password string) bool {
    adminPassword := getAdminPassword()
    if adminPassword == "" {
        fmt.Errorf("admin password is not set, all requests will be rejected")
        return false
    }
    return password == getAdminPassword()
}
```

`fmt.Errorf` 返回一个 error 对象但未被赋值/打印——这是典型的误用，意图应是 `log.Printf` 或 `log.Println`。功能上 `return false` 没问题，但说明此处缺少 **静态分析/lint** 卡点，类似问题大概率在别处也存在（`middleware/auth_middleware.go:16,19` 就有 `fmt.Println("lel", err)` 这种调试残留日志直接进了生产代码）。

### 2.4 JWT 仅 HS256 + 单一 `JWT_SECRET`，无 key rotation / 撤销名单（低）

`internal/auth/auth.go` 用对称密钥签发 admin 与 refresh token，秘钥来自环境变量。生产建议至少：

- 加 `kid` 头，便于轮换；
- refresh token 应可吊销（目前没有 jti/黑名单，被偷的 7 天 refresh 完全不可挽回）。

### 2.5 缓存 5 分钟 Expo 账号鉴权，撤销/封禁有最长 5 分钟窗口（低）

`services.FetchExpoUserAccountInformations` 用 token sha256 做 key，TTL 5 分钟（`internal/services/expo.go:308`）。如果 Expo 侧封禁了某 token，本服务最多还会信任它 5 分钟。

### 2.6 Dashboard 静态文件路径校验薄弱（低）

`internal/router/router.go:82-87` 对 `dashboard` 路径做 prefix 校验：

```go
filePath := filepath.Join(dashboardPath, r.URL.Path[len("/dashboard/"):])
if !strings.HasPrefix(filePath, dashboardPath) {
    http.Error(w, "Forbidden", http.StatusForbidden)
    return
}
```

`filepath.Join` 已经 normalize 了 `..`，配合 `HasPrefix` 基本能挡 path traversal。但 Windows 下大小写/分隔符差异可能产生 false negative；Linux 下还算安全。建议改用 `filepath.Rel` + 显式拒绝 `..`。

---

## 3. 一致性 / 并发 / 数据完整性

### 3.1 "上传重复内容时删除新 update 目录" 存在 TOCTOU 与误删（中等）

`internal/handlers/upload_handler.go:107-138` 流程：

1. 上传 update X 完成；
2. 服务端读 "最新已 check 的 update Y"；
3. 若 `AreUpdatesIdentical(X, Y)` → **物理删除 X 的目录**。

问题：

- 在 X 与 Y 同时上传的并发场景下，"最新 update" 是不确定的；后到者可能误判前一个为重复并删掉自己，也可能两个并发上传互相删。
- `AreUpdatesIdentical` 比较的是 `metadata.Fingerprint = sha256(metadata||branch||runtimeVersion)`，**不含 platform**——但同一 update 在 iOS / Android 是分别上传的，可能误判跨平台同 fingerprint 为重复（实际上 metadata.json 中包含 platform 区段，理论上不同，但 republish 流水线下可能完全一致）。
- 删除是 list+batch delete（`s3Bucket.go:31-90`），过程中若客户端正在用 manifest 引用其中文件，将立即 404。

### 3.2 `update-metadata.json` 写入非原子，崩溃后留下半成品（中等）

EOOTA 没有外部 DB，update 状态完全靠 bucket 内若干 marker 文件（`update-metadata.json`、`.check`、`rollback`）来表征。`MarkUpdateAsChecked → StoreUpdateUUIDInMetadata` 流程在 S3 上是 `GetObject → Decode → Marshal → PutObject`：

- 若两个并发请求同时改同一个 update 的 metadata，会出现 last-write-wins，丢失中间状态。
- 没有 If-Match / version 校验，无法 detect 冲突。
- `.check` 标记和 `update-metadata.json` 的更新不是事务的，崩溃可能留下 "metadata 已更新但 .check 缺失" 的中间态——前者影响 manifest 内容、后者影响 `IsUpdateValid` 判定，可能让 client 看到一个未被列入候选的 update。

### 3.3 `RequestUploadUrlHandler` 立刻预写 `update-metadata.json`（中等）

`internal/handlers/upload_handler.go:271-283`，签发 presigned URL 的 **同一请求** 就把 `update-metadata.json` 写进 bucket。后果：

- 列表（`GetUpdates`）会看到一个目录、但里面只有 metadata，没有任何资产；这条 update 不会通过 `IsUpdateValid`（无 `.check`），所以不会被发给客户端，**但 dashboard 的 update 列表会看到 "幽灵" update**（`apps/dashboard` 列表来自 `GetUpdates`，没有过滤 `.check`，需另行确认）。
- 客户端如果只调 presigned URL 上传到一半失败、不再调 `markUpdateAsUploaded`，残留目录永远不会被回收。**缺少 GC**。
- `platform` 字段在该阶段是可选的；如果客户端不传，写入 `{"platform": "", ...}`，后续 `filterPlatformUpdates` 永远过滤不到它，但仍占空间。

建议：要么 presign 阶段不写 metadata（推迟到 mark 阶段），要么加后台 GC 扫超过 N 小时未 `.check` 的 update 目录。

### 3.4 `GetLatestUpdateBundlePathForRuntimeVersion` 缓存 30 分钟 + `MarkUpdateAsChecked` 主动失效，但缺少跨实例同步（中等）

`internal/update/updates.go:217` 写入 cacheKey TTL 1800s。`MarkUpdateAsChecked` 会 `cache.Delete(...)`（同上 88-91 行）。但：

- 当使用 `localCache`（默认）且部署多副本（k8s 多 Pod、helm 默认 replicaCount>1），一个 Pod 上的 `MarkUpdateAsChecked` 无法让其它 Pod 立即看到新 update。其它 Pod 客户端最多要等 30 分钟才能看到新版本。
- 强制 Redis 不是默认行为，README 中没有把 "多副本部署必须 Redis" 标红。

### 3.5 `Rollback` 流程对 channel→branch 的影响未在文档说明（轻微）

`CreateRollback` 在指定 branch 内插入一个含 `rollback` 文件的伪 update，`GetUpdateType` 返回 `Rollback`，manifest 走 `rollBackToEmbedded`。然而它并不会自动撤销 channel→branch 的 mapping；如果客户端把 channel 切换到另一个 branch 后再切回，可能再次拿到老的 normal update（在 rollback update 之前的）——因为 rollback 仅作用于 update 列表里 createdAt 最大者。逻辑本身正确，但是对运维的语义边界没有写清。

---

## 4. 可观测 / 可维护性 / 代码质量

### 4.1 调试日志残留（低）

`internal/middleware/auth_middleware.go:16-19`：

```go
expoAuth := helpers.GetExpoAuth(r)
fmt.Println(expoAuth)   // 打印 token / session
...
fmt.Println("lel", err) // 调试残留
```

**会把 bearer token 写入标准输出日志**——任何接日志的系统（fluentbit, cloudwatch, loki）都会留底，构成凭证泄露。属于必须立刻删的问题。

### 4.2 `fmt.Errorf` 用作日志（如 §2.3）、Print/Println 混用 `log` 与 `fmt`，无结构化日志（低）

整个 codebase 大量 `log.Printf("[RequestID: %s] ...")`，但没有 zap/zerolog 之类的结构化日志，难以接 ELK/Loki 做字段查询。`fmt.Println("Updating channel...")`（`services/expo.go:168`）等也是直出 stdout 无 level。

### 4.3 测试覆盖很不均衡

只有 `internal/bucket/*_test.go`、`internal/cache/cache_test.go`、`internal/branch/branch_test.go`、`internal/cdn/cdn_test.go`。**最关键的 manifest 组装、签名、rollback、republish、AssetsHandler 都没有单元测试**。这也是 §2.1 那种关键鉴权漏掉一行的根本原因。

### 4.4 `auth_middleware` 既支持 dashboard JWT 又支持 Expo 账号鉴权，靠 `Use-Expo-Auth` header 切换（轻微）

`internal/middleware/auth_middleware.go:13-25`，客户端可自行选择鉴权模式。该模式倒不构成绕过（两种模式都各自验证），但应当 **在路由层固定**，而不是让请求自报模式——后者降低审计可读性。

### 4.5 `internal/services/expo.go:168 fmt.Println("Updating channel branch mapping for channel:", ...)` 等大段裸 `fmt.Println` 会污染生产日志（低）

### 4.6 Asset 压缩同步内存编码（中）

`compression.ServeCompressedAsset` 把整个资产载入 `[]byte` 再做 gzip/brotli 编码（`internal/compression/compression.go:35`）。对一个 10MB 的 hermes bundle，并发 100 个请求即吃 1GB 内存。建议改成 streaming（`io.Copy` + `gzip.Writer`），同时设置 `Vary: Accept-Encoding`（当前 **未设**，会让中间 CDN 错误共享不同编码版本）。

### 4.7 Manifest/Asset cache 写入永不过期（中）

`ComposeUpdateManifest` 与 `shapeManifestAsset` 调用 `cache.Set(..., nil)`——`nil` TTL 在 `localCache` 与 `redisCache` 中均代表永不过期。考虑到 manifest 的 `Id` 是 update 内容 hash，永久有效逻辑上对；但只要换了协议字段（升级了 EOOTA 版本），所有旧条目都成为不可清理的脏数据，必须依赖 `version.Version` 前缀失配 + LRU 自然淘汰。Redis 上无 LRU 默认 policy 时会持续累积内存，需运维显式设 maxmemory-policy。

### 4.8 `localBucket` 提供示例部署，但 `helpers/url.go` 等存在轻量逻辑没有针对路径越界做防御

未深入逐一审计；如开放公网 self-host，建议把 `localBucket` 标注为 "dev only"，强制生产用 S3/GCS。

---

## 5. 与 xavia-ota 的逐项对比

| 维度 | xavia-ota | expo-open-ota | 优劣 |
| --- | --- | --- | --- |
| 实现语言 / 框架 | Next.js + TS API Routes | Go + gorilla/mux | EOOTA 部署更轻、内存/并发更好 |
| `createdAt` | 始终用 `now()`，完全错（§1.1） | 用 S3 LastModified，部分错（§1.2） | **EOOTA 更接近正确** |
| `metadata` 字段 | 始终 `{}`（§1.2） | `{"branch":"..."}`（§1.3） | EOOTA 更对 |
| `expo-manifest-filters` 头 | 缺失（违反 MUST） | 已发送 `branch="..."` | **EOOTA 正确** |
| `expo-server-defined-headers` 头 | 缺失 | 缺失 | 持平（均缺，但非 MUST） |
| Channel ↔ Branch | 无 channel 概念 | 与 Expo 官方 GraphQL channel/branch 同步 | **EOOTA 显著更对** |
| Rollback 协议（`rollBackToEmbedded`） | 不支持 | 支持 | **EOOTA 显著更对** |
| Code signing | 支持但 keyid 未深入 | 已实现签名但 keyid 硬编码 `"main"`（§1.5） | 持平偏弱 |
| 资产 URL 含 `updateId` | ✗ | ✗（CDN 路径含、源站路径不含，§1.1） | **均错，EOOTA 部分缓解** |
| 上传方式 | POST body 经 Next.js 4MB body 限制，必须分片 | presigned URL 直传 S3/GCS | **EOOTA 显著更好** |
| 鉴权 | 共享 `UPLOAD_KEY` 单字符串 | Expo OAuth（owner-only） | **EOOTA 显著更安全**…… |
| 鉴权漏洞 | — | **rollback / republish 鉴权不完整（§2.1）** | **EOOTA 出现 critical** |
| 多租户 | 单租户 | 单租户（owner-bound，§2.2） | 均为单租户 |
| 外部 DB | Postgres | 无 DB，全部存 bucket | EOOTA 部署更简单，但缺事务性（§3.2） |
| CDN 重定向 | 无 | CloudFront / GCS signed URL | **EOOTA 显著更好** |
| 压缩 | 无 | brotli/gzip on-the-fly（内存全载，§4.6） | EOOTA 偏好但实现有缺陷 |
| Cache 层 | 无 | Redis / Local 可插拔 | **EOOTA 更好** |
| Metrics / Dashboard | 无 | Prometheus + Dashboard SPA + Grafana 模板 | **EOOTA 显著更好** |
| 官方 CLI | 无（依赖手工 curl/脚本） | `apps/eoas` CLI + Github Action | **EOOTA 显著更好** |
| 测试覆盖 | 较少 | 主要逻辑无测试（§4.3） | 均薄弱 |
| 代码质量小问题 | TypeScript 较干净 | 残留 `fmt.Println("lel"...)` 等（§4.1） | xavia 略好 |

### 选型建议

- **个人/小团队、需求简单、看重对 Expo 协议完整支持（含 channel/rollback/code signing）+ CDN/Dashboard/CLI**：推荐 EOOTA，但务必在部署前自行修复 §2.1（rollback/republish 鉴权）、§4.1（日志泄露 token）。
- **完全自托管、想要 SQL 可视化、暂不需要 rollback/CDN**：xavia-ota 上手更快，但要承担 §1.1/§1.2/§1.3 等多个协议不符问题；几乎无法长期演进到生产。
- **大规模 / 多团队 / 高 SLA**：两者都不直接合适。EOOTA 架构更近商用，但缺少 §2.2 的多租户、§3 的并发一致性、§4.6 的资产 streaming，需要 fork 改造。

---

## 6. 严重程度汇总表

| 编号 | 问题 | 类别 | 严重程度 | 修复成本 |
| --- | --- | --- | --- | --- |
| §2.1 | rollback/republish 未调用 `ValidateExpoAuth`，任何 Expo 账户可任意操作 | 安全 | **Critical** | 极低（两行修复） |
| §4.1 | 中间件 `fmt.Println(expoAuth)` 把 token 写日志 | 安全 | **High** | 极低 |
| §1.1 | 资产 URL 不含 `updateId`，跨 update 错配 + 破坏 immutable cache | 协议正确性 | **High** | 中 |
| §3.1 | "重复 update 自动删除" 在并发下误删 / 数据丢失 | 一致性 | **High** | 中 |
| §1.5 | `expo-signature` keyid 硬编码 `"main"`、不带 `alg` | 协议正确性 | **Medium** | 低 |
| §1.2 | `createdAt` 使用 metadata.json 的 LastModified，会被 `MarkUpdateAsChecked` 漂移 | 协议正确性 | **Medium** | 低 |
| §3.2 | metadata/.check 非原子写、无版本号 | 一致性 | **Medium** | 中 |
| §3.3 | presign 阶段就写 metadata，残留目录无 GC | 一致性/容量 | **Medium** | 中 |
| §3.4 | 多副本部署下 lastUpdate 缓存 30 分钟跨实例不一致 | 一致性 | **Medium** | 低（默认走 Redis） |
| §4.6 | 资产压缩全载内存且未设 `Vary` | 性能/缓存 | **Medium** | 中 |
| §4.7 | manifest/asset cache 无 TTL | 容量 | **Low** | 低 |
| §2.2 | 单 owner 模型，无多租户 | 架构 | **Low**（取决于使用场景） | 高 |
| §2.4 | JWT 单密钥无 rotation/revocation | 安全 | **Low** | 中 |
| §2.5 | Expo 鉴权缓存 5 分钟，撤销有窗口 | 安全 | **Low** | 低 |
| §2.6 | dashboard 静态文件 path 校验稍弱（Linux 已 OK） | 安全 | **Low** | 低 |
| §1.3 | `metadata` 仅含 branch，无法更细粒度过滤 | 能力 | **Low** | 低 |
| §1.4 | 未发送 `expo-server-defined-headers` | 能力 | **Low** | 中 |
| §1.6 | 签名前二次 Marshal，存在漂移风险 | 健壮性 | **Low** | 低 |
| §2.3 / §4.2 / §4.5 | 残留 `fmt.Errorf`/`fmt.Println` 当日志、无结构化日志 | 可维护性 | **Low** | 低 |
| §4.3 | 关键路径无单元测试 | 可维护性 | **Medium** | 中 |
| §4.4 | `Use-Expo-Auth` 头由客户端切换鉴权模式 | 设计 | **Low** | 低 |
| §3.5 | rollback 与 channel mapping 交互未文档化 | 文档 | **Low** | 低 |

---

## 7. 推荐改进路线（按 ROI 排序）

1. **立即修复（一个 PR 即可）**：
   - 把 `rollback_handler.go`、`republish_handler.go` 中的 `FetchExpoUserAccountInformations` 改成 `ValidateExpoAuth`（§2.1）。
   - 删除 `auth_middleware.go` 中的 `fmt.Println(expoAuth)`、`fmt.Println("lel", err)` 调试日志（§4.1）。
   - `expo-signature` 的 `keyid` 改成读取配置或 cert 自带 keyid，并显式带 `alg=...`（§1.5）。
2. **本周内可上**：
   - `BuildFinalManifestAssetUrlURL` 加入 `updateId` 参数；`AssetsHandler` 用该参数定位 update（§1.1）。
   - `createdAt` 改成 `time.UnixMilli(updateId)`（§1.2），避免被 `MarkUpdateAsChecked` 漂移。
   - 加 `Vary: Accept-Encoding` 头到资产响应（§4.6）。
3. **下阶段（需要一些设计）**：
   - 引入后台 GC 清理无 `.check` 标记且超过 N 小时的 update 目录（§3.3）。
   - 给 `update-metadata.json` 写入加 If-Match / ETag 乐观锁（§3.2）。
   - 把 "比对最新 update 是否相同后自动删" 改成 dry-run / 标记 `.duplicate` 文件而不删（§3.1），由用户决定。
   - 多副本部署文档明确强制 Redis（§3.4）。
4. **战略级**：
   - 多租户：用 `EXPO_APP_ID` + Expo collaborators 角色替代 "owner only"（§2.2）。
   - Code signing：支持多 keyid 同步，便于轮换（§1.5、§2.4）。
   - 关键路径 e2e 测试（manifest/rollback/republish/CDN）（§4.3）。
   - 结构化日志（zap/zerolog），并把客户端 token 列入 redact 规则（§4.2）。

---

## 8. 结论

- **横向**：EOOTA 是目前主流第三方 Expo Updates 服务端实现里 **架构最完整** 的开源选项，从 channel/branch、code signing、rollback、CDN、cache、metrics、dashboard、CLI 都有；明显领先 xavia-ota。
- **纵向**：协议正确性已基本贴近 Expo 官方参考实现，但 **资产 URL 不含 updateId（§1.1）** 与 **rollback/republish 鉴权缺口（§2.1）** 两点是阻塞性问题，前者会触发客户端 hash 不匹配、后者直接被远程接管发布通道。**未修这两个问题不应直接对外公网部署。**
- 修完上述 critical/high 后，本项目可以作为 EAS Update 之外一个 **可生产** 的开源替代；进一步要支持多团队 SaaS 用法，需要在多租户、并发一致性、多 keyid 三个方向再投入工程。
