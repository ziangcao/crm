package repo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"cordialcrm/backend/internal/entity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type GormRepository struct{ db *gorm.DB }

func New(db *gorm.DB) *GormRepository { return &GormRepository{db: db} }

type userModel struct {
	ID                             string `gorm:"primaryKey;size:24"`
	Username                       string `gorm:"uniqueIndex;size:80"`
	Name, PasswordHash, Department string
	Role                           string
	OrganizationID, RoleID         string
	Active                         bool
	AuthVersion                    int
	CreatedAt                      time.Time
}
type recordModel struct {
	ID                   string  `gorm:"primaryKey;size:24"`
	Module               string  `gorm:"index"`
	OrganizationID       string  `gorm:"index"`
	OwnerDepartment      string  `gorm:"index"`
	Data                 string  `gorm:"type:text"`
	OwnerID              *string `gorm:"index"`
	Version              int
	CreatedAt, UpdatedAt time.Time
}
type sessionModel struct {
	Token     string `gorm:"primaryKey;size:64"`
	UserID    string `gorm:"index"`
	ExpiresAt time.Time
}
type loginFailureModel struct {
	Username     string `gorm:"primaryKey;size:80"`
	Failures     int
	BlockedUntil time.Time
}
type followupModel struct {
	ID                                string `gorm:"primaryKey;size:24"`
	RecordID, UserID, Content, Method string
	NextDate                          *time.Time
	CreatedAt                         time.Time
}
type followupCommentModel struct {
	ID, UserID, Content  string
	FollowupID           string `gorm:"index"`
	CreatedAt, UpdatedAt time.Time
}
type followupPlanModel struct {
	ID, Module, RecordID, OwnerID, Content, Method string
	EstimatedAt                                    time.Time
	Status                                         string
	Converted                                      bool
	CreatedAt, UpdatedAt                           time.Time
}
type followupPlanCommentModel struct {
	ID, UserID, Content  string
	PlanID               string `gorm:"index"`
	CreatedAt, UpdatedAt time.Time
}
type followupPlanReminderModel struct {
	ID       string `gorm:"primaryKey;size:24"`
	PlanID   string `gorm:"uniqueIndex:idx_followup_plan_reminder_recipient"`
	UserID   string `gorm:"uniqueIndex:idx_followup_plan_reminder_recipient"`
	RemindAt time.Time
}
type ownershipHistoryModel struct {
	ID, Module, RecordID, PreviousOwnerID, OwnerID, OperatorID, Action string
	CreatedAt                                                          time.Time
}
type approvalModel struct {
	ID                                              string `gorm:"primaryKey;size:24"`
	RecordID, RequestedBy, Status, Comment          string
	FlowID                                          string `gorm:"index"`
	FlowVersion, CurrentStep                        int
	ReviewedBy, DelegatedToID, AdditionalApproverID *string
	CreatedAt                                       time.Time
	ReviewedAt                                      *time.Time
}
type approvalDecisionModel struct {
	ID                            string `gorm:"primaryKey;size:24"`
	ApprovalID                    string `gorm:"index"`
	ReviewerID, Decision, Comment string
	Step                          int
	CreatedAt                     time.Time
}
type approvalFlowModel struct {
	ID           string `gorm:"primaryKey;size:24"`
	Module, Name string `gorm:"index"`
	Enabled      bool
	Version      int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
type approvalFlowVersionModel struct {
	ID        string `gorm:"primaryKey;size:24"`
	FlowID    string `gorm:"index"`
	Steps     string `gorm:"type:text"`
	Version   int
	CreatedAt time.Time
}
type auditModel struct {
	ID                             string `gorm:"primaryKey;size:24"`
	UserID, Action, Module, Detail string
	RecordID                       *string
	CreatedAt                      time.Time
}
type attachmentModel struct {
	ID, RecordID, Module, UserID, Name, ContentType string
	GroupID                                         string `gorm:"index"`
	Version                                         int
	Size                                            int64
	CreatedAt                                       time.Time
}
type notificationModel struct {
	ID, UserID, Type, Subject, Content, ResourceID, ResourceType, Status string
	CreatedAt                                                            time.Time
}
type customFieldModel struct {
	Module     string `gorm:"primaryKey"`
	Key        string `gorm:"primaryKey"`
	Definition string `gorm:"type:text"`
}
type standardFieldSettingModel struct {
	Module   string `gorm:"primaryKey"`
	Key      string `gorm:"primaryKey"`
	Label    string
	Archived bool
}
type organizationModel struct {
	ID        string `gorm:"primaryKey;size:24"`
	Name      string
	CreatedAt time.Time
}
type departmentModel struct {
	ID                             string `gorm:"primaryKey;size:24"`
	OrganizationID, ParentID, Name string
	Sort                           int
}
type roleModel struct {
	ID                                             string `gorm:"primaryKey;size:24"`
	OrganizationID, Name, PermissionIDs, DataScope string
	DepartmentIDs, FieldRestrictions               string
	BuiltIn                                        bool
}
type moduleModel struct {
	ID                                                                           string `gorm:"primaryKey;size:24"`
	Key                                                                          string `gorm:"uniqueIndex"`
	Name, Icon, Layout, DocumentTemplate, GroupName, Description                 string
	SalesStages, Stages, CirculationType, StageTransitions                       string `gorm:"type:text"`
	Enabled, Initialized, AfootRollback, EndRollback, Shared, Approvable, Custom bool
}
type apiKeyModel struct {
	ID, UserID, Name, Prefix, SecretHash string
	Active                               bool
	CreatedAt                            time.Time
	ExpiresAt, LastUsedAt                *time.Time
}
type agentModel struct {
	ID, OrganizationID, Name, Prompt, ToolNames string
	Enabled                                     bool
	CreatedAt, UpdatedAt                        time.Time
}
type dashboardModel struct {
	ID, OrganizationID, Name, Widgets string
	IsDefault                         bool
	CreatedAt, UpdatedAt              time.Time
}
type integrationSettingModel struct {
	Key                      string `gorm:"primaryKey"`
	Endpoint, EncryptedToken string
	UpdatedAt                time.Time
}
type fieldModel struct {
	ID                                 string `gorm:"primaryKey;size:24"`
	ModuleID, Key, Name, Type, Options string
	Required, Unique, ReadOnly         bool
	Sort                               int
}

func (userModel) TableName() string                 { return "users" }
func (recordModel) TableName() string               { return "records" }
func (sessionModel) TableName() string              { return "sessions" }
func (loginFailureModel) TableName() string         { return "login_attempts" }
func (followupModel) TableName() string             { return "followups" }
func (followupCommentModel) TableName() string      { return "followup_comments" }
func (followupPlanModel) TableName() string         { return "followup_plans" }
func (followupPlanCommentModel) TableName() string  { return "followup_plan_comments" }
func (approvalModel) TableName() string             { return "approvals" }
func (approvalDecisionModel) TableName() string     { return "approval_decisions" }
func (approvalFlowModel) TableName() string         { return "approval_flows" }
func (approvalFlowVersionModel) TableName() string  { return "approval_flow_versions" }
func (auditModel) TableName() string                { return "audit" }
func (attachmentModel) TableName() string           { return "attachments" }
func (notificationModel) TableName() string         { return "notifications" }
func (followupPlanReminderModel) TableName() string { return "followup_plan_reminders" }
func (ownershipHistoryModel) TableName() string     { return "ownership_histories" }
func (customFieldModel) TableName() string          { return "custom_fields" }
func (standardFieldSettingModel) TableName() string { return "standard_field_settings" }
func (organizationModel) TableName() string         { return "organizations" }
func (departmentModel) TableName() string           { return "departments" }
func (roleModel) TableName() string                 { return "roles" }
func (moduleModel) TableName() string               { return "modules" }
func (fieldModel) TableName() string                { return "module_fields" }
func (apiKeyModel) TableName() string               { return "api_keys" }
func (agentModel) TableName() string                { return "agents" }
func (dashboardModel) TableName() string            { return "dashboards" }
func (integrationSettingModel) TableName() string   { return "integration_settings" }
func (r *GormRepository) Migrate(ctx context.Context) error {
	return r.db.WithContext(ctx).AutoMigrate(&userModel{}, &recordModel{}, &sessionModel{}, &loginFailureModel{}, &followupModel{}, &followupCommentModel{}, &followupPlanModel{}, &followupPlanCommentModel{}, &followupPlanReminderModel{}, &ownershipHistoryModel{}, &approvalModel{}, &approvalDecisionModel{}, &approvalFlowModel{}, &approvalFlowVersionModel{}, &auditModel{}, &attachmentModel{}, &notificationModel{}, &customFieldModel{}, &standardFieldSettingModel{}, &organizationModel{}, &departmentModel{}, &roleModel{}, &moduleModel{}, &fieldModel{}, &apiKeyModel{}, &agentModel{}, &dashboardModel{}, &integrationSettingModel{})
}

func (r *GormRepository) BootstrapModules(ctx context.Context, definitions []entity.ModuleDefinition) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var models []moduleModel
		if err := tx.Find(&models).Error; err != nil {
			return err
		}
		existing := make(map[string]moduleModel, len(models))
		for _, model := range models {
			existing[model.Key] = model
		}
		for _, definition := range definitions {
			model, ok := existing[definition.Key]
			if !ok {
				if err := tx.Create(&moduleModel{ID: entity.NewID(), Key: definition.Key, Name: definition.Name, Icon: definition.Icon, Enabled: true, Initialized: true}).Error; err != nil {
					return err
				}
				continue
			}
			if !model.Initialized {
				if err := tx.Model(&moduleModel{}).Where("id = ?", model.ID).Updates(map[string]any{"name": definition.Name, "icon": definition.Icon, "enabled": true, "initialized": true}).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}
func (r *GormRepository) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		original := r.db
		r.db = tx
		defer func() { r.db = original }()
		return fn(ctx)
	})
}

