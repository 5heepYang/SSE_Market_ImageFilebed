import { reactive } from 'vue'

export interface User {
  id: number
  name: string
  email?: string
  avatar_url?: string
}

const TOKEN_KEY = 'imagebed_token'
const USER_KEY = 'imagebed_user'
const REDIRECT_KEY = 'imagebed_redirect'

export const auth = reactive<{ token: string; user: User | null }>({
  token: localStorage.getItem(TOKEN_KEY) || '',
  user: JSON.parse(localStorage.getItem(USER_KEY) || 'null') as User | null,
})

export function isLoggedIn(): boolean {
  return !!auth.token
}

export function setAuth(token: string, user: User) {
  auth.token = token
  auth.user = user
  localStorage.setItem(TOKEN_KEY, token)
  localStorage.setItem(USER_KEY, JSON.stringify(user))
}

export function clearAuth() {
  auth.token = ''
  auth.user = null
  localStorage.removeItem(TOKEN_KEY)
  localStorage.removeItem(USER_KEY)
}

// —— 登录跳转辅助 ——

// 保存登录后要跳回的目标路径（标签页级）。
export function saveRedirect(path: string) {
  sessionStorage.setItem(REDIRECT_KEY, path)
}

export function consumeRedirect(): string {
  const p = sessionStorage.getItem(REDIRECT_KEY) || '/'
  sessionStorage.removeItem(REDIRECT_KEY)
  return p
}
