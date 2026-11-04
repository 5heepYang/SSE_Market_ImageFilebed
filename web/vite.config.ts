import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import UnoCSS from 'unocss/vite'

// 部署到 SSE Deploy 平台时，通过构建参数 VITE_BASE_PATH 指定子路径前缀（如 /sdpy/test/imagebed）。
// 本地开发默认为根路径。
const rawBase = process.env.VITE_BASE_PATH || '/'
const base = rawBase.endsWith('/') ? rawBase : rawBase + '/'

export default defineConfig({
  base,
  plugins: [vue(), UnoCSS()],
  server: {
    port: 5173,
    proxy: {
      '/api': { target: 'http://localhost:8080', changeOrigin: true },
      '/i': { target: 'http://localhost:8080', changeOrigin: true },
    },
  },
})
