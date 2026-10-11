package usecase

import (
	"context"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"cordialcrm/backend/internal/entity"
)

type SystemRepository interface {
	WithinTransaction(context.Context, func(context.Context) error) error
	ListOrganizations(context.Context) ([]entity.Organization, error)
	SaveOrganization(context.Context, entity.Organization) error
	DeleteOrganization(context.Context, string) error
	ListDepartments(context.Context, string) ([]entity.Department, error)
	SaveDepartment(context.Context, entity.Department) error
	DeleteDepartment(context.Context, string, string) error
	ListRoles(context.Context, string) ([]entity.SystemRole, error)
	FindRoleByID(context.Context, string) (entity.SystemRole, error)
	SaveRole(context.Context, entity.SystemRole) error
	DeleteRole(context.Context, string) error
	ListCustomFields(context.Context, string) ([]entity.CustomField, error)
	ListStandardFieldSettings(context.Context, string) ([]entity.StandardFieldSetting, error)
	ListModules(context.Context) ([]entity.ModuleDefinition, error)
	SaveModule(context.Context, entity.ModuleDefinition) error
	DeleteModule(context.Context, string, string) error
}

type ListModulesInput struct {
	Actor UserOutput
}

type ListModulesOutput struct {
	Modules []ModuleOutput `json:"modules"`
}

type ListPermissionsOutput struct {
	Groups []PermissionGroup `json:"groups"`
}

type ModuleOutput struct {
	ID               string                   `json:"id"`
	Key              string                   `json:"key"`
	Name             string                   `json:"name"`
	Icon             string                   `json:"icon"`
	Enabled          bool                     `json:"enabled"`
	Layout           []entity.FieldLayout     `json:"layout"`
	DocumentTemplate string                   `json:"document_template"`
	SalesStages      []string                 `json:"sales_stages,omitempty"`
	Stages           []entity.StageDefinition `json:"stages,omitempty"`
	AfootRollback    bool                     `json:"afoot_rollback"`
	EndRollback      bool                     `json:"end_rollback"`
	CirculationType  string                   `json:"circulation_type,omitempty"`
	StageTransitions []entity.StageTransition `json:"stage_transitions,omitempty"`
	Group            string                   `json:"group,omitempty"`
	Description      string                   `json:"description,omitempty"`
	Shared           bool                     `json:"shared"`
	Approvable       bool                     `json:"approvable"`
	Custom           bool                     `json:"custom"`
	Fields           []entity.FieldDefinition `json:"fields,omitempty"`
}

type SaveModuleInput struct {
	Actor  UserOutput
	Module entity.ModuleDefinition
}
type DeleteModuleInput struct {
	Actor   UserOutput
	ID, Key string
}

type SystemUseCase struct {
	repository SystemRepository
}

type ListOrganizationsInput struct{ Actor UserOutput }
type OrganizationOutput struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}
type CreateOrganizationInput struct {
	Actor UserOutput
	Name  string
}
type UpdateOrganizationInput struct {
	Actor    UserOutput
	ID, Name string
}
type DeleteOrganizationInput struct {
	Actor UserOutput
	ID    string
}
type OrganizationImportRow struct {
	Organization     string `json:"organization"`
	Department       string `json:"department"`
	ParentDepartment string `json:"parent_department"`
	Sort             int    `json:"sort"`
}
type ImportOrganizationsInput struct {
	Actor UserOutput
	Rows  []OrganizationImportRow
}
type ImportOrganizationsOutput struct {
	Organizations int `json:"organizations"`
	Departments   int `json:"departments"`
}
type ListDepartmentsInput struct {
	Actor          UserOutput
	OrganizationID string
}
type DepartmentOutput struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organization_id"`
	ParentID       string `json:"parent_id"`
	Name           string `json:"name"`
	Sort           int    `json:"sort"`
}
type SaveDepartmentInput struct {
	Actor                              UserOutput
	ID, OrganizationID, ParentID, Name string
	Sort                               int
}
type DeleteDepartmentInput struct {
	Actor          UserOutput
	ID             string
	OrganizationID string
}
type ListRolesInput struct {
	Actor          UserOutput
	OrganizationID string
}
type RoleOutput struct {
	ID                string                             `json:"id"`
	OrganizationID    string                             `json:"organization_id"`
	Name              string                             `json:"name"`
	BuiltIn           bool                               `json:"built_in"`
	PermissionIDs     []string                           `json:"permission_ids"`
	DataScope         string                             `json:"data_scope"`
	DepartmentIDs     []string                           `json:"department_ids"`
	FieldRestrictions map[string]entity.FieldRestriction `json:"field_restrictions"`
}
type SaveRoleInput struct {
	Actor                    UserOutput
	ID, OrganizationID, Name string
	PermissionIDs            []string
	DataScope                string
	DepartmentIDs            []string
	FieldRestrictions        map[string]entity.FieldRestriction
}
type DeleteRoleInput struct {
	Actor          UserOutput
	ID             string
	OrganizationID string
}

