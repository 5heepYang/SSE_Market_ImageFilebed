import { createApp } from 'vue'
import 'uno.css'
import './styles/global.css'
import App from './App.vue'
import router from './router'
import { trySilentSSO } from './api/auth'
import { setAuth } from './store/auth'

const TOKEN_KEY = 'imagebed_token'
/** 匿名用户等待静默 SSO 的上限（毫秒）；超时先放行，避免一直白屏 */
const SSO_WAIT_MS = 4000

/**
 * 启动前的登录态引导（与线上生产行为一致）：
 *
 * - 本地已有 token（7 天有效）→ 立刻启动，SSO 会话在后台静默轮换，零闪烁；
 * - 本地无 token → 等静默 SSO 最多 4 秒，命中则直接以登录态启动，
 *   避免「先渲染匿名态、再刷新变登录态」的连环闪烁。
 *
 * 历史背景：这段逻辑原先由构建后手工注入 dist/index.html 的引导脚本承担
 * （含 2026-11-18 的防闪烁修复），源码里从未存在。此处回收进源码，
 * 使「从源码重新构建」不再丢失 SSO 无感登录与防闪烁行为。
 */
async function bootstrapAuth(): Promise<void> {
  if (localStorage.getItem(TOKEN_KEY)) {
    trySilentSSO()
      .then((r) => {
        if (r?.token)
          setAuth(r.token, r.user)
      })
      .catch(() => { /* 静默失败：沿用本地旧 token */ })
    return
  }

  const ssoPromise = trySilentSSO()
  const raced = await Promise.race([
    ssoPromise,
    new Promise<null>(resolve => setTimeout(() => resolve(null), SSO_WAIT_MS)),
  ])

  if (raced?.token) {
    setAuth(raced.token, raced.user)
    return
  }

  // 超时先放行渲染；SSO 结果晚到且命中时，热更新登录态并跳回原目标，
  // 无需整页刷新（慢链路下 SSO 常见超过 4 秒，旧版在此处会 reload 闪一下）。
  ssoPromise.then((late) => {
    if (!late?.token)
      return
    setAuth(late.token, late.user)
    const current = router.currentRoute.value
    if (current.name === 'login') {
      const target = (current.query.redirect as string) || '/'
      router.replace(target).catch(() => { /* 路由跳转失败不阻塞 */ })
    }
  }).catch(() => { /* 静默失败 */ })
}

bootstrapAuth().finally(() => {
  createApp(App).use(router).mount('#app')
})
