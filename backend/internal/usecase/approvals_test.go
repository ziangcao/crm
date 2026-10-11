package usecase

import (
	"context"
	"testing"

	"cordialcrm/backend/internal/entity"
)

type approvalTestRepository struct {
	Repository
	approval  entity.Approval
	record    entity.Record
	flow      entity.ApprovalFlowVersion
	decisions []entity.ApprovalDecision
	failApply bool
}

func (r *approvalTestRepository) FindApproval(_ context.Context, _ string) (entity.Approval, error) {
	return r.approval, nil
}

func (r *approvalTestRepository) FindRecord(_ context.Context, _, _ string) (entity.Record, error) {
	item := r.record
	item.Data = make(map[string]any, len(r.record.Data))
	for key, value := range r.record.Data {
		item.Data[key] = value
	}
	return item, nil
}

func (r *approvalTestRepository) ApplyApprovalDecision(_ context.Context, _ entity.Approval, updated entity.Approval, record *entity.Record, decision entity.ApprovalDecision) (bool, error) {
	if r.failApply {
		return false, nil
	}
	if record != nil {
		r.record = *record
	}
	r.approval = updated
	r.decisions = append(r.decisions, decision)
	return true, nil
}

func (r *approvalTestRepository) UpdateRecord(_ context.Context, record entity.Record, _ int) error {
	r.record = record
	return nil
}

func (r *approvalTestRepository) UpdateApproval(_ context.Context, approval entity.Approval) error {
	r.approval = approval
	return nil
}

func (r *approvalTestRepository) FindApprovalFlowVersion(_ context.Context, _ string, _ int) (entity.ApprovalFlowVersion, error) {
	return r.flow, nil
}

func (r *approvalTestRepository) CreateAudit(_ context.Context, _ entity.Audit) error { return nil }

func (r *approvalTestRepository) CreateApprovalDecision(_ context.Context, decision entity.ApprovalDecision) error {
	r.decisions = append(r.decisions, decision)
	return nil
}

func (r *approvalTestRepository) ListApprovalDecisions(_ context.Context, _ string) ([]entity.ApprovalDecision, error) {
	return r.decisions, nil
}

func (r *approvalTestRepository) FindUserByID(_ context.Context, id string) (entity.User, error) {
	if id == "manager-2" {
		return entity.User{ID: id, Name: "经理二", Role: entity.RoleManager, Active: true}, nil
	}
	return entity.User{ID: id, Name: "经理一", Role: entity.RoleManager, Active: true}, nil
}

func TestWithdrawApprovalRestoresDraft(t *testing.T) {
	repository := &approvalTestRepository{
		approval: entity.Approval{ID: "approval-1", RecordID: "quote-1", RequestedBy: "user-1", Status: entity.ApprovalPending},
		record:   entity.Record{ID: "quote-1", Module: "quotes", Version: 2, Data: map[string]any{"name": "报价单", "approval_status": "pending"}},
	}
	service := NewApproval(repository, nil)
	out, err := service.Withdraw(context.Background(), WithdrawApprovalInput{Actor: UserOutput{ID: "user-1"}, ID: "approval-1"})
	if err != nil {
		t.Fatalf("withdraw approval: %v", err)
	}
	if out.Status != string(entity.ApprovalWithdrawn) || repository.approval.Status != entity.ApprovalWithdrawn || repository.record.Data["approval_status"] != string(entity.ApprovalDraft) {
		t.Fatalf("unexpected withdraw result: %#v, %#v, %#v", out, repository.approval, repository.record.Data)
	}
}

func TestWithdrawApprovalRejectsNonRequester(t *testing.T) {
	repository := &approvalTestRepository{approval: entity.Approval{ID: "approval-1", RequestedBy: "user-1", Status: entity.ApprovalPending}, record: entity.Record{Data: map[string]any{}}}
	service := NewApproval(repository, nil)
	if _, err := service.Withdraw(context.Background(), WithdrawApprovalInput{Actor: UserOutput{ID: "user-2"}, ID: "approval-1"}); err == nil {
		t.Fatal("expected non-requester withdrawal to fail")
	}
}

