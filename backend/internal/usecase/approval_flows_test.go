package usecase

import (
	"context"
	"testing"

	"cordialcrm/backend/internal/entity"
)

type approvalFlowTestRepository struct {
	flows    []entity.ApprovalFlow
	versions []entity.ApprovalFlowVersion
	users    []entity.User
}

func (r *approvalFlowTestRepository) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}
func (r *approvalFlowTestRepository) ListApprovalFlows(_ context.Context, module string) ([]entity.ApprovalFlow, error) {
	items := make([]entity.ApprovalFlow, 0, len(r.flows))
	for _, flow := range r.flows {
		if module == "" || flow.Module == module {
			items = append(items, flow)
		}
	}
	return items, nil
}
func (r *approvalFlowTestRepository) SaveApprovalFlow(_ context.Context, updated entity.ApprovalFlow) error {
	for index, flow := range r.flows {
		if flow.ID == updated.ID {
			r.flows[index] = updated
			return nil
		}
	}
	r.flows = append(r.flows, updated)
	return nil
}
func (r *approvalFlowTestRepository) SaveApprovalFlowVersion(_ context.Context, version entity.ApprovalFlowVersion) error {
	r.versions = append(r.versions, version)
	return nil
}
func (r *approvalFlowTestRepository) FindCurrentApprovalFlowVersion(_ context.Context, flowID string) (entity.ApprovalFlowVersion, error) {
	for index := len(r.versions) - 1; index >= 0; index-- {
		if r.versions[index].FlowID == flowID {
			return r.versions[index], nil
		}
	}
	return entity.ApprovalFlowVersion{}, ErrNotFound
}
func (r *approvalFlowTestRepository) FindApprovalFlowVersion(_ context.Context, flowID string, version int) (entity.ApprovalFlowVersion, error) {
	for _, item := range r.versions {
		if item.FlowID == flowID && item.Version == version {
			return item, nil
		}
	}
	return entity.ApprovalFlowVersion{}, ErrNotFound
}
func (r *approvalFlowTestRepository) ListUsers(context.Context) ([]entity.User, error) {
	return r.users, nil
}
func (r *approvalFlowTestRepository) ListCustomFields(_ context.Context, _ string) ([]entity.CustomField, error) {
	return nil, nil
}
func (r *approvalFlowTestRepository) ListStandardFieldSettings(_ context.Context, _ string) ([]entity.StandardFieldSetting, error) {
	return nil, nil
}

func TestSaveApprovalFlowCreatesNewVersionAndDisablesPeer(t *testing.T) {
	repository := &approvalFlowTestRepository{flows: []entity.ApprovalFlow{{ID: "flow-old", Module: "quotes", Name: "旧流程", Enabled: true, Version: 1}}}
	service := NewApprovalFlow(repository)
	actor := UserOutput{ID: "admin", Role: entity.RoleAdmin}
	created, err := service.Save(context.Background(), SaveApprovalFlowInput{Actor: actor, Module: "quotes", Name: "新流程", Enabled: true, Steps: []entity.ApprovalFlowStep{{Order: 1, ApproverRoles: []entity.Role{entity.RoleManager}}}})
	if err != nil {
		t.Fatalf("save approval flow: %v", err)
	}
	if created.Flow.Version != 1 || !created.Flow.Enabled || len(repository.versions) != 1 || repository.flows[0].Enabled {
		t.Fatalf("unexpected created flow: %#v, %#v", created, repository)
	}
	updated, err := service.Save(context.Background(), SaveApprovalFlowInput{Actor: actor, ID: created.Flow.ID, Module: "quotes", Name: "更新流程", Enabled: true, Steps: []entity.ApprovalFlowStep{{Order: 1, ApproverRoles: []entity.Role{entity.RoleAdmin}}, {Order: 2, ApproverRoles: []entity.Role{entity.RoleManager}}}})
	if err != nil {
		t.Fatalf("update approval flow: %v", err)
	}
	if updated.Flow.Version != 2 || len(repository.versions) != 2 || len(updated.Flow.Steps) != 2 {
		t.Fatalf("expected version two with two steps: %#v", updated)
	}
}

