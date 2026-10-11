package usecase

import (
	"context"
	"testing"
	"time"

	"cordialcrm/backend/internal/entity"
)

type followupReminderRepositoryStub struct {
	plans         []entity.FollowupPlan
	reminders     map[string]entity.FollowupPlanReminder
	notifications []entity.Notification
}

func (s *followupReminderRepositoryStub) ListFollowupPlansDueBy(_ context.Context, from, until time.Time) ([]entity.FollowupPlan, error) {
	items := make([]entity.FollowupPlan, 0, len(s.plans))
	for _, plan := range s.plans {
		if !plan.EstimatedAt.Before(from) && !plan.EstimatedAt.After(until) && !plan.Converted && (plan.Status == entity.FollowupPlanPrepared || plan.Status == entity.FollowupPlanUnderway) {
			items = append(items, plan)
		}
	}
	return items, nil
}

func (s *followupReminderRepositoryStub) DeliverFollowupPlanReminder(_ context.Context, reminder entity.FollowupPlanReminder, notification entity.Notification) (bool, error) {
	key := reminder.PlanID + ":" + reminder.UserID
	if _, found := s.reminders[key]; found {
		return false, nil
	}
	s.reminders[key] = reminder
	s.notifications = append(s.notifications, notification)
	return true, nil
}

func TestFollowupReminderDeliversOnceWithinWindow(t *testing.T) {
	now := time.Date(2026, time.September, 26, 10, 0, 0, 0, time.UTC)
	repository := &followupReminderRepositoryStub{plans: []entity.FollowupPlan{{ID: "plan-1", OwnerID: "user-1", Content: "确认续费", EstimatedAt: now.Add(24 * time.Hour), Status: entity.FollowupPlanPrepared}}, reminders: map[string]entity.FollowupPlanReminder{}}
	service := NewFollowupReminder(repository)
	first, err := service.Run(context.Background(), RunFollowupRemindersInput{Now: now})
	if err != nil || first.Candidates != 1 || first.Delivered != 1 || len(repository.notifications) != 1 {
		t.Fatalf("unexpected first run: %#v, notifications=%#v, err=%v", first, repository.notifications, err)
	}
	second, err := service.Run(context.Background(), RunFollowupRemindersInput{Now: now.Add(time.Minute)})
	if err != nil || second.Delivered != 0 || len(repository.notifications) != 1 {
		t.Fatalf("unexpected second run: %#v, notifications=%#v, err=%v", second, repository.notifications, err)
	}
}

func TestFollowupReminderSkipsIneligiblePlans(t *testing.T) {
	now := time.Date(2026, time.September, 26, 10, 0, 0, 0, time.UTC)
	repository := &followupReminderRepositoryStub{plans: []entity.FollowupPlan{
		{ID: "past", OwnerID: "user-1", EstimatedAt: now.Add(-time.Second), Status: entity.FollowupPlanPrepared},
		{ID: "late", OwnerID: "user-1", EstimatedAt: now.Add(24*time.Hour + time.Second), Status: entity.FollowupPlanPrepared},
		{ID: "completed", OwnerID: "user-1", EstimatedAt: now.Add(time.Hour), Status: entity.FollowupPlanCompleted},
		{ID: "converted", OwnerID: "user-1", EstimatedAt: now.Add(time.Hour), Status: entity.FollowupPlanPrepared, Converted: true},
	}, reminders: map[string]entity.FollowupPlanReminder{}}
	output, err := NewFollowupReminder(repository).Run(context.Background(), RunFollowupRemindersInput{Now: now})
	if err != nil || output.Candidates != 0 || output.Delivered != 0 || len(repository.notifications) != 0 {
		t.Fatalf("unexpected output: %#v, notifications=%#v, err=%v", output, repository.notifications, err)
	}
}
