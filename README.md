# 图床（Image Hosting）

一个为软工集市提供图片 / 文件中转服务的平台：图片与文档上传、按需缩放、tus 断点续传、UniSSO 单点登录、MinIO 与本地磁盘双存储，附带一套 Vue 3 管理界面。

## 能力清单

- **上传**：multipart、原始二进制、JSON(base64)、**tus 断点续传**（2MB 分片，单文件上限 150MB）
- **图片处理**：自动缩略图；直链支持按需缩放 `?w=&h=&q=&fmt=`（不放大、体积兜底、结果落存储缓存）
- **登录**：UniSSO 邮箱 + 密码；**静默 SSO**（读取 `ssemarket.cn` 父域共享 Cookie 免登录）；单点登出
- **鉴权三层**：用户 JWT（Web）／ API Token（第三方模块）／ **短期上传凭证**（浏览器直连 tus，避免暴露长效 token）
- **并发控制**：按账号限制在传会话数；新建会话走准入令牌桶；全平台在传总量兜底
- **存储**：MinIO（对象存储）优先，未配置时回落本地目录
- **管理界面**：图片列表 / 搜索 / 可见性切换 / 删除 / 上传 / Token 管理 / 预览

## 技术栈

| 层 | 组成 |
|---|---|
| 后端 | Go + Gin + GORM(SQLite, pure-go) + tusd v2 + minio-go v7 + imaging |
| 前端 | Vue 3 + Vite + UnoCSS + tus-js-client + axios |

## 架构

```
浏览器 / 第三方模块
  ├─ Web 管理界面（Vue）──登录(JWT)──┐
  ├─ tus 直连上传（短期凭证）─────────┤
  └─ API 调用（API Token）───────────┤
                                      ▼
                              Go 服务（gin）
                     ├─ SQLite：images / users / tokens / upload_sessions
                     ├─ 对象存储：MinIO bucket  或  本地 DATA_DIR
                     └─ 静态托管：web/dist（存在时）
```

## 目录结构

```
.
├── deploy.sh              # 平台部署入口（构建前端 + 编译/启动后端）
├── config.yaml            # 运行配置（密钥请用环境变量或 .env.local 覆盖）
├── docs/                  # 补充文档
├── server/                # Go 后端
│   ├── blob/              # 存储抽象（MinIO / 本地）
│   ├── config/            # 配置加载与环境变量覆盖
│   ├── controller/        # HTTP 处理器
│   ├── core/              # 路由、数据库、静态文件与缩放、tus 接入
│   ├── dto/ model/ repository/ service/ middleware/ response/ util/
│   └── main.go
├── tools/                 # 运维小工具（数据库内存化 / 同步）
│   ├── db_bootstrap.py    # 启动时从 MinIO 拉回 DB 到 /dev/shm
│   ├── db_persist.py      # 定时把 DB 快照上传 MinIO（保留最近 N 份）
│   ├── minio_sigv4.py     # 极简 S3 客户端（SigV4）
│   └── swap-db.sh         # 换库脚本（mv 语义，失败自恢复）
└── web/                   # Vue 前端
```

## 快速开始（本地）

### 后端

```bash
cd server
# UniSSO 凭证（不配置也能启动，只是登录接口不可用）
export UNISSO_ISSUER=https://sso.ssemarket.cn
export UNISSO_CLIENT_ID=your_client_id
export UNISSO_CLIENT_SECRET=your_client_secret
go run .
```

首次启动会自动创建 SQLite 数据库及 4 张表（`images` / `users` / `tokens` / `upload_sessions`），默认监听 `:8080`。

> 未配置 UniSSO 时无法通过 Web 登录；如需调接口，可使用已创建好的 API Token（`imgb_` 前缀）直接访问 `/api/upload`。

### 前端

```bash
cd web
npm install
npm run dev        # http://localhost:5173
```

Vite 已将 `/api`、`/i`、`/o` 代理到后端 `:8080`。生产时执行 `npm run build`，后端检测到 `web/dist` 后会直接托管。

