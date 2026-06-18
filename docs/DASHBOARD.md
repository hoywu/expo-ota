# Dashboard 实现文档

> 状态：已定稿 (accepted)  
> 关联：[IMPLEMENTATION.md](./IMPLEMENTATION.md)（系统范围与 Admin API）、[server/api/admin/admin.api](../server/api/admin/admin.api)（类型与路由契约）、[dashboard/CONTEXT.md](../dashboard/CONTEXT.md)（前端技术约束）

---

## 0. 术语速查

Dashboard 侧复用 [server/CONTEXT.md](../server/CONTEXT.md) 的领域词汇。UI 文案需特别注意：

| 领域术语 | Dashboard 展示名 | 说明 |
| -------- | ---------------- | ---- |
| **RepublishPrevious** | Republish Previous | 复制历史 update 为新 pending 草稿；**不要**叫 Rollback（该词保留给协议 directive） |
| **Publish** | 发布 | 将 `pending` update 切为 `published`；与 Finalize 不同 |
| **Finalize** | — | 仅 CLI/API Token 上传链路（plan / finalize）；Dashboard MVP **不提供**浏览器内上传 |
| **AppSlug** | App Slug | 创建后只读，出现在 manifest URL 与路径中 |
| **ApiToken** | API Token | 形态 `ota_pat_…`，明文仅创建时展示一次 |

---

## 1. 范围与目标

### 1.1 In-Scope（MVP）

Dashboard 是面向内部管理员的 SPA，通过同源 Nginx 反代调用 `admin-api`（`/api/admin/*`），实现以下管理能力：

| 能力域 | Dashboard 职责 |
| ------ | -------------- |
| **鉴权** | 登录、token 刷新、登出、会话探活 |
| **App** | 列表、创建、编辑 name/description、软删 |
| **Update** | 列表与筛选、详情（manifest 预览 + 统计）、发布 pending、Republish Previous、单条删除、批量 cleanup |
| **API Token** | 按 App 创建/列表/撤销；创建后一次性展示明文 |
| **Code Signing** | 查看/生成/导入/启用禁用/删除 signing key；下载公钥 PEM |
| **User** | 管理员账号 CRUD（创建、改密、启用/禁用） |
| **Audit Log** | 按 App 查看管理写操作审计 |

可观测性：Update 详情页展示 6 项统计数字（manifest 请求设备数、成功/失败设备数、下载耗时 min/max/avg），数字使用 `@number-flow/vue` 动效。

### 1.2 Out-of-Scope（明确不做）

