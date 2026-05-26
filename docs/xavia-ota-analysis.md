# xavia-ota 项目问题分析与汇总

> 分析基于：
> - 仓库 `xavia-io/xavia-ota`（本地 `xavia-ota/`，README + 全部源码）
> - Expo Updates 协议规范 v1（<https://docs.expo.dev/technical-specs/expo-updates-1/>）
> - `expo-updates` 客户端 README（`expo/packages/expo-updates/README.md`）
> - Expo 官方示例服务端 `custom-expo-updates-server`（本地 `custom-expo-updates-server/`）
> - `xavia-io/xavia-ota` 的全部 Issues（通过 `gh issue list` 拉取）

xavia-ota 是 Next.js + TypeScript 编写的 expo-updates 协议的第三方服务端实现，整体结构清晰，但与 Expo Updates v1 协议、Expo 官方参考实现对照后，仍存在不少 **正确性、协议一致性、安全性、性能与架构** 上的问题。下面按问题类别汇总。

---

## 1. 协议正确性 / 与 Expo Updates v1 规范不一致

### 1.1 `manifest.createdAt` 永远使用 "当前时间"（严重）

`apiUtils/helpers/UpdateHelper.ts:82` 中 `getMetadataAsync` 直接返回：

```ts
createdAt: new Date().toISOString(),
```

而规范明确要求 `createdAt` 是 **update 的创建时间**（"The date and time at which the update was created…"）。Expo 参考实现使用 `metadataStat.birthtime` 读取文件的真实创建时间（`custom-expo-updates-server/expo-updates-server/common/helpers.ts:134`）。

后果：

- 客户端依赖 `createdAt` 选择 "最新 update"，xavia 总返回 `now` 等于让此字段失去语义。
- 任何基于时间窗口的灰度/过滤/缓存策略都失效。
- 单一 update 在不同请求中会得到不同 `id` 计算上下文（虽然 `id` 由 `metadata.json` 的哈希决定，但二者的语义不再匹配）。

### 1.2 `metadata` 字段总是空 `{}`（严重 / 缺失能力）

`pages/api/manifest.ts:190`：

```ts
metadata: {},
```

规范定义 `metadata` 是 string-valued dictionary，并配合响应头 `expo-manifest-filters` 用来做 **客户端过滤**（频道、分组、灰度等）。xavia 完全没有产生也没有使用它，因此根本无法实现：

- 多 channel（preview/production/staging）
- 分组/灰度/canary（对应 issue #7：明确询问 "canary or rollout releases?"）
- 按版本/平台/构建号过滤

### 1.3 缺少 `expo-manifest-filters` 与 `expo-server-defined-headers` 响应头（违反 MUST）

规范的 "Common response headers" 章节：

> `expo-manifest-filters` … **MUST** … The client library **MUST** store the manifest filters …
> `expo-server-defined-headers` … defines headers that a client library **MUST** store …

xavia 在 `manifest.ts` 的三个分支（`putUpdateInResponseAsync` / `putRollBackInResponseAsync` / `putNoUpdateAvailableInResponseAsync`）都没有发送这两个头。

### 1.4 资产 URL 不稳定，可能导致下载到错误版本的资源（严重 / 数据正确性）

`UpdateHelper.getAssetMetadataAsync` 构造的资产 URL 是：

```ts
url: `${process.env.HOST}/api/assets?asset=${arg.filePath}&runtimeVersion=${arg.runtimeVersion}&platform=${arg.platform}`,
```

没有任何 update / timestamp 标识。而 `pages/api/assets.ts:30` 处理资产请求时调用 `getLatestUpdateBundlePathForRuntimeVersionAsync`，会读取 **当前最新** 的 bundle。

这意味着：

1. 客户端先拿到 manifest A（来自 bundle v1，资产 hash 是 H1）。
2. 服务端发布新 bundle v2（同一资产路径但内容不同）。
3. 客户端下载资产时，`/api/assets?asset=…` 命中 v2 的内容（hash H2）。
4. 客户端按 manifest A 中的 H1 校验，hash 失配 → 整次更新失败；或在极端情况下加载了与 manifest 不匹配的资产组合。

规范明确：

> An asset located at a particular URL **MUST NOT** be changed or removed since client libraries may fetch assets for any update at any time.

这是对该 MUST 的直接违反，是协议级 bug。

### 1.5 `cache-control` 不符合规范（性能）

