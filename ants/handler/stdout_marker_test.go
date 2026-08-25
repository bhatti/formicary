package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"plexobject.com/formicary/internal/types"
)

// --------------------------------------------------------------------------
// parseStdoutMarker — unit tests
// --------------------------------------------------------------------------

func TestParseStdoutMarker_Valid(t *testing.T) {
	cases := []struct {
		name      string
		suffix    string
		wantKey   string
		wantValue string
	}{
		{"simple", "IssueNumber::42", "IssueNumber", "42"},
		{"empty value", "KEY::", "KEY", ""},
		{"value with colon", "URL::https://example.com", "URL", "https://example.com"},
		{"value contains double colon", "PAIR::a::b", "PAIR", "a::b"},
		{"key with inner spaces trimmed", "  MY_KEY ::hello", "MY_KEY", "hello"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			k, v, ok := parseStdoutMarker(tc.suffix)
			assert.True(t, ok, "expected ok=true")
			assert.Equal(t, tc.wantKey, k)
			assert.Equal(t, tc.wantValue, v)
		})
	}
}

func TestParseStdoutMarker_Invalid(t *testing.T) {
	cases := []struct {
		name   string
		suffix string
	}{
		{"no separator", "KEYVALUE"},
		{"empty input", ""},
		{"only separator", "::value"},
		{"whitespace-only key", "   ::value"},
		{"separator at start", "::nokey"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, ok := parseStdoutMarker(tc.suffix)
			assert.False(t, ok, "expected ok=false for suffix %q", tc.suffix)
		})
	}
}

// --------------------------------------------------------------------------
// parseAndApplyStdoutMarkers — integration tests via TaskResponse
// --------------------------------------------------------------------------

func newTestResponse() *types.TaskResponse {
	return &types.TaskResponse{
		TaskContext: make(map[string]interface{}),
		JobContext:  make(map[string]interface{}),
	}
}

func TestParseAndApplyStdoutMarkers_AddJobContext(t *testing.T) {
	resp := newTestResponse()
	parseAndApplyStdoutMarkers("::add-job-context IssueNumber::42\n", resp)
	assert.Equal(t, "42", resp.JobContext["IssueNumber"])
	assert.Empty(t, resp.TaskContext)
}

func TestParseAndApplyStdoutMarkers_AddTaskContext(t *testing.T) {
	resp := newTestResponse()
	parseAndApplyStdoutMarkers("::add-task-context SELECTED_MODEL::claude-3-5-sonnet\n", resp)
	assert.Equal(t, "claude-3-5-sonnet", resp.TaskContext["SELECTED_MODEL"])
	assert.Empty(t, resp.JobContext)
}

func TestParseAndApplyStdoutMarkers_BothMarkers(t *testing.T) {
	stdout := `
Some regular output
::add-job-context PR_URL::https://github.com/org/repo/pull/1
::add-task-context SELECTED_TRACKER::jira
More output
`
	resp := newTestResponse()
	parseAndApplyStdoutMarkers(stdout, resp)
	assert.Equal(t, "https://github.com/org/repo/pull/1", resp.JobContext["PR_URL"])
	assert.Equal(t, "jira", resp.TaskContext["SELECTED_TRACKER"])
}

func TestParseAndApplyStdoutMarkers_NonMarkerLinesIgnored(t *testing.T) {
	stdout := "normal line\n::not-a-known-marker KEY::val\nanother line\n"
	resp := newTestResponse()
	parseAndApplyStdoutMarkers(stdout, resp)
	assert.Empty(t, resp.JobContext)
	assert.Empty(t, resp.TaskContext)
}

func TestParseAndApplyStdoutMarkers_EmptyLines(t *testing.T) {
	resp := newTestResponse()
	parseAndApplyStdoutMarkers("\n\n\n", resp)
	assert.Empty(t, resp.JobContext)
	assert.Empty(t, resp.TaskContext)
}

func TestParseAndApplyStdoutMarkers_EmptyValue(t *testing.T) {
	resp := newTestResponse()
	parseAndApplyStdoutMarkers("::add-task-context EMPTY_KEY::\n", resp)
	assert.Equal(t, "", resp.TaskContext["EMPTY_KEY"])
}

func TestParseAndApplyStdoutMarkers_MalformedMarkerIgnored(t *testing.T) {
	resp := newTestResponse()
	// missing :: separator after key
	parseAndApplyStdoutMarkers("::add-job-context NOKEYVALUE\n", resp)
	assert.Empty(t, resp.JobContext)
}

func TestParseAndApplyStdoutMarkers_MultipleJobContextEntries(t *testing.T) {
	stdout := `
::add-job-context BranchName::feature/abc
::add-job-context CommitCount::3
::add-job-context PRUrl::https://example.com/pr/7
`
	resp := newTestResponse()
	parseAndApplyStdoutMarkers(stdout, resp)
	assert.Equal(t, "feature/abc", resp.JobContext["BranchName"])
	assert.Equal(t, "3", resp.JobContext["CommitCount"])
	assert.Equal(t, "https://example.com/pr/7", resp.JobContext["PRUrl"])
}

func TestParseAndApplyStdoutMarkers_ValueWithDoubleColon(t *testing.T) {
	resp := newTestResponse()
	parseAndApplyStdoutMarkers("::add-task-context PAIR::a::b\n", resp)
	// only first :: splits; rest is the value
	assert.Equal(t, "a::b", resp.TaskContext["PAIR"])
}

