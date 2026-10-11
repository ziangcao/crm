<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { ArrowUpRight, Building2, CheckCircle2, Download, Edit3, MessageSquare, Paperclip, Plus, Printer, Send, Trash2, Upload, UserRound, X } from 'lucide-vue-next'
import { api, dateTime, money, statusLabel, statusTone } from './api'
import type { CRMRecord, Detail, FollowupComment, LineItem, Module, Page, SubtableRow, User } from './types'
import FollowupPlans from './FollowupPlans.vue'
import { hasModulePermission } from './permissions'
import { compressProductImage } from './productImages'

const props = defineProps<{ module: Module; recordId: string; user: User; modules: Module[]; revision: number }>()
const emit = defineEmits<{ close: []; edit: [record: CRMRecord]; changed: []; 'open-record': [module: string, id: string]; create: [module: string, initial: Record<string, string>] }>()
const record = ref<Detail>(), tab = ref('overview'), error = ref(''), busy = ref(false), users = ref<User[]>([]), assignee = ref('')
const content = ref(''), method = ref('电话'), nextDate = ref(''), confirmAction = ref('')
const followupCommentDrafts = ref<Record<string, string>>({})
type Attachment = { id: string; name: string; content_type: string; size: number; version: number; created_at: string }
const attachments = ref<Attachment[]>([]), selectedFile = ref<File>(), attachmentBusy = ref(false)
const productImageFiles = ref<File[]>([])
const imageUploadStatus = ref('')
const productImages = computed(() => props.module.key === 'products' ? attachments.value.filter(item => ['image/jpeg', 'image/png', 'image/webp', 'image/gif'].includes(item.content_type.toLowerCase())) : [])
type OwnershipHistory = { id: string; previous_owner_id: string; previous_owner_name: string; owner_id: string; owner_name: string; operator_name: string; action: string; created_at: string }
const ownershipHistory = ref<OwnershipHistory[]>([])
const inWritableScope = computed(() => !!record.value && (props.user.role_id ? true : props.user.role !== 'sales' || (record.value.owner_id === props.user.id && !props.module.shared)))
const canWrite = computed(() => inWritableScope.value && hasModulePermission(props.user, props.module.key, 'UPDATE', !record.value?.owner_id && ['customers', 'leads'].includes(props.module.key)))
const canEdit = computed(() => canWrite.value && !['pending', 'approved'].includes(String(record.value?.data.approval_status)) && !record.value?.data.converted_customer_id)
const canDelete = computed(() => inWritableScope.value && hasModulePermission(props.user, props.module.key, 'DELETE', !record.value?.owner_id && ['customers', 'leads'].includes(props.module.key)) && !['pending', 'approved'].includes(String(record.value?.data.approval_status)))
const canClaim = computed(() => !!record.value && !record.value.owner_id && !record.value.data.frozen && hasModulePermission(props.user, props.module.key, 'PICK', true))
const canRelease = computed(() => inWritableScope.value && hasModulePermission(props.user, props.module.key, 'RECYCLE'))
const canAssign = computed(() => !!record.value && (!record.value.data.frozen || !!record.value.owner_id) && hasModulePermission(props.user, props.module.key, record.value.owner_id ? 'TRANSFER' : 'ASSIGN', !record.value.owner_id))
const canFreeze = computed(() => !!record.value && !record.value.owner_id && ['customers', 'leads'].includes(props.module.key) && hasModulePermission(props.user, props.module.key, 'FREEZE', true))
const canVoid = computed(() => !!record.value && props.module.key === 'quotes' && record.value.data.approval_status === 'approved' && !record.value.data.voided && hasModulePermission(props.user, 'quotes', 'VOIDED'))
const canMerge = computed(() => !!record.value?.owner_id && props.module.key === 'customers' && inWritableScope.value && hasModulePermission(props.user, 'customers', 'MERGE'))
const canChangeContractStage = computed(() => !!record.value && props.module.key === 'contracts' && record.value.data.approval_status === 'approved' && inWritableScope.value && hasModulePermission(props.user, 'contracts', 'STAGE'))
const canConvert = computed(() => inWritableScope.value && hasModulePermission(props.user, 'leads', 'UPDATE') && hasModulePermission(props.user, 'customers', 'ADD') && hasModulePermission(props.user, 'contacts', 'ADD'))
const canPrint = computed(() => hasModulePermission(props.user, props.module.key, props.module.key === 'quotes' || props.module.key === 'orders' ? 'DOWNLOAD' : 'EXPORT'))
const approval = computed(() => record.value?.approvals.find(a => a.status === 'pending'))
const supportsPlans = computed(() => ['customers', 'leads', 'opportunities'].includes(props.module.key))
const printable = computed(() => !!props.module.document_template || ['quotes', 'contracts', 'orders'].includes(props.module.key))
async function load() {
  try { record.value = await api<Detail>(`/records/${props.module.key}/${props.recordId}`); attachments.value = (await api<{items: Attachment[]}>(`/records/${props.module.key}/${props.recordId}/attachments`)).items; ownershipHistory.value = ['customers', 'leads'].includes(props.module.key) ? (await api<{items: OwnershipHistory[]}>(`/records/${props.module.key}/${props.recordId}/ownership-history`)).items : []; error.value = ''; assignee.value = record.value.owner_id || '' }
  catch (e) { error.value = (e as Error).message }
}
onMounted(async () => { await load(); if (canAssign.value) users.value = await api<User[]>('/users') })
watch(() => props.recordId, () => { tab.value = 'overview'; productImageFiles.value = []; imageUploadStatus.value = ''; load() })
watch(() => props.revision, load)
async function action(name: string) {
  if (!record.value || busy.value) return
  busy.value = true; error.value = ''
	try {
		if (name === 'delete') { await api(`/records/${props.module.key}/${record.value.id}?version=${record.value.version}`, 'DELETE'); emit('changed'); emit('close'); return }
		let extra: Record<string, unknown> = name === 'assign' ? { owner_id: assignee.value } : {}
		if (name === 'freeze') {
			const reason = window.prompt('请输入冻结原因（必填）')?.trim()
			if (!reason) return
			const daysText = window.prompt('冻结天数（1-365，留空表示永久冻结）', '')?.trim()
			if (daysText === undefined) return
			const freeze_days = daysText ? Number(daysText) : 0
			if (!Number.isInteger(freeze_days) || freeze_days < 0 || freeze_days > 365) { error.value = '冻结天数应为 1-365，或留空表示永久冻结'; return }
			extra = { reason, freeze_days }
		}
		if (name === 'unfreeze' && !window.confirm('确认解除冻结？')) return
		if (name === 'void' && !window.confirm('确认作废此报价？已关联合同的报价不能作废。')) return
		if (name === 'merge') {
			const page = await api<Page>('/records/customers?page=1&size=100')
			const candidates = page.items.filter(item => item.id !== record.value?.id && item.owner_id)
			if (!candidates.length) { error.value = '没有可合并的客户'; return }
			const value = window.prompt(`输入要并入当前客户的客户 ID，多个 ID 用逗号分隔：\n${candidates.slice(0, 20).map(item => `${item.data.name} (${item.id})`).join('\n')}`)?.trim()
			if (!value) return
			extra = { merge_ids: [...new Set(value.split(',').map(item => item.trim()).filter(Boolean))] }
			if (!window.confirm('确认合并？源客户将删除，关联业务、跟进计划和附件将迁移到当前客户。')) return
		}
		if (name === 'stage') {
			const stages = props.module.fields.find(field => field.key === 'stage')?.options || []
			const target_stage = window.prompt(`请选择合同阶段：${stages.join('、')}`, String(record.value.data.stage || stages[0] || ''))?.trim()
			if (!target_stage) return
			if (!stages.includes(target_stage)) { error.value = '合同阶段无效'; return }
			extra = { target_stage }
		}
		await api(`/records/${props.module.key}/${record.value.id}/actions/${name}`, 'POST', { version: record.value.version, ...extra })
		confirmAction.value = ''; emit('changed')
		if (name === 'claim') { emit('close'); return }
		await load()
  } catch (e) { error.value = (e as Error).message }
  finally { busy.value = false }
}
async function splitOrder() {
	if (!record.value || busy.value || !Array.isArray(record.value.data.items)) return
	const value = window.prompt(`输入要拆出的明细序号，使用逗号分隔（1-${record.value.data.items.length}）`)
	if (!value) return
	const item_indexes = [...new Set(value.split(',').map(item => Number(item.trim()) - 1))]
	if (item_indexes.some(index => !Number.isInteger(index) || index < 0 || index >= (record.value?.data.items as LineItem[]).length)) { error.value = '拆分序号无效'; return }
	busy.value = true; error.value = ''
	try { await api(`/records/orders/${record.value.id}/actions/split`, 'POST', { version: record.value.version, item_indexes }); emit('changed'); await load() }
	catch (e) { error.value = (e as Error).message }
	finally { busy.value = false }
}
async function withdrawApproval() {
  if (!approval.value || busy.value) return
  busy.value = true; error.value = ''
  try { await api(`/approvals/${approval.value.id}/withdraw`, 'POST'); emit('changed'); await load() }
  catch (e) { error.value = (e as Error).message }
  finally { busy.value = false }
}
async function followup() {
  busy.value = true
  try {
    await api(`/records/${props.module.key}/${props.recordId}/followups`, 'POST', { content: content.value, method: method.value, next_date: nextDate.value || null })
    content.value = ''; nextDate.value = ''; await load(); emit('changed')
  } catch (e) { error.value = (e as Error).message }
  finally { busy.value = false }
}
async function addFollowupComment(followupId: string) {
  if (!record.value) return
  const content = (followupCommentDrafts.value[followupId] || '').trim()
  if (!content) return
  try {
    const comment = await api<FollowupComment>(`/records/${props.module.key}/${record.value.id}/followups/${followupId}/comments`, 'POST', { content })
    const followup = record.value.followups.find(item => item.id === followupId)
    if (followup) followup.comments = [...(followup.comments || []), comment]
    followupCommentDrafts.value[followupId] = ''
  } catch (e) { error.value = (e as Error).message }
}
async function editFollowupComment(followupId: string, comment: FollowupComment) {
  if (!record.value) return
  const content = window.prompt('编辑评论', comment.content)?.trim()
  if (!content || content === comment.content) return
  try {
    const updated = await api<FollowupComment>(`/records/${props.module.key}/${record.value.id}/followups/${followupId}/comments/${comment.id}`, 'PUT', { content })
    const followup = record.value.followups.find(item => item.id === followupId)
    if (followup) followup.comments = (followup.comments || []).map(item => item.id === updated.id ? updated : item)
  } catch (e) { error.value = (e as Error).message }
}
async function removeFollowupComment(followupId: string, comment: FollowupComment) {
  if (!record.value || !confirm('删除此评论？')) return
  try {
    await api(`/records/${props.module.key}/${record.value.id}/followups/${followupId}/comments/${comment.id}`, 'DELETE')
    const followup = record.value.followups.find(item => item.id === followupId)
    if (followup) followup.comments = (followup.comments || []).filter(item => item.id !== comment.id)
  } catch (e) { error.value = (e as Error).message }
}
function chooseAttachment(event: Event) { selectedFile.value = (event.target as HTMLInputElement).files?.[0] }
function chooseProductImages(event: Event) {
  const input = event.target as HTMLInputElement
  const files = Array.from(input.files || [])
  imageUploadStatus.value = ''
  if (files.length > 3 || files.some(file => !['image/jpeg', 'image/png', 'image/webp', 'image/gif'].includes(file.type) || file.size > 5 * 1024 * 1024 || !file.size)) {
    productImageFiles.value = []
    error.value = '每次最多选择 3 张 JPEG、PNG、WebP 或 GIF 图片，单张不超过 5 MB'
  } else { productImageFiles.value = files; error.value = '' }
  input.value = ''
}
async function uploadProductImages() {
  if (!productImageFiles.value.length || attachmentBusy.value) return
  attachmentBusy.value = true; error.value = ''
  const count = productImageFiles.value.length
  imageUploadStatus.value = `正在处理 ${count} 张图片，请勿重复操作…`
  try {
    const form = new FormData()
    const files = await Promise.all(productImageFiles.value.map(compressProductImage))
    imageUploadStatus.value = `正在上传 ${count} 张图片，请勿重复操作…`
    files.forEach(file => form.append('files', file))
    const response = await fetch(`/api/records/products/${props.recordId}/attachments`, { method: 'POST', credentials: 'same-origin', body: form })
    if (!response.ok) throw new Error((await response.json().catch(() => ({}))).detail || '图片上传失败')
    productImageFiles.value = []
    await load()
    imageUploadStatus.value = `已成功上传 ${count} 张图片`
  } catch (e) { error.value = (e as Error).message; imageUploadStatus.value = '图片上传失败，请查看上方错误提示' }
  finally { attachmentBusy.value = false }
}
async function uploadAttachment() {
  if (!selectedFile.value || attachmentBusy.value) return
  attachmentBusy.value = true; error.value = ''
  try {
    const form = new FormData(); form.append('file', selectedFile.value)
    const response = await fetch(`/api/records/${props.module.key}/${props.recordId}/attachments`, { method: 'POST', credentials: 'same-origin', body: form })
    if (!response.ok) throw new Error((await response.json().catch(() => ({}))).detail || '附件上传失败，请稍后重试')
    selectedFile.value = undefined; await load()
  } catch (e) { error.value = (e as Error).message }
  finally { attachmentBusy.value = false }
}
async function deleteAttachment(id: string) {
  if (attachmentBusy.value) return
  attachmentBusy.value = true
  try { await api(`/attachments/${id}`, 'DELETE'); await load() }
  catch (e) { error.value = (e as Error).message }
  finally { attachmentBusy.value = false }
}
const fileSize = (size: number) => size < 1024 * 1024 ? `${Math.max(1, Math.ceil(size / 1024))} KB` : `${(size / 1024 / 1024).toFixed(1)} MB`
function itemUnitPrice(item: LineItem): string { return item.unit_price || item.price || '0' }
function itemSubtotal(item: LineItem): string { return item.subtotal || String(Number(item.quantity) * Number(itemUnitPrice(item))) }
function value(field: { key: string; type: string }): string {
  if (!record.value) return ''
  const v = record.value.data[field.key]
  if (field.type === 'reference') return record.value.references[field.key] || '—'
  if (field.type === 'money' || field.type === 'formula') return v ? money(v) : '—'
  if (field.type === 'subtable') return Array.isArray(v) ? `${v.length} 条明细` : '—'
  return String(v ?? '') || '—'
}
const label = (key: string) => props.modules.find(m => m.key === key)?.label || key
function relatedCreate(module: string) {
	if (!record.value || !hasModulePermission(props.user, module, 'ADD')) return
	if (module === 'payments' && !hasModulePermission(props.user, 'contracts', 'PAYMENT')) return
  const initial: Record<string, string> = { customer_id: props.module.key === 'customers' ? record.value.id : String(record.value.data.customer_id || '') }
  if (props.module.key === 'contracts') initial.contract_id = record.value.id
  if (props.module.key === 'opportunities') initial.opportunity_id = record.value.id
  if (props.module.key === 'quotes') { initial.quote_id = record.value.id; initial.amount = String(record.value.data.amount); initial.name = String(record.value.data.name) + ' · 合同' }
	emit('create', module, initial)
}
function escapeHTML(value: unknown): string { return String(value ?? '').replace(/[&<>'"]/g, character => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#39;', '"': '&quot;' }[character] || character)) }
function renderDocumentTemplate(): string {
	if (!record.value || !props.module.document_template) return ''
	const values: Record<string, unknown> = { ...record.value.data, id: record.value.id, owner_name: record.value.owner_name, created_at: dateTime(record.value.created_at), updated_at: dateTime(record.value.updated_at) }
	return escapeHTML(props.module.document_template).replace(/\{\{\s*([a-z][a-z0-9_]*)\s*\}\}/g, (_, key: string) => escapeHTML(Array.isArray(values[key]) ? JSON.stringify(values[key]) : values[key] ?? '—')).replace(/\n/g, '<br>')
}
function printRecord() {
	if (!record.value) return
	const printWindow = window.open('', '_blank')
	if (!printWindow) { error.value = '浏览器阻止了打印窗口，请允许弹出窗口后重试'; return }
	const fields = props.module.fields.filter(field => !['items', 'subtable'].includes(field.type)).map(field => `<tr><th>${escapeHTML(field.label)}</th><td>${escapeHTML(value(field))}</td></tr>`).join('')
	const items = Array.isArray(record.value.data.items) ? `<section><h2>产品明细</h2><table><thead><tr><th>产品</th><th>规格</th><th>数量</th><th>单价</th><th>折扣</th><th>税率</th><th>含税小计</th></tr></thead><tbody>${(record.value.data.items as LineItem[]).map(item => `<tr><td>${escapeHTML(item.name)}</td><td>${escapeHTML(item.specification || "—")}</td><td>${escapeHTML(item.quantity)}</td><td>${escapeHTML(money(itemUnitPrice(item)))}</td><td>${escapeHTML(item.discount_rate || "0")}% </td><td>${escapeHTML(item.tax_rate || "0")}% </td><td>${escapeHTML(money(itemSubtotal(item)))}</td></tr>`).join('')}</tbody></table><p class="total">合计：${escapeHTML(money(record.value.data.amount))}</p></section>` : ''
	const body = props.module.document_template ? `<section class="template">${renderDocumentTemplate()}</section>` : `<table>${fields}</table>${items}`
	printWindow.document.write(`<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><title>${escapeHTML(props.module.label)} - ${escapeHTML(record.value.data.name)}</title><style>body{font:14px -apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;color:#1f2937;margin:44px auto;max-width:800px;line-height:1.8}h1{font-size:26px;margin:0 0 8px}h2{font-size:17px;margin:32px 0 12px}.meta{color:#6b7280;margin-bottom:28px}.template{white-space:normal;border-top:1px solid #d1d5db;padding-top:24px}table{width:100%;border-collapse:collapse}th,td{border:1px solid #d1d5db;padding:10px;text-align:left;vertical-align:top}th{width:30%;background:#f8fafc;font-weight:600}.total{text-align:right;font-size:17px;font-weight:700;margin-top:14px}@media print{body{margin:18px}}</style></head><body><h1>${escapeHTML(props.module.label)}</h1><p class="meta">${escapeHTML(record.value.data.name)} · ${escapeHTML(dateTime(record.value.created_at))}</p>${body}</body></html>`)
	printWindow.document.close()
	printWindow.focus()
	printWindow.print()
}
</script>

