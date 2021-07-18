package utils

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_ShouldMatchTags(t *testing.T) {
	require.Error(t, MatchTags("org, dynamic, cbot", "org static seattle"))
	require.NoError(t, MatchTags("org, dynamic, cbot", "dynamic;org;cbot"))
	require.Error(t, MatchTags("chicago;org;static", "org static seattle"))
	require.NoError(t, MatchTags("seattle;chicago;org;static", "org static chicago"))
	require.NoError(t, MatchTags("seattle;chicago;org;static", "org static seattle"))
	require.NoError(t, MatchTags("seattle;chicago;org;static", "org static chicago"))
	require.NoError(t, MatchTags("input body", ""))
	require.Error(t, MatchTags("input body", "input html"))
	require.NoError(t, MatchTags("input html body", "input body"))
	require.Error(t, MatchTags("", "input html"))
	require.NoError(t, MatchTags("", ""))
}

func Test_ShouldEmptyOrWildString(t *testing.T) {
	require.True(t, isWildMatches("", ""))
	require.True(t, isWildMatches("*", ""))
	require.True(t, isWildMatches("", "*"))
	require.True(t, isWildMatches("*", "*"))
	require.True(t, isWildMatches("blah", "*"))
	require.True(t, isWildMatches("*", "blah"))
	require.False(t, isWildMatches("Nah", "blah"))
}

func Test_ShouldReplaceDirPath(t *testing.T) {
	require.EqualValues(t, []string{"/source/one", "/source/two"}, ReplaceDirPath([]string{"/source/one", "/source/two"}, ""))
	require.EqualValues(t, []string{"/target/one", "/target/two"}, ReplaceDirPath([]string{"/source/one", "/source/two"}, "/target"))
}
