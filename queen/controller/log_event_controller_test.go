// SPDX-License-Identifier: AGPL-3.0-or-later

package controller

import (
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/internal/events"
	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/repository"
)

func newTestLogEvent(requestID, msg, level string) *events.LogEvent {
	ev := events.NewLogEvent("task", "user1", requestID, "job-type", "task-type", "exec1", "task-exec1", msg, "", "ant1")
	ev.Level = level
	return ev
}

func Test_ShouldQueryLogsByRequestID(t *testing.T) {
	// GIVEN a log event controller backed by a real in-memory repository
	logRepo, err := repository.NewTestLogEventRepository()
	require.NoError(t, err)
	webServer := web.NewStubWebServer()
	ctrl := NewLogEventController(logRepo, webServer)

	const requestID = "req-test-001"

	ev1 := newTestLogEvent(requestID, "hello world", "info")
	_, err = logRepo.Save(ev1)
	require.NoError(t, err)

	ev2 := newTestLogEvent(requestID, "second line", "warn")
	_, err = logRepo.Save(ev2)
	require.NoError(t, err)

	// A log for a different request — must NOT appear in the result
	otherEv := newTestLogEvent("other-req-999", "unrelated", "info")
	_, err = logRepo.Save(otherEv)
	require.NoError(t, err)

	// WHEN querying by request ID
	ctx := web.NewStubContext(&http.Request{Body: io.NopCloser(nil), URL: &url.URL{}})
	ctx.Params["id"] = requestID
	err = ctrl.queryLogs(ctx)

	// THEN the response contains only logs for the given request
	require.NoError(t, err)
	resp := ctx.Result.(map[string]interface{})
	records := resp["records"].([]logRecord)
	require.Len(t, records, 2)
	require.Equal(t, "hello world", records[0].Message)
	require.Equal(t, "second line", records[1].Message)
	require.Equal(t, int64(2), resp["total"].(int64))
}

func Test_ShouldReturnBadRequestWhenRequestIDMissing(t *testing.T) {
	// GIVEN a log event controller
	logRepo, err := repository.NewTestLogEventRepository()
	require.NoError(t, err)
	webServer := web.NewStubWebServer()
	ctrl := NewLogEventController(logRepo, webServer)

	// WHEN querying without an id param
	ctx := web.NewStubContext(&http.Request{Body: io.NopCloser(nil), URL: &url.URL{}})
	err = ctrl.queryLogs(ctx)

	// THEN the stub returns a 400 error (stub JSON returns error for non-2xx status)
	require.Error(t, err)
	require.Contains(t, err.Error(), "request id")
}

func Test_ShouldRespectLimitParam(t *testing.T) {
	// GIVEN a log event controller with 5 records
	logRepo, err := repository.NewTestLogEventRepository()
	require.NoError(t, err)
	webServer := web.NewStubWebServer()
	ctrl := NewLogEventController(logRepo, webServer)

	const reqID = "req-limit-test"
	for i := 0; i < 5; i++ {
		ev := newTestLogEvent(reqID, "line", "info")
		_, err = logRepo.Save(ev)
		require.NoError(t, err)
	}

	// WHEN querying with limit=2
	ctx := web.NewStubContext(&http.Request{Body: io.NopCloser(nil), URL: &url.URL{}})
	ctx.Params["id"] = reqID
	ctx.Params["limit"] = "2"
	err = ctrl.queryLogs(ctx)

	require.NoError(t, err)
	resp := ctx.Result.(map[string]interface{})
	records := resp["records"].([]logRecord)
	require.Len(t, records, 2)
}
