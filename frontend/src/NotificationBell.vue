<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { Bell } from 'lucide-vue-next'
import { api } from './api'

type Notification = { id: string; type: string; subject: string; content: string; resource_id: string; resource_type: string; created_at: string }

const open = ref(false)
const busy = ref(false)
const items = ref<Notification[]>([])
const unread = ref(0)
let stream: EventSource | undefined

async function load() {
  busy.value = true
  try {
    const [latest, count] = await Promise.all([
      api<{ items: Notification[] }>('/notifications/latest'),
      api<{ unread: number }>('/notifications/count'),
    ])
    items.value = latest.items
    unread.value = count.unread
  } finally {
    busy.value = false
  }
}

async function toggle() {
  open.value = !open.value
  if (open.value) await load()
}

async function read(item: Notification) {
  await api('/notifications/' + item.id + '/read', 'POST')
  await load()
  if (item.resource_id && item.resource_type) window.dispatchEvent(new CustomEvent('open-record', { detail: { id: item.resource_id, module: item.resource_type } }))
}

async function readAll() {
  await api('/notifications/read-all', 'POST')
  await load()
}

onMounted(() => {
  load().catch(() => undefined)
  stream = new EventSource('/api/notifications/stream')
  stream.addEventListener('notification', event => {
    try {
      const payload = JSON.parse((event as MessageEvent<string>).data) as { count: { unread: number }; items: Notification[] }
      unread.value = payload.count.unread
      if (open.value) items.value = payload.items
    } catch {}
  })
})
onUnmounted(() => stream?.close())
</script>

<template>
  <div class="notification-menu">
    <button class="icon-button notification-button" aria-label="查看通知" title="通知中心" @click="toggle">
      <Bell :size="18"/><i v-if="unread">{{ unread > 99 ? '99+' : unread }}</i>
    </button>
    <section v-if="open" class="notification-panel">
      <header><b>通知中心</b><button v-if="unread" class="text-button" :disabled="busy" @click="readAll">全部已读</button></header>
      <button v-for="item in items" :key="item.id" class="notification-item" :disabled="busy" @click="read(item)">
		<b>{{ item.type === 'announcement' ? '公告 · ' : '' }}{{ item.subject }}</b><span>{{ item.content }}</span><time>{{ new Date(item.created_at).toLocaleString() }}</time>
      </button>
      <p v-if="!busy && !items.length" class="muted">暂无未读通知</p>
      <p v-if="busy" class="muted">正在加载…</p>
    </section>
  </div>
</template>
