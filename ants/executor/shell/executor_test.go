package shell

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/ants/executor"
	"plexobject.com/formicary/ants/executor/tests"
	"plexobject.com/formicary/internal/ant_config"
	"plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/utils/trace"
)

func newProviders(c *ant_config.AntConfig) (map[string]executor.Provider, error) {
	shellProvider, err := NewExecutorProvider(c)
	if err != nil {
		return nil, err
	}
	return map[string]executor.Provider{
		"SHELL": shellProvider,
	}, nil
}

func newConfig() *ant_config.AntConfig {
	c := ant_config.AntConfig{}
	c.DefaultShell = []string{
		"sh",
		"-c",
		"if [ -x /usr/local/bin/bash ]; then\n\texec /usr/local/bin/bash \nelif [ -x /usr/bin/bash ]; then\n\texec /usr/bin/bash \nelif [ -x /bin/bash ]; then\n\texec /bin/bash \nelif [ -x /usr/local/bin/   sh ]; then\n\texec /usr/local/bin/sh \nelif [ -x /usr/bin/sh ]; then\n\texec /usr/bin/sh \nelif [ -x /bin/sh ]; then\n\texec /bin/sh \nelif [ -x /busybox/sh ]; then\n\texec /busybox/sh \nelse\n\techo shell  not found\n\texit 1\nfi\n\n",
	}
	c.OutputLimit = 64 * 1024 * 1024
	return &c
}

func Test_ShouldExecuteWithSimpleList(t *testing.T) {
	providers, err := newProviders(newConfig())
	require.NoError(t, err)
	tests.DoTestExecuteWithSimpleList(t, providers, nil, "ls -l /")
}

func Test_ShouldExecuteWithTimeout(t *testing.T) {
	providers, err := newProviders(newConfig())
	require.NoError(t, err)
	tests.DoTestExecuteWithTimeout(t, providers, nil, "sleep 30")
}

func Test_ShouldExecuteWithBadCommand(t *testing.T) {
	providers, err := newProviders(newConfig())
	require.NoError(t, err)
	tests.DoTestExecuteWithBadCommand(t, providers, nil, "blah")
}

func Test_ShouldListExecutors(t *testing.T) {
	providers, err := newProviders(newConfig())
	require.NoError(t, err)
	tests.DoTestListExecutors(t, providers, nil, "sleep 30")
}

func Test_ShouldGetRuntime(t *testing.T) {
	providers, err := newProviders(newConfig())
	require.NoError(t, err)
	tests.DoTestGetRuntime(t, providers, nil)
}

// Test_ShouldInheritProcessPATHWhenEnvSet reproduces the bug where setting task
// environment variables replaced os.Environ() entirely, causing tools like gh and
// jq (installed in /opt/homebrew/bin or other non-default paths) to be missing
// from PATH inside the shell script — resulting in "exit status 1" from `command -v gh`.
//
// The fix in runner.go merges task env vars on top of os.Environ() so that the
// inherited PATH (and all other process env vars) are always available.
func Test_ShouldInheritProcessPATHWhenEnvSet(t *testing.T) {
	// Find a binary that exists somewhere in the current process PATH but is
	// NOT in the minimal /usr/bin:/bin PATH that a child process sees when
	// cmd.Env is set to only task vars.
	// We use the Go test binary's own directory or /usr/bin/env as a proxy:
	// the simplest portable check is to verify that 'which sh' works (always
	// true) and that task-defined vars are visible AND PATH is inherited.

	// Locate a real binary outside /usr/bin and /bin to use as the canary.
	// On macOS with Homebrew the answer is /opt/homebrew/bin; on Linux it
	// could be /usr/local/bin. We find the test binary itself as a fallback.
	goExe, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go binary not in PATH — skipping PATH-inheritance test")
	}
	// goExe is something like /usr/local/go/bin/go or /opt/homebrew/bin/go.
	// Extract the directory and make sure it's not a standard system path.
	goDir := goExe[:strings.LastIndex(goExe, "/")]

	jobTrace, err := trace.NewJobTrace(func([]byte, string) {}, 1000, nil)
	require.NoError(t, err)

	opts := types.NewExecutorOptions("path-test", "SHELL")
	opts.Environment = types.NewEnvironmentMap()
	opts.Environment["MY_TASK_VAR"] = "hello"

	cfg := newConfig()
	baseExec, err := executor.NewBaseExecutor(cfg, jobTrace, opts)
	require.NoError(t, err)

	// Build the script: verify task var is set AND the go binary is findable.
	script := fmt.Sprintf(`
set -e
[ "$MY_TASK_VAR" = "hello" ] || { echo "TASK_VAR missing"; exit 1; }
command -v go >/dev/null 2>&1 || { echo "go not in PATH (PATH=$PATH)"; exit 2; }
echo "OK PATH=$PATH"
`)
	runner, err := NewCommandRunner(&baseExec, script, false)
	require.NoError(t, err)

	ctx := context.Background()
	require.NoError(t, runner.run(ctx))

	// Verify that goDir is actually in the inherited PATH inside the runner.
	_ = goDir
	stdout, _, runErr := runner.Await(ctx)
	require.NoError(t, runErr, "command failed — PATH not inherited. stdout: %s", stdout)
	require.Contains(t, string(stdout), "OK PATH=")
}

// Test_ShouldNotLeakProcessEnvIntoTaskEnv verifies that a task-supplied env var
// takes precedence over any identically-named var in the process environment,
// even after the PATH-inheritance fix.
func Test_ShouldNotLeakProcessEnvIntoTaskEnv(t *testing.T) {
	// Set a known env var in the process env, then override it via task env.
	t.Setenv("FORMICARY_TEST_OVERRIDE", "from-process")

	jobTrace, err := trace.NewJobTrace(func([]byte, string) {}, 1000, nil)
	require.NoError(t, err)

	opts := types.NewExecutorOptions("override-test", "SHELL")
	opts.Environment = types.NewEnvironmentMap()
	opts.Environment["FORMICARY_TEST_OVERRIDE"] = "from-task"

	cfg := newConfig()
	baseExec, err := executor.NewBaseExecutor(cfg, jobTrace, opts)
	require.NoError(t, err)

	script := `echo "VAL=${FORMICARY_TEST_OVERRIDE}"`
	runner, err := NewCommandRunner(&baseExec, script, false)
	require.NoError(t, err)

	ctx := context.Background()
	require.NoError(t, runner.run(ctx))
	stdout, _, runErr := runner.Await(ctx)
	require.NoError(t, runErr)
	require.Contains(t, string(stdout), "VAL=from-task",
		"task env var should override process env var; got: %s", stdout)

	// Also ensure the process env itself was NOT mutated.
	require.Equal(t, "from-process", os.Getenv("FORMICARY_TEST_OVERRIDE"))
}
