<script setup lang="ts">
import { computed, nextTick, onMounted, ref } from "vue"
import { useRoute, useRouter } from "vue-router"
import { getImage, getImageFile, type ImageItem } from "../api/image"
import { auth } from "../store/auth"
import { formatBytes } from "../utils/format"

type PreviewKind = "image" | "pdf" | "docx" | "xlsx" | "pptx" | "office" | "other"

const route = useRoute()
const router = useRouter()

const loading = ref(true)
const rendering = ref(false)
const error = ref("")
const image = ref<ImageItem | null>(null)
const previewEl = ref<HTMLElement | null>(null)

const id = Number(route.params.id)

function signed(url: string): string {
  if (!url || image.value?.visibility !== "private") return url
  const sep = url.includes("?") ? "&" : "?"
  return url + sep + "token=" + encodeURIComponent(auth.token)
}

const typeKind = computed<PreviewKind>(() => {
  const mime = image.value?.mime_type || ""
  if (mime.startsWith("image/")) return "image"
  if (mime === "application/pdf") return "pdf"
  if (mime === "application/vnd.openxmlformats-officedocument.wordprocessingml.document") return "docx"
  if (mime === "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet") return "xlsx"
  if (mime === "application/vnd.openxmlformats-officedocument.presentationml.presentation") return "pptx"
  if (
    mime === "application/msword" ||
    mime === "application/vnd.ms-excel" ||
    mime === "application/vnd.ms-powerpoint"
  ) {
    return "office"
  }
  return "other"
})

const needsRenderer = computed(() => ["docx", "xlsx", "pptx"].includes(typeKind.value))

const fileUrl = computed(() => (image.value ? signed(image.value.url) : ""))