func (r *GormRepository) MoveRecordActivities(ctx context.Context, module, sourceID, targetID string) error {
	for _, update := range []struct {
		model any
		where string
		args  []any
	}{
		{model: &followupModel{}, where: "record_id = ?", args: []any{sourceID}},
		{model: &followupPlanModel{}, where: "module = ? AND record_id = ?", args: []any{module, sourceID}},
		{model: &attachmentModel{}, where: "module = ? AND record_id = ?", args: []any{module, sourceID}},
	} {
		if err := r.db.WithContext(ctx).Model(update.model).Where(update.where, update.args...).Update("record_id", targetID).Error; err != nil {
			return err
		}
	}
	return nil
}
func toUser(m userModel) entity.User {
	return entity.User{ID: m.ID, Username: m.Username, Name: m.Name, PasswordHash: m.PasswordHash, Department: m.Department, Role: entity.Role(m.Role), OrganizationID: m.OrganizationID, RoleID: m.RoleID, Active: m.Active, AuthVersion: m.AuthVersion, CreatedAt: m.CreatedAt}
}
func fromUser(e entity.User) userModel {
	return userModel{ID: e.ID, Username: e.Username, Name: e.Name, PasswordHash: e.PasswordHash, Department: e.Department, Role: string(e.Role), OrganizationID: e.OrganizationID, RoleID: e.RoleID, Active: e.Active, AuthVersion: e.AuthVersion, CreatedAt: e.CreatedAt}
}
func (r *GormRepository) userWithPermissions(ctx context.Context, model userModel) (entity.User, error) {
	user := toUser(model)
	if user.RoleID == "" {
		return user, nil
	}
	role, err := r.FindRoleByID(ctx, user.RoleID)
	if err != nil {
		return entity.User{}, err
	}
	if role.OrganizationID != user.OrganizationID {
		return entity.User{}, errors.New("invalid user role assignment")
	}
	user.PermissionIDs = role.PermissionIDs
	user.DataScope = role.DataScope
	user.DepartmentIDs = role.DepartmentIDs
	user.FieldRestrictions = role.FieldRestrictions
	if role.DataScope == "department_and_children" && user.Department != "" {
		var departments []departmentModel
		if err := r.db.WithContext(ctx).Where("organization_id = ?", user.OrganizationID).Find(&departments).Error; err != nil {
			return entity.User{}, err
		}
		children := map[string][]string{}
		for _, department := range departments {
			children[department.ParentID] = append(children[department.ParentID], department.ID)
		}
		user.DepartmentIDs = []string{user.Department}
		for index := 0; index < len(user.DepartmentIDs); index++ {
			user.DepartmentIDs = append(user.DepartmentIDs, children[user.DepartmentIDs[index]]...)
		}
	}
	return user, nil
}
func (r *GormRepository) CountUsers(ctx context.Context) (int, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&userModel{}).Count(&n).Error
	return int(n), err
}
func (r *GormRepository) CreateUser(ctx context.Context, e entity.User) error {
	model := fromUser(e)
	return r.db.WithContext(ctx).Create(&model).Error
}
func (r *GormRepository) FindUserByID(ctx context.Context, id string) (entity.User, error) {
	var m userModel
	err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error
	if err != nil {
		return entity.User{}, err
	}
	return r.userWithPermissions(ctx, m)
}
func (r *GormRepository) FindUserByUsername(ctx context.Context, name string) (entity.User, error) {
	var m userModel
	err := r.db.WithContext(ctx).First(&m, "username = ?", name).Error
	if err != nil {
		return entity.User{}, err
	}
	return r.userWithPermissions(ctx, m)
}
func (r *GormRepository) ListUsers(ctx context.Context) ([]entity.User, error) {
	var models []userModel
	err := r.db.WithContext(ctx).Order("created_at").Find(&models).Error
	out := make([]entity.User, len(models))
	for i, m := range models {
		out[i], err = r.userWithPermissions(ctx, m)
		if err != nil {
			return nil, err
		}
	}
	return out, err
}
func (r *GormRepository) UpdateUser(ctx context.Context, e entity.User) error {
	model := fromUser(e)
	return r.db.WithContext(ctx).Save(&model).Error
}
func (r *GormRepository) CreateSession(ctx context.Context, token, userID string, expires time.Time) error {
	return r.db.WithContext(ctx).Create(&sessionModel{Token: token, UserID: userID, ExpiresAt: expires}).Error
}
func (r *GormRepository) FindSessionUser(ctx context.Context, token string, now time.Time) (entity.User, error) {
	var session sessionModel
	if err := r.db.WithContext(ctx).First(&session, "token = ? AND expires_at > ?", token, now).Error; err != nil {
		return entity.User{}, err
	}
	return r.FindUserByID(ctx, session.UserID)
}
func (r *GormRepository) DeleteSession(ctx context.Context, token string) error {
	return r.db.WithContext(ctx).Delete(&sessionModel{}, "token = ?", token).Error
}
func (r *GormRepository) DeleteUserSessions(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&sessionModel{}, "user_id = ?", id).Error
}
func (r *GormRepository) RegisterLoginFailure(ctx context.Context, username string, now time.Time) (bool, error) {
	var m loginFailureModel
	err := r.db.WithContext(ctx).First(&m, "username = ?", username).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, err
	}
	if err == nil && m.BlockedUntil.After(now) {
		return true, nil
	}
	if err != nil || !m.BlockedUntil.IsZero() {
		m = loginFailureModel{Username: username}
	}
	m.Failures++
	if m.Failures >= 5 {
		m.BlockedUntil = now.Add(15 * time.Minute)
	}
	if err := r.db.WithContext(ctx).Save(&m).Error; err != nil {
		return false, err
	}
	return m.BlockedUntil.After(now), nil
}
func (r *GormRepository) ClearLoginFailures(ctx context.Context, name string) error {
	return r.db.WithContext(ctx).Delete(&loginFailureModel{}, "username = ?", name).Error
}
func (r *GormRepository) ListApprovalFlows(ctx context.Context, module string) ([]entity.ApprovalFlow, error) {
	var models []approvalFlowModel
	db := r.db.WithContext(ctx).Order("module, version DESC")
	if module != "" {
		db = db.Where("module = ?", module)
	}
	err := db.Find(&models).Error
	items := make([]entity.ApprovalFlow, len(models))
	for index, model := range models {
		items[index] = entity.ApprovalFlow{ID: model.ID, Module: model.Module, Name: model.Name, Enabled: model.Enabled, Version: model.Version, CreatedAt: model.CreatedAt, UpdatedAt: model.UpdatedAt}
	}
	return items, err
}
func (r *GormRepository) SaveApprovalFlow(ctx context.Context, flow entity.ApprovalFlow) error {
	return r.db.WithContext(ctx).Save(&approvalFlowModel{ID: flow.ID, Module: flow.Module, Name: flow.Name, Enabled: flow.Enabled, Version: flow.Version, CreatedAt: flow.CreatedAt, UpdatedAt: flow.UpdatedAt}).Error
}
func (r *GormRepository) SaveApprovalFlowVersion(ctx context.Context, version entity.ApprovalFlowVersion) error {
	steps, err := json.Marshal(version.Steps)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Save(&approvalFlowVersionModel{ID: version.ID, FlowID: version.FlowID, Version: version.Version, Steps: string(steps), CreatedAt: version.CreatedAt}).Error
}
func (r *GormRepository) FindCurrentApprovalFlowVersion(ctx context.Context, flowID string) (entity.ApprovalFlowVersion, error) {
	var model approvalFlowVersionModel
	err := r.db.WithContext(ctx).Where("flow_id = ?", flowID).Order("version DESC").First(&model).Error
	return toApprovalFlowVersion(model), err
}
func (r *GormRepository) FindApprovalFlowVersion(ctx context.Context, flowID string, version int) (entity.ApprovalFlowVersion, error) {
	var model approvalFlowVersionModel
	err := r.db.WithContext(ctx).Where("flow_id = ? AND version = ?", flowID, version).First(&model).Error
	return toApprovalFlowVersion(model), err
}
func toApprovalFlowVersion(model approvalFlowVersionModel) entity.ApprovalFlowVersion {
	var steps []entity.ApprovalFlowStep
	_ = json.Unmarshal([]byte(model.Steps), &steps)
	return entity.ApprovalFlowVersion{ID: model.ID, FlowID: model.FlowID, Version: model.Version, Steps: steps, CreatedAt: model.CreatedAt}
}
func toRecord(m recordModel) entity.Record {
	var data map[string]any
	_ = json.Unmarshal([]byte(m.Data), &data)
	if data == nil {
		data = map[string]any{}
	}
	return entity.Record{ID: m.ID, Module: m.Module, OrganizationID: m.OrganizationID, Data: data, OwnerID: m.OwnerID, OwnerDepartment: m.OwnerDepartment, Version: m.Version, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt}
}
func fromRecord(e entity.Record) recordModel {
	data := e.Data
	if data == nil {
		data = map[string]any{}
	}
	raw, _ := json.Marshal(data)
	return recordModel{ID: e.ID, Module: e.Module, OrganizationID: e.OrganizationID, OwnerDepartment: e.OwnerDepartment, Data: string(raw), OwnerID: e.OwnerID, Version: e.Version, CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt}
}
func (r *GormRepository) CreateRecord(ctx context.Context, e entity.Record) error {
	model := fromRecord(e)
	return r.db.WithContext(ctx).Create(&model).Error
}
func (r *GormRepository) FindRecord(ctx context.Context, module, id string) (entity.Record, error) {
	var m recordModel
	db := r.db.WithContext(ctx)
	if module != "" {
		db = db.Where("module = ?", module)
	}
	err := db.First(&m, "id = ?", id).Error
	return toRecord(m), err
}
func (r *GormRepository) ListRecords(ctx context.Context, module string) ([]entity.Record, error) {
	var models []recordModel
	err := r.db.WithContext(ctx).Where("module = ?", module).Order("updated_at DESC, id").Find(&models).Error
	out := make([]entity.Record, len(models))
	for i, m := range models {
		out[i] = toRecord(m)
	}
	return out, err
}
func (r *GormRepository) ListIdleOwnedCustomers(ctx context.Context, before time.Time) ([]entity.Record, error) {
	var models []recordModel
	err := r.db.WithContext(ctx).Where("module = ? AND owner_id IS NOT NULL AND updated_at <= ?", "customers", before).Order("updated_at, id").Find(&models).Error
	items := make([]entity.Record, len(models))
	for index, model := range models {
		items[index] = toRecord(model)
	}
	return items, err
}
func (r *GormRepository) UpdateRecord(ctx context.Context, e entity.Record, version int) error {
	m := fromRecord(e)
	result := r.db.WithContext(ctx).Model(&recordModel{}).Where("id = ? AND module = ? AND version = ?", e.ID, e.Module, version).Updates(map[string]any{"data": m.Data, "organization_id": m.OrganizationID, "owner_id": m.OwnerID, "owner_department": m.OwnerDepartment, "version": version + 1, "updated_at": e.UpdatedAt})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
func (r *GormRepository) DeleteRecord(ctx context.Context, module, id string, version int) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Where("id = ? AND module = ? AND version = ?", id, module, version).Delete(&recordModel{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		followups := tx.Model(&followupModel{}).Select("id").Where("record_id = ?", id)
		if err := tx.Where("followup_id IN (?)", followups).Delete(&followupCommentModel{}).Error; err != nil {
			return err
		}
		return tx.Where("record_id = ?", id).Delete(&followupModel{}).Error
	})
}
func (r *GormRepository) CreateFollowup(ctx context.Context, e entity.Followup) error {
	return r.db.WithContext(ctx).Create(&followupModel{ID: e.ID, RecordID: e.RecordID, UserID: e.UserID, Content: e.Content, Method: e.Method, NextDate: e.NextDate, CreatedAt: e.CreatedAt}).Error
}
func (r *GormRepository) FindFollowup(ctx context.Context, id string) (entity.Followup, error) {
	var model followupModel
	err := r.db.WithContext(ctx).First(&model, "id = ?", id).Error
	return entity.Followup{ID: model.ID, RecordID: model.RecordID, UserID: model.UserID, Content: model.Content, Method: model.Method, NextDate: model.NextDate, CreatedAt: model.CreatedAt}, err
}
func (r *GormRepository) ListFollowups(ctx context.Context, recordID string) ([]entity.Followup, error) {
	var models []followupModel
	err := r.db.WithContext(ctx).Where("record_id = ?", recordID).Order("created_at DESC, id").Find(&models).Error
	out := make([]entity.Followup, len(models))
	for i, m := range models {
		out[i] = entity.Followup{ID: m.ID, RecordID: m.RecordID, UserID: m.UserID, Content: m.Content, Method: m.Method, NextDate: m.NextDate, CreatedAt: m.CreatedAt}
	}
	return out, err
}
func (r *GormRepository) CreateFollowupComment(ctx context.Context, item entity.FollowupComment) error {
	return r.db.WithContext(ctx).Create(&followupCommentModel{ID: item.ID, FollowupID: item.FollowupID, UserID: item.UserID, Content: item.Content, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}).Error
}
func (r *GormRepository) FindFollowupComment(ctx context.Context, id string) (entity.FollowupComment, error) {
	var model followupCommentModel
	err := r.db.WithContext(ctx).First(&model, "id = ?", id).Error
	return entity.FollowupComment{ID: model.ID, FollowupID: model.FollowupID, UserID: model.UserID, Content: model.Content, CreatedAt: model.CreatedAt, UpdatedAt: model.UpdatedAt}, err
}
func (r *GormRepository) ListFollowupComments(ctx context.Context, followupID string) ([]entity.FollowupComment, error) {
	var models []followupCommentModel
	err := r.db.WithContext(ctx).Where("followup_id = ?", followupID).Order("created_at, id").Find(&models).Error
	items := make([]entity.FollowupComment, len(models))
	for index, model := range models {
		items[index] = entity.FollowupComment{ID: model.ID, FollowupID: model.FollowupID, UserID: model.UserID, Content: model.Content, CreatedAt: model.CreatedAt, UpdatedAt: model.UpdatedAt}
	}
	return items, err
}
func (r *GormRepository) UpdateFollowupComment(ctx context.Context, item entity.FollowupComment) error {
	return r.db.WithContext(ctx).Model(&followupCommentModel{}).Where("id = ?", item.ID).Updates(map[string]any{"content": item.Content, "updated_at": item.UpdatedAt}).Error
}
func (r *GormRepository) DeleteFollowupComment(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&followupCommentModel{}, "id = ?", id).Error
}
func (r *GormRepository) CreateFollowupPlan(ctx context.Context, item entity.FollowupPlan) error {
	return r.db.WithContext(ctx).Create(&followupPlanModel{ID: item.ID, Module: item.Module, RecordID: item.RecordID, OwnerID: item.OwnerID, Content: item.Content, Method: item.Method, EstimatedAt: item.EstimatedAt, Status: string(item.Status), Converted: item.Converted, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}).Error
}
func (r *GormRepository) FindFollowupPlan(ctx context.Context, id string) (entity.FollowupPlan, error) {
	var model followupPlanModel
	err := r.db.WithContext(ctx).First(&model, "id = ?", id).Error
	return toFollowupPlan(model), err
}
func (r *GormRepository) ListFollowupPlans(ctx context.Context, module, recordID string, status entity.FollowupPlanStatus, ownerID string, page, size int) ([]entity.FollowupPlan, int, error) {
	db := r.db.WithContext(ctx).Model(&followupPlanModel{}).Where("module = ? AND record_id = ?", module, recordID)
	if status != "" {
		db = db.Where("status = ?", status)
	}
	if ownerID != "" {
		db = db.Where("owner_id = ?", ownerID)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var models []followupPlanModel
	err := db.Order("estimated_at, created_at DESC, id").Offset((page - 1) * size).Limit(size).Find(&models).Error
	items := make([]entity.FollowupPlan, len(models))
	for index, model := range models {
		items[index] = toFollowupPlan(model)
	}
	return items, int(total), err
}
func (r *GormRepository) UpdateFollowupPlan(ctx context.Context, item entity.FollowupPlan) error {
	return r.db.WithContext(ctx).Model(&followupPlanModel{}).Where("id = ?", item.ID).Updates(map[string]any{"owner_id": item.OwnerID, "content": item.Content, "method": item.Method, "estimated_at": item.EstimatedAt, "status": item.Status, "converted": item.Converted, "updated_at": item.UpdatedAt}).Error
}
func (r *GormRepository) DeleteFollowupPlan(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("plan_id = ?", id).Delete(&followupPlanCommentModel{}).Error; err != nil {
			return err
		}
		if err := tx.Where("plan_id = ?", id).Delete(&followupPlanReminderModel{}).Error; err != nil {
			return err
		}
		return tx.Delete(&followupPlanModel{}, "id = ?", id).Error
	})
}
func (r *GormRepository) CreateFollowupPlanComment(ctx context.Context, item entity.FollowupPlanComment) error {
	return r.db.WithContext(ctx).Create(&followupPlanCommentModel{ID: item.ID, PlanID: item.PlanID, UserID: item.UserID, Content: item.Content, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}).Error
}
func (r *GormRepository) FindFollowupPlanComment(ctx context.Context, id string) (entity.FollowupPlanComment, error) {
	var model followupPlanCommentModel
	err := r.db.WithContext(ctx).First(&model, "id = ?", id).Error
	return entity.FollowupPlanComment{ID: model.ID, PlanID: model.PlanID, UserID: model.UserID, Content: model.Content, CreatedAt: model.CreatedAt, UpdatedAt: model.UpdatedAt}, err
}
func (r *GormRepository) ListFollowupPlanComments(ctx context.Context, planID string) ([]entity.FollowupPlanComment, error) {
	var models []followupPlanCommentModel
	err := r.db.WithContext(ctx).Where("plan_id = ?", planID).Order("created_at, id").Find(&models).Error
	items := make([]entity.FollowupPlanComment, len(models))
	for index, model := range models {
		items[index] = entity.FollowupPlanComment{ID: model.ID, PlanID: model.PlanID, UserID: model.UserID, Content: model.Content, CreatedAt: model.CreatedAt, UpdatedAt: model.UpdatedAt}
	}
	return items, err
}
func (r *GormRepository) UpdateFollowupPlanComment(ctx context.Context, item entity.FollowupPlanComment) error {
	return r.db.WithContext(ctx).Model(&followupPlanCommentModel{}).Where("id = ?", item.ID).Updates(map[string]any{"content": item.Content, "updated_at": item.UpdatedAt}).Error
}
func (r *GormRepository) DeleteFollowupPlanComment(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&followupPlanCommentModel{}, "id = ?", id).Error
}
func toFollowupPlan(model followupPlanModel) entity.FollowupPlan {
	return entity.FollowupPlan{ID: model.ID, Module: model.Module, RecordID: model.RecordID, OwnerID: model.OwnerID, Content: model.Content, Method: model.Method, EstimatedAt: model.EstimatedAt, Status: entity.FollowupPlanStatus(model.Status), Converted: model.Converted, CreatedAt: model.CreatedAt, UpdatedAt: model.UpdatedAt}
}
func (r *GormRepository) ListFollowupPlansDueBy(ctx context.Context, from, until time.Time) ([]entity.FollowupPlan, error) {
	var models []followupPlanModel
	err := r.db.WithContext(ctx).Where("estimated_at >= ? AND estimated_at <= ? AND status IN ? AND converted = ?", from, until, []entity.FollowupPlanStatus{entity.FollowupPlanPrepared, entity.FollowupPlanUnderway}, false).Order("estimated_at, id").Find(&models).Error
	items := make([]entity.FollowupPlan, len(models))
	for index, model := range models {
		items[index] = toFollowupPlan(model)
	}
	return items, err
}
func (r *GormRepository) DeliverFollowupPlanReminder(ctx context.Context, reminder entity.FollowupPlanReminder, notification entity.Notification) (bool, error) {
	delivered := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "plan_id"}, {Name: "user_id"}}, DoNothing: true}).Create(&followupPlanReminderModel{ID: reminder.ID, PlanID: reminder.PlanID, UserID: reminder.UserID, RemindAt: reminder.RemindAt})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		if err := tx.Create(&notificationModel{ID: notification.ID, UserID: notification.UserID, Type: notification.Type, Subject: notification.Subject, Content: notification.Content, ResourceID: notification.ResourceID, ResourceType: notification.ResourceType, Status: string(notification.Status), CreatedAt: notification.CreatedAt}).Error; err != nil {
			return err
		}
		delivered = true
		return nil
	})
	return delivered, err
}
func (r *GormRepository) CreateOwnershipHistory(ctx context.Context, item entity.OwnershipHistory) error {
	return r.db.WithContext(ctx).Create(&ownershipHistoryModel{ID: item.ID, Module: item.Module, RecordID: item.RecordID, PreviousOwnerID: item.PreviousOwnerID, OwnerID: item.OwnerID, OperatorID: item.OperatorID, Action: item.Action, CreatedAt: item.CreatedAt}).Error
}
func (r *GormRepository) ListOwnershipHistory(ctx context.Context, module, recordID string) ([]entity.OwnershipHistory, error) {
	var models []ownershipHistoryModel
	err := r.db.WithContext(ctx).Where("module = ? AND record_id = ?", module, recordID).Order("created_at DESC, id DESC").Find(&models).Error
	items := make([]entity.OwnershipHistory, len(models))
	for index, model := range models {
		items[index] = entity.OwnershipHistory{ID: model.ID, Module: model.Module, RecordID: model.RecordID, PreviousOwnerID: model.PreviousOwnerID, OwnerID: model.OwnerID, OperatorID: model.OperatorID, Action: model.Action, CreatedAt: model.CreatedAt}
	}
	return items, err
}
func (r *GormRepository) CreateApproval(ctx context.Context, e entity.Approval) error {
	return r.db.WithContext(ctx).Create(&approvalModel{ID: e.ID, RecordID: e.RecordID, RequestedBy: e.RequestedBy, Status: string(e.Status), Comment: e.Comment, FlowID: e.FlowID, FlowVersion: e.FlowVersion, CurrentStep: e.CurrentStep, ReviewedBy: e.ReviewedBy, DelegatedToID: e.DelegatedToID, AdditionalApproverID: e.AdditionalApproverID, CreatedAt: e.CreatedAt, ReviewedAt: e.ReviewedAt}).Error
}
func (r *GormRepository) SubmitApproval(ctx context.Context, approval entity.Approval, record entity.Record, version int) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		model := fromRecord(record)
		result := tx.Model(&recordModel{}).Where("id = ? AND module = ? AND version = ?", record.ID, record.Module, version).Updates(map[string]any{"data": model.Data, "version": version + 1, "updated_at": record.UpdatedAt})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return tx.Create(&approvalModel{ID: approval.ID, RecordID: approval.RecordID, RequestedBy: approval.RequestedBy, Status: string(approval.Status), Comment: approval.Comment, FlowID: approval.FlowID, FlowVersion: approval.FlowVersion, CurrentStep: approval.CurrentStep, DelegatedToID: approval.DelegatedToID, AdditionalApproverID: approval.AdditionalApproverID, CreatedAt: approval.CreatedAt}).Error
	})
}
func (r *GormRepository) FindApproval(ctx context.Context, id string) (entity.Approval, error) {
	var m approvalModel
	err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error
	return entity.Approval{ID: m.ID, RecordID: m.RecordID, RequestedBy: m.RequestedBy, Status: entity.ApprovalStatus(m.Status), Comment: m.Comment, FlowID: m.FlowID, FlowVersion: m.FlowVersion, CurrentStep: m.CurrentStep, ReviewedBy: m.ReviewedBy, DelegatedToID: m.DelegatedToID, AdditionalApproverID: m.AdditionalApproverID, CreatedAt: m.CreatedAt, ReviewedAt: m.ReviewedAt}, err
}
func (r *GormRepository) ListApprovals(ctx context.Context) ([]entity.Approval, error) {
	var models []approvalModel
	err := r.db.WithContext(ctx).Order("created_at DESC").Find(&models).Error
	out := make([]entity.Approval, len(models))
	for i, m := range models {
		out[i] = entity.Approval{ID: m.ID, RecordID: m.RecordID, RequestedBy: m.RequestedBy, Status: entity.ApprovalStatus(m.Status), Comment: m.Comment, FlowID: m.FlowID, FlowVersion: m.FlowVersion, CurrentStep: m.CurrentStep, ReviewedBy: m.ReviewedBy, DelegatedToID: m.DelegatedToID, AdditionalApproverID: m.AdditionalApproverID, CreatedAt: m.CreatedAt, ReviewedAt: m.ReviewedAt}
	}
	return out, err
}
func (r *GormRepository) UpdateApproval(ctx context.Context, e entity.Approval) error {
	return r.db.WithContext(ctx).Model(&approvalModel{}).Where("id = ?", e.ID).Updates(map[string]any{"status": e.Status, "current_step": e.CurrentStep, "reviewed_by": e.ReviewedBy, "delegated_to_id": e.DelegatedToID, "additional_approver_id": e.AdditionalApproverID, "comment": e.Comment, "reviewed_at": e.ReviewedAt}).Error
}
func (r *GormRepository) ApplyApprovalDecision(ctx context.Context, previous, updated entity.Approval, record *entity.Record, decision entity.ApprovalDecision) (bool, error) {
	applied := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if record != nil {
			model := fromRecord(*record)
			result := tx.Model(&recordModel{}).Where("id = ? AND module = ? AND version = ?", record.ID, record.Module, record.Version).Updates(map[string]any{"data": model.Data, "version": record.Version + 1, "updated_at": record.UpdatedAt})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return gorm.ErrRecordNotFound
			}
		}
		query := tx.Model(&approvalModel{}).Where("id = ? AND status = ? AND current_step = ?", previous.ID, previous.Status, previous.CurrentStep)
		if previous.DelegatedToID == nil {
			query = query.Where("delegated_to_id IS NULL")
		} else {
			query = query.Where("delegated_to_id = ?", *previous.DelegatedToID)
		}
		if previous.AdditionalApproverID == nil {
			query = query.Where("additional_approver_id IS NULL")
		} else {
			query = query.Where("additional_approver_id = ?", *previous.AdditionalApproverID)
		}
		result := query.Updates(map[string]any{"status": updated.Status, "current_step": updated.CurrentStep, "reviewed_by": updated.ReviewedBy, "delegated_to_id": updated.DelegatedToID, "additional_approver_id": updated.AdditionalApproverID, "comment": updated.Comment, "reviewed_at": updated.ReviewedAt})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		if err := tx.Create(&approvalDecisionModel{ID: decision.ID, ApprovalID: decision.ApprovalID, ReviewerID: decision.ReviewerID, Decision: decision.Decision, Comment: decision.Comment, Step: decision.Step, CreatedAt: decision.CreatedAt}).Error; err != nil {
			return err
		}
		applied = true
		return nil
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return applied, err
}
func (r *GormRepository) CreateApprovalDecision(ctx context.Context, decision entity.ApprovalDecision) error {
	return r.db.WithContext(ctx).Create(&approvalDecisionModel{ID: decision.ID, ApprovalID: decision.ApprovalID, ReviewerID: decision.ReviewerID, Decision: decision.Decision, Comment: decision.Comment, Step: decision.Step, CreatedAt: decision.CreatedAt}).Error
}
func (r *GormRepository) ListApprovalDecisions(ctx context.Context, approvalID string) ([]entity.ApprovalDecision, error) {
	var models []approvalDecisionModel
	err := r.db.WithContext(ctx).Where("approval_id = ?", approvalID).Order("step, created_at, id").Find(&models).Error
	items := make([]entity.ApprovalDecision, len(models))
	for index, model := range models {
		items[index] = entity.ApprovalDecision{ID: model.ID, ApprovalID: model.ApprovalID, ReviewerID: model.ReviewerID, Decision: model.Decision, Comment: model.Comment, Step: model.Step, CreatedAt: model.CreatedAt}
	}
	return items, err
}
func (r *GormRepository) CreateAudit(ctx context.Context, e entity.Audit) error {
	return r.db.WithContext(ctx).Create(&auditModel{ID: e.ID, UserID: e.UserID, Action: e.Action, Module: e.Module, Detail: e.Detail, RecordID: e.RecordID, CreatedAt: e.CreatedAt}).Error
}
func (r *GormRepository) ListAudit(ctx context.Context, page, size int) ([]entity.Audit, int, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&auditModel{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var models []auditModel
	err := r.db.WithContext(ctx).Order("created_at DESC").Offset((page - 1) * size).Limit(size).Find(&models).Error
	out := make([]entity.Audit, len(models))
	for i, m := range models {
		out[i] = entity.Audit{ID: m.ID, UserID: m.UserID, Action: m.Action, Module: m.Module, Detail: m.Detail, RecordID: m.RecordID, CreatedAt: m.CreatedAt}
	}
	return out, int(total), err
}
func (r *GormRepository) CreateAttachment(ctx context.Context, item entity.Attachment) error {
	return r.db.WithContext(ctx).Create(&attachmentModel{ID: item.ID, RecordID: item.RecordID, Module: item.Module, UserID: item.UserID, Name: item.Name, ContentType: item.ContentType, GroupID: item.GroupID, Version: item.Version, Size: item.Size, CreatedAt: item.CreatedAt}).Error
}
func (r *GormRepository) FindAttachment(ctx context.Context, id string) (entity.Attachment, error) {
	var model attachmentModel
	err := r.db.WithContext(ctx).First(&model, "id = ?", id).Error
	return entity.Attachment{ID: model.ID, RecordID: model.RecordID, Module: model.Module, UserID: model.UserID, Name: model.Name, ContentType: model.ContentType, GroupID: model.GroupID, Version: model.Version, Size: model.Size, CreatedAt: model.CreatedAt}, err
}
func (r *GormRepository) ListAttachments(ctx context.Context, recordID string) ([]entity.Attachment, error) {
	var models []attachmentModel
	err := r.db.WithContext(ctx).Where("record_id = ?", recordID).Order("created_at DESC, id").Find(&models).Error
	items := make([]entity.Attachment, len(models))
	for index, model := range models {
		items[index] = entity.Attachment{ID: model.ID, RecordID: model.RecordID, Module: model.Module, UserID: model.UserID, Name: model.Name, ContentType: model.ContentType, GroupID: model.GroupID, Version: model.Version, Size: model.Size, CreatedAt: model.CreatedAt}
	}
	return items, err
}
func (r *GormRepository) DeleteAttachment(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Delete(&attachmentModel{}, "id = ?", id).Error
}
func (r *GormRepository) CreateNotification(ctx context.Context, item entity.Notification) error {
	return r.db.WithContext(ctx).Create(&notificationModel{ID: item.ID, UserID: item.UserID, Type: item.Type, Subject: item.Subject, Content: item.Content, ResourceID: item.ResourceID, ResourceType: item.ResourceType, Status: string(item.Status), CreatedAt: item.CreatedAt}).Error
}

func (r *GormRepository) ListAnnouncements(ctx context.Context, organizationID string, page, size int) ([]entity.Notification, int, error) {
	var models []notificationModel
	err := r.db.WithContext(ctx).Table("notifications").Select("notifications.*").Joins("JOIN users ON users.id = notifications.user_id").Where("users.organization_id = ? AND notifications.type = ? AND notifications.resource_type = ? AND notifications.resource_id <> ?", organizationID, "announcement", "announcement", "").Order("notifications.created_at DESC, notifications.id DESC").Find(&models).Error
	if err != nil {
		return nil, 0, err
	}
	unique := make([]entity.Notification, 0)
	seen := map[string]bool{}
	for _, model := range models {
		if seen[model.ResourceID] {
			continue
		}
		seen[model.ResourceID] = true
		unique = append(unique, entity.Notification{ID: model.ResourceID, Type: model.Type, Subject: model.Subject, Content: model.Content, ResourceID: model.ResourceID, ResourceType: model.ResourceType, Status: entity.NotificationStatus(model.Status), CreatedAt: model.CreatedAt})
	}
	total := len(unique)
	start := (page - 1) * size
	if start >= total {
		return []entity.Notification{}, total, nil
	}
	end := min(start+size, total)
	return unique[start:end], total, nil
}

func (r *GormRepository) UpdateAnnouncement(ctx context.Context, organizationID, id, subject, content string) (bool, error) {
	users := r.db.WithContext(ctx).Model(&userModel{}).Select("id").Where("organization_id = ?", organizationID)
	result := r.db.WithContext(ctx).Model(&notificationModel{}).Where("resource_type = ? AND resource_id = ? AND user_id IN (?)", "announcement", id, users).Updates(map[string]any{"subject": subject, "content": content})
	return result.RowsAffected > 0, result.Error
}

func (r *GormRepository) DeleteAnnouncement(ctx context.Context, organizationID, id string) (bool, error) {
	users := r.db.WithContext(ctx).Model(&userModel{}).Select("id").Where("organization_id = ?", organizationID)
	result := r.db.WithContext(ctx).Where("resource_type = ? AND resource_id = ? AND user_id IN (?)", "announcement", id, users).Delete(&notificationModel{})
	return result.RowsAffected > 0, result.Error
}
func (r *GormRepository) ListNotifications(ctx context.Context, userID string, status entity.NotificationStatus, page, size int) ([]entity.Notification, int, error) {
	db := r.db.WithContext(ctx).Model(&notificationModel{}).Where("user_id = ?", userID)
	if status != "" {
		db = db.Where("status = ?", status)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var models []notificationModel
	err := db.Order("created_at DESC, id").Offset((page - 1) * size).Limit(size).Find(&models).Error
	items := make([]entity.Notification, len(models))
	for index, model := range models {
		items[index] = entity.Notification{ID: model.ID, UserID: model.UserID, Type: model.Type, Subject: model.Subject, Content: model.Content, ResourceID: model.ResourceID, ResourceType: model.ResourceType, Status: entity.NotificationStatus(model.Status), CreatedAt: model.CreatedAt}
	}
	return items, int(total), err
}
func (r *GormRepository) MarkNotificationRead(ctx context.Context, id, userID string) (bool, error) {
	result := r.db.WithContext(ctx).Model(&notificationModel{}).Where("id = ? AND user_id = ?", id, userID).Update("status", entity.NotificationRead)
	return result.RowsAffected > 0, result.Error
}
func (r *GormRepository) MarkAllNotificationsRead(ctx context.Context, userID string) (int, error) {
	result := r.db.WithContext(ctx).Model(&notificationModel{}).Where("user_id = ? AND status = ?", userID, entity.NotificationUnread).Update("status", entity.NotificationRead)
	return int(result.RowsAffected), result.Error
}

type FileStorage struct{ directory string }

func NewFileStorage(directory string) (*FileStorage, error) {
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return nil, fmt.Errorf("创建附件目录失败: %w", err)
	}
	return &FileStorage{directory: directory}, nil
}
func (s *FileStorage) path(id string) string { return filepath.Join(s.directory, id) }
func (s *FileStorage) Save(_ context.Context, id string, content []byte) error {
	return os.WriteFile(s.path(id), content, 0o600)
}
func (s *FileStorage) Read(_ context.Context, id string) ([]byte, error) {
	return os.ReadFile(s.path(id))
}
func (s *FileStorage) Delete(_ context.Context, id string) error {
	err := os.Remove(s.path(id))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
func (r *GormRepository) ListCustomFields(ctx context.Context, module string) ([]entity.CustomField, error) {
	var models []customFieldModel
	err := r.db.WithContext(ctx).Where("module = ?", module).Order("key").Find(&models).Error
	out := make([]entity.CustomField, len(models))
	for i, m := range models {
		definition := map[string]any{}
		_ = json.Unmarshal([]byte(m.Definition), &definition)
		out[i] = entity.CustomField{Module: m.Module, Key: m.Key, Definition: definition}
	}
	return out, err
}
func (r *GormRepository) CreateCustomField(ctx context.Context, e entity.CustomField) error {
	definition, err := json.Marshal(e.Definition)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Create(&customFieldModel{Module: e.Module, Key: e.Key, Definition: string(definition)}).Error
}
func (r *GormRepository) UpdateCustomField(ctx context.Context, e entity.CustomField) error {
	definition, err := json.Marshal(e.Definition)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Model(&customFieldModel{}).Where("module = ? AND key = ?", e.Module, e.Key).Update("definition", string(definition)).Error
}
func (r *GormRepository) DeleteCustomField(ctx context.Context, module, key string) error {
	return r.db.WithContext(ctx).Delete(&customFieldModel{}, "module = ? AND key = ?", module, key).Error
}
func (r *GormRepository) ListStandardFieldSettings(ctx context.Context, module string) ([]entity.StandardFieldSetting, error) {
	var models []standardFieldSettingModel
	err := r.db.WithContext(ctx).Where("module = ?", module).Find(&models).Error
	settings := make([]entity.StandardFieldSetting, len(models))
	for index, model := range models {
		settings[index] = entity.StandardFieldSetting{Module: model.Module, Key: model.Key, Label: model.Label, Archived: model.Archived}
	}
	return settings, err
}
func (r *GormRepository) SaveStandardFieldSetting(ctx context.Context, setting entity.StandardFieldSetting) error {
	return r.db.WithContext(ctx).Save(&standardFieldSettingModel{Module: setting.Module, Key: setting.Key, Label: setting.Label, Archived: setting.Archived}).Error
}

func (r *GormRepository) ListOrganizations(ctx context.Context) ([]entity.Organization, error) {
	var models []organizationModel
	err := r.db.WithContext(ctx).Order("created_at, name").Find(&models).Error
	items := make([]entity.Organization, len(models))
	for index, model := range models {
		items[index] = entity.Organization{ID: model.ID, Name: model.Name, CreatedAt: model.CreatedAt}
	}
	return items, err
}

func (r *GormRepository) SaveOrganization(ctx context.Context, item entity.Organization) error {
	return r.db.WithContext(ctx).Save(&organizationModel{ID: item.ID, Name: item.Name, CreatedAt: item.CreatedAt}).Error
}

func (r *GormRepository) DeleteOrganization(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		checks := []any{&userModel{}, &departmentModel{}, &roleModel{}, &recordModel{}}
		for _, model := range checks {
			var count int64
			if err := tx.Model(model).Where("organization_id = ?", id).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return entity.NewBusinessError("conflict", "组织仍有关联成员、部门、角色或业务数据，无法删除")
			}
		}
		result := tx.Delete(&organizationModel{}, "id = ?", id)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return entity.NewBusinessError("not_found", "组织不存在")
		}
		return nil
	})
}

