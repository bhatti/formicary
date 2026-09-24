package admin

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/resource"
)

// requireSuccessOrRedirect accepts nil or a "302 u <url>" stub-redirect as success.
// The stub web context returns "302 u <path>" for successful redirects — this is expected.
func requireSuccessOrRedirect(t *testing.T, err error) {
	t.Helper()
	if err != nil && !strings.HasPrefix(err.Error(), "302 ") {
		require.NoError(t, err)
	}
}

func newExecutorAdminController(t *testing.T, terminateErr error) *ExecutionContainerAdminController {
	t.Helper()
	stub := resource.NewStub()
	stub.TerminateError = terminateErr
	return NewExecutionContainerAdminController(stub, web.NewStubWebServer())
}

func newTerminateContext(id, antID, method string) web.APIContext {
	c := web.NewStubContext(&http.Request{URL: &url.URL{}})
	c.Params["id"] = id
	c.Params["antID"] = antID
	c.Params["method"] = method
	return c
}

// ─── deleteExecutionContainer ────────────────────────────────────────────────

func Test_ShouldTerminateExecutorSuccessfully(t *testing.T) {
	ctrl := newExecutorAdminController(t, nil)
	err := ctrl.deleteExecutionContainer(newTerminateContext("pod-abc", "ant-1", string(types.Kubernetes)))
	requireSuccessOrRedirect(t, err)
}

func Test_ShouldReturnErrorWhenTerminationFails(t *testing.T) {
	terminateErr := errors.New("pod not found on ant")
	ctrl := newExecutorAdminController(t, terminateErr)
	err := ctrl.deleteExecutionContainer(newTerminateContext("pod-abc", "ant-1", string(types.Kubernetes)))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pod not found on ant")
}

func Test_ShouldReturnErrorWhenContainerIDMissing(t *testing.T) {
	ctrl := newExecutorAdminController(t, nil)
	err := ctrl.deleteExecutionContainer(newTerminateContext("", "ant-1", string(types.Kubernetes)))
	require.Error(t, err)
}

func Test_ShouldReturnErrorWhenAntIDMissing(t *testing.T) {
	ctrl := newExecutorAdminController(t, nil)
	err := ctrl.deleteExecutionContainer(newTerminateContext("pod-abc", "", string(types.Kubernetes)))
	require.Error(t, err)
}

func Test_ShouldReturnErrorWhenMethodMissing(t *testing.T) {
	ctrl := newExecutorAdminController(t, nil)
	err := ctrl.deleteExecutionContainer(newTerminateContext("pod-abc", "ant-1", ""))
	require.Error(t, err)
}