func NewSystem(repository SystemRepository) *SystemUseCase {
	return &SystemUseCase{repository: repository}
}

func (u *SystemUseCase) ListPermissions(ctx context.Context, input ListModulesInput) (ListPermissionsOutput, error) {
	if err := requirePermission(input.Actor, "SYSTEM_ROLE:READ"); err != nil {
		return ListPermissionsOutput{}, err
	}
	groups := append([]PermissionGroup(nil), PermissionCatalog()...)
	modules, err := u.repository.ListModules(ctx)
	if err != nil {
		return ListPermissionsOutput{}, err
	}
	for _, module := range modules {
		if module.Custom {
			groups = append(groups, PermissionGroup{Name: module.Name, Permissions: []PermissionDefinition{{ID: module.Key + ".read", Resource: module.Key, Action: "LEGACY_READ", Name: "查看"}, {ID: module.Key + ".write", Resource: module.Key, Action: "LEGACY_WRITE", Name: "维护"}}})
		}
	}
	return ListPermissionsOutput{Groups: groups}, nil
}

func (u *SystemUseCase) ListModules(ctx context.Context, input ListModulesInput) (ListModulesOutput, error) {
	if err := requirePermission(input.Actor, "MODULE_SETTING:READ"); err != nil {
		return ListModulesOutput{}, err
	}
	modules, err := u.repository.ListModules(ctx)
	if err != nil {
		return ListModulesOutput{}, err
	}
	output := make([]ModuleOutput, len(modules))
	for index, item := range modules {
		output[index] = ModuleOutput{ID: item.ID, Key: item.Key, Name: item.Name, Icon: item.Icon, Enabled: item.Enabled, Layout: item.Layout, DocumentTemplate: item.DocumentTemplate, SalesStages: item.SalesStages, Stages: item.Stages, AfootRollback: item.AfootRollback, EndRollback: item.EndRollback, CirculationType: item.CirculationType, StageTransitions: item.StageTransitions, Group: item.Group, Description: item.Description, Shared: item.Shared, Approvable: item.Approvable, Custom: item.Custom, Fields: item.Fields}
	}
	return ListModulesOutput{Modules: output}, nil
}

func (u *SystemUseCase) ListOrganizations(ctx context.Context, input ListOrganizationsInput) ([]OrganizationOutput, error) {
	if err := requirePermission(input.Actor, "SYS_ORGANIZATION:READ"); err != nil {
		return nil, err
	}
	items, err := u.repository.ListOrganizations(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]OrganizationOutput, len(items))
	for index, item := range items {
		result[index] = OrganizationOutput{ID: item.ID, Name: item.Name, CreatedAt: item.CreatedAt}
	}
	return result, nil
}

func (u *SystemUseCase) CreateOrganization(ctx context.Context, input CreateOrganizationInput) (OrganizationOutput, error) {
	if err := requirePermission(input.Actor, "SYS_ORGANIZATION:ADD"); err != nil {
		return OrganizationOutput{}, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" || len([]rune(name)) > 100 {
		return OrganizationOutput{}, ErrValidation
	}
	item := entity.Organization{ID: entity.NewID(), Name: name, CreatedAt: time.Now().UTC()}
	if err := u.repository.SaveOrganization(ctx, item); err != nil {
		return OrganizationOutput{}, err
	}
	return OrganizationOutput{ID: item.ID, Name: item.Name, CreatedAt: item.CreatedAt}, nil
}

