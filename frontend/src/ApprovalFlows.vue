<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ArrowDown, ArrowUp, Check, Plus, Power, RefreshCw, X } from 'lucide-vue-next'
import { api } from './api'
import type { Module, User } from './types'
import { hasPermission } from './permissions'

type Role = 'admin' | 'manager' | 'sales'
interface ApprovalCondition { Field: string; Operator: 'eq' | 'neq' | 'contains' | 'empty'; Value: string }
interface ApprovalFlowStep { Order: number; ApproverRoles: Role[]; ApproverIDs?: string[]; MinAmount?: string; Conditions?: ApprovalCondition[] }
interface ApprovalFlow { id: string; module: string; name: string; enabled: boolean; version: number; steps: ApprovalFlowStep[] }

const props = defineProps<{ modules: Module[]; user: User }>()
const emit = defineEmits<{ changed: [] }>()
const items = ref<ApprovalFlow[]>([]), users = ref<User[]>([]), moduleKey = ref(''), name = ref(''), steps = ref<ApprovalFlowStep[]>([{ Order: 1, ApproverRoles: ['manager'], ApproverIDs: [] }]), editingID = ref(''), editingEnabled = ref(false), busy = ref(false), loading = ref(false), error = ref(''), success = ref('')
const approvableModules = computed(() => props.modules.filter(module => module.approvable))
const configuredModule = computed(() => props.modules.find(module => module.key === moduleKey.value))
const conditionFields = computed(() => {
  const fields = configuredModule.value?.fields.filter(field => field.type !== 'items') || []
  return fields.some(field => field.key === 'amount') ? fields : [{ key: 'amount', label: '金额（元）', type: 'money', required: false }, ...fields]
})
const roleLabel: Record<Role, string> = { admin: '管理员', manager: '销售经理', sales: '销售成员' }
const canAdd = computed(() => hasPermission(props.user, 'PROCESS_SETTING:ADD'))
const canUpdate = computed(() => hasPermission(props.user, 'PROCESS_SETTING:UPDATE'))

async function load() {
  loading.value = true; error.value = ''
  try { const [flows, members] = await Promise.all([api<{items: ApprovalFlow[]}>('/approval-flows'), api<User[]>('/users')]); items.value = flows.items; users.value = members.filter(user => user.active && (user.role === 'admin' || user.role === 'manager')) }
  catch (exception) { error.value = (exception as Error).message }
  finally { loading.value = false }
}
function addStep() { steps.value.push({ Order: steps.value.length + 1, ApproverRoles: ['manager'], ApproverIDs: [], Conditions: [] }) }
function addCondition(step: ApprovalFlowStep) { (step.Conditions ||= []).push({ Field: '', Operator: 'eq', Value: '' }) }
function removeCondition(step: ApprovalFlowStep, index: number) { step.Conditions?.splice(index, 1) }
function removeStep(index: number) { steps.value.splice(index, 1); steps.value.forEach((step, order) => step.Order = order + 1); steps.value[steps.value.length - 1].MinAmount = '' }
function moveStep(index: number, direction: -1 | 1) {
  const target = index + direction
  if (target < 0 || target >= steps.value.length) return
  const [step] = steps.value.splice(index, 1)
  steps.value.splice(target, 0, step)
  steps.value.forEach((item, order) => item.Order = order + 1)
  steps.value[steps.value.length - 1].MinAmount = ''
}
function stepSummary(step: ApprovalFlowStep) {
  const approvers = step.ApproverIDs?.length ? `${step.ApproverIDs.length} 名指定成员` : step.ApproverRoles.map(role => roleLabel[role]).join('、')
  const rules = [...(step.MinAmount ? [`金额 ≥ ${step.MinAmount} 元`] : []), ...((step.Conditions || []).map(condition => `${conditionFields.value.find(field => field.key === condition.Field)?.label || condition.Field} ${condition.Operator === 'eq' ? '等于' : condition.Operator === 'neq' ? '不等于' : condition.Operator === 'contains' ? '包含' : '为空'} ${condition.Value}`))]
  return { approvers, rules: rules.length ? rules.join(' 且 ') : '无条件，直接进入本级' }
}
function editFlow(flow: ApprovalFlow) { editingID.value = flow.id; editingEnabled.value = flow.enabled; moduleKey.value = flow.module; name.value = flow.name; steps.value = flow.steps.map(step => ({ Order: step.Order, ApproverRoles: [...step.ApproverRoles], ApproverIDs: [...(step.ApproverIDs || [])], MinAmount: step.MinAmount || '', Conditions: (step.Conditions || []).map(condition => ({ ...condition })) })) }
function toggleRole(step: ApprovalFlowStep, role: Role) {
  if (step.ApproverRoles.includes(role)) {
    if (step.ApproverRoles.length > 1) step.ApproverRoles = step.ApproverRoles.filter(item => item !== role)
  } else step.ApproverRoles.push(role)
}
async function save() {
  if (busy.value) return
  busy.value = true; error.value = ''; success.value = ''
  try {
    await api(editingID.value ? `/approval-flows/${editingID.value}` : '/approval-flows', editingID.value ? 'PUT' : 'POST', { module: moduleKey.value, name: name.value, enabled: editingEnabled.value, steps: steps.value })
    name.value = ''; moduleKey.value = ''; editingID.value = ''; editingEnabled.value = false; steps.value = [{ Order: 1, ApproverRoles: ['manager'], ApproverIDs: [], Conditions: [] }]; success.value = '审批流版本已保存'; await load(); emit('changed')
  } catch (exception) { error.value = (exception as Error).message }
  finally { busy.value = false }
}
async function setEnabled(flow: ApprovalFlow) {
  busy.value = true; error.value = ''; success.value = ''
  try { await api(`/approval-flows/${flow.id}/enabled`, 'POST', { enabled: !flow.enabled }); await load(); success.value = flow.enabled ? '审批流已停用' : '审批流已启用'; emit('changed') }
  catch (exception) { error.value = (exception as Error).message }
  finally { busy.value = false }
}
onMounted(load)
</script>