func TestDecideApprovalAdvancesConfiguredFlow(t *testing.T) {
	repository := &approvalTestRepository{
		approval: entity.Approval{ID: "approval-1", RecordID: "quote-1", RequestedBy: "user-1", Status: entity.ApprovalPending, FlowID: "flow-1", FlowVersion: 1, CurrentStep: 1},
		record:   entity.Record{ID: "quote-1", Module: "quotes", Version: 2, Data: map[string]any{"name": "报价单", "approval_status": "pending"}},
		flow:     entity.ApprovalFlowVersion{FlowID: "flow-1", Version: 1, Steps: []entity.ApprovalFlowStep{{Order: 1, ApproverRoles: []entity.Role{entity.RoleManager}}, {Order: 2, ApproverRoles: []entity.Role{entity.RoleAdmin}}}},
	}
	service := NewApproval(repository, nil)
	if err := service.Decide(context.Background(), DecideApprovalInput{Actor: UserOutput{ID: "manager-1", Role: entity.RoleManager}, ID: "approval-1", Decision: "approved"}); err != nil {
		t.Fatalf("approve first step: %v", err)
	}
	if repository.approval.CurrentStep != 2 || repository.approval.Status != entity.ApprovalPending || repository.record.Data["approval_status"] != "pending" {
		t.Fatalf("first step should keep approval pending: %#v %#v", repository.approval, repository.record)
	}
	if err := service.Decide(context.Background(), DecideApprovalInput{Actor: UserOutput{ID: "admin-1", Role: entity.RoleAdmin}, ID: "approval-1", Decision: "approved"}); err != nil {
		t.Fatalf("approve final step: %v", err)
	}
	if repository.approval.Status != entity.ApprovalApproved || repository.record.Data["approval_status"] != "approved" {
		t.Fatalf("final step should approve record: %#v %#v", repository.approval, repository.record)
	}
	if len(repository.decisions) != 2 || repository.decisions[0].Step != 1 || repository.decisions[1].Step != 2 {
		t.Fatalf("expected decisions for both approval steps: %#v", repository.decisions)
	}
}

func TestDecideApprovalAllowsConfiguredSalesApprover(t *testing.T) {
	owner := "sales-1"
	repository := &approvalTestRepository{
		approval: entity.Approval{ID: "approval-1", RecordID: "quote-1", RequestedBy: "user-1", Status: entity.ApprovalPending, FlowID: "flow-1", FlowVersion: 1, CurrentStep: 1},
		record:   entity.Record{ID: "quote-1", Module: "quotes", OwnerID: &owner, Version: 1, Data: map[string]any{"name": "报价单", "approval_status": "pending"}},
		flow:     entity.ApprovalFlowVersion{FlowID: "flow-1", Version: 1, Steps: []entity.ApprovalFlowStep{{Order: 1, ApproverRoles: []entity.Role{entity.RoleSales}}}},
	}
	if err := NewApproval(repository, nil).Decide(context.Background(), DecideApprovalInput{Actor: UserOutput{ID: "sales-1", Role: entity.RoleSales}, ID: "approval-1", Decision: "approved"}); err != nil {
		t.Fatalf("approve configured sales step: %v", err)
	}
	if repository.approval.Status != entity.ApprovalApproved {
		t.Fatalf("sales approval should finish configured flow: %#v", repository.approval)
	}
}

func TestConfiguredSalesApproverCannotDecideOtherOwnersRecord(t *testing.T) {
	owner := "sales-1"
	repository := &approvalTestRepository{approval: entity.Approval{ID: "approval-1", RecordID: "quote-1", Status: entity.ApprovalPending, FlowID: "flow-1", FlowVersion: 1, CurrentStep: 1}, record: entity.Record{ID: "quote-1", Module: "quotes", OwnerID: &owner, Data: map[string]any{"approval_status": "pending"}}, flow: entity.ApprovalFlowVersion{Steps: []entity.ApprovalFlowStep{{Order: 1, ApproverRoles: []entity.Role{entity.RoleSales}}}}}
	if err := NewApproval(repository, nil).Decide(context.Background(), DecideApprovalInput{Actor: UserOutput{ID: "sales-2", Role: entity.RoleSales}, ID: "approval-1", Decision: "approved"}); err != ErrForbidden {
		t.Fatalf("cross-owner decision: %v", err)
	}
	if len(repository.decisions) != 0 {
		t.Fatal("unauthorized decision was persisted")
	}
}

