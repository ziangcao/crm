package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"cordialcrm/backend/internal/config"
	"cordialcrm/backend/internal/controller"
	"cordialcrm/backend/internal/entity"
	"cordialcrm/backend/internal/repo"
	"cordialcrm/backend/internal/usecase"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const seedNote = "系统演示数据，请勿用于生产"

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	c, err := config.Load()
	if err != nil {
		return err
	}
	var dialector gorm.Dialector
	if c.Driver == "postgres" {
		dialector = postgres.Open(c.DSN)
	} else {
		dialector = mysql.Open(c.DSN)
	}
	db, err := gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		return errors.New("数据库连接失败，请检查配置及服务状态")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	repository := repo.New(db)
	if err := repository.Migrate(ctx); err != nil {
		return err
	}
	actor, err := demoUser(ctx, repository)
	if err != nil {
		return err
	}
	if err := repository.BootstrapModules(ctx, builtInModules()); err != nil {
		return err
	}
	records := usecase.NewRecord(repository, repository)
	notifications := usecase.NewNotification(repository)
	return seed(ctx, repository, records, usecase.NewApproval(repository, notifications), usecase.NewSystem(repository), notifications, actor)
}

func builtInModules() []entity.ModuleDefinition {
	items := make([]entity.ModuleDefinition, 0, len(usecase.Modules))
	for key, module := range usecase.Modules {
		items = append(items, entity.ModuleDefinition{Key: key, Name: module.Label})
	}
	return items
}

func demoUser(ctx context.Context, repository *repo.GormRepository) (usecase.UserOutput, error) {
	user, err := repository.FindUserByUsername(ctx, "demo")
	if err == nil {
		return usecase.UserOutput{ID: user.ID, Username: user.Username, Name: user.Name, Department: user.Department, Role: user.Role, OrganizationID: user.OrganizationID, RoleID: user.RoleID, PermissionIDs: user.PermissionIDs, DataScope: user.DataScope, DepartmentIDs: user.DepartmentIDs, FieldRestrictions: user.FieldRestrictions, Active: user.Active}, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return usecase.UserOutput{}, err
	}
	return usecase.NewUser(repository, controller.Passwords{}).Create(ctx, usecase.CreateUserInput{
		Actor: usecase.UserOutput{Role: entity.RoleAdmin}, Username: "demo", Password: "DemoPass123", Name: "演示管理员", Department: "销售部", Role: entity.RoleAdmin,
	})
}

