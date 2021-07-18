package shell

import (
	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/ants/config"
	"plexobject.com/formicary/ants/executor"
	"plexobject.com/formicary/ants/executor/tests"
	"testing"
)

func newProviders(c *config.AntConfig) (map[string]executor.Provider, error) {
	shellProvider, err := NewExecutorProvider(c)
	if err != nil {
		return nil, err
	}
	return map[string]executor.Provider{
		"SHELL": shellProvider,
	}, nil
}

func newConfig() *config.AntConfig {
	c := config.AntConfig{}
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
