#!/usr/bin/env python3
"""极简 S3/MinIO 客户端（SigV4 签名），供 db_bootstrap / db_persist 使用。

只实现本用例需要的三个操作：GET 对象、PUT 对象、LIST 前缀、DELETE 对象。
所有请求逐字节流式处理，不缓存大对象；内存占用与对象大小同阶（数据库仅数 MB）。
"""
import datetime
import hashlib
import hmac
import os
import urllib.error
import urllib.parse
import urllib.request

REGION, SERVICE = 'us-east-1', 's3'


def _cfg():
    env = os.environ
    endpoint = env['MINIO_ENDPOINT']
    host = endpoint.split('://', 1)[1]
    ak = env.get('MINIO_ACCESS_KEY') or env.get('MINIO_ACCESS_KEY_ID')
    sk = env.get('MINIO_SECRET') or env.get('MINIO_SECRET_ACCESS_KEY')
    bucket = env['MINIO_BUCKET']
    return host, ak, sk, bucket


def _sign(host, ak, sk, method, uri, query, payload):
    now = datetime.datetime.now(datetime.timezone.utc)
    amz, ds = now.strftime('%Y%m%dT%H%M%SZ'), now.strftime('%Y%m%d')
    ph = hashlib.sha256(payload).hexdigest()
    headers = 'host:%s\nx-amz-content-sha256:%s\nx-amz-date:%s\n' % (host, ph, amz)
    canonical = '%s\n%s\n%s\n%s\n%s\n%s' % (method, uri, query, headers,
                                            'host;x-amz-content-sha256;x-amz-date', ph)
    scope = '%s/%s/%s/aws4_request' % (ds, REGION, SERVICE)
    to_sign = 'AWS4-HMAC-SHA256\n%s\n%s\n%s' % (amz, scope, hashlib.sha256(canonical.encode()).hexdigest())

    def h(key, msg):
        return hmac.new(key, msg.encode(), hashlib.sha256).digest()

    k = h(('AWS4' + sk).encode(), ds)
    k = h(k, REGION)
    k = h(k, SERVICE)
    k = h(k, 'aws4_request')
    sig = hmac.new(k, to_sign.encode(), hashlib.sha256).hexdigest()
    return {
        'Authorization': 'AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=host;x-amz-content-sha256;x-amz-date, Signature=%s' % (ak, scope, sig),
        'x-amz-date': amz,
        'x-amz-content-sha256': ph,
    }


def _request(method, key, query='', data=None, timeout=60):
    host, ak, sk, bucket = _cfg()
    uri = '/%s/%s' % (bucket, key) if key else '/%s' % bucket
    url = 'http://%s%s%s' % (host, uri, ('?' + query) if query else '')
    req = urllib.request.Request(url, data=data, method=method)
    for k, v in _sign(host, ak, sk, method, uri, query, data or b'').items():
        req.add_header(k, v)
    return urllib.request.urlopen(req, timeout=timeout)


def s3_get(key, timeout=60):
    with _request('GET', key, timeout=timeout) as resp:
        return resp.read()


def s3_put(key, data, timeout=180):
    with _request('PUT', key, data=data, timeout=timeout) as resp:
        return resp.status


def s3_delete(key, timeout=30):
    try:
        with _request('DELETE', key, timeout=timeout) as resp:
            return resp.status
    except urllib.error.HTTPError as e:
        if e.code == 404:
            return 404
        raise


def s3_list_prefix(prefix, timeout=60):
    """返回 [(key, size)]"""
    import re
    out, token = [], None
    while True:
        # SigV4 规范查询串要求：参数按 key 字典序排列，且按 RFC3986 全量编码
        # （'/' 必须写成 %2F）。顺序或编码任一不符都会 403。
        params = {'list-type': '2', 'max-keys': '1000', 'prefix': prefix}
        if token:
            params['continuation-token'] = token
        q = '&'.join(
            '%s=%s' % (k, urllib.parse.quote(v, safe=''))
            for k, v in sorted(params.items())
        )
        with _request('GET', '', query=q, timeout=timeout) as resp:
            body = resp.read().decode('utf-8', 'replace')
        keys = re.findall(r'<Key>[^<]+</Key>', body)
        sizes = [int(s) for s in re.findall(r'<Size>(\d+)</Size>', body)]
        for k, sz in zip(keys, sizes):
            out.append((k[5:-6], sz))
        m = re.search(r'<NextContinuationToken>([^<]+)</NextContinuationToken>', body)
        if not m:
            return out
        token = m.group(1)
