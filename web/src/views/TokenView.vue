<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { listTokens, createToken, revokeToken, type TokenItem, type CreatedToken } from '../api/token'
import { pushToast } from '../composables/toast'
import { formatTime, copyText } from '../utils/format'

const tokens = ref<TokenItem[]>([])
const loading = ref(false)
const name = ref('')
const expiry = ref('never')
const creating = ref(false)
const createdToken = ref<CreatedToken | null>(null)
const copied = ref(false)

const expiryOptions = [
  { value: 'never', label: '永不过期' },
  { value: 'hour', label: '1 小时' },
  { value: 'day', label: '1 天' },
  { value: 'week', label: '7 天' },
  { value: 'month', label: '30 天' },
]

function expiryValue(): string | null {
  const now = Date.now()
  const add = (ms: number) => new Date(now + ms).toISOString()
  switch (expiry.value) {
    case 'hour':
      return add(3_600_000)
    case 'day':
      return add(86_400_000)
    case 'week':
      return add(7 * 86_400_000)
    case 'month':
      return add(30 * 86_400_000)
    default:
      return null
  }
}

async function load() {
  loading.value = true
  try {
    tokens.value = await listTokens()
  } catch (e: any) {
    pushToast(e.message || '加载失败', 'error')
  } finally {
    loading.value = false
  }
}

async function onCreate() {
  if (!name.value.trim()) {
    pushToast('请输入 Token 名称', 'error')
    return
  }
  creating.value = true
  try {
    createdToken.value = await createToken({
      name: name.value.trim(),
      expires_at: expiryValue(),
    })
    copied.value = false
    name.value = ''
    await load()
  } catch (e: any) {
    pushToast(e.message || '创建失败', 'error')
  } finally {
    creating.value = false
  }
}

async function copyToken() {
  if (!createdToken.value) return
  if (await copyText(createdToken.value.token)) {
    copied.value = true
    pushToast('已复制 Token', 'success')
  }
}

async function onRevoke(id: number) {
  if (!window.confirm('确定吊销该 Token 吗？使用它的脚本将立即失效。')) return
  try {
    await revokeToken(id)
    pushToast('已吊销', 'success')
    await load()
  } catch (e: any) {
    pushToast(e.message || '操作失败', 'error')
  }
}

function isExpired(t: TokenItem): boolean {
  return !!t.expires_at && new Date(t.expires_at).getTime() < Date.now()
}

onMounted(load)
</script>

<template>
  <div>
    <h2 class="text-lg font-semibold text-slate-800">API Token</h2>
    <p class="mt-1 text-sm text-slate-500">
      用于脚本或第三方工具上传图片。Token 原文仅在创建时展示一次，请妥善保存。
    </p>

    <div class="mt-4 rounded-2xl border border-slate-200 bg-white p-5 shadow-sm">
      <div class="flex flex-wrap items-end gap-3">
        <div class="flex-1 min-w-[200px]">
          <label class="mb-1 block text-sm text-slate-600">名称</label>
          <input
            v-model="name"
            type="text"
            class="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm outline-none transition focus:border-indigo-400 focus:ring-2 focus:ring-indigo-100"
            placeholder="例如：Typora 脚本"
          />
        </div>
        <div>
          <label class="mb-1 block text-sm text-slate-600">过期时间</label>
          <select
            v-model="expiry"
            class="rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm"
          >
            <option v-for="o in expiryOptions" :key="o.value" :value="o.value">
              {{ o.label }}
            </option>
          </select>
        </div>
        <button
          :disabled="creating"
          class="rounded-lg bg-indigo-600 px-4 py-2 text-sm text-white transition hover:bg-indigo-700 disabled:opacity-60"
          @click="onCreate"
        >
          {{ creating ? '创建中…' : '创建 Token' }}
        </button>
      </div>
    </div>

    <div class="mt-5 overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm">
      <div v-if="loading" class="py-16 text-center text-sm text-slate-400">加载中…</div>
      <div v-else-if="tokens.length === 0" class="py-16 text-center text-sm text-slate-400">
        还没有 Token，创建一个吧
      </div>
      <table v-else class="w-full text-sm">
        <thead class="border-b border-slate-100 bg-slate-50 text-left text-xs text-slate-500">
          <tr>
            <th class="px-4 py-3 font-medium">名称</th>
            <th class="px-4 py-3 font-medium">创建时间</th>
            <th class="px-4 py-3 font-medium">过期时间</th>
            <th class="px-4 py-3 font-medium">最近使用</th>
            <th class="px-4 py-3 text-right font-medium">操作</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-slate-100">
          <tr v-for="t in tokens" :key="t.id">
            <td class="px-4 py-3 font-medium text-slate-700">{{ t.name }}</td>
            <td class="px-4 py-3 text-slate-500">{{ formatTime(t.created_at) }}</td>
            <td class="px-4 py-3">
              <span
                v-if="t.expires_at"
                :class="isExpired(t) ? 'text-rose-500' : 'text-slate-500'"
              >
                {{ formatTime(t.expires_at) }}{{ isExpired(t) ? '（已过期）' : '' }}
              </span>
              <span v-else class="text-slate-400">永不过期</span>
            </td>
            <td class="px-4 py-3 text-slate-500">{{ formatTime(t.last_used_at) }}</td>
            <td class="px-4 py-3 text-right">
              <button
                class="rounded-md bg-rose-50 px-3 py-1.5 text-xs text-rose-500 transition hover:bg-rose-100"
                @click="onRevoke(t.id)"
              >
                吊销
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- 一次性 Token 展示弹窗 -->
    <div
      v-if="createdToken"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4"
      @click.self="createdToken = null"
    >
      <div class="w-full max-w-md rounded-2xl bg-white p-6 shadow-xl">
        <h3 class="text-base font-semibold text-slate-800">Token 已创建</h3>
        <p class="mt-1 text-sm text-slate-500">
          请立即复制保存，关闭后无法再次查看原文。
        </p>
        <div
          class="mt-4 break-all rounded-lg bg-slate-100 p-3 font-mono text-xs text-slate-700"
        >
          {{ createdToken.token }}
        </div>
        <div class="mt-5 flex justify-end gap-2">
          <button
            class="rounded-lg bg-indigo-600 px-4 py-2 text-sm text-white transition hover:bg-indigo-700"
            @click="copyToken"
          >
            {{ copied ? '已复制' : '复制 Token' }}
          </button>
          <button
            class="rounded-lg border border-slate-300 px-4 py-2 text-sm text-slate-600 transition hover:bg-slate-100"
            @click="createdToken = null"
          >
            关闭
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
