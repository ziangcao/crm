<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ArrowDownLeft, ArrowRight, ArrowUpRight, CalendarDays, Check, CheckCircle2, ChevronRight, CircleDollarSign, Clock3, Layers3, Plus, Target, TrendingUp, Users } from 'lucide-vue-next'
import { api, compactMoney, dateTime, money } from './api'
import type { CRMRecord, DashboardData, User } from './types'
import { hasModulePermission } from './permissions'

const props = defineProps<{ user: User; revision: number }>()
const emit = defineEmits<{ navigate: [key: string]; 'open-record': [module: string, id: string]; create: [key: string] }>()
const data = ref<DashboardData>(), error = ref(''), busyTask = ref('')
interface DashboardConfig { id: string; name: string; widgets: { key: string; width: 1 | 2 }[]; is_default: boolean }
const dashboardConfigs = ref<DashboardConfig[]>([]), selectedDashboardID = ref('')
const activeDashboard = computed(() => dashboardConfigs.value.find(item => item.id === selectedDashboardID.value) || dashboardConfigs.value.find(item => item.is_default))
const widgetVisible = (key: string) => !activeDashboard.value || activeDashboard.value.widgets.some(widget => widget.key === key)
const widgetStyle = (key: string) => { const index = activeDashboard.value?.widgets.findIndex(widget => widget.key === key) ?? -1; const widget = activeDashboard.value?.widgets[index]; return { order: index < 0 ? undefined : index, gridColumn: widget?.width === 2 ? '1 / -1' : undefined } }
const today = new Date().toLocaleDateString('zh-CN', { month: 'long', day: 'numeric', weekday: 'long' })
const chartMax = computed(() => Math.max(1, ...data.value?.months.flatMap(m => [Number(m.contracts), Number(m.payments)]) ?? [1]))
const funnelMax = computed(() => Math.max(1, ...data.value?.funnel.map(f => f.count) ?? [1]))
async function load() { try { const [dashboardData, configResult] = await Promise.all([api<DashboardData>('/dashboard'), api<{items: DashboardConfig[]}>('/dashboard/configs')]); data.value = dashboardData; dashboardConfigs.value = configResult.items; if (!selectedDashboardID.value) selectedDashboardID.value = dashboardConfigs.value.find(item => item.is_default)?.id || dashboardConfigs.value[0]?.id || ''; error.value = '' } catch (e) { error.value = (e as Error).message } }
async function complete(task: CRMRecord) {
  busyTask.value = task.id
  try { await api(`/records/tasks/${task.id}`, 'PUT', { version: task.version, data: { ...task.data, status: '已完成' } }); await load() }
  catch (e) { error.value = (e as Error).message }
  finally { busyTask.value = '' }
}
onMounted(load)
</script>

