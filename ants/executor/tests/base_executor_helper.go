package tests

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/require"
	"os"
	"plexobject.com/formicary/internal/utils/trace"
	"strings"
	"testing"
	"time"

	"plexobject.com/formicary/ants/executor"
	"plexobject.com/formicary/internal/types"
)

const prefix = "formicary-test"

var integrationTestsEnabled = len(os.Getenv("INTEGRATION_EXECUTION_TESTS")) > 0

// CleanupContainers tests cleanup of containers
func CleanupContainers(
	t *testing.T,
	providers map[string]executor.Provider,
	opts *types.ExecutorOptions,
	matching string) {
	if !integrationTestsEnabled {
		t.Logf("Integration tests for executors is disabled, skipping CleanupContainers %v", providers)
		return
	}
	if opts == nil {
		opts = &types.ExecutorOptions{
			Environment:          types.NewEnvironmentMap(),
			Artifacts:            types.NewArtifacts(),
			Cache:                types.NewCacheConfig(),
			DependentArtifactIDs: make([]string, 0),
			NodeTolerations:      make(map[string]string),
			PodLabels:            make(map[string]string),
			PodAnnotations:       make(map[string]string),
			Services:             make([]types.Service, 0),
			MainContainer:        types.NewContainerDefinition(),
			HelperContainer:      types.NewContainerDefinition(),
		}
	}

	ctx := context.Background()
	for _, provider := range providers {
		all, err := provider.AllRunningExecutors(ctx)
		require.NoError(t, err)
		for _, next := range all {
			if strings.Contains(next.GetName(), matching) {
				//t.Logf("!!!Stopping executor %s %s %s", next.GetID(), next.GetName(), next.GetState())
				_ = provider.StopExecutor(ctx, next.GetName(), opts)
			}
		}
	}
}

