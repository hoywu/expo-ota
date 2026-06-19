# 生产部署指南

本文档描述如何部署 Expo OTA：**后端**通过项目根目录的 Docker Compose 启动；**前端 Dashboard** 由运维方自行构建静态资源，并用 Nginx（或其他 Web Server）托管。

## 架构概览

```text
                         ┌──────────────────────────────────────┐
                         │  Nginx / Caddy / 其他 Web Server     │
                         │  :80 / :443                          │
                         │  静态 SPA + 反向代理 API             │
                         └──────────┬───────────────┬───────────┘
                                    │               │
                      /api/apps/*   │               │  /api/admin/*
                                    ▼              ▼
                           ┌──────────────┐  ┌──────────────┐
                           │ protocol-api │  │  admin-api   │
                           │ 127.0.0.1    │  │  127.0.0.1   │
                           │    :8080     │  │    :8081     │
                           └──────┬───────┘  └──────┬───────┘
                                  │                 │
                                  └────────┬────────┘
                                           ▼
                                  ┌──────────────┐
                                  │ PostgreSQL   │
                                  │  (Docker)    │
                                  └──────────────┘

外部依赖：腾讯云 COS（资产存储，由 admin-api 直连）
```

与本地开发的区别：

| 场景        | 数据库                  | 配置                                              |
| ----------- | ----------------------- | ------------------------------------------------- |
| 本地开发    | `make infra-up` 仅起 PG | 项目根目录 `.env`，`DB_URL` 指向 `localhost:5432` |
| 生产 Docker | compose 内置 `db` 服务  | 项目根目录 `.env`，`DB_URL` 主机名改为 `db`       |

`docker-compose.yml` 将 `protocol-api`、`admin-api` 绑定到 `127.0.0.1`，仅本机 Nginx 可反代，数据库不暴露公网。

## 前置条件