func TestSaveApprovalFlowRejectsInvalidStepsAndNonAdmin(t *testing.T) {
	service := NewApprovalFlow(&approvalFlowTestRepository{})
	input := SaveApprovalFlowInput{Actor: UserOutput{Role: entity.RoleAdmin}, Module: "quotes", Name: "流程"}
	if _, err := service.Save(context.Background(), input); err != ErrValidation {
		t.Fatalf("expected validation error for empty steps, got %v", err)
	}
	input.Steps = []entity.ApprovalFlowStep{{Order: 2, ApproverRoles: []entity.Role{entity.RoleManager}}}
	if _, err := service.Save(context.Background(), input); err != ErrValidation {
		t.Fatalf("expected validation error for non-contiguous orders, got %v", err)
	}
	input.Actor.Role = entity.RoleSales
	input.Steps[0].Order = 1
	if _, err := service.Save(context.Background(), input); err != ErrForbidden {
		t.Fatalf("expected forbidden error for non-admin save, got %v", err)
	}
}

func TestValidateApprovalAmountThresholds(t *testing.T) {
	steps := []entity.ApprovalFlowStep{{Order: 1, ApproverRoles: []entity.Role{entity.RoleManager}, MinAmount: "100000.00"}, {Order: 2, ApproverRoles: []entity.Role{entity.RoleAdmin}}}
	if err := validateApprovalFlowSteps(steps); err != nil {
		t.Fatalf("valid amount threshold rejected: %v", err)
	}
	for _, amount := range []string{"-1", "bad", "0"} {
		steps[0].MinAmount = amount
		if err := validateApprovalFlowSteps(steps); err != ErrValidation {
			t.Fatalf("accepted invalid threshold %q: %v", amount, err)
		}
	}
	steps[0].MinAmount = "100000"
	steps[1].MinAmount = "200000"
	if err := validateApprovalFlowSteps(steps); err != ErrValidation {
		t.Fatalf("accepted conditional final step: %v", err)
	}
}

func TestSaveApprovalFlowRejectsNonApprovableModule(t *testing.T) {
	service := NewApprovalFlow(&approvalFlowTestRepository{})
	_, err := service.Save(context.Background(), SaveApprovalFlowInput{Actor: UserOutput{Role: entity.RoleAdmin}, Module: "customers", Name: "错误流程", Steps: []entity.ApprovalFlowStep{{Order: 1, ApproverRoles: []entity.Role{entity.RoleAdmin}}}})
	if err != ErrValidation {
		t.Fatalf("non-approvable module accepted: %v", err)
	}
}

func TestSaveApprovalFlowValidatesAssignedApprovers(t *testing.T) {
	service := NewApprovalFlow(&approvalFlowTestRepository{users: []entity.User{{ID: "manager", Role: entity.RoleManager, Active: true}, {ID: "sales", Role: entity.RoleSales, Active: true}, {ID: "inactive", Role: entity.RoleAdmin}}})
	input := SaveApprovalFlowInput{Actor: UserOutput{Role: entity.RoleAdmin}, Module: "quotes", Name: "指定审批", Steps: []entity.ApprovalFlowStep{{Order: 1, ApproverIDs: []string{"manager"}}}}
	if _, err := service.Save(context.Background(), input); err != nil {
		t.Fatalf("valid assigned approver rejected: %v", err)
	}
	for _, userID := range []string{"sales", "inactive", "missing"} {
		input.ID = ""
		input.Steps[0].ApproverIDs = []string{userID}
		if _, err := service.Save(context.Background(), input); err != ErrValidation {
			t.Fatalf("invalid assigned approver %q: %v", userID, err)
		}
	}
}

func TestSaveApprovalFlowValidatesFieldConditions(t *testing.T) {
	service := NewApprovalFlow(&approvalFlowTestRepository{})
	input := SaveApprovalFlowInput{Actor: UserOutput{Role: entity.RoleAdmin}, Module: "quotes", Name: "条件审批", Steps: []entity.ApprovalFlowStep{{Order: 1, ApproverRoles: []entity.Role{entity.RoleManager}, Conditions: []entity.ApprovalCondition{{Field: "amount", Operator: "eq", Value: "100"}}}}}
	if _, err := service.Save(context.Background(), input); err != nil {
		t.Fatalf("valid field condition rejected: %v", err)
	}
	for _, condition := range []entity.ApprovalCondition{{Field: "missing", Operator: "eq", Value: "x"}, {Field: "amount", Operator: "invalid", Value: "x"}, {Field: "amount", Operator: "empty", Value: "x"}, {Field: "amount", Operator: "contains", Value: ""}} {
		input.ID = ""
		input.Steps[0].Conditions = []entity.ApprovalCondition{condition}
		if _, err := service.Save(context.Background(), input); err != ErrValidation {
			t.Fatalf("invalid condition %#v accepted: %v", condition, err)
		}
	}
}
