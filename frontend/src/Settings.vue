<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { Building2, ChevronLeft, ChevronRight, LockKeyhole, Plus, ShieldCheck, UserRound, X } from 'lucide-vue-next'
import { api, dateTime, roleLabel } from './api'
import type { Field, Module, PermissionDefinition, PermissionGroup, User } from './types'
import { hasPermission } from './permissions'

const props = defineProps<{ mode: string; user: User; modules: Module[] }>()
const emit = defineEmits<{ changed: [] }>()
const users = ref<User[]>([]), error = ref(''), success = ref(''), busy = ref(false), editor = ref(false), editing = ref<User>(), selectedModule = ref('customers')
const member = ref({ username: '', name: '', password: '', role: 'sales', organization_id: '', role_id: '', department: '', active: true })
const custom = ref({ key: 'custom_', label: '', type: 'text', options: '', sources: [] as string[], columns: [] as Field[] })
const currentPassword = ref(''), newPassword = ref(''), page = ref(1), auditTotal = ref(0)
interface Audit { id: string; user_name: string; action: string; module: string; detail: string; created_at: string }
const audits = ref<Audit[]>([])
interface Organization { id: string; name: string; created_at: string }
interface Department { id: string; organization_id: string; parent_id: string; name: string; sort: number }
interface FieldRestriction { hidden: string[]; read_only: string[] }
interface SystemRole { id: string; organization_id: string; name: string; built_in: boolean; permission_ids: string[]; data_scope: string; department_ids: string[]; field_restrictions?: Record<string, FieldRestriction> }
interface FieldLayout { key: string; group: string; sort: number }
interface StageConfig { name: string; type: 'AFOOT' | 'END'; rate: number }
interface StageTransition { origin: string; target: string }
interface ModuleFieldDefinition { id?: string; key: string; name: string; type: string; required: boolean; unique: boolean; read_only: boolean; options: string[]; sort?: number }
interface SystemModule { id: string; key: string; name: string; icon: string; group?: string; description?: string; enabled: boolean; shared?: boolean; approvable?: boolean; custom?: boolean; fields?: ModuleFieldDefinition[]; layout?: FieldLayout[]; document_template?: string; sales_stages?: string[]; stages?: StageConfig[]; afoot_rollback?: boolean; end_rollback?: boolean; circulation_type?: 'NORMAL' | 'ADVANCED'; stage_transitions?: StageTransition[] }
interface APIKey { id: string; name: string; prefix: string; active: boolean; created_at: string; expires_at?: string; last_used_at?: string }
interface IntegrationStatus { key: string; name: string; configured: boolean; mock: boolean; endpoint: string; has_token: boolean; managed: boolean }
interface WeComStatus { configured: boolean; oauth: boolean; managed: boolean; corp_id: string; agent_id: string; redirect_url: string; has_secret: boolean }
interface AgentDefinition { id: string; name: string; prompt: string; tool_names: string[]; enabled: boolean; created_at: string; updated_at: string }
interface AgentTool { name: string; description: string }
interface DashboardWidget { key: string; width: 1 | 2 }
interface DashboardConfig { id: string; name: string; widgets: DashboardWidget[]; is_default: boolean; created_at: string; updated_at: string }
const organizations = ref<Organization[]>([]), departments = ref<Department[]>([]), systemRoles = ref<SystemRole[]>([]), selectedOrganization = ref('')
const memberRoles = ref<SystemRole[]>([]), memberDepartments = ref<Department[]>([])
const systemModules = ref<SystemModule[]>([])
const customModuleEditor = ref(false), editingCustomModuleID = ref('')
const customModuleForm = ref({ key: 'custom_', name: '', group: '自定义业务', description: '', enabled: true, custom: true, shared: false, approvable: false, fields: [{ key: 'name', name: '名称', type: 'text', required: true, unique: false, read_only: false, options: [] }] as ModuleFieldDefinition[] })
const layoutModule = ref<SystemModule>(), layoutFields = ref<FieldLayout[]>([])
const apiKeys = ref<APIKey[]>([]), apiKeyName = ref(''), createdSecret = ref('')
const announcementSubject = ref(''), announcementContent = ref('')
type Announcement = { id: string; subject: string; content: string; created_at: string }
const announcements = ref<Announcement[]>([])
const integrations = ref<IntegrationStatus[]>([])
const editingIntegrationKey = ref(''), integrationForm = ref({ endpoint: '', token: '' })
const weCom = ref<WeComStatus>({ configured: false, oauth: false, managed: false, corp_id: '', agent_id: '', redirect_url: '', has_secret: false })
const editingWeCom = ref(false), weComForm = ref({ corp_id: '', agent_id: '', secret: '', redirect_url: '' })
const agents = ref<AgentDefinition[]>([]), agentTools = ref<AgentTool[]>([]), editingAgentID = ref('')
const agentForm = ref({ name: '', prompt: '', tool_names: [] as string[], enabled: true })
const dashboardConfigs = ref<DashboardConfig[]>([]), editingDashboardID = ref('')
const dashboardForm = ref({ name: '', widgets: [] as DashboardWidget[], is_default: true })
const dashboardWidgetLabels: Record<string, string> = { stats: '核心指标', performance: '销售业绩趋势', funnel: '销售漏斗', tasks: '待办任务', activities: '最近跟进', overdue: '逾期回款提醒' }
const permissionGroups = ref<PermissionGroup[]>([])
const organizationName = ref(''), departmentName = ref(''), departmentParentID = ref(''), departmentSort = ref(1), editingDepartmentID = ref(''), roleName = ref(''), rolePermissions = ref<string[]>([]), roleDataScope = ref('self'), roleDepartmentIDs = ref<string[]>([]), roleFieldRestrictions = ref<Record<string, FieldRestriction>>({}), editingRoleID = ref('')
const organizationImportText = ref('')
const title = computed(() => ({ users: '成员与权限', organizations: '组织与角色', dashboard_configs: '工作台配置', modules: '业务模块', fields: '自定义字段', agents: '智能助手', audit: '操作日志', profile: '个人设置' } as Record<string,string>)[props.mode])
const fields = computed(() => props.modules.find(m => m.key === selectedModule.value)?.fields || [])
const departmentTree = computed(() => {
  const children = new Map<string, Department[]>()
  departments.value.forEach(item => children.set(item.parent_id, [...(children.get(item.parent_id) || []), item]))
  const result: Array<Department & { depth: number }> = []
  const append = (parentID: string, depth: number) => (children.get(parentID) || []).sort((left, right) => left.sort - right.sort || left.name.localeCompare(right.name, 'zh-CN')).forEach(item => { result.push({ ...item, depth }); append(item.id, depth + 1) })
  append('', 0)
  return result
})
const departmentParentOptions = computed(() => {
  if (!editingDepartmentID.value) return departmentTree.value
  const excluded = new Set([editingDepartmentID.value])
  let changed = true
  while (changed) {
    changed = false
    departments.value.forEach(item => { if (excluded.has(item.parent_id) && !excluded.has(item.id)) { excluded.add(item.id); changed = true } })
  }
  return departmentTree.value.filter(item => !excluded.has(item.id))
})
const can = (permissionID: string) => hasPermission(props.user, permissionID)
async function load() {
  error.value = ''
  try {
    if (props.mode === 'users') { users.value = await api<User[]>('/users'); organizations.value = (await api<{items: Organization[]}>('/system/organizations')).items; memberRoles.value = (await Promise.all(organizations.value.map(item => api<{items: SystemRole[]}>(`/system/organizations/${item.id}/roles`)))).flatMap(result => result.items || []) }
    if (props.mode === 'audit') { const result = await api<{items: Audit[]; total: number}>('/audit?page=' + page.value); audits.value = result.items; auditTotal.value = result.total }
		if (props.mode === 'organizations') { const [organizationResult, permissionResult] = await Promise.all([api<{items: Organization[]}>('/system/organizations'), api<{groups: PermissionGroup[]}>('/system/permissions')]); organizations.value = organizationResult.items; permissionGroups.value = permissionResult.groups; if (!selectedOrganization.value && organizations.value.length) selectedOrganization.value = organizations.value[0].id; await loadOrganization() }
		if (props.mode === 'modules') { systemModules.value = (await api<{modules: SystemModule[]}>('/system/modules')).modules; if (can('SYSTEM_SETTING:READ')) { const [integrationResult, weComResult] = await Promise.all([api<{items: IntegrationStatus[]}>('/integrations'), api<WeComStatus>('/integrations/wecom/config')]); integrations.value = integrationResult.items; weCom.value = weComResult } else integrations.value = []; announcements.value = can('SYSTEM_NOTICE:READ') && props.user.organization_id ? (await api<{items: Announcement[]}>('/notifications/announcements')).items : [] }
		if (props.mode === 'profile') apiKeys.value = can('PERSONAL_API_KEY:READ') ? (await api<{items: APIKey[]}>('/auth/api-keys')).items : []
		if (props.mode === 'agents') { const [agentResult, toolResult] = await Promise.all([api<{items: AgentDefinition[]}>('/agents'), api<{tools: AgentTool[]}>('/agent/tools')]); agents.value = agentResult.items; agentTools.value = toolResult.tools }
		if (props.mode === 'dashboard_configs') dashboardConfigs.value = (await api<{items: DashboardConfig[]}>('/dashboard/configs')).items
  } catch (e) { error.value = (e as Error).message }
}
watch(() => props.mode, () => { success.value = ''; editor.value = false; load() })
onMounted(load)
async function loadMemberRoles() {
  if (!member.value.organization_id) { systemRoles.value = []; member.value.role_id = ''; return }
  systemRoles.value = memberRoles.value.filter(role => role.organization_id === member.value.organization_id)
  if (!systemRoles.value.some(role => role.id === member.value.role_id)) member.value.role_id = ''
}
async function loadMemberDepartments() {
  if (!member.value.organization_id) { memberDepartments.value = []; member.value.department = ''; return }
  try {
    memberDepartments.value = (await api<{items: Department[]}>(`/system/organizations/${member.value.organization_id}/departments`)).items || []
    if (member.value.department && !memberDepartments.value.some(item => item.id === member.value.department)) member.value.department = ''
  } catch (e) { error.value = (e as Error).message }
}
async function edit(user?: User) {
  editing.value = user; error.value = ''; member.value = user ? { username: user.username, name: user.name, password: '', role: user.role, organization_id: user.organization_id || '', role_id: user.role_id || '', department: user.department, active: !!user.active } : { username: '', name: '', password: '', role: 'sales', organization_id: '', role_id: '', department: '', active: true }; await Promise.all([loadMemberRoles(), loadMemberDepartments()]); editor.value = true
}
async function saveUser() {
  busy.value = true; error.value = ''
  try {
    if (editing.value) { const { username, ...data } = member.value; await api('/users/' + editing.value.id, 'PUT', { ...data, password: data.password || null }) }
    else { const { active, ...data } = member.value; await api('/users', 'POST', data) }
    editor.value = false; success.value = '成员信息已保存'; await load(); emit('changed')
  } catch (e) { error.value = (e as Error).message }
  finally { busy.value = false }
}
async function syncWeComUsers() {
	busy.value = true; error.value = ''
	try { const result = await api<{count: number}>('/auth/wecom/sync', 'POST'); await load(); success.value = `已同步 ${result.count} 名企业微信成员`; emit('changed') }
	catch (e) { error.value = (e as Error).message }
	finally { busy.value = false }
}
async function saveField() {
  busy.value = true; error.value = ''
  try { await api('/catalog/' + selectedModule.value + '/fields', 'POST', { ...custom.value, options: custom.value.options.split('\n').map(s => s.trim()).filter(Boolean) }); success.value = '字段已添加，业务表单和详情页将自动显示'; custom.value = { key: 'custom_', label: '', type: 'text', options: '', sources: [], columns: [] }; emit('changed') }
  catch (e) { error.value = (e as Error).message }
  finally { busy.value = false }
}
async function deleteField(field: Field) {
  if (!confirm(`停用字段“${field.label}”？字段将从表单中隐藏，历史记录中的已有数据会保留。`)) return
  busy.value = true; error.value = ''
  try { await api(`/catalog/${selectedModule.value}/fields/${field.key}`, 'DELETE'); success.value = '字段已停用'; emit('changed') }
  catch (e) { error.value = (e as Error).message }
  finally { busy.value = false }
}
async function renameField(field: Field) {
  const label = window.prompt('字段名称', field.label)?.trim()
  if (!label || label === field.label) return
  busy.value = true; error.value = ''
  try { await api(`/catalog/${selectedModule.value}/fields/${field.key}`, 'PUT', { label }); success.value = '字段名称已更新'; emit('changed') }
  catch (e) { error.value = (e as Error).message }
  finally { busy.value = false }
}
async function savePassword() {
  busy.value = true; error.value = ''
  try { await api('/auth/password', 'POST', { current_password: currentPassword.value, new_password: newPassword.value }); currentPassword.value = ''; newPassword.value = ''; success.value = '密码已更新，其他设备的会话已失效' }
  catch (e) { error.value = (e as Error).message }
  finally { busy.value = false }
}
async function createAPIKey() {
  if (!apiKeyName.value.trim()) return
  busy.value = true; error.value = ''; createdSecret.value = ''
  try { const result = await api<APIKey & {secret: string}>('/auth/api-keys', 'POST', { name: apiKeyName.value }); apiKeyName.value = ''; createdSecret.value = result.secret; await load(); success.value = 'API Key 已创建，请立即妥善保存密钥' }
  catch (e) { error.value = (e as Error).message }
  finally { busy.value = false }
}
async function toggleAPIKey(item: APIKey) {
  busy.value = true; error.value = ''
  try { await api(`/auth/api-keys/${item.id}`, 'PUT', { name: item.name, active: !item.active, expires_at: item.expires_at || null }); await load(); success.value = `${item.name}已${item.active ? '停用' : '启用'}` }
  catch (e) { error.value = (e as Error).message }
  finally { busy.value = false }
}
async function deleteAPIKey(item: APIKey) {
  if (!confirm(`撤销 API Key“${item.name}”？使用此密钥的 Agent 将立即失去访问权限。`)) return
  busy.value = true; error.value = ''
  try { await api(`/auth/api-keys/${item.id}`, 'DELETE'); await load(); success.value = 'API Key 已撤销' }
  catch (e) { error.value = (e as Error).message }
  finally { busy.value = false }
}
async function loadOrganization() {
  if (!selectedOrganization.value) { departments.value = []; systemRoles.value = []; return }
  const [departmentResult, roleResult] = await Promise.all([
    api<{items: Department[]}>(`/system/organizations/${selectedOrganization.value}/departments`),
    api<{items: SystemRole[]}>(`/system/organizations/${selectedOrganization.value}/roles`),
  ])
  departments.value = departmentResult.items || []; systemRoles.value = (roleResult.items || []).map(item => ({ ...item, permission_ids: item.permission_ids || [] }))
}
async function createOrganization() {
  if (!organizationName.value.trim()) return
  busy.value = true; error.value = ''
  try { const item = await api<Organization>('/system/organizations', 'POST', { name: organizationName.value }); organizations.value.push(item); selectedOrganization.value = item.id; organizationName.value = ''; await loadOrganization(); success.value = '组织已创建'; emit('changed') }
  catch (e) { error.value = (e as Error).message }
  finally { busy.value = false }
}
async function renameOrganization(item: Organization) {
	const name = window.prompt('组织名称', item.name)?.trim()
	if (!name || name === item.name) return
	busy.value = true; error.value = ''; success.value = ''
	try { const updated = await api<Organization>(`/system/organizations/${item.id}`, 'PUT', { name }); organizations.value = organizations.value.map(current => current.id === updated.id ? updated : current); success.value = '组织名称已更新'; emit('changed') }
	catch (e) { error.value = (e as Error).message }
	finally { busy.value = false }
}
async function deleteOrganization(item: Organization) {
	if (!window.confirm(`删除组织“${item.name}”？仅空组织可以删除。`)) return
	busy.value = true; error.value = ''; success.value = ''
	try { await api(`/system/organizations/${item.id}`, 'DELETE'); organizations.value = organizations.value.filter(current => current.id !== item.id); if (selectedOrganization.value === item.id) { selectedOrganization.value = organizations.value[0]?.id || ''; await loadOrganization() }; success.value = '组织已删除'; emit('changed') }
	catch (e) { error.value = (e as Error).message }
	finally { busy.value = false }
}
async function importOrganizations() {
	busy.value = true; error.value = ''; success.value = ''
	try {
		const rows = JSON.parse(organizationImportText.value)
		if (!Array.isArray(rows)) throw new Error('组织导入内容必须是 JSON 数组')
		const result = await api<{organizations: number; departments: number}>('/system/organizations/import', 'POST', { rows })
		organizationImportText.value = ''; organizations.value = (await api<{items: Organization[]}>('/system/organizations')).items; if (!selectedOrganization.value && organizations.value.length) selectedOrganization.value = organizations.value[0].id; await loadOrganization(); success.value = `导入完成：新增 ${result.organizations} 个组织、${result.departments} 个部门`; emit('changed')
	} catch (e) { error.value = (e as Error).message }
	finally { busy.value = false }
}
function editDepartment(item: Department) {
  editingDepartmentID.value = item.id; departmentName.value = item.name; departmentParentID.value = item.parent_id; departmentSort.value = item.sort
}
function cancelDepartmentEdit() {
  editingDepartmentID.value = ''; departmentName.value = ''; departmentParentID.value = ''; departmentSort.value = departments.value.length + 1
}
async function saveDepartment() {
  if (!selectedOrganization.value || !departmentName.value.trim()) return
  busy.value = true; error.value = ''
  try { const path = editingDepartmentID.value ? `/system/departments/${editingDepartmentID.value}` : '/system/departments'; await api(path, editingDepartmentID.value ? 'PUT' : 'POST', { organization_id: selectedOrganization.value, parent_id: departmentParentID.value, name: departmentName.value, sort: departmentSort.value }); cancelDepartmentEdit(); await loadOrganization(); success.value = '部门已保存' }
  catch (e) { error.value = (e as Error).message }
  finally { busy.value = false }
}
async function deleteDepartment(item: Department) {
  if (!confirm(`删除部门“${item.name}”？仅没有下级部门、成员和角色范围引用时可以删除。`)) return
  busy.value = true; error.value = ''
  try { await api(`/system/departments/${item.id}?organization_id=${encodeURIComponent(item.organization_id)}`, 'DELETE'); if (editingDepartmentID.value === item.id) cancelDepartmentEdit(); await loadOrganization(); success.value = '部门已删除' }
  catch (e) { error.value = (e as Error).message }
  finally { busy.value = false }
}
async function createSystemRole() {
  if (!selectedOrganization.value || !roleName.value.trim()) return
  busy.value = true; error.value = ''
  try { await api('/system/roles', 'POST', { id: editingRoleID.value, organization_id: selectedOrganization.value, name: roleName.value, permission_ids: rolePermissions.value, data_scope: roleDataScope.value, department_ids: roleDepartmentIDs.value, field_restrictions: roleFieldRestrictions.value }); roleName.value = ''; rolePermissions.value = []; roleDataScope.value = 'self'; roleDepartmentIDs.value = []; resetRoleFieldRestrictions(); editingRoleID.value = ''; await loadOrganization(); success.value = '角色已保存' }
  catch (e) { error.value = (e as Error).message }
  finally { busy.value = false }
}
function resetRoleFieldRestrictions(source: Record<string, FieldRestriction> = {}) { roleFieldRestrictions.value = Object.fromEntries(props.modules.map(module => { const keys = new Set(module.fields.map(field => field.key)); return [module.key, { hidden: (source[module.key]?.hidden || []).filter(key => keys.has(key)), read_only: (source[module.key]?.read_only || []).filter(key => keys.has(key)) }] })) }
function editSystemRole(item: SystemRole) { editingRoleID.value = item.id; roleName.value = item.name; rolePermissions.value = [...item.permission_ids]; roleDataScope.value = item.data_scope || 'self'; roleDepartmentIDs.value = [...(item.department_ids || [])]; resetRoleFieldRestrictions(item.field_restrictions) }
async function deleteSystemRole(item: SystemRole) {
	if (!confirm(`删除角色“${item.name}”？仍有成员使用时将无法删除。`)) return
	busy.value = true; error.value = ''
	try { await api(`/system/roles/${item.id}?organization_id=${encodeURIComponent(item.organization_id)}`, 'DELETE'); await loadOrganization(); success.value = '角色已删除' }
	catch (e) { error.value = (e as Error).message }
	finally { busy.value = false }
}
function toggleRolePermission(permission: PermissionDefinition) {
	if (permission.action === 'READ' && !rolePermissions.value.includes(permission.id)) rolePermissions.value = rolePermissions.value.filter(id => !id.startsWith(permission.resource + ':'))
	if (permission.action !== 'READ' && rolePermissions.value.includes(permission.id)) {
		const readID = `${permission.resource}:READ`
		if (permissionGroups.value.some(group => group.permissions.some(item => item.id === readID)) && !rolePermissions.value.includes(readID)) rolePermissions.value.push(readID)
	}
}
resetRoleFieldRestrictions()
async function toggleModule(item: SystemModule) {
  busy.value = true; error.value = ''
  try { await api('/system/modules', 'POST', { ...item, enabled: !item.enabled }); await load(); success.value = `${item.name}已${item.enabled ? '停用' : '启用'}`; emit('changed') }
  catch (e) { error.value = (e as Error).message }
  finally { busy.value = false }
}
function editCustomModule(item?: SystemModule) {
  customModuleEditor.value = true; editingCustomModuleID.value = item?.id || ''
  customModuleForm.value = item ? { key: item.key, name: item.name, group: item.group || '自定义业务', description: item.description || '', enabled: item.enabled, custom: true, shared: !!item.shared, approvable: !!item.approvable, fields: (item.fields || []).map(field => ({ ...field, options: [...(field.options || [])] })) } : { key: 'custom_', name: '', group: '自定义业务', description: '', enabled: true, custom: true, shared: false, approvable: false, fields: [{ key: 'name', name: '名称', type: 'text', required: true, unique: false, read_only: false, options: [] }] }
}
function addCustomModuleField() { customModuleForm.value.fields.push({ key: '', name: '', type: 'text', required: false, unique: false, read_only: false, options: [] }) }
async function saveCustomModule() {
  busy.value = true; error.value = ''
  try { await api('/system/modules', 'POST', { ...customModuleForm.value, id: editingCustomModuleID.value, fields: customModuleForm.value.fields.map((field, index) => ({ ...field, options: field.options.filter(Boolean), sort: index + 1 })) }); customModuleEditor.value = false; await load(); success.value = '自定义模块已保存'; emit('changed') }
  catch (e) { error.value = (e as Error).message }
  finally { busy.value = false }
}
async function deleteCustomModule(item: SystemModule) {
  if (!confirm(`删除自定义模块“${item.name}”？仅没有业务数据时可以删除。`)) return
  busy.value = true; error.value = ''
  try { await api(`/system/modules/${item.id}?key=${encodeURIComponent(item.key)}`, 'DELETE'); await load(); success.value = '自定义模块已删除'; emit('changed') }
  catch (e) { error.value = (e as Error).message }
  finally { busy.value = false }
}
function editLayout(item: SystemModule) {
  const module = props.modules.find(value => value.key === item.key)
  if (!module) return
  const saved = new Map((item.layout || []).map(field => [field.key, field]))
	layoutFields.value = [...module.fields].sort((left, right) => (saved.get(left.key)?.sort || Number.MAX_SAFE_INTEGER) - (saved.get(right.key)?.sort || Number.MAX_SAFE_INTEGER)).map((field, index) => ({ key: field.key, group: saved.get(field.key)?.group || '基本信息', sort: index + 1 }))
	const defaultStages = module.fields.find(field => field.key === 'stage')?.options || []
	const stages = item.stages?.length ? item.stages : defaultStages.map((name, index) => ({ name, type: ((['赢单', '输单', '履行完毕', '作废', '归档'].includes(name) || index === defaultStages.length - 1) ? 'END' : 'AFOOT') as 'AFOOT' | 'END', rate: name === '赢单' ? 100 : name === '输单' ? 0 : Math.round(index / Math.max(1, defaultStages.length - 1) * 100) }))
	layoutModule.value = { ...item, stages: stages.map(stage => ({ ...stage })), afoot_rollback: item.afoot_rollback ?? true, end_rollback: item.end_rollback ?? false, circulation_type: item.circulation_type || 'NORMAL', stage_transitions: (item.stage_transitions || []).map(transition => ({ ...transition })) }
}
function moveLayoutField(index: number, offset: number) {
  const target = index + offset
  if (target < 0 || target >= layoutFields.value.length) return
  const current = layoutFields.value[index]
  layoutFields.value[index] = layoutFields.value[target]
  layoutFields.value[target] = current
  layoutFields.value.forEach((field, fieldIndex) => { field.sort = fieldIndex + 1 })
}
function addStage() {
  if (!layoutModule.value || (layoutModule.value.stages?.length || 0) >= 15) return
  layoutModule.value.stages ||= []
  layoutModule.value.stages.push({ name: '', type: 'AFOOT', rate: 50 })
}
function moveStage(index: number, offset: number) {
  const stages = layoutModule.value?.stages
  if (!stages || index + offset < 0 || index + offset >= stages.length) return
  const target = index + offset; [stages[index], stages[target]] = [stages[target], stages[index]]
}
function removeStage(index: number) {
  const module = layoutModule.value
  if (!module?.stages || module.stages.length <= 2) return
  const name = module.stages[index].name
  module.stages.splice(index, 1)
  module.stage_transitions = (module.stage_transitions || []).filter(item => item.origin !== name && item.target !== name)
}
function transitionEnabled(origin: string, target: string) { return !!layoutModule.value?.stage_transitions?.some(item => item.origin === origin && item.target === target) }
function toggleStageTransition(origin: string, target: string, enabled: boolean) {
  if (!layoutModule.value || origin === target) return
  const transitions = layoutModule.value.stage_transitions || []
  layoutModule.value.stage_transitions = enabled ? [...transitions, { origin, target }] : transitions.filter(item => item.origin !== origin || item.target !== target)
}
async function saveLayout() {
  if (!layoutModule.value) return
  busy.value = true; error.value = ''
	try { const stages = ['opportunities', 'contracts'].includes(layoutModule.value.key) ? layoutModule.value.stages : undefined; await api('/system/modules', 'POST', { ...layoutModule.value, layout: layoutFields.value, stages, sales_stages: stages?.map(stage => stage.name) }); await load(); layoutModule.value = undefined; success.value = '模块配置已保存'; emit('changed') }
  catch (e) { error.value = (e as Error).message }
  finally { busy.value = false }
}
async function publishAnnouncement() {
	if (!announcementSubject.value.trim() || !announcementContent.value.trim()) return
	busy.value = true; error.value = ''
	try { const result = await api<{count: number}>('/notifications/announcements', 'POST', { subject: announcementSubject.value, content: announcementContent.value }); announcementSubject.value = ''; announcementContent.value = ''; if (can('SYSTEM_NOTICE:READ')) announcements.value = (await api<{items: Announcement[]}>('/notifications/announcements')).items; success.value = `公告已发送给 ${result.count} 名成员` }
	catch (e) { error.value = (e as Error).message }
	finally { busy.value = false }
}
async function editAnnouncement(item: Announcement) {
	const subject = window.prompt('公告标题', item.subject)?.trim()
	if (!subject) return
	const content = window.prompt('公告内容', item.content)?.trim()
	if (!content) return
	busy.value = true; error.value = ''; success.value = ''
	try { await api(`/notifications/announcements/${item.id}`, 'PUT', { subject, content }); item.subject = subject; item.content = content; success.value = '公告已更新' }
	catch (e) { error.value = (e as Error).message }
	finally { busy.value = false }
}
async function deleteAnnouncement(item: Announcement) {
	if (!window.confirm(`删除公告“${item.subject}”？`)) return
	busy.value = true; error.value = ''; success.value = ''
	try { await api(`/notifications/announcements/${item.id}`, 'DELETE'); announcements.value = announcements.value.filter(current => current.id !== item.id); success.value = '公告已删除' }
	catch (e) { error.value = (e as Error).message }
	finally { busy.value = false }
}
function editAgent(item?: AgentDefinition) {
  editingAgentID.value = item?.id || ''
  agentForm.value = item ? { name: item.name, prompt: item.prompt, tool_names: [...item.tool_names], enabled: item.enabled } : { name: '', prompt: '', tool_names: [], enabled: true }
}
async function saveAgent() {
  busy.value = true; error.value = ''
  try { const path = editingAgentID.value ? `/agents/${editingAgentID.value}` : '/agents'; await api(path, editingAgentID.value ? 'PUT' : 'POST', agentForm.value); editAgent(); await load(); success.value = 'Agent 配置已保存' }
  catch (e) { error.value = (e as Error).message }
  finally { busy.value = false }
}
async function toggleAgent(item: AgentDefinition) {
  busy.value = true; error.value = ''
  try { await api(`/agents/${item.id}`, 'PUT', { name: item.name, prompt: item.prompt, tool_names: item.tool_names, enabled: !item.enabled }); await load(); success.value = `Agent 已${item.enabled ? '停用' : '启用'}` }
  catch (e) { error.value = (e as Error).message }
  finally { busy.value = false }
}
async function deleteAgent(item: AgentDefinition) {
  if (!confirm(`删除 Agent“${item.name}”？`)) return
  busy.value = true; error.value = ''
  try { await api(`/agents/${item.id}`, 'DELETE'); if (editingAgentID.value === item.id) editAgent(); await load(); success.value = 'Agent 已删除' }
  catch (e) { error.value = (e as Error).message }
  finally { busy.value = false }
}
function editDashboard(item?: DashboardConfig) {
  editingDashboardID.value = item?.id || ''
  dashboardForm.value = item ? { name: item.name, widgets: item.widgets.map(widget => ({ ...widget })), is_default: item.is_default } : { name: '', widgets: Object.keys(dashboardWidgetLabels).map(key => ({ key, width: key === 'stats' || key === 'overdue' ? 2 : 1 } as DashboardWidget)), is_default: true }
}
function moveDashboardWidget(index: number, offset: number) {
  const target = index + offset
  if (target < 0 || target >= dashboardForm.value.widgets.length) return
  const current = dashboardForm.value.widgets[index]; dashboardForm.value.widgets[index] = dashboardForm.value.widgets[target]; dashboardForm.value.widgets[target] = current
}
function addDashboardWidget(key: string) { if (!dashboardForm.value.widgets.some(widget => widget.key === key)) dashboardForm.value.widgets.push({ key, width: 1 }) }
async function saveDashboardConfig() {
  busy.value = true; error.value = ''
  try { const path = editingDashboardID.value ? `/dashboard/configs/${editingDashboardID.value}` : '/dashboard/configs'; await api(path, editingDashboardID.value ? 'PUT' : 'POST', dashboardForm.value); editingDashboardID.value = ''; await load(); success.value = '工作台配置已保存'; emit('changed') }
  catch (e) { error.value = (e as Error).message }
  finally { busy.value = false }
}
async function deleteDashboardConfig(item: DashboardConfig) {
  if (!confirm(`删除工作台“${item.name}”？`)) return
  busy.value = true; error.value = ''
  try { await api(`/dashboard/configs/${item.id}`, 'DELETE'); if (editingDashboardID.value === item.id) editingDashboardID.value = ''; await load(); success.value = '工作台配置已删除'; emit('changed') }
  catch (e) { error.value = (e as Error).message }
  finally { busy.value = false }
}
async function testIntegration(item: IntegrationStatus) {
	busy.value = true; error.value = ''
	try { await api(`/integrations/${item.key}/test`, 'POST'); success.value = `${item.name}${item.mock ? '模拟' : ''}连接测试通过` }
	catch (e) { error.value = (e as Error).message }
	finally { busy.value = false }
}
function editIntegration(item: IntegrationStatus) { editingIntegrationKey.value = item.key; integrationForm.value = { endpoint: item.endpoint || '', token: '' } }
async function saveIntegration() {
  if (!editingIntegrationKey.value) return
  busy.value = true; error.value = ''
  try { await api(`/integrations/${editingIntegrationKey.value}`, 'PUT', integrationForm.value); editingIntegrationKey.value = ''; integrationForm.value = { endpoint: '', token: '' }; await load(); success.value = '连接器配置已保存，Token 已加密存储' }
  catch (e) { error.value = (e as Error).message }
  finally { busy.value = false }
}
async function deleteIntegration(item: IntegrationStatus) {
  if (!confirm(`删除“${item.name}”的可视化配置？删除后将恢复环境变量配置。`)) return
  busy.value = true; error.value = ''
  try { await api(`/integrations/${item.key}`, 'DELETE'); if (editingIntegrationKey.value === item.key) editingIntegrationKey.value = ''; await load(); success.value = '可视化配置已删除' }
  catch (e) { error.value = (e as Error).message }
  finally { busy.value = false }
}
function editWeCom() {
  weComForm.value = { corp_id: weCom.value.corp_id, agent_id: weCom.value.agent_id, secret: '', redirect_url: weCom.value.redirect_url }
  editingWeCom.value = true
}
async function saveWeCom() {
  busy.value = true; error.value = ''
  try { await api('/integrations/wecom/config', 'PUT', weComForm.value); editingWeCom.value = false; await load(); success.value = '企业微信配置已保存并即时生效' }
  catch (e) { error.value = (e as Error).message }
  finally { busy.value = false }
}
async function testWeCom() {
  busy.value = true; error.value = ''
  try { await api('/integrations/wecom/test', 'POST'); success.value = '企业微信连接测试通过' }
  catch (e) { error.value = (e as Error).message }
  finally { busy.value = false }
}
async function deleteWeCom() {
  if (!confirm('删除后台企业微信配置？如已设置环境变量，将自动恢复环境变量配置。')) return
  busy.value = true; error.value = ''
  try { await api('/integrations/wecom/config', 'DELETE'); editingWeCom.value = false; await load(); success.value = '后台配置已删除' }
  catch (e) { error.value = (e as Error).message }
  finally { busy.value = false }
}
const typeName = (type: string) => ({ text: '单行文本', textarea: '多行文本', select: '下拉选择', number: '数字', money: '金额', formula: '公式求和', subtable: '子表', date: '日期', reference: '关联记录', items: '产品明细', email: '邮箱' } as Record<string,string>)[type]
function addSubtableColumn() { custom.value.columns.push({ key: '', label: '', type: 'text', required: false }) }
</script>

