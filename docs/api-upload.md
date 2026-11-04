# 图床 API 上传说明

通过 HTTP API 上传图片并获取外链，适合脚本、第三方工具（Typora、PicGo、Obsidian、命令行等）接入。

## 1. 概览

| 项 | 说明 |
|----|------|
| 接口 | `POST /api/upload` |
| 鉴权 | 个人 API Token（`Authorization: Bearer <token>` 或 `?token=<token>`） |
| 上传方式 | multipart / 原始二进制 / JSON(base64) |
| 返回 | 直链 `url`、缩略图 `thumb_url`、`markdown`、`html`、`size`、`width`、`height` |
| 支持类型 | 图片：png / jpg / jpeg / gif / webp / bmp / svg；文档：pdf / doc / docx / xls / xlsx / ppt / pptx |
| 大小上限 | 单文件默认 20MB（`config.yaml` 的 `upload.max_size_mb`） |

## 2. 获取 Token

1. 浏览器登录图床（软工集市 OAuth）。
2. 进入「Token」页，点「创建 Token」，可选设置过期时间（不设则永不过期）。
3. 复制生成的 Token（形如 `imgb_<64位hex>`）——**原文仅此一次可见**，请妥善保存。

> 也可用管理接口创建（需先登录拿到 JWT）：
>
> ```bash
> curl -X POST "$BASE/api/tokens" \
>   -H "Authorization: Bearer $JWT" \
>   -H "Content-Type: application/json" \
>   -d '{"name":"typora","expires_at":"2026-12-31T23:59:59+08:00"}'
> # → { "code": 0, "data": { "token": "imgb_...", "id": 1, ... } }
> ```

## 3. 上传图片

`$BASE` 为后端地址（本地 `http://localhost:8080`，生产以实际部署地址为准），`$TOKEN` 为第 2 步的 Token。

### 3.1 multipart（推荐，支持多文件）

```bash
curl -X POST "$BASE/api/upload" \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@photo.png" \
  -F "file=@another.jpg"
```

### 3.2 原始二进制

```bash
curl -X POST "$BASE/api/upload" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: image/png" \
  --data-binary @photo.png
```

### 3.3 JSON base64

```bash
curl -X POST "$BASE/api/upload" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"image":"<base64编码>","filename":"photo.png"}'
```

### 3.5 上传文档

文档使用 multipart 上传即可（推荐）；也可用原始二进制方式，`Content-Type` 需为对应 MIME：

```bash
curl -X POST "$BASE/api/upload" \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@report.docx" \
  -F "file=@slides.pptx" \
  -F "file=@sheet.xlsx" \
  -F "file=@paper.pdf"
```

### 3.4 用 `?token=` 传鉴权（便于无 Header 的场景）

```bash
curl -X POST "$BASE/api/upload?token=$TOKEN" -F "file=@photo.png"
```

## 4. 返回结果

成功（`code=0`）时 `data` 为数组（一次可传多张），每项：

```json
{
  "code": 0,
  "message": "ok",
  "data": [
    {
      "url": "/i/images/2026/08/15/xxxx.png",
      "original_name": "photo.png",
      "thumb_url": "/i/thumbs/2026/08/15/xxxx.jpg",
      "markdown": "![photo.png](/i/images/2026/08/15/xxxx.png)",
      "html": "<img src=\"/i/images/2026/08/15/xxxx.png\" alt=\"photo.png\" />",
      "mime_type": "image/png",
      "size": 20480,
      "width": 1920,
      "height": 1080
    }
  ]
}
```

| 字段 | 含义 |
|------|------|
| `url` | 图片为 `/i/images/...`，文档为 `/o/objects/...`；生产已带完整域名，本地开发为相对路径 |
| `original_name` | 上传时的原始文件名 |
| `thumb_url` | 图片缩略图直链（文档为空） |
| `markdown` | 图片为 `![name](url)`，文档为 `[name](url)` |
| `html` | 图片为 `<img>` 标签，文档为 `<a>` 下载链接 |
| `size` | 字节数 |
| `width` / `height` | 宽高（SVG 等非位图为 0） |

## 5. 获取完整链接

`url` 若以 `/` 开头（相对路径），拼上站点域名即可：

```
完整链接 = 站点域名 + url
```

例如本地开发 `url = "/i/images/2026/08/15/xxxx.png"`，站点域名为 `https://image.ssemarket.cn`，则完整链接为：

```
https://image.ssemarket.cn/i/images/2026/08/15/xxxx.png
```

> 生产（SSE Deploy 子域名）已把 `https://image.ssemarket.cn` 写入 `PUBLIC_BASE` / `SDPY_PUBLIC_URL`，返回的 `url` 已带完整域名；本地开发未配置时返回相对路径，补上域名即可。

### 5.1 按需生成不同尺寸

图片直链支持查询参数按需缩放，第一次请求会生成并缓存，后续同尺寸请求直接命中缓存：

```text
GET /i/images/2026/09/09/<uuid>.png?w=800&h=600&q=85&fmt=jpg
```

| 参数 | 默认 | 说明 |
|------|------|------|
| `w` | `0` | 目标宽度；为 0 时按高度等比缩放 |
| `h` | `0` | 目标高度；为 0 时按宽度等比缩放 |
| `q` | `85` | JPEG 质量，`1-100`；PNG 忽略 |
| `fmt` | 按原图 | `jpg` / `png`；PNG 保持透明，其余默认 JPEG |

`w` 和 `h` 同时指定时按比例缩放到给定边界内，不拉伸变形。生成结果缓存在对象存储的 `cache/` 目录下，公开图片响应头为 `Cache-Control: public, max-age=31536000, immutable`。

## 6. 常见错误

| code | 说明 | 处理 |
|------|------|------|
| 401 | Token 不存在 / 已过期 / 已吊销 | 重新创建 Token |
| 400 | 文件类型不支持 / 超大小 / 解析失败 | 换 png/jpg/gif/webp/bmp/svg，减小体积 |
| 500 | 服务端异常 | 联系管理员 |

## 7. 说明

- Token 明文仅在创建时返回一次，服务端只存 SHA-256；吊销后立即失效。
- 图片按文件内容嗅探 MIME 并做白名单校验；文档按扩展名 + 文件头双重校验，伪装扩展名无法绕过。
- 私有图需携带本人 JWT 或 Token 才能访问；API 上传默认 `public`。
- 文档直链默认 `inline`（浏览器可预览 PDF）；如需强制下载，访问时加 `?download=1`。

## 8. 断点续传 / 分片上传

大文档建议使用 tus 协议上传，接口为：

```text
POST /api/upload/resumable/
```

- 鉴权：`Authorization: Bearer <token>`。
- 客户端按 1MB 分片上传，服务端默认允许单文件最大 `150MB`、单请求最大 `2MB`。
- 创建上传时需在 `Upload-Metadata` 中携带 `filename`，例如 `filename <base64(报告.docx)>`。
- 上传完成后，响应头包含：
  - `X-Object-URL`：最终对象直链。
  - `X-Object-Name`：原始文件名。
  - `X-Object-Mime`：MIME 类型。
- 断点续传、暂停恢复、偏移量管理由 tusd 处理；服务重启后可从上次已持久化偏移继续。

推荐前端使用 `tus-js-client`，后端协议实现为 `tusd`。
