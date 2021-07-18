package http

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/ants/config"
	"plexobject.com/formicary/ants/executor"
	"plexobject.com/formicary/ants/executor/tests"
	"plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/web"
	"testing"
	"time"
)

var httpMethods = []types.TaskMethod{
	types.HTTPGet,
	types.HTTPPostForm,
	types.HTTPPostJSON,
	types.HTTPPutJSON,
	types.HTTPDelete,
}

var goodEndpoints = []string{
	"https://jsonplaceholder.typicode.com/todos/1",
	"https://jsonplaceholder.typicode.com/todos",
	"https://jsonplaceholder.typicode.com/todos",
	"https://jsonplaceholder.typicode.com/todos/1",
	"https://jsonplaceholder.typicode.com/todos/1",
}

var errorEndpoints = []string{
	"https://jsonplaceholder.typicode.com/errors/1",
	"https://jsonplaceholder.typicode.com/errors",
	"https://jsonplaceholder.typicode.com/errors",
	"https://jsonplaceholder.typicode.com/errors/1",
	"https://jsonplaceholder.typicode.com/errors/1",
}

var timeoutEndpoints = []string{
	"https://jsonplaceholder.typicode.com/timeout/1",
	"https://jsonplaceholder.typicode.com/timeout",
	"https://jsonplaceholder.typicode.com/timeout",
	"https://jsonplaceholder.typicode.com/timeout/1",
	"https://jsonplaceholder.typicode.com/timeout/1",
}

func newProviders(c *config.AntConfig, method types.TaskMethod) (map[string]executor.Provider, error) {
	httpClient := web.NewStubHTTPClient()
	httpClient.GetMapping["https://jsonplaceholder.typicode.com/todos/1"] = web.NewStubHTTPResponse(200, "test get bin response")
	httpClient.PostMapping["https://jsonplaceholder.typicode.com/todos"] = web.NewStubHTTPResponse(200, "test post bin response")
	httpClient.PutMapping["https://jsonplaceholder.typicode.com/todos/1"] = web.NewStubHTTPResponse(200, "test put bin response")
	httpClient.DeleteMapping["https://jsonplaceholder.typicode.com/todos/1"] = web.NewStubHTTPResponse(200, "test delete bin response")

	httpClient.GetMapping["https://jsonplaceholder.typicode.com/errors/1"] = web.NewStubHTTPResponseError(500, 0, fmt.Errorf("random error"))
	httpClient.PostMapping["https://jsonplaceholder.typicode.com/errors"] = web.NewStubHTTPResponseError(500, 0, fmt.Errorf("random error"))
	httpClient.PutMapping["https://jsonplaceholder.typicode.com/errors/1"] = web.NewStubHTTPResponseError(500, 0, fmt.Errorf("random error"))
	httpClient.DeleteMapping["https://jsonplaceholder.typicode.com/errors/1"] = web.NewStubHTTPResponseError(500, 0, fmt.Errorf("random error"))

	httpClient.GetMapping["https://jsonplaceholder.typicode.com/timeout/1"] = web.NewStubHTTPResponseError(500, 10*time.Second, fmt.Errorf("timeout error"))
	httpClient.PostMapping["https://jsonplaceholder.typicode.com/timeout"] = web.NewStubHTTPResponseError(500, 10*time.Second, fmt.Errorf("timeout error"))
	httpClient.PutMapping["https://jsonplaceholder.typicode.com/timeout/1"] = web.NewStubHTTPResponseError(500, 10*time.Second, fmt.Errorf("timeout error"))
	httpClient.DeleteMapping["https://jsonplaceholder.typicode.com/timeout/1"] = web.NewStubHTTPResponseError(500, 10*time.Second, fmt.Errorf("timeout error"))

	httpProvider, err := NewExecutorProvider(c, httpClient)
	if err != nil {
		return nil, err
	}
	return map[string]executor.Provider{
		string(method): httpProvider,
	}, nil
}

func newConfig() *config.AntConfig {
	c := config.AntConfig{}
	c.OutputLimit = 64 * 1024 * 1024
	return &c
}

func Test_ShouldExecuteWithSimpleList(t *testing.T) {
	for i, method := range httpMethods {
		providers, err := newProviders(newConfig(), method)
		require.NoError(t, err)
		opts := types.NewExecutorOptions("http-"+string(method), method)
		tests.DoTestExecuteWithSimpleList(t, providers, opts, goodEndpoints[i])
	}
}

func Test_ShouldExecuteWithTimeout(t *testing.T) {
	for i, method := range httpMethods {
		providers, err := newProviders(newConfig(), method)
		require.NoError(t, err)
		opts := types.NewExecutorOptions("http-"+string(method), method)
		tests.DoTestExecuteWithTimeout(t, providers, opts, timeoutEndpoints[i])
	}
}

func Test_ShouldExecuteWithBadCommand(t *testing.T) {
	for i, method := range httpMethods {
		providers, err := newProviders(newConfig(), method)
		require.NoError(t, err)
		opts := types.NewExecutorOptions("http-"+string(method), method)
		tests.DoTestExecuteWithBadCommand(t, providers, opts, errorEndpoints[i])
	}
}

func Test_ShouldListExecutors(t *testing.T) {
	for i, method := range httpMethods {
		providers, err := newProviders(newConfig(), method)
		require.NoError(t, err)
		opts := types.NewExecutorOptions("http-"+string(method), method)
		tests.DoTestListExecutors(t, providers, opts, goodEndpoints[i])
	}
}

func Test_ShouldGetRuntime(t *testing.T) {
	for _, method := range httpMethods {
		providers, err := newProviders(newConfig(), method)
		require.NoError(t, err)
		opts := types.NewExecutorOptions("http-"+string(method), method)
		tests.DoTestGetRuntime(t, providers, opts)
	}
}