<template>
  <div class="page dashboard">
    <header class="page-heading"><div><div class="eyebrow">WORKSPACE OVERVIEW</div><h1>你好，{{ user.name }} <span class="greeting-dot"></span></h1><p>把握每一次连接，让增长持续发生。</p></div><div class="heading-actions"><select v-if="dashboardConfigs.length > 1" v-model="selectedDashboardID" aria-label="选择工作台"><option v-for="item in dashboardConfigs" :key="item.id" :value="item.id">{{ item.name }}{{ item.is_default ? '（默认）' : '' }}</option></select><span class="today"><CalendarDays :size="16"/>{{ today }}</span><button v-if="hasModulePermission(user, 'customers', 'ADD')" class="button primary" @click="emit('create', 'customers')"><Plus :size="17"/>新建客户</button></div></header>
    <div v-if="error" class="error-banner">{{ error }} <button class="text-button" @click="load">重试</button></div>
    <div v-if="!data && !error" class="loading-state"><span class="spinner"></span>正在汇总业务数据…</div>
    <template v-if="data">
      <div class="overview-strip"><div class="overview-copy"><span class="live-dot"></span><b>每一个好关系，都是增长的开始。</b><span>今天也向目标再靠近一步。</span></div><button @click="emit('navigate', 'opportunities')">查看销售机会 <ArrowRight :size="16"/></button></div>
      <section v-if="widgetVisible('stats')" class="stat-grid">
        <button class="stat-card" @click="emit('navigate', 'customers')"><div class="stat-top"><span>客户总数</span><span class="stat-icon blue"><Users :size="20"/></span></div><strong>{{ data.customers }}<small>家</small></strong><footer><span class="subtle">累计沉淀的客户资源</span><ArrowUpRight :size="17"/></footer></button>
        <button class="stat-card" @click="emit('navigate', 'opportunities')"><div class="stat-top"><span>在跟商机金额</span><span class="stat-icon purple"><Target :size="20"/></span></div><strong>{{ compactMoney(data.pipeline) }}</strong><footer><span class="purple-text">{{ data.opportunities }} 个商机正在推进</span><ArrowUpRight :size="17"/></footer></button>
        <button class="stat-card" @click="emit('navigate', 'contracts')"><div class="stat-top"><span>累计合同金额</span><span class="stat-icon blue"><Layers3 :size="20"/></span></div><strong>{{ compactMoney(data.contract_amount) }}</strong><footer><span class="subtle">已审批通过的合同</span><ArrowUpRight :size="17"/></footer></button>
        <button class="stat-card" @click="emit('navigate', 'payments')"><div class="stat-top"><span>累计回款金额</span><span class="stat-icon green"><CircleDollarSign :size="20"/></span></div><strong>{{ compactMoney(data.received_amount) }}</strong><footer><span class="green-text">待回款 {{ compactMoney(data.receivable) }}</span><ArrowUpRight :size="17"/></footer></button>
      </section>
      <div class="dashboard-grid">
        <section v-if="widgetVisible('performance')" class="panel performance" :style="widgetStyle('performance')"><header class="panel-heading"><div><h2>销售业绩趋势</h2><p>合同与回款 · 最近六个月</p></div><span class="period-label">近 6 个月 <CalendarDays :size="14"/></span></header><div class="chart-legend"><span><i class="legend-dot blue-bg"></i>合同金额</span><span><i class="legend-dot mint-bg"></i>回款金额</span><small>单位：元</small></div><div class="bar-chart"><div class="chart-axis"><span>{{ compactMoney(chartMax) }}</span><span>{{ compactMoney(chartMax / 2) }}</span><span>0</span></div><div class="chart-plot"><div class="chart-gridline top"></div><div class="chart-gridline middle"></div><div class="chart-gridline bottom"></div><div v-for="month in data.months" :key="month.month" class="bar-group"><div class="bars"><div class="bar contract-bar" :style="{ height: `${Math.max(Number(month.contracts) ? 2 : 0, Number(month.contracts) / chartMax * 100)}%` }" :title="'合同：' + money(month.contracts)"></div><div class="bar payment-bar" :style="{ height: `${Math.max(Number(month.payments) ? 2 : 0, Number(month.payments) / chartMax * 100)}%` }" :title="'回款：' + money(month.payments)"></div></div><span>{{ Number(month.month.slice(5)) }}月</span></div></div></div><footer class="chart-footer"><TrendingUp :size="16"/><span>金额按审批通过的合同签约日与回款到账日统计</span></footer></section>
        <section v-if="widgetVisible('funnel')" class="panel funnel" :style="widgetStyle('funnel')"><header class="panel-heading"><div><h2>销售漏斗</h2><p>关注推进中的每一个阶段</p></div><button class="icon-button" aria-label="打开商机" @click="emit('navigate', 'opportunities')"><ArrowUpRight :size="19"/></button></header><div v-for="(stage, index) in data.funnel.slice(0, 5)" :key="stage.stage" class="funnel-row"><div class="funnel-label"><span><i :style="{ background: ['#4774ec','#7599f3','#9bb8fa','#bdd0fa','#50baa2'][index] }"></i>{{ stage.stage }}</span><b>{{ stage.count }} <small>个</small></b></div><div class="funnel-track"><div :style="{ width: `${Math.max(stage.count ? 6 : 0, stage.count / funnelMax * 100)}%`, background: ['#4774ec','#7599f3','#9bb8fa','#bdd0fa','#50baa2'][index] }"></div></div></div><div class="funnel-summary"><span>加权预计成交额</span><strong>{{ compactMoney(data.weighted_pipeline) }}</strong></div></section>
        <section v-if="widgetVisible('tasks')" class="panel task-panel" :style="widgetStyle('tasks')"><header class="panel-heading"><div><h2>待办任务 <span class="count-pill">{{ data.tasks.length }}</span></h2><p>重要的事，有序推进</p></div><button v-if="hasModulePermission(user, 'tasks', 'READ')" class="text-button" @click="emit('navigate', 'tasks')">全部任务 <ChevronRight :size="15"/></button></header><div v-if="!data.tasks.length" class="empty"><CheckCircle2 :size="30"/><strong>所有任务都处理完了</strong><span>创建下一步计划，持续跟进客户。</span><button v-if="hasModulePermission(user, 'tasks', 'ADD')" class="text-button" @click="emit('create', 'tasks')">新建任务</button></div><div v-for="task in data.tasks" :key="task.id" class="task-row"><button v-if="hasModulePermission(user, 'tasks', 'UPDATE')" class="task-checkbox" :disabled="!!busyTask" aria-label="完成任务" @click="complete(task)"><Check v-if="busyTask === task.id" :size="13"/></button><button class="task-name" @click="emit('open-record', 'tasks', task.id)"><b>{{ task.data.name }}</b><span>{{ task.references.customer_id || '日常工作' }}</span></button><span :class="['badge', task.data.priority === '高' ? 'red' : 'neutral']">{{ task.data.priority }}优先级</span><span class="task-date" :class="{ overdue: String(task.data.due_date) < new Date().toLocaleDateString('sv-SE') }"><Clock3 :size="13"/>{{ String(task.data.due_date).slice(5) }}</span></div></section>
        <section v-if="widgetVisible('activities')" class="panel activity-panel" :style="widgetStyle('activities')"><header class="panel-heading"><div><h2>最近跟进</h2><p>客户关系的每一次进展</p></div><span class="tiny-label">实时动态</span></header><div v-if="!data.activities.length" class="empty"><span>暂无跟进记录</span><button class="text-button" @click="emit('navigate', 'customers')">从客户详情添加一次跟进</button></div><button v-for="(item, index) in data.activities.slice(0, 4)" :key="item.id" class="activity-row" @click="emit('open-record', item.module, item.record_id)"><span class="activity-avatar" :class="index % 2 ? 'mint' : ''">{{ item.user_name.slice(-1) }}</span><div><div class="activity-title"><b>{{ item.user_name }}</b><time>{{ dateTime(item.created_at) }}</time></div><p>通过{{ item.method }}跟进 <strong>{{ item.name }}</strong></p><span>{{ item.content }}</span></div></button></section>
      </div>
      <section v-if="widgetVisible('overdue') && data.overdue_plans.length" class="overdue-panel"><ArrowDownLeft :size="20"/><div><b>{{ data.overdue_plans.length }} 笔回款计划已逾期</b><span>及时跟进，帮助每一笔收入按计划到账。</span></div><button class="text-button" @click="emit('navigate', 'payment_plans')">查看回款计划 <ArrowRight :size="15"/></button></section>
    </template>
    <footer class="page-footer">CORDIAL CRM <span>连接客户 · 协同销售 · 持续增长</span></footer>
  </div>
</template>
