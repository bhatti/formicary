package kubernetes

import (
	"github.com/stretchr/testify/require"
	"os"
	"plexobject.com/formicary/ants/executor"
	"plexobject.com/formicary/ants/executor/tests"
	"plexobject.com/formicary/internal/ant_config"
	"plexobject.com/formicary/internal/types"
	"testing"
)

var integrationTestsEnabled = len(os.Getenv("INTEGRATION_EXECUTION_TESTS")) > 0

// start minikube - minikube start --driver=docker
func newProviders(c *ant_config.AntConfig) (map[string]executor.Provider, error) {
	kubernetesProvider, err := NewExecutorProvider(c)
	if err != nil {
		return nil, err
	}
	return map[string]executor.Provider{
		"KUBERNETES": kubernetesProvider,
	}, nil
}

func newConfig() *ant_config.AntConfig {
	c := ant_config.AntConfig{}
	c.DefaultShell = []string{
		"sh",
		"-c",
		"if [ -x /usr/local/bin/bash ]; then\n\texec /usr/local/bin/bash \nelif [ -x /usr/bin/bash ]; then\n\texec /usr/bin/bash \nelif [ -x /bin/bash ]; then\n\texec /bin/bash \nelif [ -x /usr/local/bin/   sh ]; then\n\texec /usr/local/bin/sh \nelif [ -x /usr/bin/sh ]; then\n\texec /usr/bin/sh \nelif [ -x /bin/sh ]; then\n\texec /bin/sh \nelif [ -x /busybox/sh ]; then\n\texec /busybox/sh \nelse\n\techo shell  not found\n\texit 1\nfi\n\n",
	}
	c.Kubernetes.Namespace = "default"
	c.Kubernetes.Registry.PullPolicy = types.PullPolicyAlways
	c.Kubernetes.Registry.Server = ""   //os.Getenv("KUBERNETES_REGISTRY_SERVER")
	c.Kubernetes.Registry.Username = "" //os.Getenv("KUBERNETES_REGISTRY_USERNAME")
	c.Kubernetes.Registry.Password = "" //os.Getenv("KUBERNETES_REGISTRY_PASSWORD")
	//c.Kubernetes.Host = "tcp://192.168.1.102:2375"
	//c.Kubernetes.Server = "192.168.1.102"
	c.Kubernetes.Server = "localhost"
	c.OutputLimit = 64 * 1024 * 1024
	return &c
}

func Test_ShouldCleanup(t *testing.T) {
	if !integrationTestsEnabled {
		t.Logf("Integration tests for executors is disabled, skipping Test_ShouldCleanup")
		return
	}
	providers, err := newProviders(newConfig())
	require.NoError(t, err)
	tests.CleanupContainers(t, providers, nil, "formicary-test")
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
