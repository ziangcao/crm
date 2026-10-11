package usecase

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"cordialcrm/backend/internal/entity"
	"github.com/xuri/excelize/v2"
)

type recordTestRepository struct {
	Repository
	records []entity.Record
	modules []entity.ModuleDefinition
}

type customFieldRepositoryStub struct {
	Repository
	fields   []entity.CustomField
	flows    []entity.ApprovalFlow
	steps    []entity.ApprovalFlowStep
	settings []entity.StandardFieldSetting
}

type archivedFieldUpdateStub struct {
	*customFieldRepositoryStub
	record           entity.Record
	documentTemplate string
}

func (r *archivedFieldUpdateStub) ListModules(context.Context) ([]entity.ModuleDefinition, error) {
	return []entity.ModuleDefinition{{Key: "customers", Enabled: true, DocumentTemplate: r.documentTemplate}}, nil
}
func (r *archivedFieldUpdateStub) FindRecord(context.Context, string, string) (entity.Record, error) {
	return r.record, nil
}
func (r *archivedFieldUpdateStub) UpdateRecord(_ context.Context, record entity.Record, _ int) error {
	r.record = record
	return nil
}
func (r *archivedFieldUpdateStub) CreateAudit(context.Context, entity.Audit) error { return nil }

func (r *customFieldRepositoryStub) ListModules(context.Context) ([]entity.ModuleDefinition, error) {
	return []entity.ModuleDefinition{{Key: "custom_demo", Custom: true, Enabled: true, Fields: []entity.FieldDefinition{{Key: "name", Name: "名称", Type: "text", Required: true}}}}, nil
}
func (r *customFieldRepositoryStub) ListCustomFields(context.Context, string) ([]entity.CustomField, error) {
	return r.fields, nil
}
func (r *customFieldRepositoryStub) ListStandardFieldSettings(context.Context, string) ([]entity.StandardFieldSetting, error) {
	return r.settings, nil
}
func (r *customFieldRepositoryStub) SaveStandardFieldSetting(_ context.Context, setting entity.StandardFieldSetting) error {
	for index := range r.settings {
		if r.settings[index].Module == setting.Module && r.settings[index].Key == setting.Key {
			r.settings[index] = setting
			return nil
		}
	}
	r.settings = append(r.settings, setting)
	return nil
}
func (r *customFieldRepositoryStub) UpdateCustomField(_ context.Context, field entity.CustomField) error {
	for index := range r.fields {
		if r.fields[index].Key == field.Key {
			r.fields[index] = field
			return nil
		}
	}
	return ErrNotFound
}
func (r *customFieldRepositoryStub) ListApprovalFlows(context.Context, string) ([]entity.ApprovalFlow, error) {
	return r.flows, nil
}
func (r *customFieldRepositoryStub) FindCurrentApprovalFlowVersion(context.Context, string) (entity.ApprovalFlowVersion, error) {
	return entity.ApprovalFlowVersion{Steps: r.steps}, nil
}

func TestRenameAndArchiveCustomField(t *testing.T) {
	repository := &customFieldRepositoryStub{fields: []entity.CustomField{{Module: "custom_demo", Key: "custom_region", Definition: map[string]any{"label": "区域", "type": "text"}}}}
	service := NewRecord(repository, repository)
	admin := UserOutput{Role: entity.RoleAdmin}
	if err := service.RenameCustomField(context.Background(), admin, "custom_demo", "custom_region", " 新区域 "); err != nil {
		t.Fatal(err)
	}
	if repository.fields[0].Definition["label"] != "新区域" || repository.fields[0].Key != "custom_region" {
		t.Fatalf("rename changed the field key: %#v", repository.fields[0])
	}
	if err := service.DeleteCustomField(context.Background(), admin, "custom_demo", "custom_region"); err != nil {
		t.Fatal(err)
	}
	if repository.fields[0].Definition["archived"] != true {
		t.Fatal("field was not archived")
	}
	module, err := service.moduleWithCustomFields(context.Background(), "custom_demo")
	if err != nil || len(module.Fields) != 1 {
		t.Fatalf("archived field is still visible: %#v, %v", module.Fields, err)
	}
	if err := service.RenameCustomField(context.Background(), admin, "custom_demo", "custom_region", "再次改名"); err != ErrNotFound {
		t.Fatalf("archived field was renamed: %v", err)
	}
	if err := service.AddCustomField(context.Background(), AddCustomFieldInput{Actor: admin, Module: "custom_demo", Key: "custom_region", Label: "重复区域", Type: "text"}); err == nil {
		t.Fatal("archived field key was reused")
	}
}

func TestArchiveCustomFieldRejectsReferencesAndPermission(t *testing.T) {
	repository := &customFieldRepositoryStub{fields: []entity.CustomField{
		{Module: "custom_demo", Key: "custom_amount", Definition: map[string]any{"label": "金额", "type": "number"}},
		{Module: "custom_demo", Key: "custom_total", Definition: map[string]any{"label": "合计", "type": "formula", "sources": []any{"custom_amount"}}},
	}}
	service := NewRecord(repository, repository)
	if err := service.DeleteCustomField(context.Background(), UserOutput{RoleID: "role-1", PermissionIDs: []string{"CUSTOM_FORM:ADD"}}, "custom_demo", "custom_amount"); err != ErrForbidden {
		t.Fatalf("delete accepted add permission: %v", err)
	}
	admin := UserOutput{Role: entity.RoleAdmin}
	if err := service.DeleteCustomField(context.Background(), admin, "custom_demo", "custom_amount"); err == nil {
		t.Fatal("formula reference was ignored")
	}
	repository.fields[1].Definition["archived"] = true
	repository.flows = []entity.ApprovalFlow{{ID: "flow-1", Module: "custom_demo", Enabled: true}}
	repository.steps = []entity.ApprovalFlowStep{{Conditions: []entity.ApprovalCondition{{Field: "custom_amount", Operator: "eq", Value: "10"}}}}
	if err := service.DeleteCustomField(context.Background(), admin, "custom_demo", "custom_amount"); err == nil {
		t.Fatal("approval reference was ignored")
	}
}

func TestUpdatePreservesArchivedFieldValue(t *testing.T) {
	repository := &archivedFieldUpdateStub{
		customFieldRepositoryStub: &customFieldRepositoryStub{fields: []entity.CustomField{{Module: "customers", Key: "custom_region", Definition: map[string]any{"label": "区域", "type": "text", "archived": true}}}},
		record:                    entity.Record{ID: "customer-1", Module: "customers", Version: 1, Data: map[string]any{"name": "原客户", "industry": "互联网", "source": "官网咨询", "level": "B · 潜力", "status": "潜在客户", "custom_region": "华东"}},
	}
	service := NewRecord(repository, repository)
	_, err := service.Update(context.Background(), RecordInput{Actor: UserOutput{Role: entity.RoleAdmin}, Module: "customers", ID: "customer-1", Version: 1, Data: map[string]any{"name": "新客户", "industry": "互联网", "source": "官网咨询", "level": "B · 潜力", "status": "潜在客户"}})
	if err != nil {
		t.Fatal(err)
	}
	if repository.record.Data["custom_region"] != "华东" || repository.record.Data["name"] != "新客户" {
		t.Fatalf("archived value was lost: %#v", repository.record.Data)
	}
}

func TestStandardFieldRenameDisableAndPreserveValue(t *testing.T) {
	repository := &archivedFieldUpdateStub{
		customFieldRepositoryStub: &customFieldRepositoryStub{},
		record:                    entity.Record{ID: "customer-1", Module: "customers", Version: 1, Data: map[string]any{"name": "原客户", "industry": "互联网", "source": "官网咨询", "level": "B · 潜力", "status": "潜在客户", "website": "https://example.com"}},
	}
	service := NewRecord(repository, repository)
	admin := UserOutput{Role: entity.RoleAdmin}
	if err := service.RenameCustomField(context.Background(), admin, "customers", "website", "官网地址"); err != nil {
		t.Fatal(err)
	}
	module, err := service.moduleWithCustomFields(context.Background(), "customers")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, field := range module.Fields {
		if field.Key == "website" {
			found = field.Label == "官网地址" && field.Standard && field.Disableable
		}
	}
	if !found {
		t.Fatal("standard field name or controls missing from catalog")
	}
	if err := service.DeleteCustomField(context.Background(), admin, "customers", "website"); err != nil {
		t.Fatal(err)
	}
	module, err = service.moduleWithCustomFields(context.Background(), "customers")
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range module.Fields {
		if field.Key == "website" {
			t.Fatal("disabled standard field remained visible")
		}
	}
	_, err = service.Update(context.Background(), RecordInput{Actor: admin, Module: "customers", ID: "customer-1", Version: 1, Data: map[string]any{"name": "新客户", "industry": "互联网", "source": "官网咨询", "level": "B · 潜力", "status": "潜在客户"}})
	if err != nil || repository.record.Data["website"] != "https://example.com" {
		t.Fatalf("disabled standard field value was lost: %v, %#v", err, repository.record.Data)
	}
}