- 规范建议资产使用 `cache-control: public, max-age=31536000, immutable`。
- xavia 的 `pages/api/assets.ts` **完全没有设置 `cache-control`**，CDN/浏览器无法缓存；同时由于 1.4，本来也不能加 `immutable`。

### 1.6 不支持响应压缩

- 规范 "Compression" 章节：assets SHOULD 支持 gzip 与 brotli。
- xavia 的 `assets.ts` 仅 `res.end(asset)`，没有 `content-encoding`、没有读取客户端 `accept-encoding`、没有压缩。
- 与 issue #27（"Api routes response size limit"）叠加，对 20MB 的 HBC bundle 几乎不可用。

### 1.7 不做 `accept` 协商，未返回 406

规范要求服务端按 `accept` 头进行 proactive content negotiation，不支持的格式应返回 `406`。xavia 无论客户端 `accept` 是什么，永远返回 `multipart/mixed`。

### 1.8 `assetRequestHeaders` 残留示例假数据

`pages/api/manifest.ts:217`：

```ts
assetRequestHeaders[asset.key] = {
  'test-header': 'test-header-value',
};
```

这是直接抄自 Expo 参考服务端的示例代码，但参考代码里也只是 demo，xavia 进入生产仍未删除，导致每个客户端的每个资产请求都被告知必须带上无意义的 `test-header`。

### 1.9 代码签名实现不完整

- `expo-expect-signature` 是 SFV dictionary，可携带 `keyid` 与 `alg`。规范："the client SHOULD use the certificate that matches this `keyid` to verify the signature"，意味着服务端应使用客户端要求的 keyId。
- xavia（`pages/api/manifest.ts:212` 等多处）**硬编码 `keyid: 'main'`**，完全忽略 `expo-expect-signature` 中的 `keyid`/`alg`。
- 仅支持 `RSA-SHA256`，没有协商算法的能力。
- `expo-expect-signature` header 只用做 "是否需要签名" 的 bool 判断，且没解析 SFV，违反规范对该 header 解析的要求。

### 1.10 `launchAsset.fileExtension: '.bundle'` 与规范冲突

规范："The `fileExtension` field will be ignored for this asset and **SHOULD be omitted**." xavia 仍然主动写入 `.bundle`。属于轻微不一致（同样的小瑕疵在参考实现中存在，但 xavia 没有修正）。

### 1.11 `rollBackToEmbedded` 分支是 "死代码"

`manifest.ts` 通过 `getTypeOfUpdateAsync` 检查 zip 内是否包含名为 `rollback` 的 entry；然后调用 `createRollBackDirectiveAsync` 发出 `rollBackToEmbedded` directive。

但是查阅整个仓库（`upload.ts`、`rollback.ts`、`build-and-publish-app-release.sh`）：**没有任何路径会上传带有 `rollback` 标志的 zip**。也就是说 xavia 实际上 **从未真正使用过 `rollBackToEmbedded`**。这与 README 描述的 "rollback support" 给出的实现完全不同——见第 4 节关于回滚逻辑的问题。

---

## 2. 安全问题

### 2.1 管理 API 完全未鉴权（严重）

`pages/api/login.ts` 只是比较一个明文密码并 `return success: true`，**不下发 session/JWT/cookie**。前端只是把 "已登录" 状态存在客户端，但下列 API **完全没有任何鉴权**：

- `pages/api/rollback.ts`：任何匿名用户都能 POST 一个 `path` 让服务端回滚！
- `pages/api/releases.ts`：列出全部历史发布（path、commit hash、commit message、文件大小）。
- `pages/api/tracking/all.ts`、`pages/api/tracking/[release_id].ts`：分析数据外泄。

这是非常严重的安全漏洞。任何能访问到 OTA 域名的人都可以：

1. 列出全部历史 release，得到内部 commit 信息、bundle 路径。
2. 直接调用 `/api/rollback` 把 app 回滚到任意旧版本（含未授权的 bundle 路径），实现可定向的拒绝服务 / 引发已知漏洞的旧版本被分发。

issue #25 已经提出（"What's stopping someone else from publishing an update?"）。

### 2.2 上传只依赖一个共享密钥 `UPLOAD_KEY`（issue #17、#25）