<template>
  <div class="page">
    <header class="page-heading"><div><div class="eyebrow">APPROVAL AUTOMATION</div><h1>审批流配置</h1><p>同一业务模块仅允许一个启用流程；已提交单据始终按提交时的流程版本流转。</p></div><button class="icon-button" :disabled="loading" aria-label="刷新审批流" @click="load"><RefreshCw :size="17" :class="{ spinning: loading }"/></button></header>
    <p v-if="error" class="error-banner">{{ error }}</p><p v-if="success" class="success-banner"><Check :size="16"/>{{ success }}</p>
    <section v-if="editingID ? canUpdate : canAdd" class="panel setting-form"><h2>{{ editingID ? '编辑审批流' : '新建审批流' }}</h2><form @submit.prevent="save"><div class="settings-grid"><label class="field"><span>业务模块</span><select v-model="moduleKey" required :disabled="!!editingID"><option value="" disabled>请选择需要审批的模块</option><option v-for="module in approvableModules" :key="module.key" :value="module.key">{{ module.label }}</option></select></label><label class="field"><span>流程名称</span><input v-model.trim="name" maxlength="100" required placeholder="例如：标准报价审批"/></label></div><p class="muted">可为前面的审批级别设置金额门槛或字段条件；条件全部满足时执行该级，未匹配的级别自动跳过，最后一级始终执行。</p><div class="approval-canvas" aria-label="审批流程画布"><div class="approval-start">提交单据</div><template v-for="(step, index) in steps" :key="`node-${step.Order}`"><div class="approval-connector"><span>条件通过</span></div><article class="approval-node"><header><b>第 {{ step.Order }} 级审批</b><div><button type="button" class="icon-button" :disabled="index === 0" aria-label="上移审批节点" @click="moveStep(index, -1)"><ArrowUp :size="14"/></button><button type="button" class="icon-button" :disabled="index === steps.length - 1" aria-label="下移审批节点" @click="moveStep(index, 1)"><ArrowDown :size="14"/></button></div></header><strong>{{ stepSummary(step).approvers }}</strong><small>{{ stepSummary(step).rules }}</small></article></template><div class="approval-connector"></div><div class="approval-end">审批完成</div></div><div class="flow-steps"><div v-for="(step, index) in steps" :key="step.Order" class="flow-step"><b>第 {{ step.Order }} 级</b><div class="role-options"><button v-for="(label, role) in roleLabel" :key="role" type="button" :class="['text-button', { 'green-text': step.ApproverRoles.includes(role) }]" @click="toggleRole(step, role)">{{ step.ApproverRoles.includes(role) ? '✓ ' : '' }}{{ label }}</button></div><div class="field"><span>指定审批人（可选；指定后仅这些成员可处理）</span><div class="role-options"><label v-for="member in users" :key="member.id" class="checkbox-label"><input v-model="step.ApproverIDs" type="checkbox" :value="member.id"/>{{ member.name }}</label></div></div><label v-if="index < steps.length - 1" class="field"><span>最低金额（元，留空表示总是执行）</span><input v-model="step.MinAmount" type="number" min="0.01" step="0.01" placeholder="例如：100000"/></label><div class="field"><span>字段条件（全部满足时才执行本级）</span><div v-for="(condition, conditionIndex) in step.Conditions || []" :key="conditionIndex" class="role-options"><select v-model="condition.Field" aria-label="条件字段"><option value="" disabled>选择字段</option><option v-for="field in conditionFields" :key="field.key" :value="field.key">{{ field.label }}</option></select><select v-model="condition.Operator" aria-label="条件操作符"><option value="eq">等于</option><option value="neq">不等于</option><option value="contains">包含</option><option value="empty">为空</option></select><input v-if="condition.Operator !== 'empty'" v-model.trim="condition.Value" maxlength="100" placeholder="条件值"/><button type="button" class="icon-button danger" aria-label="删除字段条件" @click="removeCondition(step, conditionIndex)"><X :size="16"/></button></div><button type="button" class="text-button" :disabled="!moduleKey" @click="addCondition(step)"><Plus :size="14"/>添加字段条件</button></div><button v-if="steps.length > 1" type="button" class="icon-button danger" aria-label="删除审批步骤" @click="removeStep(index)"><X :size="16"/></button></div></div><footer class="modal-footer"><button type="button" class="button" @click="addStep"><Plus :size="15"/>新增审批级别</button><button class="button primary" :disabled="busy || !moduleKey || !name">{{ busy ? '保存中…' : editingID ? '保存新版本' : '创建流程' }}</button></footer></form></section>
    <section class="panel data-panel"><header class="panel-heading"><div><h2>已有审批流</h2><p>启用后，新提交的单据将按此流程流转。</p></div><span class="tiny-label">{{ items.length }} 条</span></header><div v-if="loading" class="loading-state">加载审批流…</div><div v-else-if="!items.length" class="empty table-empty"><h3>暂无审批流</h3><p>请先创建一个流程。</p></div><div v-else class="table-scroll"><table><thead><tr><th>名称</th><th>模块</th><th>审批级别</th><th>版本</th><th>状态</th><th></th></tr></thead><tbody><tr v-for="flow in items" :key="flow.id"><td><b>{{ flow.name }}</b></td><td>{{ approvableModules.find(module => module.key === flow.module)?.label || flow.module }}</td><td><span v-for="step in flow.steps" :key="step.Order" class="badge neutral">第{{ step.Order }}级：{{ step.ApproverRoles.map(role => roleLabel[role]).join('、') }}{{ step.MinAmount ? `（≥${step.MinAmount}元）` : '' }}</span></td><td>v{{ flow.version }}</td><td><span :class="['badge', flow.enabled ? 'green' : 'neutral']">{{ flow.enabled ? '已启用' : '未启用' }}</span></td><td><button v-if="canUpdate" class="text-button" :disabled="busy" @click="editFlow(flow)">编辑</button><button v-if="canUpdate" class="text-button" :disabled="busy" @click="setEnabled(flow)"><Power :size="14"/>{{ flow.enabled ? '停用' : '启用' }}</button></td></tr></tbody></table></div></section>
  </div>
</template>
