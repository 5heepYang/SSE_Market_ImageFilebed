#!/bin/bash
# 在 pod 内换库：优先操作内存盘（/dev/shm），全部用 mv（重命名），不复制数据。
# 用法：swap-db.sh <program> <data_dir> <new_db_file>
set -uo pipefail
PROG="${1:?program}"
DATA="${2:?data dir}"
NEW="${3:?new db file}"
SUP=${SUPERVISORCTL:-/home/cloud/.local/bin/supervisorctl}
CONF=${SUPERVISORD_CONF:-/home/cloud/deploy/supervisord.conf}
TS=$(date +%Y%m%d-%H%M%S)

# 运行中的库优先在内存盘
if [ -f /dev/shm/imagebed.db ]; then
  LIVE=/dev/shm/imagebed.db
  BAK=/dev/shm/imagebed.db.bak-$TS
else
  LIVE=$DATA/imagebed.db
  BAK=$DATA/imagebed.db.bak-$TS
fi

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

echo "[swap] 记录数: $(python3 - "$LIVE" <<'PY'
import sqlite3, sys
db = sqlite3.connect('file:%s?mode=ro' % sys.argv[1], uri=True)
print(db.execute('select count(*) from images').fetchone()[0])
PY
)"
# 换库后立即持久化一份到 MinIO，避免重启时回退到旧快照
DB_PATH="$LIVE" python3 "$(dirname "$0")/db_persist.py" --once >/dev/null 2>&1 || true
echo "[swap] 完成（应用由 trap 拉起）"
exit 0
