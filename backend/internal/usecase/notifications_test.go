package usecase

import (
	"context"
	"testing"
	"time"

	"cordialcrm/backend/internal/entity"
)

type notificationRepositoryStub struct {
	items   []entity.Notification
	read    bool
	readAll int
	created entity.Notification
}

func (s *notificationRepositoryStub) ListUsers(_ context.Context) ([]entity.User, error) {
	return []entity.User{{ID: "active", Username: "active", Active: true, OrganizationID: "org-1"}, {ID: "inactive", Active: false, OrganizationID: "org-1"}, {ID: "other", Active: true, OrganizationID: "org-2"}}, nil
}

func (s *notificationRepositoryStub) FindUserByID(_ context.Context, id string) (entity.User, error) {
	return entity.User{ID: id, Username: "wecom-user"}, nil
}

type notificationSenderStub struct{ recipient, subject, content string }

func (s *notificationSenderStub) Send(_ context.Context, recipient, subject, content string) error {
	s.recipient, s.subject, s.content = recipient, subject, content
	return nil
}

func (s *notificationRepositoryStub) CreateNotification(_ context.Context, item entity.Notification) error {
	s.created = item
	s.items = append(s.items, item)
	return nil
}

func (s *notificationRepositoryStub) ListNotifications(_ context.Context, userID string, status entity.NotificationStatus, page, size int) ([]entity.Notification, int, error) {
	items := make([]entity.Notification, 0)
	for _, item := range s.items {
		if item.UserID == userID && (status == "" || item.Status == status) {
			items = append(items, item)
		}
	}
	return items, len(items), nil
}
func (s *notificationRepositoryStub) ListAnnouncements(_ context.Context, organizationID string, page, size int) ([]entity.Notification, int, error) {
	items := make([]entity.Notification, 0)
	seen := map[string]bool{}
	for _, item := range s.items {
		if item.Type == "announcement" && item.ResourceID != "" && !seen[item.ResourceID] {
			seen[item.ResourceID] = true
			item.ID = item.ResourceID
			items = append(items, item)
		}
	}
	return items, len(items), nil
}
func (s *notificationRepositoryStub) UpdateAnnouncement(_ context.Context, organizationID, id, subject, content string) (bool, error) {
	updated := false
	for index := range s.items {
		if s.items[index].ResourceID == id {
			s.items[index].Subject, s.items[index].Content = subject, content
			updated = true
		}
	}
	return updated, nil
}
func (s *notificationRepositoryStub) DeleteAnnouncement(_ context.Context, organizationID, id string) (bool, error) {
	items := s.items[:0]
	deleted := false
	for _, item := range s.items {
		if item.ResourceID == id {
			deleted = true
			continue
		}
		items = append(items, item)
	}
	s.items = items
	return deleted, nil
}

func (s *notificationRepositoryStub) MarkNotificationRead(_ context.Context, id, userID string) (bool, error) {
	for index := range s.items {
		if s.items[index].ID == id && s.items[index].UserID == userID {
			s.items[index].Status = entity.NotificationRead
			s.read = true
			return true, nil
		}
	}
	return false, nil
}

func (s *notificationRepositoryStub) MarkAllNotificationsRead(_ context.Context, userID string) (int, error) {
	for index := range s.items {
		if s.items[index].UserID == userID && s.items[index].Status == entity.NotificationUnread {
			s.items[index].Status = entity.NotificationRead
			s.readAll++
		}
	}
	return s.readAll, nil
}

func TestNotificationCreateRequiresAdmin(t *testing.T) {
	repository := &notificationRepositoryStub{}
	service := NewNotification(repository)
	_, err := service.Create(context.Background(), CreateNotificationInput{Actor: UserOutput{Role: entity.RoleSales}, Recipient: UserOutput{ID: "user"}, Subject: "标题", Content: "内容"})
	if err == nil {
		t.Fatal("expected authorization error")
	}
}