func (r *GormRepository) ListDepartments(ctx context.Context, organizationID string) ([]entity.Department, error) {
	var models []departmentModel
	err := r.db.WithContext(ctx).Where("organization_id = ?", organizationID).Order("sort, name").Find(&models).Error
	items := make([]entity.Department, len(models))
	for index, model := range models {
		items[index] = entity.Department{ID: model.ID, OrganizationID: model.OrganizationID, ParentID: model.ParentID, Name: model.Name, Sort: model.Sort}
	}
	return items, err
}

func (r *GormRepository) SaveDepartment(ctx context.Context, item entity.Department) error {
	return r.db.WithContext(ctx).Save(&departmentModel{ID: item.ID, OrganizationID: item.OrganizationID, ParentID: item.ParentID, Name: item.Name, Sort: item.Sort}).Error
}
func (r *GormRepository) DeleteDepartment(ctx context.Context, id, organizationID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&departmentModel{}).Where("organization_id = ? AND parent_id = ?", organizationID, id).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return entity.NewBusinessError("conflict", "部门仍有下级部门，无法删除")
		}
		if err := tx.Model(&userModel{}).Where("organization_id = ? AND department = ?", organizationID, id).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return entity.NewBusinessError("conflict", "部门仍有关联成员，无法删除")
		}
		var roles []roleModel
		if err := tx.Where("organization_id = ?", organizationID).Find(&roles).Error; err != nil {
			return err
		}
		for _, role := range roles {
			var departmentIDs []string
			if role.DepartmentIDs != "" && json.Unmarshal([]byte(role.DepartmentIDs), &departmentIDs) != nil {
				return entity.NewBusinessError("conflict", "角色部门范围配置无效，无法删除部门")
			}
			if slices.Contains(departmentIDs, id) {
				return entity.NewBusinessError("conflict", "部门仍被角色数据范围引用，无法删除")
			}
		}
		result := tx.Delete(&departmentModel{}, "id = ? AND organization_id = ?", id, organizationID)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return entity.NewBusinessError("not_found", "部门不存在")
		}
		return nil
	})
}
func (r *GormRepository) ListRoles(ctx context.Context, organizationID string) ([]entity.SystemRole, error) {
	var models []roleModel
	err := r.db.WithContext(ctx).Where("organization_id = ?", organizationID).Order("name").Find(&models).Error
	items := make([]entity.SystemRole, len(models))
	for index, model := range models {
		var permissions []string
		var departmentIDs []string
		fieldRestrictions := map[string]entity.FieldRestriction{}
		_ = json.Unmarshal([]byte(model.PermissionIDs), &permissions)
		_ = json.Unmarshal([]byte(model.DepartmentIDs), &departmentIDs)
		_ = json.Unmarshal([]byte(model.FieldRestrictions), &fieldRestrictions)
		items[index] = entity.SystemRole{ID: model.ID, OrganizationID: model.OrganizationID, Name: model.Name, BuiltIn: model.BuiltIn, PermissionIDs: permissions, DataScope: model.DataScope, DepartmentIDs: departmentIDs, FieldRestrictions: fieldRestrictions}
	}
	return items, err
}

