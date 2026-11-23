#!/usr/bin/env bash
# imagebed → sseinfra / SDPY 压缩包入口（仓根可执行）
# 主入口：https://image.ssemarket.cn （子域名）；路径 /sdpy/image/ 仍可作备用
# 要求：前台常驻，监听 0.0.0.0:$PORT（默认 8080）
set -euo pipefail

PORT="${PORT:-8080}"
ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT"

# 可选本地环境文件：存放不便入库的配置（UNISSO_* 密钥、SDPY_PUBLIC_URL 等）。
# 文件位于部署根目录、已被 .gitignore 忽略；此处只补充「尚未设置」的变量，
# 平台注入的环境变量优先级更高，不会被覆盖。
if [ -f "$ROOT/.env.local" ]; then
  while IFS= read -r line || [ -n "$line" ]; do
    case "$line" in ''|\#*) continue ;; esac
    key="${line%%=*}"
    [ -n "$key" ] || continue
    if [ -z "${!key:-}" ]; then
      export "${line}"
    fi
  done < "$ROOT/.env.local"
  echo "[deploy] 已加载 .env.local（补充缺失变量）"
fi

# 部署平台会向进程注入需要认证的 HTTP(S)_PROXY（实测返回 407），
# 构建阶段的包下载（npm ci / Go 工具链）会因此失败。
# 本集群实测可直连 go 镜像、npm registry 与 GitHub，构建期清除代理即可。
unset HTTP_PROXY HTTPS_PROXY ALL_PROXY http_proxy https_proxy all_proxy 2>/dev/null || true

DATA="${SDPY_DATA_DIR:-${DATA_DIR:-$ROOT/data}}"
mkdir -p "$DATA/images" "$DATA/thumbs" "$DATA/objects" "$DATA/chunks"
export DATA_DIR="$DATA"
export DB_PATH="${DB_PATH:-$DATA/imagebed.db}"

# 数据库内存化：本平台 pod 的 /home/cloud 是 NFS，存储卡顿时每次查询都会阻塞
# （实测单请求 16 秒）。DB 只有几 MB，放到 /dev/shm（pod 内存）后读写为微秒级；
# 持久化由 tools/db_persist.py 定时把快照传回 MinIO，pod 重启时再从 MinIO 拉回。
# 仅当 MinIO 已配置时启用；任何一步失败都回退到 NFS，保证服务一定起得来。
if [ -n "${MINIO_ENDPOINT:-}" ] && [ -n "${MINIO_BUCKET:-}" ] && [ -d /dev/shm ] \
   && { [ -n "${MINIO_ACCESS_KEY:-}" ] || [ -n "${MINIO_ACCESS_KEY_ID:-}" ]; }; then
  SHM_DB="$(python3 "$ROOT/tools/db_bootstrap.py" "$ROOT" 2>>"${LOG_DIR:-$ROOT/logs}/db-persist.log" | tail -1 || true)"
  if [ "$SHM_DB" = "/dev/shm/imagebed.db" ] && [ -f "$SHM_DB" ]; then
    export DB_PATH="$SHM_DB"
    echo "[deploy] 数据库已载入内存盘: $DB_PATH"
    mkdir -p "${LOG_DIR:-$ROOT/logs}"
    setsid nohup python3 "$ROOT/tools/db_persist.py" \
      >> "${LOG_DIR:-$ROOT/logs}/db-persist.log" 2>&1 &
    echo "[deploy] 已启动 DB 持久化循环"
  else
    echo "[deploy] 内存盘不可用，数据库回退 NFS: $DB_PATH"
  fi
fi

export PORT
export SERVER_MODE="${SERVER_MODE:-release}"

PUBLIC_URL="${SDPY_PUBLIC_URL:-}"
# 去掉末尾 /
PUBLIC_URL="${PUBLIC_URL%/}"

