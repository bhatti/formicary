package controller

import (
	"net/http"
	"plexobject.com/formicary/internal/acl"
	"runtime/debug"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/web"
)

// DefaultController structure
type DefaultController struct {
	webserver web.Server
}

// NewDefaultController instantiates default controller for ant
func NewDefaultController(
	webserver web.Server) *DefaultController {
	ctrl := &DefaultController{
		webserver: webserver,
	}
	webserver.GET("/", ctrl.health, acl.NewPermission(acl.Health, acl.Metrics)).Name = "ant_health"
	webserver.GET("/metrics", web.WrapHandler(promhttp.Handler()), acl.NewPermission(acl.Health, acl.Metrics))
	if err := prometheus.Register(prometheus.NewBuildInfoCollector()); err != nil {
		debug.PrintStack()
		logrus.WithFields(logrus.Fields{
			"Component": "AntController",
			"Error":     err,
		}).Error("failed to register prometheus collector")
	}
	return ctrl
}

// ********************************* HTTP Handlers ***********************************
// health - information about ant
func (ctrl *DefaultController) health(c web.APIContext) error {
	res := map[string]interface{}{"HEALTH": "GOOD"}
	return c.JSON(http.StatusOK, res)
}
