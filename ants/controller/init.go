package controller

import (
	"strconv"

	"plexobject.com/formicary/ants/config"
	"plexobject.com/formicary/internal/web"
)

// StartWebServer starts controllers for health metrics
func StartWebServer(antCfg *config.AntConfig, webServer web.Server) {
	NewDefaultController(webServer)
	webServer.Start(":" + strconv.Itoa(antCfg.CommonConfig.HTTPPort))
}
