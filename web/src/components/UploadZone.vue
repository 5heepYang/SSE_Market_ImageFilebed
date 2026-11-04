<script setup lang="ts">
import { ref, onUnmounted } from "vue"
import { useUploadQueue } from "../composables/useUploadQueue"
import { pushToast } from "../composables/toast"
import { copyText } from "../utils/format"

const {
  entries,
  isUploading,
  doneEntries,
  activeEntries,
  enqueue,
  clear,
  onUploadComplete,
} = useUploadQueue()

const inputRef = ref<HTMLInputElement>()
const dragging = ref(false)
const showResults = ref(false)

// 一批上传结束后展示结果面板；上传区（拖拽框）此时已自动恢复初始状态
const stopComplete = onUploadComplete(() => {
  showResults.value = true
})
onUnmounted(stopComplete)

function onPick() {
  inputRef.value?.click()
}

function onInput(e: Event) {
  const target = e.target as HTMLInputElement
  if (target.files && target.files.length) {
    handleFiles(Array.from(target.files))
    target.value = ""
  }
}

function onDrop(e: DragEvent) {
  dragging.value = false
  if (e.dataTransfer?.files?.length) {
    handleFiles(Array.from(e.dataTransfer.files))
  }
}

function handleFiles(files: File[]) {
  // 禁止空文件
  const nonEmpty = files.filter((f) => f.size > 0)
  const emptyCount = files.length - nonEmpty.length
  if (emptyCount > 0) {
    pushToast(
      emptyCount === files.length
        ? "不能上传空文件"
        : `已跳过 ${emptyCount} 个空文件`,
      "error",
    )
  }
  if (nonEmpty.length === 0) return

  const valid = nonEmpty.filter(
    (f) =>
      f.type.startsWith("image/") ||
      /\.(png|jpe?g|gif|webp|bmp|svg|pdf|docx?|xlsx?|pptx?)$/i.test(f.name),
  )
  if (valid.length === 0) {
    pushToast("请选择图片或文档文件（pdf/doc/docx/xls/xlsx/ppt/pptx）", "error")
    return
  }
  if (valid.length > 10) {
    pushToast("一次最多上传 10 个文件", "error")
    return
  }
  showResults.value = false
  enqueue(valid)
}

async function copyAll() {
  const lines = doneEntries.value.map((e) => e.result?.markdown || "")
  if (lines.length === 0) return
  const ok = await copyText(lines.join("\n\n"))
  if (ok) pushToast("已复制所有 Markdown 链接", "success")
  else pushToast("复制失败", "error")
}

async function copyOne(text: string) {
  const ok = await copyText(text)
  if (ok) pushToast("已复制到剪贴板", "success")
  else pushToast("复制失败", "error")
}

function resetAll() {
  clear()
  showResults.value = false
}
</script>