- 仅一个长期、共享的密钥，没有过期/轮换/作用域。
- 通过 form-data 明文上传。
- 错配返回 400 而非 401/403，便于枚举。
- 没有限流。
- `process.env.UPLOAD_KEY` 没设 / 空字符串的情况下，没有 fail-closed 逻辑（值为 `undefined`，`undefined !== uploadKey` 会通过 / 失败，依赖具体值）。强烈建议至少加 minimum length & 启动期强制存在性校验。
- 任何拿到 `UPLOAD_KEY` 的人都可上传任意 JS bundle —— 这就是 issue #17 ("Possible RCE vulnerability") 的核心：上传的 JS 会被全量发到所有终端用户运行，属于 client-side RCE。

### 2.3 路径遍历（中-高）

`runtimeVersion` 直接拼接到存储路径：

```ts
const updatesDirectoryForRuntimeVersion = `updates/${runtimeVersion}`;
```

`LocalStorage` 内部使用 `path.join(this.baseDir, dirPath)`，`path.join` 会解析 `..` 段，因此攻击者可通过 `expo-runtime-version: ../../etc/passwd` 之类的 header 让服务端在 `/app/local-releases` 之外访问/写入文件。其他存储驱动（S3、GCS、Supabase）会拼出意外 key 但风险较低；本地存储的是真正的目录遍历。

修复：白名单 / 严格校验 `runtimeVersion`（如 `/^[\w.\-]+$/`）。

### 2.4 zip 文件被信任、`metadata.json` 完整性未校验

- `upload.ts` 接到 zip 后直接 `AdmZip(file.filepath)` 解析其中的 `metadata.json` 计算 hash → 落库。
- 没有校验 zip 内是否含 `expoconfig.json`、`fileMetadata` 结构是否合法、是否有非法路径条目、bundle 文件是否真的存在等。
- 若上传的 zip 携带恶意 `metadata.json`（path 指向 `..` 路径），虽然此后只是 `entryName === filePath` 精确匹配，但仍可造成 `assets.ts` 抛错 / DoS。
- 没有任何病毒/大小限制（虽然 Next 默认有 4MB body 限制，但 formidable 配置 `bodyParser: false` 后由 formidable 处理，未显式设置 `maxFileSize`，默认 200MB）。

### 2.5 `login.ts` 用普通字符串比较

`password === adminPassword` 是非常态时间安全比较，理论上可时序攻击；同时弱密码、暴力破解、限流缺失都没处理。

### 2.6 `releases` / 错误响应可能泄露内部信息

多处 `res.json({ error })` 把原始 `Error` 对象（含 message、stack 间接信息）抛给客户端。

---

## 3. 数据库 / 状态机一致性问题

### 3.1 `getLatestReleaseRecordForRuntimeVersion` 漏选 `update_id`（issue #41，确认存在的 Bug）

`apiUtils/database/LocalDatabase.ts:18-28`：

```sql
SELECT id, runtime_version as "runtimeVersion", path, timestamp, commit_hash as "commitHash"
FROM releases WHERE runtime_version = $1
ORDER BY timestamp DESC LIMIT 1
```

**没有选 `update_id as "updateId"`**，所以 `manifest.ts:65` 读到的 `releaseRecord.updateId` 永远是 `undefined`，对 `expo-current-update-id` 的快速比对永远不命中。结果：

- 每次客户端请求 manifest 都会从存储完整下载并解压 zip，再算一次 hash → 慢、昂贵、白白生成流量与 CPU。
- 在 4MB 限制（issue #27）/ 大 bundle 场景下严重拖累首字节响应时间。

### 3.2 `rollback.ts` 不写 `update_id`、不写 `commit_message` 类型断言

```ts
await DatabaseFactory.getDatabase().createRelease({
  path: newPath, runtimeVersion, timestamp: ..., commitHash, commitMessage,
});
```

少了 `updateId` 字段（类型是 optional，但实际上由 manifest 处依赖的字段），导致回滚生成的 release 记录在数据库里 `update_id IS NULL`，叠加 3.1 让 "no-update-available 短路"对回滚后的 release 完全失效。

### 3.3 Schema 用 `VARCHAR(255)` 存 commit message

`containers/database/schema/releases.sql`：`commit_message VARCHAR(255)`。超过 255 的 commit 消息会插入失败或被截断。应使用 `TEXT`。

### 3.4 `SupabaseDatabase` 已存在但 README/Docker 默认配置（`docker-compose.yml`）使用 `DB_TYPE=supabase`，而文档（README）只声称支持 PostgreSQL

