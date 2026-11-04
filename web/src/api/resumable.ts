import * as tus from "tus-js-client"
import { auth } from "../store/auth"

const BASE = import.meta.env.VITE_API_BASE || window.location.origin
const RESUMABLE_ENDPOINT = `${BASE.replace(/\/+$/, "")}/api/upload/resumable/`

/** 每个分片请求的超时设置 */
const CONNECT_TIMEOUT_MS = 30_000 // 连接超时：请求发出后 30 秒内没有任何进展
const READ_WRITE_TIMEOUT_MS = 60_000 // 读取/写入超时：连接建立后 60 秒无数据流动（有进展则重置）

export interface ResumableResult {
  url: string
  originalName: string
  mimeType: string
  markdown: string
  html: string
}

/**
 * 为单个分片请求装上「连接超时 + 读/写空闲超时」计时器，超时则中断该请求。
 * - 连接阶段（尚未收到响应头、也没有任何上传进度）超过 30 秒 → 连接超时；
 * - 连接建立后，上行/下行只要还有数据在流动就不断重置 60 秒计时器，
 *   停滞超过 60 秒 → 读/写超时。慢速但正常传输不会被误杀。
 */
function armChunkTimeouts(xhr: XMLHttpRequest, onTimeout: (message: string) => void) {
  let timer: number | undefined
  let connected = false

  const armReadWrite = () => {
    window.clearTimeout(timer)
    timer = window.setTimeout(() => {
      onTimeout("读取/写入超时（60 秒），请检查网络后重试")
      xhr.abort()
    }, READ_WRITE_TIMEOUT_MS)
  }

  // 连接阶段：30 秒内无任何进展则中止
  window.clearTimeout(timer)
  timer = window.setTimeout(() => {
    onTimeout("连接超时（30 秒），请检查网络后重试")
    xhr.abort()
  }, CONNECT_TIMEOUT_MS)

  const markConnected = () => {
    if (!connected) {
      connected = true
    }
    armReadWrite() // 数据有进展：切换/重置 60 秒读/写空闲计时
  }

  // 收到响应头 → 视为已连接
  xhr.addEventListener("readystatechange", () => {
    if (xhr.readyState >= XMLHttpRequest.HEADERS_RECEIVED) {
      markConnected()
    }
  })
  // 下行有数据到达
  xhr.addEventListener("progress", markConnected)
  // 上行（发送分片内容）有数据发出
  xhr.upload?.addEventListener("progress", markConnected)
  // 请求结束（成功/失败/中止）→ 停止计时器
  xhr.addEventListener("loadend", () => window.clearTimeout(timer))
}

/** 创建一个 tus 断点续传上传并返回 Promise。每片 ~1MB。 */
export function resumableUpload(
  file: File,
  onProgress?: (percent: number) => void,
): Promise<ResumableResult> {
  return new Promise((resolve, reject) => {
    let timeoutMessage: string | null = null

    const upload = new tus.Upload(file, {
      endpoint: RESUMABLE_ENDPOINT,
      chunkSize: 1 * 1024 * 1024,
      retryDelays: [0, 1000, 3000, 5000],
      removeFingerprintOnSuccess: true,
      metadata: { filename: file.name },
      headers: { Authorization: `Bearer ${auth.token}` },
      onBeforeRequest(req: any) {
        timeoutMessage = null // 新请求开始，重置超时原因
        const xhr = req.getUnderlyingObject() as XMLHttpRequest
        armChunkTimeouts(xhr, (message) => {
          timeoutMessage = message
        })
      },
      onError(err) {
        reject(new Error(timeoutMessage || err.message || "上传失败"))
      },
      onProgress(bytesSent, bytesTotal) {
        onProgress?.(Math.round((bytesSent / bytesTotal) * 100))
      },
      onSuccess() {
        const lastResp = upload.url ? upload.url : null
        const xhr = (upload as any)._xhr
        const getHeader = (k: string) =>
          xhr?.getResponseHeader?.(k) || ""

        const url = getHeader("X-Object-URL") || lastResp || ""
        const originalName = getHeader("X-Object-Name") || file.name
        const mimeType = getHeader("X-Object-Mime") || file.type

        if (!url) {
          reject(new Error("上传成功但未获取到文件链接"))
          return
        }

        const isImage = mimeType.startsWith("image/")
        const md = isImage
          ? `![${originalName}](${url})`
          : `[${originalName}](${url})`
        const html = isImage
          ? `<img src="${url}" alt="${originalName}" />`
          : `<a href="${url}" target="_blank" rel="noopener">${originalName}</a>`

        resolve({ url, originalName, mimeType, markdown: md, html })
      },
    })
    // 如果之前中断过，自动恢复
    upload.findPreviousUploads().then((uploads) => {
      if (uploads.length > 0) {
        upload.resumeFromPreviousUpload(uploads[0])
      }
      upload.start()
    })
  })
}
