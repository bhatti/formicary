package buildversion

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_VersionStringDevMode(t *testing.T) {
	// Makefile produces: version=0.1.69, commit=v0.1.67-1-g5c60db2-dirty
	v := New("0.1.69", "v0.1.67-1-g5c60db2-dirty", "2026-06-09T03:56:37", "formicary")
	s := v.String()
	// Should be a single token, not duplicated
	require.NotContains(t, s, " ", "version string must not contain spaces (no duplication)")
	require.True(t, strings.HasPrefix(s, "v0.1.69"), "must start with the semantic version")
	require.Contains(t, s, "g5c60db2", "must include short git hash")
	require.Contains(t, s, "dirty", "must include dirty flag")
}

func Test_VersionStringDockerMode(t *testing.T) {
	// Dockerfile produces: version=APP_VERSION (e.g. 0.1.0), commit=short hash (abc1234)
	v := New("0.1.0", "abc1234", "2026-06-09T03:56:37", "formicary")
	s := v.String()
	require.NotContains(t, s, " ", "version string must not contain spaces")
	require.True(t, strings.HasPrefix(s, "v0.1.0"), "must start with the semantic version")
	require.Contains(t, s, "gabc1234", "must include short hash prefixed with g")
}

func Test_VersionStringFallback(t *testing.T) {
	// Empty version falls back to commit
	v := New("", "abc1234", "2026-06-09T03:56:37", "formicary")
	s := v.String()
	require.Equal(t, "abc1234", s)
}

func Test_VersionStringVersionEqualsCommit(t *testing.T) {
	v := New("abc1234", "abc1234", "2026-06-09T03:56:37", "formicary")
	s := v.String()
	require.Equal(t, "abc1234", s)
}