func TestNotificationLifecycle(t *testing.T) {
	repository := &notificationRepositoryStub{items: []entity.Notification{{ID: "first", UserID: "user", Subject: "未读", Content: "内容", Status: entity.NotificationUnread, CreatedAt: time.Now()}, {ID: "second", UserID: "user", Subject: "已读", Content: "内容", Status: entity.NotificationRead, CreatedAt: time.Now()}}}
	service := NewNotification(repository)
	actor := UserOutput{ID: "user"}

	items, err := service.List(context.Background(), ListNotificationsInput{Actor: actor, Status: "unread"})
	if err != nil || items.Total != 1 || items.Items[0].ID != "first" {
		t.Fatalf("unexpected unread list: %#v, %v", items, err)
	}
	count, err := service.Count(context.Background(), actor)
	if err != nil || count.All != 2 || count.Unread != 1 || count.Read != 1 {
		t.Fatalf("unexpected count: %#v, %v", count, err)
	}
	if err = service.Read(context.Background(), ReadNotificationInput{Actor: actor, ID: "first"}); err != nil || !repository.read {
		t.Fatalf("expected notification to be read: %v", err)
	}
	if _, err = service.ReadAll(context.Background(), actor); err != nil {
		t.Fatalf("read all: %v", err)
	}
	count, err = service.Count(context.Background(), actor)
	if err != nil || count.Unread != 0 || count.Read != 2 {
		t.Fatalf("unexpected final count: %#v, %v", count, err)
	}
}

func TestNotificationForwardsToConfiguredSender(t *testing.T) {
	repository := &notificationRepositoryStub{}
	sender := &notificationSenderStub{}
	service := NewNotification(repository, sender)
	_, err := service.Create(context.Background(), CreateNotificationInput{Actor: UserOutput{Role: entity.RoleAdmin}, Recipient: UserOutput{ID: "user"}, Subject: "待审批", Content: "请处理报价"})
	if err != nil || sender.recipient != "wecom-user" || sender.subject != "待审批" || sender.content != "请处理报价" {
		t.Fatalf("unexpected external delivery: %#v, %v", sender, err)
	}
}

func TestBroadcastAnnouncementTargetsActiveOrganizationUsers(t *testing.T) {
	repository := &notificationRepositoryStub{}
	service := NewNotification(repository)
	count, err := service.BroadcastAnnouncement(context.Background(), BroadcastAnnouncementInput{Actor: UserOutput{Role: entity.RoleAdmin, OrganizationID: "org-1"}, Subject: "系统维护", Content: "今晚 22:00 开始维护"})
	if err != nil || count != 1 || len(repository.items) != 1 || repository.items[0].UserID != "active" || repository.items[0].Type != "announcement" {
		t.Fatalf("unexpected announcement result: count=%d, items=%#v, err=%v", count, repository.items, err)
	}
}

func TestAnnouncementManagementRequiresScopedPermissions(t *testing.T) {
	repository := &notificationRepositoryStub{items: []entity.Notification{{ID: "copy-1", UserID: "active", Type: "announcement", Subject: "旧标题", Content: "旧内容", ResourceID: "announcement-1", ResourceType: "announcement", Status: entity.NotificationUnread}}}
	service := NewNotification(repository)
	actor := UserOutput{Role: entity.RoleSales, RoleID: "role-1", OrganizationID: "org-1", PermissionIDs: []string{"SYSTEM_NOTICE:READ", "SYSTEM_NOTICE:UPDATE", "SYSTEM_NOTICE:DELETE"}}
	listed, err := service.ListAnnouncements(context.Background(), ListNotificationsInput{Actor: actor})
	if err != nil || listed.Total != 1 || listed.Items[0].ID != "announcement-1" {
		t.Fatalf("list announcements: %#v err=%v", listed, err)
	}
	if err := service.UpdateAnnouncement(context.Background(), AnnouncementInput{Actor: actor, ID: "announcement-1", Subject: "新标题", Content: "新内容"}); err != nil || repository.items[0].Subject != "新标题" {
		t.Fatalf("update announcement: items=%#v err=%v", repository.items, err)
	}
	if err := service.DeleteAnnouncement(context.Background(), AnnouncementInput{Actor: actor, ID: "announcement-1"}); err != nil || len(repository.items) != 0 {
		t.Fatalf("delete announcement: items=%#v err=%v", repository.items, err)
	}
	actor.PermissionIDs = nil
	if _, err := service.ListAnnouncements(context.Background(), ListNotificationsInput{Actor: actor}); err != ErrForbidden {
		t.Fatalf("expected announcement read rejection, got %v", err)
	}
}
