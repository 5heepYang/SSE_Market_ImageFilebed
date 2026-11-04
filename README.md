# 图床（Image Hosting）

一个自包含的图床服务：前端管理页 + 双通道鉴权 + SQLite 存储，登录使用**软工集市统一 OAuth**。

- **统一登录**：仅软工集市（SSE Market）用户可登录，走 OAuth 2.0 授权码模式，账号体系由集市统一提供。
- **前端管理页**：登录后只能看到**自己身份下**的图片与文档，支持拖拽/点击上传、缩略图网格、搜索、分页、可见性切换、复制直链 / Markdown / HTML；点击文档会在新标签页打开预览，PDF / DOCX / XLSX / PPTX 均可在线查看。
- **API 上传**：供脚本 / 第三方工具调用，支持 `multipart`、原始二进制、JSON(base64) 三种方式；除图片外还支持 pdf / doc / docx / xls / xlsx / ppt / pptx。
- **个人 Token**：可设置过期时间（1 小时 / 1 天 / 7 天 / 30 天 / 永不过期），原文仅创建时展示一次，服务端只存 SHA-256。
- **SQLite**：纯 Go 驱动（`glebarez/sqlite`），Windows 下免 CGO。

## 技术栈

| 层 | 选型 |
|----|------|
| 后端 | Go + Gin + GORM + SQLite（纯 Go）+ JWT |
| 前端 | Vue 3 + TypeScript + Vite + UnoCSS + axios |
| 身份 | SSE Market OAuth（软工集市统一身份认证） |

## 目录结构

```
imagebed/
├── server/            # Go 后端
│   ├── main.go        # 入口
│   ├── config.yaml    # 配置（含 oauth 段）
│   ├── config/ core/ model/ repository/ service/ controller/ middleware/ dto/ response/ util/
│   └── data/          # 运行时生成：images/ thumbs/ objects/ imagebed.db
├── web/               # Vue 前端
│   └── src/ views/ components/ api/ store/ router/ composables/ utils/
├── deploy/            # SSE Deploy 部署产物（Dockerfile / compose.yml / pack.ps1）
└── README.md
```

## 快速开始

### 0. 前置：在 SSE Market 注册 OAuth 应用

向平台管理员申请或自行注册，获得 `app_id` / `app_secret`，并登记 `redirect_uri`（回调地址，须与前端路由 `/oauth/callback` 完全一致，含子路径前缀）。

### 1. 配置并启动后端

```powershell
cd imagebed/server
# 通过环境变量注入 OAuth 凭证（app_secret 禁止提交到 Git；变量统一加 IMAGEBED_ 前缀）
$env:IMAGEBED_OAUTH_APP_ID = "your_app_id"
$env:IMAGEBED_OAUTH_APP_SECRET = "your_app_secret"
$env:IMAGEBED_OAUTH_REDIRECT_URI = "http://localhost:5173/oauth/callback"
go run .
```

首次启动自动创建 SQLite 数据库与三张表（`data/imagebed.db`），监听 `:8080`。

### 2. 启动前端（开发）

```powershell
cd imagebed/web
npm install
npm run dev        # http://localhost:5173
```

Vite 把 `/api`、`/i` 代理到后端 `:8080`。打开 `http://localhost:5173`，未登录会自动跳转到软工集市授权页，授权后回调完成登录。

### 3. 生产部署（可选）

```powershell
cd imagebed/web && npm run build     # 产物在 web/dist
cd ../server && go run .             # 后端检测到 web/dist 后直接托管前端
```

### 4. SSE Deploy 平台部署

**推荐（K8s / sseinfra）**：

```powershell
cd imagebed
powershell -ExecutionPolicy Bypass -File .\deploy\pack-sseinfra.ps1
# → imagebed-sseinfra.zip（含 deploy.sh，听 0.0.0.0:8080）
```

模块名用 `image`，在 SDPY **访问域名**绑定子域 `image`（用途=前端、设为主），配置：

```text
IMAGEBED_OAUTH_REDIRECT_URI=https://image.ssemarket.cn/oauth/callback
```

正式访问：**https://image.ssemarket.cn/**。详见 [`deploy/README-sseinfra.md`](./deploy/README-sseinfra.md)。

**可选（本机 Docker / compose）**：

```powershell
powershell -ExecutionPolicy Bypass -File .\deploy\pack.ps1
# → imagebed-deploy.zip（frontend/ + backend/ + compose.yml）
```

压缩包结构：


