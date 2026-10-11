package entity

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

type Role string

const (
	RoleAdmin   Role = "admin"
	RoleManager Role = "manager"
	RoleSales   Role = "sales"
)

type ApprovalStatus string

const (
	ApprovalDraft     ApprovalStatus = "draft"
	ApprovalPending   ApprovalStatus = "pending"
	ApprovalApproved  ApprovalStatus = "approved"
	ApprovalRejected  ApprovalStatus = "rejected"
	ApprovalWithdrawn ApprovalStatus = "withdrawn"
)

type NotificationStatus string

const (
	NotificationUnread NotificationStatus = "unread"
	NotificationRead   NotificationStatus = "read"
)

type FollowupPlanStatus string

const (
	FollowupPlanPrepared  FollowupPlanStatus = "prepared"
	FollowupPlanUnderway  FollowupPlanStatus = "underway"
	FollowupPlanCompleted FollowupPlanStatus = "completed"
	FollowupPlanCancelled FollowupPlanStatus = "cancelled"
)

type User struct {
	ID, Username, Name, PasswordHash, Department string
	Role                                         Role
	OrganizationID, RoleID                       string
	PermissionIDs                                []string
	DataScope                                    string
	DepartmentIDs                                []string
	FieldRestrictions                            map[string]FieldRestriction
	Active                                       bool
	AuthVersion                                  int
	CreatedAt                                    time.Time
}

type Record struct {
	ID, Module, OrganizationID string
	Data                       map[string]any
	OwnerID                    *string
	OwnerDepartment            string
	Version                    int
	CreatedAt                  time.Time
	UpdatedAt                  time.Time
}

type Followup struct {
	ID, RecordID, UserID, Content, Method string
	NextDate                              *time.Time
	CreatedAt                             time.Time
}

type FollowupComment struct {
	ID, FollowupID, UserID, Content string
	CreatedAt, UpdatedAt            time.Time
}

type FollowupPlan struct {
	ID, Module, RecordID, OwnerID, Content, Method string
	EstimatedAt                                    time.Time
	Status                                         FollowupPlanStatus
	Converted                                      bool
	CreatedAt, UpdatedAt                           time.Time
}

type FollowupPlanComment struct {
	ID, PlanID, UserID, Content string
	CreatedAt, UpdatedAt        time.Time
}

type FollowupPlanReminder struct {
	ID, PlanID, UserID string
	RemindAt           time.Time
}

type OwnershipHistory struct {
	ID, Module, RecordID, PreviousOwnerID, OwnerID, OperatorID, Action string
	CreatedAt                                                          time.Time
}

type Approval struct {
	ID, RecordID, RequestedBy, Comment              string
	FlowID                                          string
	FlowVersion, CurrentStep                        int
	Status                                          ApprovalStatus
	ReviewedBy, DelegatedToID, AdditionalApproverID *string
	CreatedAt                                       time.Time
	ReviewedAt                                      *time.Time
}

type ApprovalDecision struct {
	ID, ApprovalID, ReviewerID, Decision, Comment string
	Step                                          int
	CreatedAt                                     time.Time
}

type Audit struct {
	ID, UserID, Action, Module, Detail string
	RecordID                           *string
	CreatedAt                          time.Time
}

type Attachment struct {
	ID, RecordID, Module, UserID, Name, ContentType string
	GroupID                                         string
	Version                                         int
	Size                                            int64
	CreatedAt                                       time.Time
}

type Notification struct {
	ID, UserID, Type, Subject, Content, ResourceID, ResourceType string
	Status                                                       NotificationStatus
	CreatedAt                                                    time.Time
}

type CustomField struct {
	Module, Key string
	Definition  map[string]any
}

type StandardFieldSetting struct {
	Module, Key, Label string
	Archived           bool
}

type APIKey struct {
	ID, UserID, Name, Prefix, SecretHash string
	Active                               bool
	CreatedAt                            time.Time
	ExpiresAt, LastUsedAt                *time.Time
}

type BusinessError struct {
	Code    string
	Message string
}

func (e *BusinessError) Error() string { return e.Message }

func NewBusinessError(code, message string) error {
	return &BusinessError{Code: code, Message: message}
}

func NewID() string {
	bytes := make([]byte, 12)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}
	return hex.EncodeToString(bytes)
}
