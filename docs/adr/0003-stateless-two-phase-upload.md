# ADR-0003: 两阶段发布 + 无状态 finalize

## 状态

accepted (2026-06-04)

## 背景

Expo Updates 协议要求服务端能接收来自 CLI / CI / Dashboard 的 update 上传。实现模式有两种：

1. **有状态 upload session**：服务端创建 `upload_sessions` 表，记 status / 计划上传哪些 asset / 已上传哪些 / 失败原因
2. **两阶段 + 无状态 finalize**：plan 一次性返回所有 missing asset 的 pre-signed URL，CLI 上传后调 finalize，finalize 重新跑去重 + 校验 + 写入

## 决策

采用方案 2：**两阶段 + 无状态 finalize**。

```
POST /api/admin/apps/{slug}/uploads/plan     # 一次性, 无状态
POST /api/admin/apps/{slug}/uploads/finalize # 一次性, 无状态, 内部重跑去重
```

## 理由

- **更少状态机**：`upload_sessions` 表及其状态流转（created → uploading → finalized / expired / failed）是典型易出错点（崩溃后残留、超时回收、并发 finalize）
- **plan 本身幂等**：同样的 metadata + asset list 调用两次，返回完全相同结果
- **finalize 本身幂等**：assets 表 `UNIQUE (app_id, sha256)` + `INSERT ON CONFLICT DO NOTHING` 保证重试安全
- **CLI 崩溃无需恢复**：下次重跑 plan 即可，pre-signed URL 过期（15 min）就重新签
- **更容易实现 multipart / 重试**：HTTP 边界清晰，CLI 端逻辑线性
- **更少代码**：少一张表、少一组状态机、少一组清理 cron

## 后果

- finalize 必传完整 manifest + asset list（不能只传 plan 响应 id）
- CLI 端需要保存 plan 响应至少到 finalize 完成（数据结构约 100KB 以内可接受）
- 重复调用 finalize 产生副作用：会产生重复 update 行（不同 `created_at`），需在 final 端去重
  - 解决：finalize 用 `INSERT ... WHERE NOT EXISTS (SELECT 1 FROM updates WHERE app_id = $1 AND manifest_uuid = $2)`，失败返回 409 Conflict + 现有 update id
- 无法用 plan 响应做"未来续传"标记；CLI 必须自己管理重试

## 未来触发重评的条件

- 单次 update asset 数量超过 10000，request body 过大需要拆批
- 需要"长时间 multipart upload"支持（数小时级别）
- 需要"上传未完成就查询进度"的 UI