// DoTestExecuteWithSimpleList tests execution of commands
func DoTestExecuteWithSimpleList(
	t *testing.T,
	providers map[string]executor.Provider,
	defOpts *types.ExecutorOptions,
	cmd string) {
	if !integrationTestsEnabled {
		t.Logf("Integration tests for executors is disabled, skipping DoTestExecuteWithSimpleList %v - %s",
			providers, cmd)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	defer CleanupContainers(t, providers, defOpts, prefix)
	// GIVEN a set of executor providers
	for name, provider := range providers {
		var stdout []byte
		var stderr []byte
		opts := buildOptions(defOpts, "ls-"+name, providers)
		err := opts.Validate()
		require.NoError(t, err)
		//t.Logf("****** TestExecuteWithSimpleList BEGIN provider %s", name)
		started := time.Now()
		jobTrace, err := trace.NewJobTrace(func(bytes []byte) {}, 1000, make([]string, 0))
		require.NoError(t, err)

		// AND an executor
		exec, err := provider.NewExecutor(ctx, jobTrace, opts)
		require.NoError(t, err)

		// WHEN a command is executed asynchronously
		runner, err := exec.AsyncExecute(ctx, cmd, make(map[string]interface{}))
		require.NoError(t, err)

		// AND is then awaited for the completion
		stdout, stderr, err = runner.Await(ctx)
		require.NoError(t, err)

		// THEN it should have valid stdout
		require.NotEqual(t, 0, len(stdout))

		// AND no errors
		require.Equal(t, 0, len(stderr))

		// FINAL Cleanup
		err = exec.Stop()
		require.NoError(t, err)
		require.Contains(t, string(stdout), "bin")
		elapsed := time.Since(started)
		t.Logf("****** TestExecuteWithSimpleList END provider %s - elapsed %s", name, elapsed)
	}
}

// DoTestExecuteWithTimeout tests execution of commands with timeout
func DoTestExecuteWithTimeout(
	t *testing.T,
	providers map[string]executor.Provider,
	defOpts *types.ExecutorOptions,
	cmd string) {
	if !integrationTestsEnabled {
		t.Logf("Integration tests for executors is disabled, skipping DoTestExecuteWithTimeout %v - %s",
			providers, cmd)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	defer CleanupContainers(t, providers, defOpts, prefix)
	// GIVEN a set of executor providers
	for name, provider := range providers {
		opts := buildOptions(defOpts, "timeout-"+name, providers)
		err := opts.Validate()
		require.NoError(t, err)
		t.Logf("****** TestExecuteWithTimeout BEGIN provider %s", name)
		started := time.Now()
		jobTrace, err := trace.NewJobTrace(func(bytes []byte) {}, 1000, make([]string, 0))
		require.NoError(t, err)

		// AND an executor
		exec, err := provider.NewExecutor(ctx, jobTrace, opts)
		require.NoError(t, err)

		// WHEN a command that takes a long time is executed asynchronously
		runner, err := exec.AsyncExecute(ctx, cmd, make(map[string]interface{}))
		require.NoError(t, err)
		// AND is then awaited for the completion
		stdout, stderr, err := runner.Await(ctx)
		// expect timeout error

		// THEN it should fail with timeout
		require.Error(t, err)

		// AND produce no stdout or stderr
		require.Equal(t, 0, len(stdout))
		require.Equal(t, 0, len(stderr))

		// FINALIZE
		err = exec.Stop()
		require.NoError(t, err)
		elapsed := time.Since(started)
		t.Logf("****** TestExecuteWithTimeout END for %s, elapsed %s", name, elapsed)
	}
}

// DoTestExecuteWithBadCommand tests execution of bad commands
func DoTestExecuteWithBadCommand(
	t *testing.T,
	providers map[string]executor.Provider,
	defOpts *types.ExecutorOptions,
	cmd string) {
	if !integrationTestsEnabled {
		t.Logf("Integration tests for executors is disabled, skipping DoTestExecuteWithBadCommand %v - %s",
			providers, cmd)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	defer CleanupContainers(t, providers, defOpts, prefix)
	// GIVEN a set of executor providers
	for name, provider := range providers {
		opts := buildOptions(defOpts, "bad-"+name, providers)
		err := opts.Validate()
		require.NoError(t, err)
		//t.Logf("****** TestExecuteWithBadCommand BEGIN provider %s", name)
		started := time.Now()
		jobTrace, err := trace.NewJobTrace(func(bytes []byte) {}, 1000, make([]string, 0))
		require.NoError(t, err)

		// AND an executor
		exec, err := provider.NewExecutor(ctx, jobTrace, opts)
		require.NoError(t, err)

		// WHEN an invalid command is executed asynchronously
		runner, err := exec.AsyncExecute(ctx, cmd, make(map[string]interface{}))
		require.NoError(t, err)

		// AND is then awaited for the completion
		stdout, stderr, err := runner.Await(ctx)

		// THEN it should fail
		require.Error(t, err) // should fail
		if len(stderr) == 0 && len(stdout) == 0 {
			t.Logf("empty stdout '%s' or stderr '%s' for %s", stdout, stderr, name)
		}

		// FINALIZE
		err = exec.Stop()
		require.NoError(t, err)
		elapsed := time.Since(started)
		t.Logf("****** TestExecuteWithBadCommand END for %s elapsed %s", name, elapsed)
	}
}

// DoTestListExecutors tests list of containers
func DoTestListExecutors(
	t *testing.T,
	providers map[string]executor.Provider,
	defOpts *types.ExecutorOptions,
	cmd string) {
	if !integrationTestsEnabled {
		t.Logf("Integration tests for executors is disabled, skipping DoTestListExecutors %v - %s",
			providers, cmd)
		return
	}
	ctx := context.Background()
	defer CleanupContainers(t, providers, defOpts, prefix)

	// GIVEN a set of executor providers
	for name, provider := range providers {
		opts := buildOptions(defOpts, "list-"+name, providers)
		err := opts.Validate()
		require.NoError(t, err)
		containerName := opts.Name
		t.Logf("****** TestListExecutors BEGIN provider %s", name)
		started := time.Now()
		jobTrace, err := trace.NewJobTrace(func(bytes []byte) {}, 1000, make([]string, 0))
		require.NoError(t, err)

		// WHEN executors are created
		for i := 0; i < 5; i++ {
			opts.Name = fmt.Sprintf("%s-%d", containerName, i)
			exec, err := provider.NewExecutor(ctx, jobTrace, opts)
			require.NoError(t, err)
			_, err = exec.AsyncExecute(ctx, cmd, make(map[string]interface{}))
			require.NoError(t, err)
		}

		// AND provider's list executor method is called
		list, err := provider.ListExecutors(ctx)

		// THEN it should not fail and return same number of executors
		require.NoError(t, err)
		require.Equal(t, 5, len(list))
		for _, exec := range list {
			t.Logf("TestExecuteWithTimeout exec %s", exec.GetName())
		}
		elapsed := time.Since(started)
		t.Logf("****** TestListExecutors END provider %s, elapsed %s", name, elapsed)
	}
}

// DoTestGetRuntime tests runtime info
func DoTestGetRuntime(
	t *testing.T,
	providers map[string]executor.Provider,
	defOpts *types.ExecutorOptions) {
	if !integrationTestsEnabled {
		t.Logf("Integration tests for executors is disabled, skipping DoTestGetRuntime %v",
			providers)
		return
	}
	ctx := context.Background()
	defer CleanupContainers(t, providers, defOpts, prefix)
	// GIVEN a set of executor providers
	for name, provider := range providers {
		opts := buildOptions(defOpts, "stop-"+name, providers)
		err := opts.Validate()
		require.NoError(t, err)
		t.Logf("****** TestGetRuntime BEGIN provider %s", name)
		started := time.Now()
		jobTrace, err := trace.NewJobTrace(func(bytes []byte) {}, 1000, make([]string, 0))
		require.NoError(t, err)

		// AND an executor
		exec, err := provider.NewExecutor(ctx, jobTrace, opts)
		require.NoError(t, err)

		// WHEN runtime info is invoked
		rt := exec.GetRuntimeInfo(ctx)

		// THEN it should return valid executor name/id
		if !strings.Contains(rt, exec.GetName()) || !strings.Contains(rt, exec.GetID()) {
			t.Fatalf("Missing name %s or id %s: Unexpected runtime '%s'", exec.GetName(), exec.GetID(), rt)
		}
		_ = exec.Stop()
		elapsed := time.Since(started)
		t.Logf("****** TestGetRuntime END provider %s, elapsed %s", name, elapsed)
	}
}

/////////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////
func buildOptions(
	defOpts *types.ExecutorOptions,
	name string,
	providers map[string]executor.Provider) *types.ExecutorOptions {
	if defOpts != nil {
		opts := defOpts
		opts.Name = prefix + name
		return opts
	}
	return newOptions(name, providers)
}

func newOptions(suffix string, providers map[string]executor.Provider) *types.ExecutorOptions {
	opts := types.NewExecutorOptions(fmt.Sprintf("%s-%s-%d", prefix, suffix, time.Now().Unix()), "")
	for k := range providers {
		opts.Method = types.TaskMethod(k)
	}
	opts.MainContainer.Image = "alpine"
	return opts
}
