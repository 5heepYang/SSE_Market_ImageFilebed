import { http } from './http'

export interface TokenItem {
  id: number
  name: string
  expires_at: string | null
  last_used_at: string | null
  created_at: string
}

export interface CreatedToken extends TokenItem {
  token: string
}

export function listTokens() {
  return http.get('/api/tokens') as Promise<TokenItem[]>
}

export function createToken(data: { name: string; expires_at: string | null }) {
  return http.post('/api/tokens', data) as Promise<CreatedToken>
}

export function revokeToken(id: number) {
  return http.delete(`/api/tokens/${id}`) as Promise<null>
}