# 子域名模式：https://image.ssemarket.cn （路径仅 / 或空）
is_subdomain_root() {
  local u="$1"
  [ -n "$u" ] || return 1
  # 有 host，且 path 为空或 /
  echo "$u" | grep -qE '^https?://[^/]+$' && return 0
  echo "$u" | grep -qE '^https?://[^/]+/$' && return 0
  return 1
}

if is_subdomain_root "$PUBLIC_URL"; then
  BASE_PATH="/"
  # 同源：前端与 API 同域，Axios 用相对路径
  API_BASE=""
  export PUBLIC_BASE="${PUBLIC_BASE:-$PUBLIC_URL}"
  export PATH_PREFIX=""
  echo "[deploy] 子域名模式 PUBLIC_URL=$PUBLIC_URL → VITE_BASE_PATH=/ 同源 API"
elif [ -n "${SDPY_PUBLIC_BASE:-}" ]; then
  BASE_PATH="${SDPY_PUBLIC_BASE}"
  case "$BASE_PATH" in
    */) ;;
    *) BASE_PATH="${BASE_PATH}/" ;;
  esac
  # 路径模式前端和 API 同源，统一使用前端挂载前缀。
  # 若 SDPY_PUBLIC_URL 只是主站根地址，跨到根 /api 会打到别的服务，导致文件预览失败。
  API_BASE="${BASE_PATH}"
  if [ -z "${PUBLIC_BASE:-}" ]; then
    if [ -n "${SDPY_API_PUBLIC_URL:-}" ]; then
      export PUBLIC_BASE="$SDPY_API_PUBLIC_URL"
    elif [ -n "$PUBLIC_URL" ]; then
      export PUBLIC_BASE="$PUBLIC_URL"
    fi
  fi
  export PATH_PREFIX="${SDPY_PUBLIC_BASE:-}"
  echo "[deploy] 路径模式 BASE_PATH=$BASE_PATH API_BASE=$API_BASE"
else
  # 打包默认 / 预构建；运行时若无 SDPY_* 则按子域名约定
  BASE_PATH="/"
  API_BASE=""
  export PUBLIC_BASE="${PUBLIC_BASE:-https://image.ssemarket.cn}"
  export PATH_PREFIX=""
  echo "[deploy] 默认子域名约定 PUBLIC_BASE=$PUBLIC_BASE"
fi

# UniSSO 客户端凭证由 supervisor 环境变量注入（UNISSO_CLIENT_ID / UNISSO_CLIENT_SECRET / UNISSO_ISSUER），
# 此处不再做任何 OAuth 变量兼容。

echo "[deploy] imagebed PORT=$PORT bind=0.0.0.0 DATA_DIR=$DATA_DIR"
echo "[deploy] PUBLIC_BASE=${PUBLIC_BASE:-} BASE_PATH=$BASE_PATH API_BASE=${API_BASE:-<same-origin>}"

build_web() {
  local web_dir="$ROOT/web"
  [ -f "$web_dir/package.json" ] || return 0
  command -v node >/dev/null 2>&1 || {
    echo "[deploy] 需要 node 以构建前端" >&2
    exit 1
  }
  echo "[deploy] 使用 Node $(node -v)；构建前端 VITE_BASE_PATH=$BASE_PATH …"
  (
    cd "$web_dir"
    if [ -f package-lock.json ]; then
      npm ci || npm install
    else
      npm install
    fi
    VITE_BASE_PATH="$BASE_PATH" VITE_API_BASE="$API_BASE" npm run build
  )
}

NEED_WEB=0
MARK_KEY="${BASE_PATH}|${API_BASE}|subdomain-v1"
if [ "${FORCE_REBUILD_WEB:-0}" = "1" ]; then
  NEED_WEB=1
elif [ ! -d "$ROOT/web/dist" ] || [ ! -f "$ROOT/web/dist/index.html" ]; then
  NEED_WEB=1
