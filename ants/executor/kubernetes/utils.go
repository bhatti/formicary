package kubernetes

import (
	api "k8s.io/api/core/v1"
	"plexobject.com/formicary/ants/config"
	domain "plexobject.com/formicary/internal/types"
)

func getCapabilities(defaultCapDrop []string, capAdd []string, capDrop []string) *api.Capabilities {
	enabled := make(map[string]bool)

	for _, v := range defaultCapDrop {
		enabled[v] = false
	}

	for _, v := range capAdd {
		enabled[v] = true
	}

	for _, v := range capDrop {
		enabled[v] = false
	}

	if len(enabled) < 1 {
		return nil
	}

	return buildCapabilities(enabled)
}

func buildCapabilities(enabled map[string]bool) *api.Capabilities {
	tags := new(api.Capabilities)

	for c, add := range enabled {
		if add {
			tags.Add = append(tags.Add, api.Capability(c))
			continue
		}
		tags.Drop = append(tags.Drop, api.Capability(c))
	}

	return tags
}

func getCommandAndArgs(
	imageDefinition domain.Image,
	command ...string) ([]string, []string) {
	if len(command) == 0 && len(imageDefinition.Command) > 0 {
		command = imageDefinition.Command
	}

	var args []string
	if len(imageDefinition.Entrypoint) > 0 {
		args = command
		command = imageDefinition.Entrypoint
	}

	return command, args
}

// getDefaultCapDrop returns the default tags that should be dropped from a build container.
func getDefaultCapDrop() []string {
	return []string{
		"NET_RAW",
	}
}

func buildVariables(
	config *config.KubernetesConfig,
	opts *domain.ExecutorOptions,
	helper bool) []api.EnvVar {
	e := make([]api.EnvVar, 0)
	for k, v := range config.Environment {
		e = append(e, api.EnvVar{Name: k, Value: v})
	}
	if helper {
		for k, v := range opts.HelperEnvironment {
			e = append(e, api.EnvVar{Name: k, Value: v})
		}
	} else {
		for k, v := range opts.Environment {
			e = append(e, api.EnvVar{Name: k, Value: v})
		}
	}
	return e
}
