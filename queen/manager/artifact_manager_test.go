package manager

import (
	"context"
	"github.com/stretchr/testify/require"
	"io"
	"plexobject.com/formicary/internal/artifacts"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/repository"
	"strings"
	"testing"
	"time"
)

func Test_ShouldExpireArtifacts(t *testing.T) {
	// GIVEN artifact-manager

	serverCfg := config.TestServerConfig()
	err := serverCfg.Validate()
	require.NoError(t, err)

	mgr := newTestArtifactManager(t, err, serverCfg)

	qc, err := repository.NewTestQC()
	require.NoError(t, err)
	serverCfg.DefaultArtifactExpiration = time.Millisecond
	for i := 0; i < 10; i++ {
		in := io.NopCloser(strings.NewReader("test"))
		_, err := mgr.UploadArtifact(
			context.Background(),
			qc,
			in,
			make(map[string]string))
		require.NoError(t, err)
	}
	time.Sleep(2 * time.Millisecond)
	// WHEN expiring
	expired, _, err := mgr.ExpireArtifacts(
		context.Background(),
		qc,
		time.Millisecond,
		10000)

	// THEN it should not fail
	require.NoError(t, err)
	require.Equal(t, 10, expired)
}

func Test_ShouldUploadArtifacts(t *testing.T) {
	// GIVEN artifact-manager
	serverCfg := config.TestServerConfig()
	err := serverCfg.Validate()
	require.NoError(t, err)
	qc, err := repository.NewTestQC()
	require.NoError(t, err)

	mgr := newTestArtifactManager(t, err, serverCfg)

	// WHEN querying artifacts
	_, total, err := mgr.QueryArtifacts(
		context.Background(),
		qc,
		make(map[string]interface{}),
		0,
		100,
		make([]string, 0))

	// THEN it should not fail
	require.NoError(t, err)
	require.Equal(t, int64(0), total)

	// GIVEN uploaded artifact
	in := io.NopCloser(strings.NewReader("test"))
	art, err := mgr.UploadArtifact(
		context.Background(),
		qc,
		in,
		make(map[string]string))
	require.NoError(t, err)

	// WHEN getting artifact
	loaded, err := mgr.GetArtifact(
		context.Background(),
		qc,
		art.ID)

	// THEN it should not fail and return valid artifact
	require.NoError(t, err)
	require.Equal(t, art.ID, loaded.ID)

	// WHEN deleting artifact
	err = mgr.DeleteArtifact(context.Background(), qc, art.ID)

	// THEN it should not fail
	require.NoError(t, err)

	// WHEN getting artifact after delete
	_, err = mgr.GetArtifact(
		context.Background(),
		qc,
		art.ID)

	// THEN it should fail
	require.Error(t, err)
}

func newTestArtifactManager(t *testing.T, err error, serverCfg *config.ServerConfig) *ArtifactManager {
	artifactService, err := artifacts.NewStub(&serverCfg.Common.S3)
	require.NoError(t, err)
	artifactRepository, err := repository.NewTestArtifactRepository()
	require.NoError(t, err)
	logRepository, err := repository.NewTestLogEventRepository()
	require.NoError(t, err)

	mgr, err := NewArtifactManager(
		serverCfg,
		logRepository,
		artifactRepository,
		artifactService)
	require.NoError(t, err)
	return mgr
}