func (u *SystemUseCase) UpdateOrganization(ctx context.Context, input UpdateOrganizationInput) (OrganizationOutput, error) {
	if err := requirePermission(input.Actor, "SYS_ORGANIZATION:UPDATE"); err != nil {
		return OrganizationOutput{}, err
	}
	name := strings.TrimSpace(input.Name)
	if input.ID == "" || name == "" || len([]rune(name)) > 100 {
		return OrganizationOutput{}, ErrValidation
	}
	if input.Actor.OrganizationID != "" && input.Actor.OrganizationID != input.ID {
		return OrganizationOutput{}, ErrForbidden
	}
	items, err := u.repository.ListOrganizations(ctx)
	if err != nil {
		return OrganizationOutput{}, err
	}
	for _, item := range items {
		if item.ID != input.ID {
			continue
		}
		item.Name = name
		if err := u.repository.SaveOrganization(ctx, item); err != nil {
			return OrganizationOutput{}, err
		}
		return OrganizationOutput{ID: item.ID, Name: item.Name, CreatedAt: item.CreatedAt}, nil
	}
	return OrganizationOutput{}, ErrNotFound
}

func (u *SystemUseCase) DeleteOrganization(ctx context.Context, input DeleteOrganizationInput) error {
	if err := requirePermission(input.Actor, "SYS_ORGANIZATION:DELETE"); err != nil {
		return err
	}
	if input.ID == "" {
		return ErrValidation
	}
	if input.Actor.OrganizationID != "" && input.Actor.OrganizationID != input.ID {
		return ErrForbidden
	}
	items, err := u.repository.ListOrganizations(ctx)
	if err != nil {
		return err
	}
	for _, item := range items {
		if item.ID == input.ID {
			return u.repository.DeleteOrganization(ctx, input.ID)
		}
	}
	return ErrNotFound
}

func (u *SystemUseCase) ImportOrganizations(ctx context.Context, input ImportOrganizationsInput) (ImportOrganizationsOutput, error) {
	if err := requirePermission(input.Actor, "SYS_ORGANIZATION:IMPORT"); err != nil {
		return ImportOrganizationsOutput{}, err
	}
	if len(input.Rows) == 0 || len(input.Rows) > 500 {
		return ImportOrganizationsOutput{}, ErrValidation
	}
	result := ImportOrganizationsOutput{}
	err := u.repository.WithinTransaction(ctx, func(transactionContext context.Context) error {
		organizations, err := u.repository.ListOrganizations(transactionContext)
		if err != nil {
			return err
		}
		byOrganizationName := make(map[string]entity.Organization, len(organizations))
		for _, organization := range organizations {
			byOrganizationName[organization.Name] = organization
		}
		for _, row := range input.Rows {
			name := strings.TrimSpace(row.Organization)
			if name == "" || len([]rune(name)) > 100 || row.Sort < 0 {
				return ErrValidation
			}
			if _, exists := byOrganizationName[name]; exists {
				continue
			}
			organization := entity.Organization{ID: entity.NewID(), Name: name, CreatedAt: time.Now().UTC()}
			if err := u.repository.SaveOrganization(transactionContext, organization); err != nil {
				return err
			}
			byOrganizationName[name] = organization
			result.Organizations++
		}
		departmentsByOrganization := map[string]map[string]entity.Department{}
		for _, organization := range byOrganizationName {
			departments, err := u.repository.ListDepartments(transactionContext, organization.ID)
			if err != nil {
				return err
			}
			departmentsByOrganization[organization.ID] = map[string]entity.Department{}
			for _, department := range departments {
				departmentsByOrganization[organization.ID][department.Name] = department
			}
		}
		pending := append([]OrganizationImportRow(nil), input.Rows...)
		seen := map[string]bool{}
		for len(pending) > 0 {
			next := make([]OrganizationImportRow, 0)
			progress := false
			for _, row := range pending {
				organizationName := strings.TrimSpace(row.Organization)
				departmentName := strings.TrimSpace(row.Department)
				parentName := strings.TrimSpace(row.ParentDepartment)
				if departmentName == "" {
					if parentName != "" {
						return ErrValidation
					}
					continue
				}
				if len([]rune(departmentName)) > 100 || len([]rune(parentName)) > 100 || departmentName == parentName {
					return ErrValidation
				}
				organization := byOrganizationName[organizationName]
				key := organization.ID + "\x00" + departmentName
				if seen[key] {
					return ErrValidation
				}
				if _, exists := departmentsByOrganization[organization.ID][departmentName]; exists {
					seen[key] = true
					continue
				}
				parentID := ""
				if parentName != "" {
					parent, exists := departmentsByOrganization[organization.ID][parentName]
					if !exists {
						next = append(next, row)
						continue
					}
					parentID = parent.ID
				}
				department := entity.Department{ID: entity.NewID(), OrganizationID: organization.ID, ParentID: parentID, Name: departmentName, Sort: row.Sort}
				if err := u.repository.SaveDepartment(transactionContext, department); err != nil {
					return err
				}
				seen[key] = true
				departmentsByOrganization[organization.ID][departmentName] = department
				result.Departments++
				progress = true
			}
			if len(next) > 0 && !progress {
				return entity.NewBusinessError("validation", "上级部门不存在或形成循环")
			}
			pending = next
		}
		return nil
	})
	if err != nil {
		return ImportOrganizationsOutput{}, err
	}
	return result, nil
}

