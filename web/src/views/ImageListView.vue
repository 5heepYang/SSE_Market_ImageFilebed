<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from "vue"
import {
  listImages,
  deleteImage,
  updateImage,
  type ImageItem,
} from "../api/image"
import { pushToast } from "../composables/toast"
import { useUploadQueue } from "../composables/useUploadQueue"
import UploadZone from "../components/UploadZone.vue"
import FileCard from "../components/ImageCard.vue"

// 一批上传结束后自动刷新列表，让刚上传的文件立即出现
const { onUploadComplete } = useUploadQueue()
const stopUploadWatch = onUploadComplete(() => {
  if (keyword.value || visibility.value) {
    // 有搜索/可见性过滤时保持当前视图，仅重新加载
    load()
  } else {
    // 默认视图跳回第一页，最新上传的文件排在最前
    page.value = 1
    load()
  }
})
onUnmounted(stopUploadWatch)

const images = ref<ImageItem[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 48
const keyword = ref("")
const visibility = ref("")
const loading = ref(false)

const totalPages = computed(() => Math.max(1, Math.ceil(total.value / pageSize)))

async function load() {
  loading.value = true
  try {
    const res = await listImages({
      page: page.value,
      page_size: pageSize,
      keyword: keyword.value || undefined,
      visibility: visibility.value || undefined,
    })
    images.value = res.list
    total.value = res.total
  } catch (e: any) {
    pushToast(e.message || "加载失败", "error")
  } finally {
    loading.value = false
  }
}

async function onToggle(img: ImageItem) {
  const next = img.visibility === "public" ? "private" : "public"
  try {
    await updateImage(img.id, next)
    img.visibility = next
    pushToast(next === "public" ? "已设为公开" : "已设为私有", "success")
  } catch (e: any) {
    pushToast(e.message || "操作失败", "error")
  }
}

async function onDelete(id: number) {
  try {
    await deleteImage(id)
    pushToast("已删除", "success")
    await load()
  } catch (e: any) {
    pushToast(e.message || "删除失败", "error")
  }
}

function search() {
  page.value = 1
  load()
}

onMounted(load)
</script>

<template>
  <div>
    <UploadZone />

    <!-- 工具栏 -->
    <div class="mt-5 flex flex-wrap items-center justify-between gap-3">
      <div class="flex items-center gap-3">
        <input
          v-model="keyword"
          @keyup.enter="search"
          placeholder="搜索文件名…"
          class="w-56 rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm outline-none transition focus:border-indigo-400 focus:ring-2 focus:ring-indigo-100"
        />
        <select
          v-model="visibility"
          @change="search"
          class="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"
        >
          <option value="">全部</option>
          <option value="public">公开</option>
          <option value="private">私有</option>
        </select>
        <button
          class="rounded-lg bg-indigo-600 px-4 py-2 text-sm text-white transition hover:bg-indigo-700"
          @click="search"
        >
          搜索
        </button>
        <button
          v-if="images.length > 0"
          class="rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm text-slate-600 transition hover:bg-slate-100"
          @click="load()"
        >
          刷新
        </button>
      </div>
      <div class="text-sm text-slate-500">
        共 {{ total }} 个文件
        <span v-if="loading" class="ml-2 text-indigo-500">加载中…</span>
      </div>
    </div>

    <!-- 文件网格 -->
    <div v-if="loading && images.length === 0" class="py-20 text-center text-slate-400">
      加载中…
    </div>

    <div v-else-if="images.length === 0" class="py-20 text-center text-slate-400">
      还没有文件，先上传一个吧
    </div>

    <div v-else class="mt-4 grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
      <FileCard
        v-for="img in images"
        :key="img.id"
        :image="img"
        @toggle="onToggle"
        @delete="onDelete"
      />
    </div>

    <!-- 分页 -->
    <div v-if="totalPages > 1" class="mt-6 flex items-center justify-center gap-4 text-sm">
      <button
        :disabled="page <= 1"
        class="rounded-lg border border-slate-300 px-4 py-2 disabled:opacity-40"
        @click="page--; load()"
      >
        上一页
      </button>
      <span class="text-slate-600">第 {{ page }} / {{ totalPages }} 页</span>
      <button
        :disabled="page >= totalPages"
        class="rounded-lg border border-slate-300 px-4 py-2 disabled:opacity-40"
        @click="page++; load()"
      >
        下一页
      </button>
    </div>
  </div>
</template>
