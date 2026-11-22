<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { login } from '../api/auth'
import { setAuth } from '../store/auth'

const route = useRoute()
const router = useRouter()

const email = ref('')
const password = ref('')
const busy = ref(false)
const error = ref('')

async function submit() {
  if (busy.value)
    return
  error.value = ''
  if (!email.value.trim() || !password.value) {
    error.value = '请输入邮箱和密码'
    return
  }
  busy.value = true
  try {
    const { token, user } = await login(email.value.trim(), password.value)
    setAuth(token, user)
    const target = (route.query.redirect as string) || '/'
    await router.replace(target)
  }
  catch (e: any) {
    error.value = e?.message || '登录失败，请重试'
  }
  finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="mx-auto flex min-h-screen max-w-md flex-col items-center justify-center px-4">
    <div class="mb-8 flex flex-col items-center gap-3">
      <img
        src="https://image.ssemarket.cn/i/images/2026/08/24/694837ed-65b6-400a-86f1-0b5886b2fb78.png"
        alt="图床"
        class="h-14 w-14 object-contain"
      >
      <h1 class="text-xl font-semibold text-slate-800">图床</h1>
      <p class="text-sm text-slate-500">使用 UniSSO 统一身份认证（邮箱 + 密码）登录</p>
    </div>

    <form class="w-full space-y-4" @submit.prevent="submit">
      <div class="space-y-1">
        <label for="login-email" class="block text-sm text-slate-600">邮箱</label>
        <input
          id="login-email"
          v-model="email"
          type="email"
          autocomplete="username"
          placeholder="you@example.com"
          class="w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm text-slate-800 outline-none transition focus:border-indigo-500 focus:ring-2 focus:ring-indigo-100"
        >
      </div>

      <div class="space-y-1">
        <label for="login-password" class="block text-sm text-slate-600">密码</label>
        <input
          id="login-password"
          v-model="password"
          type="password"
          autocomplete="current-password"
          placeholder="密码"
          class="w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm text-slate-800 outline-none transition focus:border-indigo-500 focus:ring-2 focus:ring-indigo-100"
        >
      </div>

      <p v-if="error" class="text-sm text-rose-600">
        {{ error }}
      </p>

      <button
        type="submit"
        :disabled="busy"
        class="w-full rounded-lg bg-indigo-600 px-4 py-2.5 text-sm font-medium text-white transition hover:bg-indigo-700 disabled:cursor-not-allowed disabled:opacity-60"
      >
        {{ busy ? '登录中…' : '登录' }}
      </button>
    </form>

    <p class="mt-6 text-xs text-slate-400">仅限软件工程学院内部账号登录</p>
  </div>
</template>
