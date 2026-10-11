package controller

import (
	"net/http"

	"cordialcrm/backend/internal/entity"
	"cordialcrm/backend/internal/usecase"
	"github.com/gin-gonic/gin"
)

func (h *HTTP) listDashboardConfigs(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	out, err := h.dashboards.List(c.Request.Context(), actor)
	respond(c, http.StatusOK, gin.H{"items": out}, err)
}

func (h *HTTP) saveDashboardConfig(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	var payload struct {
		Name      string                   `json:"name"`
		Widgets   []entity.DashboardWidget `json:"widgets"`
		IsDefault bool                     `json:"is_default"`
	}
	if !bind(c, &payload) {
		return
	}
	out, err := h.dashboards.Save(c.Request.Context(), usecase.DashboardConfigInput{Actor: actor, ID: c.Param("id"), Name: payload.Name, Widgets: payload.Widgets, IsDefault: payload.IsDefault})
	status := http.StatusOK
	if c.Param("id") == "" {
		status = http.StatusCreated
	}
	respond(c, status, out, err)
}

func (h *HTTP) deleteDashboardConfig(c *gin.Context) {
	actor, ok := h.actor(c)
	if !ok {
		return
	}
	err := h.dashboards.Delete(c.Request.Context(), actor, c.Param("id"))
	respond(c, http.StatusOK, gin.H{"ok": true}, err)
}
