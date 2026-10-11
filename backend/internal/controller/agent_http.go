package controller

import (
	"net/http"

	"cordialcrm/backend/internal/usecase"
	"github.com/gin-gonic/gin"
)

type AgentToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

func (h *HTTP) listAgentTools(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	if !usecase.HasPermission(actor, "AGENT:READ") {
		respond(c, 0, nil, usecase.ErrForbidden)
		return
	}
	c.JSON(http.StatusOK, gin.H{"tools": agentToolDefinitions()})
}

func agentToolDefinitions() []AgentToolDefinition {
	return []AgentToolDefinition{
		{Name: "record.search", Description: "在当前用户有权限的 CRM 记录中搜索名称和字段内容。", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"query": map[string]any{"type": "string", "description": "搜索关键词"}, "limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 100}}, "required": []string{"query"}}},
		{Name: "record.list", Description: "分页读取指定 CRM 模块中当前用户可见的记录。", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"module": map[string]any{"type": "string"}, "query": map[string]any{"type": "string"}, "scope": map[string]any{"type": "string", "enum": []string{"all", "mine", "pool"}}, "status": map[string]any{"type": "string"}, "page": map[string]any{"type": "integer", "minimum": 1}, "size": map[string]any{"type": "integer", "minimum": 1, "maximum": 100}}, "required": []string{"module"}}},
		{Name: "record.get", Description: "读取一条当前用户有权限访问的 CRM 记录。", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"module": map[string]any{"type": "string"}, "id": map[string]any{"type": "string"}}, "required": []string{"module", "id"}}},
		{Name: "followup-plan.list", Description: "读取一条记录的跟进计划。", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"module": map[string]any{"type": "string"}, "record_id": map[string]any{"type": "string"}, "status": map[string]any{"type": "string"}, "mine": map[string]any{"type": "boolean"}, "page": map[string]any{"type": "integer", "minimum": 1}, "size": map[string]any{"type": "integer", "minimum": 1, "maximum": 100}}, "required": []string{"module", "record_id"}}},
	}
}

func (h *HTTP) listAgents(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	out, err := h.agents.List(c.Request.Context(), actor)
	respond(c, http.StatusOK, gin.H{"items": out}, err)
}

func (h *HTTP) saveAgent(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	var payload struct {
		Name      string   `json:"name"`
		Prompt    string   `json:"prompt"`
		ToolNames []string `json:"tool_names"`
		Enabled   bool     `json:"enabled"`
	}
	if !bind(c, &payload) {
		return
	}
	out, err := h.agents.Save(c.Request.Context(), usecase.AgentInput{Actor: actor, ID: c.Param("id"), Name: payload.Name, Prompt: payload.Prompt, ToolNames: payload.ToolNames, Enabled: payload.Enabled})
	status := http.StatusOK
	if c.Param("id") == "" {
		status = http.StatusCreated
	}
	respond(c, status, out, err)
}

func (h *HTTP) deleteAgent(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	err := h.agents.Delete(c.Request.Context(), actor, c.Param("id"))
	respond(c, http.StatusOK, gin.H{"ok": true}, err)
}

func (h *HTTP) agentRecordList(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	if !usecase.HasPermission(actor, "AGENT:READ") {
		respond(c, 0, nil, usecase.ErrForbidden)
		return
	}
	var payload struct {
		Module string `json:"module"`
		Query  string `json:"query"`
		Scope  string `json:"scope"`
		Status string `json:"status"`
		Page   int    `json:"page"`
		Size   int    `json:"size"`
	}
	if !bind(c, &payload) {
		return
	}
	out, err := h.recordTool.List(c.Request.Context(), usecase.ListRecordsInput{Actor: actor, Module: payload.Module, Query: payload.Query, Scope: payload.Scope, Status: payload.Status, Page: payload.Page, Size: payload.Size})
	respond(c, http.StatusOK, out, err)
}

func (h *HTTP) agentRecordGet(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	if !usecase.HasPermission(actor, "AGENT:READ") {
		respond(c, 0, nil, usecase.ErrForbidden)
		return
	}
	var payload struct {
		Module string `json:"module"`
		ID     string `json:"id"`
	}
	if !bind(c, &payload) {
		return
	}
	out, err := h.recordTool.Get(c.Request.Context(), usecase.RecordInput{Actor: actor, Module: payload.Module, ID: payload.ID})
	respond(c, http.StatusOK, out, err)
}

func (h *HTTP) agentFollowupPlanList(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	if !usecase.HasPermission(actor, "AGENT:READ") {
		respond(c, 0, nil, usecase.ErrForbidden)
		return
	}
	var payload struct {
		Module   string `json:"module"`
		RecordID string `json:"record_id"`
		Status   string `json:"status"`
		Mine     bool   `json:"mine"`
		Page     int    `json:"page"`
		Size     int    `json:"size"`
	}
	if !bind(c, &payload) {
		return
	}
	out, err := h.followupPlanTool.List(c.Request.Context(), usecase.ListFollowupPlansInput{Actor: actor, Module: payload.Module, RecordID: payload.RecordID, Status: payload.Status, Mine: payload.Mine, Page: payload.Page, Size: payload.Size})
	respond(c, http.StatusOK, out, err)
}

func (h *HTTP) agentRecordSearch(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	if !usecase.HasPermission(actor, "AGENT:READ") {
		respond(c, 0, nil, usecase.ErrForbidden)
		return
	}
	var payload struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if !bind(c, &payload) {
		return
	}
	out, err := h.recordTool.Search(c.Request.Context(), usecase.SearchInput{Actor: actor, Query: payload.Query, Limit: payload.Limit})
	respond(c, http.StatusOK, out, err)
}