func (u *SystemUseCase) ListDepartments(ctx context.Context, input ListDepartmentsInput) ([]DepartmentOutput, error) {
	if err := requirePermission(input.Actor, "SYS_ORGANIZATION:READ"); err != nil {
		return nil, err
	}
	if input.OrganizationID == "" {
		return nil, ErrValidation
	}
	items, err := u.repository.ListDepartments(ctx, input.OrganizationID)
	if err != nil {
		return nil, err
	}
	result := make([]DepartmentOutput, len(items))
	for index, item := range items {
		result[index] = DepartmentOutput{ID: item.ID, OrganizationID: item.OrganizationID, ParentID: item.ParentID, Name: item.Name, Sort: item.Sort}
	}
	return result, nil
}

func (u *SystemUseCase) SaveDepartment(ctx context.Context, input SaveDepartmentInput) (DepartmentOutput, error) {
	action := "ADD"
	if input.ID != "" {
		action = "UPDATE"
	}
	if err := requirePermission(input.Actor, "SYS_ORGANIZATION:"+action); err != nil {
		return DepartmentOutput{}, err
	}
	if input.OrganizationID == "" || strings.TrimSpace(input.Name) == "" || len([]rune(strings.TrimSpace(input.Name))) > 100 || input.Sort < 0 {
		return DepartmentOutput{}, ErrValidation
	}
	if input.Actor.OrganizationID != "" && input.Actor.OrganizationID != input.OrganizationID {
		return DepartmentOutput{}, ErrForbidden
	}
	departments, err := u.repository.ListDepartments(ctx, input.OrganizationID)
	if err != nil {
		return DepartmentOutput{}, err
	}
	byID := make(map[string]entity.Department, len(departments))
	for _, department := range departments {
		byID[department.ID] = department
	}
	if input.ID == "" {
		input.ID = entity.NewID()
	} else if _, found := byID[input.ID]; !found {
		return DepartmentOutput{}, ErrNotFound
	}
	if input.ParentID == input.ID {
		return DepartmentOutput{}, entity.NewBusinessError("validation", "部门不能设为自己的上级")
	}
	for parentID := input.ParentID; parentID != ""; {
		parent, found := byID[parentID]
		if !found {
			return DepartmentOutput{}, entity.NewBusinessError("validation", "上级部门不存在或不属于当前组织")
		}
		if parent.ID == input.ID {
			return DepartmentOutput{}, entity.NewBusinessError("validation", "部门层级不能形成循环")
		}
		parentID = parent.ParentID
	}
	item := entity.Department{ID: input.ID, OrganizationID: input.OrganizationID, ParentID: input.ParentID, Name: strings.TrimSpace(input.Name), Sort: input.Sort}
	if err := u.repository.SaveDepartment(ctx, item); err != nil {
		return DepartmentOutput{}, err
	}
	return DepartmentOutput{ID: item.ID, OrganizationID: item.OrganizationID, ParentID: item.ParentID, Name: item.Name, Sort: item.Sort}, nil
}

