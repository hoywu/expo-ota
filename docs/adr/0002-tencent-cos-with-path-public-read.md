# ADR-0002: 腾讯云 COS 路径级公有读

## 状态

accepted (2026-06-04)

## 背景

Expo Updates 协议要求 asset URL 不可变。`manifest.assets[].url` 暴露给客户端后，任何持有该 URL 的中间缓存（CDN、客户端本地）在整个 update 生命周期内都可能再次拉取。如果 URL 后端内容会变，客户端会拉到错误资产导致整次更新失败。

实现这一约束有三种走法：

1. **整桶 public-read**：所有 COS 对象公开，URL 长期可用
2. **整桶 private + 短期 signed URL**：每次返回 15-30 min 过期签名
3. **整桶 private + 路径 `apps/*/assets/*` 公有读**：asset 路径公开，其他路径保持 private

## 决策

采用方案 3：**整桶 private，`apps/*/assets/*` 路径级公有读**。

## 理由

- **方案 1 风险太大**：CI 误传错路径也泄露；上传脚本可写其他路径被人读到
- **方案 2 不可行**：`expo-updates` 客户端可能在任意时间重拉 asset（断点续传、缓存失效），短期 signed URL 必然过期
- **方案 3 是正确解**：
  - asset 路径公开：客户端、CDN、浏览器都能长期缓存（`Cache-Control: public, max-age=31536000, immutable`）
  - 其他路径（admin 上传临时文件、未来可能的 logs/ 路径）保持 private
  - asset URL 形态稳定：`https://<cos-domain>/apps/{appSlug}/assets/{sha256_b64url}`
  - 内容寻址天然防"误改"：URL 包含 sha256，改了内容 URL 也变

## 后果

- 部署文档明确要求 COS 桶配置路径级 ACL
- 资产上传（CLI 端 / Bun 脚本）走 pre-signed PUT URL，**无需** CORS（COS SDK 走 HTTPS PUT）
- Manifest asset URL 直接硬编码到 manifest JSON，无需服务端在请求时生成签名
- 上传完成后删除源文件权限立即可被 admin 撤销（修改 ACL 即可，不影响已发布 URL）
- 失去"上传后立即隐藏"的能力（asset 路径公开），但内容寻址保证不会泄露错内容

## 未来触发重评的条件

- COS 桶需要支持多租户隔离
- 出现"asset 必须鉴权才能下载"的安全需求
- 切换到其他对象存储（S3 / OSS / R2）
