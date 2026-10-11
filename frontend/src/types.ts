export type SubtableRow = Record<string, string | number | null>
export type Value = string | number | boolean | null | LineItem[] | SubtableRow[]
export interface LineItem { product_id: string; name?: string; specification?: string; quantity: string; unit_price: string; price?: string; discount_rate?: string; discount_amount?: string; tax_rate?: string; tax_amount?: string; pre_tax_amount?: string; subtotal?: string }
export interface Field { key: string; label: string; type: string; required: boolean; read_only?: boolean; custom?: boolean; standard?: boolean; disableable?: boolean; group?: string; sources?: string[]; columns?: Field[]; options?: string[]; module?: string; min?: number; max?: number }
export interface Module { key: string; label: string; description: string; group: string; approvable: boolean; shared?: boolean; fields: Field[]; document_template?: string }
export interface User { id: string; username: string; name: string; role: 'admin' | 'manager' | 'sales'; organization_id: string; role_id: string; permission_ids: string[]; department: string; active: boolean }
export interface PermissionDefinition { id: string; resource: string; action: string; name: string }
export interface PermissionGroup { name: string; permissions: PermissionDefinition[] }
export interface CRMRecord { id: string; module: string; owner_id: string | null; owner_name: string; version: number; created_at: string; updated_at: string; data: Record<string, Value>; references: Record<string, string> }
export interface Page { items: CRMRecord[]; total: number; page: number; size: number }
export interface FollowupComment { id: string; followup_id: string; user_id: string; user_name: string; content: string; created_at: string }
export interface Followup { id: string; content: string; method: string; next_date?: string; user_name: string; created_at: string; comments: FollowupComment[] }
export interface ApprovalDecision { id: string; reviewer_id: string; reviewer_name: string; decision: string; comment: string; step: number; created_at: string }
export interface Approval { id: string; record_id: string; name: string; module: string; status: string; requested_by: string; requester_name: string; reviewer_name?: string; comment: string; current_step?: number; total_steps?: number; current_approver_roles?: User['role'][]; current_approver_ids?: string[]; delegated_to_id?: string; delegated_to_name?: string; additional_approver_id?: string; additional_approver_name?: string; decisions: ApprovalDecision[]; created_at: string; reviewed_at?: string }
export interface Detail extends CRMRecord { followups: Followup[]; approvals: Approval[]; related: CRMRecord[] }
export interface DashboardData {
  customers: number; leads: number; opportunities: number; pipeline: string; weighted_pipeline: string; contract_amount: string; received_amount: string; receivable: string; conversion_rate: number;
  funnel: { stage: string; count: number; amount: string }[];
  months: { month: string; contracts: string; payments: string }[];
  activities: (Followup & { module: string; name: string; record_id: string })[];
  tasks: CRMRecord[]; overdue_plans: { id: string; name: string; remaining: string; due_date: string }[]; counts: Record<string, number>;
}
