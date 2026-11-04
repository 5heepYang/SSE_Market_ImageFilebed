import { http } from './http'

export interface ImageItem {
  id: number
  original_name: string
  url: string
  thumb_url: string
  mime_type: string
  size: number
  width: number
  height: number
  visibility: 'public' | 'private'
  created_at: string
}

export interface ImagePage {
  list: ImageItem[]
  total: number
  page: number
  page_size: number
}

export function listImages(params: {
  page: number
  page_size: number
  keyword?: string
  visibility?: string
}) {
  return http.get('/api/images', { params }) as Promise<ImagePage>
}

export function getImage(id: number) {
  return http.get(`/api/images/${id}`) as Promise<ImageItem>
}

export function uploadImages(files: File[]) {
  const fd = new FormData()
  files.forEach((f) => fd.append('file', f))
  return http.post('/api/images', fd) as Promise<ImageItem[]>
}

export function updateImage(id: number, visibility: string) {
  return http.patch(`/api/images/${id}`, { visibility }) as Promise<ImageItem>
}

export function deleteImage(id: number) {
  return http.delete(`/api/images/${id}`) as Promise<null>
}


/** 通过同源 API 获取文件二进制内容（用于 PDF / DOCX / XLSX / PPTX 预览）。 */
export function getImageFile(id: number) {
  return http.get(`/api/images/${id}/file`, { responseType: 'blob' }) as Promise<Blob>
}