const absoluteFileUrl = computed(() => {
  const url = fileUrl.value
  if (!url) return ""
  if (/^https?:\/\//i.test(url)) return url
  return new URL(url, window.location.origin).href
})

const officePreviewUrl = computed(() => {
  if (!absoluteFileUrl.value || image.value?.visibility !== "public") return ""
  return "https://view.officeapps.live.com/op/embed.aspx?src=" + encodeURIComponent(absoluteFileUrl.value)
})

const typeLabel = computed(() => {
  switch (typeKind.value) {
    case "image":
      return "图片"
    case "pdf":
      return "PDF"
    case "docx":
      return "Word"
    case "xlsx":
      return "Excel"
    case "pptx":
    case "office":
      return "PPT"
    default:
      return "文件"
  }
})

async function renderDocx() {
  if (!previewEl.value) return
  const blob = await getImageFile(image.value!.id)
  const docx = await import("docx-preview")
  previewEl.value.innerHTML = ""
  await docx.renderAsync(blob, previewEl.value, undefined, {
    className: "docx-preview",
    inWrapper: true,
    ignoreWidth: false,
    ignoreHeight: false,
    ignoreFonts: false,
    breakPages: false,
  })
}

async function renderXlsx() {
  if (!previewEl.value) return
  const blob = await getImageFile(image.value!.id)
  const XLSX = await import("xlsx")
  const buffer = await blob.arrayBuffer()
  const workbook = XLSX.read(buffer, { type: "array" })
  const firstSheet = workbook.SheetNames[0]
  if (!firstSheet) {
    throw new Error("工作表为空")
  }
  const html = XLSX.utils.sheet_to_html(workbook.Sheets[firstSheet], { id: "xlsx-preview" })
  previewEl.value.innerHTML = html
}

async function renderPptx() {
  if (!previewEl.value) return
  const blob = await getImageFile(image.value!.id)
  const { init } = await import("pptx-preview")
  previewEl.value.innerHTML = ""
  const viewer = init(previewEl.value, { width: 960, height: 540 })
  await viewer.preview(await blob.arrayBuffer())
}

async function load() {
  loading.value = true
  error.value = ""
  try {
    image.value = await getImage(id)
    if (!image.value) throw new Error("文件不存在")

    await nextTick()

    if (typeKind.value === "pdf") {
      if (!fileUrl.value) throw new Error("文件地址为空")
      window.location.href = fileUrl.value
      return
    }

    if (typeKind.value === "image" || typeKind.value === "office") {
      return
    }
    if (typeKind.value === "docx") {
      loading.value = false
      rendering.value = true
      await nextTick()
      await renderDocx()
    } else if (typeKind.value === "xlsx") {
      loading.value = false
      rendering.value = true
      await nextTick()
      await renderXlsx()
    } else if (typeKind.value === "pptx") {
      loading.value = false
      rendering.value = true
      await nextTick()
      await renderPptx()
    } else {
      error.value = "暂不支持预览此文件类型"
    }
  } catch (e: any) {
    error.value = e?.message || "预览加载失败"
  } finally {
    loading.value = false
    rendering.value = false
  }
}

function goList() {
  router.push({ name: "images" })
}

onMounted(async () => {
  await nextTick()
  await load()
})
</script>

<template>
  <div>
    <div class="mb-4 flex items-start justify-between gap-3">
      <div class="min-w-0">
        <h1 v-if="image" class="truncate text-lg font-semibold text-slate-800">
          {{ image.original_name }}
        </h1>
        <p v-if="image" class="mt-1 text-sm text-slate-400">
          {{ typeLabel }} &middot; {{ formatBytes(image.size) }}
        </p>
      </div>
      <div class="flex shrink-0 items-center gap-2">
        <a
          v-if="fileUrl"
          :href="fileUrl"
          target="_blank"
          rel="noopener"
          class="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm text-slate-600 transition hover:bg-slate-100"
        >
          下载原文件
        </a>
        <button
          class="rounded-lg bg-indigo-600 px-4 py-2 text-sm text-white transition hover:bg-indigo-700"
          @click="goList"
        >
          返回列表
        </button>
      </div>
    </div>

    <div v-if="loading" class="flex items-center justify-center py-24 text-slate-400">
      <span
        class="mr-2 inline-block h-5 w-5 animate-spin rounded-full border-2 border-indigo-200 border-t-indigo-600"
      />
      正在加载预览…
    </div>

    <div v-else-if="error" class="flex flex-col items-center gap-3 py-20 text-slate-400">
      <span class="text-4xl">😕</span>
      <p class="text-sm">{{ error }}</p>
      <a
        v-if="fileUrl"
        :href="fileUrl"
        target="_blank"
        rel="noopener"
        class="rounded-lg bg-indigo-600 px-4 py-2 text-sm text-white transition hover:bg-indigo-700"
      >
        下载原文件
      </a>
    </div>

    <template v-else>
      <div v-if="rendering" class="mb-3 text-center text-sm text-slate-400">
        <span
          class="mr-2 inline-block h-4 w-4 animate-spin rounded-full border-2 border-indigo-200 border-t-indigo-600 align-middle"
        />
        正在渲染文件…
      </div>

      <img
        v-if="typeKind === 'image'"
        :src="fileUrl"
        :alt="image?.original_name"
        class="mx-auto max-h-[78vh] max-w-full rounded-xl border border-slate-200 bg-white object-contain"
      />

      <div
        v-else-if="typeKind === 'pdf'"
        class="flex items-center justify-center py-24 text-slate-400"
      >
        正在打开 PDF…
      </div>

      <iframe
        v-else-if="typeKind === 'office' && officePreviewUrl"
        :src="officePreviewUrl"
        class="h-[78vh] w-full rounded-xl border border-slate-200 bg-white"
      />

      <div
        v-else-if="typeKind === 'office' && !officePreviewUrl"
        class="flex flex-col items-center gap-3 py-20 text-slate-400"
      >
        <span class="text-4xl">📊</span>
        <p class="text-sm">旧版 Office 文件暂不支持在线预览，请下载后使用 Office 打开。</p>
        <a
          :href="fileUrl"
          target="_blank"
          rel="noopener"
          class="rounded-lg bg-indigo-600 px-4 py-2 text-sm text-white transition hover:bg-indigo-700"
        >
          下载原文件
        </a>
      </div>

      <div
        v-show="needsRenderer"
        ref="previewEl"
        class="preview-renderer overflow-auto rounded-xl border border-slate-200 bg-white p-5"
      />
    </template>
  </div>
</template>

<style scoped>
.preview-renderer :deep(.docx-preview) {
  max-width: 100%;
}

.preview-renderer :deep(.docx-preview table),
.preview-renderer :deep(#xlsx-preview) {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}

.preview-renderer :deep(.docx-preview td),
.preview-renderer :deep(.docx-preview th),
.preview-renderer :deep(#xlsx-preview td),
.preview-renderer :deep(#xlsx-preview th) {
  border: 1px solid #d1d5db;
  padding: 4px 8px;
}

.preview-renderer :deep(#xlsx-preview th) {
  background: #f1f5f9;
  font-weight: 600;
}
</style>
