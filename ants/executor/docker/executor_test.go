package docker

import (
	"github.com/stretchr/testify/require"
	"os"
	"plexobject.com/formicary/ants/config"
	"plexobject.com/formicary/ants/executor"
	"plexobject.com/formicary/ants/executor/tests"
	"testing"
)

var integrationTestsEnabled = len(os.Getenv("INTEGRATION_EXECUTION_TESTS")) > 0

// start minikube - minikube start --driver=docker
func newProviders(c *config.AntConfig) (map[string]executor.Provider, error) {
	dockerProvider, err := NewExecutorProvider(c)
	if err != nil {
		return nil, err
	}
	return map[string]executor.Provider{
		"DOCKER": dockerProvider,
	}, nil
}

func newConfig() *config.AntConfig {
	c := config.AntConfig{}
	c.DefaultShell = []string{
		"sh",
		"-c",
		"if [ -x /usr/local/bin/bash ]; then\n\texec /usr/local/bin/bash \nelif [ -x /usr/bin/bash ]; then\n\texec /usr/bin/bash \nelif [ -x /bin/bash ]; then\n\texec /bin/bash \nelif [ -x /usr/local/bin/   sh ]; then\n\texec /usr/local/bin/sh \nelif [ -x /usr/bin/sh ]; then\n\texec /usr/bin/sh \nelif [ -x /bin/sh ]; then\n\texec /bin/sh \nelif [ -x /busybox/sh ]; then\n\texec /busybox/sh \nelse\n\techo shell  not found\n\texit 1\nfi\n\n",
	}
	c.Docker.Host = "tcp://localhost:2375"
	c.OutputLimit = 64 * 1024 * 1024
	return &c
}

func Test_ShouldExecuteWithSimpleList(t *testing.T) {
	if !integrationTestsEnabled {
		t.Logf("Integration tests for executors is disabled, skipping Test_ShouldExecuteWithSimpleList")
		return
	}
	providers, err := newProviders(newConfig())
	require.NoError(t, err)
	tests.DoTestExecuteWithSimpleList(t, providers, nil, "ls -l /")
}

func Test_ShouldExecuteWithTimeout(t *testing.T) {
	if !integrationTestsEnabled {
		t.Logf("Integration tests for executors is disabled, skipping Test_ShouldExecuteWithTimeout")
		return
	}
	providers, err := newProviders(newConfig())
	require.NoError(t, err)
	tests.DoTestExecuteWithTimeout(t, providers, nil, "sleep 30")
}

func Test_ShouldExecuteWithBadCommand(t *testing.T) {
	if !integrationTestsEnabled {
		t.Logf("Integration tests for executors is disabled, skipping Test_ShouldExecuteWithBadCommand")
		return
	}
	providers, err := newProviders(newConfig())
	require.NoError(t, err)
	tests.DoTestExecuteWithBadCommand(t, providers, nil, "blah")
}

func Test_ShouldListExecutors(t *testing.T) {
	if !integrationTestsEnabled {
		t.Logf("Integration tests for executors is disabled, skipping Test_ShouldListExecutors")
		return
	}
	providers, err := newProviders(newConfig())
	require.NoError(t, err)
	tests.DoTestListExecutors(t, providers, nil, "sleep 30")
}

func Test_ShouldGetRuntime(t *testing.T) {
	if !integrationTestsEnabled {
		t.Logf("Integration tests for executors is disabled, skipping Test_ShouldGetRuntime")
		return
	}
	providers, err := newProviders(newConfig())
	require.NoError(t, err)
	tests.DoTestGetRuntime(t, providers, nil)
}
