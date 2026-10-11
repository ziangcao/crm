package usecase

import (
	"context"
	"fmt"
	"time"

	"cordialcrm/backend/internal/entity"
)

type ApprovalUseCase struct {
	repo          Repository
	notifications *NotificationUseCase
}

func NewApproval(repo Repository, notifications *NotificationUseCase) *ApprovalUseCase {
	return &ApprovalUseCase{repo: repo, notifications: notifications}
}

type ListApprovalsInput struct {
	Actor  UserOutput
	Status string
}
type ApprovalOutput struct {
	ID                     string                   `json:"id"`
	RecordID               string                   `json:"record_id"`
	Module                 string                   `json:"module"`
	Name                   string                   `json:"name"`
	Status                 string                   `json:"status"`
	RequestedBy            string                   `json:"requested_by"`
	RequesterName          string                   `json:"requester_name"`
	ReviewerName           string                   `json:"reviewer_name"`
	Comment                string                   `json:"comment"`
	CurrentStep            int                      `json:"current_step,omitempty"`
	TotalSteps             int                      `json:"total_steps,omitempty"`
	CurrentApproverRoles   []entity.Role            `json:"current_approver_roles,omitempty"`
	CurrentApproverIDs     []string                 `json:"current_approver_ids,omitempty"`
	DelegatedToID          *string                  `json:"delegated_to_id,omitempty"`
	DelegatedToName        string                   `json:"delegated_to_name,omitempty"`
	AdditionalApproverID   *string                  `json:"additional_approver_id,omitempty"`
	AdditionalApproverName string                   `json:"additional_approver_name,omitempty"`
	Decisions              []ApprovalDecisionOutput `json:"decisions"`
	CreatedAt              time.Time                `json:"created_at"`
	ReviewedAt             *time.Time               `json:"reviewed_at,omitempty"`
}
type ApprovalDecisionOutput struct {
	ID           string    `json:"id"`
	ReviewerID   string    `json:"reviewer_id"`
	ReviewerName string    `json:"reviewer_name"`
	Decision     string    `json:"decision"`
	Comment      string    `json:"comment"`
	Step         int       `json:"step"`
	CreatedAt    time.Time `json:"created_at"`
}
type DecideApprovalInput struct {
	Actor                 UserOutput
	ID, Decision, Comment string
}
type WithdrawApprovalInput struct {
	Actor UserOutput
	ID    string
}
type WithdrawApprovalOutput struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}
type TransferApprovalInput struct {
	Actor                     UserOutput
	ID, TargetUserID, Comment string
}
type AddSignerInput struct {
	Actor                     UserOutput
	ID, TargetUserID, Comment string
}