func (r *GormRepository) FindRoleByID(ctx context.Context, id string) (entity.SystemRole, error) {
	var model roleModel
	if err := r.db.WithContext(ctx).First(&model, "id = ?", id).Error; err != nil {
		return entity.SystemRole{}, err
	}
	var permissions []string
	var departmentIDs []string
	fieldRestrictions := map[string]entity.FieldRestriction{}
	if err := json.Unmarshal([]byte(model.PermissionIDs), &permissions); err != nil {
		return entity.SystemRole{}, err
	}
	if err := json.Unmarshal([]byte(model.DepartmentIDs), &departmentIDs); err != nil {
		return entity.SystemRole{}, err
	}
	if strings.TrimSpace(model.FieldRestrictions) != "" {
		if err := json.Unmarshal([]byte(model.FieldRestrictions), &fieldRestrictions); err != nil {
			return entity.SystemRole{}, err
		}
	}
	return entity.SystemRole{ID: model.ID, OrganizationID: model.OrganizationID, Name: model.Name, BuiltIn: model.BuiltIn, PermissionIDs: permissions, DataScope: model.DataScope, DepartmentIDs: departmentIDs, FieldRestrictions: fieldRestrictions}, nil
}

func (r *GormRepository) SaveRole(ctx context.Context, item entity.SystemRole) error {
	permissions, err := json.Marshal(item.PermissionIDs)
	if err != nil {
		return err
	}
	departmentIDs, err := json.Marshal(item.DepartmentIDs)
	if err != nil {
		return err
	}
	fieldRestrictions, err := json.Marshal(item.FieldRestrictions)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Save(&roleModel{ID: item.ID, OrganizationID: item.OrganizationID, Name: item.Name, BuiltIn: item.BuiltIn, PermissionIDs: string(permissions), DataScope: item.DataScope, DepartmentIDs: string(departmentIDs), FieldRestrictions: string(fieldRestrictions)}).Error
}
func (r *GormRepository) DeleteRole(ctx context.Context, id string) error {
	var count int64
	if err := r.db.WithContext(ctx).Model(&userModel{}).Where("role_id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return entity.NewBusinessError("conflict", "角色仍有成员使用，无法删除")
	}
	result := r.db.WithContext(ctx).Delete(&roleModel{}, "id = ? AND built_in = ?", id, false)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return entity.NewBusinessError("not_found", "角色不存在")
	}
	return nil
}
func (r *GormRepository) ListModules(ctx context.Context) ([]entity.ModuleDefinition, error) {
	var modules []moduleModel
	if err := r.db.WithContext(ctx).Order("name").Find(&modules).Error; err != nil {
		return nil, err
	}
	items := make([]entity.ModuleDefinition, len(modules))
	for index, model := range modules {
		layout := []entity.FieldLayout{}
		salesStages := []string{}
		stages := []entity.StageDefinition{}
		stageTransitions := []entity.StageTransition{}
		if model.Layout != "" {
			if err := json.Unmarshal([]byte(model.Layout), &layout); err != nil {
				return nil, err
			}
		}
		if model.SalesStages != "" {
			if err := json.Unmarshal([]byte(model.SalesStages), &salesStages); err != nil {
				return nil, err
			}
		}
		if model.Stages != "" {
			if err := json.Unmarshal([]byte(model.Stages), &stages); err != nil {
				return nil, err
			}
		}
		if model.StageTransitions != "" {
			if err := json.Unmarshal([]byte(model.StageTransitions), &stageTransitions); err != nil {
				return nil, err
			}
		}
		var fields []fieldModel
		if err := r.db.WithContext(ctx).Where("module_id = ?", model.ID).Order("sort").Find(&fields).Error; err != nil {
			return nil, err
		}
		fieldDefinitions := make([]entity.FieldDefinition, len(fields))
		for fieldIndex, field := range fields {
			var options []string
			if field.Options != "" {
				if err := json.Unmarshal([]byte(field.Options), &options); err != nil {
					return nil, err
				}
			}
			fieldDefinitions[fieldIndex] = entity.FieldDefinition{ID: field.ID, ModuleID: field.ModuleID, Key: field.Key, Name: field.Name, Type: field.Type, Required: field.Required, Unique: field.Unique, ReadOnly: field.ReadOnly, Options: options, Sort: field.Sort}
		}
		items[index] = entity.ModuleDefinition{ID: model.ID, Key: model.Key, Name: model.Name, Icon: model.Icon, DocumentTemplate: model.DocumentTemplate, Group: model.GroupName, Description: model.Description, Enabled: model.Enabled, Shared: model.Shared, Approvable: model.Approvable, Custom: model.Custom, Fields: fieldDefinitions, Layout: layout, SalesStages: salesStages, Stages: stages, AfootRollback: model.AfootRollback, EndRollback: model.EndRollback, CirculationType: model.CirculationType, StageTransitions: stageTransitions}
	}
	return items, nil
}
func (r *GormRepository) SaveModule(ctx context.Context, item entity.ModuleDefinition) error {
	layout, err := json.Marshal(item.Layout)
	if err != nil {
		return err
	}
	salesStages, err := json.Marshal(item.SalesStages)
	if err != nil {
		return err
	}
	stages, err := json.Marshal(item.Stages)
	if err != nil {
		return err
	}
	stageTransitions, err := json.Marshal(item.StageTransitions)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&moduleModel{ID: item.ID, Key: item.Key, Name: item.Name, Icon: item.Icon, GroupName: item.Group, Description: item.Description, Layout: string(layout), DocumentTemplate: item.DocumentTemplate, SalesStages: string(salesStages), Stages: string(stages), AfootRollback: item.AfootRollback, EndRollback: item.EndRollback, CirculationType: item.CirculationType, StageTransitions: string(stageTransitions), Enabled: item.Enabled, Shared: item.Shared, Approvable: item.Approvable, Custom: item.Custom, Initialized: true}).Error; err != nil {
			return err
		}
		if !item.Custom {
			return nil
		}
		ids := make([]string, 0, len(item.Fields))
		for _, field := range item.Fields {
			options, err := json.Marshal(field.Options)
			if err != nil {
				return err
			}
			ids = append(ids, field.ID)
			if err := tx.Save(&fieldModel{ID: field.ID, ModuleID: item.ID, Key: field.Key, Name: field.Name, Type: field.Type, Options: string(options), Required: field.Required, Unique: field.Unique, ReadOnly: field.ReadOnly, Sort: field.Sort}).Error; err != nil {
				return err
			}
		}
		query := tx.Where("module_id = ?", item.ID)
		if len(ids) > 0 {
			query = query.Where("id NOT IN ?", ids)
		}
		return query.Delete(&fieldModel{}).Error
	})
}

