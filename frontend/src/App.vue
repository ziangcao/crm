<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { Activity, ArrowRight, Bell, Blocks, BookOpen, BriefcaseBusiness, Building2, ChartNoAxesCombined, CheckSquare, ChevronDown, ChevronRight, CircleDollarSign, ClipboardCheck, ContactRound, Database, FileCheck2, FileSignature, FileText, HandCoins, House, Layers3, LayoutDashboard, ListFilter, LogOut, Menu, Package, Plus, Search, Settings2, ShieldCheck, ShoppingBag, Sparkles, Target, Users, Workflow, X } from 'lucide-vue-next'
import { api, ApiError, roleLabel } from './api'
import type { CRMRecord, Module, User, Value } from './types'
import Dashboard from './Dashboard.vue'
import RecordList from './RecordList.vue'
import RecordForm from './RecordForm.vue'
import RecordDetail from './RecordDetail.vue'
import Approvals from './Approvals.vue'
import ApprovalFlows from './ApprovalFlows.vue'
import Settings from './Settings.vue'
import Analytics from './Analytics.vue'
import { hasModulePermission, hasPermission } from './permissions'

const user = ref<User>(), modules = ref<Module[]>([]), loading = ref(true), setup = ref(false), weComEnabled = ref(false), authBusy = ref(false), error = ref(''), toast = ref(''), mobileMenu = ref(false)
const username = ref(''), password = ref(''), name = ref(''), demo = ref(true)
const searchQuery = ref(''), searchItems = ref<CRMRecord[]>([]), searchOpen = ref(false), searchBusy = ref(false)
const route = ref(location.hash.slice(1) || 'dashboard'), revision = ref(0), formModule = ref<Module>(), editRecord = ref<CRMRecord>(), initial = ref<Record<string, Value>>(), detail = ref<{module: Module; id: string}>()
const icons: Record<string, typeof Building2> = { dashboard: LayoutDashboard, dashboard_configs: LayoutDashboard, analytics: ChartNoAxesCombined, leads: ListFilter, lead_pool: Database, customers: Building2, customer_pool: Layers3, contacts: ContactRound, opportunities: Target, products: Package, prices: CircleDollarSign, quotes: FileText, contracts: FileSignature, orders: ShoppingBag, companies: Building2, invoices: FileCheck2, payment_plans: HandCoins, payments: CircleDollarSign, tenders: BriefcaseBusiness, tasks: CheckSquare, approvals: ClipboardCheck, users: Users, organizations: Building2, modules: Blocks, approval_flows: Workflow, fields: Settings2, agents: Sparkles, audit: Activity }
const moduleKeys = new Set(['leads', 'lead_pool', 'customers', 'customer_pool', 'contacts', 'opportunities', 'products', 'prices', 'quotes', 'contracts', 'orders', 'companies', 'invoices', 'payment_plans', 'payments', 'tenders', 'tasks'])
const systemRoutePermissions: Record<string, string[]> = { users: ['SYS_ORGANIZATION:READ', 'SYSTEM_ROLE:ADD_USER'], organizations: ['SYS_ORGANIZATION:READ', 'SYSTEM_ROLE:READ'], dashboard_configs: ['DASHBOARD:READ'], modules: ['MODULE_SETTING:READ'], approval_flows: ['PROCESS_SETTING:READ'], fields: ['CUSTOM_FORM:READ'], agents: ['AGENT:READ'], audit: ['OPERATION_LOG:READ'] }
function canNavigate(key: string): boolean {
  if (!user.value) return false
  if (moduleKeys.has(key) || modules.value.some(module => module.key === key)) {
    const pool = key.endsWith('_pool'), moduleKey = pool ? key.replace('_pool', 's') : key
    return modules.value.some(module => module.key === moduleKey) && hasModulePermission(user.value, moduleKey, 'READ', pool)
  }
  if (key === 'dashboard' || key === 'analytics') return hasPermission(user.value, 'DASHBOARD:READ')
  if (key === 'approvals') return modules.value.some(module => module.approvable && hasModulePermission(user.value!, module.key, 'READ'))
	if (key === 'organizations') return hasPermission(user.value, 'SYS_ORGANIZATION:READ') && hasPermission(user.value, 'SYSTEM_ROLE:READ')
  return !systemRoutePermissions[key] || systemRoutePermissions[key].some(permission => hasPermission(user.value!, permission))
}
const nav = computed(() => [
  { label: '工作空间', items: [{ key: 'dashboard', label: '工作台' }, { key: 'analytics', label: '自助 BI' }, { key: 'tasks', label: '任务计划' }, { key: 'approvals', label: '审批中心' }] },
  { label: '客户经营', items: [{ key: 'leads', label: '线索管理' }, { key: 'lead_pool', label: '线索池' }, { key: 'customers', label: '客户管理' }, { key: 'customer_pool', label: '客户公海' }, { key: 'contacts', label: '联系人' }] },
  { label: '销售协作', items: [{ key: 'opportunities', label: '商机管理' }, { key: 'products', label: '产品管理' }, { key: 'prices', label: '价格管理' }, { key: 'quotes', label: '报价管理' }, { key: 'tenders', label: '标讯管理' }, ...modules.value.filter(module => module.key.startsWith('custom_')).map(module => ({ key: module.key, label: module.label }))] },
  { label: '交易履约', items: [{ key: 'contracts', label: '合同管理' }, { key: 'orders', label: '订单管理' }, { key: 'companies', label: '工商抬头' }, { key: 'invoices', label: '发票管理' }, { key: 'payment_plans', label: '回款计划' }, { key: 'payments', label: '回款管理' }] },
  { label: '系统管理', items: [{ key: 'users', label: '成员与权限' }, { key: 'organizations', label: '组织与角色' }, { key: 'dashboard_configs', label: '工作台配置' }, { key: 'modules', label: '业务模块' }, { key: 'approval_flows', label: '审批流配置' }, { key: 'fields', label: '自定义字段' }, { key: 'agents', label: '智能助手' }, { key: 'audit', label: '操作日志' }] },
].map(group => ({ ...group, items: group.items.filter(item => canNavigate(item.key)) })).filter(group => group.items.length))
const currentNav = computed(() => nav.value.flatMap(g => g.items).find(item => item.key === route.value))
const currentModule = computed(() => modules.value.find(m => m.key === (route.value === 'customer_pool' ? 'customers' : route.value === 'lead_pool' ? 'leads' : route.value)))
const currentGroup = computed(() => nav.value.find(g => g.items.some(i => i.key === route.value))?.label || '个人设置')
const settingsMode = computed(() => ['users', 'organizations', 'dashboard_configs', 'modules', 'fields', 'agents', 'audit', 'profile'].includes(route.value))