## 配置（`config.yaml`）

| 键 | 默认 | 说明 |
|---|---|---|
| `server.port` | `8080` | 监听端口 |
| `server.mode` | `debug` | gin 模式（生产用 `release`） |
| `server.public_base_url` | `""` | 对外访问前缀，如 `https://image.ssemarket.cn` |
| `server.frontend_dist` | `../web/dist` | 前端产物目录，存在则托管 |
| `server.allowed_origins` | — | CORS 白名单（含 tus 自定义头与 `X-Object-URL` 暴露） |
| `database.path` | — | SQLite 路径；建议用 `DB_PATH` 环境变量覆盖 |
| `jwt.secret` | — | 会话签名密钥；**务必用 `JWT_SECRET` 或 `.env.local` 注入，勿提交真值** |
| `jwt.expire_hours` | `168` | 登录态有效期 |
| `upload.max_size_mb` | `20` | **旧 multipart 通道**单文件上限（tus 通道见 `resumable`） |
| `upload.thumb_max_edge` | `400` | 缩略图最长边 |
| `resumable.enabled` | `true` | 是否启用分片/断点续传 |
| `resumable.max_size_mb` | `150` | tus 单文件总大小上限 |
| `resumable.max_chunk_mb` | `2` | 单个分片请求体上限 |
| `resumable.chunks_dir` | `""` | 分片临时目录，留空为 `data/chunks` |
| `resumable.max_concurrent` | `30` | 单账号同时上传中的会话上限 |
| `resumable.admission_per_second` | `5` | 新建会话准入令牌桶速率 |
| `resumable.admission_burst` | `20` | 准入令牌桶突发容量 |
| `resumable.global_max_concurrent` | `100` | 全平台在传总数上限（负值不限） |
| `resumable.credential_ttl_minutes` | `60` | 短期上传凭证有效期 |
| `resumable.session_ttl_hours` | `4` | 未完成会话/分片回收时长 |
| `resumable.active_window_minutes` | — | 并发计数只统计该时间窗内活跃的会话 |
| `storage.data_dir` | — | 本地存储根目录（未配置 MinIO 时使用） |
| `oauth.issuer` / `oauth.client_id` / `oauth.client_secret` | — | **UniSSO** 凭证（历史命名沿用 `oauth` 段） |

## 环境变量

环境变量优先于 `config.yaml`；部署平台上建议用环境变量注入密钥。

| 变量 | 作用 |
|---|---|
| `PORT` / `SERVER_MODE` | 端口 / gin 模式 |
| `PUBLIC_BASE` / `SDPY_PUBLIC_URL` / `SDPY_API_PUBLIC_URL` / `SDPY_PUBLIC_BASE` / `PATH_PREFIX` | 外网前缀与路径部署前缀 |
| `DATA_DIR` / `SDPY_DATA_DIR` | 本地数据根目录 |
| `DB_PATH` | SQLite 路径（可指向 `/dev/shm` 以避开慢速磁盘，见「运维」） |
| `JWT_SECRET` | 会话签名密钥 |
| `FRONTEND_DIST` | 前端产物目录 |
| `IMAGEBED_ALLOWED_ORIGINS` | CORS 白名单（逗号分隔） |
| `MINIO_ENDPOINT` / `MINIO_ACCESS_KEY` / `MINIO_SECRET` / `MINIO_BUCKET` | 四项**全部**提供才启用 MinIO，否则回落本地目录 |
| `UNISSO_ISSUER` / `UNISSO_CLIENT_ID` / `UNISSO_CLIENT_SECRET` | UniSSO 凭证 |
| `IMAGEBED_RESUMABLE_MAX_SIZE_MB` / `_MAX_CHUNK_MB` / `_CHUNKS_DIR` / `_MAX_CONCURRENT` / `_ADMISSION_PER_SECOND` / `_ADMISSION_BURST` / `_GLOBAL_MAX_CONCURRENT` / `_CREDENTIAL_TTL_MINUTES` / `_SESSION_TTL_HOURS` / `_ACTIVE_WINDOW_MINUTES` | `resumable` 各段覆盖 |
| `DB_PERSIST_SECONDS` / `DB_PERSIST_KEEP` | 仅 `tools/db_persist.py` 使用：持久化间隔秒数 / 保留份数 |

