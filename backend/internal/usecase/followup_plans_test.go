package usecase

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"cordialcrm/backend/internal/entity"
)

func TestFollowupPlanCommentOutputUsesFrontendFieldNames(t *testing.T) {
	createdAt := time.Date(2026, time.October, 7, 6, 30, 0, 0, time.UTC)
	payload, err := json.Marshal(FollowupPlanCommentOutput{ID: "comment-1", PlanID: "plan-1", UserID: "user-1", UserName: "销售甲", Content: "按计划推进", CreatedAt: createdAt, UpdatedAt: createdAt})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{`"plan_id":"plan-1"`, `"user_id":"user-1"`, `"user_name":"销售甲"`, `"created_at":"2026-10-07T06:30:00Z"`} {
		if !strings.Contains(string(payload), expected) {
			t.Fatalf("followup plan comment field missing: %s in %s", expected, payload)
		}
	}
}

type followupPlanRepositoryStub struct {
	record    entity.Record
	user      entity.User
	plan      entity.FollowupPlan
	followups []entity.Followup
	comments  []entity.FollowupPlanComment
}

func (s *followupPlanRepositoryStub) FindRecord(_ context.Context, _, _ string) (entity.Record, error) {
	return s.record, nil
}
func (s *followupPlanRepositoryStub) FindUserByID(_ context.Context, _ string) (entity.User, error) {
	return s.user, nil
}
func (s *followupPlanRepositoryStub) CreateFollowupPlan(_ context.Context, plan entity.FollowupPlan) error {
	s.plan = plan
	return nil
}
func (s *followupPlanRepositoryStub) FindFollowupPlan(_ context.Context, _ string) (entity.FollowupPlan, error) {
	return s.plan, nil
}
func (s *followupPlanRepositoryStub) ListFollowupPlans(_ context.Context, _, _ string, _ entity.FollowupPlanStatus, _ string, _, _ int) ([]entity.FollowupPlan, int, error) {
	return []entity.FollowupPlan{s.plan}, 1, nil
}
func (s *followupPlanRepositoryStub) UpdateFollowupPlan(_ context.Context, plan entity.FollowupPlan) error {
	s.plan = plan
	return nil
}
func (s *followupPlanRepositoryStub) DeleteFollowupPlan(_ context.Context, _ string) error {
	s.plan = entity.FollowupPlan{}
	s.comments = nil
	return nil
}
func (s *followupPlanRepositoryStub) CreateFollowupPlanComment(_ context.Context, item entity.FollowupPlanComment) error {
	s.comments = append(s.comments, item)
	return nil
}
func (s *followupPlanRepositoryStub) FindFollowupPlanComment(_ context.Context, id string) (entity.FollowupPlanComment, error) {
	for _, item := range s.comments {
		if item.ID == id {
			return item, nil
		}
	}
	return entity.FollowupPlanComment{}, context.Canceled
}
func (s *followupPlanRepositoryStub) ListFollowupPlanComments(_ context.Context, _ string) ([]entity.FollowupPlanComment, error) {
	return s.comments, nil
}
func (s *followupPlanRepositoryStub) DeleteFollowupPlanComment(_ context.Context, id string) error {
	for index, item := range s.comments {
		if item.ID == id {
			s.comments = append(s.comments[:index], s.comments[index+1:]...)
			return nil
		}
	}
	return context.Canceled
}
func (s *followupPlanRepositoryStub) UpdateFollowupPlanComment(_ context.Context, updated entity.FollowupPlanComment) error {
	for index, item := range s.comments {
		if item.ID == updated.ID {
			s.comments[index] = updated
			return nil
		}
	}
	return context.Canceled
}
func (s *followupPlanRepositoryStub) CreateFollowup(_ context.Context, item entity.Followup) error {
	s.followups = append(s.followups, item)
	return nil
}