func TestAssignedApproverMustMatchCurrentMember(t *testing.T) {
	repository := &approvalTestRepository{approval: entity.Approval{ID: "approval-1", RecordID: "quote-1", Status: entity.ApprovalPending, FlowID: "flow-1", FlowVersion: 1, CurrentStep: 1}, record: entity.Record{ID: "quote-1", Module: "quotes", Data: map[string]any{"approval_status": "pending"}}, flow: entity.ApprovalFlowVersion{Steps: []entity.ApprovalFlowStep{{Order: 1, ApproverIDs: []string{"manager-1"}}}}}
	service := NewApproval(repository, nil)
	if err := service.Decide(context.Background(), DecideApprovalInput{Actor: UserOutput{ID: "manager-2", Role: entity.RoleManager}, ID: "approval-1", Decision: "approved"}); err != ErrForbidden {
		t.Fatalf("unassigned manager approved: %v", err)
	}
	if err := service.Decide(context.Background(), DecideApprovalInput{Actor: UserOutput{ID: "manager-1", Role: entity.RoleManager}, ID: "approval-1", Decision: "approved"}); err != nil {
		t.Fatalf("assigned manager rejected: %v", err)
	}
}

func TestApprovalSkipsAmountGatedSteps(t *testing.T) {
	steps := []entity.ApprovalFlowStep{{Order: 1, ApproverRoles: []entity.Role{entity.RoleManager}, MinAmount: "100000"}, {Order: 2, ApproverRoles: []entity.Role{entity.RoleAdmin}, MinAmount: "200000"}, {Order: 3, ApproverRoles: []entity.Role{entity.RoleManager}}}
	record := entity.Record{ID: "quote-1", Module: "quotes", Version: 2, Data: map[string]any{"name": "报价单", "amount": "150000", "approval_status": "pending"}}
	if step := nextApplicableApprovalStep(steps, entity.Record{Data: map[string]any{"amount": "50000"}}, 0); step != 3 {
		t.Fatalf("low amount should start at fallback step, got %d", step)
	}
	if step := nextApplicableApprovalStep(steps, record, 0); step != 1 {
		t.Fatalf("mid amount should start at step one, got %d", step)
	}
	repository := &approvalTestRepository{approval: entity.Approval{ID: "approval-1", RecordID: "quote-1", Status: entity.ApprovalPending, FlowID: "flow-1", FlowVersion: 1, CurrentStep: 1}, record: record, flow: entity.ApprovalFlowVersion{FlowID: "flow-1", Version: 1, Steps: steps}}
	service := NewApproval(repository, nil)
	if err := service.Decide(context.Background(), DecideApprovalInput{Actor: UserOutput{ID: "manager", Role: entity.RoleManager}, ID: "approval-1", Decision: "approved"}); err != nil {
		t.Fatal(err)
	}
	if repository.approval.CurrentStep != 3 || repository.approval.Status != entity.ApprovalPending {
		t.Fatalf("expected step two skipped: %#v", repository.approval)
	}
	if err := service.Decide(context.Background(), DecideApprovalInput{Actor: UserOutput{ID: "manager", Role: entity.RoleManager}, ID: "approval-1", Decision: "approved"}); err != nil {
		t.Fatal(err)
	}
	if repository.approval.Status != entity.ApprovalApproved || len(repository.decisions) != 2 {
		t.Fatalf("expected completion at fallback step: %#v", repository)
	}
}

func TestApprovalSkipsUnmatchedFieldCondition(t *testing.T) {
	steps := []entity.ApprovalFlowStep{{Order: 1, ApproverRoles: []entity.Role{entity.RoleManager}, Conditions: []entity.ApprovalCondition{{Field: "type", Operator: "eq", Value: "特殊"}}}, {Order: 2, ApproverRoles: []entity.Role{entity.RoleAdmin}}}
	if step := nextApplicableApprovalStep(steps, entity.Record{Data: map[string]any{"type": "普通"}}, 0); step != 2 {
		t.Fatalf("expected unmatched step skipped, got %d", step)
	}
	if step := nextApplicableApprovalStep(steps, entity.Record{Data: map[string]any{"type": "特殊"}}, 0); step != 1 {
		t.Fatalf("expected matching step, got %d", step)
	}
}

func TestFailedApprovalDecisionKeepsRecordAndHistory(t *testing.T) {
	repository := &approvalTestRepository{approval: entity.Approval{ID: "approval-1", RecordID: "quote-1", Status: entity.ApprovalPending}, record: entity.Record{ID: "quote-1", Module: "quotes", Version: 1, Data: map[string]any{"approval_status": "pending"}}, failApply: true}
	err := NewApproval(repository, nil).Decide(context.Background(), DecideApprovalInput{Actor: UserOutput{ID: "manager", Role: entity.RoleManager}, ID: "approval-1", Decision: "approved"})
	if err != ErrConflict || repository.approval.Status != entity.ApprovalPending || repository.record.Data["approval_status"] != "pending" || len(repository.decisions) != 0 {
		t.Fatalf("failed decision changed stored state: %#v, %#v, %v", repository.approval, repository.record, err)
	}
}