## 存储后端与对象布局

两种模式二选一（由 `MINIO_*` 四项是否齐全决定）：

| 模式 | 说明 |
|---|---|
| `minio` | 对象写入 bucket；日志会打印 `二进制存储后端: minio:<bucket>` |
| `local` | 写入 `DATA_DIR` 下的同名目录结构；日志打印 `二进制存储后端: local` |

对象 key 布局（两种模式一致）：

```
images/YYYY/MM/DD/<uuid>.<ext>       # 图片原图
objects/YYYY/MM/DD/<uuid>.<ext>      # 文档/附件
thumbs/YYYY/MM/DD/<uuid>.jpg         # 缩略图
cache/<原图路径>_w<宽>_h<高>_q<质量>.jpg   # 按需缩放结果缓存（可再生）
```

## API

统一响应：`{ "code": 0, "message": "ok", "data": ... }`;`code != 0` 为失败。

### 认证（UniSSO）

```bash
# 邮箱 + 密码登录（后端经 UniSSO password/verify 校验，要求内部账号）
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"you@example.com","password":"***"}'
# → data: { "token": "<jwt>", "user": {...} }

# 静默 SSO：读取父域共享 Cookie（unisso_sid，兼容期回落 sse_sso）
curl -X POST http://localhost:8080/api/auth/sso/try
# → 200 同上；401 无会话 / 会话失效；503 图床暂不可用

# 单点登出：撤销共享会话 + 清除父域 Cookie（新旧 Cookie 名双清）
curl -X POST http://localhost:8080/api/auth/logout

# 当前用户
curl http://localhost:8080/api/auth/me -H "Authorization: Bearer <jwt>"
```

### 鉴权体系（三层）

| 凭证 | 签发方 | 用途 | 有效期 |
|---|---|---|---|
| 用户 JWT | `POST /api/auth/login` 或 `POST /api/auth/sso/try` | Web 界面、管理接口 | `jwt.expire_hours` |
| API Token（`imgb_` 前缀） | `POST /api/tokens`（需 JWT） | 第三方模块调用上传/接口 | 创建时指定，可为永久 |
| 短期上传凭证 | `POST /api/upload/credential`（JWT 或 API Token） | **浏览器直连 tus**,避免暴露长效 token | `resumable.credential_ttl_minutes` |

### 上传

#### ① 管理接口（JWT，multipart）

```bash
curl -X POST http://localhost:8080/api/images \
  -H "Authorization: Bearer $JWT" \
  -F "file=@a.png" -F "file=@b.jpg"
```

#### ② API 上传（API Token）

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

返回 `{ url, thumb_url, markdown, html, mime_type, size, width, height }`。

#### ③ tus 断点续传（推荐用于大文件）

**第一步：换取短期上传凭证**

```bash
curl -X POST http://localhost:8080/api/upload/credential \
  -H "Authorization: Bearer $JWT"      # 也可用 API Token
# → data: { "token": "<短期jwt>", "expires_in": 3600 }
# 前端用它拼出 tus 端点：<public_base>/api/upload/resumable/
```

**第二步：tus 协议直连**（可用 `tus-js-client`，或 curl 手动验证）

```bash
# 建会话（POST），声明总长度与元数据
curl -X POST http://localhost:8080/api/upload/resumable/ \
  -H "Authorization: Bearer <短期jwt>" \
  -H "Tus-Resumable: 1.0.0" \
  -H "Upload-Length: 33554432" \
  -H "Upload-Metadata: filename ZGF0YS5kb2N4" \
  -D - -o /dev/null
# → 201 + Location: <上传URL>

# 上传分片（PATCH），可多次、可中断后按偏移续传
curl -X PATCH <Location> \
  -H "Authorization: Bearer <短期jwt>" \
  -H "Tus-Resumable: 1.0.0" \
  -H "Upload-Offset: 0" \
  -H "Content-Type: application/offset+octet-stream" \
  --data-binary @chunk1

# 查询进度（HEAD） / 取消（DELETE）
```