<template>
  <!-- 拖拽/选择区：无进行中的上传任务时始终显示（上传结束后自动恢复原状） -->
  <div v-if="activeEntries.length === 0">
    <div
      class="flex cursor-pointer flex-col items-center justify-center gap-2 rounded-2xl border-2 border-dashed border-slate-300 bg-white py-10 transition"
      :class="dragging ? 'border-indigo-500 bg-indigo-50' : 'hover:border-indigo-400 hover:bg-indigo-50/40'"
      @click="onPick"
      @dragover.prevent="dragging = true"
      @dragleave.prevent="dragging = false"
      @drop.prevent="onDrop"
    >
      <div class="text-3xl">📤</div>
      <div class="text-sm font-medium text-slate-700">点击或拖拽文件到此处上传</div>
      <div class="text-xs text-slate-400">
        图片自动压缩上传 · 文档走断点续传 · 单文件 ≤ 150MB
      </div>
      <div class="text-xs text-slate-400">
        支持 png/jpg/gif/webp/bmp/svg · pdf/doc/docx/xls/xlsx/ppt/pptx
      </div>
      <input
        ref="inputRef"
        type="file"
        accept="image/*,.pdf,.doc,.docx,.xls,.xlsx,.ppt,.pptx"
        multiple
        class="hidden"
        @change="onInput"
      />
    </div>
  </div>

  <!-- 上传进度 -->
  <div v-if="activeEntries.length > 0" class="space-y-2">
    <div class="flex items-center justify-between">
      <span class="text-sm font-medium text-slate-700">上传中…</span>
      <span class="text-xs text-slate-400">
        {{ entries.filter((e) => e.status === "done").length }}/{{ entries.length }}
      </span>
    </div>
    <div
      v-for="entry in activeEntries"
      :key="entry.id"
      class="overflow-hidden rounded-xl border border-slate-200 bg-white"
    >
      <div class="flex items-center gap-3 px-3 py-2">
        <div class="shrink-0 text-lg">
          {{ entry.status === "done" ? "✅" : entry.status === "error" ? "❌" : "📄" }}
        </div>
        <div class="min-w-0 flex-1">
          <div class="truncate text-sm text-slate-800">{{ entry.file.name }}</div>
          <div v-if="entry.status === 'error'" class="mt-0.5 text-xs text-rose-500">
            {{ entry.error }}
          </div>
          <div
            v-if="entry.status === 'uploading'"
            class="mt-1 h-1.5 w-full overflow-hidden rounded-full bg-slate-100"
          >
            <div
              class="h-full rounded-full bg-indigo-500 transition-all duration-300"
              :style="{ width: entry.progress + '%' }"
            />
          </div>
        </div>
        <div
          v-if="entry.status === 'uploading'"
          class="shrink-0 text-xs font-medium text-indigo-600"
        >
          {{ entry.progress }}%
        </div>
      </div>
    </div>
  </div>

  <!-- 上传完成 → 展示结果（上传区已恢复，此处仅为复制链接保留） -->
  <div v-if="showResults && doneEntries.length > 0" class="mt-4 space-y-3">
    <div class="flex items-center justify-between">
      <span class="text-sm font-medium text-slate-700">
        上传完成（{{ doneEntries.length }} 个文件）
      </span>
      <div class="flex gap-2">
        <button
          class="rounded-lg bg-indigo-600 px-3 py-1.5 text-xs text-white transition hover:bg-indigo-700"
          @click="copyAll"
        >
          复制全部 Markdown
        </button>
        <button
          class="rounded-lg border border-slate-200 px-3 py-1.5 text-xs text-slate-600 transition hover:bg-slate-100"
          @click="resetAll"
        >
          收起
        </button>
      </div>
    </div>

    <div v-for="entry in doneEntries" :key="entry.id" class="overflow-hidden rounded-xl border border-slate-200 bg-white">
      <div class="flex items-start gap-3 px-3 py-2.5">
        <div class="shrink-0 pt-0.5 text-lg">
          {{ entry.result?.mimeType?.startsWith("image/") ? "🖼️" : "📄" }}
        </div>
        <div class="min-w-0 flex-1 space-y-1.5">
          <div class="truncate text-sm font-medium text-slate-800">
            {{ entry.result?.originalName || entry.file.name }}
          </div>
          <!-- Markdown 预览 -->
          <div
            class="cursor-pointer rounded-md bg-slate-50 px-2.5 py-1.5 font-mono text-xs text-slate-600 transition hover:bg-indigo-50"
            title="点击复制"
            @click="copyOne(entry.result?.markdown || '')"
          >
            {{ entry.result?.markdown }}
          </div>
          <!-- 操作按钮 -->
          <div class="flex gap-2 text-xs">
            <button
              class="rounded-md bg-indigo-50 px-2.5 py-1 text-indigo-600 transition hover:bg-indigo-100"
              @click="copyOne(entry.result?.markdown || '')"
            >
              复制 Markdown
            </button>
            <button
              class="rounded-md bg-indigo-50 px-2.5 py-1 text-indigo-600 transition hover:bg-indigo-100"
              @click="copyOne(entry.result?.html || '')"
            >
              复制 HTML
            </button>
            <button
              class="rounded-md bg-indigo-50 px-2.5 py-1 text-indigo-600 transition hover:bg-indigo-100"
              @click="copyOne(entry.result?.url || '')"
            >
              复制链接
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>

  <!-- 失败明细（部分成功时不显示整体失败面板，改在此列出失败文件） -->
  <div
    v-if="showResults && doneEntries.length > 0 && entries.some((e) => e.status === 'error')"
    class="mt-3 space-y-1 rounded-xl border border-rose-200 bg-rose-50 px-3 py-2"
  >
    <div
      v-for="entry in entries.filter((e) => e.status === 'error')"
      :key="entry.id"
      class="flex items-center gap-2 text-xs text-rose-600"
    >
      <span>❌</span>
      <span class="truncate font-medium">{{ entry.file.name }}</span>
      <span class="truncate text-rose-400">{{ entry.error }}</span>
    </div>
  </div>

  <!-- 全部失败 -->
  <div
    v-if="showResults && doneEntries.length === 0 && entries.length > 0"
    class="mt-4 flex flex-col items-center gap-3 rounded-2xl border-2 border-dashed border-rose-200 bg-rose-50 py-8"
  >
    <div class="text-3xl">❌</div>
    <div class="text-sm font-medium text-rose-600">全部文件上传失败</div>
    <button
      class="rounded-lg border border-rose-200 px-4 py-1.5 text-xs text-rose-600 transition hover:bg-rose-100"
      @click="resetAll"
    >
      重新选择
    </button>
  </div>
</template>