func TestStandardFieldDisableRejectsProtectedAndReferencedFields(t *testing.T) {
	repository := &archivedFieldUpdateStub{customFieldRepositoryStub: &customFieldRepositoryStub{}}
	service := NewRecord(repository, repository)
	admin := UserOutput{Role: entity.RoleAdmin}
	if err := service.DisableStandardField(context.Background(), admin, "customers", "name"); err == nil {
		t.Fatal("required standard field was disabled")
	}
	if standardFieldCanDisable("leads", Modules["leads"].Fields[3]) {
		t.Fatal("lead phone used by conversion was allowed to disable")
	}
	repository.flows = []entity.ApprovalFlow{{ID: "flow-1", Module: "customers"}}
	repository.steps = []entity.ApprovalFlowStep{{Conditions: []entity.ApprovalCondition{{Field: "website"}}}}
	if err := service.DisableStandardField(context.Background(), admin, "customers", "website"); err == nil {
		t.Fatal("field referenced by approval flow was disabled")
	}
	repository.flows = nil
	repository.documentTemplate = "{{website}}"
	if err := service.DisableStandardField(context.Background(), admin, "customers", "website"); err == nil {
		t.Fatal("field referenced by document template was disabled")
	}
}

func TestDetailOutputSerializesRecordReferences(t *testing.T) {
	output := DetailOutput{RecordOutput: RecordOutput{References: map[string]string{"customer_id": "星云科技"}}}
	payload, err := json.Marshal(output)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), `"references":{"customer_id":"星云科技"}`) {
		t.Fatalf("detail references missing from response: %s", payload)
	}
}

func TestOwnershipHistoryOutputUsesFrontendFieldNames(t *testing.T) {
	createdAt := time.Date(2026, time.September, 29, 9, 24, 26, 0, time.UTC)
	output := ListOwnershipHistoryOutput{Items: []OwnershipHistoryOutput{{
		ID: "history-1", PreviousOwnerID: "owner-1", PreviousOwnerName: "销售甲", OwnerID: "owner-2", OwnerName: "销售乙",
		OperatorID: "operator-1", OperatorName: "管理员", Action: "assign", CreatedAt: createdAt,
	}}}
	payload, err := json.Marshal(output)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		`"previous_owner_name":"销售甲"`, `"owner_name":"销售乙"`, `"operator_name":"管理员"`,
		`"action":"assign"`, `"created_at":"2026-09-29T09:24:26Z"`,
	} {
		if !strings.Contains(string(payload), expected) {
			t.Fatalf("ownership history field missing: %s in %s", expected, payload)
		}
	}
}

func TestFollowupCommentOutputUsesFrontendFieldNames(t *testing.T) {
	createdAt := time.Date(2026, time.October, 7, 6, 30, 0, 0, time.UTC)
	payload, err := json.Marshal(FollowupCommentOutput{ID: "comment-1", FollowupID: "followup-1", UserID: "user-1", UserName: "销售甲", Content: "已确认需求", CreatedAt: createdAt, UpdatedAt: createdAt})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{`"followup_id":"followup-1"`, `"user_id":"user-1"`, `"user_name":"销售甲"`, `"created_at":"2026-10-07T06:30:00Z"`} {
		if !strings.Contains(string(payload), expected) {
			t.Fatalf("followup comment field missing: %s in %s", expected, payload)
		}
	}
}

func TestStageTransitionRules(t *testing.T) {
	definitions := []entity.ModuleDefinition{{Key: "opportunities", Enabled: true, Stages: []entity.StageDefinition{{Name: "需求确认", Type: "AFOOT"}, {Name: "方案报价", Type: "AFOOT"}, {Name: "赢单", Type: "END"}}, CirculationType: "NORMAL"}}
	service := NewRecord(recordTestRepository{modules: definitions}, recordTestRepository{modules: definitions})
	if err := service.validateStageTransition(context.Background(), "opportunities", "需求确认", "方案报价"); err != nil {
		t.Fatalf("forward transition rejected: %v", err)
	}
	if err := service.validateStageTransition(context.Background(), "opportunities", "方案报价", "需求确认"); err == nil {
		t.Fatal("afoot rollback accepted while disabled")
	}
	definitions[0].AfootRollback = true
	definitions[0].EndRollback = true
	service = NewRecord(recordTestRepository{modules: definitions}, recordTestRepository{modules: definitions})
	if err := service.validateStageTransition(context.Background(), "opportunities", "方案报价", "需求确认"); err != nil {
		t.Fatalf("enabled afoot rollback rejected: %v", err)
	}
	if err := service.validateStageTransition(context.Background(), "opportunities", "赢单", "方案报价"); err != nil {
		t.Fatalf("enabled end rollback rejected: %v", err)
	}
	definitions[0].CirculationType = "ADVANCED"
	definitions[0].StageTransitions = []entity.StageTransition{{Origin: "需求确认", Target: "赢单"}}
	service = NewRecord(recordTestRepository{modules: definitions}, recordTestRepository{modules: definitions})
	if err := service.validateStageTransition(context.Background(), "opportunities", "需求确认", "赢单"); err != nil {
		t.Fatalf("configured advanced transition rejected: %v", err)
	}
	if err := service.validateStageTransition(context.Background(), "opportunities", "需求确认", "方案报价"); err == nil {
		t.Fatal("unconfigured advanced transition accepted")
	}
}

type poolListingRepository struct {
	Repository
	records []entity.Record
}

func (r *poolListingRepository) ListModules(_ context.Context) ([]entity.ModuleDefinition, error) {
	return []entity.ModuleDefinition{{Key: "leads", Enabled: true}}, nil
}
func (r *poolListingRepository) ListCustomFields(_ context.Context, _ string) ([]entity.CustomField, error) {
	return nil, nil
}
func (r *poolListingRepository) ListStandardFieldSettings(_ context.Context, _ string) ([]entity.StandardFieldSetting, error) {
	return nil, nil
}
func (r *poolListingRepository) ListRecords(_ context.Context, _ string) ([]entity.Record, error) {
	return r.records, nil
}
func (r *poolListingRepository) FindUserByID(_ context.Context, id string) (entity.User, error) {
	return entity.User{ID: id, Name: "销售甲"}, nil
}

type inventoryRepository struct {
	Repository
	products map[string]entity.Record
}

func (repository *inventoryRepository) FindRecord(_ context.Context, module, id string) (entity.Record, error) {
	if module != "products" {
		return entity.Record{}, ErrNotFound
	}
	product, found := repository.products[id]
	if !found {
		return entity.Record{}, ErrNotFound
	}
	data := make(map[string]any, len(product.Data))
	for key, value := range product.Data {
		data[key] = value
	}
	product.Data = data
	return product, nil
}

func (repository *inventoryRepository) UpdateRecord(_ context.Context, product entity.Record, version int) error {
	if product.Version != version {
		return ErrConflict
	}
	product.Version++
	repository.products[product.ID] = product
	return nil
}

type referencePermissionRepository struct {
	Repository
	item entity.Record
}

type approvalSubmissionRepository struct {
	Repository
	record     entity.Record
	approval   entity.Approval
	version    entity.ApprovalFlowVersion
	failSubmit bool
}

func (repository *approvalSubmissionRepository) FindRecord(_ context.Context, _, _ string) (entity.Record, error) {
	item := repository.record
	item.Data = make(map[string]any, len(repository.record.Data))
	for key, value := range repository.record.Data {
		item.Data[key] = value
	}
	return item, nil
}
func (repository *approvalSubmissionRepository) ListApprovalFlows(_ context.Context, _ string) ([]entity.ApprovalFlow, error) {
	return []entity.ApprovalFlow{{ID: "flow-1", Module: "quotes", Enabled: true, Version: 1}}, nil
}
func (repository *approvalSubmissionRepository) FindApprovalFlowVersion(_ context.Context, _ string, _ int) (entity.ApprovalFlowVersion, error) {
	return repository.version, nil
}
func (repository *approvalSubmissionRepository) CreateApproval(_ context.Context, item entity.Approval) error {
	repository.approval = item
	return nil
}
func (repository *approvalSubmissionRepository) SubmitApproval(_ context.Context, approval entity.Approval, record entity.Record, _ int) error {
	if repository.failSubmit {
		return errors.New("write failed")
	}
	repository.approval, repository.record = approval, record
	return nil
}
func (repository *approvalSubmissionRepository) UpdateRecord(_ context.Context, item entity.Record, _ int) error {
	repository.record = item
	return nil
}
func (repository *approvalSubmissionRepository) CreateAudit(_ context.Context, _ entity.Audit) error {
	return nil
}
func (repository *approvalSubmissionRepository) FindUserByID(_ context.Context, id string) (entity.User, error) {
	return entity.User{ID: id, Name: "提交人"}, nil
}

