// SPDX-License-Identifier: AGPL-3.0-or-later

package admin

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/acl"
	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/repository"
)

// LogAdminController handles admin-only log viewing for queen and ant workers.
type LogAdminController struct {
	logRepo   repository.LogEventRepository
	webserver web.Server
}

// NewLogAdminController registers log admin routes and returns the controller.
func NewLogAdminController(
	logRepo repository.LogEventRepository,
	webserver web.Server,
) *LogAdminController {
	ctrl := &LogAdminController{
		logRepo:   logRepo,
		webserver: webserver,
	}
	webserver.GET("/dashboard/reports/logs", ctrl.queenLogs, acl.NewPermission(acl.Report, acl.Query)).Name = "admin_queen_logs"
	webserver.GET("/dashboard/ants/:id/logs", ctrl.antLogs, acl.NewPermission(acl.AntExecutor, acl.Query)).Name = "admin_ant_logs"
	return ctrl
}

func (ctrl *LogAdminController) queenLogs(c web.APIContext) error {
	// source=system → fetch live pod logs from kubernetes instead of the DB.
	if c.QueryParam("source") == "system" {
		return ctrl.renderKubeLogs(c)
	}
	params := buildLogQueryParams(c, "")
	return ctrl.renderLogs(c, params, "logs/index", "System Logs")
}

func (ctrl *LogAdminController) renderKubeLogs(c web.APIContext) error {
	limit := clampLogLimit(c.QueryParam("limit"))
	level := c.QueryParam("level")
	if level == "" {
		level = "info"
	}
	q := strings.ToLower(c.QueryParam("q"))

	lines, err := fetchKubeLogs(c.Request().Context(), int64(limit))
	if err != nil {
		logrus.WithError(err).Warn("failed to fetch kubernetes pod logs")
	}

	lines = filterByLevel(lines, level)

	if q != "" {
		filtered := lines[:0]
		for _, l := range lines {
			if strings.Contains(strings.ToLower(l.Raw), q) {
				filtered = append(filtered, l)
			}
		}
		lines = filtered
	}

	res := map[string]interface{}{
		"Title":    "System Logs (pod)",
		"Lines":    lines,
		"Count":    len(lines),
		"Level":    level,
		"PageSize": limit,
		"Source":   "system",
		"Q":        c.QueryParam("q"),
	}
	if err != nil {
		res["Error"] = err.Error()
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "logs/system", res)
}

func (ctrl *LogAdminController) antLogs(c web.APIContext) error {
	antID := c.Param("id")
	// Registration IDs are "hostname@orgID" but log events store only the hostname.
	if idx := strings.Index(antID, "@"); idx != -1 {
		antID = antID[:idx]
	}
	params := buildLogQueryParams(c, antID)
	return ctrl.renderLogs(c, params, "logs/ant", "Ant Logs: "+antID)
}

func (ctrl *LogAdminController) renderLogs(c web.APIContext, params map[string]interface{}, tmpl string, title string) error {
	page := 0
	pageSize := clampLogLimit(c.QueryParam("limit"))
	if p, err := strconv.Atoi(c.QueryParam("page")); err == nil && p > 0 {
		page = p
	}
	records, total, err := ctrl.logRepo.Query(params, page, pageSize, []string{"created_at desc"})
	if err != nil {
		// If the level/source column doesn't exist (EC2 schema migration pending),
		// retry without those filters so the page still shows data.
		delete(params, "level")
		delete(params, "source")
		records, total, err = ctrl.logRepo.Query(params, page, pageSize, []string{"created_at desc"})
		if err != nil {
			records = nil
			total = 0
		}
	}
	totalPages := int64(0)
	if pageSize > 0 {
		totalPages = (total + int64(pageSize) - 1) / int64(pageSize)
	}
	res := make(map[string]interface{})
	res["Title"] = title
	res["Records"] = records
	res["TotalRecords"] = total
	res["Page"] = page
	res["PageSize"] = pageSize
	res["TotalPages"] = totalPages
	res["Level"] = params["level"]
	res["Source"] = params["source"]
	res["AntID"] = params["ant_id"]
	res["Since"] = c.QueryParam("since")
	res["Q"] = c.QueryParam("q")
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, tmpl, res)
}

func buildLogQueryParams(c web.APIContext, antID string) map[string]interface{} {
	params := map[string]interface{}{}
	if level := c.QueryParam("level"); level != "" {
		params["level"] = level
	} else {
		params["level"] = "info" // default: info+
	}
	if since := c.QueryParam("since"); since != "" {
		params["since"] = since
	} else {
		params["since"] = time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
	}
	if source := c.QueryParam("source"); source != "" {
		params["source"] = source
	}
	if q := c.QueryParam("q"); q != "" {
		params["q"] = q
	}
	if antID != "" {
		params["ant_id"] = antID
	} else if aid := c.QueryParam("ant_id"); aid != "" {
		params["ant_id"] = aid
	}
	return params
}

func clampLogLimit(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 500
	}
	if n > 1000 {
		return 1000
	}
	return n
}
