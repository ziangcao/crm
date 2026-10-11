<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { Search } from 'lucide-vue-next'
import { api } from './api'
import type { CRMRecord, Page } from './types'

const props = defineProps<{ modelValue: string; module: string; required?: boolean; disabled?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: string]; select: [record: CRMRecord] }>()
const items = ref<CRMRecord[]>([]), query = ref(''), error = ref(''), loading = ref(false)
let sequence = 0, timer: ReturnType<typeof setTimeout>
async function load() {
  const seq = ++sequence
  loading.value = true
  try {
    const page = await api<Page>(`/records/${props.module}?size=100&q=${encodeURIComponent(query.value)}`)
    const choices = page.items.filter(r => r.owner_id || props.module === 'products')
    if (props.modelValue && !choices.some(r => r.id === props.modelValue)) {
      choices.unshift(await api<CRMRecord>(`/records/${props.module}/${props.modelValue}`))
    }
    if (seq === sequence) { items.value = choices; error.value = '' }
  } catch (e) { if (seq === sequence) error.value = (e as Error).message }
  finally { if (seq === sequence) loading.value = false }
}
watch(query, () => { clearTimeout(timer); timer = setTimeout(load, 250) })
watch(() => props.module, load)
onMounted(load)
function select(event: Event) {
  const value = (event.target as HTMLSelectElement).value
  emit('update:modelValue', value)
  const record = items.value.find(r => r.id === value)
  if (record) emit('select', record)
}
</script>

<template>
  <div class="reference-input">
    <div class="reference-search"><Search :size="14"/><input v-model="query" placeholder="搜索关联记录" aria-label="搜索关联记录" :disabled="disabled" /></div>
    <select :value="modelValue" :required="required" :disabled="disabled" @change="select"><option value="">{{ loading ? '加载中…' : '请选择（最多显示 100 条，可搜索）' }}</option><option v-for="r in items" :key="r.id" :value="r.id">{{ r.data.name }}</option></select>
    <small v-if="error" class="error-text">{{ error }}</small>
  </div>
</template>