func (u *ApprovalUseCase) List(ctx context.Context, input ListApprovalsInput) ([]ApprovalOutput, error) {
	items, err := u.repo.ListApprovals(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]ApprovalOutput, 0, len(items))
	for _, item := range items {
		if input.Status != "" && string(item.Status) != input.Status {
			continue
		}
		record, err := u.repo.FindRecord(ctx, "", item.RecordID)
		if err != nil {
			continue
		}
		if !visible(input.Actor, Modules[record.Module], record, false) {
			continue
		}
		if item.Status == entity.ApprovalPending && item.AdditionalApproverID != nil && *item.AdditionalApproverID != input.Actor.ID && (record.OwnerID == nil || *record.OwnerID != input.Actor.ID) {
			continue
		}
		if item.Status == entity.ApprovalPending && item.AdditionalApproverID == nil && item.DelegatedToID != nil && *item.DelegatedToID != input.Actor.ID && (record.OwnerID == nil || *record.OwnerID != input.Actor.ID) {
			continue
		}
		if item.Status == entity.ApprovalPending && item.FlowID != "" {
			if version, err := u.repo.FindApprovalFlowVersion(ctx, item.FlowID, item.FlowVersion); err == nil && item.CurrentStep >= 1 && item.CurrentStep <= len(version.Steps) {
				step := version.Steps[item.CurrentStep-1]
				if len(step.ApproverIDs) > 0 && !canApprove(input.Actor, step) && (record.OwnerID == nil || *record.OwnerID != input.Actor.ID) {
					continue
				}
			}
		}
		if input.Actor.Role == entity.RoleSales && (record.OwnerID == nil || *record.OwnerID != input.Actor.ID) {
			continue
		}
		output, err := approvalOutput(ctx, u.repo, item, record)
		if err != nil {
			return nil, err
		}
		result = append(result, output)
	}
	return result, nil
}
func (u *ApprovalUseCase) Decide(ctx context.Context, input DecideApprovalInput) error {
	if input.Actor.RoleID != "" {
		return ErrForbidden
	}
	item, err := u.repo.FindApproval(ctx, input.ID)
	if err != nil {
		return ErrNotFound
	}
	if item.Status != entity.ApprovalPending {
		return entity.NewBusinessError("conflict", "该审批已处理")
	}
	if item.AdditionalApproverID != nil && *item.AdditionalApproverID != input.Actor.ID {
		return ErrForbidden
	}
	if item.AdditionalApproverID == nil && item.DelegatedToID != nil && *item.DelegatedToID != input.Actor.ID {
		return ErrForbidden
	}
	if item.FlowID == "" && input.Actor.Role == entity.RoleSales {
		return entity.NewBusinessError("forbidden", "需要管理员或经理权限")
	}
	if input.Decision != "approved" && input.Decision != "rejected" {
		return ErrValidation
	}
	if input.Decision == "rejected" && input.Comment == "" {
		return entity.NewBusinessError("validation", "驳回时请填写原因")
	}
	record, err := u.repo.FindRecord(ctx, "", item.RecordID)
	if err != nil {
		return ErrNotFound
	}
	if input.Actor.Role == entity.RoleSales && (record.OwnerID == nil || *record.OwnerID != input.Actor.ID) {
		return ErrForbidden
	}
	previous := item
	decision := entity.ApprovalDecision{ID: entity.NewID(), ApprovalID: item.ID, ReviewerID: input.Actor.ID, Decision: input.Decision, Comment: input.Comment, Step: item.CurrentStep, CreatedAt: time.Now().UTC()}
	if item.FlowID != "" {
		version, err := u.repo.FindApprovalFlowVersion(ctx, item.FlowID, item.FlowVersion)
		if err != nil || item.CurrentStep < 1 || item.CurrentStep > len(version.Steps) {
			return ErrNotFound
		}
		if item.AdditionalApproverID == nil && item.DelegatedToID == nil && !canApprove(input.Actor, version.Steps[item.CurrentStep-1]) {
			return ErrForbidden
		}
		nextStep := nextApplicableApprovalStep(version.Steps, record, item.CurrentStep)
		if input.Decision == "approved" && item.AdditionalApproverID != nil {
			item.AdditionalApproverID = nil
			applied, err := u.repo.ApplyApprovalDecision(ctx, previous, item, nil, decision)
			if err != nil {
				return err
			}
			if !applied {
				return ErrConflict
			}
			return nil
		}
		if input.Decision == "approved" && nextStep != 0 {
			item.CurrentStep = nextStep
			item.DelegatedToID = nil
			item.AdditionalApproverID = nil
			applied, err := u.repo.ApplyApprovalDecision(ctx, previous, item, nil, decision)
			if err != nil {
				return err
			}
			if !applied {
				return ErrConflict
			}
			return nil
		}
	}
	record.Data["approval_status"] = input.Decision
	record.UpdatedAt = time.Now().UTC()
	item.Status = entity.ApprovalStatus(input.Decision)
	item.Comment = input.Comment
	item.ReviewedBy = &input.Actor.ID
	now := time.Now().UTC()
	item.ReviewedAt = &now
	applied, err := u.repo.ApplyApprovalDecision(ctx, previous, item, &record, decision)
	if err != nil {
		return err
	}
	if !applied {
		return ErrConflict
	}
	if u.notifications != nil {
		result := map[string]string{"approved": "已通过", "rejected": "已驳回"}[input.Decision]
		_ = u.notifications.send(ctx, notificationMessage{RecipientID: item.RequestedBy, Type: "approval", Subject: "审批" + result, Content: fmt.Sprintf("%s《%s》的审批%s。", moduleLabel(record.Module), record.Data["name"], result), ResourceID: record.ID, ResourceType: record.Module})
	}
	return nil
}

