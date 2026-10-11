package usecase

import (
	"context"
	"strings"
	"time"

	"cordialcrm/backend/internal/entity"
)

type NotificationSender interface {
	Send(context.Context, string, string, string) error
}

type notificationRecipientRepository interface {
	FindUserByID(context.Context, string) (entity.User, error)
	ListUsers(context.Context) ([]entity.User, error)
}

type NotificationUseCase struct {
	repo       NotificationRepository
	recipients notificationRecipientRepository
	senders    []NotificationSender
}

func NewNotification(repo NotificationRepository, senders ...NotificationSender) *NotificationUseCase {
	recipients, _ := repo.(notificationRecipientRepository)
	return &NotificationUseCase{repo: repo, recipients: recipients, senders: senders}
}

type CreateNotificationInput struct {
	Actor, Recipient                                 UserOutput
	Type, Subject, Content, ResourceID, ResourceType string
}
type ListNotificationsInput struct {
	Actor      UserOutput
	Status     string
	Page, Size int
}
type NotificationOutput struct {
	ID           string    `json:"id"`
	Type         string    `json:"type"`
	Subject      string    `json:"subject"`
	Content      string    `json:"content"`
	ResourceID   string    `json:"resource_id"`
	ResourceType string    `json:"resource_type"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}
type ListNotificationsOutput struct {
	Items []NotificationOutput `json:"items"`
	Total int                  `json:"total"`
}
type NotificationCountOutput struct {
	All    int `json:"all"`
	Unread int `json:"unread"`
	Read   int `json:"read"`
}
type ReadNotificationInput struct {
	Actor UserOutput
	ID    string
}
type BroadcastAnnouncementInput struct {
	Actor            UserOutput
	Subject, Content string
}
type AnnouncementInput struct {
	Actor            UserOutput
	ID               string
	Subject, Content string
}

func notificationOutput(item entity.Notification) NotificationOutput {
	return NotificationOutput{ID: item.ID, Type: item.Type, Subject: item.Subject, Content: item.Content, ResourceID: item.ResourceID, ResourceType: item.ResourceType, Status: string(item.Status), CreatedAt: item.CreatedAt}
}

type notificationMessage struct {
	RecipientID, Type, Subject, Content, ResourceID, ResourceType string
}

func (u *NotificationUseCase) send(ctx context.Context, message notificationMessage) error {
	if message.RecipientID == "" || strings.TrimSpace(message.Subject) == "" || strings.TrimSpace(message.Content) == "" {
		return ErrValidation
	}
	item := entity.Notification{ID: entity.NewID(), UserID: message.RecipientID, Type: strings.TrimSpace(message.Type), Subject: strings.TrimSpace(message.Subject), Content: strings.TrimSpace(message.Content), ResourceID: message.ResourceID, ResourceType: message.ResourceType, Status: entity.NotificationUnread, CreatedAt: time.Now().UTC()}
	if err := u.repo.CreateNotification(ctx, item); err != nil {
		return err
	}
	u.deliver(ctx, item)
	return nil
}

func (u *NotificationUseCase) Create(ctx context.Context, input CreateNotificationInput) (NotificationOutput, error) {
	if err := requirePermission(input.Actor, "SYSTEM_NOTICE:ADD"); err != nil {
		return NotificationOutput{}, err
	}
	if input.Recipient.ID == "" || strings.TrimSpace(input.Subject) == "" || strings.TrimSpace(input.Content) == "" {
		return NotificationOutput{}, entity.NewBusinessError("validation", "通知内容不完整")
	}
	item := entity.Notification{ID: entity.NewID(), UserID: input.Recipient.ID, Type: strings.TrimSpace(input.Type), Subject: strings.TrimSpace(input.Subject), Content: strings.TrimSpace(input.Content), ResourceID: input.ResourceID, ResourceType: input.ResourceType, Status: entity.NotificationUnread, CreatedAt: time.Now().UTC()}
	if err := u.repo.CreateNotification(ctx, item); err != nil {
		return NotificationOutput{}, err
	}
	u.deliver(ctx, item)
	return notificationOutput(item), nil
}

func (u *NotificationUseCase) BroadcastAnnouncement(ctx context.Context, input BroadcastAnnouncementInput) (int, error) {
	if err := requirePermission(input.Actor, "SYSTEM_NOTICE:ADD"); err != nil {
		return 0, err
	}
	if u.recipients == nil || strings.TrimSpace(input.Subject) == "" || strings.TrimSpace(input.Content) == "" || len([]rune(input.Subject)) > 100 || len([]rune(input.Content)) > 5000 {
		return 0, ErrValidation
	}
	users, err := u.recipients.ListUsers(ctx)
	if err != nil {
		return 0, err
	}
	count := 0
	announcementID := entity.NewID()
	for _, user := range users {
		if !user.Active || (input.Actor.OrganizationID != "" && user.OrganizationID != input.Actor.OrganizationID) {
			continue
		}
		if err := u.send(ctx, notificationMessage{RecipientID: user.ID, Type: "announcement", Subject: input.Subject, Content: input.Content, ResourceID: announcementID, ResourceType: "announcement"}); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func (u *NotificationUseCase) ListAnnouncements(ctx context.Context, input ListNotificationsInput) (ListNotificationsOutput, error) {
	if err := requirePermission(input.Actor, "SYSTEM_NOTICE:READ"); err != nil {
		return ListNotificationsOutput{}, err
	}
	if err := requireOrganization(input.Actor); err != nil {
		return ListNotificationsOutput{}, err
	}
	if input.Page < 1 {
		input.Page = 1
	}
	if input.Size < 1 || input.Size > 100 {
		input.Size = 20
	}
	items, total, err := u.repo.ListAnnouncements(ctx, input.Actor.OrganizationID, input.Page, input.Size)
	if err != nil {
		return ListNotificationsOutput{}, err
	}
	result := make([]NotificationOutput, len(items))
	for index, item := range items {
		result[index] = notificationOutput(item)
	}
	return ListNotificationsOutput{Items: result, Total: total}, nil
}

func (u *NotificationUseCase) UpdateAnnouncement(ctx context.Context, input AnnouncementInput) error {
	if err := requirePermission(input.Actor, "SYSTEM_NOTICE:UPDATE"); err != nil {
		return err
	}
	if err := requireOrganization(input.Actor); err != nil {
		return err
	}
	if input.ID == "" || strings.TrimSpace(input.Subject) == "" || strings.TrimSpace(input.Content) == "" || len([]rune(input.Subject)) > 100 || len([]rune(input.Content)) > 5000 {
		return ErrValidation
	}
	updated, err := u.repo.UpdateAnnouncement(ctx, input.Actor.OrganizationID, input.ID, strings.TrimSpace(input.Subject), strings.TrimSpace(input.Content))
	if err != nil {
		return err
	}
	if !updated {
		return ErrNotFound
	}
	return nil
}

func (u *NotificationUseCase) DeleteAnnouncement(ctx context.Context, input AnnouncementInput) error {
	if err := requirePermission(input.Actor, "SYSTEM_NOTICE:DELETE"); err != nil {
		return err
	}
	if err := requireOrganization(input.Actor); err != nil {
		return err
	}
	if input.ID == "" {
		return ErrValidation
	}
	deleted, err := u.repo.DeleteAnnouncement(ctx, input.Actor.OrganizationID, input.ID)
	if err != nil {
		return err
	}
	if !deleted {
		return ErrNotFound
	}
	return nil
}

func (u *NotificationUseCase) deliver(ctx context.Context, item entity.Notification) {
	if u.recipients == nil || len(u.senders) == 0 {
		return
	}
	recipient, err := u.recipients.FindUserByID(ctx, item.UserID)
	if err != nil || recipient.Username == "" {
		return
	}
	for _, sender := range u.senders {
		deliveryContext, cancel := context.WithTimeout(ctx, 5*time.Second)
		_ = sender.Send(deliveryContext, recipient.Username, item.Subject, item.Content)
		cancel()
	}
}
func (u *NotificationUseCase) List(ctx context.Context, input ListNotificationsInput) (ListNotificationsOutput, error) {
	if input.Page < 1 {
		input.Page = 1
	}
	if input.Size < 1 || input.Size > 100 {
		input.Size = 20
	}
	status := entity.NotificationStatus(input.Status)
	if input.Status != "" && status != entity.NotificationUnread && status != entity.NotificationRead {
		return ListNotificationsOutput{}, ErrValidation
	}
	items, total, err := u.repo.ListNotifications(ctx, input.Actor.ID, status, input.Page, input.Size)
	if err != nil {
		return ListNotificationsOutput{}, err
	}
	output := make([]NotificationOutput, len(items))
	for index, item := range items {
		output[index] = notificationOutput(item)
	}
	return ListNotificationsOutput{Items: output, Total: total}, nil
}
func (u *NotificationUseCase) Latest(ctx context.Context, actor UserOutput) ([]NotificationOutput, error) {
	output, err := u.List(ctx, ListNotificationsInput{Actor: actor, Status: string(entity.NotificationUnread), Page: 1, Size: 5})
	return output.Items, err
}
func (u *NotificationUseCase) Count(ctx context.Context, actor UserOutput) (NotificationCountOutput, error) {
	_, all, err := u.repo.ListNotifications(ctx, actor.ID, "", 1, 1)
	if err != nil {
		return NotificationCountOutput{}, err
	}
	_, unread, err := u.repo.ListNotifications(ctx, actor.ID, entity.NotificationUnread, 1, 1)
	if err != nil {
		return NotificationCountOutput{}, err
	}
	return NotificationCountOutput{All: all, Unread: unread, Read: all - unread}, nil
}
func (u *NotificationUseCase) Read(ctx context.Context, input ReadNotificationInput) error {
	found, err := u.repo.MarkNotificationRead(ctx, input.ID, input.Actor.ID)
	if err != nil {
		return err
	}
	if !found {
		return ErrNotFound
	}
	return nil
}
func (u *NotificationUseCase) ReadAll(ctx context.Context, actor UserOutput) (int, error) {
	return u.repo.MarkAllNotificationsRead(ctx, actor.ID)
}
