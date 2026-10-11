export class ApiError extends Error {
  constructor(public status: number, message: string) { super(message) }
}

export async function api<T>(path: string, method = 'GET', data?: unknown): Promise<T> {
  let response: Response
  try {
    response = await fetch('/api' + path, { method, credentials: 'same-origin', headers: data === undefined ? {} : { 'Content-Type': 'application/json' }, body: data === undefined ? undefined : JSON.stringify(data) })
  } catch { throw new Error('无法连接服务，请检查后端是否运行') }
  const result = await response.json().catch(() => ({}))
  if (!response.ok) {
    const message = typeof result.detail === 'string' ? result.detail : Array.isArray(result.detail) ? result.detail.map((v: {msg: string}) => v.msg).join('；') : '请求失败，请稍后重试'
    if (response.status === 401 && path !== '/auth/login' && path !== '/auth/me') window.dispatchEvent(new Event('session-expired'))
    throw new ApiError(response.status, message)
  }
  return result as T
}

export const money = (value: unknown) => new Intl.NumberFormat('zh-CN', { style: 'currency', currency: 'CNY', maximumFractionDigits: 2 }).format(Number(value || 0))
export const compactMoney = (value: unknown) => Number(value || 0) >= 10000 ? (Number(value) / 10000).toLocaleString('zh-CN', { maximumFractionDigits: 2 }) + ' 万' : money(value)
export const dateTime = (value: string) => new Date(value).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
export const statusLabel: Record<string, string> = { draft: '草稿', pending: '审批中', approved: '已通过', rejected: '已驳回', withdrawn: '已撤回' }
export const roleLabel = { admin: '系统管理员', manager: '销售经理', sales: '销售成员' }
export function statusTone(value: unknown): string {
  const s = String(value)
  if (['approved', '赢单', '已完成', '合作客户', '中标', '已转化', '在售'].includes(s)) return 'green'
  if (['rejected', '输单', '无效', '流失客户', '高'].includes(s)) return 'red'
  if (['pending', '跟进中', '商务谈判', '进行中', '准备投标'].includes(s)) return 'orange'
  return 'blue'
}