func (u *ApprovalUseCase) AddSigner(ctx context.Context, input AddSignerInput) error {
	if input.Actor.RoleID != "" || (input.Actor.Role != entity.RoleAdmin && input.Actor.Role != entity.RoleManager) || input.TargetUserID == "" || input.TargetUserID == input.Actor.ID {
		return ErrValidation
	}
	item, err := u.repo.FindApproval(ctx, input.ID)
	if err != nil {
		return ErrNotFound
	}
	if item.Status != entity.ApprovalPending || item.AdditionalApproverID != nil {
		return entity.NewBusinessError("conflict", "当前审批不能加签")
	}
	record, err := u.repo.FindRecord(ctx, "", item.RecordID)
	if err != nil {
		return ErrNotFound
	}
	if !visible(input.Actor, Modules[record.Module], record, false) {
		return ErrForbidden
	}
	if item.DelegatedToID != nil {
		if *item.DelegatedToID != input.Actor.ID {
			return ErrForbidden
		}
	} else if item.FlowID != "" {
		version, err := u.repo.FindApprovalFlowVersion(ctx, item.FlowID, item.FlowVersion)
		if err != nil || item.CurrentStep < 1 || item.CurrentStep > len(version.Steps) || !canApprove(input.Actor, version.Steps[item.CurrentStep-1]) {
			return ErrForbidden
		}
	}
	target, err := u.repo.FindUserByID(ctx, input.TargetUserID)
	if err != nil {
		return ErrNotFound
	}
	if !target.Active || (target.Role != entity.RoleAdmin && target.Role != entity.RoleManager) || target.OrganizationID != input.Actor.OrganizationID {
		return ErrForbidden
	}
	previous := item
	item.AdditionalApproverID = &target.ID
	decision := entity.ApprovalDecision{ID: entity.NewID(), ApprovalID: item.ID, ReviewerID: input.Actor.ID, Decision: "added", Comment: input.Comment, Step: item.CurrentStep, CreatedAt: time.Now().UTC()}
	applied, err := u.repo.ApplyApprovalDecision(ctx, previous, item, nil, decision)
	if err != nil {
		return err
	}
	if !applied {
		return ErrConflict
	}
	_ = u.repo.CreateAudit(ctx, entity.Audit{ID: entity.NewID(), UserID: input.Actor.ID, Action: "审批加签", Module: record.Module, RecordID: &record.ID, Detail: fmt.Sprintf("%s 加签给 %s", record.Data["name"], target.Name), CreatedAt: time.Now().UTC()})
	if u.notifications != nil {
		_ = u.notifications.send(ctx, notificationMessage{RecipientID: target.ID, Type: "approval", Subject: "待审批加签", Content: fmt.Sprintf("%s 将%s《%s》加签给你。", input.Actor.Name, moduleLabel(record.Module), record.Data["name"]), ResourceID: record.ID, ResourceType: record.Module})
	}
	return nil
}