func (r *GormRepository) DeleteModule(ctx context.Context, id, key string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&recordModel{}).Where("module = ?", key).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return entity.NewBusinessError("conflict", "模块已有业务数据，无法删除；可改为停用")
		}
		if err := tx.Delete(&customFieldModel{}, "module = ?", key).Error; err != nil {
			return err
		}
		if err := tx.Delete(&fieldModel{}, "module_id = ?", id).Error; err != nil {
			return err
		}
		result := tx.Delete(&moduleModel{}, "id = ? AND key = ? AND custom = ?", id, key, true)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return entity.NewBusinessError("not_found", "自定义模块不存在")
		}
		return nil
	})
}

func (r *GormRepository) CreateAPIKey(ctx context.Context, item entity.APIKey) error {
	return r.db.WithContext(ctx).Create(&apiKeyModel{ID: item.ID, UserID: item.UserID, Name: item.Name, Prefix: item.Prefix, SecretHash: item.SecretHash, Active: item.Active, CreatedAt: item.CreatedAt, ExpiresAt: item.ExpiresAt, LastUsedAt: item.LastUsedAt}).Error
}

func (r *GormRepository) ListAPIKeys(ctx context.Context, userID string) ([]entity.APIKey, error) {
	var models []apiKeyModel
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("created_at DESC").Find(&models).Error; err != nil {
		return nil, err
	}
	items := make([]entity.APIKey, len(models))
	for index, model := range models {
		items[index] = entity.APIKey{ID: model.ID, UserID: model.UserID, Name: model.Name, Prefix: model.Prefix, SecretHash: model.SecretHash, Active: model.Active, CreatedAt: model.CreatedAt, ExpiresAt: model.ExpiresAt, LastUsedAt: model.LastUsedAt}
	}
	return items, nil
}