行为说明：

- 分片用 `filestore` 落盘、`memorylocker` 进程内加锁；**下载接口关闭**
- 完成响应会带 **`X-Object-URL`** 头（最终直链），前端据此展示/保存
- 限制：单文件 ≤ `resumable.max_size_mb`(150MB)、单片 ≤ `max_chunk_mb`(2MB)
- 并发：单账号在传 ≤ `max_concurrent`(30)；新建会话准入 5/秒（突发 20）；全平台 ≤ 100
- 未完成会话 `session_ttl_hours`(4h) 后自动回收分片

> 客户端建议：`tus-js-client` 的 `chunkSize` 设为 2MB，凭证临近过期时主动换取新凭证（401 时重试）。

### 图片管理（需 JWT）

```bash
# 列表（分页 / 关键词 / 可见性）
curl "http://localhost:8080/api/images?page=1&page_size=24&keyword=cat&visibility=public" \
  -H "Authorization: Bearer $JWT"

# 详情 / 取文件
curl http://localhost:8080/api/images/1          -H "Authorization: Bearer $JWT"
curl http://localhost:8080/api/images/1/file     -H "Authorization: Bearer $JWT"

# 切换可见性 / 删除
curl -X PATCH http://localhost:8080/api/images/1 \
  -H "Authorization: Bearer $JWT" -H "Content-Type: application/json" \
  -d '{"visibility":"private"}'
curl -X DELETE http://localhost:8080/api/images/1 -H "Authorization: Bearer $JWT"
```

### Token 管理（需 JWT）

```bash
# 创建（expires_at 为 RFC3339；省略或 null 表示永不过期）
curl -X POST http://localhost:8080/api/tokens \
  -H "Authorization: Bearer $JWT" -H "Content-Type: application/json" \
  -d '{"name":"typora","expires_at":"2026-12-31T23:59:59+08:00"}'
# → data: { token, id, name, expires_at, created_at }（完整 token 只在此次返回）

curl http://localhost:8080/api/tokens            -H "Authorization: Bearer $JWT"
curl -X DELETE http://localhost:8080/api/tokens/1 -H "Authorization: Bearer $JWT"
```

### 文件直链与按需缩放

| 路径 | 用途 |
|---|---|
| `GET /i/<path>` | 图片（`images/...`、`thumbs/...`） |
| `GET /o/<path>` | 文档/附件（`objects/...`） |

私有文件需携带本人 JWT 或 Token（`Authorization` 头或 `?token=` 查询参数）。

**按需缩放**（仅图片）：

```bash
GET /i/images/2026/11/22/<uuid>.png?w=1600&q=82&fmt=jpg
```

| 参数 | 说明 |
|---|---|
| `w` / `h` | 目标宽/高（至少给一个；上限 4096） |
| `q` | JPEG 质量（默认 85） |
| `fmt` | `jpg`（默认）或 `png` |

行为约定：

- **不放大**：原图小于目标尺寸时直接返回原图
- **体积兜底**：重编码后反而变大时回退原图（图形类 PNG 常见）
- 生成结果写入 `cache/...`,后续命中直接返回；无参数时永远返回原图

## 部署

### SSE Deploy 平台

平台以仓库根目录的 `deploy.sh` 为入口，典型要求：

1. 创建「自定义部署」：绑定本仓库、类型选**常驻服务**、健康检查填 `/health`
2. 端口由平台分配（frp），应用监听 `0.0.0.0:$PORT`
3. 子域名填写**单层**前缀（如 `image-test` → `image-test.ssemarket.cn`,受现有 `*.ssemarket.cn` 证书覆盖）

`deploy.sh` 的行为：构建前端（无 `web/dist` 或部署前缀变化时）→ 编译缺失的后端二进制 → 加载 `.env.local` → 启动 `imagebed`。

