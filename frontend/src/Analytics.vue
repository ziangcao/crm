<script setup lang="ts">
import { computed, ref } from 'vue'
import { BarChart3 } from 'lucide-vue-next'
import { api, money } from './api'
import type { Module } from './types'

const props = defineProps<{ modules: Module[] }>()
const moduleKey = ref(props.modules[0]?.key || ''), groupBy = ref(''), metric = ref('count'), field = ref(''), rows = ref<{group: string; value: string}[]>([]), busy = ref(false), error = ref('')
const selectedModule = computed(() => props.modules.find(module => module.key === moduleKey.value))
const groupFields = computed(() => selectedModule.value?.fields.filter(item => !['items', 'subtable'].includes(item.type)) || [])
const metricFields = computed(() => selectedModule.value?.fields.filter(item => ['money', 'number', 'formula'].includes(item.type)) || [])
const maximum = computed(() => Math.max(1, ...rows.value.map(row => Number(row.value))))
function resetFields() { groupBy.value = groupFields.value[0]?.key || ''; field.value = metricFields.value[0]?.key || ''; rows.value = [] }
async function run() {
	if (!moduleKey.value || !groupBy.value || (metric.value === 'sum' && !field.value)) return
	busy.value = true; error.value = ''
	try { rows.value = (await api<{rows: {group: string; value: string}[]}>(`/analytics/report?module=${encodeURIComponent(moduleKey.value)}&group_by=${encodeURIComponent(groupBy.value)}&metric=${metric.value}&field=${encodeURIComponent(field.value)}`)).rows }
	catch (exception) { error.value = (exception as Error).message }
	finally { busy.value = false }
}
resetFields()
</script>

<template>
  <div class="page"><header class="page-heading"><div><div class="eyebrow">SELF-SERVICE ANALYTICS</div><h1>自助 BI</h1><p>选择业务模块、分组维度和指标，实时生成受当前权限约束的经营报表。</p></div></header>
    <section class="panel setting-form"><form class="analytics-controls" @submit.prevent="run"><label class="field"><span>业务模块</span><select v-model="moduleKey" @change="resetFields"><option v-for="module in modules" :key="module.key" :value="module.key">{{ module.label }}</option></select></label><label class="field"><span>分组维度</span><select v-model="groupBy" required><option v-for="item in groupFields" :key="item.key" :value="item.key">{{ item.label }}</option></select></label><label class="field"><span>指标</span><select v-model="metric"><option value="count">记录数量</option><option value="sum">数值汇总</option></select></label><label v-if="metric === 'sum'" class="field"><span>汇总字段</span><select v-model="field" required><option v-for="item in metricFields" :key="item.key" :value="item.key">{{ item.label }}</option></select></label><button class="button primary" :disabled="busy">{{ busy ? '生成中…' : '生成报表' }}</button></form></section>
    <p v-if="error" class="error-banner">{{ error }}</p><section class="panel data-panel"><header class="panel-heading"><div><h2>分析结果</h2><p>{{ rows.length }} 个分组</p></div><BarChart3 :size="20"/></header><div v-if="!rows.length" class="empty table-empty">选择条件后生成报表</div><div v-else class="analytics-chart"><div v-for="row in rows" :key="row.group" class="analytics-row"><b>{{ row.group }}</b><div><i :style="{ width: `${Number(row.value) / maximum * 100}%` }"></i></div><strong>{{ metric === 'sum' ? money(row.value) : Number(row.value).toFixed(0) }}</strong></div></div></section>
  </div>
</template>