func (r *GormRepository) FindAPIKeyByPrefix(ctx context.Context, prefix string) (entity.APIKey, error) {
	var model apiKeyModel
	err := r.db.WithContext(ctx).Where("prefix = ?", prefix).First(&model).Error
	return entity.APIKey{ID: model.ID, UserID: model.UserID, Name: model.Name, Prefix: model.Prefix, SecretHash: model.SecretHash, Active: model.Active, CreatedAt: model.CreatedAt, ExpiresAt: model.ExpiresAt, LastUsedAt: model.LastUsedAt}, err
}

func (r *GormRepository) UpdateAPIKey(ctx context.Context, item entity.APIKey) error {
	return r.db.WithContext(ctx).Save(&apiKeyModel{ID: item.ID, UserID: item.UserID, Name: item.Name, Prefix: item.Prefix, SecretHash: item.SecretHash, Active: item.Active, CreatedAt: item.CreatedAt, ExpiresAt: item.ExpiresAt, LastUsedAt: item.LastUsedAt}).Error
}

func (r *GormRepository) DeleteAPIKey(ctx context.Context, id, userID string) error {
	result := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).Delete(&apiKeyModel{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *GormRepository) ListAgents(ctx context.Context, organizationID string) ([]entity.AgentDefinition, error) {
	var models []agentModel
	query := r.db.WithContext(ctx).Order("created_at DESC")
	if organizationID != "" {
		query = query.Where("organization_id = ?", organizationID)
	}
	if err := query.Find(&models).Error; err != nil {
		return nil, err
	}
	items := make([]entity.AgentDefinition, len(models))
	for index, model := range models {
		var toolNames []string
		if err := json.Unmarshal([]byte(model.ToolNames), &toolNames); err != nil {
			return nil, err
		}
		items[index] = entity.AgentDefinition{ID: model.ID, OrganizationID: model.OrganizationID, Name: model.Name, Prompt: model.Prompt, ToolNames: toolNames, Enabled: model.Enabled, CreatedAt: model.CreatedAt, UpdatedAt: model.UpdatedAt}
	}
	return items, nil
}

func (r *GormRepository) FindAgent(ctx context.Context, id string) (entity.AgentDefinition, error) {
	var model agentModel
	if err := r.db.WithContext(ctx).First(&model, "id = ?", id).Error; err != nil {
		return entity.AgentDefinition{}, err
	}
	var toolNames []string
	if err := json.Unmarshal([]byte(model.ToolNames), &toolNames); err != nil {
		return entity.AgentDefinition{}, err
	}
	return entity.AgentDefinition{ID: model.ID, OrganizationID: model.OrganizationID, Name: model.Name, Prompt: model.Prompt, ToolNames: toolNames, Enabled: model.Enabled, CreatedAt: model.CreatedAt, UpdatedAt: model.UpdatedAt}, nil
}

func (r *GormRepository) SaveAgent(ctx context.Context, item entity.AgentDefinition) error {
	toolNames, err := json.Marshal(item.ToolNames)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Save(&agentModel{ID: item.ID, OrganizationID: item.OrganizationID, Name: item.Name, Prompt: item.Prompt, ToolNames: string(toolNames), Enabled: item.Enabled, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}).Error
}

func (r *GormRepository) DeleteAgent(ctx context.Context, id, organizationID string) error {
	result := r.db.WithContext(ctx).Delete(&agentModel{}, "id = ? AND organization_id = ?", id, organizationID)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *GormRepository) ListDashboards(ctx context.Context, organizationID string) ([]entity.DashboardDefinition, error) {
	var models []dashboardModel
	query := r.db.WithContext(ctx).Order("is_default DESC, created_at")
	if organizationID != "" {
		query = query.Where("organization_id = ?", organizationID)
	}
	if err := query.Find(&models).Error; err != nil {
		return nil, err
	}
	items := make([]entity.DashboardDefinition, len(models))
	for index, model := range models {
		var widgets []entity.DashboardWidget
		if err := json.Unmarshal([]byte(model.Widgets), &widgets); err != nil {
			return nil, err
		}
		items[index] = entity.DashboardDefinition{ID: model.ID, OrganizationID: model.OrganizationID, Name: model.Name, Widgets: widgets, IsDefault: model.IsDefault, CreatedAt: model.CreatedAt, UpdatedAt: model.UpdatedAt}
	}
	return items, nil
}

func (r *GormRepository) FindDashboard(ctx context.Context, id string) (entity.DashboardDefinition, error) {
	var model dashboardModel
	if err := r.db.WithContext(ctx).First(&model, "id = ?", id).Error; err != nil {
		return entity.DashboardDefinition{}, err
	}
	var widgets []entity.DashboardWidget
	if err := json.Unmarshal([]byte(model.Widgets), &widgets); err != nil {
		return entity.DashboardDefinition{}, err
	}
	return entity.DashboardDefinition{ID: model.ID, OrganizationID: model.OrganizationID, Name: model.Name, Widgets: widgets, IsDefault: model.IsDefault, CreatedAt: model.CreatedAt, UpdatedAt: model.UpdatedAt}, nil
}

func (r *GormRepository) SaveDashboard(ctx context.Context, item entity.DashboardDefinition) error {
	widgets, err := json.Marshal(item.Widgets)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if item.IsDefault {
			if err := tx.Model(&dashboardModel{}).Where("organization_id = ? AND id <> ?", item.OrganizationID, item.ID).Update("is_default", false).Error; err != nil {
				return err
			}
		}
		return tx.Save(&dashboardModel{ID: item.ID, OrganizationID: item.OrganizationID, Name: item.Name, Widgets: string(widgets), IsDefault: item.IsDefault, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}).Error
	})
}

