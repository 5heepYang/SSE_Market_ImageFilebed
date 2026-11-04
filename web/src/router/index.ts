import { createRouter, createWebHistory } from 'vue-router'
import { isLoggedIn, isOAuthPending } from '../store/auth'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/login',
      name: 'login',
      component: () => import('../views/LoginView.vue'),
    },
    {
      path: '/oauth/callback',
      name: 'oauth-callback',
      component: () => import('../views/OAuthCallbackView.vue'),
    },
    {
      path: '/',
      name: 'images',
      component: () => import('../views/ImageListView.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/tokens',
      name: 'tokens',
      component: () => import('../views/TokenView.vue'),
      meta: { requiresAuth: true },
    },
    {
      path: '/preview/:id',
      name: 'preview',
      component: () => import('../views/PreviewView.vue'),
      meta: { requiresAuth: true },
    },
  ],
})

router.beforeEach((to) => {
  // 任意路由携带 code+state 时统一转到回调页（防止 redirect_uri 配到根路径时丢失）
  if (to.query.code && to.query.state && to.name !== 'oauth-callback') {
    return { name: 'oauth-callback', query: { code: to.query.code, state: to.query.state } }
  }
  // 回调处理期间不抢跑跳登录
  if (isOAuthPending()) return true
  if (to.meta.requiresAuth && !isLoggedIn()) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  if (to.name === 'login' && isLoggedIn()) {
    return { name: 'images' }
  }
})

export default router