<template>
  <div class="drawer-backdrop" @click.self="emit('close')"><section class="detail-drawer" role="dialog" aria-modal="true" :aria-label="module.label + '详情'">
    <header class="drawer-header"><span>{{ module.label }}详情</span><button class="icon-button" aria-label="关闭详情" @click="emit('close')"><X :size="21"/></button></header>
    <div v-if="error" class="error-banner" role="alert">{{ error }}</div>
    <div v-if="!record" class="loading-state">正在加载…</div>
    <template v-if="record">
      <div class="detail-hero"><div class="entity-avatar"><Building2 :size="25"/></div><div><h2>{{ record.data.name }}</h2><div class="detail-meta"><span><UserRound :size="13"/>{{ record.owner_name }}</span><span>{{ dateTime(record.created_at) }} 创建</span><span v-if="record.data.approval_status" :class="['badge', statusTone(record.data.approval_status)]">{{ statusLabel[String(record.data.approval_status)] }}</span></div></div></div>
      <div class="detail-actions">
        <button v-if="canEdit" class="button" @click="emit('edit', record)"><Edit3 :size="15"/>编辑</button>
		<button v-if="printable && canPrint" class="button" @click="printRecord"><Printer :size="15"/>打印单据</button>
		<button v-if="canVoid" class="button danger" :disabled="busy" @click="action('void')">作废报价</button>
		<button v-if="canMerge" class="button" :disabled="busy" @click="action('merge')">合并客户</button>
		<button v-if="canChangeContractStage" class="button" :disabled="busy" @click="action('stage')">变更合同阶段</button>
        <button v-if="module.approvable && canEdit" class="button primary" :disabled="busy" @click="action('submit')"><Send :size="14"/>提交审批</button>
		<button v-if="approval?.requested_by === user.id" class="button" :disabled="busy" @click="withdrawApproval">撤回审批</button>
        <button v-if="['customers', 'leads'].includes(module.key) && canClaim" class="button primary" :disabled="busy" @click="action('claim')">领取到我的{{ module.label }}</button>
		<button v-if="canFreeze && !record.data.frozen" class="button" :disabled="busy" @click="action('freeze')">冻结公海记录</button>
		<button v-if="canFreeze && record.data.frozen" class="button" :disabled="busy" @click="action('unfreeze')">解除冻结</button>
        <button v-if="module.key === 'leads' && canConvert && record.owner_id" class="button primary" :disabled="busy" @click="confirmAction = 'convert'">转为客户</button>
        <button v-if="module.key === 'quotes' && record.data.approval_status === 'approved' && !record.data.voided && hasModulePermission(user, 'contracts', 'ADD')" class="button primary" @click="relatedCreate('contracts')">创建合同</button>
        <template v-if="module.key === 'contracts' && record.data.approval_status === 'approved'"><button v-if="hasModulePermission(user, 'orders', 'ADD')" class="button" @click="relatedCreate('orders')">创建订单</button><button v-if="hasModulePermission(user, 'payment_plans', 'ADD')" class="button" @click="relatedCreate('payment_plans')">制定回款计划</button><button v-if="hasModulePermission(user, 'payments', 'ADD') && hasModulePermission(user, 'contracts', 'PAYMENT')" class="button primary" @click="relatedCreate('payments')">登记回款</button></template>
        <button v-if="module.key === 'orders' && record.data.approval_status === 'draft' && (record.data.items as LineItem[])?.length > 1 && canWrite && hasModulePermission(user, 'orders', 'ADD')" class="button" :disabled="busy" @click="splitOrder">拆分订单</button><button v-if="module.key === 'orders' && record.data.approval_status === 'approved' && record.data.delivery_status !== '已完成' && canWrite" class="button primary" :disabled="busy" @click="confirmAction = 'deliver'">完成交付</button>
        <button v-if="['customers', 'leads'].includes(module.key) && record.owner_id && canRelease && !record.data.converted_customer_id" class="button" @click="confirmAction = 'release'">放入公海</button>
        <button v-if="canDelete" class="icon-button danger" aria-label="删除记录" @click="confirmAction = 'delete'"><Trash2 :size="16"/></button>
      </div>
      <div v-if="confirmAction" class="inline-confirm"><p>{{ ({ delete: '确定删除此记录？此操作不可恢复，关联记录将阻止删除。', release: '将该记录放入公海？客户的关联业务数据将一并移交给下一位领取人。', convert: '将创建客户与联系人，并保留线索跟进记录。确认转化？', deliver: '确认此订单已完成交付？' } as Record<string,string>)[confirmAction] }}</p><button class="button" :disabled="busy" @click="confirmAction = ''">取消</button><button class="button primary" :disabled="busy" @click="action(confirmAction)">{{ busy ? '处理中…' : '确认' }}</button></div>
      <div v-if="record.data.converted_customer_id" class="info-banner">该线索已转化为客户 <button class="text-button" @click="emit('open-record', 'customers', String(record.data.converted_customer_id))">查看客户 <ArrowUpRight :size="14"/></button></div>
	  <div v-if="record.data.frozen" class="info-banner">该公海记录已冻结：{{ record.data.freeze_reason }}<span v-if="record.data.unfreeze_at"> · {{ dateTime(String(record.data.unfreeze_at)) }} 自动解冻</span></div>
	  <div v-if="record.data.voided" class="info-banner">该报价已作废，不能继续生成合同。</div>
      <div v-if="approval" class="info-banner">此单据正在审批中，业务字段已锁定。<button class="text-button" @click="emit('close')">可前往审批中心处理</button></div>
      <nav class="detail-tabs"><button v-for="t in [{ key: 'overview', label: '基本信息' }, { key: 'followups', label: '跟进记录', count: record.followups.length }, ...(supportsPlans ? [{ key: 'plans', label: '跟进计划' }] : []), ...(['customers', 'leads'].includes(module.key) ? [{ key: 'ownership', label: '负责人历史', count: ownershipHistory.length }] : []), { key: 'related', label: '关联业务', count: record.related.length }, { key: 'attachments', label: '附件', count: attachments.length }, { key: 'approvals', label: '审批记录', count: record.approvals.length }]" :key="t.key" :class="{ active: tab === t.key }" @click="tab = t.key">{{ t.label }}<span v-if="t.count">{{ t.count }}</span></button></nav>
      <div class="detail-content">
        <section v-if="tab === 'overview' && module.key === 'products'" class="product-image-section"><h3>产品图片</h3><div v-if="canWrite" class="product-image-upload"><label><span>{{ productImageFiles.length ? '已选择：' + productImageFiles.map(file => file.name).join('、') : '选择图片（最多 3 张，单张不超过 5 MB）' }}</span><input type="file" accept="image/jpeg,image/png,image/webp,image/gif" multiple :disabled="attachmentBusy" @change="chooseProductImages"/></label><button class="button primary" :disabled="!productImageFiles.length || attachmentBusy" @click="uploadProductImages"><Upload :size="14"/>{{ imageUploadStatus && attachmentBusy ? '上传中…' : '上传图片' }}</button></div><div v-if="imageUploadStatus" class="info-banner" role="status">{{ imageUploadStatus }}</div><div v-if="!productImages.length" class="empty small">暂无产品图片</div><div v-else class="product-image-grid"><div v-for="item in productImages" :key="item.id" class="product-image-card"><a :href="'/api/attachments/' + item.id" target="_blank" rel="noreferrer"><img :src="'/api/attachments/' + item.id" :alt="item.name" loading="lazy"/></a><footer><span :title="item.name">{{ item.name }}</span><button v-if="canWrite" class="icon-button danger" :disabled="attachmentBusy" :aria-label="'删除图片' + item.name" @click="deleteAttachment(item.id)"><Trash2 :size="15"/></button></footer></div></div></section>
        <template v-if="tab === 'overview'"><div class="detail-fields"><template v-for="field in module.fields" :key="field.key"><div v-if="!['items','subtable'].includes(field.type)" :class="{ full: field.type === 'textarea' }"><label>{{ field.label }}</label><button v-if="field.type === 'reference' && record.data[field.key]" class="reference-link" @click="emit('open-record', field.module!, String(record.data[field.key]))">{{ value(field) }} <ArrowUpRight :size="13"/></button><p v-else>{{ value(field) }}</p></div></template></div><div v-for="field in module.fields.filter(field => field.type === 'subtable')" :key="field.key" class="detail-line-items"><h3>{{ field.label }}</h3><table><thead><tr><th v-for="column in field.columns || []" :key="column.key">{{ column.label }}</th></tr></thead><tbody><tr v-for="(row, index) in (record.data[field.key] as SubtableRow[] || [])" :key="index"><td v-for="column in field.columns || []" :key="column.key">{{ column.type === 'money' ? money(row[column.key]) : row[column.key] || '—' }}</td></tr></tbody></table></div><div v-if="record.data.items" class="detail-line-items"><h3>产品明细</h3><table><thead><tr><th>产品</th><th>规格</th><th>数量</th><th>单价</th><th>折扣</th><th>税率</th><th>含税小计</th></tr></thead><tbody><tr v-for="(item, index) in record.data.items as LineItem[]" :key="index"><td>{{ item.name }}</td><td>{{ item.specification || '—' }}</td><td>{{ item.quantity }}</td><td>{{ money(itemUnitPrice(item)) }}</td><td>{{ item.discount_rate || '0' }}%</td><td>{{ item.tax_rate || '0' }}%</td><td>{{ money(itemSubtotal(item)) }}</td></tr></tbody></table><div class="item-total">合计 <strong>{{ money(record.data.amount) }}</strong></div></div><div v-if="canAssign && ['customers', 'leads', 'tasks'].includes(module.key) && !record.data.converted_customer_id" class="assign-box"><h3>负责人分配</h3><p>客户分配会同步更新关联业务的负责人。</p><div><select v-model="assignee"><option value="">请选择负责人</option><option v-for="member in users.filter(u => u.active)" :key="member.id" :value="member.id">{{ member.name }}</option></select><button class="button" :disabled="!assignee || busy || assignee === record.owner_id" @click="action('assign')">确认分配</button></div></div></template>
        <template v-if="tab === 'followups'"><form v-if="canWrite" class="followup-form" @submit.prevent="followup"><h3><MessageSquare :size="16"/>记录一次新的进展</h3><textarea v-model="content" required maxlength="5000" placeholder="客户沟通了什么？下一步需要做什么？" rows="4"></textarea><div><select v-model="method" aria-label="跟进方式"><option v-for="v in ['电话','微信','邮件','拜访','会议','其他']" :key="v">{{ v }}</option></select><label>下次跟进<input v-model="nextDate" type="date"/></label><button class="button primary" :disabled="busy || !content.trim()">发布跟进</button></div></form><div v-if="!record.followups.length" class="empty"><MessageSquare :size="30"/><strong>还没有跟进记录</strong><span>记录客户需求，让团队协作更顺畅。</span></div><div v-for="f in record.followups" :key="f.id" class="followup-card"><header><b>{{ f.user_name }}</b><span class="badge blue">{{ f.method }}</span><time>{{ dateTime(f.created_at) }}</time></header><p>{{ f.content }}</p><small v-if="f.next_date">下次跟进：{{ f.next_date }}</small><section class="plan-comments"><b>协作评论</b><div v-if="!(f.comments || []).length" class="muted">暂无评论</div><div v-for="comment in f.comments || []" :key="comment.id" class="plan-comment"><strong>{{ comment.user_name }}</strong><span>{{ comment.content }}</span><time>{{ dateTime(comment.created_at) }}</time><button v-if="comment.user_id === user.id || ['admin', 'manager'].includes(user.role)" class="text-button" @click="editFollowupComment(f.id, comment)">编辑</button><button v-if="comment.user_id === user.id || ['admin', 'manager'].includes(user.role)" class="text-button danger" @click="removeFollowupComment(f.id, comment)">删除</button></div><form v-if="canWrite" @submit.prevent="addFollowupComment(f.id)"><textarea v-model="followupCommentDrafts[f.id]" required maxlength="2000" placeholder="添加协作评论" rows="2"/><button class="text-button">发表评论</button></form></section></div></template>
        <FollowupPlans v-if="tab === 'plans' && supportsPlans" :module="module" :record-id="record.id" :can-write="canWrite" :user-id="user.id" :manager="['admin', 'manager'].includes(user.role)" @changed="emit('changed')"/>
        <template v-if="tab === 'ownership'"><div v-if="!ownershipHistory.length" class="empty"><UserRound :size="30"/><span>暂无负责人变更记录</span></div><div v-for="item in ownershipHistory" :key="item.id" class="followup-card"><header><span class="badge blue">{{ ({ claim: '领取', release: '释放', assign: '分配' } as Record<string, string>)[item.action] || item.action }}</span><time>{{ dateTime(item.created_at) }}</time></header><p>{{ item.previous_owner_name || '公海' }} → {{ item.owner_name || '公海' }}</p><small>操作人：{{ item.operator_name }}</small></div></template>
        <template v-if="tab === 'related'"><div v-if="module.key === 'customers' && record.owner_id && canWrite" class="related-shortcuts"><button v-for="key in ['contacts','opportunities','tasks','companies']" :key="key" class="button" @click="relatedCreate(key)"><Plus :size="14"/>{{ label(key) }}</button></div><div v-if="!record.related.length" class="empty">暂无关联业务</div><button v-for="r in record.related" :key="r.id" class="related-row" @click="emit('open-record', r.module, r.id)"><span class="badge neutral">{{ label(r.module) }}</span><b>{{ r.data.name }}</b><span>{{ r.data.amount ? money(r.data.amount) : r.owner_name }}</span><ArrowUpRight :size="15"/></button></template>
        <template v-if="tab === 'attachments'"><div v-if="canWrite" class="attachment-upload"><Paperclip :size="19"/><label><span>{{ selectedFile?.name || '选择文件（单个文件最大 20MB）' }}</span><input type="file" @change="chooseAttachment"/></label><button class="button primary" :disabled="!selectedFile || attachmentBusy" @click="uploadAttachment"><Upload :size="14"/>上传</button></div><div v-if="!attachments.length" class="empty"><Paperclip :size="30"/><strong>还没有附件</strong><span>上传合同、报价单或沟通材料，集中保存业务资料。</span></div><div v-for="item in attachments" :key="item.id" class="attachment-row"><Paperclip :size="18"/><div><b>{{ item.name }} <small>v{{ item.version || 1 }}</small></b><small>{{ fileSize(item.size) }} · {{ dateTime(item.created_at) }}</small></div><a class="icon-button" :href="'/api/attachments/' + item.id + '?download=1'" :download="item.name" :aria-label="'下载' + item.name"><Download :size="16"/></a><button v-if="canWrite" class="icon-button danger" :disabled="attachmentBusy" :aria-label="'删除' + item.name" @click="deleteAttachment(item.id)"><Trash2 :size="16"/></button></div></template>
        <template v-if="tab === 'approvals'"><div v-if="!record.approvals.length" class="empty"><CheckCircle2 :size="30"/><span>暂无审批记录</span></div><div v-for="a in record.approvals" :key="a.id" class="followup-card"><header><span :class="['badge', statusTone(a.status)]">{{ statusLabel[a.status] }}</span><time>{{ dateTime(a.created_at) }}</time></header><p>{{ a.requester_name }} 提交审批{{ a.total_steps ? ' · 共 ' + a.total_steps + ' 级' : '' }}</p><div v-if="a.decisions?.length" class="approval-history"><div v-for="decision in a.decisions" :key="decision.id"><span :class="['badge', statusTone(decision.decision)]">第{{ decision.step }}级 · {{ statusLabel[decision.decision] }}</span><b>{{ decision.reviewer_name || '未知处理人' }}</b><time>{{ dateTime(decision.created_at) }}</time><small>{{ decision.comment || '未填写审批意见' }}</small></div></div><small v-else>{{ a.comment || '暂无审批意见' }}</small></div></template>
      </div>
    </template>
  </section></div>
</template>
