package usecase

import (
	"context"
	"strings"
	"time"

	"cordialcrm/backend/internal/entity"
)

type FollowupPlanUseCase struct{ repo FollowupPlanRepository }

func NewFollowupPlan(repo FollowupPlanRepository) *FollowupPlanUseCase {
	return &FollowupPlanUseCase{repo: repo}
}

type FollowupPlanInput struct {
	Actor                                      UserOutput
	Module, RecordID, OwnerID, Content, Method string
	EstimatedAt                                time.Time
}
type UpdateFollowupPlanInput struct {
	FollowupPlanInput
	ID      string
	Convert bool
}
type UpdateFollowupPlanStatusInput struct {
	Actor      UserOutput
	ID, Status string
}
type ListFollowupPlansInput struct {
	Actor                    UserOutput
	Module, RecordID, Status string
	Mine                     bool
	Page, Size               int
}
type FollowupPlanOutput struct {
	ID          string    `json:"id"`
	Module      string    `json:"module"`
	RecordID    string    `json:"record_id"`
	OwnerID     string    `json:"owner_id"`
	OwnerName   string    `json:"owner_name"`
	Content     string    `json:"content"`
	Method      string    `json:"method"`
	Status      string    `json:"status"`
	EstimatedAt time.Time `json:"estimated_at"`
	Converted   bool      `json:"converted"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
type ListFollowupPlansOutput struct {
	Items []FollowupPlanOutput `json:"items"`
	Total int                  `json:"total"`
	Page  int                  `json:"page"`
	Size  int                  `json:"size"`
}
type CreateFollowupPlanCommentInput struct {
	Actor           UserOutput
	PlanID, Content string
}
type ListFollowupPlanCommentsInput struct {
	Actor  UserOutput
	PlanID string
}
type DeleteFollowupPlanCommentInput struct {
	Actor             UserOutput
	PlanID, CommentID string
}
type UpdateFollowupPlanCommentInput struct {
	Actor                      UserOutput
	PlanID, CommentID, Content string
}
type FollowupPlanCommentOutput struct {
	ID        string    `json:"id"`
	PlanID    string    `json:"plan_id"`
	UserID    string    `json:"user_id"`
	UserName  string    `json:"user_name"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
type ListFollowupPlanCommentsOutput struct {
	Items []FollowupPlanCommentOutput `json:"items"`
}

func (u *FollowupPlanUseCase) Create(ctx context.Context, input FollowupPlanInput) (FollowupPlanOutput, error) {
	if err := u.access(ctx, input.Actor, input.Module, input.RecordID, true); err != nil {
		return FollowupPlanOutput{}, err
	}
	ownerID, content, method, err := u.normalize(ctx, input)
	if err != nil {
		return FollowupPlanOutput{}, err
	}
	now := time.Now().UTC()
	plan := entity.FollowupPlan{ID: entity.NewID(), Module: input.Module, RecordID: input.RecordID, OwnerID: ownerID, Content: content, Method: method, EstimatedAt: input.EstimatedAt.UTC(), Status: entity.FollowupPlanPrepared, CreatedAt: now, UpdatedAt: now}
	if err = u.repo.CreateFollowupPlan(ctx, plan); err != nil {
		return FollowupPlanOutput{}, err
	}
	return u.output(ctx, plan)
}
func (u *FollowupPlanUseCase) List(ctx context.Context, input ListFollowupPlansInput) (ListFollowupPlansOutput, error) {
	if err := u.access(ctx, input.Actor, input.Module, input.RecordID, false); err != nil {
		return ListFollowupPlansOutput{}, err
	}
	status, err := followupPlanStatus(input.Status)
	if err != nil {
		return ListFollowupPlansOutput{}, err
	}
	if input.Page < 1 {
		input.Page = 1
	}
	if input.Size < 1 || input.Size > 100 {
		input.Size = 20
	}
	ownerID := ""
	if input.Mine {
		ownerID = input.Actor.ID
	}
	plans, total, err := u.repo.ListFollowupPlans(ctx, input.Module, input.RecordID, status, ownerID, input.Page, input.Size)
	if err != nil {
		return ListFollowupPlansOutput{}, err
	}
	items := make([]FollowupPlanOutput, 0, len(plans))
	for _, plan := range plans {
		output, err := u.output(ctx, plan)
		if err != nil {
			return ListFollowupPlansOutput{}, err
		}
		items = append(items, output)
	}
	return ListFollowupPlansOutput{Items: items, Total: total, Page: input.Page, Size: input.Size}, nil
}
func (u *FollowupPlanUseCase) Update(ctx context.Context, input UpdateFollowupPlanInput) (FollowupPlanOutput, error) {
	plan, err := u.editable(ctx, input.Actor, input.ID)
	if err != nil {
		return FollowupPlanOutput{}, err
	}
	input.Module, input.RecordID = plan.Module, plan.RecordID
	ownerID, content, method, err := u.normalize(ctx, input.FollowupPlanInput)
	if err != nil {
		return FollowupPlanOutput{}, err
	}
	plan.OwnerID, plan.Content, plan.Method, plan.EstimatedAt, plan.UpdatedAt = ownerID, content, method, input.EstimatedAt.UTC(), time.Now().UTC()
	if input.Convert && !plan.Converted {
		plan.Status, plan.Converted = entity.FollowupPlanCompleted, true
		if err = u.repo.CreateFollowup(ctx, entity.Followup{ID: entity.NewID(), RecordID: plan.RecordID, UserID: input.Actor.ID, Content: plan.Content, Method: plan.Method, CreatedAt: plan.UpdatedAt}); err != nil {
			return FollowupPlanOutput{}, err
		}
	}
	if err = u.repo.UpdateFollowupPlan(ctx, plan); err != nil {
		return FollowupPlanOutput{}, err
	}
	return u.output(ctx, plan)
}
func (u *FollowupPlanUseCase) UpdateStatus(ctx context.Context, input UpdateFollowupPlanStatusInput) (FollowupPlanOutput, error) {
	plan, err := u.editable(ctx, input.Actor, input.ID)
	if err != nil {
		return FollowupPlanOutput{}, err
	}
	status, err := followupPlanStatus(input.Status)
	if err != nil || status == "" {
		return FollowupPlanOutput{}, ErrValidation
	}
	plan.Status, plan.UpdatedAt = status, time.Now().UTC()
	if err = u.repo.UpdateFollowupPlan(ctx, plan); err != nil {
		return FollowupPlanOutput{}, err
	}
	return u.output(ctx, plan)
}
func (u *FollowupPlanUseCase) Delete(ctx context.Context, actor UserOutput, id string) error {
	plan, err := u.editable(ctx, actor, id)
	if err != nil {
		return err
	}
	if plan.Converted {
		return entity.NewBusinessError("conflict", "已转为跟进记录的计划不能删除")
	}
	return u.repo.DeleteFollowupPlan(ctx, id)
}
func (u *FollowupPlanUseCase) CreateComment(ctx context.Context, input CreateFollowupPlanCommentInput) (FollowupPlanCommentOutput, error) {
	plan, err := u.repo.FindFollowupPlan(ctx, input.PlanID)
	if err != nil {
		return FollowupPlanCommentOutput{}, ErrNotFound
	}
	if err = u.access(ctx, input.Actor, plan.Module, plan.RecordID, true); err != nil {
		return FollowupPlanCommentOutput{}, err
	}
	content := strings.TrimSpace(input.Content)
	if content == "" || len([]rune(content)) > 2000 {
		return FollowupPlanCommentOutput{}, ErrValidation
	}
	now := time.Now().UTC()
	comment := entity.FollowupPlanComment{ID: entity.NewID(), PlanID: plan.ID, UserID: input.Actor.ID, Content: content, CreatedAt: now, UpdatedAt: now}
	if err = u.repo.CreateFollowupPlanComment(ctx, comment); err != nil {
		return FollowupPlanCommentOutput{}, err
	}
	return u.commentOutput(ctx, comment)
}
func (u *FollowupPlanUseCase) UpdateComment(ctx context.Context, input UpdateFollowupPlanCommentInput) (FollowupPlanCommentOutput, error) {
	plan, err := u.repo.FindFollowupPlan(ctx, input.PlanID)
	if err != nil {
		return FollowupPlanCommentOutput{}, ErrNotFound
	}
	if err = u.access(ctx, input.Actor, plan.Module, plan.RecordID, true); err != nil {
		return FollowupPlanCommentOutput{}, err
	}
	comment, err := u.repo.FindFollowupPlanComment(ctx, input.CommentID)
	if err != nil || comment.PlanID != plan.ID {
		return FollowupPlanCommentOutput{}, ErrNotFound
	}
	if comment.UserID != input.Actor.ID && !isManager(input.Actor) {
		return FollowupPlanCommentOutput{}, ErrForbidden
	}
	content := strings.TrimSpace(input.Content)
	if content == "" || len([]rune(content)) > 2000 {
		return FollowupPlanCommentOutput{}, ErrValidation
	}
	comment.Content, comment.UpdatedAt = content, time.Now().UTC()
	if err = u.repo.UpdateFollowupPlanComment(ctx, comment); err != nil {
		return FollowupPlanCommentOutput{}, err
	}
	return u.commentOutput(ctx, comment)
}
func (u *FollowupPlanUseCase) ListComments(ctx context.Context, input ListFollowupPlanCommentsInput) (ListFollowupPlanCommentsOutput, error) {
	plan, err := u.repo.FindFollowupPlan(ctx, input.PlanID)
	if err != nil {
		return ListFollowupPlanCommentsOutput{}, ErrNotFound
	}
	if err = u.access(ctx, input.Actor, plan.Module, plan.RecordID, false); err != nil {
		return ListFollowupPlanCommentsOutput{}, err
	}
	comments, err := u.repo.ListFollowupPlanComments(ctx, plan.ID)
	if err != nil {
		return ListFollowupPlanCommentsOutput{}, err
	}
	items := make([]FollowupPlanCommentOutput, 0, len(comments))
	for _, comment := range comments {
		output, err := u.commentOutput(ctx, comment)
		if err != nil {
			return ListFollowupPlanCommentsOutput{}, err
		}
		items = append(items, output)
	}
	return ListFollowupPlanCommentsOutput{Items: items}, nil
}
func (u *FollowupPlanUseCase) DeleteComment(ctx context.Context, input DeleteFollowupPlanCommentInput) error {
	plan, err := u.repo.FindFollowupPlan(ctx, input.PlanID)
	if err != nil {
		return ErrNotFound
	}
	if err = u.access(ctx, input.Actor, plan.Module, plan.RecordID, true); err != nil {
		return err
	}
	comment, err := u.repo.FindFollowupPlanComment(ctx, input.CommentID)
	if err != nil || comment.PlanID != plan.ID {
		return ErrNotFound
	}
	if comment.UserID != input.Actor.ID && !isManager(input.Actor) {
		return ErrForbidden
	}
	return u.repo.DeleteFollowupPlanComment(ctx, comment.ID)
}
func (u *FollowupPlanUseCase) normalize(ctx context.Context, input FollowupPlanInput) (string, string, string, error) {
	if input.Module != "customers" && input.Module != "leads" && input.Module != "opportunities" {
		return "", "", "", ErrValidation
	}
	content, method := strings.TrimSpace(input.Content), strings.TrimSpace(input.Method)
	if content == "" || len([]rune(content)) > 3000 || method == "" || len([]rune(method)) > 32 || input.EstimatedAt.IsZero() {
		return "", "", "", ErrValidation
	}
	ownerID := input.OwnerID
	if ownerID == "" {
		ownerID = input.Actor.ID
	}
	owner, err := u.repo.FindUserByID(ctx, ownerID)
	if err != nil || !owner.Active {
		return "", "", "", entity.NewBusinessError("validation", "计划负责人不存在或已停用")
	}
	if ownerID != input.Actor.ID && !isManager(input.Actor) {
		return "", "", "", ErrForbidden
	}
	return ownerID, content, method, nil
}
func (u *FollowupPlanUseCase) access(ctx context.Context, actor UserOutput, moduleKey, recordID string, write bool) error {
	m, err := module(moduleKey)
	if err != nil {
		return err
	}
	if moduleKey != "customers" && moduleKey != "leads" && moduleKey != "opportunities" {
		return ErrValidation
	}
	record, err := u.repo.FindRecord(ctx, moduleKey, recordID)
	if err != nil || !visible(actor, m, record, write) {
		return ErrNotFound
	}
	return nil
}
func (u *FollowupPlanUseCase) editable(ctx context.Context, actor UserOutput, id string) (entity.FollowupPlan, error) {
	plan, err := u.repo.FindFollowupPlan(ctx, id)
	if err != nil {
		return entity.FollowupPlan{}, ErrNotFound
	}
	if err = u.access(ctx, actor, plan.Module, plan.RecordID, true); err != nil {
		return entity.FollowupPlan{}, err
	}
	if plan.OwnerID != actor.ID && !isManager(actor) {
		return entity.FollowupPlan{}, ErrForbidden
	}
	return plan, nil
}
func (u *FollowupPlanUseCase) output(ctx context.Context, plan entity.FollowupPlan) (FollowupPlanOutput, error) {
	owner, err := u.repo.FindUserByID(ctx, plan.OwnerID)
	if err != nil {
		return FollowupPlanOutput{}, err
	}
	return FollowupPlanOutput{ID: plan.ID, Module: plan.Module, RecordID: plan.RecordID, OwnerID: plan.OwnerID, OwnerName: owner.Name, Content: plan.Content, Method: plan.Method, Status: string(plan.Status), EstimatedAt: plan.EstimatedAt, Converted: plan.Converted, CreatedAt: plan.CreatedAt, UpdatedAt: plan.UpdatedAt}, nil
}
func (u *FollowupPlanUseCase) commentOutput(ctx context.Context, comment entity.FollowupPlanComment) (FollowupPlanCommentOutput, error) {
	user, err := u.repo.FindUserByID(ctx, comment.UserID)
	if err != nil {
		return FollowupPlanCommentOutput{}, err
	}
	return FollowupPlanCommentOutput{ID: comment.ID, PlanID: comment.PlanID, UserID: comment.UserID, UserName: user.Name, Content: comment.Content, CreatedAt: comment.CreatedAt, UpdatedAt: comment.UpdatedAt}, nil
}
func followupPlanStatus(value string) (entity.FollowupPlanStatus, error) {
	if value == "" {
		return "", nil
	}
	status := entity.FollowupPlanStatus(value)
	if status != entity.FollowupPlanPrepared && status != entity.FollowupPlanUnderway && status != entity.FollowupPlanCompleted && status != entity.FollowupPlanCancelled {
		return "", ErrValidation
	}
	return status, nil
}