func TestFollowupPlanCreateAndConvert(t *testing.T) {
	actor := UserOutput{ID: "user-1", Role: entity.RoleAdmin}
	repository := &followupPlanRepositoryStub{record: entity.Record{ID: "customer-1", Module: "customers", OwnerID: &actor.ID}, user: entity.User{ID: actor.ID, Name: "演示用户", Active: true}}
	service := NewFollowupPlan(repository)
	created, err := service.Create(context.Background(), FollowupPlanInput{Actor: actor, Module: "customers", RecordID: "customer-1", Content: "确认续费范围", Method: "电话", EstimatedAt: time.Now().Add(time.Hour)})
	if err != nil || created.Status != string(entity.FollowupPlanPrepared) || created.OwnerID != actor.ID {
		t.Fatalf("unexpected created plan: %#v, %v", created, err)
	}
	converted, err := service.Update(context.Background(), UpdateFollowupPlanInput{ID: created.ID, Convert: true, FollowupPlanInput: FollowupPlanInput{Actor: actor, Content: "确认续费范围", Method: "电话", EstimatedAt: created.EstimatedAt}})
	if err != nil || converted.Status != string(entity.FollowupPlanCompleted) || !converted.Converted || len(repository.followups) != 1 {
		t.Fatalf("unexpected converted plan: %#v, followups=%#v, err=%v", converted, repository.followups, err)
	}
}

func TestFollowupPlanRejectsInvalidStatus(t *testing.T) {
	if _, err := followupPlanStatus("invalid"); err == nil {
		t.Fatal("expected invalid status error")
	}
}

func TestFollowupPlanComment(t *testing.T) {
	actor := UserOutput{ID: "user-1", Role: entity.RoleAdmin}
	repository := &followupPlanRepositoryStub{record: entity.Record{ID: "customer-1", Module: "customers", OwnerID: &actor.ID}, user: entity.User{ID: actor.ID, Name: "演示用户", Active: true}, plan: entity.FollowupPlan{ID: "plan-1", Module: "customers", RecordID: "customer-1", OwnerID: actor.ID}}
	service := NewFollowupPlan(repository)
	created, err := service.CreateComment(context.Background(), CreateFollowupPlanCommentInput{Actor: actor, PlanID: "plan-1", Content: "请在会议前确认报价范围"})
	if err != nil || created.UserName != "演示用户" || len(repository.comments) != 1 {
		t.Fatalf("unexpected created comment: %#v, comments=%#v, err=%v", created, repository.comments, err)
	}
	listed, err := service.ListComments(context.Background(), ListFollowupPlanCommentsInput{Actor: actor, PlanID: "plan-1"})
	if err != nil || len(listed.Items) != 1 || listed.Items[0].Content != created.Content {
		t.Fatalf("unexpected listed comments: %#v, err=%v", listed, err)
	}
	updated, err := service.UpdateComment(context.Background(), UpdateFollowupPlanCommentInput{Actor: actor, PlanID: "plan-1", CommentID: created.ID, Content: "请在会议前确认最终报价范围"})
	if err != nil || updated.Content != "请在会议前确认最终报价范围" || !updated.UpdatedAt.After(updated.CreatedAt) {
		t.Fatalf("unexpected updated comment: %#v, err=%v", updated, err)
	}
}

func TestFollowupPlanDeleteClearsComments(t *testing.T) {
	actor := UserOutput{ID: "user-1", Role: entity.RoleAdmin}
	repository := &followupPlanRepositoryStub{record: entity.Record{ID: "customer-1", Module: "customers", OwnerID: &actor.ID}, user: entity.User{ID: actor.ID, Name: "演示用户", Active: true}, plan: entity.FollowupPlan{ID: "plan-1", Module: "customers", RecordID: "customer-1", OwnerID: actor.ID}, comments: []entity.FollowupPlanComment{{ID: "comment-1", PlanID: "plan-1"}}}
	if err := NewFollowupPlan(repository).Delete(context.Background(), actor, "plan-1"); err != nil || len(repository.comments) != 0 {
		t.Fatalf("unexpected deleted comments: %#v, err=%v", repository.comments, err)
	}
}
