<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { logout as apiLogout } from './api/auth'
import { auth, clearAuth } from './store/auth'
import Toast from './components/Toast.vue'

const route = useRoute()
const router = useRouter()

const logoUrl =
  'https://image.ssemarket.cn/i/images/2026/08/24/694837ed-65b6-400a-86f1-0b5886b2fb78.png'
const showNav = computed(() => route.name !== 'login')

// 单点登出：先通知后端撤销 UniSSO 共享会话并清父域 Cookie，再清本地 JWT。
// 后端失败不阻塞本地登出（本地 JWT 无服务端状态，删掉即失效）。
async function logout() {
  try {
    await apiLogout()
  }
  catch { /* 忽略：网络或会话已失效 */ }
  clearAuth()
  router.push('/login')
}
</script>

<template>
  <div class="min-h-screen bg-slate-50">
    <Toast />

    <header
      v-if="showNav"
      class="sticky top-0 z-40 border-b border-slate-200 bg-white/80 backdrop-blur"
    >
      <div class="mx-auto flex h-14 max-w-6xl items-center gap-6 px-4">
        <router-link to="/" class="flex items-center gap-2 text-lg font-semibold text-slate-800">
          <img :src="logoUrl" alt="图床" class="h-6 w-6 object-contain" />
          <span>图床</span>
        </router-link>

        <nav class="flex items-center gap-1 text-sm">
          <router-link
            to="/"
            class="rounded-lg px-3 py-1.5 transition"
            :class="
              route.name === 'images'
                ? 'bg-indigo-50 font-medium text-indigo-600'
                : 'text-slate-600 hover:bg-slate-100'
            "
          >
            图片
          </router-link>
          <router-link
            to="/tokens"
            class="rounded-lg px-3 py-1.5 transition"
            :class="
              route.name === 'tokens'
                ? 'bg-indigo-50 font-medium text-indigo-600'
                : 'text-slate-600 hover:bg-slate-100'
            "
          >
            Token
          </router-link>
        </nav>

        <div class="ml-auto flex items-center gap-3">
          <span class="text-sm text-slate-600">{{ auth.user?.name }}</span>
          <button
            class="rounded-lg border border-slate-200 px-3 py-1.5 text-sm text-slate-600 transition hover:bg-slate-100"
            @click="logout"
          >
            退出
          </button>
        </div>
      </div>
    </header>

    <main class="mx-auto max-w-6xl px-4 py-6">
      <router-view />
    </main>
  </div>
</template>
