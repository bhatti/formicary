package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_ShouldValidateLocalMode(t *testing.T) {
	c := &S3Config{LocalMode: true}
	require.NoError(t, c.Validate())
	require.Equal(t, "localkey", c.AccessKeyID)
	require.Equal(t, "localsecret", c.SecretAccessKey)
	require.Equal(t, "formicary-artifacts", c.Bucket)
	require.Equal(t, "us-east-1", c.Region)
	require.Equal(t, "./data/seaweedfs", c.LocalDataDir)
	require.Equal(t, "weed", c.LocalWeedBin)
}

func Test_ShouldPreserveExplicitLocalModeFields(t *testing.T) {
	c := &S3Config{
		LocalMode:    true,
		LocalDataDir: "/custom/path",
		LocalWeedBin: "/usr/local/bin/weed",
		Bucket:       "my-bucket",
	}
	require.NoError(t, c.Validate())
	require.Equal(t, "/custom/path", c.LocalDataDir)
	require.Equal(t, "/usr/local/bin/weed", c.LocalWeedBin)
	require.Equal(t, "my-bucket", c.Bucket)
}

func Test_ShouldRejectExternalModeWithoutCredentials(t *testing.T) {
	c := &S3Config{}
	require.Error(t, c.Validate())
}

func Test_ShouldValidateExternalMode(t *testing.T) {
	c := &S3Config{
		Endpoint:        "s3.amazonaws.com",
		AccessKeyID:     "key",
		SecretAccessKey: "secret",
		Bucket:          "my-bucket",
	}
	require.NoError(t, c.Validate())
}

func Test_ShouldDefaultEndpointAndRegionInExternalMode(t *testing.T) {
	c := &S3Config{
		AccessKeyID:     "key",
		SecretAccessKey: "secret",
		Bucket:          "bucket",
	}
	require.NoError(t, c.Validate())
	require.Equal(t, "s3.amazonaws.com", c.Endpoint)
	require.Equal(t, "US-WEST-2", c.Region)
}

func Test_ShouldIsLocalModeReturnCorrectValue(t *testing.T) {
	require.True(t, (&S3Config{LocalMode: true}).IsLocalMode())
	require.False(t, (&S3Config{}).IsLocalMode())
}

func Test_ShouldLocalContainerEndpointDefaultsToDockerInternal(t *testing.T) {
	c := &S3Config{LocalMode: true, Endpoint: "localhost:9876"}
	ep := c.LocalContainerEndpoint()
	require.Equal(t, "host.docker.internal:9876", ep)
}

func Test_ShouldLocalContainerEndpointUsesCustomHost(t *testing.T) {
	c := &S3Config{LocalMode: true, Endpoint: "localhost:9876", LocalContainerHost: "172.17.0.1"}
	ep := c.LocalContainerEndpoint()
	require.Equal(t, "172.17.0.1:9876", ep)
}

func Test_ShouldLocalContainerEndpointFallsBackToDefaultPort(t *testing.T) {
	// Endpoint not set yet (before subprocess assigns it)
	c := &S3Config{LocalMode: true}
	ep := c.LocalContainerEndpoint()
	require.Equal(t, "host.docker.internal:8333", ep)
}

func Test_ShouldBuildEndpointWithHTTP(t *testing.T) {
	c := &S3Config{Endpoint: "localhost:9000", UseSSL: false}
	require.Equal(t, "http://localhost:9000", c.BuildEndpoint())
}

func Test_ShouldBuildEndpointWithHTTPS(t *testing.T) {
	c := &S3Config{Endpoint: "s3.example.com", UseSSL: true}
	require.Equal(t, "https://s3.example.com", c.BuildEndpoint())
}
