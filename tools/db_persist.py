#!/usr/bin/env python3
"""
数据库持久化循环：把内存盘里的 SQLite 定时快照上传到 MinIO。

- 立即执行一次（用于首次播种 / 重启后立刻补一份）
- 之后每 DB_PERSIST_SECONDS（默认 300 秒）一次
- 只保留最近 DB_PERSIST_KEEP（默认 10）份历史对象，自动清理
- 内存占用：单次快照约等于数据库大小（3~5MB），完成即释放
- 进程互斥：用 flock 锁文件，避免重启后出现多个循环

对象布局：
  meta/imagebed.db                最新快照（bootstrap 直接拉这个）
  meta/history/imagebed-<ts>.db   历史快照（保留最近 N 份）
"""
import fcntl
import os
import sqlite3
import sys
import time

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from minio_sigv4 import s3_delete, s3_list_prefix, s3_put  # noqa: E402

LOCK = '/tmp/db-persist.lock'
DB = os.environ.get('DB_PATH', '/dev/shm/imagebed.db')
INTERVAL = int(os.environ.get('DB_PERSIST_SECONDS', '300'))
KEEP = int(os.environ.get('DB_PERSIST_KEEP', '10'))
LATEST = 'meta/imagebed.db'
HISTORY_PREFIX = 'meta/history/'


def snapshot(path):
    tmp = '/tmp/db-snapshot.tmp'
    if os.path.exists(tmp):
        os.unlink(tmp)
    db = sqlite3.connect(DB)
    try:
        db.execute('VACUUM INTO ?', (tmp,))
    finally:
        db.close()
    with open(tmp, 'rb') as f:
        data = f.read()
    os.unlink(tmp)
    return data


def persistence_round():
    data = snapshot(DB)
    ts = time.strftime('%Y%m%d-%H%M%S')
    s3_put(LATEST, data)
    s3_put(HISTORY_PREFIX + 'imagebed-%s.db' % ts, data)
    # 清理超量历史
    items = s3_list_prefix(HISTORY_PREFIX)
    items.sort(key=lambda t: t[0])
    for key, _size in items[:max(0, len(items) - KEEP)]:
        s3_delete(key)
    return len(data), len(items)


def main():
    if '--once' in sys.argv:  # 手动播种：上传一次后退出
        size, n = persistence_round()
        print('[persist] 单次上传完成 %d bytes（历史 %d 份）' % (size, n))
        return 0

    lock = open(LOCK, 'w')
    try:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
    except OSError:
        print('[persist] 已有实例在运行，退出')
        return 0

    print('[persist] 启动 db=%s interval=%ds keep=%d' % (DB, INTERVAL, KEEP), flush=True)
    while True:
        try:
            size, n = persistence_round()
            print('[persist] %s 上传 %d bytes（历史 %d 份）' % (time.strftime('%F %T'), size, n), flush=True)
        except Exception as e:  # noqa: BLE001
            print('[persist] %s 失败: %s: %s' % (time.strftime('%F %T'), type(e).__name__, e), flush=True)
        time.sleep(INTERVAL)


if __name__ == '__main__':
    sys.exit(main())
