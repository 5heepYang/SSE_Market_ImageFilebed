import type { User } from '../store/auth'
import { apiBase, http } from './http'

/** 邮箱 + 密码登录（后端经 UniSSO password/verify 校验，要求内部账号） */
export function login(email: string, password: string) {
  return http.post('/api/auth/login', { email, password }) as Promise<{ token: string, user: User }>
}

/** 单点登出：撤销 UniSSO 共享会话并清理父域 Cookie；本地 JWT 由调用方清掉 */
export function logout() {
  return http.post('/api/auth/logout') as Promise<unknown>
}

export function me() {
  return http.get('/api/auth/me') as Promise<User>
}

/**
 * 静默 SSO 探测：读取 UniSSO 共享 Cookie，命中则直接签发本地会话。
 *
 * 刻意使用原生 fetch 而非 axios：axios 的全局 401 拦截器会 clearAuth 并整页
 * 跳登录页，而这里的 401 是"未登录"的正常结果，必须由调用方安静处理。
 * 任何失败（未登录 / 服务不可用 / 网络错误）都返回 null，不抛异常。
 */
export async function trySilentSSO(): Promise<{ token: string, user: User } | null> {
  try {
    const base = apiBase()
    const resp = await fetch(`${base}api/auth/sso/try`, {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
    })
    if (!resp.ok)
      return null
    const body = await resp.json()
    if (body?.code === 0 && body?.data?.token)
      return body.data as { token: string, user: User }
    return null
  }
  catch {
    return null
  }
}
