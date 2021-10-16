package admin

import (
	"net/http"

	"plexobject.com/formicary/internal/acl"
	"plexobject.com/formicary/internal/health"
	"plexobject.com/formicary/internal/web"
)

// HealthAdminController structure
type HealthAdminController struct {
	heathMonitor *health.Monitor
	webserver    web.Server
}

// NewHealthAdminController admin dashboard for managing artifacts -- admin only
func NewHealthAdminController(
	heathMonitor *health.Monitor,
	webserver web.Server) *HealthAdminController {
	h := &HealthAdminController{
		heathMonitor: heathMonitor,
		webserver:    webserver,
	}
	webserver.GET("/dashboard/health", h.getHealth, acl.NewPermission(acl.Health, acl.View)).Name = "get_admin_health"
	return h
}

// ********************************* HTTP Handlers ***********************************
// getHealth - queries artifact
func (h *HealthAdminController) getHealth(c web.APIContext) error {
	overall, statuses := h.heathMonitor.GetAllStatuses()
	res := map[string]interface{}{"OverallStatus": overall, "Statuses": statuses}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "health/index", res)
}
