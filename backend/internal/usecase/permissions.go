package usecase

import (
	"strings"

	"cordialcrm/backend/internal/entity"
)

type PermissionDefinition struct {
	ID       string `json:"id"`
	Resource string `json:"resource"`
	Action   string `json:"action"`
	Name     string `json:"name"`
}

type PermissionGroup struct {
	Name        string                 `json:"name"`
	Permissions []PermissionDefinition `json:"permissions"`
}

var actionNames = map[string]string{
	"READ": "查看", "ADD": "新增", "UPDATE": "编辑", "DELETE": "删除", "IMPORT": "导入", "EXPORT": "导出",
	"SYNC": "同步", "RESET_PASSWORD": "重置密码", "ADD_USER": "添加成员", "REMOVE_USER": "移除成员", "EDIT": "编辑",
	"TRANSFER": "转移", "RECYCLE": "放入公海", "MERGE": "合并", "PICK": "领取", "ASSIGN": "分配", "FREEZE": "冻结",
	"RESIGN": "转化", "DOWNLOAD": "下载", "VOIDED": "作废", "STAGE": "阶段变更", "PAYMENT": "回款登记",
}

func permissionGroup(name, resource string, actions ...string) PermissionGroup {
	permissions := make([]PermissionDefinition, 0, len(actions))
	for _, action := range actions {
		permissions = append(permissions, PermissionDefinition{ID: resource + ":" + action, Resource: resource, Action: action, Name: actionNames[action]})
	}
	return PermissionGroup{Name: name, Permissions: permissions}
}

var permissionGroups = []PermissionGroup{
	permissionGroup("组织架构", "SYS_ORGANIZATION", "READ", "ADD", "UPDATE", "DELETE", "IMPORT", "SYNC"),
	permissionGroup("组织成员", "SYS_ORGANIZATION_USER", "RESET_PASSWORD"),
	permissionGroup("角色权限", "SYSTEM_ROLE", "READ", "ADD", "UPDATE", "DELETE", "ADD_USER", "REMOVE_USER"),
	permissionGroup("模块设置", "MODULE_SETTING", "READ", "UPDATE"),
	permissionGroup("系统公告", "SYSTEM_NOTICE", "READ", "ADD", "UPDATE", "DELETE"),
	permissionGroup("系统设置", "SYSTEM_SETTING", "READ", "ADD", "UPDATE", "DELETE"),
	permissionGroup("操作日志", "OPERATION_LOG", "READ"),
	permissionGroup("流程设置", "PROCESS_SETTING", "READ", "ADD", "UPDATE", "DELETE"),
	permissionGroup("许可证", "LICENSE", "READ", "EDIT"),
	permissionGroup("客户管理", "CUSTOMER_MANAGEMENT", "READ", "ADD", "UPDATE", "TRANSFER", "RECYCLE", "DELETE", "IMPORT", "EXPORT", "MERGE"),
	permissionGroup("客户公海", "CUSTOMER_MANAGEMENT_POOL", "READ", "UPDATE", "PICK", "ASSIGN", "FREEZE", "DELETE", "IMPORT", "EXPORT"),
	permissionGroup("联系人", "CUSTOMER_MANAGEMENT_CONTACT", "READ", "ADD", "UPDATE", "DELETE", "IMPORT", "EXPORT"),
	permissionGroup("线索管理", "CLUE_MANAGEMENT", "READ", "ADD", "UPDATE", "TRANSFER", "RECYCLE", "DELETE", "IMPORT", "EXPORT"),
	permissionGroup("线索池", "CLUE_MANAGEMENT_POOL", "READ", "UPDATE", "PICK", "ASSIGN", "FREEZE", "DELETE", "IMPORT", "EXPORT"),
	permissionGroup("商机管理", "OPPORTUNITY_MANAGEMENT", "READ", "ADD", "UPDATE", "TRANSFER", "DELETE", "IMPORT", "EXPORT", "RESIGN"),
	permissionGroup("报价管理", "OPPORTUNITY_QUOTATION", "READ", "ADD", "UPDATE", "DELETE", "DOWNLOAD", "VOIDED"),
	permissionGroup("产品管理", "PRODUCT_MANAGEMENT", "READ", "ADD", "UPDATE", "DELETE", "IMPORT", "EXPORT"),
	permissionGroup("价格管理", "PRICE", "READ", "ADD", "UPDATE", "DELETE", "IMPORT", "EXPORT"),
	permissionGroup("工作台", "DASHBOARD", "READ", "ADD", "UPDATE", "DELETE"),
	permissionGroup("个人 API Key", "PERSONAL_API_KEY", "READ", "ADD", "UPDATE", "DELETE"),
	permissionGroup("智能助手", "AGENT", "READ", "ADD", "UPDATE", "DELETE"),
	permissionGroup("标讯", "TENDER", "READ"),
	permissionGroup("合同", "CONTRACT", "READ", "ADD", "UPDATE", "DELETE", "IMPORT", "EXPORT", "STAGE", "PAYMENT"),
	permissionGroup("回款计划", "CONTRACT_PAYMENT_PLAN", "READ", "ADD", "UPDATE", "DELETE", "IMPORT", "EXPORT"),
	permissionGroup("回款记录", "CONTRACT_PAYMENT_RECORD", "READ", "ADD", "UPDATE", "DELETE", "IMPORT", "EXPORT"),
	permissionGroup("工商抬头", "CONTRACT_BUSINESS_TITLE", "READ", "ADD", "UPDATE", "DELETE", "EXPORT", "IMPORT"),
	permissionGroup("发票", "CONTRACT_INVOICE", "READ", "ADD", "UPDATE", "DELETE", "IMPORT", "EXPORT"),
	permissionGroup("订单", "ORDER", "READ", "ADD", "UPDATE", "DELETE", "DOWNLOAD", "IMPORT", "EXPORT"),
	permissionGroup("自定义表单", "CUSTOM_FORM", "READ", "ADD", "UPDATE", "DELETE"),
	permissionGroup("任务计划", "TASK_MANAGEMENT", "READ", "ADD", "UPDATE", "DELETE", "IMPORT", "EXPORT"),
}

