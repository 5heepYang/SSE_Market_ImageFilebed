import axios, { type AxiosInstance } from 'axios'
import { auth, clearAuth } from '../store/auth'

/** 子域名同源时 VITE_API_BASE 为空；路径部署时为完整后端公网地址 */
export function apiBase(): string {
  const api = (import.meta.env.VITE_API_BASE || '').trim()
  if (api) return api.replace(/\/?$/, '/')
  // 同源模式下必须跟随前端部署前缀：
  // 子域名/根路径部署为 /，路径部署（如 /test/image/）时 API 也应在 /test/image/api/。
  const base = import.meta.env.BASE_URL || '/'
  return base.endsWith('/') ? base : base + '/'
}

const instance: AxiosInstance = axios.create({
  baseURL: apiBase(),
  timeout: 60000,
})

instance.interceptors.request.use((config) => {
  if (auth.token) {
    config.headers.Authorization = `Bearer ${auth.token}`
  }
  return config
})

instance.interceptors.response.use(
  (resp) => {
    const body = resp.data
    if (body && typeof body === 'object' && 'code' in body) {
      if (body.code === 0) return body.data
      return Promise.reject(new Error(body.message || '请求失败'))
    }
    return body
  },
  (error) => {
    const status = error.response?.status
    const body = error.response?.data
    const message = body?.message || error.message || '网络错误'
    if (status === 401) {
      clearAuth()
      const loginPath = `${import.meta.env.BASE_URL}login`
      if (window.location.pathname !== loginPath) {
        window.location.replace(loginPath)
      }
    }
    return Promise.reject(new Error(message))
  },
)

export const http = {
  get: (url: string, config?: any) => instance.get(url, config) as unknown as Promise<any>,
  post: (url: string, data?: any, config?: any) =>
    instance.post(url, data, config) as unknown as Promise<any>,
  patch: (url: string, data?: any) => instance.patch(url, data) as unknown as Promise<any>,
  delete: (url: string) => instance.delete(url) as unknown as Promise<any>,
}