func TestTransferApprovalAssignsCurrentStepToRecipient(t *testing.T) {
	repository := &approvalTestRepository{approval: entity.Approval{ID: "approval-1", RecordID: "quote-1", Status: entity.ApprovalPending, FlowID: "flow-1", FlowVersion: 1, CurrentStep: 1}, record: entity.Record{ID: "quote-1", Module: "quotes", Data: map[string]any{"name": "报价单", "approval_status": "pending"}}, flow: entity.ApprovalFlowVersion{Steps: []entity.ApprovalFlowStep{{Order: 1, ApproverRoles: []entity.Role{entity.RoleManager}}}}}
	service := NewApproval(repository, nil)
	if err := service.Transfer(context.Background(), TransferApprovalInput{Actor: UserOutput{ID: "manager-1", Name: "经理一", Role: entity.RoleManager}, ID: "approval-1", TargetUserID: "manager-2", Comment: "请代为处理"}); err != nil {
		t.Fatalf("transfer approval: %v", err)
	}
	if repository.approval.DelegatedToID == nil || *repository.approval.DelegatedToID != "manager-2" || len(repository.decisions) != 1 || repository.decisions[0].Decision != "transferred" {
		t.Fatalf("unexpected transfer state: %#v, %#v", repository.approval, repository.decisions)
	}
	if err := service.Decide(context.Background(), DecideApprovalInput{Actor: UserOutput{ID: "manager-1", Role: entity.RoleManager}, ID: "approval-1", Decision: "approved"}); err != ErrForbidden {
		t.Fatalf("original approver should lose access: %v", err)
	}
	if err := service.Decide(context.Background(), DecideApprovalInput{Actor: UserOutput{ID: "manager-2", Role: entity.RoleManager}, ID: "approval-1", Decision: "approved"}); err != nil {
		t.Fatalf("recipient should decide: %v", err)
	}
}

func TestAddSignerReturnsApprovalToOriginalApprover(t *testing.T) {
	repository := &approvalTestRepository{approval: entity.Approval{ID: "approval-1", RecordID: "quote-1", Status: entity.ApprovalPending, FlowID: "flow-1", FlowVersion: 1, CurrentStep: 1}, record: entity.Record{ID: "quote-1", Module: "quotes", Data: map[string]any{"name": "报价单", "approval_status": "pending"}}, flow: entity.ApprovalFlowVersion{Steps: []entity.ApprovalFlowStep{{Order: 1, ApproverRoles: []entity.Role{entity.RoleManager}}}}}
	service := NewApproval(repository, nil)
	if err := service.AddSigner(context.Background(), AddSignerInput{Actor: UserOutput{ID: "manager-1", Name: "经理一", Role: entity.RoleManager}, ID: "approval-1", TargetUserID: "manager-2", Comment: "请补充审核"}); err != nil {
		t.Fatalf("add signer: %v", err)
	}
	if repository.approval.AdditionalApproverID == nil || *repository.approval.AdditionalApproverID != "manager-2" || repository.decisions[0].Decision != "added" {
		t.Fatalf("unexpected add signer state: %#v, %#v", repository.approval, repository.decisions)
	}
	if err := service.Decide(context.Background(), DecideApprovalInput{Actor: UserOutput{ID: "manager-1", Role: entity.RoleManager}, ID: "approval-1", Decision: "approved"}); err != ErrForbidden {
		t.Fatalf("original approver should wait for signer: %v", err)
	}
	if err := service.Decide(context.Background(), DecideApprovalInput{Actor: UserOutput{ID: "manager-2", Role: entity.RoleManager}, ID: "approval-1", Decision: "approved"}); err != nil {
		t.Fatalf("additional signer should approve: %v", err)
	}
	if repository.approval.AdditionalApproverID != nil || repository.approval.Status != entity.ApprovalPending {
		t.Fatalf("approval should return to original approver: %#v", repository.approval)
	}
	if err := service.Decide(context.Background(), DecideApprovalInput{Actor: UserOutput{ID: "manager-1", Role: entity.RoleManager}, ID: "approval-1", Decision: "approved"}); err != nil {
		t.Fatalf("original approver should continue: %v", err)
	}
}