func (u *SystemUseCase) DeleteDepartment(ctx context.Context, input DeleteDepartmentInput) error {
	if err := requirePermission(input.Actor, "SYS_ORGANIZATION:DELETE"); err != nil {
		return err
	}
	if input.ID == "" || input.OrganizationID == "" {
		return ErrValidation
	}
	if input.Actor.OrganizationID != "" && input.Actor.OrganizationID != input.OrganizationID {
		return ErrForbidden
	}
	departments, err := u.repository.ListDepartments(ctx, input.OrganizationID)
	if err != nil {
		return err
	}
	for _, department := range departments {
		if department.ID == input.ID {
			return u.repository.DeleteDepartment(ctx, input.ID, input.OrganizationID)
		}
	}
	return ErrNotFound
}

func (u *SystemUseCase) ListRoles(ctx context.Context, input ListRolesInput) ([]RoleOutput, error) {
	if err := requirePermission(input.Actor, "SYSTEM_ROLE:READ"); err != nil {
		return nil, err
	}
	if input.OrganizationID == "" {
		return nil, ErrValidation
	}
	items, err := u.repository.ListRoles(ctx, input.OrganizationID)
	if err != nil {
		return nil, err
	}
	result := make([]RoleOutput, len(items))
	for index, item := range items {
		permissions := item.PermissionIDs
		if permissions == nil {
			permissions = []string{}
		}
		result[index] = RoleOutput{ID: item.ID, OrganizationID: item.OrganizationID, Name: item.Name, BuiltIn: item.BuiltIn, PermissionIDs: permissions, DataScope: item.DataScope, DepartmentIDs: item.DepartmentIDs, FieldRestrictions: item.FieldRestrictions}
	}
	return result, nil
}

