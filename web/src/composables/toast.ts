import { reactive } from 'vue'

export interface Toast {
  id: number
  message: string
  type: 'success' | 'error' | 'info'
}

export const toasts = reactive<Toast[]>([])
let seq = 0

export function pushToast(message: string, type: Toast['type'] = 'info') {
  const t: Toast = { id: ++seq, message, type }
  toasts.push(t)
  setTimeout(() => {
    const i = toasts.findIndex((x) => x.id === t.id)
    if (i >= 0) toasts.splice(i, 1)
  }, 2800)
}
