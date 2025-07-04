package controller

import (
	"plexobject.com/formicary/internal/ant_config"
	"plexobject.com/formicary/queen/controller"
	"strconv"

	"plexobject.com/formicary/internal/web"
)

// StartWebServer starts controllers for health metrics
func StartWebServer(antCfg *ant_config.AntConfig, webServer web.Server) {
	NewDefaultController(webServer)
	if antCfg.Common.Debug {
		controller.NewProfileStatsController(&antCfg.Common, webServer)
	}
	webServer.Start(":" + strconv.Itoa(antCfg.Common.HTTPPort))
}

func StopWebServer(webServer web.Server) {
	webServer.Stop()
}