func (u *SystemUseCase) SaveRole(ctx context.Context, input SaveRoleInput) (RoleOutput, error) {
	action := "ADD"
	if input.ID != "" {
		action = "UPDATE"
	}
	if err := requirePermission(input.Actor, "SYSTEM_ROLE:"+action); err != nil {
		return RoleOutput{}, err
	}
	if input.OrganizationID == "" || strings.TrimSpace(input.Name) == "" || len([]rune(strings.TrimSpace(input.Name))) > 100 {
		return RoleOutput{}, ErrValidation
	}
	organizations, err := u.repository.ListOrganizations(ctx)
	if err != nil {
		return RoleOutput{}, err
	}
	found := false
	for _, organization := range organizations {
		if organization.ID == input.OrganizationID {
			found = true
			break
		}
	}
	if !found {
		return RoleOutput{}, ErrValidation
	}
	seen := map[string]bool{}
	validPermissions := validPermissionIDs()
	moduleDefinitions, err := u.repository.ListModules(ctx)
	if err != nil {
		return RoleOutput{}, err
	}
	validModuleKeys := map[string]bool{}
	for key := range Modules {
		validModuleKeys[key] = true
	}
	for _, module := range moduleDefinitions {
		if module.Custom {
			validModuleKeys[module.Key] = true
		}
	}
	for _, permission := range input.PermissionIDs {
		if seen[permission] {
			return RoleOutput{}, ErrValidation
		}
		if !validPermissions[permission] {
			parts := strings.Split(permission, ".")
			if len(parts) != 2 || (parts[1] != "read" && parts[1] != "write") {
				return RoleOutput{}, ErrValidation
			}
			if !validModuleKeys[parts[0]] {
				return RoleOutput{}, ErrValidation
			}
		}
		seen[permission] = true
	}
	if input.DataScope == "" {
		input.DataScope = "self"
	}
	if !slices.Contains([]string{"all", "self", "department", "department_and_children", "specified_departments"}, input.DataScope) {
		return RoleOutput{}, ErrValidation
	}
	departments, err := u.repository.ListDepartments(ctx, input.OrganizationID)
	if err != nil {
		return RoleOutput{}, err
	}
	validDepartments := map[string]bool{}
	for _, department := range departments {
		validDepartments[department.ID] = true
	}
	departmentIDs := make([]string, 0, len(input.DepartmentIDs))
	seenDepartments := map[string]bool{}
	for _, departmentID := range input.DepartmentIDs {
		if !validDepartments[departmentID] || seenDepartments[departmentID] {
			return RoleOutput{}, ErrValidation
		}
		seenDepartments[departmentID] = true
		departmentIDs = append(departmentIDs, departmentID)
	}
	if input.DataScope == "specified_departments" && len(departmentIDs) == 0 {
		return RoleOutput{}, ErrValidation
	}
	fieldRestrictions := make(map[string]entity.FieldRestriction, len(input.FieldRestrictions))
	for moduleKey, restriction := range input.FieldRestrictions {
		definition, found := Modules[moduleKey]
		if !found {
			return RoleOutput{}, ErrValidation
		}
		fields := make(map[string]Field, len(definition.Fields))
		archived, err := u.archivedStandardFields(ctx, moduleKey)
		if err != nil {
			return RoleOutput{}, err
		}
		for _, field := range definition.Fields {
			if !archived[field.Key] {
				fields[field.Key] = field
			}
		}
		customFields, err := u.repository.ListCustomFields(ctx, moduleKey)
		if err != nil {
			return RoleOutput{}, err
		}
		for _, field := range customFields {
			if field.Definition["archived"] != true {
				fields[field.Key] = Field{Key: field.Key, Required: false}
			}
		}
		hidden := map[string]bool{}
		for _, key := range restriction.Hidden {
			field, exists := fields[key]
			if !exists || field.Required || hidden[key] {
				return RoleOutput{}, ErrValidation
			}
			hidden[key] = true
		}
		readOnly := map[string]bool{}
		for _, key := range restriction.ReadOnly {
			field, exists := fields[key]
			if !exists || field.Required || hidden[key] || readOnly[key] {
				return RoleOutput{}, ErrValidation
			}
			readOnly[key] = true
		}
		if len(hidden) > 0 || len(readOnly) > 0 {
			fieldRestrictions[moduleKey] = entity.FieldRestriction{Hidden: restriction.Hidden, ReadOnly: restriction.ReadOnly}
		}
	}
	if input.ID == "" {
		input.ID = entity.NewID()
	} else {
		existing, err := u.repository.FindRoleByID(ctx, input.ID)
		if err != nil || existing.OrganizationID != input.OrganizationID || existing.BuiltIn {
			return RoleOutput{}, ErrForbidden
		}
	}
	item := entity.SystemRole{ID: input.ID, OrganizationID: input.OrganizationID, Name: strings.TrimSpace(input.Name), PermissionIDs: input.PermissionIDs, DataScope: input.DataScope, DepartmentIDs: departmentIDs, FieldRestrictions: fieldRestrictions}
	if err := u.repository.SaveRole(ctx, item); err != nil {
		return RoleOutput{}, err
	}
	permissions := item.PermissionIDs
	if permissions == nil {
		permissions = []string{}
	}
	return RoleOutput{ID: item.ID, OrganizationID: item.OrganizationID, Name: item.Name, BuiltIn: item.BuiltIn, PermissionIDs: permissions, DataScope: item.DataScope, DepartmentIDs: item.DepartmentIDs, FieldRestrictions: item.FieldRestrictions}, nil
}

func (u *SystemUseCase) DeleteRole(ctx context.Context, input DeleteRoleInput) error {
	if err := requirePermission(input.Actor, "SYSTEM_ROLE:DELETE"); err != nil {
		return err
	}
	if input.ID == "" || input.OrganizationID == "" {
		return ErrValidation
	}
	role, err := u.repository.FindRoleByID(ctx, input.ID)
	if err != nil {
		return ErrNotFound
	}
	if role.OrganizationID != input.OrganizationID || role.BuiltIn {
		return ErrForbidden
	}
	return u.repository.DeleteRole(ctx, input.ID)
}

