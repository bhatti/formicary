package controller

import (
	"net/http"
	"net/http/pprof"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/acl"
	"plexobject.com/formicary/internal/health"
	"plexobject.com/formicary/internal/web"
)

// HealthController structure
type HealthController struct {
	heathMonitor *health.Monitor
	webserver    web.Server
}

// NewHealthController instantiates controller for updating artifacts
func NewHealthController(
	heathMonitor *health.Monitor,
	webserver web.Server) *HealthController {
	h := &HealthController{
		heathMonitor: heathMonitor,
		webserver:    webserver,
	}
	webserver.GET("/api/health", h.getHealth, acl.NewPermission(acl.Health, acl.Query)).Name = "get_health"
	webserver.GET("/api/metrics", web.WrapHandler(promhttp.Handler()), acl.NewPermission(acl.Health, acl.Metrics))
	// Check runtime.SetBlockProfileRate, runtime.SetMutexProfileFraction, go tool pprof.
	webserver.GET("/api/pprof", func(c web.APIContext) error {
		pprof.Profile(c.Response(), c.Request())
		return nil
	}, acl.NewPermission(acl.Profile, acl.View))
	if err := prometheus.Register(prometheus.NewBuildInfoCollector()); err != nil {
		logrus.WithFields(logrus.Fields{
			"Component": "HealthController",
			"Error":     err,
		}).Error("failed to register prometheus collector")
	}
	return h
}

// ********************************* HTTP Handlers ***********************************

// Returns prometheus health metrics.
// `This requires admin access`
// responses:
//   200: metricsQueryResponse

// Returns health status.
// `This requires admin access`
// responses:
//   200: healthQueryResponse
func (h *HealthController) getHealth(c web.APIContext) error {
	overall, statuses := h.heathMonitor.GetAllStatuses()
	resp := HealthQueryResponse{
		OverallStatus:            overall,
		DependentServiceStatuses: statuses,
	}
	if overall.Healthy() {
		return c.JSON(http.StatusOK, resp)
	}
	return c.JSON(http.StatusFailedDependency, resp)
}

// ********************************* Swagger types ***********************************

// The params for health status and metrics
type healthQueryParams struct {
	// in:query
}

// HealthQueryResponse defines health Status for overall and dependent services.
type HealthQueryResponse struct {
	OverallStatus            *health.ServiceStatus   `json:"overall_status"`
	DependentServiceStatuses []*health.ServiceStatus `json:"dependent_service_statuses"`
}

// Results of health-status
type healthQueryResponseBody struct {
	// in:body
	Body HealthQueryResponse
}

// Results of prometheus-metrics
type metricsQueryResponseBody struct {
	// in:body
	Body []string
}