- Docker Engine 24+ 与 Docker Compose v2
- 一台可访问公网的服务器（建议 2 vCPU / 4 GB RAM 起）
- 已创建腾讯云 COS 桶，并配置 `apps/*/assets/*` 路径公有读（详见 [docs/IMPLEMENTATION.md](./docs/IMPLEMENTATION.md) §12）
- 域名 DNS 已解析到服务器（若启用 HTTPS）
- 构建前端需 Node.js `^20.19.0` 或 `>=22.12.0`，推荐使用 [Bun](https://bun.sh/)

## 一、部署后端（Docker）

### 1. 准备环境变量

```bash
cp .env.example .env
```

编辑 `.env`，至少修改以下项：

```bash
POSTGRES_PASSWORD=...          # 强密码
DB_URL=postgresql://${POSTGRES_USER}:${POSTGRES_PASSWORD}@db:5432/${POSTGRES_DB}
AUTH_ACCESS_SECRET=...         # openssl rand -hex 32
AUTH_REFRESH_SECRET=...        # openssl rand -hex 32
SIGNING_KEY_ENCRYPTION_KEY=... # openssl rand -base64 32
COS_SECRET_ID=...
COS_SECRET_KEY=...
COS_REGION=...
COS_BUCKET=...
INITIAL_ADMIN_PASSWORD=...     # 首次登录后立即在 Dashboard 改密
```

**注意**：Docker 部署时 `DB_URL` 主机名必须为 `db`（compose 内部服务名），不能使用 `localhost`。

### 2. 构建并启动

```bash
docker compose up -d --build
```

首次启动会自动：

1. 拉起 PostgreSQL 并等待健康检查
2. 运行 `migrate` 一次性任务（goose 迁移）
3. 启动 `protocol-api`（`:8080`）、`admin-api`（`:8081`）

### 2.1 本地构建镜像并推送至服务器（无需拷贝源码）

若希望在本地或 CI 完成编译，服务器只拉取镜像运行，无需拷贝整个 Git 仓库或安装 Go 工具链。后端所有服务（`migrate`、`protocol-api`、`admin-api`、`asset-gc`）共用同一镜像 `expo-ota-api`。

**本地构建并打标签：**

```bash
# 在项目根目录
docker build -t expo-ota-api:1.0.0 -f Dockerfile --target api .
```

**方式 A：推送到镜像仓库（推荐）**

```bash
# 以 GHCR 为例，换成你的 registry 地址与 tag
REGISTRY=ghcr.io/<org>/expo-ota-api
TAG=1.0.0

docker tag expo-ota-api:1.0.0 "$REGISTRY:$TAG"
docker push "$REGISTRY:$TAG"
```

服务器只需保留运行所需文件（无需源码）：

```text
/opt/expo-ota/
├── docker-compose.yml
└── .env
```

在服务器上拉取镜像、打上 compose 使用的本地名，然后启动（跳过构建）：

```bash
cd /opt/expo-ota
docker pull "$REGISTRY:$TAG"
docker tag "$REGISTRY:$TAG" expo-ota-api
docker compose up -d --no-build
```

升级版本时重复 `pull` → `tag` → `docker compose up -d --no-build`；若数据库 schema 有变更，再执行 `docker compose run --rm migrate`。

**方式 B：无镜像仓库，直接传输镜像包**

适合内网或单机部署，用 `docker save` / `docker load` 代替 registry：

```bash
# 本地
docker save expo-ota-api:1.0.0 | gzip > expo-ota-api-1.0.0.tar.gz
scp expo-ota-api-1.0.0.tar.gz docker-compose.yml server:/opt/expo-ota/
```

```bash
# 服务器（.env 需事先放好）
cd /opt/expo-ota
gunzip -c expo-ota-api-1.0.0.tar.gz | docker load
docker compose up -d --no-build
```

**前端**：同样可在本地构建，仅同步静态产物，无需拷贝 `dashboard/` 源码：

```bash
make build-dashboard
rsync -a --delete dashboard/dist/ server:/var/www/expo-ota/
```

### 3. 验证后端

```bash
curl -fsS http://127.0.0.1:8080/healthz
curl -fsS http://127.0.0.1:8080/readyz
curl -fsS http://127.0.0.1:8081/api/admin/healthz
```

## 二、构建并托管前端

Dashboard 是 Vue SPA，默认通过**同源**路径 `/api/admin` 访问管理 API（见 `dashboard/src/api/client.ts`）。生产环境应由 Web Server 将 `/api/admin/*` 反代到 `admin-api`，将 `/api/apps/*` 反代到 `protocol-api`，**不建议**将前端与 API 部署在不同域名（服务端未配置 CORS）。

### 1. 构建静态资源

在项目根目录执行：

```bash
make build-dashboard
# 或
cd dashboard && bun install && bun run build
```

产物位于 `dashboard/dist/`。

### 2. 部署到 Web Server

将 `dashboard/dist/` 内容复制到 Web Server 的站点根目录，例如：

```bash
sudo mkdir -p /var/www/expo-ota
sudo rsync -a --delete dashboard/dist/ /var/www/expo-ota/
```

### 3. Nginx 配置示例

以下配置将 API 反代到本机 Docker 后端（`127.0.0.1:8080/8081`），并包含 [docs/IMPLEMENTATION.md](./docs/IMPLEMENTATION.md) §11.3 规定的限速规则：

| 端点                            | 限速       | Key                                |
| ------------------------------- | ---------- | ---------------------------------- |
| `POST /api/admin/login`         | 5 req/min  | IP                                 |
| `GET /api/apps/{slug}/manifest` | 30 req/min | `expo-device-id` header（兜底 IP） |
| `POST /api/apps/{slug}/events`  | 60 req/min | `expo-device-id` header（兜底 IP） |
| 其他 `/api/admin/*`             | 60 req/min | IP                                 |

```nginx
# /etc/nginx/conf.d/expo-ota.conf

upstream protocol_api {
    server 127.0.0.1:8080;
    keepalive 16;
}

upstream admin_api {
    server 127.0.0.1:8081;
    keepalive 16;
}

# 限速 zone 定义（放在 http 块内，与 server 块同级）
limit_req_zone $binary_remote_addr zone=login_limit:10m rate=5r/m;
limit_req_zone $device_or_ip zone=manifest_limit:10m rate=30r/m;
limit_req_zone $device_or_ip zone=events_limit:10m rate=60r/m;
limit_req_zone $binary_remote_addr zone=admin_limit:10m rate=60r/m;

# manifest / events 优先按 expo-device-id 限速，无 header 时回退到 IP
map $http_expo_device_id $device_or_ip {
    default $binary_remote_addr;
    "~."    $http_expo_device_id;
}

server {
    listen 80;
    server_name ota.example.com;

    root /var/www/expo-ota;
    index index.html;

    client_max_body_size 20m;

    add_header X-Content-Type-Options nosniff always;
    add_header X-Frame-Options DENY always;
    add_header Referrer-Policy no-referrer always;

    location = /healthz {
        proxy_pass http://protocol_api/healthz;
        proxy_http_version 1.1;
        proxy_set_header Connection "";
    }

    location = /readyz {
        proxy_pass http://protocol_api/readyz;
        proxy_http_version 1.1;
        proxy_set_header Connection "";
    }

    location ~ ^/api/apps/[^/]+/manifest$ {
        limit_req zone=manifest_limit burst=10 nodelay;
        proxy_pass http://protocol_api;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Connection "";
    }

    location ~ ^/api/apps/[^/]+/events$ {
        limit_req zone=events_limit burst=20 nodelay;
        proxy_pass http://protocol_api;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Connection "";
    }

    location = /api/admin/login {
        limit_req zone=login_limit burst=3 nodelay;
        proxy_pass http://admin_api;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Connection "";
        proxy_read_timeout 300s;
    }

    location /api/admin/ {
        limit_req zone=admin_limit burst=30 nodelay;
        proxy_pass http://admin_api;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Connection "";
        proxy_read_timeout 300s;
    }

    location / {
        try_files $uri $uri/ /index.html;
    }
}
```

若使用 `conf.d/` 片段，`limit_req_zone` 与 `map` 需放在主配置的 `http` 块内（不可放在 `server` 块中）。也可将上述内容合并为单一 `nginx.conf`。

启用配置并重载：

```bash
sudo nginx -t && sudo systemctl reload nginx
```

浏览器访问 `https://ota.example.com/login`，使用 `INITIAL_ADMIN_USERNAME` / `INITIAL_ADMIN_PASSWORD` 登录。

### 4. HTTPS

若由本机 Nginx 终结 TLS，在 HTTP `server` 块前增加 80 → 443 跳转，并将上述 `server` 块改为监听 443：

```nginx
server {
    listen 80;
    server_name ota.example.com;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl;
    http2 on;
    server_name ota.example.com;

    ssl_certificate     /etc/nginx/certs/fullchain.pem;
    ssl_certificate_key /etc/nginx/certs/privkey.pem;
    ssl_session_timeout 1d;
    ssl_session_cache shared:SSL:10m;
    ssl_protocols TLSv1.2 TLSv1.3;

    # 其余 location 块与 HTTP 示例相同，并额外添加：
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    # ...
}
```

也可在前置负载均衡（如云 LB）终结 TLS，本机 Nginx 仅监听 HTTP。

### 5. 其他 Web Server

只要满足以下路由即可：

| 路径                  | 反代目标                                |
| --------------------- | --------------------------------------- |
| `/api/apps/*`         | `http://127.0.0.1:8080`                 |
| `/api/admin/*`        | `http://127.0.0.1:8081`                 |
| `/healthz`、`/readyz` | `http://127.0.0.1:8080`                 |
| 其余路径              | 静态文件 + SPA fallback（`index.html`） |

Caddy、Traefik 等均可，需自行配置等价反代与（可选）限速。

## 常用运维命令

```bash
# 查看状态
docker compose ps

# 查看日志
docker compose logs -f protocol-api admin-api

# 重新构建并滚动更新 API
docker compose up -d --build protocol-api admin-api

# 手动跑数据库迁移（升级版本后）
docker compose run --rm migrate

# 孤儿资产 GC（建议 cron 每日执行）
docker compose --profile maintenance run --rm asset-gc

# 停止
docker compose down

# 停止并删除数据卷（危险，会清空数据库）
docker compose down -v
```

前端有变更时，重新 `make build-dashboard` 并同步 `dashboard/dist/` 到站点目录即可，无需重启 Docker 后端（除非 API 有变更）。

### 建议的 asset-gc cron

```cron
0 3 * * * cd /opt/expo-ota && docker compose --profile maintenance run --rm asset-gc >> /var/log/expo-ota-asset-gc.log 2>&1
```

## 发布首个 OTA Update

1. Dashboard → 创建 App，记录 `appSlug`
2. Dashboard → API Tokens → 创建 token，保存明文（只显示一次）
3. 在 CI 配置 `OTA_TOKEN` 与 `OTA_SERVER_URL`（指向你的域名）
4. 运行 `cli/publish.ts` 上传 bundle（生成 `pending` 草稿）
5. Dashboard → Updates → 点击 **Publish** 正式发布

客户端 `app.json` 中 `updates.url` 应指向：

```text
https://<your-domain>/api/apps/<appSlug>/manifest
```

## 环境变量说明

| 变量                         | 必填 | 说明                                          |
| ---------------------------- | ---- | --------------------------------------------- |
| `POSTGRES_USER`              | 否   | 默认 `admin`                                  |
| `POSTGRES_PASSWORD`          | 是   | 数据库密码                                    |
| `POSTGRES_DB`                | 否   | 默认 `expo_ota`                               |
| `DB_URL`                     | 是   | PostgreSQL 连接串；Docker 部署使用主机名 `db` |
| `AUTH_ACCESS_SECRET`         | 是   | JWT access token 密钥                         |
| `AUTH_REFRESH_SECRET`        | 是   | JWT refresh token 密钥                        |
| `SIGNING_KEY_ENCRYPTION_KEY` | 是   | 代码签名私钥 AES 加密密钥                     |
| `COS_*`                      | 是   | 腾讯云 COS 凭据与桶配置                       |
| `COS_DOMAIN`                 | 否   | 自定义 COS 域名                               |
| `COS_KEY_PREFIX`             | 否   | 对象 key 前缀                                 |
| `INITIAL_ADMIN_USERNAME`     | 否   | 首次启动创建的管理员用户名                    |
| `INITIAL_ADMIN_PASSWORD`     | 是   | 首次启动创建的管理员密码                      |

完整协议与 COS 配置说明见 [docs/IMPLEMENTATION.md](./docs/IMPLEMENTATION.md) §12。

## 安全 checklist

- [ ] 修改所有默认密码与随机密钥
- [ ] 限制服务器安全组，仅开放 80/443（及 SSH）
- [ ] PostgreSQL 与 API 不映射到公网（compose 默认绑定 `127.0.0.1`）
- [ ] 首次登录后立即修改管理员密码
- [ ] 将 `.env` 纳入密钥管理，避免长期明文存放在磁盘
- [ ] 定期备份 `expo_ota_pg_data` 卷
- [ ] Nginx 配置限速与安全响应头（见上文 §3 示例）

## 故障排查

| 现象                 | 可能原因                          | 处理                                                     |
| -------------------- | --------------------------------- | -------------------------------------------------------- |
| `migrate` 失败       | `DB_URL` 错误或数据库未就绪       | 检查 `.env` 中主机名为 `db`，`docker compose logs db`    |
| `/readyz` 非 200     | 数据库连接失败                    | 确认 `DB_URL` 主机名为 `db`                              |
| Dashboard 登录 502   | Nginx 未反代或 API 未启动         | `curl 127.0.0.1:8081/api/admin/healthz`，检查 Nginx 配置 |
| Dashboard 空白 / 404 | 静态文件未部署或 `try_files` 缺失 | 确认 `root` 指向 `dashboard/dist` 内容                   |
| finalize 超时        | COS 凭据错误或资产未上传          | 检查 COS 配置与桶权限                                    |
| 客户端拉不到更新     | manifest 路径或反代错误           | 确认 `/api/apps/<slug>/manifest` 指向 `protocol-api`     |

## 文件索引

| 文件                       | 用途                                                                         |
| -------------------------- | ---------------------------------------------------------------------------- |
| `Dockerfile`               | 多阶段构建 Go API 服务（`protocol-api`、`admin-api`、`migrate`、`asset-gc`） |
| `docker-compose.yml`       | 生产编排：`db` / `migrate` / `protocol-api` / `admin-api` / `asset-gc`       |
| `docker-compose.infra.yml` | **仅开发用**，只启动 PostgreSQL                                              |
| `.env.example`             | 环境变量模板                                                                 |
| `dashboard/dist/`          | `make build-dashboard` 产出，由 Web Server 托管                              |
