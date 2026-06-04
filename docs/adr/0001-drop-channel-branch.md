# ADR-0001: 砍掉 channel / branch 概念

## 状态

accepted (2026-06-04)

## 背景

EAS Update 与所有主流第三方实现（expo-open-ota、xavia-ota）都采用 `channel` + `branch` 两层概念：

- `channel`：构建时写入 binary 的"通道"（如 `production` / `staging`）
- `branch`：服务端一条 update 序列
- `channel → branch` 路由：支持百分比灰度、A/B 分桶、稳定分桶

本项目最初的设计文档（`IMPLEMENTATION_GUIDE_CLAUDE.md` / `IMPLEMENTATION_GUIDE_GPT.md`）照搬了完整 EAS 模型。

## 决策

本项目**不实现 channel / branch 概念**。Routing 维度简化为 `(appSlug, runtimeVersion, platform)`。

## 理由

1. **业务规模不匹配**：EAS 的 channel 模型是为"几百个 app + 多团队 + 灰度发布"设计的。
2. **runtimeVersion 已是天然路由**：客户端 binary 决定 `expo-runtime-version` 头，"老 binary 还在跑"就是 runtimeVersion 区分的天然场景。
3. **灰度需求不成立**：小规模下，测试组用单独的 `runtimeVersion=1.0.0-rc1` binary 即可。
4. **降复杂度**：
   - 砍掉 `channels` / `channel_branch_mappings` / `branch_directives` 三张表
   - 砍掉 channel routing 算法
   - 砍掉稳定分桶、百分比权重、A/B 测试
   - 砍掉 `expo-server-defined-headers` 在 MVP 的实现
   - 砍掉 `rollBackToEmbedded` 在协议层的实现（因为没有"branch 切到 embedded"概念）

## 后果

- 客户端 manifest 请求 URL：`GET /api/apps/{appSlug}/manifest`
- 服务端查 latest update：`(app, runtimeVersion, platform)` 三元组唯一确定
- "Rollback" 改为"复制旧 update 为新行"（见 §9 IMPLEMENTATION.md）
- "回退到 embedded" 协议 directive 不再实现；如未来需要，单独加一张 `app_directives` 表
- 客户端无需配置 `expo-channel-name` 即可工作（`Updates.channel` 字段忽略）

## 未来触发重评的条件

- App 数量超过 5 个
- 出现"同 binary 多个 update 流"诉求（员工版 / 外部版同 runtimeVersion）
- 出现真正的"灰度"诉求（同一 App 不同用户群不同 update）