`README.md` "What database options are supported?" 只列出 PostgreSQL，但 `containers/prod/docker-compose.yml` 默认 `DB_TYPE=supabase`，且 `DatabaseFactory` 中有 `SupabaseDatabase.ts`。文档/默认配置/能力声明三者不一致。

### 3.5 跟踪数据混入主请求路径

`putUpdateInResponseAsync` 在 `res.end()` 之后才同步调用 `getReleaseByPath` + `createTracking`。两条额外 SQL：

- 阻塞 Node 事件循环 / connection pool。
- `getReleaseByPath` 仅按 `path` 匹配，受 3.1 / 3.2 影响时常匹配不到，tracking 默默丢失，无告警。
- 在高 QPS 下数据库会成为瓶颈，但功能上又是 "可有可无的统计"，应改为异步队列。

---

## 4. 回滚机制的设计问题

README 中：

> When a new update is published, it becomes the "active" update and the previous update… we copy the inactive update with a new timestamp and push it to the front of the queue.

具体看 `pages/api/rollback.ts`：仅仅是把旧 zip **拷贝一份**，盖一个新 timestamp 文件名再写入 storage 和 DB。这种 "复制为新发布" 的 "rollback" 有多重问题：

1. **不是协议意义的 rollback**：协议里的 `rollBackToEmbedded` directive 是把客户端回退到嵌入在 binary 中的初始 update。xavia 这种做法应称为 "republish previous"，与 README 中 "rollback" 一词以及 `rollBackToEmbedded` 路径混淆。
2. **没有创建包含 `rollback` 标志的 zip**，因此 `manifest.ts` 中那条 `rollBackToEmbedded` 分支永远不会被触发（参见 1.11，死代码）。
3. **完全没有鉴权**（参见 2.1）。
4. **审计性差**：DB 没有 `is_rollback_of: <old_id>` 之类的关系字段，回滚后无法追溯。
5. **新 release 拷贝后，`update_id` 没写**（参见 3.2），与新发布行为不一致。

---

## 5. 性能与可扩展性

### 5.1 把 release 存成单个 zip + 每次 manifest/asset 都重新拉/解压（严重性能问题）

- `ZipHelper.getZipFromStorage` 每个 release 一份全 zip 在进程内存里缓存 5 分钟（`Map`），**没有 LRU/总大小上限**，多 runtime version 会持续增长，是典型内存泄漏。
- 在多副本部署下进程内 cache 不共享，每个实例都要单独拉取整 zip → 冷启动延迟高。
- 单个资产请求要在内存里持有整个 zip（可能数十 MB）。
- 反观参考实现：把 update 解压到磁盘子目录（`updates/<rv>/<timestamp>/...`），按文件直读，简单且天然支持 Node.js stream 与 OS 文件系统缓存。
- 与 Next.js Pages API 的 4MB body 限制叠加（issue #27），需要手动 `responseLimit: false` 或改为流式响应；xavia 没做。

### 5.2 `assets.ts` 重复执行 manifest 的全部解析工作

每个资产请求都：

1. 从 storage 下载整 zip（命中或填充 cache）。
2. 解析 `metadata.json`。
3. 在 `fileMetadata[platform].assets` 中线性搜索匹配 `path`。

这是 O(N×N) 的请求总成本。应直接由 `asset` 路径 + bundle 唯一定位即可，无需再读 metadata。

### 5.3 manifest 每个 asset 都做一次 `getAssetMetadataAsync`（线性内 IO 风暴）

`pages/api/manifest.ts:170-181` 用 `Promise.all` 并发对每个资产单独调用 `getAssetMetadataAsync`，而该函数里又有 `getZipFromStorage` + `getFileFromZip`：

- zip cache 命中时还能凑合；首次冷启动一个 manifest 请求可能制造数十次同一 zip 的解析。
- 在多个 manifest 请求并发时，cache 还可能尚未填充，导致并发抢 IO。

### 5.4 排序 timestamp 用 `parseInt(name.split('.')[0], 10)`

`UpdateHelper.getLatestUpdateBundlePathForRuntimeVersionAsync:39`：

```ts
.sort((a, b) => parseInt(b.name.split('.')[0], 10) - parseInt(a.name.split('.')[0], 10));
```

时间戳格式 `YYYYMMDDHHmmss` 是 14 位整数，落在 `Number.MAX_SAFE_INTEGER` 内（16 位），但极易被混入非数字文件名时崩溃。直接字符串比较即可。