func (repository referencePermissionRepository) FindRecord(_ context.Context, _, _ string) (entity.Record, error) {
	return repository.item, nil
}

type importTestRepository struct {
	Repository
	records []entity.Record
}

type followupCommentTestRepository struct {
	Repository
	record   entity.Record
	followup entity.Followup
	comments []entity.FollowupComment
	user     entity.User
}

type ownershipHistoryTestRepository struct {
	Repository
	record    entity.Record
	histories []entity.OwnershipHistory
	user      entity.User
}

type specialActionRepository struct {
	Repository
	record    entity.Record
	contracts []entity.Record
	deleted   bool
}

func (r *specialActionRepository) ListCustomFields(_ context.Context, _ string) ([]entity.CustomField, error) {
	return nil, nil
}
func (r *specialActionRepository) ListStandardFieldSettings(_ context.Context, _ string) ([]entity.StandardFieldSetting, error) {
	return nil, nil
}

func (r *specialActionRepository) ListModules(_ context.Context) ([]entity.ModuleDefinition, error) {
	return []entity.ModuleDefinition{{Key: r.record.Module, Enabled: true}, {Key: "contracts", Enabled: true}}, nil
}

func (r *specialActionRepository) FindRecord(_ context.Context, module, id string) (entity.Record, error) {
	if r.record.Module != "customers" && module == "customers" && id == "customer-1" {
		return entity.Record{ID: id, Module: module, OrganizationID: r.record.OrganizationID, Data: map[string]any{"name": "客户"}}, nil
	}
	if module != r.record.Module || id != r.record.ID {
		return entity.Record{}, ErrNotFound
	}
	item := r.record
	item.Data = make(map[string]any, len(r.record.Data))
	for key, value := range r.record.Data {
		item.Data[key] = value
	}
	return item, nil
}
func (r *specialActionRepository) ListRecords(_ context.Context, module string) ([]entity.Record, error) {
	if module == "contracts" {
		return r.contracts, nil
	}
	return nil, nil
}
func (r *specialActionRepository) UpdateRecord(_ context.Context, item entity.Record, version int) error {
	if r.record.Version != version {
		return ErrConflict
	}
	item.Version = version + 1
	r.record = item
	return nil
}
func (r *specialActionRepository) DeleteRecord(_ context.Context, module, id string, version int) error {
	if r.record.Module != module || r.record.ID != id || r.record.Version != version {
		return ErrConflict
	}
	r.deleted = true
	return nil
}
func (r *specialActionRepository) CreateAudit(_ context.Context, _ entity.Audit) error { return nil }
func (r *specialActionRepository) FindUserByID(_ context.Context, id string) (entity.User, error) {
	return entity.User{ID: id, Name: "销售"}, nil
}

type mergeCustomerRepository struct {
	Repository
	records         map[string][]entity.Record
	movedActivities map[string]string
}

func (r *mergeCustomerRepository) ListModules(_ context.Context) ([]entity.ModuleDefinition, error) {
	return []entity.ModuleDefinition{{Key: "customers", Enabled: true}, {Key: "contacts", Enabled: true}}, nil
}
func (r *mergeCustomerRepository) FindRecord(_ context.Context, module, id string) (entity.Record, error) {
	for _, item := range r.records[module] {
		if item.ID == id {
			return item, nil
		}
	}
	return entity.Record{}, ErrNotFound
}
func (r *mergeCustomerRepository) ListRecords(_ context.Context, module string) ([]entity.Record, error) {
	return r.records[module], nil
}
func (r *mergeCustomerRepository) UpdateRecord(_ context.Context, item entity.Record, version int) error {
	for index, existing := range r.records[item.Module] {
		if existing.ID == item.ID && existing.Version == version {
			item.Version++
			r.records[item.Module][index] = item
			return nil
		}
	}
	return ErrConflict
}
func (r *mergeCustomerRepository) DeleteRecord(_ context.Context, module, id string, version int) error {
	for index, item := range r.records[module] {
		if item.ID == id && item.Version == version {
			r.records[module] = append(r.records[module][:index], r.records[module][index+1:]...)
			return nil
		}
	}
	return ErrConflict
}
func (r *mergeCustomerRepository) MoveRecordActivities(_ context.Context, _, sourceID, targetID string) error {
	r.movedActivities[sourceID] = targetID
	return nil
}
func (r *mergeCustomerRepository) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}
func (r *mergeCustomerRepository) CreateAudit(_ context.Context, _ entity.Audit) error { return nil }
func (r *mergeCustomerRepository) FindUserByID(_ context.Context, id string) (entity.User, error) {
	return entity.User{ID: id, Name: "销售"}, nil
}

type followupActivityRepository struct {
	Repository
	record     entity.Record
	followups  []entity.Followup
	failUpdate bool
}

func (r *followupActivityRepository) ListModules(_ context.Context) ([]entity.ModuleDefinition, error) {
	return []entity.ModuleDefinition{{Key: "customers", Enabled: true}}, nil
}
func (r *followupActivityRepository) FindRecord(_ context.Context, _, _ string) (entity.Record, error) {
	return r.record, nil
}
func (r *followupActivityRepository) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	followups, record := append([]entity.Followup(nil), r.followups...), r.record
	if err := fn(ctx); err != nil {
		r.followups, r.record = followups, record
		return err
	}
	return nil
}
func (r *followupActivityRepository) CreateFollowup(_ context.Context, item entity.Followup) error {
	r.followups = append(r.followups, item)
	return nil
}
func (r *followupActivityRepository) UpdateRecord(_ context.Context, item entity.Record, _ int) error {
	if r.failUpdate {
		return errors.New("update failed")
	}
	r.record = item
	return nil
}
func (r *followupActivityRepository) CreateAudit(_ context.Context, _ entity.Audit) error { return nil }

func (r *ownershipHistoryTestRepository) FindRecord(_ context.Context, _, _ string) (entity.Record, error) {
	return r.record, nil
}
func (r *ownershipHistoryTestRepository) UpdateRecord(_ context.Context, item entity.Record, version int) error {
	if version != r.record.Version {
		return errors.New("version conflict")
	}
	r.record = item
	r.record.Version = version + 1
	return nil
}
func (r *ownershipHistoryTestRepository) ListModules(_ context.Context) ([]entity.ModuleDefinition, error) {
	return []entity.ModuleDefinition{{Key: "customers", Enabled: true}}, nil
}
func (r *ownershipHistoryTestRepository) ListRecords(_ context.Context, _ string) ([]entity.Record, error) {
	return nil, nil
}
func (r *ownershipHistoryTestRepository) FindUserByID(_ context.Context, id string) (entity.User, error) {
	if id == r.user.ID {
		return r.user, nil
	}
	return entity.User{}, errors.New("not found")
}
func (r *ownershipHistoryTestRepository) CreateOwnershipHistory(_ context.Context, item entity.OwnershipHistory) error {
	r.histories = append(r.histories, item)
	return nil
}
func (r *ownershipHistoryTestRepository) ListOwnershipHistory(_ context.Context, _, _ string) ([]entity.OwnershipHistory, error) {
	return r.histories, nil
}
func (r *ownershipHistoryTestRepository) CreateAudit(_ context.Context, _ entity.Audit) error {
	return nil
}

func (r *followupCommentTestRepository) ListModules(_ context.Context) ([]entity.ModuleDefinition, error) {
	return []entity.ModuleDefinition{{Key: "customers", Enabled: true}}, nil
}
func (r *followupCommentTestRepository) FindRecord(_ context.Context, _, id string) (entity.Record, error) {
	if id != r.record.ID {
		return entity.Record{}, errors.New("not found")
	}
	return r.record, nil
}
func (r *followupCommentTestRepository) FindFollowup(_ context.Context, _ string) (entity.Followup, error) {
	return r.followup, nil
}
func (r *followupCommentTestRepository) CreateFollowupComment(_ context.Context, item entity.FollowupComment) error {
	r.comments = append(r.comments, item)
	return nil
}
func (r *followupCommentTestRepository) ListFollowupComments(_ context.Context, _ string) ([]entity.FollowupComment, error) {
	return r.comments, nil
}
func (r *followupCommentTestRepository) FindFollowupComment(_ context.Context, id string) (entity.FollowupComment, error) {
	for _, item := range r.comments {
		if item.ID == id {
			return item, nil
		}
	}
	return entity.FollowupComment{}, errors.New("not found")
}
func (r *followupCommentTestRepository) UpdateFollowupComment(_ context.Context, updated entity.FollowupComment) error {
	for index, item := range r.comments {
		if item.ID == updated.ID {
			r.comments[index] = updated
			return nil
		}
	}
	return errors.New("not found")
}
func (r *followupCommentTestRepository) DeleteFollowupComment(_ context.Context, id string) error {
	for index, item := range r.comments {
		if item.ID == id {
			r.comments = append(r.comments[:index], r.comments[index+1:]...)
			return nil
		}
	}
	return errors.New("not found")
}
func (r *followupCommentTestRepository) FindUserByID(_ context.Context, _ string) (entity.User, error) {
	return r.user, nil
}