func (u *ApprovalUseCase) Transfer(ctx context.Context, input TransferApprovalInput) error {
	if input.Actor.RoleID != "" || (input.Actor.Role != entity.RoleAdmin && input.Actor.Role != entity.RoleManager) || input.TargetUserID == "" || input.TargetUserID == input.Actor.ID {
		return ErrValidation
	}
	item, err := u.repo.FindApproval(ctx, input.ID)
	if err != nil {
		return ErrNotFound
	}
	if item.Status != entity.ApprovalPending {
		return entity.NewBusinessError("conflict", "只有审批中的记录可以转签")
	}
	record, err := u.repo.FindRecord(ctx, "", item.RecordID)
	if err != nil {
		return ErrNotFound
	}
	if !visible(input.Actor, Modules[record.Module], record, false) {
		return ErrForbidden
	}
	if item.DelegatedToID != nil {
		if *item.DelegatedToID != input.Actor.ID {
			return ErrForbidden
		}
	} else if item.FlowID != "" {
		version, err := u.repo.FindApprovalFlowVersion(ctx, item.FlowID, item.FlowVersion)
		if err != nil || item.CurrentStep < 1 || item.CurrentStep > len(version.Steps) {
			return ErrNotFound
		}
		if !canApprove(input.Actor, version.Steps[item.CurrentStep-1]) {
			return ErrForbidden
		}
	}
	target, err := u.repo.FindUserByID(ctx, input.TargetUserID)
	if err != nil {
		return ErrNotFound
	}
	if !target.Active || (target.Role != entity.RoleAdmin && target.Role != entity.RoleManager) || target.OrganizationID != input.Actor.OrganizationID {
		return ErrForbidden
	}
	previous := item
	item.DelegatedToID = &target.ID
	decision := entity.ApprovalDecision{ID: entity.NewID(), ApprovalID: item.ID, ReviewerID: input.Actor.ID, Decision: "transferred", Comment: input.Comment, Step: item.CurrentStep, CreatedAt: time.Now().UTC()}
	applied, err := u.repo.ApplyApprovalDecision(ctx, previous, item, nil, decision)
	if err != nil {
		return err
	}
	if !applied {
		return ErrConflict
	}
	_ = u.repo.CreateAudit(ctx, entity.Audit{ID: entity.NewID(), UserID: input.Actor.ID, Action: "审批转签", Module: record.Module, RecordID: &record.ID, Detail: fmt.Sprintf("%s 转签给 %s", record.Data["name"], target.Name), CreatedAt: time.Now().UTC()})
	if u.notifications != nil {
		_ = u.notifications.send(ctx, notificationMessage{RecipientID: target.ID, Type: "approval", Subject: "待审批转签", Content: fmt.Sprintf("%s 将%s《%s》转签给你。", input.Actor.Name, moduleLabel(record.Module), record.Data["name"]), ResourceID: record.ID, ResourceType: record.Module})
	}
	return nil
}

func roleCanApprove(role entity.Role, roles []entity.Role) bool {
	for _, candidate := range roles {
		if candidate == role {
			return true
		}
	}
	return false
}

func canApprove(actor UserOutput, step entity.ApprovalFlowStep) bool {
	for _, userID := range step.ApproverIDs {
		if userID == actor.ID {
			return true
		}
	}
	if len(step.ApproverIDs) > 0 {
		return false
	}
	return roleCanApprove(actor.Role, step.ApproverRoles)
}

func (u *ApprovalUseCase) Withdraw(ctx context.Context, input WithdrawApprovalInput) (WithdrawApprovalOutput, error) {
	item, err := u.repo.FindApproval(ctx, input.ID)
	if err != nil {
		return WithdrawApprovalOutput{}, ErrNotFound
	}
	if item.RequestedBy != input.Actor.ID {
		return WithdrawApprovalOutput{}, entity.NewBusinessError("forbidden", "只有提交人可以撤回审批")
	}
	if item.Status != entity.ApprovalPending {
		return WithdrawApprovalOutput{}, entity.NewBusinessError("conflict", "只有审批中的记录可以撤回")
	}
	record, err := u.repo.FindRecord(ctx, "", item.RecordID)
	if err != nil {
		return WithdrawApprovalOutput{}, ErrNotFound
	}
	if !visible(input.Actor, Modules[record.Module], record, true) {
		return WithdrawApprovalOutput{}, ErrForbidden
	}
	record.Data["approval_status"] = string(entity.ApprovalDraft)
	record.UpdatedAt = time.Now().UTC()
	previous := item
	item.Status = entity.ApprovalWithdrawn
	item.Comment = "提交人已撤回审批"
	decision := entity.ApprovalDecision{ID: entity.NewID(), ApprovalID: item.ID, ReviewerID: input.Actor.ID, Decision: string(entity.ApprovalWithdrawn), Comment: item.Comment, Step: item.CurrentStep, CreatedAt: time.Now().UTC()}
	applied, err := u.repo.ApplyApprovalDecision(ctx, previous, item, &record, decision)
	if err != nil {
		return WithdrawApprovalOutput{}, err
	}
	if !applied {
		return WithdrawApprovalOutput{}, ErrConflict
	}
	_ = u.repo.CreateAudit(ctx, entity.Audit{ID: entity.NewID(), UserID: input.Actor.ID, Action: "撤回审批", Module: record.Module, RecordID: &record.ID, Detail: fmt.Sprint(record.Data["name"]), CreatedAt: time.Now().UTC()})
	return WithdrawApprovalOutput{ID: item.ID, Status: string(item.Status)}, nil
}