func (u *SystemUseCase) SaveModule(ctx context.Context, input SaveModuleInput) error {
	permission := "MODULE_SETTING:UPDATE"
	if input.Module.Custom {
		permission = "CUSTOM_FORM:ADD"
	}
	if err := requirePermission(input.Actor, permission); err != nil {
		return err
	}
	if input.Module.Key == "" || input.Module.Name == "" {
		return ErrValidation
	}
	if input.Module.ID == "" {
		input.Module.ID = entity.NewID()
	}
	definition, found := Modules[input.Module.Key]
	if input.Module.Custom {
		if err := validateCustomModule(&input.Module); err != nil {
			return err
		}
		return u.repository.SaveModule(ctx, input.Module)
	}
	if !found {
		return ErrValidation
	}
	if len(input.Module.Stages) == 0 && input.Module.Key == "opportunities" && len(input.Module.SalesStages) > 0 {
		for _, name := range input.Module.SalesStages {
			stageType, rate := "AFOOT", 50
			if name == "赢单" {
				stageType, rate = "END", 100
			}
			if name == "输单" {
				stageType, rate = "END", 0
			}
			input.Module.Stages = append(input.Module.Stages, entity.StageDefinition{Name: name, Type: stageType, Rate: rate})
		}
	}
	if (input.Module.Key == "opportunities" || input.Module.Key == "contracts") && len(input.Module.Stages) > 0 {
		if err := validateStageConfiguration(&input.Module); err != nil {
			return err
		}
	} else if len(input.Module.SalesStages) > 0 || len(input.Module.Stages) > 0 || len(input.Module.StageTransitions) > 0 {
		return ErrValidation
	}
	if len([]rune(input.Module.DocumentTemplate)) > 10000 {
		return ErrValidation
	}
	if input.Module.DocumentTemplate != "" {
		valid := map[string]bool{"id": true, "owner_name": true, "created_at": true, "updated_at": true}
		archived, err := u.archivedStandardFields(ctx, input.Module.Key)
		if err != nil {
			return err
		}
		for _, field := range definition.Fields {
			if !archived[field.Key] {
				valid[field.Key] = true
			}
		}
		customFields, err := u.repository.ListCustomFields(ctx, input.Module.Key)
		if err != nil {
			return err
		}
		for _, field := range customFields {
			if field.Definition["archived"] != true {
				valid[field.Key] = true
			}
		}
		placeholderPattern := regexp.MustCompile(`\{\{\s*([a-z][a-z0-9_]*)\s*\}\}`)
		for _, match := range placeholderPattern.FindAllStringSubmatch(input.Module.DocumentTemplate, -1) {
			if !valid[match[1]] {
				return ErrValidation
			}
		}
		if strings.Contains(placeholderPattern.ReplaceAllString(input.Module.DocumentTemplate, ""), "{{") || strings.Contains(placeholderPattern.ReplaceAllString(input.Module.DocumentTemplate, ""), "}}") {
			return ErrValidation
		}
	}
	if err := u.validateModuleLayout(ctx, input.Module); err != nil {
		return err
	}
	return u.repository.SaveModule(ctx, input.Module)
}

func (u *SystemUseCase) DeleteModule(ctx context.Context, input DeleteModuleInput) error {
	if err := requirePermission(input.Actor, "CUSTOM_FORM:ADD"); err != nil {
		return err
	}
	if input.ID == "" || !regexp.MustCompile(`^custom_[a-z][a-z0-9_]{1,35}$`).MatchString(input.Key) {
		return ErrValidation
	}
	return u.repository.DeleteModule(ctx, input.ID, input.Key)
}

func validateCustomModule(module *entity.ModuleDefinition) error {
	if !regexp.MustCompile(`^custom_[a-z][a-z0-9_]{1,35}$`).MatchString(module.Key) || strings.TrimSpace(module.Name) == "" || len([]rune(module.Name)) > 40 || len(module.Fields) == 0 || len(module.Fields) > 60 {
		return ErrValidation
	}
	module.Name = strings.TrimSpace(module.Name)
	module.Group = strings.TrimSpace(module.Group)
	if module.Group == "" {
		module.Group = "自定义业务"
	}
	if len([]rune(module.Group)) > 40 || len([]rune(module.Description)) > 200 {
		return ErrValidation
	}
	if module.ID == "" {
		module.ID = entity.NewID()
	}
	seen, hasName := map[string]bool{}, false
	for index := range module.Fields {
		field := &module.Fields[index]
		field.Key, field.Name = strings.TrimSpace(field.Key), strings.TrimSpace(field.Name)
		if !regexp.MustCompile(`^[a-z][a-z0-9_]{0,35}$`).MatchString(field.Key) || field.Name == "" || len([]rune(field.Name)) > 40 || seen[field.Key] || !slices.Contains([]string{"text", "textarea", "number", "money", "date", "email", "select"}, field.Type) || (field.Type == "select" && len(field.Options) == 0) {
			return ErrValidation
		}
		if field.Key == "name" {
			field.Type, field.Required, hasName = "text", true, true
		}
		if field.ID == "" {
			field.ID = entity.NewID()
		}
		field.ModuleID, field.Sort = module.ID, index+1
		seen[field.Key] = true
	}
	if !hasName {
		return entity.NewBusinessError("validation", "自定义模块必须包含标识为 name 的名称字段")
	}
	module.Layout = make([]entity.FieldLayout, len(module.Fields))
	for index, field := range module.Fields {
		module.Layout[index] = entity.FieldLayout{Key: field.Key, Group: "基本信息", Sort: index + 1}
	}
	return nil
}

