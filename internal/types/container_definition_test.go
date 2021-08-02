package types

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_ShouldCreateContainerDefinition(t *testing.T) {
	// Given container definition
	def := NewContainerDefinition()

	// WHEN accessing properties
	// THEN it should return expected values
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
	def.Volumes = map[string]string{"a":"b"}
	require.Equal(t, 1, len(def.GetDockerVolumeNames()))
	require.Equal(t, 1, len(def.GetDockerVolumes()))
	def.Volumes = map[string]interface{}{"a":"b"}
	require.Equal(t, 1, len(def.GetDockerVolumeNames()))
	require.Equal(t, 1, len(def.GetDockerVolumes()))
	require.Equal(t, 1, len(def.GetDockerMounts()))
	def.Volumes = nil
	require.Equal(t, 0, len(def.GetDockerVolumeNames()))
	require.Equal(t, 0, len(def.GetDockerVolumes()))
	def.AddEmptyKubernetesVolume("name", "mount")
	require.False(t, def.HasKubernetesVolumes())
	require.Equal(t, 0, len(def.GetKubernetesVolumes().HostPaths))

	def.Volumes = map[string]interface{}{"a":"b"}
	require.Equal(t, 0, len(def.GetKubernetesVolumes().HostPaths))
	def.Volumes = nil
	require.Equal(t, 0, len(def.GetKubernetesVolumes().HostPaths))
}
