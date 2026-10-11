package usecase

import (
	"context"
	"time"

	"cordialcrm/backend/internal/entity"
)

type Repository interface {
	WithinTransaction(context.Context, func(context.Context) error) error
	CountUsers(context.Context) (int, error)
	CreateUser(context.Context, entity.User) error
	FindUserByID(context.Context, string) (entity.User, error)
	FindUserByUsername(context.Context, string) (entity.User, error)
	ListUsers(context.Context) ([]entity.User, error)
	UpdateUser(context.Context, entity.User) error
	FindRoleByID(context.Context, string) (entity.SystemRole, error)
	ListDepartments(context.Context, string) ([]entity.Department, error)
	CreateSession(context.Context, string, string, time.Time) error
	FindSessionUser(context.Context, string, time.Time) (entity.User, error)
	DeleteSession(context.Context, string) error
	DeleteUserSessions(context.Context, string) error
	RegisterLoginFailure(context.Context, string, time.Time) (bool, error)
	ClearLoginFailures(context.Context, string) error
	CreateRecord(context.Context, entity.Record) error
	FindRecord(context.Context, string, string) (entity.Record, error)
	ListRecords(context.Context, string) ([]entity.Record, error)
	UpdateRecord(context.Context, entity.Record, int) error
	DeleteRecord(context.Context, string, string, int) error
	MoveRecordActivities(context.Context, string, string, string) error
	CreateFollowup(context.Context, entity.Followup) error
	FindFollowup(context.Context, string) (entity.Followup, error)
	ListFollowups(context.Context, string) ([]entity.Followup, error)
	CreateFollowupComment(context.Context, entity.FollowupComment) error
	FindFollowupComment(context.Context, string) (entity.FollowupComment, error)
	ListFollowupComments(context.Context, string) ([]entity.FollowupComment, error)
	UpdateFollowupComment(context.Context, entity.FollowupComment) error
	DeleteFollowupComment(context.Context, string) error
	CreateApproval(context.Context, entity.Approval) error
	SubmitApproval(context.Context, entity.Approval, entity.Record, int) error
	ApplyApprovalDecision(context.Context, entity.Approval, entity.Approval, *entity.Record, entity.ApprovalDecision) (bool, error)
	FindApproval(context.Context, string) (entity.Approval, error)
	ListApprovals(context.Context) ([]entity.Approval, error)
	UpdateApproval(context.Context, entity.Approval) error
	CreateApprovalDecision(context.Context, entity.ApprovalDecision) error
	ListApprovalDecisions(context.Context, string) ([]entity.ApprovalDecision, error)
	ListApprovalFlows(context.Context, string) ([]entity.ApprovalFlow, error)
	FindCurrentApprovalFlowVersion(context.Context, string) (entity.ApprovalFlowVersion, error)
	FindApprovalFlowVersion(context.Context, string, int) (entity.ApprovalFlowVersion, error)
	CreateAudit(context.Context, entity.Audit) error
	ListAudit(context.Context, int, int) ([]entity.Audit, int, error)
	CreateAttachment(context.Context, entity.Attachment) error
	FindAttachment(context.Context, string) (entity.Attachment, error)
	ListAttachments(context.Context, string) ([]entity.Attachment, error)
	DeleteAttachment(context.Context, string) error
	CreateNotification(context.Context, entity.Notification) error
	ListNotifications(context.Context, string, entity.NotificationStatus, int, int) ([]entity.Notification, int, error)
	MarkNotificationRead(context.Context, string, string) (bool, error)
	MarkAllNotificationsRead(context.Context, string) (int, error)
	ListCustomFields(context.Context, string) ([]entity.CustomField, error)
	CreateCustomField(context.Context, entity.CustomField) error
	UpdateCustomField(context.Context, entity.CustomField) error
	DeleteCustomField(context.Context, string, string) error
	ListStandardFieldSettings(context.Context, string) ([]entity.StandardFieldSetting, error)
	SaveStandardFieldSetting(context.Context, entity.StandardFieldSetting) error
}