与 [IMPLEMENTATION.md §1.2](./IMPLEMENTATION.md#12-out-of-scope明确不做) 对齐，Dashboard 额外排除：

- **浏览器内上传**（plan / COS 直传 / finalize）：生产发布走 `cli/publish.ts` + API Token
- **Channel / Branch / 灰度路由** UI
- **多租户 / Organization / RBAC**：任意管理员等价，无角色 UI
- **Client Event 原始日志查询**：统计卡即可；原始行查 DB（见 IMPLEMENTATION §14.2）
- **Prometheus / Grafana 嵌入**
- **i18n**：MVP 仅英文 UI（或仅中文，二选一固定；实现时统一即可）

### 1.3 与 Server 的边界

```text
┌─────────────────────────────────────────────────────────┐
│  Browser (Dashboard SPA, /var/www/dashboard/)           │
│    Vue Router + Pinia + fetch(/api/admin/...)           │
└───────────────────────────┬─────────────────────────────┘
                            │ 同源 HTTPS, Authorization: Bearer
                            ▼
┌─────────────────────────────────────────────────────────┐
│  Nginx → admin-api :8081                                │
│    JWT（管理员）| 不接受 Dashboard 直接使用 API Token      │
└─────────────────────────────────────────────────────────┘

CLI/CI ──API Token──► plan / finalize（Dashboard 用 JWT Publish pending；只管理 Token，不代发上传）
Native App ──────────► protocol-api（manifest / events，不经 Dashboard）
```

---

## 2. 技术约束

以 [dashboard/CONTEXT.md](../dashboard/CONTEXT.md) 为准，摘要如下：

| 类别 | 选型 | 禁止 |
| ---- | ---- | ---- |
| 框架 | Vue 3 + Vite + Vue Router + Pinia | — |
| 包管理 | `bun` / `bunx` | npm, npx, yarn, pnpm |
| 组件库 | Nuxt UI v4（`@nuxt/ui/vite` + `@nuxt/ui/vue-plugin`） | Element Plus, Ant Design Vue, Naive UI |
| 样式 | Tailwind CSS v4 | SCSS 变量体系、CSS-in-JS |
| 数字动效 | `@number-flow/vue` | 手写补间、混用多库 |
| 测试 | Vitest + `@vue/test-utils` | — |

Nuxt UI 使用要点：

- 根组件外包 `UApp`（toast / tooltip / overlay 依赖）
- 使用 semantic colors（`text-default`, `bg-elevated`），避免 raw palette
- Dashboard 布局参考 Nuxt UI skill 的 `dashboard` layout 模式：`UDashboardGroup` + sidebar + panel

---

## 3. 信息架构与路由

### 3.1 路由表（9 个核心页面 + 辅助路由）

MVP 共 **9 个业务页面**（与 IMPLEMENTATION M9 一致），外加登录与 404：

| # | 路径 | 名称 | 鉴权 | 说明 |
| - | ---- | ---- | ---- | ---- |
| — | `/login` | Login | 否 | 未登录默认跳转 |
| 1 | `/apps` | App List | 是 | 登录后默认首页 |
| 2 | `/apps/new` | Create App | 是 | 创建 App 表单 |
| 3 | `/apps/:appSlug` | App Overview | 是 | 重定向到 `/apps/:appSlug/updates` |
| 4 | `/apps/:appSlug/updates` | Updates | 是 | App 内默认页；含 App 设置入口 |
| 5 | `/apps/:appSlug/updates/:updateId` | Update Detail | 是 | 详情 + 统计 + 操作 |
| 6 | `/apps/:appSlug/tokens` | API Tokens | 是 | Token 管理 |
| 7 | `/apps/:appSlug/signing-key` | Signing Key | 是 | 代码签名 key 管理 |
| 8 | `/admin/users` | Users | 是 | 全局用户管理 |
| 9 | `/apps/:appSlug/audit-logs` | Audit Logs | 是 | App 级审计 |
| — | `/*` | Not Found | — | `UError` 404 |

**App 内侧边栏导航**（`:appSlug` 上下文内固定）：

```text
Updates          → /apps/:appSlug/updates
API Tokens       → /apps/:appSlug/tokens
Signing Key      → /apps/:appSlug/signing-key
Audit Logs       → /apps/:appSlug/audit-logs
```

顶栏全局入口：`Apps`（返回列表）、`Users`（`/admin/users`）、当前用户菜单（改密、登出）。

### 3.2 路由守卫

```text
beforeEach:
  1. /login：已登录 → redirect /apps
  2. 需鉴权路由：无 accessToken → redirect /login?redirect=<fullPath>
  3. 进入 App 路由：prefetch GET /api/admin/apps/:appSlug；404 → toast + redirect /apps
  4. accessToken 将过期（<5min）且存在 refreshToken → 静默 POST /refresh
  5. GET /me 失败（401）→ 清 session → /login
```

### 3.3 URL 与部署

- `createWebHistory()`，Nginx 对非 `/api/*` 路径 `try_files → index.html`
- Manifest 端点 URL（只读展示，供复制）：`https://<host>/api/apps/{appSlug}/manifest`
- 静态资源 CI 构建后 rsync 至 `/var/www/dashboard/`（见 IMPLEMENTATION §12.4）

---

## 4. 鉴权与会话

### 4.1 Token 存储

| Key | 内容 | 存储位置 |
| --- | ---- | -------- |
| `ota_access_token` | JWT access token | `sessionStorage`（关 tab 即失效） |
| `ota_refresh_token` | JWT refresh token | `sessionStorage` |
| `ota_expires_at` | access 过期时间戳（ms） | `sessionStorage` |

不使用 `localStorage`，降低 XSS 持久化风险。MVP 可接受「关浏览器需重新登录」。

### 4.2 API 调用

所有鉴权请求：

```http
Authorization: Bearer <accessToken>
Content-Type: application/json
```

### 4.3 登录流程

```text
POST /api/admin/login { username, password }
  → 200: 存 tokens，redirect 至 redirect 参数或 /apps
  → 401: UAlert「Invalid username or password」
  → 429: UAlert「Too many attempts, try again later」
```

`username` 提交前 `trim` + `toLowerCase()`（与 server 一致）。

### 4.4 刷新与登出

- **Refresh**：`POST /api/admin/refresh { refreshToken }` → 轮换 access + refresh；失败则强制登出
- **Logout**：`POST /api/admin/logout`（no-op）+ 清 session + `/login`
- **探活**：App 壳 mounted 时 `GET /api/admin/me`；Pinia `auth` store 持有 `MeResp`

### 4.5 Token 生命周期（server 配置）

| Token | 有效期 |
| ----- | ------ |
| Access | 24h（`AccessExpire: 86400`） |
| Refresh | 7d（`RefreshExpire: 604800`） |

---

## 5. API 客户端层

### 5.1 模块结构（建议）

```text
dashboard/src/
├── api/
│   ├── client.ts       # fetch 封装、401 刷新、ErrorResp 解析
│   ├── auth.ts
│   ├── apps.ts
│   ├── updates.ts
│   ├── tokens.ts
│   ├── signing.ts
│   ├── users.ts
│   └── audit.ts
├── types/
│   └── admin.ts        # 与 admin.api 对齐的 TS 类型（手写或 codegen）
```

### 5.2 错误模型

Server 统一错误体（[httperr](../server/api/admin/internal/httperr/httperr.go)）：

```json
{ "code": "Bad Request", "message": "..." }
```

客户端映射：

| HTTP | Dashboard 行为 |
| ---- | -------------- |
| 400 | 表单字段旁 / toast 展示 `message` |
| 401 | 尝试 refresh；仍失败 → 登出 |
| 403 | toast「Forbidden」 |
| 404 | toast + 返回上级列表 |
| 409 | toast（如 slug 冲突、signing key 已存在） |
| 429 | toast + 禁用提交按钮 60s |
| 5xx | toast「Something went wrong」+ 可选重试 |

### 5.3 分页约定

| 端点 | Cursor 类型 | 默认 limit |
| ---- | ----------- | ---------- |
| `GET .../updates` | `updateId`（UUID） | 20 |
| `GET .../audit-logs` | `id`（bigint 字符串） | 50 |

「加载更多」按钮：`nextCursor` 非空则带 `?cursor=` 追加请求。

---

## 6. 全局布局

### 6.1 壳层结构

```text
UApp
└── RouterView
    ├── LoginLayout（/login，居中 UCard）
    └── DashboardLayout（鉴权路由）
        ├── UDashboardSidebar（全局 + App 上下文切换）
        ├── UDashboardNavbar（breadcrumb、用户菜单）
        └── UDashboardPanel（页面内容，max-w 合理约束）
```

### 6.2 共用组件

| 组件 | 职责 |
| ---- | ---- |
| `AppSwitcher` | 顶栏下拉，切换 `:appSlug` 时保留同级路由段 |
| `ConfirmModal` | 危险操作二次确认（删除 App、删除 update、cleanup、revoke token、disable user） |
| `CopyButton` | 复制 manifest URL、token 明文、公钥 PEM |
| `JsonPreview` | `manifestPreview` 语法高亮只读（`pre` + monospace，或可折叠 UAccordion） |
| `StatCard` | 包装 `@number-flow/vue` 的统计数字卡 |
| `EmptyState` | 列表无数据时的引导（如「Create your first app」） |
| `TimeAgo` | `createdAt` / `publishedAt` 相对时间 + tooltip 绝对时间 |

### 6.3 Pinia Stores

| Store | 状态 | Actions |
| ----- | ---- | ------- |
| `auth` | `user`, `isAuthenticated` | `login`, `logout`, `refresh`, `fetchMe` |
| `apps` | `items`, `currentApp` | `list`, `get`, `create`, `update`, `remove` |

Update / Token / Audit 列表以页面级 composable + `ref` 为主，不必全局 store。

---

## 7. 页面规格

### 7.1 Login（`/login`）

**目的**：管理员身份验证。

**UI**：

- `UAuthForm` 或 `UCard` + `UFormField`（username, password）
- Submit → loading 态；Enter 提交

**API**：`POST /api/admin/login`

**校验**：非空；password `type="password"`

**成功后**：写 session → 跳转 `route.query.redirect ?? '/apps'`

---

### 7.2 App List（`/apps`）

**目的**：浏览所有未删除 App，进入 App 上下文或创建新 App。

**UI**：

- 页头：标题「Apps」+ 主按钮「New App」→ `/apps/new`
- `UTable` 或 card grid 列：`appSlug`（mono）、`name`、`description`（truncate）、`createdAt`
- 行点击 → `/apps/:appSlug/updates`
- 行内菜单：Edit（打开 slideover）、Delete

**API**：

- `GET /api/admin/apps` → `{ items: AppResp[] }`

**Edit App（USlideover）**：

- 字段：`name`（必填）、`description`（可选）；**appSlug 只读展示**
- `PATCH /api/admin/apps/:appSlug`

**Delete App**：

- ConfirmModal 强调「软删，manifest 404，不可恢复 slug」
- `DELETE /api/admin/apps/:appSlug`

**空状态**：引导创建第一个 App

---

### 7.3 Create App（`/apps/new`）

**目的**：注册新 App 并分配全局唯一 `appSlug`。

**UI**：

- 表单：`appSlug`、`name`、`description`
- `appSlug` 实时校验：`^[a-z0-9][a-z0-9-]{1,30}[a-z0-9]$`（与 DB CHECK 及 server 一致）
- 提交成功 → toast + redirect `/apps/:appSlug/updates`

**API**：`POST /api/admin/apps`

**错误**：

- 400 invalid slug / empty name
- 409 slug taken

**页内提示**（静态 copy）：

- Slug 创建后不可修改
- 客户端 `updates.url` 将为 `https://<host>/api/apps/{appSlug}/manifest`

---

### 7.4 Updates List（`/apps/:appSlug/updates`）

**目的**：App 内 update 主工作台；筛选、跳转详情、批量 cleanup。

**UI 结构**：

1. **App 信息条**：name、slug（mono）、manifest URL（CopyButton）
2. **筛选栏**：
   - Platform：`All | iOS | Android` → `?platform=`
   - Runtime Version：文本或从列表 distinct 值下拉 → `?runtimeVersion=`
   - Status：`All | pending | published` → `?status=`
3. **工具栏**：
   - 「Cleanup old updates」→ ConfirmModal + 数字输入 `keepLatestN`（默认 **3**）
4. **表格**：

| 列 | 说明 |
| -- | ---- |
| Status | `UBadge`：pending=warning，published=success |
| Runtime Version | |
| Platform | ios / android 图标 |
| Message | truncate |
| Manifest UUID | mono truncate + copy |
| Created / Published | TimeAgo |
| Actions | View、Publish（仅 pending）、Republish Previous（仅 published） |

5. **分页**：cursor load more

**API**：

- `GET /api/admin/apps/:appSlug/updates?platform&runtimeVersion&status&limit&cursor`
- `POST /api/admin/apps/:appSlug/updates/:id/publish`（行内或详情）
- `POST /api/admin/apps/:appSlug/updates/:id/rollback`（UI 文案：**Republish Previous**）
- `POST /api/admin/apps/:appSlug/updates/cleanup` body `{ keepLatestN }`

**Republish Previous 流程**：

```text
ConfirmModal「Creates a new pending draft from this update」
  → POST rollback
  → toast + 跳转新 update 详情（pending，需再 Publish）
```

**Cleanup 结果展示**：

- 展示 `deletedUpdateIds.length`、`orphanAssetCount`（孤儿 asset 由后台 GC）

**业务提示**（info banner）：

- Update 由 CI/`cli/publish.ts` finalize；Dashboard 负责 **Publish** pending 草稿
- 删除已发布 update 需在该 `(runtimeVersion, platform)` 流内落后 ≥3 个版本

---

### 7.5 Update Detail（`/apps/:appSlug/updates/:updateId`）

**目的**：查看单次 update 全貌、可观测性统计、执行生命周期操作。

**布局**：两栏（md+）或纵向堆叠

#### 7.5.1 元数据区

| 字段 | 来源 |
| ---- | ---- |
| ID / Manifest UUID | 详情响应 |
| Status | `UBadge` |
| Runtime Version + Platform | |
| Message / Git commit | 可选 |
| Created / Published at | |
| Launch asset | key + 外链 `launchAssetUrl`（新 tab） |
| Rolled back from | 若 `rolled_back_from` 存在于 snapshot/metadata则展示链接（server 详情可后续扩展；MVP 可从 audit payload 或 list 上下文推断） |

#### 7.5.2 统计卡（6 个 StatCard + NumberFlow）

来自 `UpdateDetailResp.stats`：

| 指标 | 字段 |
| ---- | ---- |
| Requested devices | `requestedDevices` |
| Succeeded devices | `succeededDevices` |
| Failed devices | `failedDevices` |
| Min duration | `durationMinMs` ms |
| Max duration | `durationMaxMs` ms |
| Avg duration | `durationAvgMs` ms |

**当前 server 行为**：统计为**全时段聚合**（`GET update` 无 `from`/`to` 参数）。  
**Dashboard MVP**：直接展示上述字段。  
**后续增强**（需 server 扩展 query + IMPLEMENTATION §10.2）：时间范围选择器 1h / 24h / 7d / 30d / custom。

仅 `status=published` 时统计有意义；pending 显示 EmptyState「Not published yet」。

#### 7.5.3 Manifest 预览

- `JsonPreview` 绑定 `manifestPreview`（server 已 parse 的 JSON）
- 折叠默认 closed，避免大 JSON 卡顿

#### 7.5.4 Assets 表

`assets[]`：`key`、`sha256`（truncate）、`size`（human readable）、`url`（外链）

#### 7.5.5 操作按钮

| 操作 | 条件 | API |
| ---- | ---- | --- |
| Publish | `status=pending` | `POST .../publish` |
| Republish Previous | `status=published` | `POST .../rollback` |
| Delete | pending 任意；published 需 server 校验 rank>3 | `DELETE .../updates/:id` |

Delete 失败 400：`update must be at least 3 published versions behind...` → 友好 toast 解释规则。

**API**：`GET /api/admin/apps/:appSlug/updates/:updateId`

---

### 7.6 API Tokens（`/apps/:appSlug/tokens`）

**目的**：为 CI 创建/撤销 App 级上传凭据（plan / finalize）。

**UI**：

- 说明文案：Token 仅用于 `plan` / `finalize`；**Publish** pending update 须在 **Updates** 页用管理员 JWT 操作；明文只显示一次
- 「Create token」→ `UModal`：`name`（必填）、`expiresAt`（可选 datetime-local → RFC3339）
- 创建成功 **Modal 阻断关闭**，展示 `token` + CopyButton + 警告
- 表格：`name`、`scopes`（固定 `publish`，语义为允许上传端点）、`createdBy`、`lastUsedAt`、`expiresAt`、`createdAt`、status（revoked 灰显）
- Revoke → ConfirmModal → `DELETE .../api-tokens/:tokenId`

**API**：

- `GET /api/admin/apps/:appSlug/api-tokens`
- `POST /api/admin/apps/:appSlug/api-tokens`
- `DELETE /api/admin/apps/:appSlug/api-tokens/:tokenId`

**CI 集成提示**（静态）：

```bash
OTA_API=https://ota.example.com OTA_TOKEN=ota_pat_xxx OTA_APP_SLUG=my-app ...
bun run cli/publish.ts
# finalize 完成后，到 Updates 页 Publish pending 草稿
```

---

### 7.7 Signing Key（`/apps/:appSlug/signing-key`）

**目的**：管理每 App 单一 active RSA 签名 key；供客户端 `codeSigningCertificate` 配置。

**状态机**：

```text
无 key → 可 Generate 或 Import
有 enabled key → 可 Disable；不可 Generate/Import 新 enabled key
disabled → 可 Delete（硬删）
```

**UI 区块**：

1. **当前 Key 卡片**（数据来自 `GET .../signing-keys`：优先展示 enabled key，否则列表首条）
   - `keyId`、`algorithm`、`enabled`、`createdAt`、`disabledAt`
   - 公钥 PEM：`UTextarea` readonly + Download / Copy
   - `hasPrivateKey` false 时警告「Verify-only key, cannot sign manifests」
   - 卡片内 Enable / Disable / Delete：`PATCH/DELETE .../signing-keys/{keyId}`

2. **All keys 表格**（同 `GET .../signing-keys`）
   - 列出该 App 全部 signing key（enabled + disabled 历史）
   - 列：`keyId`、`enabled`、`createdAt`、`disabledAt`、`hasPrivateKey`
   - 每行操作（与卡片相同，均带 `keyId`）：
     - **Enable / Disable**：`PATCH .../signing-keys/{keyId} { enabled }`
     - **Delete**（仅 `disabledAt` 非空）：`DELETE .../signing-keys/{keyId}`
   - 空列表时显示 EmptyState

> `GET .../signing-key` 仍保留为只读兼容端点（返回最新一条）；Dashboard 不调用。

3. **Generate**（无 enabled key 时）
   - 输入 `keyId`（如 `main`）
   - `POST .../signing-key/generate`

4. **Import**（无 enabled key 时）
   - `keyId`、`publicKeyPem`、可选 `privateKeyPem`
   - `POST .../signing-key/import`

5. **Enable / Disable 确认**
   - Disable 前 ConfirmModal：「Clients with expo-expect-signature may fail until reconfigured」
   - Enable 另一历史 key 时，若已有 enabled key，server 返回 409

6. **Delete**（仅 disabled）
   - enabled 状态下按钮 disabled，并提示先 Disable
   - 无额外冷却时间；`disabledAt` 存在即可删

7. **Key ID 复用**
   - Generate/Import 支持复用历史 disabled key 的 `keyId`（server 先删旧行再写入）
   - 若同 `keyId` 当前仍为 enabled，server 返回 409

**客户端集成指引**（折叠 UAccordion，摘自 IMPLEMENTATION §8.4）：

- 从此页复制公钥 → 配置 `app.json` `updates.codeSigningCertificate` + `codeSigningMetadata`

**错误**：

- 409 enabled key already exists
- 400 key not disabled / invalid PEM

---

### 7.8 Users（`/admin/users`）

**目的**：管理等价管理员账号（无角色区分）。

**UI**：

- 「Create user」Modal：`username`、`password`
- 表格：`username`、`createdAt`、`lastLoginAt`、status（disabled badge）
- 行操作：Change password、Disable / Enable

**API**：

- `GET /api/admin/users`
- `POST /api/admin/users`
- `PATCH /api/admin/users/:userId/password`
- `POST /api/admin/users/:userId/disable`
- `POST /api/admin/users/:userId/enable`

**密码策略**（client 侧预校验，与 server [usersupport.go](../server/api/admin/internal/logic/admin/usersupport.go) 一致）：

- ≥10 字符
- 同时含字母与数字

**当前用户改密**：Navbar 用户菜单 → Change password Modal → `PATCH` 自己的 `userId`（来自 `/me`）

**Disable 自己**：禁止或 ConfirmModal 警告「You will be logged out」

---

### 7.9 Audit Logs（`/apps/:appSlug/audit-logs`）

**目的**：追溯 App 相关管理写操作。

**UI**：

- 筛选：`action`（select）、`actor`（userId 文本）、`from` / `to`（datetime → RFC3339）
- 表格：`occurredAt`、`action`、`actorUserId`、`targetType`、`targetId`、`ip`、payload 展开
- Load more cursor 分页

**API**：`GET /api/admin/apps/:appSlug/audit-logs?action&actor&from&to&limit&cursor`

**Action 枚举**（filter 下拉，与 server writeAudit 一致）：

| action | 含义 |
| ------ | ---- |
| `create_app` | 创建 App |
| `update_app` | 更新 App |
| `delete_app` | 软删 App |
| `finalize_update` | CLI finalize 草稿 |
| `publish_update` | 发布 update |
| `rollback_update` | Republish Previous |
| `delete_update` | 删除 update |
| `cleanup_updates` | 批量 cleanup |
| `create_api_token` | 创建 token |
| `revoke_api_token` | 撤销 token |
| `generate_signing_key` | 生成 key |
| `import_signing_key` | 导入 key |
| `patch_signing_key` | 启用/禁用 key |
| `delete_signing_key` | 删除 key |
| `create_user` | 创建用户 |
| `change_password` | 改密 |
| `disable_user` / `enable_user` | 禁用/启用用户 |
| `login_failed` | 登录失败（通常无 actor） |

`login_failed` 默认不在 App 筛选中展示（无 app 关联时 actor 可能为空）；若 server 未写入 app_id，列表仅含该 App 相关记录。

---

## 8. 交互与 UX 规范

### 8.1 加载与乐观更新

- 列表首次加载：表格 skeleton（`USkeleton`）
- 破坏性操作：**不做**乐观删除；等 API 成功后再刷新列表
- Publish / Republish：按钮 loading；成功后 refresh 行或跳转详情

### 8.2 Toast 策略

使用 Nuxt UI `useToast()`：

- 成功：短 toast（3s）
- 错误：长 toast（5s）+ `message` 原文（可英文）
- Token 创建成功：除 toast 外必须 Modal 展示明文

### 8.3 确认对话框 copy 模板

| 操作 | 标题建议 |
| ---- | -------- |
| Delete App | Delete app "{slug}"? |
| Delete pending update | Delete draft update? |
| Delete published update | Delete this update? It must be at least 3 versions behind. |
| Cleanup | Delete all but the latest {n} published updates per stream? |
| Revoke token | Revoke token "{name}"? CI using it will fail. |
| Disable signing key | Disable code signing for this app? |
| Delete signing key | Permanently delete disabled key? |
| Disable user | Disable user "{username}"? |

### 8.4 无障碍

- 表单字段 `label` + `aria-invalid`
- 图标按钮 `aria-label`
- 表格在窄屏可横向 scroll 或切换 card 列表

---

## 9. 测试

遵循 [AGENTS.md](../AGENTS.md)：`dashboard/` 下 `bun run test:unit`。

### 9.1 必测单元（Vitest）

| 模块 | 用例 |
| ---- | ---- |
| `validateAppSlug` | 合法/非法 slug |
| `validatePassword` | 边界长度、缺字母/数字 |
| `api/client` | 401 触发 refresh mock；ErrorResp 解析 |
| `auth` store | login/logout/session 清理由 |
| `StatCard` | 数字变化时渲染 NumberFlow |
| `ConfirmModal` | emit confirm/cancel |
| 路由 guard | 未登录 redirect（mock router） |

### 9.2 可选（后续）

- MSW 集成测试关键页面 data loading
- Playwright E2E（登录 → 创建 App → 列表可见）

---

## 10. 构建与部署

### 10.1 本地开发

```bash
cd dashboard
bun install
bun run dev          # Vite dev server
```

Dev 代理（`vite.config.ts` 建议添加）：

```ts
server: {
  proxy: {
    '/api/admin': { target: 'http://127.0.0.1:8081', changeOrigin: true },
  },
},
```

### 10.2 生产构建

```bash
bun run build        # type-check + vite build → dist/
bun run test:unit
bun run lint
```

### 10.3 CI 部署

```bash
rsync -avz --delete dist/ server:/opt/expo-ota/server/deploy/dashboard/
# Nginx 已挂载该目录，reload 即可
```

### 10.4 环境变量

| 变量 | 用途 |
| ---- | ---- |
| `VITE_API_BASE` | 可选；默认 `''` 表示同源 `/api/admin` |

---

## 11. 实施里程碑

与 [IMPLEMENTATION.md §13](./IMPLEMENTATION.md#13-开发路线图) M9 对齐，Dashboard 内部拆分：

| 步骤 | 交付 | 估时 |
| ---- | ---- | ---- |
| D1 脚手架 | Nuxt UI 布局壳、路由表、auth store、api client、login | 0.5d |
| D2 Apps | 列表 / 创建 / 编辑 / 删除 | 0.5d |
| D3 Updates | 列表筛选、cleanup、详情只读 | 1d |
| D4 Update 操作 | publish、republish、delete + 统计卡 NumberFlow | 0.5d |
| D5 Tokens + Signing | 两页完整流程 + copy/download | 1d |
| D6 Users + Audit | 用户管理、审计列表筛选 | 0.5d |
| D7 打磨 | 404、empty states、toast、移动端、vitest | 0.5d |
| D8 CI | build + rsync 文档/脚本 | 0.25d |

**合计**：~4 工作日。

### 11.1 建议目录结构（实现期）

```text
dashboard/src/
├── api/
├── components/
│   ├── layout/
│   ├── AppSwitcher.vue
│   ├── ConfirmModal.vue
│   ├── CopyButton.vue
│   ├── JsonPreview.vue
│   └── StatCard.vue
├── composables/
│   ├── useAuth.ts
│   └── useAppContext.ts
├── router/
│   ├── index.ts
│   └── guards.ts
├── stores/
│   └── auth.ts
├── views/
│   ├── LoginView.vue
│   ├── apps/
│   │   ├── AppListView.vue
│   │   ├── AppCreateView.vue
│   │   ├── UpdatesListView.vue
│   │   ├── UpdateDetailView.vue
│   │   ├── TokensView.vue
│   │   ├── SigningKeyView.vue
│   │   └── AuditLogsView.vue
│   └── admin/
│       └── UsersView.vue
└── utils/
    ├── format.ts
    └── validation.ts
```

---

## 12. 附录

### A. Admin API 速查

完整契约见 [admin.api](../server/api/admin/admin.api) 与 [IMPLEMENTATION.md §6](./IMPLEMENTATION.md#6-管理-api-列表)。

Dashboard **调用**的端点（不含 CLI 专用 plan/finalize）：

```text
POST   /api/admin/login
POST   /api/admin/refresh
POST   /api/admin/logout
GET    /api/admin/me
GET    /api/admin/apps
POST   /api/admin/apps
GET    /api/admin/apps/:appSlug
PATCH  /api/admin/apps/:appSlug
DELETE /api/admin/apps/:appSlug
GET    /api/admin/apps/:appSlug/updates
GET    /api/admin/apps/:appSlug/updates/:updateId
DELETE /api/admin/apps/:appSlug/updates/:updateId
POST   /api/admin/apps/:appSlug/updates/:updateId/publish
POST   /api/admin/apps/:appSlug/updates/:updateId/rollback
POST   /api/admin/apps/:appSlug/updates/cleanup
GET    /api/admin/apps/:appSlug/api-tokens
POST   /api/admin/apps/:appSlug/api-tokens
DELETE /api/admin/apps/:appSlug/api-tokens/:tokenId
GET    /api/admin/apps/:appSlug/signing-keys
GET    /api/admin/apps/:appSlug/signing-key          # 只读；Dashboard 用 signing-keys
POST   /api/admin/apps/:appSlug/signing-key/generate
POST   /api/admin/apps/:appSlug/signing-key/import
PATCH  /api/admin/apps/:appSlug/signing-keys/:keyId
DELETE /api/admin/apps/:appSlug/signing-keys/:keyId
GET    /api/admin/users
POST   /api/admin/users
PATCH  /api/admin/users/:userId/password
POST   /api/admin/users/:userId/disable
POST   /api/admin/users/:userId/enable
GET    /api/admin/apps/:appSlug/audit-logs
```

### B. 首次使用 checklist（运营）

1. 登录（`INITIAL_ADMIN_*`）→ **Users** 修改默认密码  
2. **Apps** → Create App  
3. **API Tokens** → 创建 token，配置 CI（仅 plan / finalize）  
4. （可选）**Signing Key** → Generate → 配置客户端 `app.json`  
5. CI `cli/publish.ts` finalize → **Updates** → Publish pending（管理员 JWT）  
6. 真机验证 manifest → **Update Detail** 查看统计  

### C. 已知差异与后续项

| 项 | 现状 | 后续 |
| -- | ---- | ---- |
| Update 统计时间范围 | 全时段 | server 增加 `?from=&to=` + Dashboard 选择器 |
| `rolled_back_from` 详情字段 | 未在 `UpdateDetailResp` 暴露 | server 扩展响应或从 audit 关联 |
| Dashboard 上传 | 不做 | 若需要人工发布，复用 plan/finalize API + 浏览器 COS PUT |
| i18n | 单语 | 按需引入 `@nuxt/ui` locale |

### D. 参考

- [IMPLEMENTATION.md](./IMPLEMENTATION.md)
- [dashboard/CONTEXT.md](../dashboard/CONTEXT.md)
- [CONTEXT-MAP.md](../CONTEXT-MAP.md)
- [Nuxt UI Dashboard layout](https://ui.nuxt.com/docs/getting-started/ai/mcp)（组件选型）
- [Expo Updates v1](https://docs.expo.dev/technical-specs/expo-updates-1/)
