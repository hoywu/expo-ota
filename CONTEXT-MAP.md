# Context Map

本仓库采用多 context 布局，每个 context 有独立的 `CONTEXT.md` 词汇表。

## Contexts

- [Server](./server/CONTEXT.md) — Go-Zero 后端 + 协议实现 + 数据模型
- Dashboard — 计划中（Vue 3 + Nuxt UI 客户端），尚未创建

## Relationships

- **Dashboard → Server**: Dashboard 通过 HTTP Bearer token 调 Server 的 REST API（admin + protocol）。同源（Nginx 同 server_name 反代，无 CORS）
- **Server → COS**: Server 通过腾讯云 SDK 读写对象存储（签名 URL 与 metadata）
- **CI → Server (filesystem)**: Dashboard 由 CI 构建为静态文件（Vite build → `dist/`），rsync/scp 到服务器 `/var/www/dashboard/`，Nginx 直接从文件系统提供，不经过 Go 服务

## ADRs

跨 context 的架构决策放在 [`docs/adr/`](./docs/adr/)，见 `docs/IMPLEMENTATION.md` 附录 B 索引。
