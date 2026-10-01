// SPDX-License-Identifier: AGPL-3.0-or-later

package controller

import (
	"net/http"
	"strconv"
	"time"

	"plexobject.com/formicary/internal/acl"
	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/repository"
)

// logRecord is the JSON shape returned by the log query endpoint.
type logRecord struct {
	ID        string `json:"id"`
	CreatedAt string `json:"created_at"`
	Message   string `json:"message"`
	Level     string `json:"level"`
	TaskType  string `json:"task_type"`
	AntID     string `json:"ant_id"`
}

// LogEventController exposes per-job log queries to the owner of the job.
type LogEventController struct {
	logRepo   repository.LogEventRepository
	webserver web.Server
}

// NewLogEventController registers the log query route and returns the controller.
func NewLogEventController(
	logRepo repository.LogEventRepository,
	webserver web.Server,
) *LogEventController {
	ctrl := &LogEventController{
		logRepo:   logRepo,
		webserver: webserver,
	}
	// GET /api/v1/jobs/requests/:id/logs — returns archived log events for a job request.
	// Accessible to the owner of the job (JobRequest View permission).
	webserver.GET("/api/v1/jobs/requests/:id/logs", ctrl.queryLogs,
		acl.NewPermission(acl.JobRequest, acl.View)).Name = "job_request_logs"
	return ctrl
}

// queryLogs handles GET /api/v1/jobs/requests/:id/logs
// Query params: since (RFC3339), limit (1-2000, default 500), level (info|warn|error)
func (ctrl *LogEventController) queryLogs(c web.APIContext) error {
	requestID := c.Param("id")
	if requestID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "request id is required"})
	}

	limit := 500
	if s := c.QueryParam("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			if n > 2000 {
				n = 2000
			}
			limit = n
		}
	}

	qc := web.BuildQueryContext(c)

	params := map[string]interface{}{"job_request_id": requestID}
	// Scope to the requesting user when auth is enabled; empty userID means auth is off.
	if uid := qc.GetUserID(); uid != "" {
		params["user_id"] = uid
	}

	if level := c.QueryParam("level"); level != "" {
		params["level"] = level
	}

	if since := c.QueryParam("since"); since != "" {
		if _, err := time.Parse(time.RFC3339, since); err == nil {
			params["since"] = since
		}
	}

	records, total, err := ctrl.logRepo.Query(params, 0, limit, []string{"created_at asc"})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	out := make([]logRecord, 0, len(records))
	for _, r := range records {
		out = append(out, logRecord{
			ID:        r.ID,
			CreatedAt: r.CreatedAt.Format(time.RFC3339),
			Message:   r.Message,
			Level:     r.Level,
			TaskType:  r.TaskType,
			AntID:     r.AntID,
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"records": out,
		"total":   total,
	})
}