async function loadContext() { modules.value = await api<Module[]>('/catalog') }
async function boot() {
  loading.value = true; error.value = ''
  try {
		const [setupStatus, weComStatus] = await Promise.all([api<{required: boolean}>('/setup'), api<{enabled: boolean}>('/auth/wecom/status')])
		setup.value = setupStatus.required; weComEnabled.value = weComStatus.enabled
    if (!setup.value) {
      try { user.value = await api<User>('/auth/me'); await loadContext() }
      catch (e) { if (!(e instanceof ApiError && e.status === 401)) throw e }
    }
  } catch (e) { error.value = (e as Error).message }
  finally { loading.value = false }
}
async function authenticate() {
  if (authBusy.value) return
  authBusy.value = true; error.value = ''
  try {
    user.value = await api<User>(setup.value ? '/setup' : '/auth/login', 'POST', { username: username.value, password: password.value, ...(setup.value ? { name: name.value, demo: demo.value } : {}) })
    setup.value = false; password.value = ''; await loadContext(); navigate('dashboard')
  } catch (e) { error.value = (e as Error).message }
  finally { authBusy.value = false }
}
function navigate(key: string) { route.value = key; location.hash = key; detail.value = undefined; mobileMenu.value = false }
let searchTimer: ReturnType<typeof setTimeout>
function search() {
  clearTimeout(searchTimer)
  const query = searchQuery.value.trim()
  if (!query) { searchItems.value = []; searchOpen.value = false; return }
  searchOpen.value = true
  searchTimer = setTimeout(async () => {
    searchBusy.value = true
    try { searchItems.value = (await api<{items: CRMRecord[]}>('/search?q=' + encodeURIComponent(query))).items }
    catch (e) { searchItems.value = []; notify((e as Error).message) }
    finally { searchBusy.value = false }
  }, 220)
}
function selectSearch(record: CRMRecord) { searchQuery.value = ''; searchItems.value = []; searchOpen.value = false; openRecord(record.module, record.id) }
function hashChanged() { route.value = location.hash.slice(1) || 'dashboard'; detail.value = undefined }
function sessionExpired() { user.value = undefined; detail.value = undefined; formModule.value = undefined; error.value = '登录已过期，请重新登录' }
async function logout() { try { await api('/auth/logout', 'POST'); user.value = undefined; detail.value = undefined; formModule.value = undefined; navigate('dashboard') } catch (e) { notify((e as Error).message) } }
let toastTimer: ReturnType<typeof setTimeout>
function notify(message: string) { toast.value = message; clearTimeout(toastTimer); toastTimer = setTimeout(() => toast.value = '', 5000) }
function createRecord(key: string, values?: Record<string, Value>) { editRecord.value = undefined; initial.value = values; formModule.value = modules.value.find(m => m.key === key) }
function edit(record: CRMRecord) { editRecord.value = record; initial.value = undefined; formModule.value = modules.value.find(m => m.key === record.module) }
function openRecord(key: string, id: string) { const module = modules.value.find(m => m.key === key); if (module) detail.value = { module, id } }
function openNotificationRecord(event: Event) {
  const target = (event as CustomEvent<{module?: string; id?: string}>).detail
  if (target?.module && target.id) openRecord(target.module, target.id)
}
function saved(record: CRMRecord) { formModule.value = undefined; editRecord.value = undefined; revision.value++; notify('保存成功'); openRecord(record.module, record.id) }
async function settingsChanged() { await loadContext(); user.value = await api<User>('/auth/me'); revision.value++ }
function escape(event: KeyboardEvent) { if (event.key === 'Escape') { if (formModule.value) formModule.value = undefined; else detail.value = undefined; mobileMenu.value = false } }
onMounted(() => { boot(); window.addEventListener('hashchange', hashChanged); window.addEventListener('session-expired', sessionExpired); window.addEventListener('open-record', openNotificationRecord); window.addEventListener('keydown', escape) })
onUnmounted(() => { window.removeEventListener('hashchange', hashChanged); window.removeEventListener('session-expired', sessionExpired); window.removeEventListener('open-record', openNotificationRecord); window.removeEventListener('keydown', escape) })
</script>