### `.env.local` 约定

不便入库的配置放在部署根目录的 `.env.local`（权限 600、已被 `.gitignore` 忽略）：

- `deploy.sh` **只补充尚未设置的变量**,平台注入的环境变量优先
- 典型内容：`SDPY_PUBLIC_URL`、`UNISSO_CLIENT_ID/SECRET/ISSUER`、`MINIO_ACCESS_KEY/SECRET/BUCKET`

### 已知坑

| 现象 | 原因 / 处理 |
|---|---|
| 启动报「仓库未找到可执行 deploy.sh」 | 文件权限需 `100755`（`git update-index --chmod=+x deploy.sh`） |
| `npm ERR 407 Proxy Authentication Required` | 平台会注入需认证的代理；`deploy.sh` 已在构建期 `unset` 代理 |
| `go.dev` 下载超时 | `deploy.sh` 已改为多镜像回退（go.dev → 阿里云 → golang.google.cn） |
| 改了代码但部署没生效 | 面板不一定会重新拉取代码，需确认 pod 内 `git log` 是否为最新提交 |
| 前端被意外重建 | 前端构建受 `web/dist/.sdpy_base` 标记控制；标记缺失会触发重建 |

## 运维

### 健康检查

`GET /health` 返回前端首页（HTTP 200），仅用于**探活**（进程是否存活），不代表存储/数据库可用。

### 日志

进程日志由 supervisor 收集（stdout/stderr）；平台面板可查看。上传失败、凭证签发失败会有明确日志行，建议监控其频率。

### 数据库内存化（可选，规避慢速磁盘）

当数据目录位于网络盘（NFS）时，SQLite 读写可能被拖慢。可在 MinIO 模式下启用内存化：

- 启动时 `tools/db_bootstrap.py` 从 MinIO `meta/imagebed.db` 拉取数据库到 `/dev/shm/imagebed.db`,并校验可读；失败自动回退原路径
- `tools/db_persist.py` 每 `DB_PERSIST_SECONDS`(默认 300s) 生成一致性快照并上传，保留最近 `DB_PERSIST_KEEP`(默认 10) 份
- 换库用 `tools/swap-db.sh`（mv 语义、失败自动恢复应用）
- 权衡：pod 异常终止时最多丢失一个持久化间隔内的数据库变更（**对象本身不受影响**）

### 容灾（生产 ⇄ 测试环境）

同构环境之间可按「域名切流 + 每日增量」的方式互备：

- 切流：改 nginx 上游指向（秒级生效、可回退），**不需要修改任何内容中的链接**（两个环境对象路径完全相同）
- 同步：按数据库记录逐对象增量复制（`DB` 快照 + 对象），只增不删，保留快照若干份
- 具体脚本（`imagebed-failover.py` / `imagebed-sync.sh`）部署在运维主机，不属于本仓库

## 安全说明

- **不存储用户密码**：身份由 UniSSO 校验，本地只保存用户标识与展示资料
- **密钥不进仓库**：`jwt.secret`、UniSSO `client_secret`、MinIO AK/SK 一律用环境变量或 `.env.local` 注入；仓库内的 `config.yaml` 为**脱敏占位符**
- **最小权限**：为不同环境申请独立 MinIO bucket 与独立 AK/SK（只授权该 bucket），避免误操作生产数据
- **短期凭证**：浏览器只接触有效期 `credential_ttl_minutes` 的上传凭证，且仅能用于上传
- **私有文件**：需本人 JWT / Token;公开文件走 `/i`、`/o` 直链
- **CORS**：`server.allowed_origins` / `IMAGEBED_ALLOWED_ORIGINS` 白名单控制；tus 自定义头与 `X-Object-URL` 已在默认配置中放行

## 参考

- tus 协议：https://tus.io/
- tusd（Go 实现）：https://github.com/tus/tusd
- tus-js-client：https://github.com/tus/tus-js-client
- MinIO Go SDK：https://github.com/minio/minio-go