func (r *GormRepository) DeleteDashboard(ctx context.Context, id, organizationID string) error {
	result := r.db.WithContext(ctx).Delete(&dashboardModel{}, "id = ? AND organization_id = ?", id, organizationID)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *GormRepository) ListIntegrationSettings(ctx context.Context) ([]entity.IntegrationSetting, error) {
	var models []integrationSettingModel
	if err := r.db.WithContext(ctx).Order("key").Find(&models).Error; err != nil {
		return nil, err
	}
	items := make([]entity.IntegrationSetting, len(models))
	for index, model := range models {
		items[index] = entity.IntegrationSetting{Key: model.Key, Endpoint: model.Endpoint, EncryptedToken: model.EncryptedToken, UpdatedAt: model.UpdatedAt}
	}
	return items, nil
}
func (r *GormRepository) FindIntegrationSetting(ctx context.Context, key string) (entity.IntegrationSetting, error) {
	var model integrationSettingModel
	err := r.db.WithContext(ctx).First(&model, "key = ?", key).Error
	return entity.IntegrationSetting{Key: model.Key, Endpoint: model.Endpoint, EncryptedToken: model.EncryptedToken, UpdatedAt: model.UpdatedAt}, err
}
func (r *GormRepository) SaveIntegrationSetting(ctx context.Context, item entity.IntegrationSetting) error {
	return r.db.WithContext(ctx).Save(&integrationSettingModel{Key: item.Key, Endpoint: item.Endpoint, EncryptedToken: item.EncryptedToken, UpdatedAt: item.UpdatedAt}).Error
}
func (r *GormRepository) DeleteIntegrationSetting(ctx context.Context, key string) error {
	return r.db.WithContext(ctx).Delete(&integrationSettingModel{}, "key = ?", key).Error
}
