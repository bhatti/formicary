package http

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/ants/config"
	"plexobject.com/formicary/ants/executor"
	"plexobject.com/formicary/ants/executor/tests"
	"plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/utils/trace"
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

func Test_ShouldGetNotfound(t *testing.T) {
	// GIVEN http provider
	providers, err := newProviders(newConfig(), types.HTTPGet)
	require.NoError(t, err)
	provider := providers[string(types.HTTPGet)]
	require.NotNil(t, provider)
	opts := types.NewExecutorOptions("http-get", types.HTTPGet)
	var stdout []byte
	var stderr []byte
	err = opts.Validate()
	require.NoError(t, err)
	jobTrace, err := trace.NewJobTrace(func(bytes []byte, tags string) {}, 1000, make([]string, 0))
	require.NoError(t, err)
	ctx := context.Background()
	// AND an executor
	exec, err := provider.NewExecutor(ctx, jobTrace, opts)
	require.NoError(t, err)

	// WHEN calling http get that returns not-found 404 returns
	runner, err := exec.AsyncExecute(ctx, "https://jsonplaceholder.typicode.com/notfound/1", make(map[string]types.VariableValue))
	require.NoError(t, err)

	// AND is then awaited for the completion
	stdout, stderr, err = runner.Await(ctx)
	require.Error(t, err)
	require.Equal(t, "not found error", err.Error())

	// THEN it should not have valid stdout
	require.Equal(t, 0, len(stdout))

	// AND no errors
	require.Equal(t, 0, len(stderr))

	// AND 404 error
	require.Equal(t, 404, runner.GetExitCode())

	// FINAL Cleanup
	err = exec.Stop()
	require.NoError(t, err)
}

func Test_ShouldGetCreated(t *testing.T) {
	// GIVEN http provider
	providers, err := newProviders(newConfig(), types.HTTPGet)
	require.NoError(t, err)
	provider := providers[string(types.HTTPGet)]
	require.NotNil(t, provider)
	opts := types.NewExecutorOptions("http-get", types.HTTPGet)
	var stdout []byte
	var stderr []byte
	err = opts.Validate()
	require.NoError(t, err)
	jobTrace, err := trace.NewJobTrace(func(bytes []byte, tags string) {}, 1000, make([]string, 0))
	require.NoError(t, err)
	ctx := context.Background()
	// AND an executor
	exec, err := provider.NewExecutor(ctx, jobTrace, opts)
	require.NoError(t, err)

	// WHEN calling http get that returns not-found 404 returns
	runner, err := exec.AsyncExecute(ctx, "https://jsonplaceholder.typicode.com/created/1", make(map[string]types.VariableValue))
	require.NoError(t, err)

	// AND is then awaited for the completion
	stdout, stderr, err = runner.Await(ctx)
	require.NoError(t, err)

	// THEN it should not have valid stdout
	require.Equal(t, "created response", string(stdout))

	// AND no errors
	require.Equal(t, 0, len(stderr))

	// AND 201
	require.Equal(t, 201, runner.GetExitCode())

	// FINAL Cleanup
	err = exec.Stop()
	require.NoError(t, err)
}

func Test_ShouldGetSubmitted(t *testing.T) {
	// GIVEN http provider
	providers, err := newProviders(newConfig(), types.HTTPGet)
	require.NoError(t, err)
	provider := providers[string(types.HTTPGet)]
	require.NotNil(t, provider)
	opts := types.NewExecutorOptions("http-get", types.HTTPGet)
	var stdout []byte
	var stderr []byte
	err = opts.Validate()
	require.NoError(t, err)
	jobTrace, err := trace.NewJobTrace(func(bytes []byte, tags string) {}, 1000, make([]string, 0))
	require.NoError(t, err)
	ctx := context.Background()
	// AND an executor
	exec, err := provider.NewExecutor(ctx, jobTrace, opts)
	require.NoError(t, err)

	// WHEN calling http get that returns not-found 404 returns
	runner, err := exec.AsyncExecute(ctx, "https://jsonplaceholder.typicode.com/submitted/1", make(map[string]types.VariableValue))
	require.NoError(t, err)

	// AND is then awaited for the completion
	stdout, stderr, err = runner.Await(ctx)
	require.Error(t, err)
	require.Equal(t, "submitted error", err.Error())

	// THEN it should not have valid stdout
	require.Equal(t, "", string(stdout))

	// AND no errors
	require.Equal(t, 0, len(stderr))

	// AND 202
	require.Equal(t, 202, runner.GetExitCode())

	// FINAL Cleanup
	err = exec.Stop()
	require.NoError(t, err)
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

	httpClient.GetMapping["https://jsonplaceholder.typicode.com/notfound/1"] = web.NewStubHTTPResponseError(404, 0, fmt.Errorf("not found error"))
	httpClient.GetMapping["https://jsonplaceholder.typicode.com/created/1"] = web.NewStubHTTPResponse(201, "created response")
	httpClient.GetMapping["https://jsonplaceholder.typicode.com/submitted/1"] = web.NewStubHTTPResponseError(202, 0, fmt.Errorf("submitted error"))
	httpClient.GetMapping["https://jsonplaceholder.typicode.com/moved/1"] = web.NewStubHTTPResponseError(302, 0, fmt.Errorf("moved error"))
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
