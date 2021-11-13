package types

import (
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"plexobject.com/formicary/queen/utils"
	"testing"
)

func Test_ShouldParseKubernetesVolume(t *testing.T) {
	// GIVEN job-definition loaded from pipeline yaml
	b, err := ioutil.ReadFile("../../docs/examples/check-oidc.yaml")
	require.NoError(t, err)
	serData := utils.ParseYamlTag(string(b), "task_type: oidc")
	opts := NewExecutorOptions("", "")
	err = yaml.Unmarshal([]byte(serData), opts)
	require.NoError(t, err)
	vols := opts.MainContainer.GetKubernetesVolumes()
	require.Equal(t, 1, len(vols.HostPaths))
	require.Equal(t, 1, len(vols.EmptyDirs))
	require.Equal(t, 1, len(vols.Projected))

	require.Equal(t, "mount1", vols.HostPaths[0].Name)
	require.Equal(t, "/myshared", vols.HostPaths[0].MountPath)
	require.Equal(t, "/shared", vols.HostPaths[0].HostPath)
	require.Equal(t, "mount2", vols.EmptyDirs[0].Name)
	require.Equal(t, "/myempty", vols.EmptyDirs[0].MountPath)
	require.Equal(t, "oidc-info", vols.Projected[0].Name)
	require.Equal(t, "/var/run/sigstore/cosign", vols.Projected[0].MountPath)
	require.Equal(t, 1, len(vols.Projected[0].Sources))

	require.NotNil(t, vols.Projected[0].Sources[0].ServiceAccountToken)
	require.Equal(t, "oidc-token", vols.Projected[0].Sources[0].ServiceAccountToken.Path)
	require.Equal(t, int64(600), *vols.Projected[0].Sources[0].ServiceAccountToken.ExpirationSeconds)
	require.Equal(t, "sigstore", vols.Projected[0].Sources[0].ServiceAccountToken.Audience)
}
