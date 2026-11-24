#!/bin/bash
# 在 pod 内换库：优先操作内存盘（/dev/shm），全部用 mv（重命名），不复制数据。
# 换库后立刻向 MinIO 持久化一次——否则应用重启时 db_bootstrap 会从 MinIO 拉回旧库，
# 把刚换上的新库覆盖掉（2026-11-24 事故根因）。持久化失败即回滚，绝不留下不一致状态。
# 用法：swap-db.sh <program> <data_dir> <new_db_file>
set -uo pipefail
PROG="${1:?program}"
DATA="${2:?data dir}"
NEW="${3:?new db file}"
SUP=${SUPERVISORCTL:-/home/cloud/.local/bin/supervisorctl}
CONF=${SUPERVISORD_CONF:-/home/cloud/deploy/supervisord.conf}
TOOLS_DIR="$(cd "$(dirname "$0")" && pwd)"
DEPLOY_ROOT="$(dirname "$TOOLS_DIR")"
TS=$(date +%Y%m%d-%H%M%S)

# 运行中的库优先在内存盘
if [ -f /dev/shm/imagebed.db ]; then
  LIVE=/dev/shm/imagebed.db
  BAK=/dev/shm/imagebed.db.bak-$TS
else
  LIVE=$DATA/imagebed.db
  BAK=$DATA/imagebed.db.bak-$TS
fi

# 加载部署根目录的 .env.local（MinIO 凭据），否则持久化会静默失败
if [ -f "$DEPLOY_ROOT/.env.local" ]; then
  set -a
  # shellcheck disable=SC1090
  . "$DEPLOY_ROOT/.env.local"
  set +a
fi
[ -n "${MINIO_ENDPOINT:-}" ] || export MINIO_ENDPOINT="http://minio.platform-infra.svc.cluster.local:9000"

start_app() { timeout 20 "$SUP" -c "$CONF" start "$PROG" >/dev/null 2>&1 </dev/null || true; }
trap start_app EXIT

echo "[swap] 停止应用"
timeout 30 "$SUP" -c "$CONF" stop "$PROG" >/dev/null 2>&1 </dev/null || true

echo "[swap] 旧库重命名（不复制数据）-> $(basename "$BAK")"
[ -f "$LIVE" ] && mv -f "$LIVE" "$BAK" || echo "[swap] 无旧库"
rm -f "$LIVE-wal" "$LIVE-shm" 2>/dev/null || true

echo "[swap] 新库就位"
if [ ! -f "$NEW" ]; then
  echo "[swap] 缺少新库 $NEW，回滚"
  [ -f "$BAK" ] && mv -f "$BAK" "$LIVE"
  exit 1
fi
mv -f "$NEW" "$LIVE" || { echo "[swap] 替换失败，回滚"; [ -f "$BAK" ] && mv -f "$BAK" "$LIVE"; exit 1; }
chmod 644 "$LIVE" 2>/dev/null || true

echo "[swap] 新库记录数: $(python3 - "$LIVE" <<'PY'
import sqlite3, sys
db = sqlite3.connect('file:%s?mode=ro' % sys.argv[1], uri=True)
print(db.execute('select count(*) from images').fetchone()[0])
PY
)"

# 关键步骤：把新库同步到 MinIO。失败则回滚（否则应用重启会拉回旧库）
if DB_PATH="$LIVE" python3 "$TOOLS_DIR/db_persist.py" --once; then
  echo "[swap] 已持久化到 MinIO"
else
  echo "[swap] 持久化失败，回滚旧库（避免重启后拉到旧库）"
  rm -f "$LIVE"
  [ -f "$BAK" ] && mv -f "$BAK" "$LIVE"
  exit 1
fi

echo "[swap] 完成（应用由 trap 拉起）"
exit 0