func approvalOutput(ctx context.Context, repository Repository, item entity.Approval, record entity.Record) (ApprovalOutput, error) {
	requester, _ := repository.FindUserByID(ctx, item.RequestedBy)
	reviewerName := ""
	if item.ReviewedBy != nil {
		reviewer, _ := repository.FindUserByID(ctx, *item.ReviewedBy)
		reviewerName = reviewer.Name
	}
	totalSteps := 0
	var currentApproverRoles []entity.Role
	var currentApproverIDs []string
	if item.FlowID != "" {
		if version, err := repository.FindApprovalFlowVersion(ctx, item.FlowID, item.FlowVersion); err == nil {
			totalSteps = len(version.Steps)
			if item.CurrentStep >= 1 && item.CurrentStep <= totalSteps {
				currentApproverRoles = version.Steps[item.CurrentStep-1].ApproverRoles
				currentApproverIDs = version.Steps[item.CurrentStep-1].ApproverIDs
			}
		}
	}
	delegatedToName := ""
	if item.DelegatedToID != nil {
		delegatedTo, _ := repository.FindUserByID(ctx, *item.DelegatedToID)
		delegatedToName = delegatedTo.Name
		currentApproverIDs = []string{*item.DelegatedToID}
	}
	additionalApproverName := ""
	if item.AdditionalApproverID != nil {
		additionalApprover, _ := repository.FindUserByID(ctx, *item.AdditionalApproverID)
		additionalApproverName = additionalApprover.Name
		currentApproverIDs = []string{*item.AdditionalApproverID}
	}
	decisions, err := approvalDecisionOutputs(ctx, repository, item.ID)
	if err != nil {
		return ApprovalOutput{}, err
	}
	return ApprovalOutput{ID: item.ID, RecordID: item.RecordID, Module: record.Module, Name: fmt.Sprint(record.Data["name"]), Status: string(item.Status), RequestedBy: item.RequestedBy, RequesterName: requester.Name, ReviewerName: reviewerName, Comment: item.Comment, CurrentStep: item.CurrentStep, TotalSteps: totalSteps, CurrentApproverRoles: currentApproverRoles, CurrentApproverIDs: currentApproverIDs, DelegatedToID: item.DelegatedToID, DelegatedToName: delegatedToName, AdditionalApproverID: item.AdditionalApproverID, AdditionalApproverName: additionalApproverName, Decisions: decisions, CreatedAt: item.CreatedAt, ReviewedAt: item.ReviewedAt}, nil
}

func approvalDecisionOutputs(ctx context.Context, repository Repository, approvalID string) ([]ApprovalDecisionOutput, error) {
	items, err := repository.ListApprovalDecisions(ctx, approvalID)
	if err != nil {
		return nil, err
	}
	result := make([]ApprovalDecisionOutput, 0, len(items))
	for _, item := range items {
		reviewer, _ := repository.FindUserByID(ctx, item.ReviewerID)
		result = append(result, ApprovalDecisionOutput{ID: item.ID, ReviewerID: item.ReviewerID, ReviewerName: reviewer.Name, Decision: item.Decision, Comment: item.Comment, Step: item.Step, CreatedAt: item.CreatedAt})
	}
	return result, nil
}

func moduleLabel(key string) string {
	if item, ok := Modules[key]; ok {
		return item.Label
	}
	return key
}
