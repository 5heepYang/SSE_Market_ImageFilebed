<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { redirectToOAuth } from '../api/auth'

const route = useRoute()
const failed = ref(false)

async function start() {
  failed.value = false
  const ok = await redirectToOAuth((route.query.redirect as string) || '/')
  failed.value = !ok
}

// 唯一登录方式：进入登录页即自动跳转 OAuth，无需用户点击。
onMounted(start)
</script>

<template>
  <div class="flex min-h-screen items-center justify-center px-4">
    <div class="flex flex-col items-center gap-3 text-slate-500">
      <template v-if="!failed">
        <span class="inline-block h-8 w-8 animate-spin rounded-full border-2 border-indigo-200 border-t-indigo-600" />
        <p class="text-sm">正在跳转登录…</p>
      </template>
      <template v-else>
        <p class="text-sm">登录跳转失败，请确认服务端 OAuth 已配置</p>
        <button
          type="button"
          class="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white transition hover:bg-indigo-700"
          @click="start"
        >
          重新尝试
        </button>
      </template>
    </div>
  </div>
</template>
