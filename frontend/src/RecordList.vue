<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { ArrowDownToLine, ArrowRight, ArrowUpFromLine, Building2, ChevronLeft, ChevronRight, Columns3, LayoutList, Plus, RefreshCw, Search, SlidersHorizontal, X } from 'lucide-vue-next'
import { api, compactMoney, money, statusLabel, statusTone } from './api'
import type { CRMRecord, Module, Page, User } from './types'
import { hasModulePermission } from './permissions'

const props = defineProps<{ module: Module; user: User; pool?: boolean; revision: number }>()
const emit = defineEmits<{ 'open-record': [module: string, id: string]; create: [module: string] }>()
const records = ref<CRMRecord[]>([]), total = ref(0), query = ref(''), scope = ref(props.pool ? 'pool' : 'all'), status = ref(''), page = ref(1), busy = ref(false), error = ref(''), board = ref(false), notice = ref('')
const importOpen = ref(false), importText = ref(''), importFile = ref<File>(), importBusy = ref(false)
let sequence = 0, timer: ReturnType<typeof setTimeout>
const canCreate = computed(() => !props.pool && hasModulePermission(props.user, props.module.key, 'ADD') && !(props.module.shared && props.user.role === 'sales' && !props.user.role_id))
const canImport = computed(() => !props.pool && hasModulePermission(props.user, props.module.key, 'IMPORT') && !(props.module.shared && props.user.role === 'sales' && !props.user.role_id))
const canExport = computed(() => hasModulePermission(props.user, props.module.key, 'EXPORT', !!props.pool))
const columns = computed(() => props.module.fields.filter(f => f.key !== 'name' && !['textarea', 'items'].includes(f.type)).slice(0, 5))
const statusField = computed(() => props.module.fields.find(f => ['status', 'stage'].includes(f.key)))
const params = computed(() => new URLSearchParams({ q: query.value, scope: scope.value, status: status.value, page: String(page.value), size: board.value ? '100' : '20' }).toString())
const stages = computed(() => props.module.fields.find(field => field.key === 'stage')?.options || [])
async function load() {
  const seq = ++sequence
  busy.value = true; error.value = ''
  try { const result = await api<Page>(`/records/${props.module.key}?${params.value}`); if (seq === sequence) { records.value = result.items; total.value = result.total } }
  catch (e) { if (seq === sequence) error.value = (e as Error).message }
  finally { if (seq === sequence) busy.value = false }
}
watch(query, () => { clearTimeout(timer); timer = setTimeout(() => { page.value = 1; load() }, 250) })
watch([scope, status, board], () => { page.value = 1; load() })
watch(() => props.revision, load)
onMounted(load)
function changePage(value: number) { page.value = value; load() }
function value(record: CRMRecord, key: string, type: string) {
  if (type === 'reference') return record.references[key] || '—'
  if (type === 'money') return record.data[key] ? money(record.data[key]) : '—'
  return String(record.data[key] ?? '') || '—'
}
async function exportData() {
  try {
    const response = await fetch(`/api/records/${props.module.key}/export?${params.value}`, { credentials: 'same-origin' })
    if (!response.ok) throw new Error((await response.json()).detail || '导出失败')
    const url = URL.createObjectURL(await response.blob()), a = document.createElement('a')
    a.href = url; a.download = `${props.module.label}-${new Date().toLocaleDateString('sv-SE')}.csv`; a.click(); URL.revokeObjectURL(url)
    notice.value = '已导出当前筛选条件下的数据'
  } catch (e) { error.value = (e as Error).message }
}
async function importData() {
  importBusy.value = true; error.value = ''
  try {
    const preview = await importRequest(true)
    if (!window.confirm(`已通过 ${preview.count} 条记录校验，确认导入吗？`)) return
    const result = await importRequest(false)
    importOpen.value = false; importText.value = ''; importFile.value = undefined; notice.value = `成功导入 ${result.count} 条记录`; await load()
  } catch (e) { error.value = (e as Error).message }
  finally { importBusy.value = false }
}
async function importRequest(preview: boolean): Promise<{ count: number }> {
  const path = `/api/records/${props.module.key}/import${preview ? '/preview' : ''}`
    if (importFile.value && !importFile.value.name.toLowerCase().endsWith('.json')) {
    const form = new FormData(); form.append('file', importFile.value)
    const response = await fetch(path, { method: 'POST', credentials: 'same-origin', body: form })
    if (!response.ok) throw new Error((await response.json()).detail || '导入失败')
    return response.json()
  }
  const parsed = importFile.value ? JSON.parse(await importFile.value.text()) : JSON.parse(importText.value)
    if (!Array.isArray(parsed)) throw new Error('请提供 JSON 数组，每个元素是一条记录的字段对象')
  return api<{count: number}>(`/records/${props.module.key}/import${preview ? '/preview' : ''}`, 'POST', { records: parsed })
}
function chooseImportFile(event: Event) { importFile.value = (event.target as HTMLInputElement).files?.[0] }
function downloadTemplate() {
  const row: Record<string, unknown> = {}
  for (const f of props.module.fields) row[f.key] = f.type === 'select' ? f.options?.[0] : f.type === 'items' ? [{ product_id: '填写产品ID', quantity: '1', unit_price: '100.00' }] : f.type === 'money' ? '100.00' : f.type === 'number' ? 50 : f.type === 'date' ? new Date().toLocaleDateString('sv-SE') : f.type === 'reference' ? '填写关联记录ID' : f.required ? '请填写' : ''
  const url = URL.createObjectURL(new Blob([JSON.stringify([row], null, 2)], { type: 'application/json' }))
  const a = document.createElement('a'); a.href = url; a.download = props.module.key + '-template.json'; a.click(); URL.revokeObjectURL(url)
}
function downloadCSVTemplate() {
  const content = '\ufeff' + props.module.fields.map(field => csvCell(field.key)).join(',') + '\n'
  const url = URL.createObjectURL(new Blob([content], { type: 'text/csv;charset=utf-8' }))
  const a = document.createElement('a'); a.href = url; a.download = props.module.key + '-template.csv'; a.click(); URL.revokeObjectURL(url)
}
async function downloadXLSXTemplate() {
  try {
    const response = await fetch(`/api/records/${props.module.key}/import-template`, { credentials: 'same-origin' })
    if (!response.ok) throw new Error((await response.json()).detail || '下载模板失败')
    const url = URL.createObjectURL(await response.blob()), a = document.createElement('a')
    a.href = url; a.download = props.module.key + '-template.xlsx'; a.click(); URL.revokeObjectURL(url)
  } catch (e) { error.value = (e as Error).message }
}
function csvCell(value: string): string { return /[",\r\n]/.test(value) ? '"' + value.replaceAll('"', '""') + '"' : value }
</script>

<template>
  <div class="page record-page">
    <header class="page-heading"><div><div class="eyebrow">{{ module.group }}</div><h1>{{ pool ? (module.key === 'leads' ? '线索池' : '客户公海') : module.label + '管理' }}<span class="heading-count">{{ total }}</span></h1><p>{{ pool ? '共享客户资源，让每个机会找到合适的负责人' : module.description }}</p></div><div class="heading-actions"><button v-if="canExport" class="button" @click="exportData"><ArrowDownToLine :size="16"/>导出</button><button v-if="canCreate" class="button primary" @click="emit('create', module.key)"><Plus :size="17"/>新建{{ module.label }}</button></div></header>
    <div v-if="error && !importOpen" class="error-banner" role="alert">{{ error }}<button class="text-button" @click="load">重试</button></div>
    <div v-if="notice" class="success-banner">{{ notice }}<button class="icon-button" aria-label="关闭提示" @click="notice = ''"><X :size="15"/></button></div>
    <section class="panel data-panel">
      <div class="table-tabs"><div v-if="!pool"><button :class="{ active: scope === 'all' }" @click="scope = 'all'">{{ user.role === 'sales' && !module.shared ? '我的' : '全部' }}{{ module.label }}</button><button v-if="user.role !== 'sales' && !module.shared" :class="{ active: scope === 'mine' }" @click="scope = 'mine'">我的{{ module.label }}</button></div><div v-else><button class="active">待领取{{ module.label }}</button></div><div class="table-view" v-if="module.key === 'opportunities'"><button :class="{ selected: !board }" aria-label="列表视图" @click="board = false"><LayoutList :size="16"/></button><button :class="{ selected: board }" aria-label="看板视图" @click="board = true"><Columns3 :size="16"/></button></div></div>
      <div class="table-toolbar"><div class="search-input"><Search :size="17"/><input v-model="query" :placeholder="'搜索' + module.label + '名称、关键词…'" :aria-label="'搜索' + module.label"/><button v-if="query" class="icon-button" aria-label="清除搜索" @click="query = ''"><X :size="13"/></button></div><div class="filter-select" v-if="statusField || module.approvable"><SlidersHorizontal :size="15"/><select v-model="status" aria-label="筛选状态"><option value="">全部{{ module.approvable ? '审批状态' : statusField?.label }}</option><template v-if="module.approvable"><option v-for="(text, key) in statusLabel" :key="key" :value="key">{{ text }}</option></template><template v-else><option v-for="s in statusField?.options" :key="s" :value="s">{{ s }}</option><option v-if="module.key === 'leads'" value="已转化">已转化</option></template></select></div><span class="toolbar-spacer"></span><button v-if="canImport" class="text-button muted" @click="importOpen = true"><ArrowUpFromLine :size="15"/>导入</button><button class="icon-button" aria-label="刷新数据" :disabled="busy" @click="load"><RefreshCw :size="16" :class="{ spinning: busy }"/></button></div>
      <div v-if="busy" class="table-loading"><span class="spinner"></span>加载中…</div>
      <div v-else-if="!records.length" class="empty table-empty"><div class="empty-icon"><Building2 :size="32"/></div><h3>{{ query || status ? '没有找到匹配的记录' : '从第一条' + module.label + '开始' }}</h3><p>{{ query || status ? '尝试调整搜索关键词或筛选条件。' : '把业务信息集中到这里，让每次协作更加清晰。' }}</p><button v-if="canCreate && !query && !status" class="button primary" @click="emit('create', module.key)"><Plus :size="16"/>新建{{ module.label }}</button></div>
      <div v-else-if="board" class="kanban"><div v-for="stage in stages" :key="stage" class="kanban-column"><header><span :class="['stage-dot', statusTone(stage)]"></span><b>{{ stage }}</b><span>{{ records.filter(r => r.data.stage === stage).length }}</span></header><button v-for="r in records.filter(r => r.data.stage === stage)" :key="r.id" class="kanban-card" @click="emit('open-record', module.key, r.id)"><strong>{{ r.data.name }}</strong><span>{{ r.references.customer_id }}</span><b>{{ compactMoney(r.data.amount) }}</b><footer><small>{{ r.owner_name }}</small><span>{{ r.data.expected_date }}</span></footer></button><div v-if="!records.some(r => r.data.stage === stage)" class="kanban-empty">暂无商机</div></div></div>
      <div v-else class="table-scroll"><table class="records-table"><thead><tr><th>{{ module.label }}名称</th><th v-for="c in columns" :key="c.key">{{ c.label }}</th><th v-if="module.approvable">审批状态</th><th>负责人</th><th></th></tr></thead><tbody><tr v-for="r in records" :key="r.id" @click="emit('open-record', module.key, r.id)"><td><button class="record-name" @click.stop="emit('open-record', module.key, r.id)"><span class="table-avatar">{{ String(r.data.name).slice(0, 1) }}</span>{{ r.data.name }}</button></td><td v-for="c in columns" :key="c.key"><span v-if="c.type === 'select'" :class="['badge', statusTone(r.data[c.key])]">{{ value(r, c.key, c.type) }}</span><span v-else :class="{ 'numeric': c.type === 'money' }">{{ value(r, c.key, c.type) }}</span></td><td v-if="module.approvable"><span :class="['badge', statusTone(r.data.approval_status)]">{{ statusLabel[String(r.data.approval_status)] }}</span></td><td><span class="owner-cell"><i>{{ r.owner_name.slice(-1) }}</i>{{ r.owner_name }}</span></td><td><ArrowRight :size="15" class="subtle"/></td></tr></tbody></table></div>
      <footer class="table-pagination"><span>共 {{ total }} 条记录<span v-if="board"> · 每页最多 100 条</span></span><div><button class="icon-button" aria-label="上一页" :disabled="page <= 1 || busy" @click="changePage(page - 1)"><ChevronLeft :size="17"/></button><span class="page-number">{{ page }}</span><span>/ {{ Math.max(1, Math.ceil(total / (board ? 100 : 20))) }}</span><button class="icon-button" aria-label="下一页" :disabled="page * (board ? 100 : 20) >= total || busy" @click="changePage(page + 1)"><ChevronRight :size="17"/></button></div></footer>
    </section>
    <div v-if="importOpen" class="modal-backdrop" @click.self="!importBusy && (importOpen = false)"><section class="modal" role="dialog" aria-modal="true" aria-label="导入记录"><header class="modal-header"><h2>导入{{ module.label }}</h2><button class="icon-button" aria-label="关闭导入" @click="importOpen = false"><X :size="20"/></button></header><form @submit.prevent="importData"><div class="modal-body"><p class="muted">选择 CSV / XLSX / JSON 文件，或粘贴 JSON 数组；每次最多 500 条。表头使用字段标识，关联字段填写记录 ID，产品明细使用 JSON 数组。</p><div class="role-options"><button type="button" class="text-button" @click="downloadXLSXTemplate">下载 XLSX 模板</button><button type="button" class="text-button" @click="downloadCSVTemplate">下载 CSV 模板</button><button type="button" class="text-button" @click="downloadTemplate">下载 JSON 模板</button></div><label class="field"><span>CSV、XLSX 或 JSON 文件</span><input type="file" accept=".csv,.xlsx,.json,text/csv,application/json,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" @change="chooseImportFile"/><small v-if="importFile">已选择：{{ importFile.name }}</small></label><label class="field"><span>JSON 数据（未选择文件时使用）</span><textarea v-model="importText" class="code-input" rows="8" :required="!importFile" placeholder='[{ "name": "示例" }]'/></label><p v-if="error" class="error-banner">{{ error }}</p></div><footer class="modal-footer"><button type="button" class="button" @click="importOpen = false">取消</button><button class="button primary" :disabled="importBusy">{{ importBusy ? '正在验证…' : '验证并导入' }}</button></footer></form></section></div>
  </div>
</template>