<template>
  <div v-if="loading" class="boot-screen"><div class="logo-mark"><ChartNoAxesCombined :size="27"/></div><span class="spinner"></span><p>正在连接工作空间…</p></div>
  <div v-else-if="!user" class="auth-layout"><section class="auth-brand"><a class="brand"><span class="logo-mark"><ChartNoAxesCombined :size="25"/></span><b>Cordial<span>CRM</span></b></a><div class="auth-story"><div class="eyebrow">A BETTER WAY TO CONNECT</div><h1>好关系，<br/>让生意持续生长。</h1><p>从第一条线索，到每一次长期合作。<br/>让客户、销售与团队，在一个空间里紧密连接。</p><div class="auth-illustration"><div class="orbit orbit-one"></div><div class="orbit orbit-two"></div><div class="orbit-center"><Building2 :size="40"/></div><span class="orbit-node node-one"><Users :size="23"/></span><span class="orbit-node node-two"><Target :size="23"/></span><span class="orbit-node node-three"><FileCheck2 :size="23"/></span><span class="mini-growth"><ChartNoAxesCombined :size="22"/><div><b>让每一步进展清晰可见</b><small>客户 · 商机 · 合同 · 回款</small></div></span></div></div><footer><ShieldCheck :size="15"/>私有部署，数据由你掌握</footer></section><section class="auth-main"><div class="auth-card"><span class="auth-kicker">欢迎来到你的客户工作空间</span><h2>{{ setup ? '创建管理员账号' : '欢迎回来' }}</h2><p>{{ setup ? '首次使用，完成初始化即可开始。' : '登录 Cordial CRM，继续今天的业务。' }}</p><form @submit.prevent="authenticate"><label v-if="setup" class="field"><span>你的姓名</span><input v-model="name" required maxlength="80" placeholder="请输入姓名" autocomplete="name"/></label><label class="field"><span>用户名</span><input v-model="username" required autocomplete="username" :pattern="setup ? '[a-zA-Z0-9_.-]{3,40}' : undefined" placeholder="请输入用户名"/></label><label class="field"><span>密码</span><input v-model="password" type="password" required :minlength="setup ? 10 : 1" maxlength="200" :autocomplete="setup ? 'new-password' : 'current-password'" :placeholder="setup ? '至少 10 位，包含字母与数字' : '请输入密码'"/></label><label v-if="setup" class="checkbox-label demo-checkbox"><input v-model="demo" type="checkbox"/><span>载入虚构演示数据<small>快速体验客户、商机与回款流程，可取消勾选创建空系统。</small></span></label><p v-if="error" class="error-banner" role="alert">{{ error }}</p><button class="button primary auth-submit" :disabled="authBusy">{{ authBusy ? '正在进入…' : setup ? '创建并进入工作空间' : '登录工作空间' }}<ArrowRight :size="18"/></button></form><a v-if="!setup && weComEnabled" class="button auth-submit wecom-login" href="/api/auth/wecom/start">企业微信登录<ArrowRight :size="18"/></a><div class="auth-footnote">{{ setup ? '用户名支持 3–40 位字母、数字、点、横线和下划线。' : '忘记密码？请联系系统管理员重置。' }}</div><button v-if="error" class="text-button" @click="boot">重新检查服务连接</button></div><footer>Cordial CRM · Python + TypeScript</footer></section></div>
  <div v-else class="app-shell">
    <div v-if="mobileMenu" class="sidebar-mask" @click="mobileMenu = false"></div>
    <aside class="sidebar" :class="{ expanded: mobileMenu }"><button class="brand" @click="navigate('dashboard')"><span class="logo-mark"><ChartNoAxesCombined :size="25"/></span><b>Cordial<span>CRM</span></b></button><div class="workspace-label"><span class="workspace-avatar">C</span><div><b>企业工作空间</b><small>客户关系管理平台</small></div><ChevronDown :size="14"/></div><nav class="main-nav"><section v-for="group in nav" :key="group.label" class="nav-group"><h3>{{ group.label }}</h3><button v-for="item in group.items" :key="item.key" :class="['nav-item', { active: route === item.key }]" @click="navigate(item.key)"><component :is="icons[item.key]" :size="17" :stroke-width="1.8"/><span>{{ item.label }}</span><span v-if="route === item.key" class="active-marker"></span></button></section></nav><footer class="sidebar-footer"><span class="live-dot"></span><span>私有工作空间</span><span class="version">v0.1</span></footer></aside>
    <main class="main-shell"><header class="topbar"><div class="breadcrumb"><button class="icon-button mobile-menu" aria-label="打开菜单" @click="mobileMenu = !mobileMenu"><Menu :size="21"/></button><House :size="16"/><span>{{ currentGroup }}</span><ChevronRight :size="13"/><b>{{ currentNav?.label || (route === 'profile' ? '个人设置' : '页面') }}</b></div><div class="topbar-actions"><div class="global-search"><Search :size="15"/><input v-model="searchQuery" aria-label="搜索客户、商机、合同等" placeholder="搜索客户、商机、合同…" @input="search" @focus="searchQuery && (searchOpen = true)"/><div v-if="searchOpen" class="search-results"><span v-if="searchBusy" class="muted">正在搜索…</span><button v-for="item in searchItems" :key="item.id" @click="selectSearch(item)"><b>{{ item.data.name }}</b><small>{{ modules.find(m => m.key === item.module)?.label || item.module }}</small></button><span v-if="!searchBusy && !searchItems.length" class="muted">未找到匹配记录</span></div></div><a class="docs-link" href="/api/docs" target="_blank" rel="noreferrer"><BookOpen :size="16"/>接口文档</a><span class="topbar-divider"></span><button class="icon-button" aria-label="查看待办任务" title="待办任务" @click="navigate('tasks')"><Bell :size="18"/></button><button class="user-button" @click="navigate('profile')"><span class="user-avatar">{{ user.name.slice(-1) }}</span><span><b>{{ user.name }}</b><small>{{ roleLabel[user.role] }}</small></span></button><button class="icon-button" aria-label="退出登录" title="退出登录" @click="logout"><LogOut :size="17"/></button></div></header>
      <Analytics v-if="route === 'analytics'" :modules="modules"/>
      <Dashboard v-else-if="route === 'dashboard'" :key="'dashboard-' + revision" :user="user" :revision="revision" @navigate="navigate" @open-record="openRecord" @create="createRecord"/>
      <RecordList v-else-if="currentModule" :key="route" :module="currentModule" :user="user" :pool="route.endsWith('_pool')" :revision="revision" @open-record="openRecord" @create="createRecord"/>
      <Approvals v-else-if="route === 'approvals'" :key="'approvals-' + revision" :user="user" :modules="modules" @open-record="openRecord" @changed="revision++"/>
      <ApprovalFlows v-else-if="route === 'approval_flows' && hasPermission(user, 'PROCESS_SETTING:READ')" :key="'approval-flows-' + revision" :modules="modules" :user="user" @changed="revision++"/>
      <Settings v-else-if="settingsMode && (route === 'profile' || canNavigate(route))" :key="route" :mode="route" :user="user" :modules="modules" @changed="settingsChanged"/>
      <div v-else class="empty table-empty"><h2>页面不存在或没有访问权限</h2><button class="button primary" @click="navigate('dashboard')">返回工作台</button></div>
    </main>
    <RecordDetail v-if="detail" :key="detail.id" :revision="revision" :module="detail.module" :record-id="detail.id" :modules="modules" :user="user" @close="detail = undefined" @edit="edit" @changed="revision++" @open-record="openRecord" @create="createRecord"/>
    <RecordForm v-if="formModule" :module="formModule" :record="editRecord" :initial="initial" @close="formModule = undefined" @saved="saved"/>
    <div v-if="toast" class="toast" role="status"><CheckSquare :size="17"/>{{ toast }}<button class="icon-button" aria-label="关闭提示" @click="toast = ''"><X :size="15"/></button></div>
  </div>
</template>