```
frontend/            # Vue 前端 + Dockerfile（node 构建 → nginx 托管）
backend/             # Go 后端 + Dockerfile（golang 构建 → alpine 运行）
compose.yml          # 服务名固定为 web / api
```

平台注入的环境变量（`compose.yml` 自动消费）：

| 变量 | 作用 |
|------|------|
| `SDPY_PUBLIC_URL` | 主访问 URL；子域名时为 `https://image.ssemarket.cn`，前端 `base=/` |
| `SDPY_PUBLIC_BASE` | 路径模式前缀；子域名模式下忽略 |
| `SDPY_API_PUBLIC_URL` / `PUBLIC_BASE` | 图片直链根；子域名优先用 `SDPY_PUBLIC_URL` |
| `IMAGEBED_OAUTH_APP_ID` / `IMAGEBED_OAUTH_APP_SECRET` | OAuth 应用凭证（**必填**） |
| `IMAGEBED_OAUTH_REDIRECT_URI` | **必填**，`https://image.ssemarket.cn/oauth/callback` |
| `IMAGEBED_OAUTH_BASE_URL` / `IMAGEBED_OAUTH_API_BASE_URL` | 授权页 / API 根地址（默认 `ssemarket.cn`） |

> 以上 OAuth 变量统一加 `IMAGEBED_` 前缀，避免与部署平台自身注入的 `OAUTH_*`（如 `ssemarket-sdpy` 的凭证）冲突。

后端还会读取 `PORT`、`DATA_DIR`、`DB_PATH`、`JWT_SECRET`、`SERVER_MODE` 覆盖 `config.yaml`。图片、缩略图、SQLite、JWT 密钥统一落在容器内 `/data`，并经 bind mount 挂载到宿主机 `/root/SSE_Market/sse-sdpy/data/imagebed/`，重建容器不丢数据。

### 依赖加速（国内镜像）

构建阶段已内置国内镜像，避免依赖下载缓慢/超时：

| 依赖 | 镜像地址 | 生效位置 |
|------|----------|----------|
| npm 依赖 | `registry.npmmirror.com` | `frontend/.npmrc`（`npm ci` 时自动读取） |
| Go module | `goproxy.cn`（`GOSUMDB=sum.golang.google.cn`） | `backend/Dockerfile` 的 `ENV GOPROXY` |

> Docker **基础镜像**（`node:20-alpine`、`golang:1.25-alpine`、`nginx:alpine`、`alpine:3.20`）从 Docker Hub 拉取，速度取决于**构建宿主机（sse-deploy 平台）的 Docker 加速配置**（`/etc/docker/daemon.json` 的 `registry-mirrors`），不在压缩包内控制。

## 配置（`server/config.yaml`）

| 项 | 默认 | 说明 |
|----|------|------|
| `server.port` | `8080` | 监听端口 |
| `server.mode` | `debug` | gin 模式（生产改 `release`） |
| `server.public_base_url` | `""` | 外网前缀，如 `https://img.example.com`；留空则 API 返回相对路径 |
| `server.frontend_dist` | `../web/dist` | 前端构建产物目录，存在时后端托管 |
| `jwt.secret` | — | 会话签名密钥（首次启动自动生成并持久化） |
| `jwt.expire_hours` | `168` | Web 登录态有效期（小时） |
| `upload.max_size_mb` | `20` | 单文件大小上限 |
| `upload.thumb_max_edge` | `400` | 缩略图最长边 |
| `resumable.enabled` | `true` | 是否启用断点续传/分片上传 |
| `resumable.max_size_mb` | `150` | 分片上传单文件总大小上限 |
| `resumable.max_chunk_mb` | `2` | 单个分片请求体上限（客户端建议 1MB） |
| `resumable.chunks_dir` | `""` | 临时分片目录；留空为 `data/chunks` |
| `oauth.base_url` | `https://ssemarket.cn/new` | 授权页根地址 |
| `oauth.api_base_url` | `""` | OAuth API 根地址（留空由 base_url 去掉 `/new` 推导） |
| `oauth.app_id` / `oauth.app_secret` | `""` | 应用凭证（环境变量 `IMAGEBED_OAUTH_APP_ID` / `IMAGEBED_OAUTH_APP_SECRET`） |
| `oauth.redirect_uri` | `""` | 授权回调地址（环境变量 `IMAGEBED_OAUTH_REDIRECT_URI`） |
| `oauth.authorize_path` | `/connect` | 授权页路径 |
| `oauth.scope` | `read_basic` | 授权 scope |

