<script setup lang="ts">
import { toasts } from '../composables/toast'

const typeClass: Record<string, string> = {
  success: 'bg-emerald-500',
  error: 'bg-rose-500',
  info: 'bg-slate-800',
}
</script>

<template>
  <div class="fixed top-4 left-1/2 z-50 flex -translate-x-1/2 flex-col items-center gap-2">
    <transition-group name="toast">
      <div
        v-for="t in toasts"
        :key="t.id"
        class="flex items-center rounded-lg px-4 py-2 text-sm text-white shadow-lg"
        :class="typeClass[t.type]"
      >
        {{ t.message }}
      </div>
    </transition-group>
  </div>
</template>

<style scoped>
.toast-enter-active,
.toast-leave-active {
  transition: all 0.25s ease;
}
.toast-enter-from,
.toast-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}
</style>