type ModuleCatalogRepository interface {
	ListModules(context.Context) ([]entity.ModuleDefinition, error)
}

type APIKeyRepository interface {
	CreateAPIKey(context.Context, entity.APIKey) error
	ListAPIKeys(context.Context, string) ([]entity.APIKey, error)
	FindAPIKeyByPrefix(context.Context, string) (entity.APIKey, error)
	UpdateAPIKey(context.Context, entity.APIKey) error
	DeleteAPIKey(context.Context, string, string) error
	FindUserByID(context.Context, string) (entity.User, error)
}

type PasswordHasher interface {
	Hash(string) (string, error)
	Compare(string, string) error
}

type TokenGenerator interface{ New() (string, error) }

type AttachmentStorage interface {
	Save(context.Context, string, []byte) error
	Read(context.Context, string) ([]byte, error)
	Delete(context.Context, string) error
}

type NotificationRepository interface {
	CreateNotification(context.Context, entity.Notification) error
	ListNotifications(context.Context, string, entity.NotificationStatus, int, int) ([]entity.Notification, int, error)
	ListAnnouncements(context.Context, string, int, int) ([]entity.Notification, int, error)
	UpdateAnnouncement(context.Context, string, string, string, string) (bool, error)
	DeleteAnnouncement(context.Context, string, string) (bool, error)
	MarkNotificationRead(context.Context, string, string) (bool, error)
	MarkAllNotificationsRead(context.Context, string) (int, error)
}

type FollowupPlanRepository interface {
	FindRecord(context.Context, string, string) (entity.Record, error)
	FindUserByID(context.Context, string) (entity.User, error)
	CreateFollowupPlan(context.Context, entity.FollowupPlan) error
	FindFollowupPlan(context.Context, string) (entity.FollowupPlan, error)
	ListFollowupPlans(context.Context, string, string, entity.FollowupPlanStatus, string, int, int) ([]entity.FollowupPlan, int, error)
	UpdateFollowupPlan(context.Context, entity.FollowupPlan) error
	DeleteFollowupPlan(context.Context, string) error
	CreateFollowupPlanComment(context.Context, entity.FollowupPlanComment) error
	FindFollowupPlanComment(context.Context, string) (entity.FollowupPlanComment, error)
	ListFollowupPlanComments(context.Context, string) ([]entity.FollowupPlanComment, error)
	UpdateFollowupPlanComment(context.Context, entity.FollowupPlanComment) error
	DeleteFollowupPlanComment(context.Context, string) error
	CreateFollowup(context.Context, entity.Followup) error
}

type FollowupReminderRepository interface {
	ListFollowupPlansDueBy(context.Context, time.Time, time.Time) ([]entity.FollowupPlan, error)
	DeliverFollowupPlanReminder(context.Context, entity.FollowupPlanReminder, entity.Notification) (bool, error)
}

type OwnershipHistoryRepository interface {
	CreateOwnershipHistory(context.Context, entity.OwnershipHistory) error
	ListOwnershipHistory(context.Context, string, string) ([]entity.OwnershipHistory, error)
}

type CustomerRecycleRepository interface {
	ListIdleOwnedCustomers(context.Context, time.Time) ([]entity.Record, error)
	ListUsers(context.Context) ([]entity.User, error)
}

type ApprovalFlowRepository interface {
	WithinTransaction(context.Context, func(context.Context) error) error
	ListApprovalFlows(context.Context, string) ([]entity.ApprovalFlow, error)
	SaveApprovalFlow(context.Context, entity.ApprovalFlow) error
	SaveApprovalFlowVersion(context.Context, entity.ApprovalFlowVersion) error
	FindCurrentApprovalFlowVersion(context.Context, string) (entity.ApprovalFlowVersion, error)
	FindApprovalFlowVersion(context.Context, string, int) (entity.ApprovalFlowVersion, error)
	ListUsers(context.Context) ([]entity.User, error)
	ListCustomFields(context.Context, string) ([]entity.CustomField, error)
	ListStandardFieldSettings(context.Context, string) ([]entity.StandardFieldSetting, error)
}