func seed(ctx context.Context, repository *repo.GormRepository, records *usecase.RecordUseCase, approvals *usecase.ApprovalUseCase, system *usecase.SystemUseCase, notifications *usecase.NotificationUseCase, actor usecase.UserOutput) error {
	organizationID, err := seedSystem(ctx, system, actor)
	if err != nil {
		return err
	}
	if actor.OrganizationID == "" {
		if _, err = usecase.NewOrganizationMigration(repository, repository).Assign(ctx, usecase.AssignOrganizationInput{OrganizationID: organizationID, Apply: true}); err != nil {
			return err
		}
		user, findErr := repository.FindUserByID(ctx, actor.ID)
		if findErr != nil {
			return findErr
		}
		actor = usecase.UserOutput{ID: user.ID, Username: user.Username, Name: user.Name, Department: user.Department, Role: user.Role, OrganizationID: user.OrganizationID, RoleID: user.RoleID, PermissionIDs: user.PermissionIDs, DataScope: user.DataScope, DepartmentIDs: user.DepartmentIDs, FieldRestrictions: user.FieldRestrictions, Active: user.Active}
	}
	product, err := ensure(ctx, records, actor, "products", map[string]any{"name": "Cordys CRM 企业版", "code": "CRM-ENT", "category": "SaaS", "price": "120000", "unit": "套", "status": "在售", "notes": seedNote})
	if err != nil {
		return err
	}
	customer, err := ensure(ctx, records, actor, "customers", map[string]any{"name": "星云科技有限公司", "industry": "企业服务", "source": "市场活动", "level": "A · 重点", "status": "合作客户", "phone": "13800138000", "website": "https://example.com", "address": "上海市浦东新区", "notes": seedNote})
	if err != nil {
		return err
	}
	if _, err = ensure(ctx, records, actor, "contacts", map[string]any{"name": "张晓晨", "customer_id": customer.ID, "title": "信息平台主管", "phone": "13800138001", "email": "zhang@example.com", "role": "决策者", "notes": seedNote}); err != nil {
		return err
	}
	opportunity, err := ensure(ctx, records, actor, "opportunities", map[string]any{"name": "星云 CRM 升级项目", "customer_id": customer.ID, "amount": "120000", "stage": "商务谈判", "probability": float64(75), "expected_date": "2026-12-31", "notes": seedNote})
	if err != nil {
		return err
	}
	items := []any{map[string]any{"product_id": product.ID, "name": "Cordys CRM 企业版", "quantity": 1, "unit_price": "120000"}}
	quote, err := ensure(ctx, records, actor, "quotes", map[string]any{"name": "星云科技 CRM 报价单", "customer_id": customer.ID, "opportunity_id": opportunity.ID, "items": items, "valid_until": "2026-12-31", "notes": seedNote})
	if err != nil {
		return err
	}
	contract, err := ensure(ctx, records, actor, "contracts", map[string]any{"name": "星云科技 CRM 服务合同", "code": "HT-2026-001", "customer_id": customer.ID, "quote_id": quote.ID, "amount": "120000", "signed_date": "2026-09-15", "end_date": "2027-09-14", "notes": seedNote})
	if err != nil {
		return err
	}
	company, err := ensure(ctx, records, actor, "companies", map[string]any{"name": "星云科技有限公司", "customer_id": customer.ID, "tax_number": "91310000DEMO00001X", "bank": "演示银行", "bank_account": "6222000000000001", "address": "上海市浦东新区", "notes": seedNote})
	if err != nil {
		return err
	}
	if _, err = ensure(ctx, records, actor, "orders", map[string]any{"name": "星云科技 CRM 订单", "code": "SO-2026-001", "customer_id": customer.ID, "contract_id": contract.ID, "items": items, "delivery_date": "2026-10-31", "delivery_status": "待交付", "notes": seedNote}); err != nil {
		return err
	}
	plan, err := ensure(ctx, records, actor, "payment_plans", map[string]any{"name": "星云科技首期回款", "customer_id": customer.ID, "contract_id": contract.ID, "amount": "60000", "due_date": "2026-06-30", "notes": seedNote})
	if err != nil {
		return err
	}
	if _, err = ensure(ctx, records, actor, "payment_plans", map[string]any{"name": "星云科技合同尾款", "customer_id": customer.ID, "contract_id": contract.ID, "amount": "60000", "due_date": "2026-08-31", "notes": seedNote}); err != nil {
		return err
	}
	invoice, err := ensure(ctx, records, actor, "invoices", map[string]any{"name": "星云科技首期发票", "code": "FP-2026-001", "customer_id": customer.ID, "contract_id": contract.ID, "company_id": company.ID, "amount": "60000", "issued_date": "2026-09-18", "type": "增值税专用发票", "notes": seedNote})
	if err != nil {
		return err
	}
	payment, err := ensure(ctx, records, actor, "payments", map[string]any{"name": "星云科技首期回款", "customer_id": customer.ID, "contract_id": contract.ID, "plan_id": plan.ID, "amount": "60000", "received_date": "2026-09-20", "method": "银行转账", "notes": seedNote})
	if err != nil {
		return err
	}
	if _, err = ensure(ctx, records, actor, "leads", map[string]any{"name": "蓝海智能线索", "company": "蓝海智能有限公司", "contact": "李明", "phone": "13800138002", "email": "li@example.com", "source": "官网咨询", "industry": "制造业", "status": "待跟进", "notes": seedNote}); err != nil {
		return err
	}
	if _, err = ensure(ctx, records, actor, "tenders", map[string]any{"name": "智能制造 CRM 招标", "company": "蓝海智能有限公司", "amount": "300000", "deadline": "2026-11-30", "url": "https://example.com/tender", "status": "关注中", "notes": seedNote}); err != nil {
		return err
	}
	if _, err = ensure(ctx, records, actor, "tasks", map[string]any{"name": "跟进星云科技续约需求", "customer_id": customer.ID, "due_date": "2026-10-15", "priority": "高", "status": "进行中", "notes": seedNote}); err != nil {
		return err
	}
	followupContent := "已完成产品演示，等待法务确认合同条款。"
	followups, err := repository.ListFollowups(ctx, customer.ID)
	if err != nil {
		return err
	}
	hasFollowup := false
	for _, followup := range followups {
		if followup.Content == followupContent && followup.Method == "拜访" {
			hasFollowup = true
			break
		}
	}
	if !hasFollowup {
		if err = records.AddFollowup(ctx, usecase.FollowupInput{Actor: actor, Module: "customers", RecordID: customer.ID, Content: followupContent, Method: "拜访"}); err != nil {
			return err
		}
	}
	if err := seedNotification(ctx, notifications, actor, customer); err != nil {
		return err
	}
	for _, item := range []usecase.RecordOutput{quote, contract, invoice, payment} {
		if err := approve(ctx, repository, records, approvals, actor, item); err != nil {
			return err
		}
	}
	fmt.Fprintln(os.Stdout, "演示数据已就绪；账号 demo，密码 DemoPass123（重复执行不会覆盖已有业务记录）")
	return nil
}

func seedNotification(ctx context.Context, notifications *usecase.NotificationUseCase, actor usecase.UserOutput, customer usecase.RecordOutput) error {
	items, err := notifications.List(ctx, usecase.ListNotificationsInput{Actor: actor, Page: 1, Size: 100})
	if err != nil {
		return err
	}
	const subject = "欢迎使用 Cordys CRM 演示工作空间"
	for _, item := range items.Items {
		if item.Subject == subject {
			return nil
		}
	}
	_, err = notifications.Create(ctx, usecase.CreateNotificationInput{Actor: actor, Recipient: actor, Type: "system", Subject: subject, Content: "演示客户、商机、合同与回款数据已加载完成，可从客户管理开始体验完整业务流程。", ResourceID: customer.ID, ResourceType: "customers"})
	return err
}