<template>
  <div class="page"><header class="page-heading"><div><div class="eyebrow">{{ mode === 'profile' ? 'MY ACCOUNT' : 'SYSTEM SETTINGS' }}</div><h1>{{ title }}</h1><p>清晰的规则，让团队更高效地协作</p></div><div v-if="mode === 'users'" class="heading-actions"><button v-if="can('SYS_ORGANIZATION:SYNC')" class="button" :disabled="busy" @click="syncWeComUsers">同步企业微信通讯录</button><button v-if="can('SYSTEM_ROLE:ADD_USER')" class="button primary" @click="edit()"><Plus :size="16"/>添加成员</button></div></header><p v-if="error && !editor" class="error-banner">{{ error }}</p><div v-if="success" class="success-banner">{{ success }}<button class="icon-button" aria-label="关闭提示" @click="success = ''"><X :size="15"/></button></div>
    <template v-if="mode === 'users'"><div class="permission-guide"><ShieldCheck :size="22"/><div><b>基础角色与自定义角色</b><p>管理员：系统配置与全部数据；销售经理：全部业务数据与审批；销售成员：自己的业务数据、产品目录和公海资源。绑定组织和自定义角色后，再按模块权限授权。</p></div></div><section class="panel table-scroll"><table><thead><tr><th>成员</th><th>用户名</th><th>部门</th><th>角色</th><th>状态</th><th>操作</th></tr></thead><tbody><tr v-for="u in users" :key="u.id"><td><span class="owner-cell"><i>{{ u.name.slice(-1) }}</i><b>{{ u.name }}</b></span></td><td>{{ u.username }}</td><td>{{ u.department || '未分配' }}</td><td>{{ u.role_id ? (memberRoles.find(role => role.id === u.role_id)?.name || u.role_id) : roleLabel[u.role] }}</td><td><span :class="['badge', u.active ? 'green' : 'neutral']">{{ u.active ? '启用' : '停用' }}</span></td><td><button v-if="can('SYSTEM_ROLE:ADD_USER') || can('SYSTEM_ROLE:REMOVE_USER') || can('SYS_ORGANIZATION_USER:RESET_PASSWORD')" class="text-button" @click="edit(u)">编辑</button></td></tr></tbody></table></section></template>
    <template v-if="mode === 'organizations'">
      <div class="permission-guide"><Building2 :size="22"/><div><b>组织、部门与业务角色</b><p>先选择组织，再维护其部门和角色。成员部门与角色的数据范围均基于此组织结构。</p></div></div>
      <div class="settings-grid">
        <section class="panel setting-form"><h2>组织</h2><div class="field-list"><div v-for="item in organizations" :key="item.id"><button class="text-button" :class="{ 'green-text': selectedOrganization === item.id }" @click="selectedOrganization = item.id; loadOrganization()">{{ item.name }}</button><span><button v-if="can('SYS_ORGANIZATION:UPDATE')" class="text-button" :disabled="busy" @click="renameOrganization(item)">重命名</button><button v-if="can('SYS_ORGANIZATION:DELETE')" class="text-button danger" :disabled="busy" @click="deleteOrganization(item)">删除</button></span></div><p v-if="!organizations.length" class="muted">暂无组织</p></div><form v-if="can('SYS_ORGANIZATION:ADD')" @submit.prevent="createOrganization"><label class="field"><span>新组织名称</span><input v-model="organizationName" maxlength="100" required placeholder="例如：华东销售中心"/></label><button class="button primary" :disabled="busy"><Plus :size="16"/>创建组织</button></form><form v-if="can('SYS_ORGANIZATION:IMPORT')" @submit.prevent="importOrganizations"><label class="field"><span>批量导入组织架构</span><textarea v-model="organizationImportText" class="code-input" rows="6" required placeholder='[{ "organization": "华东中心", "department": "销售部", "parent_department": "", "sort": 1 }]'/><small>最多 500 行；子部门可以写在上级部门之前，任一行失败时整批回滚。</small></label><button class="button" :disabled="busy || !organizationImportText.trim()">导入组织架构</button></form></section>
        <section class="panel setting-form"><h2>部门</h2><p class="muted">{{ organizations.find(item => item.id === selectedOrganization)?.name || '请先创建或选择组织' }}</p><div class="field-list"><div v-for="item in departmentTree" :key="item.id"><span><b>{{ '　'.repeat(item.depth) }}{{ item.depth ? '└ ' : '' }}{{ item.name }}</b><small>排序 {{ item.sort }}</small></span><span><button v-if="can('SYS_ORGANIZATION:UPDATE')" class="text-button" :disabled="busy" @click="editDepartment(item)">编辑/移动</button><button v-if="can('SYS_ORGANIZATION:DELETE')" class="text-button danger" :disabled="busy" @click="deleteDepartment(item)">删除</button></span></div><p v-if="selectedOrganization && !departments.length" class="muted">暂无部门</p></div><form v-if="editingDepartmentID ? can('SYS_ORGANIZATION:UPDATE') : can('SYS_ORGANIZATION:ADD')" @submit.prevent="saveDepartment"><label class="field"><span>部门名称</span><input v-model="departmentName" maxlength="100" :disabled="!selectedOrganization" required placeholder="例如：销售一部"/></label><label class="field"><span>上级部门</span><select v-model="departmentParentID" :disabled="!selectedOrganization"><option value="">无（一级部门）</option><option v-for="item in departmentParentOptions" :key="item.id" :value="item.id">{{ '　'.repeat(item.depth) }}{{ item.name }}</option></select></label><label class="field"><span>排序</span><input v-model.number="departmentSort" type="number" min="0" required/></label><button class="button primary" :disabled="busy || !selectedOrganization"><Plus :size="16"/>{{ editingDepartmentID ? '保存部门' : '新增部门' }}</button><button v-if="editingDepartmentID" type="button" class="button" :disabled="busy" @click="cancelDepartmentEdit">取消</button></form></section>
      </div>
      <section class="panel setting-form"><h2>角色</h2><p class="muted">按上游完整权限点授权并设置该角色可查看的记录范围，成员绑定后立即生效。</p><div class="field-list"><div v-for="item in systemRoles" :key="item.id"><span><b>{{ item.name }}</b><small>{{ item.permission_ids.length ? item.permission_ids.join('、') : '尚未配置权限' }} · {{ { all: '全部数据', self: '仅本人', department: '本部门', department_and_children: '本部门及下级', specified_departments: '指定部门' }[item.data_scope || 'self'] }}</small></span><button v-if="can('SYSTEM_ROLE:UPDATE')" class="text-button" @click="editSystemRole(item)">编辑</button><button v-if="can('SYSTEM_ROLE:DELETE') && !item.built_in" class="text-button danger" :disabled="busy" @click="deleteSystemRole(item)">删除</button></div><p v-if="selectedOrganization && !systemRoles.length" class="muted">暂无角色</p></div><form v-if="editingRoleID ? can('SYSTEM_ROLE:UPDATE') : can('SYSTEM_ROLE:ADD')" @submit.prevent="createSystemRole"><label class="field"><span>角色名称</span><input v-model="roleName" maxlength="100" :disabled="!selectedOrganization" required placeholder="例如：区域销售经理"/></label><label class="field"><span>数据范围</span><select v-model="roleDataScope" :disabled="!selectedOrganization"><option value="self">仅本人</option><option value="department">本部门</option><option value="department_and_children">本部门及下级部门</option><option value="specified_departments">指定部门</option><option value="all">全部数据</option></select></label><div v-if="roleDataScope === 'specified_departments'" class="field-list"><p v-if="!departments.length" class="muted">请先创建部门后再配置指定部门范围。</p><label v-for="item in departments" :key="item.id" class="checkbox-label"><input v-model="roleDepartmentIDs" type="checkbox" :value="item.id"/>{{ item.name }}</label></div><div v-for="group in permissionGroups" :key="group.name" class="field-list"><b>{{ group.name }}</b><label v-for="permission in group.permissions" :key="permission.id" class="checkbox-label"><input v-model="rolePermissions" type="checkbox" :value="permission.id" @change="toggleRolePermission(permission)"/>{{ permission.name }}</label></div><button class="button primary" :disabled="busy || !selectedOrganization"><Plus :size="16"/>{{ editingRoleID ? '保存角色' : '新增角色' }}</button></form></section>
      <section class="panel setting-form"><h2>字段权限</h2><p class="muted">隐藏字段不会出现在页面、搜索结果、导出和 Agent 返回中；只读字段可查看但不能修改。必填字段必须保持可编辑。</p><details v-for="module in modules" :key="module.key" class="field-list"><summary><b>{{ module.label }}</b></summary><div v-for="field in module.fields" :key="field.key"><span><b>{{ field.label }}</b><small>{{ field.required ? '必填字段不可限制' : field.key }}</small></span><label class="checkbox-label"><input v-model="roleFieldRestrictions[module.key].hidden" type="checkbox" :value="field.key" :disabled="field.required"/>隐藏</label><label class="checkbox-label"><input v-model="roleFieldRestrictions[module.key].read_only" type="checkbox" :value="field.key" :disabled="field.required || roleFieldRestrictions[module.key].hidden.includes(field.key)"/>只读</label></div></details></section>
    </template>
    <section v-if="mode === 'modules'" class="panel data-panel"><header class="panel-heading"><div><h2>业务模块</h2><p>模块配置用于后台治理；停用模块会保留既有业务数据。</p></div><div><span class="tiny-label">{{ systemModules.length }} 个模块</span><button v-if="can('CUSTOM_FORM:ADD')" class="button primary" @click="editCustomModule()"><Plus :size="16"/>新建自定义模块</button></div></header><div class="table-scroll"><table><thead><tr><th>名称</th><th>模块标识</th><th>状态</th><th>操作</th></tr></thead><tbody><tr v-for="item in systemModules" :key="item.id"><td><b>{{ item.name }}</b><span v-if="item.custom" class="badge blue">自定义</span></td><td><code>{{ item.key }}</code></td><td><span :class="['badge', item.enabled ? 'green' : 'neutral']">{{ item.enabled ? '已启用' : '已停用' }}</span></td><td><button v-if="item.custom && can('CUSTOM_FORM:ADD')" class="text-button" :disabled="busy" @click="editCustomModule(item)">设计</button><button v-if="!item.custom && can('MODULE_SETTING:UPDATE')" class="text-button" :disabled="busy" @click="editLayout(item)">布局</button><button v-if="(item.custom && can('CUSTOM_FORM:ADD')) || (!item.custom && can('MODULE_SETTING:UPDATE'))" class="text-button" :disabled="busy" @click="toggleModule(item)">{{ item.enabled ? '停用' : '启用' }}</button><button v-if="item.custom && can('CUSTOM_FORM:ADD')" class="text-button danger" :disabled="busy" @click="deleteCustomModule(item)">删除</button></td></tr></tbody></table></div></section>
    <section v-if="mode === 'modules' && customModuleEditor" class="panel setting-form"><h2>{{ editingCustomModuleID ? '编辑自定义模块' : '新建自定义模块' }}</h2><form @submit.prevent="saveCustomModule"><label class="field"><span>模块名称</span><input v-model.trim="customModuleForm.name" required maxlength="40" placeholder="例如：渠道伙伴"/></label><label class="field"><span>模块标识</span><input v-model.trim="customModuleForm.key" required pattern="custom_[a-z][a-z0-9_]{1,35}" :disabled="!!editingCustomModuleID" placeholder="custom_partners"/></label><label class="field"><span>分组</span><input v-model.trim="customModuleForm.group" required maxlength="40"/></label><label class="field"><span>说明</span><textarea v-model.trim="customModuleForm.description" maxlength="200" rows="3"/></label><div class="field-list"><div v-for="(field, index) in customModuleForm.fields" :key="index"><input v-model.trim="field.name" required maxlength="40" placeholder="字段名称"/><input v-model.trim="field.key" required pattern="[a-z][a-z0-9_]{0,35}" :disabled="field.key === 'name'" placeholder="field_key"/><select v-model="field.type" :disabled="field.key === 'name'"><option v-for="type in ['text','textarea','number','money','date','email','select']" :key="type" :value="type">{{ typeName(type) }}</option></select><label class="checkbox-label"><input v-model="field.required" type="checkbox" :disabled="field.key === 'name'"/>必填</label><label class="checkbox-label"><input v-model="field.unique" type="checkbox"/>唯一</label><input v-if="field.type === 'select'" :value="field.options.join(',')" required placeholder="选项用逗号分隔" @input="field.options = ($event.target as HTMLInputElement).value.split(',').map(value => value.trim()).filter(Boolean)"/><button v-if="field.key !== 'name'" type="button" class="text-button danger" @click="customModuleForm.fields.splice(index, 1)">删除</button></div></div><button type="button" class="button" :disabled="customModuleForm.fields.length >= 60" @click="addCustomModuleField"><Plus :size="15"/>添加字段</button><label class="checkbox-label"><input v-model="customModuleForm.enabled" type="checkbox"/>启用模块</label><label class="checkbox-label"><input v-model="customModuleForm.shared" type="checkbox"/>组织共享数据</label><label class="checkbox-label"><input v-model="customModuleForm.approvable" type="checkbox"/>启用审批状态</label><button class="button primary" :disabled="busy">保存模块</button><button type="button" class="button" @click="customModuleEditor = false">取消</button></form></section>
    <section v-if="layoutModule" class="panel setting-form"><h2>{{ layoutModule.name }}表单与文档模板</h2><p class="muted">调整字段顺序和分组；文档模板使用纯文本字段占位符，例如 <code v-pre>{{name}}</code>。</p><div class="field-list"><div v-for="(field, index) in layoutFields" :key="field.key"><span><b>{{ modules.find(module => module.key === layoutModule?.key)?.fields.find(item => item.key === field.key)?.label || field.key }}</b><small>{{ field.key }}</small></span><label class="field"><span>分组</span><input v-model.trim="field.group" maxlength="40" required/></label><button class="text-button" :disabled="busy || index === 0" @click="moveLayoutField(index, -1)">上移</button><button class="text-button" :disabled="busy || index === layoutFields.length - 1" @click="moveLayoutField(index, 1)">下移</button></div></div><label class="field"><span>打印文档模板</span><textarea v-model="layoutModule.document_template" maxlength="10000" rows="10" placeholder="{{name}}&#10;金额：{{amount}}"/></label><template v-if="['opportunities', 'contracts'].includes(layoutModule.key)"><h3>阶段配置</h3><p class="muted">支持 2–15 个阶段；至少保留一个进行中和一个完结阶段。</p><div class="field-list"><div v-for="(stage, index) in layoutModule.stages" :key="index"><input v-model.trim="stage.name" maxlength="40" required placeholder="阶段名称"/><select v-model="stage.type"><option value="AFOOT">进行中</option><option value="END">完结</option></select><label v-if="layoutModule.key === 'opportunities'" class="field"><span>赢率 %</span><input v-model.number="stage.rate" type="number" min="0" max="100" required/></label><button class="text-button" :disabled="index === 0" @click="moveStage(index, -1)">上移</button><button class="text-button" :disabled="index === (layoutModule.stages?.length || 0) - 1" @click="moveStage(index, 1)">下移</button><button class="text-button danger" :disabled="(layoutModule.stages?.length || 0) <= 2" @click="removeStage(index)">删除</button></div></div><button class="button" :disabled="(layoutModule.stages?.length || 0) >= 15" @click="addStage"><Plus :size="15"/>新增阶段</button><label class="checkbox-label"><input v-model="layoutModule.afoot_rollback" type="checkbox"/>允许进行中阶段回退</label><label class="checkbox-label"><input v-model="layoutModule.end_rollback" type="checkbox"/>允许完结阶段回退</label><label class="field"><span>流转规则</span><select v-model="layoutModule.circulation_type"><option value="NORMAL">普通流转（按顺序推进）</option><option value="ADVANCED">高级流转（指定允许路径）</option></select></label><div v-if="layoutModule.circulation_type === 'ADVANCED'" class="field-list"><div v-for="origin in layoutModule.stages" :key="origin.name"><span><b>{{ origin.name || '未命名阶段' }}</b><small>允许流转到</small></span><label v-for="target in layoutModule.stages?.filter(item => item !== origin)" :key="target.name" class="checkbox-label"><input type="checkbox" :checked="transitionEnabled(origin.name, target.name)" :disabled="!origin.name || !target.name" @change="toggleStageTransition(origin.name, target.name, ($event.target as HTMLInputElement).checked)"/>{{ target.name || '未命名' }}</label></div></div></template><button class="button primary" :disabled="busy" @click="saveLayout">保存配置</button><button class="button" :disabled="busy" @click="layoutModule = undefined">取消</button></section>
    <section v-if="mode === 'modules' && can('SYSTEM_SETTING:READ')" class="panel data-panel"><header class="panel-heading"><div><h2>企业微信</h2><p>配置企业微信登录、通讯录同步和业务通知；Secret 加密存储且不会回显。</p></div></header><div class="field-list"><div><span><b>企业微信自建应用</b><small>{{ weCom.configured ? `${weCom.managed ? '可视化' : '环境变量'}配置 · ${weCom.oauth ? '登录/同步/通知可用' : '同步/通知可用，未启用登录'}` : '未配置' }}</small></span><span :class="['badge', weCom.configured ? 'green' : 'neutral']">{{ weCom.configured ? '可用' : '待配置' }}</span><button v-if="can('SYSTEM_SETTING:ADD') || can('SYSTEM_SETTING:UPDATE')" class="text-button" :disabled="busy" @click="editWeCom">配置</button><button v-if="can('SYSTEM_SETTING:UPDATE')" class="text-button" :disabled="busy || !weCom.configured" @click="testWeCom">连接测试</button><button v-if="weCom.managed && can('SYSTEM_SETTING:DELETE')" class="text-button danger" :disabled="busy" @click="deleteWeCom">删除配置</button></div></div><form v-if="editingWeCom" class="setting-form" @submit.prevent="saveWeCom"><h3>企业微信配置</h3><label class="field"><span>企业 ID（CorpID）</span><input v-model.trim="weComForm.corp_id" required maxlength="100" placeholder="wwxxxxxxxxxxxxxxxx"/></label><label class="field"><span>自建应用 AgentId</span><input v-model.trim="weComForm.agent_id" required inputmode="numeric" maxlength="20" placeholder="1000002"/></label><label class="field"><span>自建应用 Secret</span><input v-model="weComForm.secret" type="password" maxlength="4000" :required="!weCom.has_secret" :placeholder="weCom.has_secret ? '留空保留现有 Secret' : '请输入应用 Secret'" autocomplete="new-password"/></label><label class="field"><span>OAuth 回调地址（可选）</span><input v-model.trim="weComForm.redirect_url" type="url" maxlength="2000" placeholder="https://crm.example.com/api/auth/wecom/callback"/><small>填写后登录页显示“企业微信登录”；须与企业微信后台可信域名一致。</small></label><button class="button primary" :disabled="busy">保存企业微信配置</button><button type="button" class="button" @click="editingWeCom = false">取消</button></form></section>
    <section v-if="mode === 'modules'" class="panel data-panel"><header class="panel-heading"><div><h2>集成中心</h2><p>飞书、钉钉、SSO 及外部业务适配器；Token 加密存储且不会回显。</p></div></header><div class="field-list"><div v-for="item in integrations" :key="item.key"><span><b>{{ item.name }}</b><small>{{ item.mock ? '模拟联调模式' : item.configured ? `${item.managed ? '可视化' : '环境变量'}配置 · ${item.endpoint}` : '未配置' }}{{ item.has_token ? ' · 已保存 Token' : '' }}</small></span><span :class="['badge', item.configured ? 'green' : 'neutral']">{{ item.configured ? '可用' : '待配置' }}</span><button v-if="can('SYSTEM_SETTING:ADD') || can('SYSTEM_SETTING:UPDATE')" class="text-button" :disabled="busy" @click="editIntegration(item)">配置</button><button v-if="can('SYSTEM_SETTING:UPDATE')" class="text-button" :disabled="busy || !item.configured" @click="testIntegration(item)">连接测试</button><button v-if="item.managed && can('SYSTEM_SETTING:DELETE')" class="text-button danger" :disabled="busy" @click="deleteIntegration(item)">删除配置</button></div></div><form v-if="editingIntegrationKey" class="setting-form" @submit.prevent="saveIntegration"><h3>{{ integrations.find(item => item.key === editingIntegrationKey)?.name }}配置</h3><label class="field"><span>服务端点</span><input v-model.trim="integrationForm.endpoint" type="url" required maxlength="2000" placeholder="https://example.com/webhook"/></label><label class="field"><span>Token / Secret</span><input v-model="integrationForm.token" type="password" maxlength="4000" :placeholder="integrations.find(item => item.key === editingIntegrationKey)?.has_token ? '留空保留现有 Token' : '可选'" autocomplete="new-password"/></label><button class="button primary" :disabled="busy">保存连接器</button><button type="button" class="button" @click="editingIntegrationKey = ''">取消</button></form></section><section v-if="mode === 'modules' && (can('SYSTEM_NOTICE:READ') || can('SYSTEM_NOTICE:ADD'))" class="panel setting-form"><h2>公告中心</h2><p class="muted">向当前组织全部启用成员发布站内公告；成员会通过通知中心和实时推送收到。</p><p v-if="!user.organization_id" class="error-banner">当前账号尚未归属组织，请先在“成员与权限”中完成组织分配。</p><div v-if="can('SYSTEM_NOTICE:READ') && user.organization_id" class="field-list"><div v-for="item in announcements" :key="item.id"><span><b>{{ item.subject }}</b><small>{{ item.content }}</small></span><span><button v-if="can('SYSTEM_NOTICE:UPDATE')" class="text-button" :disabled="busy" @click="editAnnouncement(item)">编辑</button><button v-if="can('SYSTEM_NOTICE:DELETE')" class="text-button danger" :disabled="busy" @click="deleteAnnouncement(item)">删除</button></span></div><p v-if="!announcements.length" class="muted">暂无历史公告</p></div><form v-if="can('SYSTEM_NOTICE:ADD') && user.organization_id" @submit.prevent="publishAnnouncement"><label class="field"><span>公告标题</span><input v-model.trim="announcementSubject" required maxlength="100" placeholder="例如：系统维护通知"/></label><label class="field"><span>公告内容</span><textarea v-model.trim="announcementContent" required maxlength="5000" rows="5" placeholder="请输入公告正文"/></label><button class="button primary" :disabled="busy || !announcementSubject.trim() || !announcementContent.trim()">发布公告</button></form></section>
    <div v-if="mode === 'fields'" class="settings-grid">
      <section class="panel"><header class="panel-heading"><div><h2>模块字段</h2><p>标准字段与自定义字段</p></div><select v-model="selectedModule" aria-label="选择模块"><option v-for="m in modules" :key="m.key" :value="m.key">{{ m.label }}</option></select></header><div class="field-list"><div v-for="f in fields" :key="f.key"><span><b>{{ f.label }}</b><small>{{ f.key }}</small></span><span class="badge neutral">{{ typeName(f.type) }}</span><button v-if="(f.custom || f.standard) && can('CUSTOM_FORM:UPDATE')" class="text-button" :disabled="busy" @click="renameField(f)">改名</button><button v-if="(f.custom || f.disableable) && can('CUSTOM_FORM:DELETE')" class="text-button danger" :disabled="busy" @click="deleteField(f)">停用</button><span v-if="f.standard" class="tiny-label">标准</span></div></div></section>
      <section v-if="can('CUSTOM_FORM:ADD')" class="panel setting-form"><h2>新增自定义字段</h2><p class="muted">新增字段为选填，不影响已有记录。</p><form @submit.prevent="saveField"><label class="field"><span>字段名称</span><input v-model="custom.label" required maxlength="40" placeholder="例如：客户区域"/></label><label class="field"><span>字段标识</span><input v-model="custom.key" required pattern="custom_[a-z][a-z0-9_]{0,35}" placeholder="custom_region"/></label><label class="field"><span>字段类型</span><select v-model="custom.type"><option v-for="t in ['text', 'textarea', 'number', 'date', 'select', 'email', 'formula', 'subtable']" :key="t" :value="t">{{ typeName(t) }}</option></select></label><label v-if="custom.type === 'select'" class="field"><span>选项（每行一个）</span><textarea v-model="custom.options" required rows="5" placeholder="华东&#10;华南"/></label><div v-if="custom.type === 'formula'" class="field-list"><p class="muted">选择同模块数值或金额字段，保存记录时由服务端自动求和。</p><label v-for="field in fields.filter(field => ['number', 'money'].includes(field.type))" :key="field.key" class="checkbox-label"><input v-model="custom.sources" type="checkbox" :value="field.key"/>{{ field.label }}</label></div><div v-if="custom.type === 'subtable'" class="field-list"><p class="muted">配置子表列，列标识使用小写字母、数字和下划线。</p><div v-for="(column, index) in custom.columns" :key="index"><input v-model.trim="column.label" required maxlength="40" placeholder="列名称"/><input v-model.trim="column.key" required pattern="[a-z][a-z0-9_]{0,35}" placeholder="column_key"/><select v-model="column.type"><option v-for="type in ['text','number','money','date']" :key="type" :value="type">{{ typeName(type) }}</option></select><label class="checkbox-label"><input v-model="column.required" type="checkbox"/>必填</label><button type="button" class="text-button danger" @click="custom.columns.splice(index, 1)">删除</button></div><button type="button" class="text-button" @click="addSubtableColumn"><Plus :size="14"/>添加列</button></div><button class="button primary" :disabled="busy || (custom.type === 'formula' && !custom.sources.length) || (custom.type === 'subtable' && !custom.columns.length)"><Plus :size="16"/>添加字段</button></form></section>
    </div>
    <div v-if="mode === 'agents'" class="settings-grid"><section class="panel"><header class="panel-heading"><div><h2>Agent 列表</h2><p>为不同业务场景配置提示词和可调用工具</p></div><button v-if="can('AGENT:ADD')" class="button primary" @click="editAgent()"><Plus :size="16"/>新增 Agent</button></header><div class="field-list"><div v-for="item in agents" :key="item.id"><span><b>{{ item.name }}</b><small>{{ item.tool_names.join('、') }} · {{ item.prompt }}</small></span><span :class="['badge', item.enabled ? 'green' : 'neutral']">{{ item.enabled ? '已启用' : '已停用' }}</span><button v-if="can('AGENT:UPDATE')" class="text-button" :disabled="busy" @click="editAgent(item)">编辑</button><button v-if="can('AGENT:UPDATE')" class="text-button" :disabled="busy" @click="toggleAgent(item)">{{ item.enabled ? '停用' : '启用' }}</button><button v-if="can('AGENT:DELETE')" class="text-button danger" :disabled="busy" @click="deleteAgent(item)">删除</button></div><p v-if="!agents.length" class="muted">暂无 Agent 配置</p></div></section><section v-if="editingAgentID ? can('AGENT:UPDATE') : can('AGENT:ADD')" class="panel setting-form"><h2>{{ editingAgentID ? '编辑 Agent' : '新增 Agent' }}</h2><form @submit.prevent="saveAgent"><label class="field"><span>名称</span><input v-model.trim="agentForm.name" required maxlength="100" placeholder="例如：销售跟进助手"/></label><label class="field"><span>系统提示词</span><textarea v-model.trim="agentForm.prompt" required maxlength="10000" rows="8" placeholder="描述 Agent 的职责、边界和回答方式"/></label><div class="field-list"><b>工具授权</b><label v-for="tool in agentTools" :key="tool.name" class="checkbox-label"><input v-model="agentForm.tool_names" type="checkbox" :value="tool.name"/>{{ tool.name }} · {{ tool.description }}</label></div><label class="checkbox-label"><input v-model="agentForm.enabled" type="checkbox"/>启用该 Agent</label><button class="button primary" :disabled="busy || !agentForm.tool_names.length">保存 Agent</button><button v-if="editingAgentID" type="button" class="button" :disabled="busy" @click="editAgent()">取消</button></form></section></div>
    <div v-if="mode === 'dashboard_configs'" class="settings-grid"><section class="panel"><header class="panel-heading"><div><h2>自定义工作台</h2><p>配置组件、顺序、宽度和默认工作台</p></div><button v-if="can('DASHBOARD:ADD')" class="button primary" @click="editDashboard()"><Plus :size="16"/>新增工作台</button></header><div class="field-list"><div v-for="item in dashboardConfigs" :key="item.id"><span><b>{{ item.name }}</b><small>{{ item.widgets.map(widget => dashboardWidgetLabels[widget.key]).join('、') }}</small></span><span v-if="item.is_default" class="badge green">默认</span><button v-if="can('DASHBOARD:UPDATE')" class="text-button" @click="editDashboard(item)">编辑</button><button v-if="can('DASHBOARD:DELETE')" class="text-button danger" :disabled="busy" @click="deleteDashboardConfig(item)">删除</button></div><p v-if="!dashboardConfigs.length" class="muted">暂无自定义工作台，当前使用系统默认布局。</p></div></section><section v-if="editingDashboardID ? can('DASHBOARD:UPDATE') : can('DASHBOARD:ADD')" class="panel setting-form"><h2>{{ editingDashboardID ? '编辑工作台' : '新增工作台' }}</h2><form @submit.prevent="saveDashboardConfig"><label class="field"><span>名称</span><input v-model.trim="dashboardForm.name" maxlength="100" required placeholder="例如：销售经理工作台"/></label><div class="field-list"><div v-for="(widget, index) in dashboardForm.widgets" :key="widget.key"><span><b>{{ dashboardWidgetLabels[widget.key] }}</b><small>第 {{ index + 1 }} 位</small></span><select v-model.number="widget.width"><option :value="1">半宽</option><option :value="2">通栏</option></select><button type="button" class="text-button" :disabled="index === 0" @click="moveDashboardWidget(index, -1)">上移</button><button type="button" class="text-button" :disabled="index === dashboardForm.widgets.length - 1" @click="moveDashboardWidget(index, 1)">下移</button><button type="button" class="text-button danger" @click="dashboardForm.widgets.splice(index, 1)">移除</button></div></div><label v-if="dashboardForm.widgets.length < Object.keys(dashboardWidgetLabels).length" class="field"><span>添加组件</span><select value="" @change="addDashboardWidget(($event.target as HTMLSelectElement).value); ($event.target as HTMLSelectElement).value = ''"><option value="">请选择</option><option v-for="(label, key) in dashboardWidgetLabels" :key="key" :value="key" :disabled="dashboardForm.widgets.some(widget => widget.key === key)">{{ label }}</option></select></label><label class="checkbox-label"><input v-model="dashboardForm.is_default" type="checkbox"/>设为组织默认工作台</label><button class="button primary" :disabled="busy || !dashboardForm.widgets.length">保存工作台</button><button v-if="editingDashboardID" type="button" class="button" @click="editingDashboardID = ''">取消</button></form></section></div>
    <section v-if="mode === 'audit'" class="panel data-panel"><div class="table-scroll"><table><thead><tr><th>操作时间</th><th>操作人</th><th>操作</th><th>模块</th><th>详情</th></tr></thead><tbody><tr v-for="a in audits" :key="a.id"><td>{{ dateTime(a.created_at) }}</td><td>{{ a.user_name }}</td><td>{{ a.action }}</td><td>{{ modules.find(m => m.key === a.module)?.label || '系统' }}</td><td>{{ a.detail || '—' }}</td></tr></tbody></table></div><footer class="table-pagination"><span>共 {{ auditTotal }} 条操作日志</span><div><button class="icon-button" aria-label="上一页" :disabled="page <= 1" @click="page--; load()"><ChevronLeft :size="16"/></button><span>{{ page }}</span><button class="icon-button" aria-label="下一页" :disabled="page * 30 >= auditTotal" @click="page++; load()"><ChevronRight :size="16"/></button></div></footer></section>
    <div v-if="mode === 'profile'" class="settings-grid"><section class="panel setting-form"><span class="profile-avatar">{{ user.name.slice(-1) }}</span><h2>{{ user.name }}</h2><p class="muted">{{ user.username }} · {{ roleLabel[user.role] }}</p><p>部门：{{ user.department || '未分配' }}</p></section><section class="panel setting-form"><h2><LockKeyhole :size="19"/>修改密码</h2><p class="muted">密码至少 10 位，包含字母与数字。</p><form @submit.prevent="savePassword"><label class="field"><span>当前密码</span><input v-model="currentPassword" type="password" required autocomplete="current-password"/></label><label class="field"><span>新密码</span><input v-model="newPassword" type="password" minlength="10" maxlength="200" required autocomplete="new-password"/></label><button class="button primary" :disabled="busy">更新密码</button></form></section><section class="panel setting-form"><h2>API Key</h2><p class="muted">供 Agent 或外部自动化使用，调用时通过 <code>X-API-Key</code> 传递。</p><form v-if="can('PERSONAL_API_KEY:ADD')" @submit.prevent="createAPIKey"><label class="field"><span>密钥名称</span><input v-model.trim="apiKeyName" required maxlength="100" placeholder="例如：本地 Agent"/></label><button class="button primary" :disabled="busy || !apiKeyName.trim()"><Plus :size="16"/>创建 API Key</button></form><div v-if="createdSecret" class="success-banner"><b>请立即复制，之后不会再次显示：</b><code>{{ createdSecret }}</code><button class="icon-button" aria-label="关闭密钥提示" @click="createdSecret = ''"><X :size="15"/></button></div><div class="field-list"><div v-for="item in apiKeys" :key="item.id"><span><b>{{ item.name }}</b><small>{{ item.prefix }}… · 创建于 {{ dateTime(item.created_at) }}{{ item.last_used_at ? ` · 最近使用 ${dateTime(item.last_used_at)}` : '' }}</small></span><span :class="['badge', item.active ? 'green' : 'neutral']">{{ item.active ? '已启用' : '已停用' }}</span><button v-if="can('PERSONAL_API_KEY:UPDATE')" class="text-button" :disabled="busy" @click="toggleAPIKey(item)">{{ item.active ? '停用' : '启用' }}</button><button v-if="can('PERSONAL_API_KEY:DELETE')" class="text-button danger" :disabled="busy" @click="deleteAPIKey(item)">撤销</button></div><p v-if="!apiKeys.length" class="muted">暂无 API Key</p></div></section></div>
    <div v-if="editor" class="modal-backdrop" @click.self="!busy && (editor = false)">
      <section class="modal narrow" role="dialog" aria-modal="true" aria-label="成员编辑">
        <header class="modal-header"><h2>{{ editing ? '编辑成员' : '添加成员' }}</h2><button class="icon-button" aria-label="关闭成员编辑" @click="editor = false"><X :size="20"/></button></header>
        <form @submit.prevent="saveUser"><div class="modal-body form-grid">
          <label class="field"><span>姓名</span><input v-model="member.name" required maxlength="80"/></label>
          <label class="field"><span>用户名</span><input v-model="member.username" required :disabled="!!editing" pattern="[a-zA-Z0-9_.-]{3,40}"/></label>
          <label class="field"><span>角色</span><select v-model="member.role" :disabled="!!member.organization_id"><option v-for="(label, key) in roleLabel" :key="key" :value="key">{{ label }}</option></select></label>
          <label class="field"><span>组织</span><select v-model="member.organization_id" @change="member.role = 'sales'; member.role_id = ''; member.department = ''; loadMemberRoles(); loadMemberDepartments()"><option value="">未绑定组织（旧权限）</option><option v-for="organization in organizations" :key="organization.id" :value="organization.id">{{ organization.name }}</option></select></label>
          <label v-if="member.organization_id" class="field"><span>自定义角色</span><select v-model="member.role_id" required><option value="">请选择角色</option><option v-for="role in systemRoles" :key="role.id" :value="role.id">{{ role.name }}</option></select></label>
          <label v-if="member.organization_id" class="field"><span>部门</span><select v-model="member.department"><option value="">未分配</option><option v-for="item in memberDepartments" :key="item.id" :value="item.id">{{ item.name }}</option></select></label>
          <label v-else class="field"><span>部门</span><input v-model="member.department" maxlength="80"/></label>
          <label class="field full"><span>{{ editing ? '重置密码（留空保留原密码）' : '初始密码' }}</span><input v-model="member.password" type="password" :required="!editing" minlength="10" maxlength="200" autocomplete="new-password" placeholder="至少 10 位，包含字母与数字"/></label>
          <label v-if="editing" class="checkbox-label full"><input v-model="member.active" type="checkbox"/>启用该账号</label><p v-if="error" class="error-banner full">{{ error }}</p>
        </div><footer class="modal-footer"><button type="button" class="button" :disabled="busy" @click="editor = false">取消</button><button class="button primary" :disabled="busy">保存成员</button></footer></form>
      </section>
    </div>
  </div>
</template>
