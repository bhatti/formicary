package admin

import (
	"net/http"
	"time"

	"plexobject.com/formicary/internal/acl"
	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/manager"

	"github.com/labstack/echo/v4"
)

// RetentionAdminController exposes an on-demand purge endpoint for ops use.
type RetentionAdminController struct {
	retentionManager *manager.RetentionManager
	webserver        web.Server
}

// NewRetentionAdminController registers the retention purge route.
func NewRetentionAdminController(
	retentionManager *manager.RetentionManager,
	webserver web.Server,
) *RetentionAdminController {
	ctrl := &RetentionAdminController{
		retentionManager: retentionManager,
		webserver:        webserver,
	}
	webserver.POST("/api/admin/jobs/retention/purge", ctrl.purge,
		acl.NewPermission(acl.JobRequest, acl.Delete)).Name = "admin_retention_purge"
	return ctrl
}

// purge triggers an immediate retention purge across all job definitions.
// swagger:route POST /api/admin/jobs/retention/purge admin-retention purgeRetention
// Triggers an immediate history retention purge across all job definitions.
// Responses:
//
//	200: retentionPurgeResponse
func (ctrl *RetentionAdminController) purge(c web.APIContext) error {
	start := time.Now()
	total, err := ctrl.retentionManager.PurgeAll(c.Request().Context())
	if err != nil {
		return &echo.HTTPError{Code: http.StatusInternalServerError, Message: err.Error()}
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"purged":      total,
		"duration_ms": time.Since(start).Milliseconds(),
	})
}
