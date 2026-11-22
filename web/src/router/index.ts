import { createRouter, createWebHistory } from 'vue-router'
import { isLoggedIn } from '../store/auth'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/login',
      name: 'login',
      component: () => import('../views/LoginView.vue'),
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

// 登录态由 main.ts 的启动引导决定（静默 SSO / 本地 token），
// 因此这里只需做最基础的路由守卫，不再有 OAuth 回调与 pending 状态。
router.beforeEach((to) => {
  if (to.meta.requiresAuth && !isLoggedIn())
    return { name: 'login', query: { redirect: to.fullPath } }
  if (to.name === 'login' && isLoggedIn())
    return { name: 'images' }
})

export default router