### 5.5 不支持反向代理子路径（issue #24）

`HOST` 直接拼接资产 URL，且 Next.js 的 `basePath` 是编译期常量。Docker 镜像运行时无法通过环境变量修改基础路径，无法部署到 `https://x/ota`。

---

## 6. 多租户 / 多应用 / 灰度（产品级缺失，issue #22、#26、#7）

- `updates/${runtimeVersion}` 是全局 namespace —— **多个 app 公用一台 xavia 时，相同 runtimeVersion 会撞车**。
- 没有 project / app / channel / release-track / branch 等概念。
- README 的 "Multiple apps" 问答没有给出方案；issue #22 关闭但实际功能未落地。
- 没有 gradual rollout / percent rollout / target audience，issue #7 已明确缺失。
- 没有 platform filtering 进入 metadata，无法对 iOS/Android 单独灰度。

---

## 7. 发布脚本 `build-and-publish-app-release.sh` 的问题

1. `jq '.expo' app.json` —— 现代 Expo 工程普遍使用 `app.config.ts` / `app.config.js`，脚本完全不兼容。
2. 写出的文件名是 `expoconfig.json`（小写 c），与 Expo 参考服务端使用的 `expoConfig.json`（大写 C）不一致。xavia 内部 `ConfigHelper` 读 `expoconfig.json`，对了，但 README 强调 "完整 expo-updates 协议兼容" 的同时却引入了与生态不同的命名约定。
3. 通过明文 `uploadKey` 走 multipart 上传，发布者本地 shell history 也会包含。
4. 强制 `git rev-parse HEAD` —— 不在 git 仓库时直接失败。
5. 没有 platform 维度，强制把 iOS、Android 资产同时打入同一个 zip 而 `expo export` 默认对二者打不同的 manifest，未配置 `--platform` 参数。
6. 上传失败时（curl 非 2xx）脚本继续删除产物再 "Done"，不返回非零退出码。

---

## 8. 文档 / Docker / 运维

- README "Storage Options" 段只列出 local 与 Supabase，FAQ 又额外列出 GCS / S3 —— 自相矛盾。
- README 在 FAQ "Can I use this with bare React Native apps?" 引用的是 **协议 v0** (`/archive/technical-specs/expo-updates-0/`)，而 xavia 代码里完整实现的是 **v1**（headers、directive 都是 v1 概念）。
- README 写明 "Is this production-ready? Yes!"，但同时存在严重 bug（如 issue #41、#17、#25、#27）以及第 2 节列举的鉴权完全缺失问题。"生产就绪" 的声明过于乐观。
- `docker-compose.yml` 默认 `DB_TYPE=supabase` 与 README 仅列 PostgreSQL 不一致。
- issue #30、#37 指出 Docker 镜像发布滞后于 release tag（v2.0.4 长时间未发布到 Docker Hub），CI/CD 流程不完整。
- 没有任何 CHANGELOG 化的迁移指南；DB schema 变化（如新增 `update_id` 列）需要使用者手动迁移。

---

## 9. 代码质量与可维护性

- 大量 `any`（如 `getMetadataAsync` 返回 `any`、`metadataJson.fileMetadata[platform].assets as any[]`）。
- 同时使用 `moment`（已被官方标注 legacy）与原生 `Date`。
- 多处 `console.error(error)` 与 `logger.error` 混用，日志策略不统一。
- 错误响应直接 `res.json({ error })` 把原始对象/`Error` 序列化抛出，信息泄露 + 序列化失败（`Error` 默认 JSON 是 `{}`）。
- `LocalStorage.getMimeType` 自维护了一份 MIME 表，重复 `mime` 库。
- `DictionaryHelper` 只有一个静态函数，没必要做成类。
- `manifest.ts` 一个文件 366 行包含三种响应分支、签名、表单组装，可拆分。
- 错误处理路径里把 `NoUpdateAvailableError` 用 throw 当作控制流，且嵌套 try/catch 检查 instanceof，可读性差。

---

## 10. 影响与风险分级汇总

