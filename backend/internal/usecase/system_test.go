package usecase

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"cordialcrm/backend/internal/entity"
)

func TestModuleOutputUsesFrontendNestedFieldNames(t *testing.T) {
	payload, err := json.Marshal(ModuleOutput{Layout: []entity.FieldLayout{{Key: "name", Group: "基本信息", Sort: 1}}, Fields: []entity.FieldDefinition{{ID: "field-1", ModuleID: "module-1", Key: "name", Name: "名称", Type: "text", Required: true, ReadOnly: true, Options: []string{}}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{`"layout":[{"key":"name","group":"基本信息","sort":1}]`, `"module_id":"module-1"`, `"read_only":true`} {
		if !strings.Contains(string(payload), expected) {
			t.Fatalf("module nested field missing: %s in %s", expected, payload)
		}
	}
}

type systemRepositoryStub struct {
	SystemRepository
	organizations       []entity.Organization
	departments         []entity.Department
	role                entity.SystemRole
	module              entity.ModuleDefinition
	deletedRole         string
	deletedOrganization string
	deletedDepartment   string
	deletedModule       string
}

func (s *systemRepositoryStub) ListOrganizations(_ context.Context) ([]entity.Organization, error) {
	if s.organizations != nil {
		return s.organizations, nil
	}
	return []entity.Organization{{ID: "org-1"}, {ID: "org-2"}}, nil
}
func (s *systemRepositoryStub) SaveOrganization(_ context.Context, item entity.Organization) error {
	for index := range s.organizations {
		if s.organizations[index].ID == item.ID {
			s.organizations[index] = item
			return nil
		}
	}
	s.organizations = append(s.organizations, item)
	return nil
}
func (s *systemRepositoryStub) DeleteOrganization(_ context.Context, id string) error {
	s.deletedOrganization = id
	return nil
}
func (s *systemRepositoryStub) WithinTransaction(ctx context.Context, action func(context.Context) error) error {
	organizations := append([]entity.Organization(nil), s.organizations...)
	departments := append([]entity.Department(nil), s.departments...)
	if err := action(ctx); err != nil {
		s.organizations, s.departments = organizations, departments
		return err
	}
	return nil
}
func (s *systemRepositoryStub) FindRoleByID(_ context.Context, id string) (entity.SystemRole, error) {
	if id == s.role.ID {
		return s.role, nil
	}
	return entity.SystemRole{}, ErrNotFound
}
func (s *systemRepositoryStub) SaveRole(_ context.Context, role entity.SystemRole) error {
	s.role = role
	return nil
}
func (s *systemRepositoryStub) DeleteRole(_ context.Context, id string) error {
	s.deletedRole = id
	return nil
}

func (s *systemRepositoryStub) ListDepartments(_ context.Context, _ string) ([]entity.Department, error) {
	return s.departments, nil
}
func (s *systemRepositoryStub) SaveDepartment(_ context.Context, item entity.Department) error {
	for index := range s.departments {
		if s.departments[index].ID == item.ID {
			s.departments[index] = item
			return nil
		}
	}
	s.departments = append(s.departments, item)
	return nil
}
func (s *systemRepositoryStub) DeleteDepartment(_ context.Context, id, _ string) error {
	s.deletedDepartment = id
	return nil
}
func (s *systemRepositoryStub) ListCustomFields(_ context.Context, _ string) ([]entity.CustomField, error) {
	return nil, nil
}
func (s *systemRepositoryStub) ListStandardFieldSettings(_ context.Context, _ string) ([]entity.StandardFieldSetting, error) {
	return nil, nil
}
func (s *systemRepositoryStub) SaveModule(_ context.Context, module entity.ModuleDefinition) error {
	s.module = module
	return nil
}
func (s *systemRepositoryStub) ListModules(_ context.Context) ([]entity.ModuleDefinition, error) {
	if s.module.Key == "" {
		return nil, nil
	}
	return []entity.ModuleDefinition{s.module}, nil
}
func (s *systemRepositoryStub) DeleteModule(_ context.Context, id, _ string) error {
	s.deletedModule = id
	return nil
}

func TestSaveDepartmentRejectsMissingAndCyclicParent(t *testing.T) {
	admin := UserOutput{ID: "admin", Role: entity.RoleAdmin}
	repository := &systemRepositoryStub{departments: []entity.Department{{ID: "root", OrganizationID: "org-1", Name: "总部"}, {ID: "sales", OrganizationID: "org-1", ParentID: "root", Name: "销售部"}}}
	service := NewSystem(repository)
	if _, err := service.SaveDepartment(context.Background(), SaveDepartmentInput{Actor: admin, ID: "new", OrganizationID: "org-1", ParentID: "other-org", Name: "无效部门"}); err == nil {
		t.Fatal("expected missing parent rejection")
	}
	if _, err := service.SaveDepartment(context.Background(), SaveDepartmentInput{Actor: admin, ID: "root", OrganizationID: "org-1", ParentID: "sales", Name: "总部"}); err == nil {
		t.Fatal("expected cyclic parent rejection")
	}
	created, err := service.SaveDepartment(context.Background(), SaveDepartmentInput{Actor: admin, OrganizationID: "org-1", ParentID: "sales", Name: "华东组", Sort: 1})
	if err != nil || created.ParentID != "sales" {
		t.Fatalf("unexpected department: %#v, err=%v", created, err)
	}
}

func TestUpdateAndDeleteDepartmentValidateOrganization(t *testing.T) {
	repository := &systemRepositoryStub{departments: []entity.Department{{ID: "sales", OrganizationID: "org-1", Name: "销售部"}, {ID: "east", OrganizationID: "org-1", ParentID: "sales", Name: "华东组"}}}
	service := NewSystem(repository)
	actor := UserOutput{Role: entity.RoleSales, RoleID: "role-1", OrganizationID: "org-1", PermissionIDs: []string{"SYS_ORGANIZATION:UPDATE", "SYS_ORGANIZATION:DELETE"}}
	updated, err := service.SaveDepartment(context.Background(), SaveDepartmentInput{Actor: actor, ID: "east", OrganizationID: "org-1", Name: "华东销售组", Sort: 2})
	if err != nil || updated.ParentID != "" || repository.departments[1].Name != "华东销售组" {
		t.Fatalf("department update failed: output=%#v departments=%#v err=%v", updated, repository.departments, err)
	}
	if _, err := service.SaveDepartment(context.Background(), SaveDepartmentInput{Actor: actor, ID: "missing", OrganizationID: "org-1", Name: "不存在"}); err != ErrNotFound {
		t.Fatalf("expected missing department rejection, got %v", err)
	}
	if _, err := service.SaveDepartment(context.Background(), SaveDepartmentInput{Actor: actor, ID: "east", OrganizationID: "org-2", Name: "越界"}); err != ErrForbidden {
		t.Fatalf("expected cross-organization update rejection, got %v", err)
	}
	if err := service.DeleteDepartment(context.Background(), DeleteDepartmentInput{Actor: actor, ID: "east", OrganizationID: "org-2"}); err != ErrForbidden {
		t.Fatalf("expected cross-organization delete rejection, got %v", err)
	}
	if err := service.DeleteDepartment(context.Background(), DeleteDepartmentInput{Actor: actor, ID: "east", OrganizationID: "org-1"}); err != nil || repository.deletedDepartment != "east" {
		t.Fatalf("department delete failed: deleted=%s err=%v", repository.deletedDepartment, err)
	}
}

func TestSaveRoleRejectsCrossOrganizationUpdateAndInvalidGrant(t *testing.T) {
	repository := &systemRepositoryStub{role: entity.SystemRole{ID: "role-1", OrganizationID: "org-1", Name: "销售"}}
	service := NewSystem(repository)
	admin := UserOutput{ID: "admin", Role: entity.RoleAdmin}
	if _, err := service.SaveRole(context.Background(), SaveRoleInput{Actor: admin, ID: "role-1", OrganizationID: "org-2", Name: "越界"}); err != ErrForbidden {
		t.Fatalf("cross-organization update: %v", err)
	}
	if _, err := service.SaveRole(context.Background(), SaveRoleInput{Actor: admin, OrganizationID: "org-1", Name: "越权", PermissionIDs: []string{"system.admin"}}); err != ErrValidation {
		t.Fatalf("invalid grant: %v", err)
	}
	if _, err := service.SaveRole(context.Background(), SaveRoleInput{Actor: admin, ID: "role-1", OrganizationID: "org-1", Name: "销售", PermissionIDs: []string{"customers.read"}}); err != nil {
		t.Fatalf("valid update: %v", err)
	}
	if _, err := service.SaveRole(context.Background(), SaveRoleInput{Actor: admin, ID: "role-1", OrganizationID: "org-1", Name: "销售", PermissionIDs: []string{"CUSTOMER_MANAGEMENT:READ", "CUSTOMER_MANAGEMENT:TRANSFER", "CUSTOMER_MANAGEMENT_POOL:PICK"}}); err != nil {
		t.Fatalf("valid upstream permissions: %v", err)
	}
}

func TestPermissionCatalogContainsUpstreamSpecialPermissions(t *testing.T) {
	valid := validPermissionIDs()
	for _, permissionID := range []string{"SYS_ORGANIZATION_USER:RESET_PASSWORD", "CUSTOMER_MANAGEMENT:MERGE", "CUSTOMER_MANAGEMENT_POOL:ASSIGN", "OPPORTUNITY_MANAGEMENT:RESIGN", "OPPORTUNITY_QUOTATION:DOWNLOAD", "OPPORTUNITY_QUOTATION:VOIDED", "PRICE:READ", "PRICE:ADD", "PRICE:UPDATE", "PRICE:DELETE", "PRICE:IMPORT", "PRICE:EXPORT", "CONTRACT:STAGE", "CONTRACT:PAYMENT", "ORDER:DOWNLOAD"} {
		if !valid[permissionID] {
			t.Fatalf("permission catalog is missing %s", permissionID)
		}
	}
}

func TestDeleteRoleRequiresPermissionAndRejectsCrossOrganization(t *testing.T) {
	repository := &systemRepositoryStub{role: entity.SystemRole{ID: "role-1", OrganizationID: "org-1", Name: "销售"}}
	service := NewSystem(repository)
	actor := UserOutput{Role: entity.RoleSales, RoleID: "custom", PermissionIDs: []string{"SYSTEM_ROLE:DELETE"}}
	if err := service.DeleteRole(context.Background(), DeleteRoleInput{Actor: actor, ID: "role-1", OrganizationID: "org-2"}); err != ErrForbidden {
		t.Fatalf("expected cross-organization rejection, got %v", err)
	}
	if err := service.DeleteRole(context.Background(), DeleteRoleInput{Actor: actor, ID: "role-1", OrganizationID: "org-1"}); err != nil || repository.deletedRole != "role-1" {
		t.Fatalf("delete role failed: deleted=%s err=%v", repository.deletedRole, err)
	}
	actor.PermissionIDs = nil
	if err := service.DeleteRole(context.Background(), DeleteRoleInput{Actor: actor, ID: "role-1", OrganizationID: "org-1"}); err != ErrForbidden {
		t.Fatalf("expected permission rejection, got %v", err)
	}
}

func TestUpdateAndDeleteOrganizationPermissions(t *testing.T) {
	repository := &systemRepositoryStub{organizations: []entity.Organization{{ID: "org-1", Name: "旧名称"}, {ID: "org-2", Name: "其他组织"}}}
	service := NewSystem(repository)
	actor := UserOutput{Role: entity.RoleSales, RoleID: "role-1", OrganizationID: "org-1", PermissionIDs: []string{"SYS_ORGANIZATION:UPDATE", "SYS_ORGANIZATION:DELETE"}}
	updated, err := service.UpdateOrganization(context.Background(), UpdateOrganizationInput{Actor: actor, ID: "org-1", Name: "新名称"})
	if err != nil || updated.Name != "新名称" || repository.organizations[0].Name != "新名称" {
		t.Fatalf("update organization failed: output=%#v organizations=%#v err=%v", updated, repository.organizations, err)
	}
	if _, err := service.UpdateOrganization(context.Background(), UpdateOrganizationInput{Actor: actor, ID: "org-2", Name: "越界"}); err != ErrForbidden {
		t.Fatalf("expected cross-organization update rejection, got %v", err)
	}
	if err := service.DeleteOrganization(context.Background(), DeleteOrganizationInput{Actor: actor, ID: "org-2"}); err != ErrForbidden {
		t.Fatalf("expected cross-organization delete rejection, got %v", err)
	}
	if err := service.DeleteOrganization(context.Background(), DeleteOrganizationInput{Actor: actor, ID: "org-1"}); err != nil || repository.deletedOrganization != "org-1" {
		t.Fatalf("delete organization failed: deleted=%s err=%v", repository.deletedOrganization, err)
	}
	actor.PermissionIDs = nil
	if err := service.DeleteOrganization(context.Background(), DeleteOrganizationInput{Actor: actor, ID: "org-1"}); err != ErrForbidden {
		t.Fatalf("expected organization delete permission rejection, got %v", err)
	}
}

func TestImportOrganizationsResolvesParentsAndRollsBack(t *testing.T) {
	repository := &systemRepositoryStub{organizations: []entity.Organization{}}
	service := NewSystem(repository)
	actor := UserOutput{Role: entity.RoleSales, RoleID: "role-1", PermissionIDs: []string{"SYS_ORGANIZATION:IMPORT"}}
	result, err := service.ImportOrganizations(context.Background(), ImportOrganizationsInput{Actor: actor, Rows: []OrganizationImportRow{
		{Organization: "华东中心", Department: "上海一组", ParentDepartment: "销售部", Sort: 2},
		{Organization: "华东中心", Department: "销售部", Sort: 1},
	}})
	if err != nil || result.Organizations != 1 || result.Departments != 2 || len(repository.departments) != 2 || repository.departments[1].ParentID != repository.departments[0].ID {
		t.Fatalf("unexpected import result: %#v organizations=%#v departments=%#v err=%v", result, repository.organizations, repository.departments, err)
	}
	beforeOrganizations, beforeDepartments := len(repository.organizations), len(repository.departments)
	_, err = service.ImportOrganizations(context.Background(), ImportOrganizationsInput{Actor: actor, Rows: []OrganizationImportRow{{Organization: "华南中心", Department: "销售组", ParentDepartment: "不存在"}}})
	if err == nil || len(repository.organizations) != beforeOrganizations || len(repository.departments) != beforeDepartments {
		t.Fatalf("failed import was not rolled back: organizations=%#v departments=%#v err=%v", repository.organizations, repository.departments, err)
	}
	actor.PermissionIDs = nil
	if _, err := service.ImportOrganizations(context.Background(), ImportOrganizationsInput{Actor: actor, Rows: []OrganizationImportRow{{Organization: "无权限"}}}); err != ErrForbidden {
		t.Fatalf("expected import permission rejection, got %v", err)
	}
}

func TestSaveRolePersistsDataScope(t *testing.T) {
	repository := &systemRepositoryStub{departments: []entity.Department{{ID: "sales", OrganizationID: "org-1"}}}
	service := NewSystem(repository)
	output, err := service.SaveRole(context.Background(), SaveRoleInput{Actor: UserOutput{Role: entity.RoleAdmin}, OrganizationID: "org-1", Name: "区域销售", PermissionIDs: []string{"customers.read"}, DataScope: "specified_departments", DepartmentIDs: []string{"sales"}, FieldRestrictions: map[string]entity.FieldRestriction{"customers": {Hidden: []string{"phone"}}}})
	if err != nil || output.DataScope != "specified_departments" || len(output.DepartmentIDs) != 1 || output.DepartmentIDs[0] != "sales" || len(output.FieldRestrictions["customers"].Hidden) != 1 || output.FieldRestrictions["customers"].Hidden[0] != "phone" {
		t.Fatalf("unexpected role scope: %#v, %v", output, err)
	}
}

func TestSaveRoleRejectsInvalidSpecifiedDepartments(t *testing.T) {
	repository := &systemRepositoryStub{departments: []entity.Department{{ID: "sales", OrganizationID: "org-1"}}}
	service := NewSystem(repository)
	admin := UserOutput{Role: entity.RoleAdmin}
	for _, input := range []SaveRoleInput{
		{Actor: admin, OrganizationID: "org-1", Name: "空部门", PermissionIDs: []string{"customers.read"}, DataScope: "specified_departments"},
		{Actor: admin, OrganizationID: "org-1", Name: "错误部门", PermissionIDs: []string{"customers.read"}, DataScope: "specified_departments", DepartmentIDs: []string{"other"}},
	} {
		if _, err := service.SaveRole(context.Background(), input); err != ErrValidation {
			t.Fatalf("invalid specified departments accepted: %#v, err=%v", input, err)
		}
	}
}

func TestSaveRoleRejectsInvalidFieldRestrictions(t *testing.T) {
	service := NewSystem(&systemRepositoryStub{})
	admin := UserOutput{Role: entity.RoleAdmin}
	for _, restrictions := range []map[string]entity.FieldRestriction{
		{"unknown": {Hidden: []string{"name"}}},
		{"customers": {Hidden: []string{"name"}}},
		{"customers": {Hidden: []string{"phone"}, ReadOnly: []string{"phone"}}},
	} {
		if _, err := service.SaveRole(context.Background(), SaveRoleInput{Actor: admin, OrganizationID: "org-1", Name: "受限销售", PermissionIDs: []string{"customers.read"}, FieldRestrictions: restrictions}); err != ErrValidation {
			t.Fatalf("invalid field restrictions accepted: %#v, %v", restrictions, err)
		}
	}
}

func TestSaveModuleValidatesDocumentTemplate(t *testing.T) {
	admin := UserOutput{Role: entity.RoleAdmin}
	for _, template := range []string{"客户：{{name}}\n负责人：{{ owner_name }}", "编号：{{id}}"} {
		repository := &systemRepositoryStub{}
		service := NewSystem(repository)
		module := entity.ModuleDefinition{Key: "customers", Name: "客户", DocumentTemplate: template}
		if err := service.SaveModule(context.Background(), SaveModuleInput{Actor: admin, Module: module}); err != nil {
			t.Fatalf("valid document template rejected: %q, %v", template, err)
		}
		if repository.module.DocumentTemplate != template {
			t.Fatalf("document template not persisted: %q", repository.module.DocumentTemplate)
		}
	}

	for _, template := range []string{"{{unknown}}", "{{Name}}", "{{ name }"} {
		service := NewSystem(&systemRepositoryStub{})
		module := entity.ModuleDefinition{Key: "customers", Name: "客户", DocumentTemplate: template}
		if err := service.SaveModule(context.Background(), SaveModuleInput{Actor: admin, Module: module}); err != ErrValidation {
			t.Fatalf("invalid document template accepted: %q, %v", template, err)
		}
	}

	service := NewSystem(&systemRepositoryStub{})
	module := entity.ModuleDefinition{Key: "customers", Name: "客户", DocumentTemplate: strings.Repeat("字", 10001)}
	if err := service.SaveModule(context.Background(), SaveModuleInput{Actor: admin, Module: module}); err != ErrValidation {
		t.Fatalf("oversized document template accepted: %v", err)
	}
}

func TestSaveModuleValidatesOpportunitySalesStages(t *testing.T) {
	admin := UserOutput{Role: entity.RoleAdmin}
	repository := &systemRepositoryStub{}
	service := NewSystem(repository)
	module := entity.ModuleDefinition{Key: "opportunities", Name: "商机", SalesStages: []string{"初步接洽", "方案确认", "赢单"}}
	if err := service.SaveModule(context.Background(), SaveModuleInput{Actor: admin, Module: module}); err != nil {
		t.Fatalf("valid sales stages rejected: %v", err)
	}
	if len(repository.module.SalesStages) != 3 || repository.module.SalesStages[1] != "方案确认" {
		t.Fatalf("sales stages not persisted: %#v", repository.module.SalesStages)
	}
	for _, stages := range [][]string{{"唯一阶段"}, {"重复", "重复"}, {"正常", " "}} {
		module.SalesStages = stages
		if err := service.SaveModule(context.Background(), SaveModuleInput{Actor: admin, Module: module}); err != ErrValidation {
			t.Fatalf("invalid sales stages accepted: %#v, %v", stages, err)
		}
	}
}

func TestSaveModuleValidatesAdvancedStageConfiguration(t *testing.T) {
	admin := UserOutput{Role: entity.RoleAdmin}
	repository := &systemRepositoryStub{}
	service := NewSystem(repository)
	module := entity.ModuleDefinition{Key: "contracts", Name: "合同", Stages: []entity.StageDefinition{{Name: "履行中", Type: "AFOOT"}, {Name: "已归档", Type: "END"}}, CirculationType: "ADVANCED", StageTransitions: []entity.StageTransition{{Origin: "履行中", Target: "已归档"}}}
	if err := service.SaveModule(context.Background(), SaveModuleInput{Actor: admin, Module: module}); err != nil {
		t.Fatalf("valid advanced stage configuration rejected: %v", err)
	}
	if repository.module.CirculationType != "ADVANCED" || len(repository.module.StageTransitions) != 1 || len(repository.module.SalesStages) != 2 {
		t.Fatalf("advanced stage configuration not persisted: %#v", repository.module)
	}
	module.StageTransitions = nil
	if err := service.SaveModule(context.Background(), SaveModuleInput{Actor: admin, Module: module}); err == nil {
		t.Fatal("advanced stage configuration without paths accepted")
	}
	module.CirculationType = "NORMAL"
	module.Stages[1].Type = "AFOOT"
	if err := service.SaveModule(context.Background(), SaveModuleInput{Actor: admin, Module: module}); err == nil {
		t.Fatal("stage configuration without an end stage accepted")
	}
}

func TestCustomModuleLifecycleValidation(t *testing.T) {
	repository := &systemRepositoryStub{}
	service := NewSystem(repository)
	actor := UserOutput{RoleID: "role-1", PermissionIDs: []string{"CUSTOM_FORM:ADD"}}
	module := entity.ModuleDefinition{Key: "custom_partners", Name: "渠道伙伴", Custom: true, Enabled: true, Fields: []entity.FieldDefinition{{Key: "name", Name: "名称", Type: "text"}, {Key: "level", Name: "等级", Type: "select", Options: []string{"核心", "普通"}}}}
	if err := service.SaveModule(context.Background(), SaveModuleInput{Actor: actor, Module: module}); err != nil {
		t.Fatalf("valid custom module rejected: %v", err)
	}
	if !repository.module.Custom || repository.module.ID == "" || len(repository.module.Layout) != 2 || !repository.module.Fields[0].Required {
		t.Fatalf("custom module not normalized: %#v", repository.module)
	}
	invalid := module
	invalid.Key = "partners"
	if err := service.SaveModule(context.Background(), SaveModuleInput{Actor: actor, Module: invalid}); err != ErrValidation {
		t.Fatalf("invalid custom module key accepted: %v", err)
	}
	if err := service.DeleteModule(context.Background(), DeleteModuleInput{Actor: actor, ID: repository.module.ID, Key: repository.module.Key}); err != nil || repository.deletedModule != repository.module.ID {
		t.Fatalf("custom module delete failed: %v", err)
	}
}
