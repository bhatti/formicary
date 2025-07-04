package controller

import (
	"github.com/oklog/ulid/v2"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/url"
	"plexobject.com/formicary/internal/artifacts"
	"plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/repository"
	"strings"
	"testing"
)

func Test_InitializeSwaggerStructsForArtifact(t *testing.T) {
	_ = artifactsQueryParamsBody{}
	_ = artifactsQueryResponseBody{}
	_ = artifactUploadParams{}
	_ = artifactResponseBody{}
	_ = artifactIDParamsBody{}
	_ = artifactUploadParams{}
	_ = artifactResponseBody{}
	_ = stringResponseBody{}
	_ = byteResponseBody{}
}

func Test_ShouldQueryArtifacts(t *testing.T) {
	// GIVEN artifact controller
	mgr := newTestArtifactManager(config.TestServerConfig(), t)
	webServer := web.NewStubWebServer()
	ctrl := NewArtifactController(mgr, webServer)
	reader := io.NopCloser(strings.NewReader("test-data"))
	req := &http.Request{Body: reader}
	ctx := web.NewStubContext(req)
	_ = ctrl.uploadArtifact(ctx)

	// WHEN querying artifacts
	req = &http.Request{URL: &url.URL{}}
	ctx = web.NewStubContext(req)
	err := ctrl.queryArtifacts(ctx)

	// THEN it should not fail and return artifacts
	require.NoError(t, err)
	recs := ctx.Result.(*PaginatedResult).Records.([]*types.Artifact)
	require.NotEqual(t, 0, len(recs))
}

func Test_ShouldUploadAndGetArtifact(t *testing.T) {
	// GIVEN artifact controller
	mgr := newTestArtifactManager(config.TestServerConfig(), t)
	webServer := web.NewStubWebServer()
	ctrl := NewArtifactController(mgr, webServer)

	// WHEN uploading artifact via post-body
	reader := io.NopCloser(strings.NewReader("test-data"))
	req := &http.Request{Body: reader}
	ctx := web.NewStubContext(req)
	err := ctrl.uploadArtifact(ctx)

	// THEN it should not fail and return artifact metadata
	require.NoError(t, err)
	artifact := ctx.Result.(*types.Artifact)
	require.NotEqual(t, "", artifact.ID)

	// WHEN getting artifact by id
	ctx.Params["id"] = artifact.ID
	err = ctrl.getArtifact(ctx)

	// THEN it should not fail and return artifact metadata
	require.NoError(t, err)
	artifact = ctx.Result.(*types.Artifact)
}

func Test_ShouldUploadAndDeleteArtifact(t *testing.T) {
	// GIVEN artifact controller
	mgr := newTestArtifactManager(config.TestServerConfig(), t)
	webServer := web.NewStubWebServer()
	ctrl := NewArtifactController(mgr, webServer)
	reader := io.NopCloser(strings.NewReader("test-data"))
	req := &http.Request{Body: reader}
	ctx := web.NewStubContext(req)

	// WHEN uploading artifact via post-body
	err := ctrl.uploadArtifact(ctx)

	// THEN it should not fail and return artifact metadata
	require.NoError(t, err)
	artifact := ctx.Result.(*types.Artifact)
	require.NotEqual(t, "", artifact.ID)

	// WHEN deleting artifact by id
	ctx.Params["id"] = artifact.ID
	err = ctrl.deleteArtifact(ctx)

	// THEN it should not fail
	require.NoError(t, err)
}

func newTestArtifactManager(serverCfg *config.ServerConfig, t *testing.T) *manager.ArtifactManager {
	artifactService, err := artifacts.NewStub(serverCfg.Common.S3)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
	artifactRepository, err := repository.NewTestArtifactRepository()
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
	artifactRepository.Clear()
	logRepository, err := repository.NewTestLogEventRepository()
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}

	art := types.NewArtifact("bucket", ulid.Make().String(), "group", "kind", "101", "sha", 100)
	art.ID = ulid.Make().String()
	_, _ = artifactRepository.Save(art)
	mgr, err := manager.NewArtifactManager(
		serverCfg,
		logRepository,
		artifactRepository,
		artifactService)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
	return mgr
}
