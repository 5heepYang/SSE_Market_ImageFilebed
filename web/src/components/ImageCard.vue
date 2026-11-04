<script setup lang="ts">
import { computed } from "vue"
import { useRouter } from "vue-router"
import type { ImageItem } from "../api/image"
import { formatBytes, formatTime, copyText } from "../utils/format"
import { pushToast } from "../composables/toast"
import { auth } from "../store/auth"

const props = defineProps<{ image: ImageItem }>()
const emit = defineEmits<{
  (e: "delete", id: number): void
  (e: "toggle", image: ImageItem): void
}>()
const router = useRouter()

function signed(url: string): string {
  if (!url || props.image.visibility !== "private") return url
  const sep = url.includes("?") ? "&" : "?"
  return url + sep + "token=" + encodeURIComponent(auth.token)
}

const displayUrl = computed(() => signed(props.image.url))
const displayThumb = computed(() => signed(props.image.thumb_url || props.image.url))
const isImage = computed(() => (props.image.mime_type || "").startsWith("image/"))
const isPdf = computed(() => props.image.mime_type === "application/pdf")

const friendlyType = computed(() => {
  const m = props.image.mime_type
  if (isImage.value) return "图片"
  if (m === "application/pdf") return "PDF"
  if (m.includes("wordprocessingml") || m.includes("msword")) return "Word"
  if (m.includes("spreadsheetml") || m.includes("ms-excel")) return "Excel"
  if (m.includes("presentationml") || m.includes("ms-powerpoint")) return "PPT"
  return m.split("/")[1] || m
})

const markdown = computed(() =>
  isImage.value
    ? "![" + props.image.original_name + "](" + displayUrl.value + ")"
    : "[" + props.image.original_name + "](" + displayUrl.value + ")"
)

const html = computed(() =>
  isImage.value
    ? "<img src=\"" + displayUrl.value + "\" alt=\"" + props.image.original_name + "\" />"
    : "<a href=\"" + displayUrl.value + "\" target=\"_blank\" rel=\"noopener\">" + props.image.original_name + "</a>"
)

async function download() {
  try {
    const resp = await fetch(displayUrl.value)
    if (!resp.ok) throw new Error('下载失败')
    const blob = await resp.blob()
    const a = document.createElement('a')
    a.href = URL.createObjectURL(blob)
    a.download = props.image.original_name
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(a.href)
    pushToast('已开始下载', 'success')
  } catch (e: any) {
    pushToast(e.message || '下载失败', 'error')
  }
}

async function doCopy(text: string, label: string) {
  const ok = await copyText(text)
  pushToast(ok ? "已复制 " + label : "复制失败", ok ? "success" : "error")
}

function confirmDelete() {
  if (window.confirm("确定删除此文件？此操作不可恢复。")) {
    emit("delete", props.image.id)
  }
}

function openPreview() {
  if (isImage.value || isPdf.value) {
    window.open(displayUrl.value, "_blank", "noopener,noreferrer")
    return
  }
  const href = router.resolve({ name: "preview", params: { id: props.image.id } }).href
  window.open(href, "_blank", "noopener,noreferrer")
}
</script>

<template>
  <div
    class="group relative flex cursor-pointer flex-col overflow-hidden rounded-xl border border-slate-200 bg-white transition hover:shadow-md hover:border-indigo-200"
    @click="openPreview"
  >
    <div class="relative flex aspect-square items-center justify-center overflow-hidden bg-slate-50">
      <img
        v-if="isImage"
        :src="displayThumb"
        :alt="image.original_name"
        loading="lazy"
        class="h-full w-full object-cover transition duration-300 group-hover:scale-105"
      />
      <div v-else class="flex flex-col items-center gap-1">
        <span class="text-5xl text-slate-300">📄</span>
        <span class="text-xs font-medium text-slate-400">{{ friendlyType }}</span>
      </div>
      <span
        class="absolute left-1.5 top-1.5 rounded px-1.5 py-0.5 text-[10px] font-medium leading-none shadow-sm"
        :class="image.visibility === 'public' ? 'bg-emerald-500/80 text-white' : 'bg-amber-500/80 text-white'"
        @click.stop="emit('toggle', image)"
      >
        {{ image.visibility === "public" ? "公开" : "私有" }}
      </span>
    </div>
    <div class="flex flex-col gap-1 p-2.5">
      <div class="truncate text-sm font-medium text-slate-800" :title="image.original_name">
        {{ image.original_name }}
      </div>
      <div class="flex items-center justify-between text-[11px] text-slate-400">
        <span>{{ friendlyType }} &middot; {{ formatBytes(image.size) }}</span>
        <span>{{ formatTime(image.created_at) }}</span>
      </div>
    </div>
    <div class="flex flex-wrap items-center justify-center gap-1 border-t border-slate-100 bg-slate-50/50 px-2 py-1.5">
      <button
        class="rounded px-1.5 py-0.5 text-[11px] text-slate-500 transition hover:bg-indigo-50 hover:text-indigo-600"
        title="复制链接"
        @click.stop="doCopy(displayUrl, '链接')"
      >复制链接</button>
      <button
        class="rounded px-1.5 py-0.5 text-[11px] text-slate-500 transition hover:bg-indigo-50 hover:text-indigo-600"
        title="复制 Markdown"
        @click.stop="doCopy(markdown, 'Markdown')"
      >复制Markdown</button>
      <button
        class="rounded px-1.5 py-0.5 text-[11px] text-slate-500 transition hover:bg-indigo-50 hover:text-indigo-600"
        title="复制 HTML"
        @click.stop="doCopy(html, 'HTML')"
      >复制HTML</button>
      <button
        class="rounded px-1.5 py-0.5 text-[11px] transition"
        :class="image.visibility === 'public' ? 'text-amber-500 hover:bg-amber-50 hover:text-amber-600' : 'text-emerald-500 hover:bg-emerald-50 hover:text-emerald-600'"
        :title="image.visibility === 'public' ? '设为私有' : '设为公开'"
        @click.stop="emit('toggle', image)"
      >{{ image.visibility === "public" ? "设为私有" : "设为公开" }}</button>
      <button
        class="rounded px-2 py-0.5 text-xs text-slate-500 transition hover:bg-indigo-50 hover:text-indigo-600"
        title="下载"
        @click.stop="download"
      >下载</button>
      <button
        class="rounded px-2 py-0.5 text-xs text-rose-400 transition hover:bg-rose-50 hover:text-rose-600"
        title="删除"
        @click.stop="confirmDelete"
      >删除</button>
    </div>
  </div>
</template>