func (r *importTestRepository) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	snapshot := append([]entity.Record(nil), r.records...)
	if err := fn(ctx); err != nil {
		r.records = snapshot
		return err
	}
	return nil
}
func (r *importTestRepository) ListModules(_ context.Context) ([]entity.ModuleDefinition, error) {
	items := make([]entity.ModuleDefinition, 0, len(Modules))
	for key := range Modules {
		items = append(items, entity.ModuleDefinition{Key: key, Enabled: true})
	}
	return items, nil
}
func (r *importTestRepository) ListCustomFields(_ context.Context, _ string) ([]entity.CustomField, error) {
	return nil, nil
}
func (r *importTestRepository) ListStandardFieldSettings(_ context.Context, _ string) ([]entity.StandardFieldSetting, error) {
	return nil, nil
}
func (r *importTestRepository) CreateRecord(_ context.Context, item entity.Record) error {
	r.records = append(r.records, item)
	return nil
}
func (r *importTestRepository) CreateAudit(_ context.Context, _ entity.Audit) error { return nil }
func (r *importTestRepository) FindUserByID(_ context.Context, _ string) (entity.User, error) {
	return entity.User{}, errors.New("not found")
}

func (r recordTestRepository) ListModules(_ context.Context) ([]entity.ModuleDefinition, error) {
	if r.modules != nil {
		return r.modules, nil
	}
	items := make([]entity.ModuleDefinition, 0, len(Modules))
	for key := range Modules {
		items = append(items, entity.ModuleDefinition{Key: key, Enabled: true})
	}
	return items, nil
}

func TestListRecordsRejectsDisabledModule(t *testing.T) {
	records := NewRecord(recordTestRepository{modules: []entity.ModuleDefinition{{Key: "leads", Enabled: false}}}, recordTestRepository{modules: []entity.ModuleDefinition{{Key: "leads", Enabled: false}}})
	_, err := records.List(context.Background(), ListRecordsInput{Actor: UserOutput{ID: "admin", Role: entity.RoleAdmin}, Module: "leads"})
	var business *entity.BusinessError
	if !errors.As(err, &business) || business.Code != "not_found" || business.Message != "模块已停用" {
		t.Fatalf("expected disabled module error, got %v", err)
	}
}

func TestLeadPoolAndBusinessListsAreMutuallyExclusive(t *testing.T) {
	owner := "user-1"
	repository := &poolListingRepository{records: []entity.Record{
		{ID: "owned", Module: "leads", OwnerID: &owner, Data: map[string]any{"name": "已领取线索"}},
		{ID: "pool", Module: "leads", Data: map[string]any{"name": "公海线索"}},
	}}
	service := NewRecord(repository, repository)
	actor := UserOutput{ID: owner, Role: entity.RoleAdmin}
	business, err := service.List(context.Background(), ListRecordsInput{Actor: actor, Module: "leads", Scope: "all"})
	if err != nil || business.Total != 1 || business.Items[0].ID != "owned" {
		t.Fatalf("unexpected business list: %#v err=%v", business, err)
	}
	pool, err := service.List(context.Background(), ListRecordsInput{Actor: actor, Module: "leads", Scope: "pool"})
	if err != nil || pool.Total != 1 || pool.Items[0].ID != "pool" {
		t.Fatalf("unexpected pool list: %#v err=%v", pool, err)
	}
}

func TestCustomRolePermissionAndOrganizationIsolation(t *testing.T) {
	actor := UserOutput{ID: "seller", Role: entity.RoleSales, OrganizationID: "org-a", RoleID: "role-a", PermissionIDs: []string{"customers.read"}}
	owner := actor.ID
	definition := Modules["customers"]
	if !visible(actor, definition, entity.Record{Module: "customers", OrganizationID: "org-a", OwnerID: &owner}, false) {
		t.Fatal("assigned member should read own organization record")
	}
	if visible(actor, definition, entity.Record{Module: "customers", OrganizationID: "org-b", OwnerID: &owner}, false) {
		t.Fatal("cross-organization record leaked")
	}
	if visible(actor, definition, entity.Record{Module: "customers", OrganizationID: "org-a", OwnerID: &owner}, true) {
		t.Fatal("read permission allowed write")
	}
	if allowed(actor, "products", false) {
		t.Fatal("ungranted module was readable")
	}
	actor.PermissionIDs = []string{"customers.write"}
	if !allowed(actor, "customers", false) || !visible(actor, definition, entity.Record{Module: "customers", OrganizationID: "org-a", OwnerID: &owner}, true) {
		t.Fatal("write permission should allow own record read and write")
	}
	if !allowed(UserOutput{ID: "legacy", Role: entity.RoleSales}, "customers", true) {
		t.Fatal("legacy member lost existing access")
	}
}

func TestUpstreamOperationPermissionsAreIndependentAndLegacyCompatible(t *testing.T) {
	actor := UserOutput{Role: entity.RoleSales, RoleID: "role-a", PermissionIDs: []string{"CUSTOMER_MANAGEMENT:READ", "CUSTOMER_MANAGEMENT:ADD", "CUSTOMER_MANAGEMENT_POOL:PICK"}}
	if !HasModulePermission(actor, "customers", "READ", false) || !HasModulePermission(actor, "customers", "ADD", false) {
		t.Fatal("explicit read and add permissions should be granted")
	}
	if HasModulePermission(actor, "customers", "UPDATE", false) || HasModulePermission(actor, "customers", "DELETE", false) || HasModulePermission(actor, "customers", "EXPORT", false) {
		t.Fatal("ungranted operations must remain forbidden")
	}
	if !HasModulePermission(actor, "customers", "PICK", true) || HasModulePermission(actor, "customers", "ASSIGN", true) {
		t.Fatal("pool special permissions were not enforced independently")
	}
	legacy := UserOutput{Role: entity.RoleSales, RoleID: "role-a", PermissionIDs: []string{"customers.write"}}
	for _, action := range []string{"READ", "ADD", "UPDATE", "DELETE", "IMPORT", "EXPORT", "TRANSFER", "RECYCLE"} {
		if !HasModulePermission(legacy, "customers", action, false) {
			t.Fatalf("legacy write permission lost %s compatibility", action)
		}
	}
}

func TestCustomRoleDataScopesLimitRecordVisibility(t *testing.T) {
	owner := "owner"
	actor := UserOutput{ID: "seller", Role: entity.RoleSales, RoleID: "role-a", OrganizationID: "org-a", PermissionIDs: []string{"customers.write"}, Department: "sales", DepartmentIDs: []string{"sales", "east"}}
	definition := Modules["customers"]
	record := entity.Record{Module: "customers", OrganizationID: "org-a", OwnerID: &owner, OwnerDepartment: "east"}
	actor.DataScope = "department_and_children"
	if !visible(actor, definition, record, false) || !visible(actor, definition, record, true) {
		t.Fatal("department descendants should be readable and writable")
	}
	actor.DataScope = "department"
	if visible(actor, definition, record, false) {
		t.Fatal("sibling department should not be visible")
	}
	actor.DataScope = "specified_departments"
	actor.DepartmentIDs = []string{"east"}
	if !visible(actor, definition, record, false) {
		t.Fatal("specified department should be visible")
	}
	actor.DataScope = "self"
	if visible(actor, definition, record, false) {
		t.Fatal("other owner should not be visible under self scope")
	}
	actor.DataScope = "all"
	if !visible(actor, definition, record, false) {
		t.Fatal("all scope should be visible")
	}
}

func TestRecordOutputHidesUnauthorizedReferenceName(t *testing.T) {
	repository := referencePermissionRepository{item: entity.Record{ID: "customer-a", Module: "customers", OrganizationID: "org-a", Data: map[string]any{"name": "机密客户"}}}
	service := NewRecord(repository, nil)
	actor := UserOutput{ID: "seller", Role: entity.RoleSales, OrganizationID: "org-a", RoleID: "role-a", PermissionIDs: []string{"opportunities.read"}}
	item := entity.Record{ID: "opportunity-a", Module: "opportunities", OrganizationID: "org-a", Data: map[string]any{"customer_id": "customer-a"}}
	output, err := service.output(context.Background(), actor, item)
	if err != nil || len(output.References) != 0 {
		t.Fatalf("reference name leaked: %#v, %v", output.References, err)
	}
}