else
  MARK="$ROOT/web/dist/.sdpy_base"
  if [ ! -f "$MARK" ] || [ "$(cat "$MARK" 2>/dev/null | tr -d '\r\n')" != "$MARK_KEY" ]; then
    NEED_WEB=1
  fi
fi

if [ "$NEED_WEB" = "1" ]; then
  build_web
  echo "$MARK_KEY" > "$ROOT/web/dist/.sdpy_base"
fi

if [ ! -f "$ROOT/web/dist/index.html" ]; then
  echo "[deploy] 缺少 web/dist/index.html" >&2
  exit 1
fi
echo "[deploy] frontend index=$(head -c 200 "$ROOT/web/dist/index.html" | tr '\n' ' ')"

BIN=""
if [ -f "$ROOT/bin/imagebed" ]; then
  chmod +x "$ROOT/bin/imagebed" 2>/dev/null || true
  BIN="$ROOT/bin/imagebed"
elif [ -f "$ROOT/imagebed" ]; then
  chmod +x "$ROOT/imagebed" 2>/dev/null || true
  BIN="$ROOT/imagebed"
fi

if [ -z "$BIN" ]; then
  if ! command -v go >/dev/null 2>&1; then
    echo "[deploy] 下载 Go 工具链…"
    GO_VER="${GO_VERSION:-1.25.8}"
    ARCH="$(uname -m)"
    case "$ARCH" in
      x86_64|amd64) GO_ARCH=amd64 ;;
      aarch64|arm64) GO_ARCH=arm64 ;;
      *) echo "[deploy] 不支持的架构: $ARCH" >&2; exit 1 ;;
    esac
    TMP="$(mktemp -d)"
    # go.dev 在部分集群网络不可达（实测 WZ8 测试 pod 超时），按顺序回退到
    # 可达镜像：阿里云镜像 / golang.google.cn。可用 GO_DOWNLOAD_BASE 覆盖首选源。
    TGZ="go${GO_VER}.linux-${GO_ARCH}.tar.gz"
    GOT=0
    for BASE in "${GO_DOWNLOAD_BASE:-https://go.dev/dl}" \
                "https://mirrors.aliyun.com/golang" \
                "https://golang.google.cn/dl"; do
      echo "[deploy] 尝试下载 $BASE/$TGZ"
      if curl -fsSL -L --connect-timeout 10 --max-time 300 "$BASE/$TGZ" -o "$TMP/go.tgz"; then
        GOT=1
        break
      fi
      echo "[deploy] 该源不可用，换下一个"
    done
    if [ "$GOT" != "1" ]; then
      echo "[deploy] Go 工具链下载失败（所有源均不可用），可用 GO_VERSION/GO_DOWNLOAD_BASE 指定" >&2
      exit 1
    fi
    mkdir -p "$ROOT/.tools"
    rm -rf "$ROOT/.tools/go"
    tar -C "$ROOT/.tools" -xzf "$TMP/go.tgz"
    rm -rf "$TMP"
    export PATH="$ROOT/.tools/go/bin:$PATH"
  fi
  echo "[deploy] go build → bin/imagebed"
  mkdir -p "$ROOT/bin"
  (
    cd "$ROOT/server"
    export GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"
    CGO_ENABLED=0 go build -trimpath -ldflags '-s -w' -o "$ROOT/bin/imagebed" .
  )
  BIN="$ROOT/bin/imagebed"
fi

export FRONTEND_DIST="${FRONTEND_DIST:-$ROOT/web/dist}"
if [ ! -f "$ROOT/config.yaml" ] && [ -f "$ROOT/server/config.yaml" ]; then
  cp -f "$ROOT/server/config.yaml" "$ROOT/config.yaml"
fi

echo "[deploy] 启动 $BIN → 0.0.0.0:${PORT} frontend=$FRONTEND_DIST path_prefix=${PATH_PREFIX:-/}"
cd "$ROOT"
exec "$BIN"
