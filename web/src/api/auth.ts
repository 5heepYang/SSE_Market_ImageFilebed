import { http } from './http'
import { saveRedirect } from '../store/auth'
import type { User } from '../store/auth'
import { pushToast } from '../composables/toast'

export function oauthLoginUrl() {
  return http.get('/api/auth/oauth/login-url') as Promise<{ authorize_url: string; state: string }>
}

export function oauthExchangeToken(code: string, state: string) {
  return http.post('/api/auth/oauth/token', { code, state }) as Promise<{ session_id: string }>
}

export function oauthFetchUserInfo(sessionId: string) {
  return http.post('/api/auth/oauth/userinfo', { session_id: sessionId }) as Promise<{
    token: string
    user: User
    profile: Record<string, unknown>
  }>
}

export function oauthConfig() {
  return http.get('/api/auth/oauth/config') as Promise<{ app_name: string; app_icon?: string }>
}

export function me() {
  return http.get('/api/auth/me') as Promise<User>
}

// —— 统一登录跳转（唯一登录方式）——

let oauthRedirecting = false

// redirectToOAuth 发起软工集市 OAuth 登录：保存回跳目标后拉取授权地址并整页跳转。
// 用模块级标记防止并发触发多次；失败时复位标记以便重试。
export async function redirectToOAuth(redirectPath = '/'): Promise<boolean> {
  if (oauthRedirecting) return false
  oauthRedirecting = true
  saveRedirect(redirectPath)
  try {
    const { authorize_url } = await oauthLoginUrl()
    window.location.href = authorize_url
    return true
  } catch (e: any) {
    oauthRedirecting = false
    pushToast(e.message || '无法发起登录，请确认 OAuth 已配置', 'error')
    return false
  }
}