func TestSubmitApprovalStartsAtFirstApplicableAmountStep(t *testing.T) {
	owner := "seller"
	repository := &approvalSubmissionRepository{
		record:  entity.Record{ID: "quote-1", Module: "quotes", OwnerID: &owner, Version: 1, Data: map[string]any{"name": "报价", "amount": "50000", "approval_status": "draft"}},
		version: entity.ApprovalFlowVersion{FlowID: "flow-1", Version: 1, Steps: []entity.ApprovalFlowStep{{Order: 1, ApproverRoles: []entity.Role{entity.RoleManager}, MinAmount: "100000"}, {Order: 2, ApproverRoles: []entity.Role{entity.RoleAdmin}}}},
	}
	_, err := NewRecord(repository, nil).Action(context.Background(), ActionInput{Actor: UserOutput{ID: owner, Role: entity.RoleSales}, Module: "quotes", ID: "quote-1", Action: "submit", Version: 1})
	if err != nil {
		t.Fatal(err)
	}
	if repository.approval.CurrentStep != 2 || repository.record.Data["approval_status"] != "pending" {
		t.Fatalf("incorrect initial approval step: %#v", repository.approval)
	}
}

func TestSubmitApprovalSkipsUnmatchedFieldCondition(t *testing.T) {
	owner := "seller"
	repository := &approvalSubmissionRepository{
		record:  entity.Record{ID: "quote-1", Module: "quotes", OwnerID: &owner, Version: 1, Data: map[string]any{"name": "报价", "amount": "50000", "approval_status": "draft", "custom_region": "华南"}},
		version: entity.ApprovalFlowVersion{FlowID: "flow-1", Version: 1, Steps: []entity.ApprovalFlowStep{{Order: 1, ApproverRoles: []entity.Role{entity.RoleManager}, Conditions: []entity.ApprovalCondition{{Field: "custom_region", Operator: "eq", Value: "华东"}}}, {Order: 2, ApproverRoles: []entity.Role{entity.RoleAdmin}}}},
	}
	_, err := NewRecord(repository, nil).Action(context.Background(), ActionInput{Actor: UserOutput{ID: owner, Role: entity.RoleSales}, Module: "quotes", ID: "quote-1", Action: "submit", Version: 1})
	if err != nil {
		t.Fatal(err)
	}
	if repository.approval.CurrentStep != 2 {
		t.Fatalf("unmatched field condition should start at step two: %#v", repository.approval)
	}
}

func TestSubmitApprovalFailureDoesNotPersistApproval(t *testing.T) {
	owner := "seller"
	repository := &approvalSubmissionRepository{record: entity.Record{ID: "quote-1", Module: "quotes", OwnerID: &owner, Version: 1, Data: map[string]any{"amount": "50000", "approval_status": "draft"}}, failSubmit: true}
	_, err := NewRecord(repository, nil).Action(context.Background(), ActionInput{Actor: UserOutput{ID: owner, Role: entity.RoleSales}, Module: "quotes", ID: "quote-1", Action: "submit", Version: 1})
	if err == nil || repository.approval.ID != "" || repository.record.Data["approval_status"] != "draft" {
		t.Fatalf("failed submit changed stored state: %#v, %#v, %v", repository.approval, repository.record, err)
	}
}

func TestImportRecordsRollsBackOnInvalidRecord(t *testing.T) {
	repository := &importTestRepository{}
	records := NewRecord(repository, repository)
	_, err := records.Import(context.Background(), ImportRecordsInput{Actor: UserOutput{ID: "admin", Role: entity.RoleAdmin}, Module: "leads", Records: []map[string]any{
		{"name": "有效线索", "company": "星云科技", "contact": "张三", "source": "官网咨询", "industry": "互联网", "status": "待跟进"},
		{"name": "无效线索"},
	}})
	if err == nil {
		t.Fatal("expected invalid import error")
	}
	if len(repository.records) != 0 {
		t.Fatalf("expected rollback, got %d stored records", len(repository.records))
	}
}

func TestImportCSVMappedFromExportHeaders(t *testing.T) {
	repository := &importTestRepository{}
	records := NewRecord(repository, repository)
	output, err := records.ImportCSV(context.Background(), UserOutput{ID: "admin", Role: entity.RoleAdmin}, "leads", []byte("ID,名称,公司,联系人,来源,行业,状态,负责人\nlead-1,有效线索,星云科技,张三,官网咨询,互联网,待跟进,管理员\n"))
	if err != nil {
		t.Fatalf("import csv: %v", err)
	}
	if output.Count != 1 || len(repository.records) != 1 || repository.records[0].Data["name"] != "有效线索" {
		t.Fatalf("unexpected imported records: %#v, %#v", output, repository.records)
	}
}

func TestImportTemplateUsesFieldKeysAndWritePermission(t *testing.T) {
	repository := recordTestRepository{}
	service := NewRecord(repository, repository)
	output, err := service.ImportTemplate(context.Background(), UserOutput{Role: entity.RoleAdmin}, "leads")
	if err != nil {
		t.Fatalf("create import template: %v", err)
	}
	book, err := excelize.OpenReader(bytes.NewReader(output.Content))
	if err != nil {
		t.Fatalf("open import template: %v", err)
	}
	defer book.Close()
	rows, err := book.GetRows(book.GetSheetName(0))
	if err != nil || len(rows) != 1 || len(rows[0]) == 0 || rows[0][0] != "name" {
		t.Fatalf("unexpected import template: %#v, %v", rows, err)
	}
	if _, err := service.ImportTemplate(context.Background(), UserOutput{Role: entity.RoleSales, RoleID: "role-a", PermissionIDs: []string{"leads.read"}}, "leads"); err != ErrForbidden {
		t.Fatalf("read-only member downloaded import template: %v", err)
	}
}

func TestImportCSVRollsBackOnInvalidRecord(t *testing.T) {
	repository := &importTestRepository{}
	records := NewRecord(repository, repository)
	_, err := records.ImportCSV(context.Background(), UserOutput{ID: "admin", Role: entity.RoleAdmin}, "leads", []byte("name,company,contact,source,industry,status\n有效线索,星云科技,张三,官网咨询,互联网,待跟进\n无效线索,,,,,\n"))
	if err == nil {
		t.Fatal("expected invalid csv import error")
	}
	if len(repository.records) != 0 {
		t.Fatalf("expected rollback, got %d stored records", len(repository.records))
	}
}

func TestPreviewImportRollsBackValidRecords(t *testing.T) {
	repository := &importTestRepository{}
	records := NewRecord(repository, repository)
	output, err := records.PreviewImport(context.Background(), ImportRecordsInput{Actor: UserOutput{ID: "admin", Role: entity.RoleAdmin}, Module: "leads", Records: []map[string]any{{"name": "有效线索", "company": "星云科技", "contact": "张三", "source": "官网咨询", "industry": "互联网", "status": "待跟进"}}})
	if err != nil {
		t.Fatalf("preview import: %v", err)
	}
	if output.Count != 1 || len(repository.records) != 0 {
		t.Fatalf("preview must not persist records: %#v, %#v", output, repository.records)
	}
}

func (r recordTestRepository) ListRecords(_ context.Context, _ string) ([]entity.Record, error) {
	return r.records, nil
}

func (r recordTestRepository) ListCustomFields(_ context.Context, _ string) ([]entity.CustomField, error) {
	return nil, nil
}
func (r recordTestRepository) ListStandardFieldSettings(_ context.Context, _ string) ([]entity.StandardFieldSetting, error) {
	return nil, nil
}
func (r recordTestRepository) FindUserByID(_ context.Context, id string) (entity.User, error) {
	return entity.User{ID: id, Name: "负责人"}, nil
}

func TestNormalizeLineItems(t *testing.T) {
	items, amount, err := normalizeLineItems([]any{map[string]any{"product_id": "product-1", "name": "企业版", "quantity": "2.5", "unit_price": "12.34"}})
	if err != nil {
		t.Fatalf("normalize line items: %v", err)
	}
	if amount.StringFixed(2) != "30.85" {
		t.Fatalf("unexpected amount: %s", amount.StringFixed(2))
	}
	item := items[0].(map[string]any)
	if item["unit_price"] != "12.34" || item["subtotal"] != "30.85" || item["quantity"] != "2.5" {
		t.Fatalf("unexpected normalized item: %#v", item)
	}
}

func TestFieldRestrictionsHideDataAndRejectWrites(t *testing.T) {
	actor := UserOutput{Role: entity.RoleSales, RoleID: "role-1", FieldRestrictions: map[string]entity.FieldRestriction{"customers": {Hidden: []string{"phone"}, ReadOnly: []string{"website"}}}}
	data := filteredData(actor, "customers", map[string]any{"name": "星云科技", "phone": "13800000000", "website": "https://example.com"})
	if _, found := data["phone"]; found || data["website"] != "https://example.com" {
		t.Fatalf("unexpected filtered data: %#v", data)
	}
	if err := validateFieldWrite(actor, Modules["customers"], map[string]any{"phone": "13900000000"}); err != ErrForbidden {
		t.Fatalf("hidden field write: %v", err)
	}
	if err := validateFieldWrite(actor, Modules["customers"], map[string]any{"website": "https://new.example.com"}); err != ErrForbidden {
		t.Fatalf("read-only field write: %v", err)
	}
	if err := validateFieldWrite(actor, Modules["customers"], map[string]any{"address": "上海"}); err != nil {
		t.Fatalf("allowed field write: %v", err)
	}
}

