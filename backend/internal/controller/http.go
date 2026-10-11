package controller

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"cordialcrm/backend/internal/entity"
	"cordialcrm/backend/internal/integration"
	"cordialcrm/backend/internal/usecase"
	"cordialcrm/backend/internal/wecom"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type Passwords struct{}

func (Passwords) Hash(value string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(value), bcrypt.DefaultCost)
	return string(bytes), err
}
func (Passwords) Compare(hash, value string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(value))
}

type Tokens struct{}

func (Tokens) New() (string, error) {
	bytes := make([]byte, 48)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

type HTTP struct {
	auth                *usecase.AuthUseCase
	users               *usecase.UserUseCase
	apiKeys             *usecase.APIKeyUseCase
	agents              *usecase.AgentUseCase
	dashboards          *usecase.DashboardConfigUseCase
	integrationSettings *usecase.IntegrationSettingUseCase
	records             *usecase.RecordUseCase
	approvalUseCase     *usecase.ApprovalUseCase
	auditUseCase        *usecase.AuditUseCase
	systemUseCase       *usecase.SystemUseCase
	attachmentUseCase   *usecase.AttachmentUseCase
	notificationUseCase *usecase.NotificationUseCase
	followupPlanUseCase *usecase.FollowupPlanUseCase
	approvalFlowUseCase *usecase.ApprovalFlowUseCase
	recordTool          *RecordTool
	followupPlanTool    *FollowupPlanTool
	cookieSecure        bool
	frontend            string
	weCom               *wecom.Client
	integrations        *integration.Registry
}

func New(auth *usecase.AuthUseCase, users *usecase.UserUseCase, apiKeys *usecase.APIKeyUseCase, agents *usecase.AgentUseCase, dashboards *usecase.DashboardConfigUseCase, integrationSettings *usecase.IntegrationSettingUseCase, records *usecase.RecordUseCase, approvals *usecase.ApprovalUseCase, audits *usecase.AuditUseCase, system *usecase.SystemUseCase, attachments *usecase.AttachmentUseCase, notifications *usecase.NotificationUseCase, plans *usecase.FollowupPlanUseCase, approvalFlows *usecase.ApprovalFlowUseCase, weCom *wecom.Client, integrations *integration.Registry, cookieSecure bool, frontend string) *HTTP {
	return &HTTP{auth: auth, users: users, apiKeys: apiKeys, agents: agents, dashboards: dashboards, integrationSettings: integrationSettings, records: records, approvalUseCase: approvals, auditUseCase: audits, systemUseCase: system, attachmentUseCase: attachments, notificationUseCase: notifications, followupPlanUseCase: plans, approvalFlowUseCase: approvalFlows, recordTool: NewRecordTool(records), followupPlanTool: NewFollowupPlanTool(plans), weCom: weCom, integrations: integrations, cookieSecure: cookieSecure, frontend: frontend}
}
func (h *HTTP) Router() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery(), h.protect())
	r.POST("/mcp", h.mcp)
	api := r.Group("/api")
	api.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
	api.GET("/docs", h.docs)
	api.GET("/openapi.json", h.openAPI)
	api.GET("/agent/tools", h.listAgentTools)
	api.POST("/agent/tools/record.search", h.agentRecordSearch)
	api.POST("/agent/tools/record.list", h.agentRecordList)
	api.POST("/agent/tools/record.get", h.agentRecordGet)
	api.POST("/agent/tools/followup-plan.list", h.agentFollowupPlanList)
	api.GET("/setup", h.setupStatus)
	api.POST("/setup", h.setup)
	api.POST("/auth/login", h.login)
	api.GET("/auth/wecom/status", h.weComStatus)
	api.GET("/auth/wecom/start", h.weComStart)
	api.GET("/auth/wecom/callback", h.weComCallback)
	api.POST("/auth/wecom/sync", h.syncWeComUsers)
	api.GET("/integrations", h.listIntegrations)
	api.PUT("/integrations/:key", h.saveIntegration)
	api.DELETE("/integrations/:key", h.deleteIntegration)
	api.POST("/integrations/:key/test", h.testIntegration)
	api.GET("/integrations/wecom/config", h.weComConfig)
	api.PUT("/integrations/wecom/config", h.saveWeComConfig)
	api.DELETE("/integrations/wecom/config", h.deleteWeComConfig)
	api.POST("/integrations/wecom/test", h.testWeComConfig)
	api.GET("/auth/me", h.me)
	api.POST("/auth/logout", h.logout)
	api.POST("/auth/password", h.password)
	api.GET("/auth/api-keys", h.listAPIKeys)
	api.POST("/auth/api-keys", h.createAPIKey)
	api.PUT("/auth/api-keys/:id", h.updateAPIKey)
	api.DELETE("/auth/api-keys/:id", h.deleteAPIKey)
	api.GET("/agents", h.listAgents)
	api.POST("/agents", h.saveAgent)
	api.PUT("/agents/:id", h.saveAgent)
	api.DELETE("/agents/:id", h.deleteAgent)
	api.GET("/dashboard/configs", h.listDashboardConfigs)
	api.POST("/dashboard/configs", h.saveDashboardConfig)
	api.PUT("/dashboard/configs/:id", h.saveDashboardConfig)
	api.DELETE("/dashboard/configs/:id", h.deleteDashboardConfig)
	api.GET("/users", h.listUsers)
	api.POST("/users", h.createUser)
	api.PUT("/users/:id", h.updateUser)
	api.GET("/catalog", h.catalog)
	api.POST("/catalog/:module/fields", h.addCustomField)
	api.PUT("/catalog/:module/fields/:key", h.renameCustomField)
	api.DELETE("/catalog/:module/fields/:key", h.deleteCustomField)
	api.GET("/dashboard", h.dashboard)
	api.GET("/analytics/report", h.analyticsReport)
	api.GET("/search", h.search)
	api.GET("/approvals", h.approvals)
	api.POST("/approvals/:id/decision", h.decision)
	api.POST("/approvals/:id/transfer", h.transferApproval)
	api.POST("/approvals/:id/add-signer", h.addApprovalSigner)
	api.POST("/approvals/:id/withdraw", h.withdrawApproval)
	api.GET("/approval-flows", h.listApprovalFlows)
	api.POST("/approval-flows", h.saveApprovalFlow)
	api.PUT("/approval-flows/:id", h.saveApprovalFlow)
	api.POST("/approval-flows/:id/enabled", h.setApprovalFlowEnabled)
	api.GET("/audit", h.audit)
	api.GET("/notifications", h.listNotifications)
	api.GET("/notifications/latest", h.latestNotifications)
	api.GET("/notifications/count", h.notificationCount)
	api.GET("/notifications/stream", h.notificationStream)
	api.POST("/notifications/:id/read", h.readNotification)
	api.POST("/notifications/read-all", h.readAllNotifications)
	api.POST("/notifications/announcements", h.broadcastAnnouncement)
	api.GET("/notifications/announcements", h.listAnnouncements)
	api.PUT("/notifications/announcements/:id", h.updateAnnouncement)
	api.DELETE("/notifications/announcements/:id", h.deleteAnnouncement)
	api.GET("/system/organizations", h.listOrganizations)
	api.POST("/system/organizations", h.createOrganization)
	api.POST("/system/organizations/import", h.importOrganizations)
	api.PUT("/system/organizations/:id", h.updateOrganization)
	api.DELETE("/system/organizations/:id", h.deleteOrganization)
	api.GET("/system/organizations/:id/departments", h.listDepartments)
	api.POST("/system/departments", h.saveDepartment)
	api.PUT("/system/departments/:id", h.saveDepartment)
	api.DELETE("/system/departments/:id", h.deleteDepartment)
	api.GET("/system/organizations/:id/roles", h.listRoles)
	api.POST("/system/roles", h.saveRole)
	api.DELETE("/system/roles/:id", h.deleteRole)
	api.GET("/system/permissions", h.listPermissions)
	api.GET("/system/modules", h.listSystemModules)
	api.POST("/system/modules", h.saveSystemModule)
	api.DELETE("/system/modules/:id", h.deleteSystemModule)
	api.GET("/records/:module", h.listRecords)
	api.POST("/records/:module", h.createRecord)
	api.GET("/records/:module/export", h.exportRecords)
	api.GET("/records/:module/import-template", h.importTemplate)
	api.POST("/records/:module/import/preview", h.previewImportRecords)
	api.POST("/records/:module/import", h.importRecords)
	api.GET("/records/:module/:id/ownership-history", h.listOwnershipHistory)
	api.GET("/records/:module/:id", h.getRecord)
	api.PUT("/records/:module/:id", h.updateRecord)
	api.DELETE("/records/:module/:id", h.deleteRecord)
	api.POST("/records/:module/:id/followups", h.followup)
	api.GET("/records/:module/:id/followups/:followupID/comments", h.listFollowupComments)
	api.POST("/records/:module/:id/followups/:followupID/comments", h.createFollowupComment)
	api.PUT("/records/:module/:id/followups/:followupID/comments/:commentID", h.updateFollowupComment)
	api.DELETE("/records/:module/:id/followups/:followupID/comments/:commentID", h.deleteFollowupComment)
	api.GET("/records/:module/:id/followup-plans", h.listFollowupPlans)
	api.POST("/records/:module/:id/followup-plans", h.createFollowupPlan)
	api.PUT("/followup-plans/:id", h.updateFollowupPlan)
	api.POST("/followup-plans/:id/status", h.updateFollowupPlanStatus)
	api.DELETE("/followup-plans/:id", h.deleteFollowupPlan)
	api.GET("/followup-plans/:id/comments", h.listFollowupPlanComments)
	api.POST("/followup-plans/:id/comments", h.createFollowupPlanComment)
	api.PUT("/followup-plans/:id/comments/:commentID", h.updateFollowupPlanComment)
	api.DELETE("/followup-plans/:id/comments/:commentID", h.deleteFollowupPlanComment)
	api.POST("/records/:module/:id/actions/:action", h.action)
	api.GET("/records/:module/:id/attachments", h.listAttachments)
	api.POST("/records/:module/:id/attachments", h.uploadAttachments)
	api.GET("/attachments/:id", h.downloadAttachment)
	api.DELETE("/attachments/:id", h.deleteAttachment)
	r.Static("/assets", filepath.Join(h.frontend, "assets"))
	r.StaticFile("/manifest.webmanifest", filepath.Join(h.frontend, "manifest.webmanifest"))
	r.StaticFile("/service-worker.js", filepath.Join(h.frontend, "service-worker.js"))
	r.StaticFile("/icons/app-icon.svg", filepath.Join(h.frontend, "icons", "app-icon.svg"))
	r.GET("/", func(c *gin.Context) { c.File(filepath.Join(h.frontend, "index.html")) })
	return r
}
func (h *HTTP) protect() gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api") {
			c.Header("Cache-Control", "no-store")
		}
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "same-origin")
		c.Next()
	}
}
func respond(c *gin.Context, status int, value any, err error) {
	if err == nil {
		c.JSON(status, value)
		return
	}
	var business *entity.BusinessError
	if errors.As(err, &business) {
		codes := map[string]int{"unauthorized": 401, "forbidden": 403, "not_found": 404, "conflict": 409, "rate_limited": 429, "validation": 422}
		c.JSON(codes[business.Code], gin.H{"detail": business.Message})
		return
	}
	c.JSON(500, gin.H{"detail": "服务器内部错误"})
}
func bind(c *gin.Context, value any) bool {
	if err := c.ShouldBindJSON(value); err != nil {
		respond(c, 0, nil, usecase.ErrValidation)
		return false
	}
	return true
}
func (h *HTTP) actor(c *gin.Context) (usecase.UserOutput, bool) {
	if secret := c.GetHeader("X-API-Key"); secret != "" {
		value, err := h.apiKeys.Authenticate(c.Request.Context(), usecase.AuthenticateAPIKeyInput{Secret: secret})
		if err != nil {
			respond(c, 0, nil, err)
			return usecase.UserOutput{}, false
		}
		return value, true
	}
	token, _ := c.Cookie("crm_session")
	value, err := h.auth.CurrentUser(c.Request.Context(), token)
	if err != nil {
		respond(c, 0, nil, err)
		return usecase.UserOutput{}, false
	}
	return value, true
}
func (h *HTTP) setSession(c *gin.Context, token string) {
	c.SetCookie("crm_session", token, 12*3600, "/api", "", h.cookieSecure, true)
}
func (h *HTTP) setupStatus(c *gin.Context) {
	required, err := h.auth.SetupRequired(c.Request.Context())
	respond(c, http.StatusOK, gin.H{"required": required}, err)
}
func (h *HTTP) setup(c *gin.Context) {
	var input usecase.SetupInput
	if !bind(c, &input) {
		return
	}
	out, err := h.auth.Setup(c.Request.Context(), input)
	if err == nil {
		h.setSession(c, out.Token)
	}
	respond(c, 201, out.User, err)
}
func (h *HTTP) login(c *gin.Context) {
	var input usecase.LoginInput
	if !bind(c, &input) {
		return
	}
	out, err := h.auth.Login(c.Request.Context(), input)
	if err == nil {
		h.setSession(c, out.Token)
	}
	respond(c, 200, out.User, err)
}
func (h *HTTP) weComStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"enabled": h.weCom != nil && h.weCom.Configured() && h.weCom.OAuthRedirectURL() != ""})
}
func (h *HTTP) weComStart(c *gin.Context) {
	redirectURL := ""
	if h.weCom != nil {
		redirectURL = h.weCom.OAuthRedirectURL()
	}
	if h.weCom == nil || !h.weCom.Configured() || redirectURL == "" {
		respond(c, 0, nil, entity.NewBusinessError("not_found", "企业微信 OAuth 未配置"))
		return
	}
	state, err := (Tokens{}).New()
	if err != nil {
		respond(c, 0, nil, err)
		return
	}
	c.SetCookie("wecom_oauth_state", state, 600, "/api/auth/wecom", "", h.cookieSecure, true)
	c.Redirect(http.StatusFound, h.weCom.AuthorizationURL(redirectURL, state))
}
func (h *HTTP) weComCallback(c *gin.Context) {
	state, _ := c.Cookie("wecom_oauth_state")
	if h.weCom == nil || state == "" || state != c.Query("state") || c.Query("code") == "" {
		respond(c, 0, nil, entity.NewBusinessError("unauthorized", "企业微信登录状态无效"))
		return
	}
	userID, err := h.weCom.UserIDByCode(c.Request.Context(), c.Query("code"))
	if err != nil {
		respond(c, 0, nil, err)
		return
	}
	out, err := h.auth.LoginExternal(c.Request.Context(), userID)
	if err != nil {
		respond(c, 0, nil, err)
		return
	}
	h.setSession(c, out.Token)
	c.SetCookie("wecom_oauth_state", "", -1, "/api/auth/wecom", "", h.cookieSecure, true)
	c.Redirect(http.StatusFound, "/")
}
func (h *HTTP) syncWeComUsers(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	if h.weCom == nil || !h.weCom.Configured() {
		respond(c, 0, nil, entity.NewBusinessError("not_found", "企业微信未配置"))
		return
	}
	items, err := h.weCom.ListUsers(c.Request.Context())
	if err != nil {
		respond(c, 0, nil, err)
		return
	}
	users := make([]usecase.ExternalUser, len(items))
	for index, item := range items {
		users[index] = usecase.ExternalUser{Username: item.UserID, Name: item.Name}
	}
	count, err := h.users.SyncExternal(c.Request.Context(), actor, users)
	respond(c, http.StatusOK, gin.H{"count": count}, err)
}
func (h *HTTP) weComConfig(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	if !usecase.HasPermission(actor, "SYSTEM_SETTING:READ") {
		respond(c, 0, nil, usecase.ErrForbidden)
		return
	}
	status, err := h.integrationSettings.WeComStatus(c.Request.Context())
	respond(c, http.StatusOK, status, err)
}
func (h *HTTP) saveWeComConfig(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	var payload usecase.WeComConfig
	if !bind(c, &payload) {
		return
	}
	err := h.integrationSettings.SaveWeCom(c.Request.Context(), actor, payload)
	respond(c, http.StatusOK, gin.H{"ok": true}, err)
}
func (h *HTTP) deleteWeComConfig(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	err := h.integrationSettings.DeleteWeCom(c.Request.Context(), actor)
	respond(c, http.StatusOK, gin.H{"ok": true}, err)
}
func (h *HTTP) testWeComConfig(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	err := h.integrationSettings.TestWeCom(c.Request.Context(), actor)
	respond(c, http.StatusOK, gin.H{"ok": true}, err)
}
func (h *HTTP) listIntegrations(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	if !usecase.HasPermission(actor, "SYSTEM_SETTING:READ") {
		respond(c, 0, nil, usecase.ErrForbidden)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": h.integrations.Statuses()})
}
func (h *HTTP) testIntegration(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	if !usecase.HasPermission(actor, "SYSTEM_SETTING:UPDATE") {
		respond(c, 0, nil, usecase.ErrForbidden)
		return
	}
	err := h.integrations.Test(c.Request.Context(), c.Param("key"))
	if err != nil {
		respond(c, 0, nil, entity.NewBusinessError("validation", err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
func (h *HTTP) saveIntegration(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	var payload struct {
		Endpoint string `json:"endpoint"`
		Token    string `json:"token"`
	}
	if !bind(c, &payload) {
		return
	}
	err := h.integrationSettings.Save(c.Request.Context(), actor, c.Param("key"), payload.Endpoint, payload.Token)
	respond(c, http.StatusOK, gin.H{"ok": true}, err)
}
func (h *HTTP) deleteIntegration(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	err := h.integrationSettings.Delete(c.Request.Context(), actor, c.Param("key"))
	respond(c, http.StatusOK, gin.H{"ok": true}, err)
}
func (h *HTTP) me(c *gin.Context) {
	actor, ok := h.actor(c)
	if ok {
		c.JSON(200, actor)
	}
}
func (h *HTTP) logout(c *gin.Context) {
	token, _ := c.Cookie("crm_session")
	err := h.auth.Logout(c.Request.Context(), token)
	c.SetCookie("crm_session", "", -1, "/api", "", h.cookieSecure, true)
	respond(c, 200, gin.H{"ok": true}, err)
}
func (h *HTTP) password(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	var payload struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if !bind(c, &payload) {
		return
	}
	out, err := h.auth.ChangePassword(c.Request.Context(), usecase.ChangePasswordInput{UserID: actor.ID, CurrentPassword: payload.CurrentPassword, NewPassword: payload.NewPassword})
	if err == nil {
		h.setSession(c, out.Token)
	}
	respond(c, 200, gin.H{"ok": true}, err)
}
func (h *HTTP) listAPIKeys(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	out, err := h.apiKeys.List(c.Request.Context(), usecase.ListAPIKeysInput{Actor: actor})
	respond(c, http.StatusOK, out, err)
}
func (h *HTTP) createAPIKey(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	var payload struct {
		Name      string     `json:"name"`
		ExpiresAt *time.Time `json:"expires_at"`
	}
	if !bind(c, &payload) {
		return
	}
	out, err := h.apiKeys.Create(c.Request.Context(), usecase.CreateAPIKeyInput{Actor: actor, Name: payload.Name, ExpiresAt: payload.ExpiresAt})
	respond(c, http.StatusCreated, out, err)
}
func (h *HTTP) updateAPIKey(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	var payload struct {
		Name      string     `json:"name"`
		Active    bool       `json:"active"`
		ExpiresAt *time.Time `json:"expires_at"`
	}
	if !bind(c, &payload) {
		return
	}
	out, err := h.apiKeys.Update(c.Request.Context(), usecase.UpdateAPIKeyInput{Actor: actor, ID: c.Param("id"), Name: payload.Name, Active: payload.Active, ExpiresAt: payload.ExpiresAt})
	respond(c, http.StatusOK, out, err)
}
func (h *HTTP) deleteAPIKey(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	err := h.apiKeys.Delete(c.Request.Context(), usecase.DeleteAPIKeyInput{Actor: actor, ID: c.Param("id")})
	respond(c, http.StatusOK, gin.H{"ok": true}, err)
}
func (h *HTTP) listUsers(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	out, err := h.users.List(c.Request.Context(), actor)
	respond(c, 200, out, err)
}
func (h *HTTP) createUser(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	var payload struct {
		Username, Password, Name, Department string
		Role                                 entity.Role
		OrganizationID, RoleID               string
	}
	if !bind(c, &payload) {
		return
	}
	out, err := h.users.Create(c.Request.Context(), usecase.CreateUserInput{Actor: actor, Username: payload.Username, Password: payload.Password, Name: payload.Name, Department: payload.Department, Role: payload.Role, OrganizationID: payload.OrganizationID, RoleID: payload.RoleID})
	respond(c, 201, out, err)
}
func (h *HTTP) updateUser(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	var payload struct {
		Name, Department       string
		Role                   entity.Role
		OrganizationID, RoleID string
		Active                 bool
		Password               *string
	}
	if !bind(c, &payload) {
		return
	}
	out, err := h.users.Update(c.Request.Context(), usecase.UpdateUserInput{Actor: actor, ID: c.Param("id"), Name: payload.Name, Department: payload.Department, Role: payload.Role, OrganizationID: payload.OrganizationID, RoleID: payload.RoleID, Active: payload.Active, Password: payload.Password})
	respond(c, 200, out, err)
}
func (h *HTTP) catalog(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	items, err := h.records.Catalog(c.Request.Context(), actor)
	respond(c, http.StatusOK, items, err)
}
func (h *HTTP) addCustomField(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	var payload struct {
		Key, Label, Type string
		Options          []string
		Sources          []string
		Columns          []usecase.Field
	}
	if !bind(c, &payload) {
		return
	}
	err := h.records.AddCustomField(c.Request.Context(), usecase.AddCustomFieldInput{Actor: actor, Module: c.Param("module"), Key: payload.Key, Label: payload.Label, Type: payload.Type, Options: payload.Options, Sources: payload.Sources, Columns: payload.Columns})
	respond(c, http.StatusCreated, gin.H{"ok": true}, err)
}
func (h *HTTP) deleteCustomField(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	err := h.records.DeleteCustomField(c.Request.Context(), actor, c.Param("module"), c.Param("key"))
	respond(c, http.StatusOK, gin.H{"ok": true}, err)
}
func (h *HTTP) renameCustomField(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	var payload struct{ Label string }
	if !bind(c, &payload) {
		return
	}
	err := h.records.RenameCustomField(c.Request.Context(), actor, c.Param("module"), c.Param("key"), payload.Label)
	respond(c, http.StatusOK, gin.H{"ok": true}, err)
}
func (h *HTTP) dashboard(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	output, err := h.records.Dashboard(c.Request.Context(), usecase.DashboardInput{Actor: actor})
	respond(c, http.StatusOK, output, err)
}
func (h *HTTP) approvals(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	output, err := h.approvalUseCase.List(c.Request.Context(), usecase.ListApprovalsInput{Actor: actor, Status: c.Query("status")})
	respond(c, http.StatusOK, output, err)
}
func (h *HTTP) decision(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	var payload struct{ Decision, Comment string }
	if !bind(c, &payload) {
		return
	}
	err := h.approvalUseCase.Decide(c.Request.Context(), usecase.DecideApprovalInput{Actor: actor, ID: c.Param("id"), Decision: payload.Decision, Comment: payload.Comment})
	respond(c, http.StatusOK, gin.H{"ok": true}, err)
}
func (h *HTTP) withdrawApproval(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	out, err := h.approvalUseCase.Withdraw(c.Request.Context(), usecase.WithdrawApprovalInput{Actor: actor, ID: c.Param("id")})
	respond(c, http.StatusOK, out, err)
}
func (h *HTTP) transferApproval(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	var payload struct {
		TargetUserID string `json:"target_user_id"`
		Comment      string `json:"comment"`
	}
	if !bind(c, &payload) {
		return
	}
	err := h.approvalUseCase.Transfer(c.Request.Context(), usecase.TransferApprovalInput{Actor: actor, ID: c.Param("id"), TargetUserID: payload.TargetUserID, Comment: payload.Comment})
	respond(c, http.StatusOK, gin.H{"ok": true}, err)
}
func (h *HTTP) addApprovalSigner(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	var payload struct {
		TargetUserID string `json:"target_user_id"`
		Comment      string `json:"comment"`
	}
	if !bind(c, &payload) {
		return
	}
	err := h.approvalUseCase.AddSigner(c.Request.Context(), usecase.AddSignerInput{Actor: actor, ID: c.Param("id"), TargetUserID: payload.TargetUserID, Comment: payload.Comment})
	respond(c, http.StatusOK, gin.H{"ok": true}, err)
}
func (h *HTTP) listApprovalFlows(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	out, err := h.approvalFlowUseCase.List(c.Request.Context(), usecase.ListApprovalFlowsInput{Actor: actor, Module: c.Query("module")})
	respond(c, http.StatusOK, out, err)
}
func (h *HTTP) saveApprovalFlow(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	var payload struct {
		Module  string                    `json:"module"`
		Name    string                    `json:"name"`
		Enabled bool                      `json:"enabled"`
		Steps   []entity.ApprovalFlowStep `json:"steps"`
	}
	if !bind(c, &payload) {
		return
	}
	status := http.StatusCreated
	if id := c.Param("id"); id != "" {
		status = http.StatusOK
	}
	out, err := h.approvalFlowUseCase.Save(c.Request.Context(), usecase.SaveApprovalFlowInput{Actor: actor, ID: c.Param("id"), Module: payload.Module, Name: payload.Name, Enabled: payload.Enabled, Steps: payload.Steps})
	respond(c, status, out, err)
}
func (h *HTTP) setApprovalFlowEnabled(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	var payload struct {
		Enabled bool `json:"enabled"`
	}
	if !bind(c, &payload) {
		return
	}
	out, err := h.approvalFlowUseCase.SetEnabled(c.Request.Context(), usecase.SetApprovalFlowEnabledInput{Actor: actor, ID: c.Param("id"), Enabled: payload.Enabled})
	respond(c, http.StatusOK, out, err)
}
func (h *HTTP) audit(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	out, err := h.auditUseCase.List(c.Request.Context(), usecase.ListAuditInput{Actor: actor, Page: page})
	respond(c, http.StatusOK, out, err)
}
func (h *HTTP) listNotifications(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	page, size := paging(c)
	out, err := h.notificationUseCase.List(c.Request.Context(), usecase.ListNotificationsInput{Actor: actor, Status: c.Query("status"), Page: page, Size: size})
	respond(c, http.StatusOK, out, err)
}
func (h *HTTP) latestNotifications(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	out, err := h.notificationUseCase.Latest(c.Request.Context(), actor)
	respond(c, http.StatusOK, gin.H{"items": out}, err)
}
func (h *HTTP) notificationCount(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	out, err := h.notificationUseCase.Count(c.Request.Context(), actor)
	respond(c, http.StatusOK, out, err)
}
func (h *HTTP) notificationStream(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		count, err := h.notificationUseCase.Count(c.Request.Context(), actor)
		if err != nil {
			return
		}
		latest, err := h.notificationUseCase.Latest(c.Request.Context(), actor)
		if err != nil {
			return
		}
		c.SSEvent("notification", gin.H{"count": count, "items": latest})
		c.Writer.Flush()
		select {
		case <-c.Request.Context().Done():
			return
		case <-ticker.C:
		}
	}
}
func (h *HTTP) readNotification(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	err := h.notificationUseCase.Read(c.Request.Context(), usecase.ReadNotificationInput{Actor: actor, ID: c.Param("id")})
	respond(c, http.StatusOK, gin.H{"ok": true}, err)
}
func (h *HTTP) readAllNotifications(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	count, err := h.notificationUseCase.ReadAll(c.Request.Context(), actor)
	respond(c, http.StatusOK, gin.H{"count": count}, err)
}
func (h *HTTP) broadcastAnnouncement(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	var payload struct {
		Subject string `json:"subject"`
		Content string `json:"content"`
	}
	if !bind(c, &payload) {
		return
	}
	count, err := h.notificationUseCase.BroadcastAnnouncement(c.Request.Context(), usecase.BroadcastAnnouncementInput{Actor: actor, Subject: payload.Subject, Content: payload.Content})
	respond(c, http.StatusOK, gin.H{"count": count}, err)
}
func (h *HTTP) listAnnouncements(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	out, err := h.notificationUseCase.ListAnnouncements(c.Request.Context(), usecase.ListNotificationsInput{Actor: actor, Page: page, Size: size})
	respond(c, http.StatusOK, out, err)
}
func (h *HTTP) updateAnnouncement(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	var payload struct {
		Subject string `json:"subject"`
		Content string `json:"content"`
	}
	if !bind(c, &payload) {
		return
	}
	err := h.notificationUseCase.UpdateAnnouncement(c.Request.Context(), usecase.AnnouncementInput{Actor: actor, ID: c.Param("id"), Subject: payload.Subject, Content: payload.Content})
	respond(c, http.StatusOK, gin.H{"ok": true}, err)
}
func (h *HTTP) deleteAnnouncement(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	err := h.notificationUseCase.DeleteAnnouncement(c.Request.Context(), usecase.AnnouncementInput{Actor: actor, ID: c.Param("id")})
	respond(c, http.StatusOK, gin.H{"ok": true}, err)
}
func (h *HTTP) search(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	out, err := h.records.Search(c.Request.Context(), usecase.SearchInput{Actor: actor, Query: c.Query("q"), Limit: limit})
	respond(c, http.StatusOK, out, err)
}
func (h *HTTP) analyticsReport(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	out, err := h.records.Report(c.Request.Context(), usecase.ReportInput{Actor: actor, Module: c.Query("module"), GroupBy: c.Query("group_by"), Metric: c.DefaultQuery("metric", "count"), Field: c.Query("field")})
	respond(c, http.StatusOK, out, err)
}
func (h *HTTP) listAttachments(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	out, err := h.attachmentUseCase.List(c.Request.Context(), usecase.ListAttachmentsInput{Actor: actor, Module: c.Param("module"), RecordID: c.Param("id")})
	respond(c, http.StatusOK, gin.H{"items": out}, err)
}
func (h *HTTP) uploadAttachments(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 20<<20)
	form, err := c.MultipartForm()
	if err != nil {
		respond(c, 0, nil, usecase.ErrValidation)
		return
	}
	files := form.File["files"]
	if len(files) == 0 {
		files = form.File["file"]
	}
	if len(files) == 0 || len(files) > 10 {
		respond(c, 0, nil, usecase.ErrValidation)
		return
	}
	items := make([]usecase.AttachmentOutput, 0, len(files))
	for _, file := range files {
		opened, err := file.Open()
		if err != nil {
			respond(c, 0, nil, err)
			return
		}
		content, readErr := io.ReadAll(opened)
		closeErr := opened.Close()
		if readErr != nil || closeErr != nil {
			respond(c, 0, nil, usecase.ErrValidation)
			return
		}
		item, err := h.attachmentUseCase.Upload(c.Request.Context(), usecase.UploadAttachmentInput{Actor: actor, Module: c.Param("module"), RecordID: c.Param("id"), Name: file.Filename, ContentType: file.Header.Get("Content-Type"), Content: content})
		if err != nil {
			respond(c, 0, nil, err)
			return
		}
		items = append(items, item)
	}
	c.JSON(http.StatusCreated, gin.H{"items": items})
}
func (h *HTTP) downloadAttachment(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	out, err := h.attachmentUseCase.Download(c.Request.Context(), usecase.DownloadAttachmentInput{Actor: actor, ID: c.Param("id")})
	if err != nil {
		respond(c, 0, nil, err)
		return
	}
	contentType := out.Attachment.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if c.Query("download") == "1" {
		c.Header("Content-Disposition", "attachment; filename="+strconv.Quote(out.Attachment.Name))
	}
	c.Data(http.StatusOK, contentType, out.Content)
}
func (h *HTTP) deleteAttachment(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	err := h.attachmentUseCase.Delete(c.Request.Context(), usecase.DownloadAttachmentInput{Actor: actor, ID: c.Param("id")})
	respond(c, http.StatusOK, gin.H{"ok": true}, err)
}
func (h *HTTP) listOrganizations(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	out, err := h.systemUseCase.ListOrganizations(c.Request.Context(), usecase.ListOrganizationsInput{Actor: actor})
	respond(c, http.StatusOK, gin.H{"items": out}, err)
}
func (h *HTTP) createOrganization(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	var payload struct {
		Name string `json:"name"`
	}
	if !bind(c, &payload) {
		return
	}
	out, err := h.systemUseCase.CreateOrganization(c.Request.Context(), usecase.CreateOrganizationInput{Actor: actor, Name: payload.Name})
	respond(c, http.StatusCreated, out, err)
}
func (h *HTTP) importOrganizations(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	var payload struct {
		Rows []usecase.OrganizationImportRow `json:"rows"`
	}
	if !bind(c, &payload) {
		return
	}
	out, err := h.systemUseCase.ImportOrganizations(c.Request.Context(), usecase.ImportOrganizationsInput{Actor: actor, Rows: payload.Rows})
	respond(c, http.StatusOK, out, err)
}
func (h *HTTP) updateOrganization(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	var payload struct {
		Name string `json:"name"`
	}
	if !bind(c, &payload) {
		return
	}
	out, err := h.systemUseCase.UpdateOrganization(c.Request.Context(), usecase.UpdateOrganizationInput{Actor: actor, ID: c.Param("id"), Name: payload.Name})
	respond(c, http.StatusOK, out, err)
}
func (h *HTTP) deleteOrganization(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	err := h.systemUseCase.DeleteOrganization(c.Request.Context(), usecase.DeleteOrganizationInput{Actor: actor, ID: c.Param("id")})
	respond(c, http.StatusOK, gin.H{"ok": true}, err)
}
func (h *HTTP) listDepartments(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	out, err := h.systemUseCase.ListDepartments(c.Request.Context(), usecase.ListDepartmentsInput{Actor: actor, OrganizationID: c.Param("id")})
	respond(c, http.StatusOK, gin.H{"items": out}, err)
}
func (h *HTTP) saveDepartment(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	var payload struct {
		ID             string `json:"id"`
		OrganizationID string `json:"organization_id"`
		ParentID       string `json:"parent_id"`
		Name           string `json:"name"`
		Sort           int    `json:"sort"`
	}
	if !bind(c, &payload) {
		return
	}
	if c.Param("id") != "" {
		payload.ID = c.Param("id")
	}
	out, err := h.systemUseCase.SaveDepartment(c.Request.Context(), usecase.SaveDepartmentInput{Actor: actor, ID: payload.ID, OrganizationID: payload.OrganizationID, ParentID: payload.ParentID, Name: payload.Name, Sort: payload.Sort})
	respond(c, http.StatusOK, out, err)
}
func (h *HTTP) deleteDepartment(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	err := h.systemUseCase.DeleteDepartment(c.Request.Context(), usecase.DeleteDepartmentInput{Actor: actor, ID: c.Param("id"), OrganizationID: c.Query("organization_id")})
	respond(c, http.StatusOK, gin.H{"ok": true}, err)
}
func (h *HTTP) listRoles(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	out, err := h.systemUseCase.ListRoles(c.Request.Context(), usecase.ListRolesInput{Actor: actor, OrganizationID: c.Param("id")})
	respond(c, http.StatusOK, gin.H{"items": out}, err)
}
func (h *HTTP) saveRole(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	var payload struct {
		ID                string                             `json:"id"`
		OrganizationID    string                             `json:"organization_id"`
		Name              string                             `json:"name"`
		PermissionIDs     []string                           `json:"permission_ids"`
		DataScope         string                             `json:"data_scope"`
		DepartmentIDs     []string                           `json:"department_ids"`
		FieldRestrictions map[string]entity.FieldRestriction `json:"field_restrictions"`
	}
	if !bind(c, &payload) {
		return
	}
	out, err := h.systemUseCase.SaveRole(c.Request.Context(), usecase.SaveRoleInput{Actor: actor, ID: payload.ID, OrganizationID: payload.OrganizationID, Name: payload.Name, PermissionIDs: payload.PermissionIDs, DataScope: payload.DataScope, DepartmentIDs: payload.DepartmentIDs, FieldRestrictions: payload.FieldRestrictions})
	respond(c, http.StatusOK, out, err)
}

func (h *HTTP) listPermissions(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	out, err := h.systemUseCase.ListPermissions(c.Request.Context(), usecase.ListModulesInput{Actor: actor})
	respond(c, http.StatusOK, out, err)
}
func (h *HTTP) deleteRole(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	err := h.systemUseCase.DeleteRole(c.Request.Context(), usecase.DeleteRoleInput{Actor: actor, ID: c.Param("id"), OrganizationID: c.Query("organization_id")})
	respond(c, http.StatusOK, gin.H{"ok": true}, err)
}
func (h *HTTP) listSystemModules(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	out, err := h.systemUseCase.ListModules(c.Request.Context(), usecase.ListModulesInput{Actor: actor})
	respond(c, http.StatusOK, out, err)
}
func (h *HTTP) saveSystemModule(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	var payload struct {
		ID               string                   `json:"id"`
		Key              string                   `json:"key"`
		Name             string                   `json:"name"`
		Icon             string                   `json:"icon"`
		Enabled          bool                     `json:"enabled"`
		Layout           []entity.FieldLayout     `json:"layout"`
		DocumentTemplate string                   `json:"document_template"`
		SalesStages      []string                 `json:"sales_stages"`
		Stages           []entity.StageDefinition `json:"stages"`
		AfootRollback    bool                     `json:"afoot_rollback"`
		EndRollback      bool                     `json:"end_rollback"`
		CirculationType  string                   `json:"circulation_type"`
		StageTransitions []entity.StageTransition `json:"stage_transitions"`
		Group            string                   `json:"group"`
		Description      string                   `json:"description"`
		Shared           bool                     `json:"shared"`
		Approvable       bool                     `json:"approvable"`
		Custom           bool                     `json:"custom"`
		Fields           []entity.FieldDefinition `json:"fields"`
	}
	if !bind(c, &payload) {
		return
	}
	err := h.systemUseCase.SaveModule(c.Request.Context(), usecase.SaveModuleInput{Actor: actor, Module: entity.ModuleDefinition{ID: payload.ID, Key: payload.Key, Name: payload.Name, Icon: payload.Icon, Group: payload.Group, Description: payload.Description, Enabled: payload.Enabled, Shared: payload.Shared, Approvable: payload.Approvable, Custom: payload.Custom, Fields: payload.Fields, Layout: payload.Layout, DocumentTemplate: payload.DocumentTemplate, SalesStages: payload.SalesStages, Stages: payload.Stages, AfootRollback: payload.AfootRollback, EndRollback: payload.EndRollback, CirculationType: payload.CirculationType, StageTransitions: payload.StageTransitions}})
	respond(c, http.StatusOK, gin.H{"ok": true}, err)
}
func (h *HTTP) deleteSystemModule(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	err := h.systemUseCase.DeleteModule(c.Request.Context(), usecase.DeleteModuleInput{Actor: actor, ID: c.Param("id"), Key: c.Query("key")})
	respond(c, http.StatusOK, gin.H{"ok": true}, err)
}
func paging(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	return page, size
}
func (h *HTTP) listRecords(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	page, size := paging(c)
	out, err := h.records.List(c.Request.Context(), usecase.ListRecordsInput{Actor: actor, Module: c.Param("module"), Query: c.Query("q"), Scope: c.DefaultQuery("scope", "all"), Status: c.Query("status"), Page: page, Size: size})
	respond(c, 200, out, err)
}
func (h *HTTP) exportRecords(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	out, err := h.records.Export(c.Request.Context(), usecase.ExportRecordsInput{Actor: actor, Module: c.Param("module"), Query: c.Query("q"), Scope: c.DefaultQuery("scope", "all"), Status: c.Query("status")})
	if err != nil {
		respond(c, http.StatusOK, nil, err)
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", out.Module.Key+".csv"))
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Status(http.StatusOK)
	writer := csv.NewWriter(c.Writer)
	header := []string{"ID"}
	for _, field := range out.Module.Fields {
		header = append(header, field.Label)
	}
	header = append(header, "负责人")
	_ = writer.Write(header)
	for _, record := range out.Items {
		row := []string{record.ID}
		for _, field := range out.Module.Fields {
			value := fmt.Sprint(record.Data[field.Key])
			if field.Type == "subtable" {
				if encoded, encodeErr := json.Marshal(record.Data[field.Key]); encodeErr == nil {
					value = string(encoded)
				}
			}
			if field.Type == "reference" && record.References[field.Key] != "" {
				value = record.References[field.Key]
			}
			row = append(row, value)
		}
		row = append(row, record.OwnerName)
		_ = writer.Write(row)
	}
	writer.Flush()
}
func (h *HTTP) importTemplate(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	out, err := h.records.ImportTemplate(c.Request.Context(), actor, c.Param("module"))
	if err != nil {
		respond(c, http.StatusOK, nil, err)
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", out.Module.Key+"-template.xlsx"))
	c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", out.Content)
}
func (h *HTTP) createRecord(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	var payload struct{ Data map[string]any }
	if !bind(c, &payload) {
		return
	}
	out, err := h.records.Create(c.Request.Context(), usecase.RecordInput{Actor: actor, Module: c.Param("module"), Data: payload.Data})
	respond(c, 201, out, err)
}
func (h *HTTP) importRecords(c *gin.Context) {
	h.importRecordsWithMode(c, false)
}
func (h *HTTP) previewImportRecords(c *gin.Context) {
	h.importRecordsWithMode(c, true)
}
func (h *HTTP) importRecordsWithMode(c *gin.Context, preview bool) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	if strings.HasPrefix(c.GetHeader("Content-Type"), "multipart/form-data") {
		file, err := c.FormFile("file")
		if err != nil {
			respond(c, 0, nil, usecase.ErrValidation)
			return
		}
		opened, err := file.Open()
		if err != nil {
			respond(c, 0, nil, usecase.ErrValidation)
			return
		}
		defer opened.Close()
		content, err := io.ReadAll(io.LimitReader(opened, 5<<20+1))
		if err != nil || len(content) > 5<<20 {
			respond(c, 0, nil, usecase.ErrValidation)
			return
		}
		var out usecase.ImportRecordsOutput
		if strings.HasSuffix(strings.ToLower(file.Filename), ".xlsx") {
			if preview {
				out, err = h.records.PreviewXLSX(c.Request.Context(), actor, c.Param("module"), content)
			} else {
				out, err = h.records.ImportXLSX(c.Request.Context(), actor, c.Param("module"), content)
			}
		} else if preview {
			out, err = h.records.PreviewCSV(c.Request.Context(), actor, c.Param("module"), content)
		} else {
			out, err = h.records.ImportCSV(c.Request.Context(), actor, c.Param("module"), content)
		}
		respond(c, importResponseStatus(preview), out, err)
		return
	}
	var payload struct {
		Records []map[string]any `json:"records"`
	}
	if !bind(c, &payload) {
		return
	}
	input := usecase.ImportRecordsInput{Actor: actor, Module: c.Param("module"), Records: payload.Records}
	var out usecase.ImportRecordsOutput
	var err error
	if preview {
		out, err = h.records.PreviewImport(c.Request.Context(), input)
	} else {
		out, err = h.records.Import(c.Request.Context(), input)
	}
	respond(c, importResponseStatus(preview), out, err)
}
func importResponseStatus(preview bool) int {
	if preview {
		return http.StatusOK
	}
	return http.StatusCreated
}
func (h *HTTP) getRecord(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	out, err := h.records.Detail(c.Request.Context(), usecase.RecordInput{Actor: actor, Module: c.Param("module"), ID: c.Param("id")})
	respond(c, 200, out, err)
}
func (h *HTTP) listOwnershipHistory(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	out, err := h.records.ListOwnershipHistory(c.Request.Context(), usecase.ListOwnershipHistoryInput{Actor: actor, Module: c.Param("module"), ID: c.Param("id")})
	respond(c, http.StatusOK, out, err)
}
func (h *HTTP) updateRecord(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	var payload struct {
		Data    map[string]any
		Version int
	}
	if !bind(c, &payload) {
		return
	}
	out, err := h.records.Update(c.Request.Context(), usecase.RecordInput{Actor: actor, Module: c.Param("module"), ID: c.Param("id"), Data: payload.Data, Version: payload.Version})
	respond(c, 200, out, err)
}
func (h *HTTP) deleteRecord(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	version, _ := strconv.Atoi(c.Query("version"))
	err := h.records.Delete(c.Request.Context(), usecase.RecordInput{Actor: actor, Module: c.Param("module"), ID: c.Param("id"), Version: version})
	respond(c, 200, gin.H{"ok": true}, err)
}
func (h *HTTP) followup(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	var payload struct {
		Content, Method string
		NextDate        *time.Time `json:"next_date"`
	}
	if !bind(c, &payload) {
		return
	}
	err := h.records.AddFollowup(c.Request.Context(), usecase.FollowupInput{Actor: actor, Module: c.Param("module"), RecordID: c.Param("id"), Content: payload.Content, Method: payload.Method, NextDate: payload.NextDate})
	respond(c, 201, gin.H{"ok": true}, err)
}
func (h *HTTP) listFollowupComments(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	out, err := h.records.ListFollowupComments(c.Request.Context(), usecase.FollowupCommentInput{Actor: actor, Module: c.Param("module"), RecordID: c.Param("id"), FollowupID: c.Param("followupID")})
	respond(c, http.StatusOK, out, err)
}
func (h *HTTP) createFollowupComment(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	var payload struct {
		Content string `json:"content"`
	}
	if !bind(c, &payload) {
		return
	}
	out, err := h.records.CreateFollowupComment(c.Request.Context(), usecase.FollowupCommentInput{Actor: actor, Module: c.Param("module"), RecordID: c.Param("id"), FollowupID: c.Param("followupID"), Content: payload.Content})
	respond(c, http.StatusCreated, out, err)
}
func (h *HTTP) updateFollowupComment(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	var payload struct{ Content string }
	if !bind(c, &payload) {
		return
	}
	out, err := h.records.UpdateFollowupComment(c.Request.Context(), usecase.UpdateFollowupCommentInput{FollowupCommentInput: usecase.FollowupCommentInput{Actor: actor, Module: c.Param("module"), RecordID: c.Param("id"), FollowupID: c.Param("followupID"), Content: payload.Content}, CommentID: c.Param("commentID")})
	respond(c, http.StatusOK, out, err)
}
func (h *HTTP) deleteFollowupComment(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	err := h.records.DeleteFollowupComment(c.Request.Context(), usecase.DeleteFollowupCommentInput{Actor: actor, Module: c.Param("module"), RecordID: c.Param("id"), FollowupID: c.Param("followupID"), ID: c.Param("commentID")})
	respond(c, http.StatusOK, gin.H{"ok": true}, err)
}
func (h *HTTP) listFollowupPlans(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	page, size := paging(c)
	out, err := h.followupPlanUseCase.List(c.Request.Context(), usecase.ListFollowupPlansInput{Actor: actor, Module: c.Param("module"), RecordID: c.Param("id"), Status: c.Query("status"), Mine: c.Query("mine") == "1", Page: page, Size: size})
	respond(c, http.StatusOK, out, err)
}
func (h *HTTP) createFollowupPlan(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	var payload struct {
		OwnerID     string    `json:"owner_id"`
		Content     string    `json:"content"`
		Method      string    `json:"method"`
		EstimatedAt time.Time `json:"estimated_at"`
	}
	if !bind(c, &payload) {
		return
	}
	out, err := h.followupPlanUseCase.Create(c.Request.Context(), usecase.FollowupPlanInput{Actor: actor, Module: c.Param("module"), RecordID: c.Param("id"), OwnerID: payload.OwnerID, Content: payload.Content, Method: payload.Method, EstimatedAt: payload.EstimatedAt})
	respond(c, http.StatusCreated, out, err)
}
func (h *HTTP) updateFollowupPlan(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	var payload struct {
		OwnerID     string    `json:"owner_id"`
		Content     string    `json:"content"`
		Method      string    `json:"method"`
		EstimatedAt time.Time `json:"estimated_at"`
		Convert     bool      `json:"convert"`
	}
	if !bind(c, &payload) {
		return
	}
	out, err := h.followupPlanUseCase.Update(c.Request.Context(), usecase.UpdateFollowupPlanInput{ID: c.Param("id"), Convert: payload.Convert, FollowupPlanInput: usecase.FollowupPlanInput{Actor: actor, OwnerID: payload.OwnerID, Content: payload.Content, Method: payload.Method, EstimatedAt: payload.EstimatedAt}})
	respond(c, http.StatusOK, out, err)
}
func (h *HTTP) updateFollowupPlanStatus(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	var payload struct {
		Status string `json:"status"`
	}
	if !bind(c, &payload) {
		return
	}
	out, err := h.followupPlanUseCase.UpdateStatus(c.Request.Context(), usecase.UpdateFollowupPlanStatusInput{Actor: actor, ID: c.Param("id"), Status: payload.Status})
	respond(c, http.StatusOK, out, err)
}
func (h *HTTP) deleteFollowupPlan(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	err := h.followupPlanUseCase.Delete(c.Request.Context(), actor, c.Param("id"))
	respond(c, http.StatusOK, gin.H{"ok": true}, err)
}
func (h *HTTP) listFollowupPlanComments(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	out, err := h.followupPlanUseCase.ListComments(c.Request.Context(), usecase.ListFollowupPlanCommentsInput{Actor: actor, PlanID: c.Param("id")})
	respond(c, http.StatusOK, out, err)
}
func (h *HTTP) createFollowupPlanComment(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	var payload struct {
		Content string `json:"content"`
	}
	if !bind(c, &payload) {
		return
	}
	out, err := h.followupPlanUseCase.CreateComment(c.Request.Context(), usecase.CreateFollowupPlanCommentInput{Actor: actor, PlanID: c.Param("id"), Content: payload.Content})
	respond(c, http.StatusCreated, out, err)
}
func (h *HTTP) updateFollowupPlanComment(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	var payload struct{ Content string }
	if !bind(c, &payload) {
		return
	}
	out, err := h.followupPlanUseCase.UpdateComment(c.Request.Context(), usecase.UpdateFollowupPlanCommentInput{Actor: actor, PlanID: c.Param("id"), CommentID: c.Param("commentID"), Content: payload.Content})
	respond(c, http.StatusOK, out, err)
}
func (h *HTTP) deleteFollowupPlanComment(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	err := h.followupPlanUseCase.DeleteComment(c.Request.Context(), usecase.DeleteFollowupPlanCommentInput{Actor: actor, PlanID: c.Param("id"), CommentID: c.Param("commentID")})
	respond(c, http.StatusOK, gin.H{"ok": true}, err)
}
func (h *HTTP) action(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	var payload struct {
		Version     int
		OwnerID     *string  `json:"owner_id"`
		ItemIndexes []int    `json:"item_indexes"`
		MergeIDs    []string `json:"merge_ids"`
		Reason      string   `json:"reason"`
		FreezeDays  int      `json:"freeze_days"`
		TargetStage string   `json:"target_stage"`
	}
	if !bind(c, &payload) {
		return
	}
	out, err := h.records.Action(c.Request.Context(), usecase.ActionInput{Actor: actor, Module: c.Param("module"), ID: c.Param("id"), Action: c.Param("action"), Version: payload.Version, OwnerID: payload.OwnerID, ItemIndexes: payload.ItemIndexes, MergeIDs: payload.MergeIDs, Reason: payload.Reason, FreezeDays: payload.FreezeDays, TargetStage: payload.TargetStage})
	respond(c, 200, out, err)
}
