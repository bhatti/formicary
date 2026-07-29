package types

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func Test_ShouldCreateContainerDefinition(t *testing.T) {
	def := NewContainerDefinition()

	require.NotEqual(t, "", def.String())
	require.False(t, def.HasDockerBindVolumes())
	require.False(t, def.HasDockerFromVolumes())
	require.False(t, def.HasKubernetesVolumes())
	def.Volumes = map[string]interface{}{"a": "b"}
	require.True(t, def.HasDockerBindVolumes())
	require.False(t, def.HasDockerFromVolumes())
	def.VolumesFrom = []string{"a"}
	require.True(t, def.HasDockerFromVolumes())
	require.False(t, def.HasKubernetesVolumes())
	require.Equal(t, 0, len(def.GetDockerVolumeNames()))
	require.Equal(t, 0, len(def.GetDockerVolumes()))
	require.Equal(t, 0, len(def.GetDockerMounts()))
	require.Equal(t, 0, len(def.GetKubernetesVolumes().HostPaths))
	def.Volumes = map[string]string{"a": "b"}
	require.Equal(t, 1, len(def.GetDockerVolumeNames()))
	require.Equal(t, 1, len(def.GetDockerVolumes()))
	def.Volumes = map[string]interface{}{"a": "b"}
	require.Equal(t, 1, len(def.GetDockerVolumeNames()))
	require.Equal(t, 1, len(def.GetDockerVolumes()))
	require.Equal(t, 1, len(def.GetDockerMounts()))
	def.Volumes = nil
	require.Equal(t, 0, len(def.GetDockerVolumeNames()))
	require.Equal(t, 0, len(def.GetDockerVolumes()))
	def.AddEmptyKubernetesVolume("name", "mount")
	require.False(t, def.HasKubernetesVolumes())
	require.Equal(t, 0, len(def.GetKubernetesVolumes().HostPaths))
	def.Volumes = map[string]interface{}{"a": "b"}
	require.Equal(t, 0, len(def.GetKubernetesVolumes().HostPaths))
	def.Volumes = nil
	require.Equal(t, 0, len(def.GetKubernetesVolumes().HostPaths))
}

func TestEnvFromYAMLSecretRef(t *testing.T) {
	input := `
image: myimage
env_from:
  - secret_ref: ai-dev-credentials
`
	var c ContainerDefinition
	err := yaml.Unmarshal([]byte(input), &c)
	require.NoError(t, err)
	require.Len(t, c.EnvFrom, 1)
	require.Equal(t, "ai-dev-credentials", c.EnvFrom[0].SecretRef)
}