func TestFieldRestrictionsFilterCatalogAndSearchData(t *testing.T) {
	actor := UserOutput{Role: entity.RoleSales, RoleID: "role-1", FieldRestrictions: map[string]entity.FieldRestriction{"customers": {Hidden: []string{"phone"}, ReadOnly: []string{"website"}}}}
	module := restrictedModule(actor, Modules["customers"])
	for _, field := range module.Fields {
		if field.Key == "phone" {
			t.Fatal("hidden field exposed by catalog")
		}
		if field.Key == "website" && !field.ReadOnly {
			t.Fatal("read-only field missing catalog marker")
		}
	}
	searchData := filteredData(actor, "customers", map[string]any{"name": "星云科技", "phone": "secret"})
	if len(searchData) != 1 || searchData["name"] != "星云科技" {
		t.Fatal("hidden field exposed to search data")
	}
}

func TestApplyModuleLayoutOrdersAndGroupsFields(t *testing.T) {
	module := applyModuleLayout(Modules["customers"], []entity.FieldLayout{{Key: "website", Group: "补充资料", Sort: 1}, {Key: "name", Group: "基本信息", Sort: 2}})
	if module.Fields[0].Key != "website" || module.Fields[0].Group != "补充资料" || module.Fields[1].Key != "name" || module.Fields[1].Group != "基本信息" {
		t.Fatalf("unexpected module layout: %#v", module.Fields[:2])
	}
}

func TestValidateCalculatesFormulaAndRejectsClientValue(t *testing.T) {
	module := Module{Fields: []Field{f("name", "名称", "text", true), {Key: "base", Label: "基础金额", Type: "money", Required: true}, {Key: "extra", Label: "附加金额", Type: "money", Required: true}, {Key: "total", Label: "合计", Type: "formula", Sources: []string{"base", "extra"}}}}
	data, err := validate(module, map[string]any{"name": "示例", "base": "12.30", "extra": "7.70"})
	if err != nil || data["total"] != "20.00" {
		t.Fatalf("unexpected formula result: %#v, %v", data, err)
	}
	if _, err := validate(module, map[string]any{"name": "示例", "base": "12.30", "extra": "7.70", "total": "1"}); err == nil {
		t.Fatal("formula value supplied by client was accepted")
	}
}

func TestValidateSubtableRows(t *testing.T) {
	module := Module{Fields: []Field{f("name", "名称", "text", true), {Key: "detail", Label: "明细", Type: "subtable", Columns: []Field{{Key: "item", Label: "项目", Type: "text", Required: true}, {Key: "amount", Label: "金额", Type: "money", Required: true}}}}}
	data, err := validate(module, map[string]any{"name": "示例", "detail": []any{map[string]any{"item": "服务", "amount": "12.30"}}})
	if err != nil {
		t.Fatalf("validate subtable: %v", err)
	}
	rows := data["detail"].([]any)
	if rows[0].(map[string]any)["amount"] != "12.30" {
		t.Fatalf("unexpected subtable: %#v", data)
	}
	if _, err := validate(module, map[string]any{"name": "示例", "detail": []any{map[string]any{"item": "服务", "unknown": "x"}}}); err == nil {
		t.Fatal("invalid subtable row was accepted")
	}
}

func TestNormalizeLineItemsAcceptsLegacyPrice(t *testing.T) {
	items, amount, err := normalizeLineItems([]any{map[string]any{"product_id": "product-1", "quantity": 3, "price": "10"}})
	if err != nil {
		t.Fatalf("normalize legacy item: %v", err)
	}
	item := items[0].(map[string]any)
	if item["unit_price"] != "10.00" || item["subtotal"] != "30.00" || amount.StringFixed(2) != "30.00" {
		t.Fatalf("unexpected normalized legacy item: %#v, %s", item, amount.StringFixed(2))
	}
}

func TestNormalizeLineItemsCalculatesDiscountAndTax(t *testing.T) {
	items, amount, err := normalizeLineItems([]any{map[string]any{"product_id": "product-1", "quantity": "2", "unit_price": "100", "discount_rate": "10", "tax_rate": "13", "specification": "专业版 / 1年"}})
	if err != nil {
		t.Fatalf("normalize discounted line item: %v", err)
	}
	item := items[0].(map[string]any)
	if item["discount_amount"] != "20.00" || item["tax_amount"] != "23.40" || item["subtotal"] != "203.40" || amount.StringFixed(2) != "203.40" || item["specification"] != "专业版 / 1年" {
		t.Fatalf("unexpected discounted line item: %#v, %s", item, amount.StringFixed(2))
	}
	for _, rate := range []any{"-1", "101", "not-number"} {
		if _, _, err := normalizeLineItems([]any{map[string]any{"product_id": "product-1", "quantity": "1", "unit_price": "100", "discount_rate": rate}}); err == nil {
			t.Fatalf("invalid discount rate accepted: %v", rate)
		}
	}
}

func TestOrderInventoryReservationReleaseAndDelivery(t *testing.T) {
	repository := &inventoryRepository{products: map[string]entity.Record{"product-1": {ID: "product-1", Module: "products", Version: 1, Data: map[string]any{"stock": float64(10), "reserved_stock": float64(0)}}}}
	service := &RecordUseCase{repo: repository}
	items := []any{map[string]any{"product_id": "product-1", "quantity": "3"}}
	if err := service.adjustOrderInventory(context.Background(), nil, items, false); err != nil {
		t.Fatalf("reserve inventory: %v", err)
	}
	product := repository.products["product-1"]
	if product.Data["stock"] != float64(7) || product.Data["reserved_stock"] != float64(3) {
		t.Fatalf("unexpected reserved inventory: %#v", product.Data)
	}
	if err := service.adjustOrderInventory(context.Background(), items, nil, false); err != nil {
		t.Fatalf("release inventory: %v", err)
	}
	product = repository.products["product-1"]
	if product.Data["stock"] != float64(10) || product.Data["reserved_stock"] != float64(0) {
		t.Fatalf("unexpected released inventory: %#v", product.Data)
	}
	if err := service.adjustOrderInventory(context.Background(), nil, items, false); err != nil {
		t.Fatalf("reserve before delivery: %v", err)
	}
	if err := service.adjustOrderInventory(context.Background(), items, nil, true); err != nil {
		t.Fatalf("deliver inventory: %v", err)
	}
	product = repository.products["product-1"]
	if product.Data["stock"] != float64(7) || product.Data["reserved_stock"] != float64(0) {
		t.Fatalf("unexpected delivered inventory: %#v", product.Data)
	}
	if err := service.adjustOrderInventory(context.Background(), nil, []any{map[string]any{"product_id": "product-1", "quantity": "8"}}, false); err == nil {
		t.Fatal("expected oversell rejection")
	}
}

func TestNormalizeLineItemsRejectsInvalidValues(t *testing.T) {
	for _, item := range []map[string]any{
		{"product_id": "", "quantity": 1, "unit_price": 1},
		{"product_id": "product-1", "quantity": 0, "unit_price": 1},
		{"product_id": "product-1", "quantity": 1, "unit_price": 0},
	} {
		if _, _, err := normalizeLineItems([]any{item}); err == nil {
			t.Fatalf("expected validation error for %#v", item)
		}
	}
}

func TestExportRecordsUsesListFilters(t *testing.T) {
	owner := "owner-1"
	records := NewRecord(recordTestRepository{records: []entity.Record{
		{ID: "lead-1", Module: "leads", OwnerID: &owner, Data: map[string]any{"name": "目标客户", "status": "待跟进"}},
		{ID: "lead-2", Module: "leads", OwnerID: &owner, Data: map[string]any{"name": "其他客户", "status": "无效"}},
	}}, recordTestRepository{})
	out, err := records.Export(context.Background(), ExportRecordsInput{Actor: UserOutput{Role: entity.RoleAdmin}, Module: "leads", Query: "目标", Scope: "all", Status: "待跟进"})
	if err != nil {
		t.Fatalf("export records: %v", err)
	}
	if len(out.Items) != 1 || out.Items[0].ID != "lead-1" || out.Module.Key != "leads" {
		t.Fatalf("unexpected export output: %#v", out)
	}
}