## API 文档

统一响应：`{ "code": 0, "message": "ok", "data": ... }`，`code != 0` 为失败。

### 认证（OAuth）

采用授权码模式，前端推荐走**两步回调**：

```bash
# ① 获取授权跳转地址（后端生成 state）
curl http://localhost:8080/api/auth/oauth/login-url
# → data: { "authorize_url": "https://ssemarket.cn/new/connect?appid=...&state=...", "state": "..." }
# 浏览器跳转 authorize_url，用户同意后回调到 redirect_uri?code=...&state=...

# ② 用 code 换 access_token（返回一次性 sessionId）
curl -X POST http://localhost:8080/api/auth/oauth/token \
  -H "Content-Type: application/json" \
  -d '{"code":"<code>","state":"<state>"}'
# → data: { "session_id": "..." }

# ③ 用 sessionId 换取本地登录态
curl -X POST http://localhost:8080/api/auth/oauth/userinfo \
  -H "Content-Type: application/json" \
  -d '{"session_id":"<session_id>"}'
# → data: { "token": "<jwt>", "user": {...}, "profile": {...} }
```

也可用一步接口 `POST /api/auth/oauth/callback`（body 同 ② 的 `{code, state}`，直接返回 `{token, user}`）。

当前用户：`GET /api/auth/me`（`Authorization: Bearer <jwt>`）。

### 管理接口（需 JWT：`Authorization: Bearer <jwt>`）

```bash
# 上传（multipart，可多文件）
curl -X POST http://localhost:8080/api/images \
  -H "Authorization: Bearer $JWT" \
  -F "file=@a.png" -F "file=@b.jpg"

# 图片列表（分页 / 搜索 / 可见性过滤）
curl "http://localhost:8080/api/images?page=1&page_size=24&keyword=cat&visibility=public" \
  -H "Authorization: Bearer $JWT"

# 切换可见性 / 删除
curl -X PATCH http://localhost:8080/api/images/1 \
  -H "Authorization: Bearer $JWT" -H "Content-Type: application/json" \
  -d '{"visibility":"private"}'
curl -X DELETE http://localhost:8080/api/images/1 -H "Authorization: Bearer $JWT"
```

### Token 管理（需 JWT）

```bash
# 创建 Token（expires_at 为 RFC3339，可省略/置 null 表示永不过期）
curl -X POST http://localhost:8080/api/tokens \
  -H "Authorization: Bearer $JWT" -H "Content-Type: application/json" \
  -d '{"name":"typora","expires_at":"2026-12-31T23:59:59+08:00"}'

curl http://localhost:8080/api/tokens -H "Authorization: Bearer $JWT"
curl -X DELETE http://localhost:8080/api/tokens/1 -H "Authorization: Bearer $JWT"
```

### API 上传（需个人 Token）

```bash
# 方式一：multipart
curl -X POST http://localhost:8080/api/upload \
  -H "Authorization: Bearer $TOKEN" -F "file=@a.png"

# 方式二：原始二进制
curl -X POST http://localhost:8080/api/upload \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: image/png" \
  --data-binary @a.png

# 方式三：JSON base64
curl -X POST http://localhost:8080/api/upload \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d "{\"image\":\"$(base64 -w0 a.png)\",\"filename\":\"a.png\"}"
```

返回 `{ url, thumb_url, markdown, html, size, width, height }`。

### 文件直链

- 公开图片：直接访问 `GET /i/<path>`。
- 公开文档：直接访问 `GET /o/<path>`。
- 私有文件：需携带本人 JWT 或 Token（`Authorization` 头或 `?token=` 查询参数）。

## 安全说明

- 身份统一由 SSE Market 提供，本地仅存 `oauth_id` 与展示资料，不存密码；`app_secret` 仅后端持有，不进前端 / 不进 Git。
- OAuth `code` 一次性、`state` 由后端 HMAC 签名防 CSRF、`access_token` 不下发浏览器（以一次性 `sessionId` 暂存 5 分钟）。
- 上传时图片按文件内容嗅探 MIME 并做白名单校验；文档按扩展名 + 文件头双重校验，伪装扩展名无法绕过；SVG 以 CSP + sandbox 头输出，降低 XSS 风险。
- Token 明文仅在创建时返回一次，数据库只保存 SHA-256；吊销后立即失效，过期时间实时校验。
- 列表 / 删除 / 更新均按 `user_id` 严格隔离，不同账号互不可见。
