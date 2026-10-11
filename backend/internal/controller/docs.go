package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *HTTP) openAPI(c *gin.Context) {
	importRequest := gin.H{"description": "一次最多导入 500 条；任意记录无效时整批回滚。支持 JSON、CSV 与 XLSX。", "content": gin.H{"application/json": gin.H{"schema": gin.H{"type": "object", "required": []string{"records"}, "properties": gin.H{"records": gin.H{"type": "array", "items": gin.H{"type": "object", "additionalProperties": true}}}}}, "multipart/form-data": gin.H{"schema": gin.H{"type": "object", "required": []string{"file"}, "properties": gin.H{"file": gin.H{"type": "string", "format": "binary"}}}}}}
	paths := gin.H{
		"/mcp":                       gin.H{"post": gin.H{"summary": "MCP JSON-RPC 入口", "security": []gin.H{{"apiKey": []string{}}}}},
		"/api/auth/login":            gin.H{"post": gin.H{"summary": "账号密码登录"}},
		"/api/auth/me":               gin.H{"get": gin.H{"summary": "读取当前用户"}},
		"/api/auth/api-keys":         gin.H{"get": gin.H{"summary": "读取 API Key"}, "post": gin.H{"summary": "创建 API Key"}},
		"/api/catalog":               gin.H{"get": gin.H{"summary": "读取启用模块、字段、布局与销售阶段"}},
		"/api/records/{module}":      gin.H{"get": gin.H{"summary": "分页查询记录"}, "post": gin.H{"summary": "创建记录"}},
		"/api/records/{module}/{id}": gin.H{"get": gin.H{"summary": "读取记录详情"}, "put": gin.H{"summary": "按版本更新记录"}, "delete": gin.H{"summary": "按版本删除记录"}},
		"/api/records/{module}/{id}/actions/{action}": gin.H{"post": gin.H{"summary": "领取、释放、分配、提交审批、交付或拆单"}},
		"/api/records/{module}/import":                gin.H{"post": gin.H{"summary": "原子导入记录", "requestBody": importRequest}},
		"/api/records/{module}/import/preview":        gin.H{"post": gin.H{"summary": "预览导入校验", "requestBody": importRequest}},
		"/api/records/{module}/export":                gin.H{"get": gin.H{"summary": "导出 CSV"}},
		"/api/search":                                 gin.H{"get": gin.H{"summary": "全局搜索"}},
		"/api/approvals":                              gin.H{"get": gin.H{"summary": "审批中心"}},
		"/api/approvals/{id}/decision":                gin.H{"post": gin.H{"summary": "通过或拒绝审批"}},
		"/api/approvals/{id}/transfer":                gin.H{"post": gin.H{"summary": "转签"}},
		"/api/approvals/{id}/add-signer":              gin.H{"post": gin.H{"summary": "加签"}},
		"/api/approval-flows":                         gin.H{"get": gin.H{"summary": "查询审批流"}, "post": gin.H{"summary": "创建审批流版本"}},
		"/api/notifications":                          gin.H{"get": gin.H{"summary": "通知中心"}},
		"/api/notifications/announcements":            gin.H{"post": gin.H{"summary": "管理员发布组织公告"}},
		"/api/notifications/stream":                   gin.H{"get": gin.H{"summary": "订阅通知 SSE"}},
		"/api/agent/tools":                            gin.H{"get": gin.H{"summary": "读取 Agent 工具清单"}},
		"/api/agent/tools/record.search":              gin.H{"post": gin.H{"summary": "按当前用户权限搜索记录"}},
	}
	c.JSON(http.StatusOK, gin.H{"openapi": "3.0.3", "info": gin.H{"title": "Cordys CRM Go API", "version": "0.2.0", "description": "CRM、审批、通知、导入导出与 Agent 接口"}, "security": []gin.H{{"cookieAuth": []string{}}, {"apiKey": []string{}}}, "components": gin.H{"securitySchemes": gin.H{"cookieAuth": gin.H{"type": "apiKey", "in": "cookie", "name": "crm_session"}, "apiKey": gin.H{"type": "apiKey", "in": "header", "name": "X-API-Key"}}}, "paths": paths})
}

func (h *HTTP) docs(c *gin.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, `<!doctype html><html lang="zh-CN"><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>Cordys CRM API</title><style>body{font:14px/1.7 system-ui;margin:0;background:#f5f7fb;color:#26354d}main{max-width:960px;margin:32px auto;padding:32px;background:#fff;border-radius:12px}h1{margin-top:0}code{background:#eef3fb;padding:2px 6px;border-radius:4px}table{width:100%;border-collapse:collapse;margin:18px 0}th,td{text-align:left;padding:10px;border-bottom:1px solid #e8edf5}th{color:#66758c}a{color:#426fc4}</style><main><h1>Cordys CRM API</h1><p>完整 OpenAPI 3.0 描述：<a href="/api/openapi.json">/api/openapi.json</a></p><p>业务接口支持 HttpOnly 会话 Cookie 或 <code>X-API-Key</code>；MCP 仅使用 API Key。</p><h2>核心接口</h2><table><tr><th>范围</th><th>入口</th><th>说明</th></tr><tr><td>记录</td><td><code>/api/records/{module}</code></td><td>查询、创建、更新、删除、导入、导出与动作</td></tr><tr><td>审批</td><td><code>/api/approvals</code></td><td>决定、撤回、转签、加签与版本化流程</td></tr><tr><td>通知</td><td><code>/api/notifications</code></td><td>通知、公告、未读统计与 SSE</td></tr><tr><td>Agent</td><td><code>/api/agent/tools</code></td><td>继承当前用户权限的工具调用</td></tr><tr><td>MCP</td><td><code>/mcp</code></td><td>initialize、tools/list、tools/call</td></tr></table><h2>导入约束</h2><p>JSON、CSV、XLSX 每批最多 500 条，先调用 <code>/import/preview</code> 验证；正式导入任一记录失败时整批回滚。</p></main></html>`)
}
