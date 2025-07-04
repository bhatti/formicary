package controller

import (
	"context"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/url"
	"plexobject.com/formicary/internal/health"
	"plexobject.com/formicary/internal/queue"
	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/config"
	"strings"
	"testing"
)

func Test_InitializeSwaggerStructsForHealth(t *testing.T) {
	_ = healthQueryParams{}
	_ = metricsQueryResponseBody{}

}

func Test_ShouldQueryHealth(t *testing.T) {
	// GIVEN health controller
	cfg := config.TestServerConfig()
	queueClient, err := queue.NewClientManager().GetClient(context.Background(), &cfg.Common)
	require.NoError(t, err)
	webServer := web.NewStubWebServer()
	heathMonitor, err := health.New(&cfg.Common, queueClient)
	require.NoError(t, err)
	ctrl := NewHealthController(heathMonitor, webServer)

	// WHEN getting health
	reader := io.NopCloser(strings.NewReader(""))
	req := &http.Request{Body: reader, URL: &url.URL{}}
	ctx := web.NewStubContext(req)
	err = ctrl.getHealth(ctx)

	// THEN it should not fail and return valid status
	require.NoError(t, err)
	out := healthQueryResponseBody{}
	out.Body.OverallStatus = ctx.Result.(HealthQueryResponse).OverallStatus
	require.NotNil(t, out.Body.OverallStatus)
}
