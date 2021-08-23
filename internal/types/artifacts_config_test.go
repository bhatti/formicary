package types

import (
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func Test_ShouldCreateArtifactsConfig(t *testing.T) {
	// Given artifacts config
	ac := NewArtifactsConfig()
	path, tm := ac.GetPathsAndExpiration(true)
	require.Equal(t, 0, len(path))
	require.NotNil(t, tm)
	require.NotNil(t, ac.Expiration())
	ac.ExpiresAfter = 2 * time.Second
	ac.Paths = []string{"a", "b"}
	path, tm = ac.GetPathsAndExpiration(true)
	require.Equal(t, 2, len(path))
	require.NotNil(t, tm)

	path, tm = ac.GetPathsAndExpiration(false)
	require.Equal(t, 2, len(path))
	require.NotNil(t, tm)
	require.NotNil(t, ac.Expiration())
}

func Test_ShouldCreateCacheConfig(t *testing.T) {
	// Given cache config
	ac := NewCacheConfig()
	require.False(t, ac.Valid())
	require.NotNil(t, ac.Expiration())
	ac.ExpiresAfter = 2 * time.Second
	require.False(t, ac.Valid())
	require.NotEqual(t, "", ac.String())
	ac.Key = "abc"
	ac.Paths = []string{"a"}
	require.True(t, ac.Valid())
	require.NotNil(t, ac.Expiration())
	require.NotEqual(t, "", ac.String())
	ac.KeyPaths = []string {"a"}
	require.NotEqual(t, "", ac.String())
}
