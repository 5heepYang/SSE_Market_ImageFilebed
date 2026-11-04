<script setup lang="ts">
import { onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { oauthExchangeToken, oauthFetchUserInfo } from '../api/auth'
import { setAuth, consumeRedirect, markOAuthPending, clearOAuthPending } from '../store/auth'
import { pushToast } from '../composables/toast'

const route = useRoute()
const router = useRouter()

onMounted(async () => {
  const code = route.query.code as string
  const state = route.query.state as string
  if (!code || !state) {
    pushToast('缺少授权参数，请重新登录', 'error')
    router.replace('/login')
    return
  }

  markOAuthPending()
  try {
    const { session_id } = await oauthExchangeToken(code, state)
    const { token, user } = await oauthFetchUserInfo(session_id)
    setAuth(token, user)
    clearOAuthPending()
    pushToast('登录成功', 'success')
    router.replace(consumeRedirect())
  } catch (e: any) {
    clearOAuthPending()
    pushToast(e.message || '登录失败，请重试', 'error')
    router.replace('/login')
  }
})
</script>

<template>
  <div class="flex min-h-screen items-center justify-center">
    <div class="flex flex-col items-center gap-3 text-slate-500">
      <span class="inline-block h-8 w-8 animate-spin rounded-full border-2 border-indigo-200 border-t-indigo-600" />
      <p class="text-sm">正在登录，请稍候…</p>
    </div>
  </div>
</template>
