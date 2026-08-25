// SPDX-License-Identifier: AGPL-3.0-or-later

package admin

import (
	"encoding/json"
	"net/http"

	"plexobject.com/formicary/internal/acl"
	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/repository"
)

// SlackTokenStatus describes where a token/value comes from (env, db, or unset).
type SlackTokenStatus struct {
	Configured bool   // true if a non-empty value was found
	Source     string // "env" or "db"
}

// slackRoutesAdminController serves /dashboard/slack/routes.
// Admin-only: reads the SlackRoutes SystemConfig and renders the parsed route table.
// Admins get an "Edit" link that opens the raw SystemConfig edit page.
type slackRoutesAdminController struct {
	systemConfigRepo repository.SystemConfigRepository
	serverCfg        *config.ServerConfig
}

// NewSlackRoutesAdminController registers the Slack routes dashboard page.
func NewSlackRoutesAdminController(
	repo repository.SystemConfigRepository,
	webserver web.Server,
) {
	ctrl := &slackRoutesAdminController{systemConfigRepo: repo}
	// SystemConfig Read is admin-only — only admins can view configured routes.
	webserver.GET("/dashboard/slack/routes", ctrl.viewRoutes,
		acl.NewPermission(acl.SystemConfig, acl.Read)).Name = "view_slack_routes"
}

// NewSlackRoutesAdminControllerWithCfg registers the Slack routes dashboard page with server config.
func NewSlackRoutesAdminControllerWithCfg(
	cfg *config.ServerConfig,
	repo repository.SystemConfigRepository,
	webserver web.Server,
) {
	ctrl := &slackRoutesAdminController{systemConfigRepo: repo, serverCfg: cfg}
	webserver.GET("/dashboard/slack/routes", ctrl.viewRoutes,
		acl.NewPermission(acl.SystemConfig, acl.Read)).Name = "view_slack_routes"
}

func (ctrl *slackRoutesAdminController) viewRoutes(c web.APIContext) error {
	routes, configID, err := ctrl.loadRoutes()
	if err != nil {
		return err
	}

	// Build Slack system settings status for the UI.
	appTokenStatus := SlackTokenStatus{}
	botTokenStatus := SlackTokenStatus{}
	channelValue := ""
	if ctrl.serverCfg != nil {
		if ctrl.serverCfg.Slack.AppToken != "" {
			appTokenStatus = SlackTokenStatus{Configured: true, Source: "env"}
		} else if at, e := ctrl.systemConfigRepo.GetByKindName("SLACK", "AppToken"); e == nil && at != nil && at.Value != "" {
			appTokenStatus = SlackTokenStatus{Configured: true, Source: "db"}
		}
		if ctrl.serverCfg.Slack.BotToken != "" {
			botTokenStatus = SlackTokenStatus{Configured: true, Source: "env"}
		} else if bt, e := ctrl.systemConfigRepo.GetByKindName("SLACK", "BotToken"); e == nil && bt != nil && bt.Value != "" {
			botTokenStatus = SlackTokenStatus{Configured: true, Source: "db"}
		}
		channelValue = ctrl.serverCfg.Slack.Channel
	}

	res := map[string]interface{}{
		"Routes":         routes,
		"ConfigID":       configID,
		"AppTokenStatus": appTokenStatus,
		"BotTokenStatus": botTokenStatus,
		"Channel":        channelValue,
	}
	web.RenderDBUserFromSession(c, res)
	return c.Render(http.StatusOK, "slack/routes", res)
}

// loadRoutes reads the SlackRoutes SystemConfig and returns the parsed routes
// plus the SystemConfig ID (for the admin Edit link). Returns empty slice when
// no routes have been configured yet.
func (ctrl *slackRoutesAdminController) loadRoutes() ([]config.SlackRouteConfig, string, error) {
	cfg, err := ctrl.systemConfigRepo.GetByKindName("JSON", "SlackRoutes")
	if err != nil || cfg == nil {
		return nil, "", nil
	}
	var routes []config.SlackRouteConfig
	if err := json.Unmarshal([]byte(cfg.Value), &routes); err != nil {
		// Return raw parse error so the view can show it rather than silently showing nothing.
		return nil, cfg.ID, err
	}
	return routes, cfg.ID, nil
}
