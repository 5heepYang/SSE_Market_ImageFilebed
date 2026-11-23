#!/usr/bin/env python3
"""
启动时把数据库从 MinIO 拉到内存盘，返回应使用的 DB 路径。

背景：pod 的 /home/cloud 是 NFS 网络盘，平台存储卡顿会让每次查询都阻塞
（实测单请求 16 秒）。数据库只有几 MB，放到 /dev/shm（pod 内存）后读写为
微秒级；持久化由 tools/db_persist.py 定时上传到 MinIO。

行为：
- MinIO 未配置 → 打印 NFS 路径（保持原行为）
- MinIO 有 meta/imagebed.db → 下载到 /dev/shm/imagebed.db，打印该路径
- 下载失败 / 对象不存在 → 打印 NFS 路径（保证服务一定起得来）

用法：db_bootstrap.py <repo_root>
"""
import os
import sqlite3
import sys
import urllib.error
import urllib.request

sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), '.'))
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

SHM = '/dev/shm/imagebed.db'
OBJECT = 'meta/imagebed.db'


def main():
    root = sys.argv[1] if len(sys.argv) > 1 else os.getcwd()
    data_dir = os.environ.get('DATA_DIR') or os.path.join(root, 'data')
    nfs_db = os.path.join(data_dir, 'imagebed.db')

    env = os.environ
    need = ('MINIO_ENDPOINT', 'MINIO_BUCKET')
    if not all(env.get(k) for k in need) or not (env.get('MINIO_ACCESS_KEY') or env.get('MINIO_ACCESS_KEY_ID')):
        print(nfs_db)
        return 0

    if not os.path.isdir('/dev/shm'):
        print(nfs_db)
        return 0

    try:
        from minio_sigv4 import s3_get  # 同目录辅助模块
    except Exception:
        print(nfs_db)
        return 0

    tmp = SHM + '.download'
    try:
        data = s3_get(OBJECT, timeout=20)
        if not data or len(data) < 4096 or data[:16] != b'SQLite format 3\x00':
            raise ValueError('下载内容不是有效 SQLite')
        with open(tmp, 'wb') as f:
            f.write(data)
        # 校验可读
        db = sqlite3.connect('file:%s?mode=ro' % tmp, uri=True)
        db.execute('select count(*) from images').fetchone()
        db.close()
        os.replace(tmp, SHM)
        print(SHM)
        return 0
    except Exception as e:  # noqa: BLE001
        print('bootstrap: fallback to NFS (%s)' % e, file=sys.stderr)
        try:
            os.unlink(tmp)
        except OSError:
            pass
        print(nfs_db)
        return 0


if __name__ == '__main__':
    sys.exit(main())
