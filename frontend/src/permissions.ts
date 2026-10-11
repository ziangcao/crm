import type { User } from './types'

export const moduleResources: Record<string, string> = {
  customers: 'CUSTOMER_MANAGEMENT', contacts: 'CUSTOMER_MANAGEMENT_CONTACT', leads: 'CLUE_MANAGEMENT',
  opportunities: 'OPPORTUNITY_MANAGEMENT', quotes: 'OPPORTUNITY_QUOTATION', products: 'PRODUCT_MANAGEMENT', prices: 'PRICE',
  contracts: 'CONTRACT', payment_plans: 'CONTRACT_PAYMENT_PLAN', payments: 'CONTRACT_PAYMENT_RECORD',
  companies: 'CONTRACT_BUSINESS_TITLE', invoices: 'CONTRACT_INVOICE', orders: 'ORDER', tenders: 'TENDER', tasks: 'TASK_MANAGEMENT',
}

const poolResources: Record<string, string> = { customers: 'CUSTOMER_MANAGEMENT_POOL', leads: 'CLUE_MANAGEMENT_POOL' }

export function hasPermission(user: User, permissionId: string): boolean {
  if (!user.role_id) {
    if (user.role === 'admin') return true
    const resource = permissionId.split(':')[0]
    return Object.values(moduleResources).includes(resource) || Object.values(poolResources).includes(resource) || permissionId === 'DASHBOARD:READ' || permissionId.startsWith('PERSONAL_API_KEY:')
  }
  return user.permission_ids?.includes(permissionId) || false
}

export function hasModulePermission(user: User, moduleKey: string, action: string, pool = false): boolean {
  const resource = pool && poolResources[moduleKey] ? poolResources[moduleKey] : moduleResources[moduleKey]
  if (!resource) return false
  if (hasPermission(user, `${resource}:${action}`)) return true
  if (!user.role_id) return true
  return user.permission_ids?.includes(`${moduleKey}.write`) || (action === 'READ' && user.permission_ids?.includes(`${moduleKey}.read`)) || false
}

export function permissionId(moduleKey: string, action: string, pool = false): string {
  return `${pool && poolResources[moduleKey] ? poolResources[moduleKey] : moduleResources[moduleKey]}:${action}`
}