| # | 问题 | 类别 | 严重程度 |
|---|---|---|---|
| 1.4 | 资产 URL 不含 update 标识，跨发布资产串流 | 协议正确性 | 严重 |
| 1.1 | `createdAt` 总为当前时间 | 协议正确性 | 严重 |
| 1.2 | `metadata` 一直为空，无 filters | 协议能力缺失 | 严重 |
| 1.3 | 缺失 `expo-manifest-filters` / `expo-server-defined-headers` | 协议违反 MUST | 高 |
| 1.5/1.6 | assets 无 cache-control、无压缩 | 性能 | 高 |
| 1.9 | 代码签名硬编码 keyid、不解析 SFV | 协议正确性 | 中-高 |
| 2.1 | 管理 API（rollback / releases / tracking）零鉴权 | 安全 | 严重 |
| 2.2 | 上传只有共享密钥，可 client-side RCE | 安全 | 严重 |
| 2.3 | runtimeVersion 路径遍历 | 安全 | 高 |
| 2.4 | 上传 zip 完整性零校验 | 安全 | 中-高 |
| 2.5 | 登录字符串比较、无 session | 安全 | 中 |
| 3.1 | LocalDatabase 漏选 update_id（issue #41） | 正确性/性能 | 严重 |
| 3.2 | rollback 不写 update_id | 正确性 | 高 |
| 3.3 | commit_message VARCHAR(255) | 正确性 | 低 |
| 3.5 | tracking 同步阻塞 + 经常静默丢失 | 可用性 | 中 |
| 4 | rollback 实现并非协议 rollback，且死代码 | 设计 | 高 |
| 5.1-5.3 | zip 全量解压 + 进程内无界 cache | 性能 / 内存 | 严重 |
| 5.5 | 不支持反向代理 basePath（issue #24） | 部署 | 中 |
| 6 | 无多租户 / channel / canary（issue #7/#22/#26） | 产品能力 | 高 |
| 7 | 发布脚本不兼容 app.config.ts、单一密钥 | 易用性 / 安全 | 中 |
| 8 | 文档/容器/CI 描述与实际不一致（issue #30/#37） | 运维 | 中 |
| 9 | 代码质量、错误处理 | 可维护性 | 低-中 |

---

## 11. 建议的改进方向（与上面问题对应）

1. **统一在上传时解压并固化 update**：
   - 服务端在 `upload.ts` 把 zip 解压成 `updates/<runtimeVersion>/<updateId>/...`，落地 `metadata.json` 真实创建时间、`updateId`、`createdAt`。
   - 资产 URL 包含 `updateId`：`/api/assets/<updateId>/<assetKey>`，可加 `cache-control: public, max-age=31536000, immutable`。
   - 同时去掉 `ZipHelper` 的进程内无界缓存。
2. **实现协议响应头与 metadata 机制**：`expo-manifest-filters`、`expo-server-defined-headers`、`metadata` 字段，作为后续 channel / canary 的基础。
3. **正确填充 `createdAt`**：使用上传时间或 manifest 中的真实时间。
4. **真正的 update id 比较短路**：修复 `LocalDatabase.getLatestReleaseRecordForRuntimeVersion` 漏选 `update_id`（issue #41）；rollback 复制也写入 `updateId`。
5. **鉴权重做**：
   - 引入 server-side session（如 HttpOnly cookie + signed JWT）。
   - 所有管理 API（`/api/rollback`、`/api/releases`、`/api/tracking/*`、`/api/upload`）强制鉴权中间件。
   - 上传支持 per-token / 短期 token / 范围（runtimeVersion / channel）。
6. **路径白名单化**：对 `expo-runtime-version`、查询参数严格正则校验，禁止 `..`、控制字符。
7. **代码签名按规范处理**：解析 `expo-expect-signature` SFV，按 `keyid` 选择对应私钥，按 `alg` 协商算法（至少检测 mismatch 时返回 4xx）。
8. **多项目 / channel 设计**：path 变为 `apps/<appId>/channels/<channel>/runtime/<rv>/<updateId>`。
9. **删除资产 manifest 中的 `test-header`** 等示例残留。
10. **资产支持 gzip/brotli 与 `accept` 协商，并配置 Next.js `responseLimit: false` 或改为 Edge / Streaming Route**。
11. **回滚机制**：要么走 `rollBackToEmbedded`（生成带 rollback 标记的 zip），要么明确改名为 "republish previous"；记录 `rolled_back_from` 关系。
12. **运维**：CI 同步 Docker tag、支持 `basePath`、文档与代码默认值对齐。

---

如需把上述任意一点展开为可执行的修复 PR/补丁，可在此基础上逐项推进。