func TestReportGroupsAndSumsVisibleRecords(t *testing.T) {
	repository := recordTestRepository{records: []entity.Record{{ID: "first", Module: "opportunities", Data: map[string]any{"name": "甲", "stage": "需求确认", "amount": "100.00"}}, {ID: "second", Module: "opportunities", Data: map[string]any{"name": "乙", "stage": "需求确认", "amount": "50.00"}}, {ID: "third", Module: "opportunities", Data: map[string]any{"name": "丙", "stage": "赢单", "amount": "200.00"}}}}
	service := NewRecord(repository, repository)
	output, err := service.Report(context.Background(), ReportInput{Actor: UserOutput{Role: entity.RoleAdmin}, Module: "opportunities", GroupBy: "stage", Metric: "sum", Field: "amount"})
	if err != nil || len(output.Rows) != 2 || output.Rows[0].Group != "赢单" || output.Rows[0].Value != "200.00" || output.Rows[1].Value != "150.00" {
		t.Fatalf("unexpected report: %#v, %v", output, err)
	}
	if _, err := service.Report(context.Background(), ReportInput{Actor: UserOutput{Role: entity.RoleAdmin}, Module: "opportunities", GroupBy: "unknown", Metric: "count"}); err != ErrValidation {
		t.Fatalf("invalid report accepted: %v", err)
	}
}

func TestFollowupCommentRequiresMatchingRecord(t *testing.T) {
	actor := UserOutput{ID: "user-1", Role: entity.RoleAdmin}
	repository := &followupCommentTestRepository{record: entity.Record{ID: "customer-1", Module: "customers", OwnerID: &actor.ID}, followup: entity.Followup{ID: "followup-1", RecordID: "customer-1"}, user: entity.User{ID: actor.ID, Name: "演示用户", Active: true}}
	service := NewRecord(repository, repository)
	created, err := service.CreateFollowupComment(context.Background(), FollowupCommentInput{Actor: actor, Module: "customers", RecordID: "customer-1", FollowupID: "followup-1", Content: "已补充会议纪要"})
	if err != nil || created.Content != "已补充会议纪要" || len(repository.comments) != 1 || created.CreatedAt.After(time.Now().UTC()) {
		t.Fatalf("unexpected comment: %#v, comments=%#v, err=%v", created, repository.comments, err)
	}
	listed, err := service.ListFollowupComments(context.Background(), FollowupCommentInput{Actor: actor, Module: "customers", RecordID: "customer-1", FollowupID: "followup-1"})
	if err != nil || len(listed.Items) != 1 || listed.Items[0].UserName != "演示用户" {
		t.Fatalf("unexpected comments: %#v, err=%v", listed, err)
	}
	_, err = service.CreateFollowupComment(context.Background(), FollowupCommentInput{Actor: actor, Module: "customers", RecordID: "customer-2", FollowupID: "followup-1", Content: "不应写入"})
	if err == nil || len(repository.comments) != 1 {
		t.Fatalf("expected mismatched record rejection, comments=%#v, err=%v", repository.comments, err)
	}
	updated, err := service.UpdateFollowupComment(context.Background(), UpdateFollowupCommentInput{FollowupCommentInput: FollowupCommentInput{Actor: actor, Module: "customers", RecordID: "customer-1", FollowupID: "followup-1", Content: "已更新会议纪要"}, CommentID: created.ID})
	if err != nil || updated.Content != "已更新会议纪要" || !updated.UpdatedAt.After(updated.CreatedAt) {
		t.Fatalf("unexpected updated comment: %#v, err=%v", updated, err)
	}
	if err := service.DeleteFollowupComment(context.Background(), DeleteFollowupCommentInput{Actor: actor, Module: "customers", RecordID: "customer-1", FollowupID: "followup-1", ID: created.ID}); err != nil || len(repository.comments) != 0 {
		t.Fatalf("unexpected deleted comments: %#v, err=%v", repository.comments, err)
	}
}

func TestCustomerOwnershipHistoryTracksClaimAndRelease(t *testing.T) {
	actor := UserOutput{ID: "user-1", Role: entity.RoleSales}
	repository := &ownershipHistoryTestRepository{record: entity.Record{ID: "customer-1", Module: "customers", Data: map[string]any{"name": "星云科技"}, Version: 1}, user: entity.User{ID: actor.ID, Name: "销售甲", Active: true}}
	service := NewRecord(repository, repository, repository)
	if _, err := service.Action(context.Background(), ActionInput{Actor: actor, Module: "customers", ID: "customer-1", Action: "claim", Version: 1}); err != nil {
		t.Fatalf("claim customer: %v", err)
	}
	if _, err := service.Action(context.Background(), ActionInput{Actor: actor, Module: "customers", ID: "customer-1", Action: "release", Version: 2}); err != nil {
		t.Fatalf("release customer: %v", err)
	}
	if len(repository.histories) != 2 || repository.histories[0].PreviousOwnerID != "" || repository.histories[0].OwnerID != actor.ID || repository.histories[1].PreviousOwnerID != actor.ID || repository.histories[1].OwnerID != "" {
		t.Fatalf("unexpected histories: %#v", repository.histories)
	}
	output, err := service.ListOwnershipHistory(context.Background(), ListOwnershipHistoryInput{Actor: actor, Module: "customers", ID: "customer-1"})
	if err != nil || len(output.Items) != 2 || output.Items[0].OwnerName != "销售甲" || output.Items[1].PreviousOwnerName != "销售甲" {
		t.Fatalf("unexpected output: %#v, err=%v", output, err)
	}
}

func TestPoolFreezeBlocksClaimUntilUnfrozen(t *testing.T) {
	actor := UserOutput{ID: "user-1", Role: entity.RoleSales, RoleID: "role-1", OrganizationID: "org-1", PermissionIDs: []string{"CUSTOMER_MANAGEMENT_POOL:READ", "CUSTOMER_MANAGEMENT_POOL:FREEZE", "CUSTOMER_MANAGEMENT_POOL:PICK"}}
	repository := &specialActionRepository{record: entity.Record{ID: "customer-1", Module: "customers", OrganizationID: "org-1", Data: map[string]any{"name": "星云科技"}, Version: 1}}
	service := NewRecord(repository, repository)
	if _, err := service.Action(context.Background(), ActionInput{Actor: actor, Module: "customers", ID: "customer-1", Action: "freeze", Version: 1, Reason: "暂不分配", FreezeDays: 7}); err != nil {
		t.Fatalf("freeze pool record: %v", err)
	}
	if _, err := service.Action(context.Background(), ActionInput{Actor: actor, Module: "customers", ID: "customer-1", Action: "claim", Version: 2}); err == nil {
		t.Fatal("frozen pool record was claimed")
	}
	if _, err := service.Action(context.Background(), ActionInput{Actor: actor, Module: "customers", ID: "customer-1", Action: "unfreeze", Version: 2}); err != nil {
		t.Fatalf("unfreeze pool record: %v", err)
	}
	if _, err := service.Action(context.Background(), ActionInput{Actor: actor, Module: "customers", ID: "customer-1", Action: "claim", Version: 3}); err != nil {
		t.Fatalf("claim unfrozen pool record: %v", err)
	}
}

func TestQuotationVoidRequiresPermissionAndRejectsLinkedContract(t *testing.T) {
	actor := UserOutput{ID: "user-1", Role: entity.RoleSales, RoleID: "role-1", OrganizationID: "org-1", PermissionIDs: []string{"OPPORTUNITY_QUOTATION:READ", "OPPORTUNITY_QUOTATION:VOIDED"}}
	record := entity.Record{ID: "quote-1", Module: "quotes", OrganizationID: "org-1", Data: map[string]any{"name": "报价一", "approval_status": "approved"}, Version: 1}
	repository := &specialActionRepository{record: record, contracts: []entity.Record{{ID: "contract-1", Module: "contracts", Data: map[string]any{"quote_id": "quote-1"}}}}
	service := NewRecord(repository, repository)
	if _, err := service.Action(context.Background(), ActionInput{Actor: actor, Module: "quotes", ID: "quote-1", Action: "void", Version: 1}); err == nil {
		t.Fatal("linked quotation was voided")
	}
	repository.contracts = nil
	if _, err := service.Action(context.Background(), ActionInput{Actor: actor, Module: "quotes", ID: "quote-1", Action: "void", Version: 1}); err != nil || repository.record.Data["voided"] != true {
		t.Fatalf("void quotation failed: record=%#v err=%v", repository.record, err)
	}
	actor.PermissionIDs = []string{"OPPORTUNITY_QUOTATION:READ"}
	repository.record = record
	if _, err := service.Action(context.Background(), ActionInput{Actor: actor, Module: "quotes", ID: "quote-1", Action: "void", Version: 1}); err != ErrForbidden {
		t.Fatalf("expected void permission rejection, got %v", err)
	}
}

