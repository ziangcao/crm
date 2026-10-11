package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cordialcrm/backend/internal/entity"
)

const followupReminderWindow = 24 * time.Hour

type FollowupReminderUseCase struct {
	repo  FollowupReminderRepository
	clock func() time.Time
}

func NewFollowupReminder(repo FollowupReminderRepository) *FollowupReminderUseCase {
	return &FollowupReminderUseCase{repo: repo, clock: time.Now}
}

type RunFollowupRemindersInput struct {
	Now time.Time
}

type RunFollowupRemindersOutput struct {
	Candidates int
	Delivered  int
}

func (u *FollowupReminderUseCase) Run(ctx context.Context, input RunFollowupRemindersInput) (RunFollowupRemindersOutput, error) {
	now := input.Now
	if now.IsZero() {
		now = u.clock()
	}
	now = now.UTC()
	plans, err := u.repo.ListFollowupPlansDueBy(ctx, now, now.Add(followupReminderWindow))
	if err != nil {
		return RunFollowupRemindersOutput{}, err
	}
	output := RunFollowupRemindersOutput{Candidates: len(plans)}
	for _, plan := range plans {
		remindAt := plan.EstimatedAt.UTC().Add(-followupReminderWindow)
		notification := entity.Notification{
			ID:           entity.NewID(),
			UserID:       plan.OwnerID,
			Type:         "followup_reminder",
			Subject:      "跟进计划即将到期",
			Content:      fmt.Sprintf("%s（预计时间：%s）", strings.TrimSpace(plan.Content), plan.EstimatedAt.In(time.Local).Format("2006-01-02 15:04")),
			ResourceID:   plan.ID,
			ResourceType: "followup_plan",
			Status:       entity.NotificationUnread,
			CreatedAt:    now,
		}
		delivered, err := u.repo.DeliverFollowupPlanReminder(ctx, entity.FollowupPlanReminder{ID: entity.NewID(), PlanID: plan.ID, UserID: plan.OwnerID, RemindAt: remindAt}, notification)
		if err != nil {
			return RunFollowupRemindersOutput{}, err
		}
		if delivered {
			output.Delivered++
		}
	}
	return output, nil
}
