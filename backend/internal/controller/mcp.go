package controller

import (
	"encoding/json"
	"net/http"

	"cordialcrm/backend/internal/usecase"
	"github.com/gin-gonic/gin"
)

type mcpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

func (h *HTTP) mcp(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	if !usecase.HasPermission(actor, "AGENT:READ") {
		respond(c, 0, nil, usecase.ErrForbidden)
		return
	}
	var request mcpRequest
	if err := c.ShouldBindJSON(&request); err != nil || request.JSONRPC != "2.0" || request.Method == "" {
		h.mcpError(c, request.ID, -32600, "无效的 JSON-RPC 请求")
		return
	}
	if len(request.ID) == 0 {
		c.Status(http.StatusAccepted)
		return
	}
	switch request.Method {
	case "initialize":
		h.mcpResult(c, request.ID, gin.H{"protocolVersion": "2024-11-05", "capabilities": gin.H{"tools": gin.H{}}, "serverInfo": gin.H{"name": "cordys-crm", "version": "0.1.0"}})
	case "tools/list":
		tools := make([]gin.H, 0, len(agentToolDefinitions()))
		for _, tool := range agentToolDefinitions() {
			tools = append(tools, gin.H{"name": tool.Name, "description": tool.Description, "inputSchema": tool.InputSchema})
		}
		h.mcpResult(c, request.ID, gin.H{"tools": tools})
	case "tools/call":
		result, err := h.mcpCall(c, actor, request.Params)
		if err != nil {
			h.mcpResult(c, request.ID, gin.H{"content": []gin.H{{"type": "text", "text": err.Error()}}, "isError": true})
			return
		}
		encoded, err := json.Marshal(result)
		if err != nil {
			h.mcpResult(c, request.ID, gin.H{"content": []gin.H{{"type": "text", "text": "工具结果序列化失败"}}, "isError": true})
			return
		}
		h.mcpResult(c, request.ID, gin.H{"content": []gin.H{{"type": "text", "text": string(encoded)}}})
	default:
		h.mcpError(c, request.ID, -32601, "方法不存在")
	}
}

func (h *HTTP) mcpCall(c *gin.Context, actor usecase.UserOutput, raw json.RawMessage) (any, error) {
	var request struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(raw, &request); err != nil || request.Name == "" {
		return nil, usecase.ErrValidation
	}
	switch request.Name {
	case "record.search":
		var input struct {
			Query string `json:"query"`
			Limit int    `json:"limit"`
		}
		if err := json.Unmarshal(request.Arguments, &input); err != nil {
			return nil, usecase.ErrValidation
		}
		return h.recordTool.Search(c.Request.Context(), usecase.SearchInput{Actor: actor, Query: input.Query, Limit: input.Limit})
	case "record.list":
		var input struct {
			Module string `json:"module"`
			Query  string `json:"query"`
			Scope  string `json:"scope"`
			Status string `json:"status"`
			Page   int    `json:"page"`
			Size   int    `json:"size"`
		}
		if err := json.Unmarshal(request.Arguments, &input); err != nil {
			return nil, usecase.ErrValidation
		}
		return h.recordTool.List(c.Request.Context(), usecase.ListRecordsInput{Actor: actor, Module: input.Module, Query: input.Query, Scope: input.Scope, Status: input.Status, Page: input.Page, Size: input.Size})
	case "record.get":
		var input struct {
			Module string `json:"module"`
			ID     string `json:"id"`
		}
		if err := json.Unmarshal(request.Arguments, &input); err != nil {
			return nil, usecase.ErrValidation
		}
		return h.recordTool.Get(c.Request.Context(), usecase.RecordInput{Actor: actor, Module: input.Module, ID: input.ID})
	case "followup-plan.list":
		var input struct {
			Module   string `json:"module"`
			RecordID string `json:"record_id"`
			Status   string `json:"status"`
			Mine     bool   `json:"mine"`
			Page     int    `json:"page"`
			Size     int    `json:"size"`
		}
		if err := json.Unmarshal(request.Arguments, &input); err != nil {
			return nil, usecase.ErrValidation
		}
		return h.followupPlanTool.List(c.Request.Context(), usecase.ListFollowupPlansInput{Actor: actor, Module: input.Module, RecordID: input.RecordID, Status: input.Status, Mine: input.Mine, Page: input.Page, Size: input.Size})
	default:
		return nil, usecase.ErrNotFound
	}
}

func (h *HTTP) mcpResult(c *gin.Context, id json.RawMessage, result any) {
	c.JSON(http.StatusOK, gin.H{"jsonrpc": "2.0", "id": json.RawMessage(id), "result": result})
}

func (h *HTTP) mcpError(c *gin.Context, id json.RawMessage, code int, message string) {
	c.JSON(http.StatusBadRequest, gin.H{"jsonrpc": "2.0", "id": json.RawMessage(id), "error": gin.H{"code": code, "message": message}})
}