var moduleResources = map[string]string{
	"customers": "CUSTOMER_MANAGEMENT", "contacts": "CUSTOMER_MANAGEMENT_CONTACT", "leads": "CLUE_MANAGEMENT",
	"opportunities": "OPPORTUNITY_MANAGEMENT", "quotes": "OPPORTUNITY_QUOTATION", "products": "PRODUCT_MANAGEMENT", "prices": "PRICE",
	"contracts": "CONTRACT", "payment_plans": "CONTRACT_PAYMENT_PLAN", "payments": "CONTRACT_PAYMENT_RECORD",
	"companies": "CONTRACT_BUSINESS_TITLE", "invoices": "CONTRACT_INVOICE", "orders": "ORDER", "tenders": "TENDER",
	"tasks": "TASK_MANAGEMENT",
}

var poolResources = map[string]string{"customers": "CUSTOMER_MANAGEMENT_POOL", "leads": "CLUE_MANAGEMENT_POOL"}

func PermissionCatalog() []PermissionGroup { return permissionGroups }

func validPermissionIDs() map[string]bool {
	valid := map[string]bool{}
	for _, group := range permissionGroups {
		for _, permission := range group.Permissions {
			valid[permission.ID] = true
		}
	}
	return valid
}

func HasPermission(actor UserOutput, permissionID string) bool {
	if actor.RoleID == "" {
		if actor.Role == entity.RoleAdmin {
			return true
		}
		resource := strings.SplitN(permissionID, ":", 2)[0]
		for _, candidate := range moduleResources {
			if resource == candidate {
				return true
			}
		}
		for _, candidate := range poolResources {
			if resource == candidate {
				return true
			}
		}
		return permissionID == "DASHBOARD:READ" || permissionID == "AGENT:READ" || strings.HasPrefix(permissionID, "PERSONAL_API_KEY:")
	}
	for _, permission := range actor.PermissionIDs {
		if permission == permissionID {
			return true
		}
	}
	return false
}

func HasModulePermission(actor UserOutput, moduleKey, action string, pool bool) bool {
	resource := moduleResources[moduleKey]
	if pool && poolResources[moduleKey] != "" {
		resource = poolResources[moduleKey]
	}
	if resource == "" {
		return false
	}
	if HasPermission(actor, resource+":"+action) {
		return true
	}
	if actor.RoleID == "" {
		return true
	}
	legacyRead, legacyWrite := moduleKey+".read", moduleKey+".write"
	for _, permission := range actor.PermissionIDs {
		if permission == legacyWrite || (action == "READ" && permission == legacyRead) {
			return true
		}
	}
	return false
}

func requirePermission(actor UserOutput, permissionID string) error {
	if !HasPermission(actor, permissionID) {
		return ErrForbidden
	}
	return nil
}

func requireOrganization(actor UserOutput) error {
	if strings.TrimSpace(actor.OrganizationID) == "" {
		return entity.NewBusinessError("validation", "当前账号尚未归属组织，请先在成员与权限中完成组织分配")
	}
	return nil
}
