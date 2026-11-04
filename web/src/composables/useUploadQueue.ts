import { ref, computed } from "vue"
import { resumableUpload, type ResumableResult } from "../api/resumable"
import { uploadImages } from "../api/image"

export interface UploadEntry {
  id: string
  file: File
  progress: number
  status: "pending" | "uploading" | "done" | "error"
  result?: ResumableResult
  error?: string
}

const entries = ref<UploadEntry[]>([])
const activeCount = ref(0)

/** 一批文件全部上传结束（无论成败）后触发的回调，用于通知列表页自动刷新 */
const completeListeners = new Set<() => void>()

let seq = 0
function genId(file: File): string {
  return `${file.name}-${Date.now()}-${seq++}`
}

export function useUploadQueue() {
  const isUploading = computed(() => activeCount.value > 0)
  const allDone = computed(() =>
    entries.value.length > 0 && entries.value.every((e) => e.status === "done" || e.status === "error"),
  )
  const doneEntries = computed(() => entries.value.filter((e) => e.status === "done"))
  const activeEntries = computed(() =>
    entries.value.filter((e) => e.status === "pending" || e.status === "uploading"),
  )

  /** 注册"一批上传结束"回调，返回取消注册的函数 */
  function onUploadComplete(fn: () => void): () => void {
    completeListeners.add(fn)
    return () => completeListeners.delete(fn)
  }

  /** 把一批文件加入上传队列。小图片走普通上传，大文件/文档走断点续传。 */
  async function enqueue(files: File[]) {
    if (files.length === 0) return

    const small: File[] = []
    const large: File[] = []

    for (const f of files) {
      const isSmallImage =
        f.size <= 20 * 1024 * 1024 && /\.(png|jpe?g|gif|webp|bmp|svg)$/i.test(f.name)
      if (isSmallImage) {
        small.push(f)
      } else {
        large.push(f)
      }
    }

    // 全部先入列，进度面板立即展示，避免小图上传期间界面空白
    const pushEntry = (f: File): UploadEntry => {
      entries.value.push({ id: genId(f), file: f, progress: 0, status: "pending" })
      return entries.value[entries.value.length - 1]
    }
    const smallEntries = small.map(pushEntry)
    const largeEntries = large.map(pushEntry)

    try {
      // 上传小图片
      if (smallEntries.length > 0) {
        activeCount.value++
        for (const e of smallEntries) e.status = "uploading"
        try {
          const results = await uploadImages(small)
          for (let i = 0; i < results.length; i++) {
            const r = results[i]
            const isImg = (r.mime_type || "").startsWith("image/")
            const md = isImg
              ? `![${r.original_name}](${r.url})`
              : `[${r.original_name}](${r.url})`
            const html = isImg
              ? `<img src="${r.url}" alt="${r.original_name}" />`
              : `<a href="${r.url}" target="_blank" rel="noopener">${r.original_name}</a>`
            const e = smallEntries[i]
            e.status = "done"
            e.progress = 100
            e.result = { url: r.url, originalName: r.original_name, mimeType: r.mime_type || "", markdown: md, html }
          }
          // 服务端跳过了部分文件（类型/大小/空文件校验未通过）
          for (let i = results.length; i < smallEntries.length; i++) {
            smallEntries[i].status = "error"
            smallEntries[i].error = "上传失败（文件类型、大小或内容不符合要求）"
          }
        } catch (e: any) {
          for (const entry of smallEntries) {
            entry.status = "error"
            entry.error = e.message || "上传失败"
          }
        } finally {
          activeCount.value--
        }
      }

      // 大文件/文档走断点续传
      for (const entry of largeEntries) {
        activeCount.value++
        entry.status = "uploading"
        try {
          const result = await resumableUpload(entry.file, (pct) => {
            entry.progress = pct
          })
          entry.status = "done"
          entry.progress = 100
          entry.result = result
        } catch (err: any) {
          entry.status = "error"
          entry.error = err.message || "上传失败"
        } finally {
          activeCount.value--
        }
      }
    } finally {
      // 一批全部结束：通知列表页自动刷新
      completeListeners.forEach((fn) => fn())
    }
  }

  function clear() {
    entries.value = []
  }

  function removeEntry(id: string) {
    const i = entries.value.findIndex((e) => e.id === id)
    if (i >= 0) entries.value.splice(i, 1)
  }

  return { entries, isUploading, allDone, doneEntries, activeEntries, enqueue, clear, removeEntry, onUploadComplete }
}
