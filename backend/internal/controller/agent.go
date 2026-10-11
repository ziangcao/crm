package controller

import (
	"context"

	"cordialcrm/backend/internal/usecase"
)

type RecordTool struct {
	records *usecase.RecordUseCase
}

func NewRecordTool(records *usecase.RecordUseCase) *RecordTool {
	return &RecordTool{records: records}
}

func (t *RecordTool) List(ctx context.Context, input usecase.ListRecordsInput) (usecase.ListRecordsOutput, error) {
	return t.records.List(ctx, input)
}

func (t *RecordTool) Get(ctx context.Context, input usecase.RecordInput) (usecase.RecordOutput, error) {
	return t.records.Get(ctx, input)
}

func (t *RecordTool) Create(ctx context.Context, input usecase.RecordInput) (usecase.RecordOutput, error) {
	return t.records.Create(ctx, input)
}

func (t *RecordTool) Update(ctx context.Context, input usecase.RecordInput) (usecase.RecordOutput, error) {
	return t.records.Update(ctx, input)
}

func (t *RecordTool) Action(ctx context.Context, input usecase.ActionInput) (usecase.RecordOutput, error) {
	return t.records.Action(ctx, input)
}

func (t *RecordTool) Search(ctx context.Context, input usecase.SearchInput) (usecase.SearchOutput, error) {
	return t.records.Search(ctx, input)
}

type NotificationTool struct {
	notifications *usecase.NotificationUseCase
}

type FollowupPlanTool struct{ plans *usecase.FollowupPlanUseCase }

func NewFollowupPlanTool(plans *usecase.FollowupPlanUseCase) *FollowupPlanTool {
	return &FollowupPlanTool{plans: plans}
}

func (t *FollowupPlanTool) List(ctx context.Context, input usecase.ListFollowupPlansInput) (usecase.ListFollowupPlansOutput, error) {
	return t.plans.List(ctx, input)
}
func (t *FollowupPlanTool) Create(ctx context.Context, input usecase.FollowupPlanInput) (usecase.FollowupPlanOutput, error) {
	return t.plans.Create(ctx, input)
}
func (t *FollowupPlanTool) Update(ctx context.Context, input usecase.UpdateFollowupPlanInput) (usecase.FollowupPlanOutput, error) {
	return t.plans.Update(ctx, input)
}
func (t *FollowupPlanTool) UpdateStatus(ctx context.Context, input usecase.UpdateFollowupPlanStatusInput) (usecase.FollowupPlanOutput, error) {
	return t.plans.UpdateStatus(ctx, input)
}
func (t *FollowupPlanTool) Delete(ctx context.Context, actor usecase.UserOutput, id string) error {
	return t.plans.Delete(ctx, actor, id)
}

func NewNotificationTool(notifications *usecase.NotificationUseCase) *NotificationTool {
	return &NotificationTool{notifications: notifications}
}

func (t *NotificationTool) List(ctx context.Context, input usecase.ListNotificationsInput) (usecase.ListNotificationsOutput, error) {
	return t.notifications.List(ctx, input)
}

func (t *NotificationTool) Count(ctx context.Context, actor usecase.UserOutput) (usecase.NotificationCountOutput, error) {
	return t.notifications.Count(ctx, actor)
}

func (t *NotificationTool) Read(ctx context.Context, input usecase.ReadNotificationInput) error {
	return t.notifications.Read(ctx, input)
}

func (t *NotificationTool) ReadAll(ctx context.Context, actor usecase.UserOutput) (int, error) {
	return t.notifications.ReadAll(ctx, actor)
}