func seedSystem(ctx context.Context, system *usecase.SystemUseCase, actor usecase.UserOutput) (string, error) {
	organizations, err := system.ListOrganizations(ctx, usecase.ListOrganizationsInput{Actor: actor})
	if err != nil {
		return "", err
	}
	organizationID := ""
	for _, item := range organizations {
		if item.Name == "演示组织" {
			organizationID = item.ID
			break
		}
	}
	if organizationID == "" {
		item, err := system.CreateOrganization(ctx, usecase.CreateOrganizationInput{Actor: actor, Name: "演示组织"})
		if err != nil {
			return "", err
		}
		organizationID = item.ID
	}
	departments, err := system.ListDepartments(ctx, usecase.ListDepartmentsInput{Actor: actor, OrganizationID: organizationID})
	if err != nil {
		return "", err
	}
	if len(departments) == 0 {
		if _, err = system.SaveDepartment(ctx, usecase.SaveDepartmentInput{Actor: actor, OrganizationID: organizationID, Name: "销售部", Sort: 1}); err != nil {
			return "", err
		}
	}
	roles, err := system.ListRoles(ctx, usecase.ListRolesInput{Actor: actor, OrganizationID: organizationID})
	if err != nil {
		return "", err
	}
	if len(roles) == 0 {
		for _, name := range []string{"管理员", "销售经理", "销售成员"} {
			if _, err = system.SaveRole(ctx, usecase.SaveRoleInput{Actor: actor, OrganizationID: organizationID, Name: name}); err != nil {
				return "", err
			}
		}
	}
	modules, err := system.ListModules(ctx, usecase.ListModulesInput{Actor: actor})
	if err != nil {
		return "", err
	}
	existing := make(map[string]bool, len(modules.Modules))
	for _, item := range modules.Modules {
		existing[item.Key] = true
	}
	for key, item := range usecase.Modules {
		if !existing[key] {
			if err = system.SaveModule(ctx, usecase.SaveModuleInput{Actor: actor, Module: entity.ModuleDefinition{Key: key, Name: item.Label, Enabled: true}}); err != nil {
				return "", err
			}
		}
	}
	return organizationID, nil
}

func approve(ctx context.Context, repository *repo.GormRepository, records *usecase.RecordUseCase, approvals *usecase.ApprovalUseCase, actor usecase.UserOutput, record usecase.RecordOutput) error {
	status := fmt.Sprint(record.Data["approval_status"])
	if status == string(entity.ApprovalApproved) {
		return nil
	}
	if status == string(entity.ApprovalDraft) {
		updated, err := records.Action(ctx, usecase.ActionInput{Actor: actor, Module: record.Module, ID: record.ID, Action: "submit", Version: record.Version})
		if err != nil {
			return err
		}
		record, status = updated, string(entity.ApprovalPending)
	}
	if status != string(entity.ApprovalPending) {
		return nil
	}
	users, err := repository.ListUsers(ctx)
	if err != nil {
		return err
	}
	for {
		items, err := approvals.List(ctx, usecase.ListApprovalsInput{Actor: actor, Status: string(entity.ApprovalPending)})
		if err != nil {
			return err
		}
		var pending *usecase.ApprovalOutput
		for index := range items {
			if items[index].RecordID == record.ID {
				pending = &items[index]
				break
			}
		}
		if pending == nil {
			return errors.New("未找到待处理的演示审批")
		}
		approver := actor
		if len(pending.CurrentApproverRoles) > 0 {
			found := false
			for _, user := range users {
				if user.Active && allowsRole(user.Role, pending.CurrentApproverRoles) {
					approver = usecase.UserOutput{ID: user.ID, Username: user.Username, Name: user.Name, Department: user.Department, Role: user.Role, Active: user.Active}
					found = true
					break
				}
			}
			if !found {
				return errors.New("未找到当前审批步骤所需的演示审批人")
			}
		}
		if err := approvals.Decide(ctx, usecase.DecideApprovalInput{Actor: approver, ID: pending.ID, Decision: "approved"}); err != nil {
			return err
		}
		if pending.TotalSteps == 0 || pending.CurrentStep >= pending.TotalSteps {
			return nil
		}
	}
}

func allowsRole(role entity.Role, allowed []entity.Role) bool {
	for _, item := range allowed {
		if role == item {
			return true
		}
	}
	return false
}

func ensure(ctx context.Context, records *usecase.RecordUseCase, actor usecase.UserOutput, module string, data map[string]any) (usecase.RecordOutput, error) {
	list, err := records.List(ctx, usecase.ListRecordsInput{Actor: actor, Module: module, Page: 1, Size: 100})
	if err != nil {
		return usecase.RecordOutput{}, err
	}
	for _, item := range list.Items {
		if item.Data["name"] == data["name"] && item.Data["notes"] == seedNote {
			return item, nil
		}
	}
	return records.Create(ctx, usecase.RecordInput{Actor: actor, Module: module, Data: data})
}