func validateStageConfiguration(module *entity.ModuleDefinition) error {
	if len(module.Stages) < 2 || len(module.Stages) > 15 {
		return ErrValidation
	}
	seen, afoot, end := map[string]bool{}, 0, 0
	module.SalesStages = make([]string, len(module.Stages))
	for index := range module.Stages {
		stage := &module.Stages[index]
		stage.Name = strings.TrimSpace(stage.Name)
		if stage.Name == "" || len([]rune(stage.Name)) > 40 || seen[stage.Name] || !slices.Contains([]string{"AFOOT", "END"}, stage.Type) || stage.Rate < 0 || stage.Rate > 100 {
			return ErrValidation
		}
		if module.Key == "contracts" {
			stage.Rate = 0
		}
		seen[stage.Name] = true
		module.SalesStages[index] = stage.Name
		if stage.Type == "AFOOT" {
			afoot++
		} else {
			end++
		}
	}
	if afoot == 0 || end == 0 {
		return entity.NewBusinessError("validation", "阶段配置至少需要一个进行中阶段和一个完结阶段")
	}
	if module.CirculationType == "" {
		module.CirculationType = "NORMAL"
	}
	if !slices.Contains([]string{"NORMAL", "ADVANCED"}, module.CirculationType) {
		return ErrValidation
	}
	transitionSeen := map[string]bool{}
	for _, transition := range module.StageTransitions {
		key := transition.Origin + "\x00" + transition.Target
		if module.CirculationType != "ADVANCED" || transition.Origin == transition.Target || !seen[transition.Origin] || !seen[transition.Target] || transitionSeen[key] {
			return ErrValidation
		}
		transitionSeen[key] = true
	}
	if module.CirculationType == "ADVANCED" && len(module.StageTransitions) == 0 {
		return entity.NewBusinessError("validation", "高级流转至少需要配置一条允许的阶段路径")
	}
	return nil
}

func (u *SystemUseCase) validateModuleLayout(ctx context.Context, module entity.ModuleDefinition) error {
	definition, found := Modules[module.Key]
	if !found {
		return ErrValidation
	}
	fields := make(map[string]bool, len(definition.Fields))
	archived, err := u.archivedStandardFields(ctx, module.Key)
	if err != nil {
		return err
	}
	for _, field := range definition.Fields {
		if !archived[field.Key] {
			fields[field.Key] = true
		}
	}
	customFields, err := u.repository.ListCustomFields(ctx, module.Key)
	if err != nil {
		return err
	}
	for _, field := range customFields {
		if field.Definition["archived"] != true {
			fields[field.Key] = true
		}
	}
	seen := map[string]bool{}
	for index, field := range module.Layout {
		if !fields[field.Key] || seen[field.Key] || strings.TrimSpace(field.Group) == "" || len([]rune(field.Group)) > 40 || field.Sort != index+1 {
			return ErrValidation
		}
		seen[field.Key] = true
	}
	sort.SliceStable(module.Layout, func(left, right int) bool { return module.Layout[left].Sort < module.Layout[right].Sort })
	return nil
}

func (u *SystemUseCase) archivedStandardFields(ctx context.Context, moduleKey string) (map[string]bool, error) {
	settings, err := u.repository.ListStandardFieldSettings(ctx, moduleKey)
	if err != nil {
		return nil, err
	}
	archived := make(map[string]bool, len(settings))
	for _, setting := range settings {
		archived[setting.Key] = setting.Archived
	}
	return archived, nil
}
