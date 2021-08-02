package types

import (
	"github.com/stretchr/testify/require"
	api "k8s.io/api/core/v1"
	"testing"
)

func Test_ShouldCreateService(t *testing.T) {
	svc := & Service {
		Name: "name",
		Alias: "alias",
		Image: "image",
		Entrypoint: []string{"entry"},
		Command: []string{"a"},
	}
	image := svc.ToImageDefinition()
	require.Equal(t, "entry", image.Entrypoint[0])
	svc.Ports = make([]Port, 1)
	image = svc.ToImageDefinition()
	require.Equal(t, svc.Ports, image.Ports)
	cnt := svc.ToContainer(make([]api.VolumeMount, 1), api.PullAlways)
	require.Equal(t, svc.Image, cnt.Image)
}

func Test_ShouldAddEmptyKubernetesVolumeForService(t *testing.T) {
	svc := Service {
		Name: "name",
		Alias: "alias",
		Image: "image",
		Entrypoint: []string{"entry"},
		Command: []string{"a"},
	}
	svc.AddEmptyKubernetesVolume("name", "mount")
	require.Equal(t, 1, len(svc.GetKubernetesVolumes().EmptyDirs))
	aliases, err := CreateHostAliases([]Service{svc}, nil)
	require.NoError(t, err)
	require.Equal(t, 1, len(aliases))
}
