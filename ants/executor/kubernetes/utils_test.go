package kubernetes

import (
	"testing"

	"github.com/stretchr/testify/require"
	api "k8s.io/api/core/v1"
	"plexobject.com/formicary/internal/ant_config"
	domain "plexobject.com/formicary/internal/types"
)

// Test_ShouldBuildEnvFrom verifies that bulk Secret/ConfigMap references are
// converted to the Kubernetes api.EnvFromSource slice correctly.
func Test_ShouldBuildEnvFrom(t *testing.T) {
	sources := []*domain.EnvFromSource{
		{SecretRef: "my-secret", Prefix: "SEC_"},
		{ConfigMapRef: "my-config"},
		{},             // empty entry — must be skipped
		{Prefix: "ORPHAN_"}, // no ref — must be skipped
	}

	result := buildEnvFrom(sources)

	require.Len(t, result, 2, "expected 2 valid envFrom entries")

	require.NotNil(t, result[0].SecretRef)
	require.Equal(t, "my-secret", result[0].SecretRef.Name)
	require.Equal(t, "SEC_", result[0].Prefix)

	require.NotNil(t, result[1].ConfigMapRef)
	require.Equal(t, "my-config", result[1].ConfigMapRef.Name)
	require.Equal(t, "", result[1].Prefix)
}

// Test_ShouldBuildEnvFromNil verifies that a nil or empty slice returns nil.
func Test_ShouldBuildEnvFromNil(t *testing.T) {
	require.Nil(t, buildEnvFrom(nil))
	require.Nil(t, buildEnvFrom([]*domain.EnvFromSource{}))
}

// Test_ShouldBuildVariablesWithEnvValueFrom verifies that individual Secret/ConfigMap
// key references (secretKeyRef / configMapKeyRef) are injected as named env vars.
func Test_ShouldBuildVariablesWithEnvValueFrom(t *testing.T) {
	config := &ant_config.KubernetesConfig{}
	opts := domain.NewExecutorOptions("test-task", "KUBERNETES")
	opts.MainContainer.EnvValueFrom = []*domain.EnvVarSource{
		{Name: "ANTHROPIC_API_KEY", SecretName: "ai-secrets", Key: "anthropic-api-key"},
		{Name: "MODEL_NAME", ConfigMapName: "ai-config", Key: "model"},
		{Name: "ORPHAN"}, // no SecretName or ConfigMapName — must be skipped
	}

	envVars := buildVariables(config, opts, false, nil)

	// Locate the injected vars
	varMap := make(map[string]api.EnvVar)
	for _, ev := range envVars {
		varMap[ev.Name] = ev
	}

	anthropicKey, ok := varMap["ANTHROPIC_API_KEY"]
	require.True(t, ok, "ANTHROPIC_API_KEY should be present")
	require.NotNil(t, anthropicKey.ValueFrom)
	require.NotNil(t, anthropicKey.ValueFrom.SecretKeyRef)
	require.Equal(t, "ai-secrets", anthropicKey.ValueFrom.SecretKeyRef.Name)
	require.Equal(t, "anthropic-api-key", anthropicKey.ValueFrom.SecretKeyRef.Key)

	modelName, ok := varMap["MODEL_NAME"]
	require.True(t, ok, "MODEL_NAME should be present")
	require.NotNil(t, modelName.ValueFrom)
	require.NotNil(t, modelName.ValueFrom.ConfigMapKeyRef)
	require.Equal(t, "ai-config", modelName.ValueFrom.ConfigMapKeyRef.Name)
	require.Equal(t, "model", modelName.ValueFrom.ConfigMapKeyRef.Key)

	_, present := varMap["ORPHAN"]
	require.False(t, present, "orphan entry with no ref should not be injected")
}

// Test_ShouldBuildVariablesSkipsEmptyEnvVarName verifies that EnvVarSource entries
// with an empty Name are skipped — Kubernetes rejects pods with nameless env vars.
func Test_ShouldBuildVariablesSkipsEmptyEnvVarName(t *testing.T) {
	config := &ant_config.KubernetesConfig{}
	opts := domain.NewExecutorOptions("test-task", "KUBERNETES")
	opts.MainContainer.EnvValueFrom = []*domain.EnvVarSource{
		{Name: "", SecretName: "sec", Key: "k"}, // empty name — must be skipped
		{Name: "VALID_KEY", SecretName: "sec", Key: "k"},
	}

	envVars := buildVariables(config, opts, false, nil)

	for _, ev := range envVars {
		require.NotEqual(t, "", ev.Name, "env var with empty name must not be injected")
	}
	varMap := make(map[string]api.EnvVar)
	for _, ev := range envVars {
		varMap[ev.Name] = ev
	}
	_, ok := varMap["VALID_KEY"]
	require.True(t, ok, "valid entry should still be present")
}

// Test_ShouldBuildVariablesHelperIgnoresEnvValueFrom verifies that EnvValueFrom on
// the main container is not injected into helper container env vars.
func Test_ShouldBuildVariablesHelperIgnoresEnvValueFrom(t *testing.T) {
	config := &ant_config.KubernetesConfig{}
	opts := domain.NewExecutorOptions("test-task", "KUBERNETES")
	opts.MainContainer.EnvValueFrom = []*domain.EnvVarSource{
		{Name: "MAIN_ONLY", SecretName: "sec", Key: "k"},
	}

	helperEnvVars := buildVariables(config, opts, true, nil)

	for _, ev := range helperEnvVars {
		require.NotEqual(t, "MAIN_ONLY", ev.Name,
			"EnvValueFrom from main container should not appear in helper env vars")
	}
}

// Test_ShouldUsePerTaskServiceAccount verifies the service account resolution:
// per-task > ant-worker default.
func Test_ShouldUsePerTaskServiceAccount(t *testing.T) {
	config := &ant_config.KubernetesConfig{ServiceAccount: "default-worker-sa"}
	opts := domain.NewExecutorOptions("test-task", "KUBERNETES")

	// No per-task override: fall back to ant-worker default
	require.Equal(t, "default-worker-sa", resolveServiceAccount(config, opts))

	// Per-task override takes precedence
	opts.MainContainer.ServiceAccount = "irsa-task-sa"
	require.Equal(t, "irsa-task-sa", resolveServiceAccount(config, opts))

	// nil opts: fall back to worker default safely
	require.Equal(t, "default-worker-sa", resolveServiceAccount(config, nil))
}