func TestPoolDeleteUsesPoolPermission(t *testing.T) {
	actor := UserOutput{ID: "user-1", Role: entity.RoleSales, RoleID: "role-1", OrganizationID: "org-1", PermissionIDs: []string{"CUSTOMER_MANAGEMENT_POOL:DELETE"}}
	repository := &specialActionRepository{record: entity.Record{ID: "customer-1", Module: "customers", OrganizationID: "org-1", Data: map[string]any{"name": "公海客户"}, Version: 1}}
	service := NewRecord(repository, repository)
	if err := service.Delete(context.Background(), RecordInput{Actor: actor, Module: "customers", ID: "customer-1", Version: 1}); err != nil || !repository.deleted {
		t.Fatalf("pool delete failed: deleted=%v err=%v", repository.deleted, err)
	}
	repository.deleted = false
	owner := "owner-1"
	repository.record.OwnerID = &owner
	if err := service.Delete(context.Background(), RecordInput{Actor: actor, Module: "customers", ID: "customer-1", Version: 1}); err != ErrForbidden || repository.deleted {
		t.Fatalf("pool permission deleted owned customer: deleted=%v err=%v", repository.deleted, err)
	}
}

func TestPoolUpdateUsesPoolPermissionAndScope(t *testing.T) {
	actor := UserOutput{ID: "user-1", Role: entity.RoleSales, RoleID: "role-1", OrganizationID: "org-1", PermissionIDs: []string{"CUSTOMER_MANAGEMENT_POOL:UPDATE"}}
	repository := &specialActionRepository{record: entity.Record{ID: "customer-1", Module: "customers", OrganizationID: "org-1", Data: map[string]any{"name": "公海客户", "industry": "互联网", "source": "官网咨询", "level": "B · 潜力", "status": "潜在客户", "phone": "", "website": "", "address": "", "notes": ""}, Version: 1}}
	service := NewRecord(repository, repository)
	updated := map[string]any{"name": "更新后的公海客户", "industry": "互联网", "source": "官网咨询", "level": "B · 潜力", "status": "潜在客户", "phone": "", "website": "", "address": "", "notes": ""}
	if _, err := service.Update(context.Background(), RecordInput{Actor: actor, Module: "customers", ID: "customer-1", Version: 1, Data: updated}); err != nil || repository.record.Data["name"] != "更新后的公海客户" {
		t.Fatalf("pool update failed: record=%#v err=%v", repository.record, err)
	}
	owner := "owner-1"
	repository.record.OwnerID = &owner
	if _, err := service.Update(context.Background(), RecordInput{Actor: actor, Module: "customers", ID: "customer-1", Version: 2, Data: updated}); err != ErrForbidden {
		t.Fatalf("pool permission updated owned customer: %v", err)
	}
}

func TestCustomerMergeMovesRelationsAndDeletesSources(t *testing.T) {
	owner := "user-1"
	actor := UserOutput{ID: owner, Role: entity.RoleSales, RoleID: "role-1", OrganizationID: "org-1", PermissionIDs: []string{"CUSTOMER_MANAGEMENT:READ", "CUSTOMER_MANAGEMENT:MERGE"}, DataScope: "self"}
	repository := &mergeCustomerRepository{records: map[string][]entity.Record{
		"customers": {
			{ID: "target", Module: "customers", OrganizationID: "org-1", OwnerID: &owner, Data: map[string]any{"name": "目标客户"}, Version: 1},
			{ID: "source", Module: "customers", OrganizationID: "org-1", OwnerID: &owner, Data: map[string]any{"name": "源客户"}, Version: 2},
		},
		"contacts": {{ID: "contact-1", Module: "contacts", OrganizationID: "org-1", OwnerID: &owner, Data: map[string]any{"name": "联系人", "customer_id": "source"}, Version: 1}},
	}, movedActivities: map[string]string{}}
	service := NewRecord(repository, repository)
	if _, err := service.Action(context.Background(), ActionInput{Actor: actor, Module: "customers", ID: "target", Action: "merge", Version: 1, MergeIDs: []string{"source"}}); err != nil {
		t.Fatalf("merge customers: %v", err)
	}
	if len(repository.records["customers"]) != 1 || repository.records["contacts"][0].Data["customer_id"] != "target" || repository.movedActivities["source"] != "target" {
		t.Fatalf("unexpected merge result: records=%#v activities=%#v", repository.records, repository.movedActivities)
	}
}

func TestContractStageRequiresSpecialPermission(t *testing.T) {
	owner := "user-1"
	record := entity.Record{ID: "contract-1", Module: "contracts", OrganizationID: "org-1", OwnerID: &owner, Data: map[string]any{"name": "合同一", "stage": "待签署", "approval_status": "approved"}, Version: 1}
	repository := &specialActionRepository{record: record}
	service := NewRecord(repository, repository)
	actor := UserOutput{ID: owner, Role: entity.RoleSales, RoleID: "role-1", OrganizationID: "org-1", PermissionIDs: []string{"CONTRACT:READ"}, DataScope: "self"}
	if _, err := service.Action(context.Background(), ActionInput{Actor: actor, Module: "contracts", ID: record.ID, Action: "stage", Version: 1, TargetStage: "履行中"}); err != ErrForbidden {
		t.Fatalf("expected stage permission rejection, got %v", err)
	}
	actor.PermissionIDs = append(actor.PermissionIDs, "CONTRACT:STAGE")
	if _, err := service.Action(context.Background(), ActionInput{Actor: actor, Module: "contracts", ID: record.ID, Action: "stage", Version: 1, TargetStage: "履行中"}); err != nil || repository.record.Data["stage"] != "履行中" {
		t.Fatalf("change contract stage: record=%#v err=%v", repository.record, err)
	}
}

func TestPaymentCreateRequiresContractPaymentPermission(t *testing.T) {
	actor := UserOutput{ID: "user-1", Role: entity.RoleSales, RoleID: "role-1", OrganizationID: "org-1", PermissionIDs: []string{"CONTRACT_PAYMENT_RECORD:ADD"}}
	repository := recordTestRepository{modules: []entity.ModuleDefinition{{Key: "payments", Enabled: true}}}
	service := NewRecord(repository, repository)
	if _, err := service.Create(context.Background(), RecordInput{Actor: actor, Module: "payments", Data: map[string]any{}}); err != ErrForbidden {
		t.Fatalf("expected contract payment permission rejection, got %v", err)
	}
}

func TestOpportunityWinToLossRequiresResignPermission(t *testing.T) {
	owner := "user-1"
	actor := UserOutput{ID: owner, Role: entity.RoleSales, RoleID: "role-1", OrganizationID: "org-1", PermissionIDs: []string{"OPPORTUNITY_MANAGEMENT:UPDATE", "CUSTOMER_MANAGEMENT:READ"}, DataScope: "self"}
	repository := &specialActionRepository{record: entity.Record{ID: "opportunity-1", Module: "opportunities", OrganizationID: "org-1", OwnerID: &owner, Data: map[string]any{"name": "重点商机", "customer_id": "customer-1", "amount": "10000", "stage": "赢单", "probability": float64(100), "expected_date": "2026-12-31", "notes": ""}, Version: 1}}
	service := NewRecord(repository, repository)
	data := map[string]any{"name": "重点商机", "customer_id": "customer-1", "amount": "10000", "stage": "输单", "probability": float64(0), "expected_date": "2026-12-31", "notes": ""}
	if _, err := service.Update(context.Background(), RecordInput{Actor: actor, Module: "opportunities", ID: repository.record.ID, Version: 1, Data: data}); err != ErrForbidden {
		t.Fatalf("expected resign permission rejection, got %v", err)
	}
	actor.PermissionIDs = append(actor.PermissionIDs, "OPPORTUNITY_MANAGEMENT:RESIGN")
	if _, err := service.Update(context.Background(), RecordInput{Actor: actor, Module: "opportunities", ID: repository.record.ID, Version: 1, Data: data}); err != nil || repository.record.Data["stage"] != "输单" {
		t.Fatalf("resign opportunity: record=%#v err=%v", repository.record, err)
	}
}

func TestAddFollowupRefreshesActivityAtomically(t *testing.T) {
	actor := UserOutput{ID: "user-1", Role: entity.RoleSales}
	before := time.Now().UTC().Add(-time.Hour)
	repository := &followupActivityRepository{record: entity.Record{ID: "customer-1", Module: "customers", OwnerID: &actor.ID, Version: 1, UpdatedAt: before}}
	service := NewRecord(repository, repository)
	if err := service.AddFollowup(context.Background(), FollowupInput{Actor: actor, Module: "customers", RecordID: "customer-1", Content: "已完成需求沟通", Method: "电话"}); err != nil {
		t.Fatalf("add followup: %v", err)
	}
	if len(repository.followups) != 1 || !repository.record.UpdatedAt.After(before) {
		t.Fatalf("expected followup and refreshed activity: %#v, %#v", repository.followups, repository.record)
	}
	repository.failUpdate = true
	if err := service.AddFollowup(context.Background(), FollowupInput{Actor: actor, Module: "customers", RecordID: "customer-1", Content: "不应保留", Method: "电话"}); err == nil || len(repository.followups) != 1 {
		t.Fatalf("expected rollback after activity update failure: followups=%#v, err=%v", repository.followups, err)
	}
}
