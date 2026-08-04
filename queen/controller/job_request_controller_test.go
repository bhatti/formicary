package controller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/url"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/manager"
	"strings"
	"testing"

	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/stats"
	"plexobject.com/formicary/queen/types"
)

func Test_InitializeSwaggerStructsForJobRequest(t *testing.T) {
	_ = jobRequestQueryParams{}
	_ = jobRequestQueryResponseBody{}
	_ = jobRequestIDParams{}
	_ = jobRequestParams{}
	_ = jobRequestBody{}
	_ = jobRequestWaitTimesBody{}
	_ = jobRequestStatsBody{}
	_ = jobRequestIDsBody{}
}

func Test_ShouldQueryJobRequests(t *testing.T) {
	// GIVEN job-request controller
	mgr := manager.AssertTestJobManager(nil, t)
	webServer := web.NewStubWebServer()
	ctrl := NewJobRequestController(mgr, webServer)
	_, err := addJobRequest(mgr)
	require.NoError(t, err)

	// WHEN querying jobs
	reader := io.NopCloser(strings.NewReader(""))
	req := &http.Request{Body: reader, URL: &url.URL{}}
	ctx := web.NewStubContext(req)
	err = ctrl.queryJobRequests(ctx)

	// THEN it should return job requests
	require.NoError(t, err)
	jobs := ctx.Result.(*PaginatedResult).Records.([]*types.JobRequest)
	require.NotEqual(t, 0, len(jobs))
}

func Test_ShouldGetJobRequests(t *testing.T) {
	// GIVEN job-request controller
	mgr := manager.AssertTestJobManager(nil, t)
	webServer := web.NewStubWebServer()
	ctrl := NewJobRequestController(mgr, webServer)
	jobReq, err := addJobRequest(mgr)
	require.NoError(t, err)

	// WHEN getting job by id
	reader := io.NopCloser(strings.NewReader(""))
	req := &http.Request{Body: reader, URL: &url.URL{}}
	ctx := web.NewStubContext(req)
	ctx.Params["id"] = jobReq.ID
	err = ctrl.getJobRequest(ctx)
	// THEN it should return job request
	require.NoError(t, err)
	saved := ctx.Result.(*types.JobRequest)
	require.NotNil(t, saved)
}

func Test_ShouldStatsJobRequests(t *testing.T) {
	// GIVEN job-request controller
	mgr := manager.AssertTestJobManager(nil, t)
	webServer := web.NewStubWebServer()
	ctrl := NewJobRequestController(mgr, webServer)
	_, err := addJobRequest(mgr)
	require.NoError(t, err)

	// WHEN getting stats
	reader := io.NopCloser(strings.NewReader(""))
	req := &http.Request{Body: reader, URL: &url.URL{}}
	ctx := web.NewStubContext(req)
	err = ctrl.statsJobRequests(ctx)

	// THEN it should return stats
	require.NoError(t, err)
	jobStats := ctx.Result.([]*types.JobCounts)
	require.NotNil(t, jobStats)
}

func Test_ShouldSubmitJobRequest(t *testing.T) {
	// GIVEN job-request controller
	mgr := manager.AssertTestJobManager(nil, t)
	jobDef, err := getTestJobDefinition(mgr)
	require.NoError(t, err)
	jobReq, err := types.NewJobRequestFromDefinition(jobDef)
	require.NoError(t, err)
	b, err := json.Marshal(jobReq)
	require.NoError(t, err)
	webServer := web.NewStubWebServer()
	ctrl := NewJobRequestController(mgr, webServer)

	// WHEN submitting job
	reader := io.NopCloser(bytes.NewReader(b))
	ctx := web.NewStubContext(&http.Request{Body: reader, Header: map[string][]string{"content-type": {"application/json"}}})
	err = ctrl.submitJobRequest(ctx)

	// THEN it should return saved job-request
	require.NoError(t, err)
	savedJob := ctx.Result.(*types.JobRequest)
	require.NotNil(t, savedJob)

	// WHEN getting job-request
	ctx.Params["id"] = savedJob.ID
	err = ctrl.getJobRequest(ctx)
	loadedJob := ctx.Result.(*types.JobRequest)

	// THEN it should return saved job-request
	require.NoError(t, err)
	require.NotNil(t, loadedJob)
}

func Test_ShouldGetWaitTimes(t *testing.T) {
	// GIVEN job-request controller
	mgr := manager.AssertTestJobManager(nil, t)
	webServer := web.NewStubWebServer()
	ctrl := NewJobRequestController(mgr, webServer)
	jobDef, err := addJobRequest(mgr)
	require.NoError(t, err)

	// WHEN getting wait-time of job-request
	reader := io.NopCloser(strings.NewReader(""))
	ctx := web.NewStubContext(&http.Request{Body: reader, Header: map[string][]string{"content-type": {"application/json"}}})
	ctx.Params["id"] = jobDef.ID
	err = ctrl.getWaitTimeJobRequest(ctx)

	// THEN it should return wait-time job-request
	require.NoError(t, err)
	estimate := ctx.Result.(stats.JobWaitEstimate)
	require.Equal(t, 0, estimate.QueueNumber)
}

func Test_ShouldCancelJobRequest(t *testing.T) {
	// GIVEN job-request controller
	mgr := manager.AssertTestJobManager(nil, t)
	webServer := web.NewStubWebServer()
	ctrl := NewJobRequestController(mgr, webServer)
	jobDef, err := addJobRequest(mgr)
	require.NoError(t, err)

	// WHEN canceling job-request
	reader := io.NopCloser(strings.NewReader(""))
	ctx := web.NewStubContext(&http.Request{Body: reader, Header: map[string][]string{"content-type": {"application/json"}}})
	ctx.Params["id"] = jobDef.ID
	err = ctrl.cancelJobRequest(ctx)

	// THEN it should not fail
	require.NoError(t, err)
}

// makeTestCronJobRequest creates a cron job definition and returns the PENDING request
// that SaveJobDefinition auto-creates. Extra vars are added to the definition and as
// params on the auto-created request (via a second Save that updates the existing row).
func makeTestCronJobRequest(t *testing.T, mgr *manager.JobManager, extraVars ...string) (*types.JobRequest, error) {
	t.Helper()
	qc := common.NewQueryContext(nil, "")
	jobDef := types.NewJobDefinition("io.formicary.test.trigger-ctrl-" + t.Name())
	_, _ = jobDef.AddVariable("jk1", "v1")
	_, _ = jobDef.AddVariable("jk2", "v2")
	for i := 0; i+1 < len(extraVars); i += 2 {
		_, _ = jobDef.AddVariable(extraVars[i], extraVars[i+1])
	}
	task := types.NewTaskDefinition("task1", common.Shell)
	task.Script = []string{"echo test"}
	jobDef.AddTask(task)
	jobDef.CronTrigger = "0 0 * * * * *"
	jobDef.UpdateRawYaml()
	jobDef, err := mgr.SaveJobDefinition(qc, jobDef)
	if err != nil {
		return nil, err
	}
	// SaveJobDefinition auto-creates the cron PENDING request; find it by user_key.
	_, userKey := jobDef.GetCronScheduleTimeAndUserKey()
	jobReq, err := mgr.GetJobRequestByUserKey(qc, userKey)
	if err != nil {
		return nil, fmt.Errorf("cron request not found by user_key %s: %w", userKey, err)
	}
	// Persist initial param values so they exist in the DB for the trigger test.
	for i := 0; i+1 < len(extraVars); i += 2 {
		_, _ = jobReq.AddParam(extraVars[i], extraVars[i+1])
	}
	if len(extraVars) > 0 {
		jobReq, err = mgr.SaveJobRequest(qc, jobReq)
		if err != nil {
			return nil, err
		}
	}
	return jobReq, nil
}

func Test_ShouldTriggerJobRequestWithoutParams(t *testing.T) {
	// GIVEN a cron job request in PENDING state
	mgr := manager.AssertTestJobManager(nil, t)
	jobReq, err := makeTestCronJobRequest(t, mgr)
	require.NoError(t, err)
	webServer := web.NewStubWebServer()
	ctrl := NewJobRequestController(mgr, webServer)

	// WHEN triggering with an empty body (no params)
	reader := io.NopCloser(strings.NewReader(""))
	ctx := web.NewStubContext(&http.Request{Body: reader, Header: map[string][]string{"content-type": {"application/json"}}})
	ctx.Params["id"] = jobReq.ID
	err = ctrl.triggerJobRequest(ctx)

	// THEN it should succeed without error (params left unchanged)
	require.NoError(t, err)
}

func Test_ShouldTriggerJobRequestWithParams(t *testing.T) {
	// GIVEN a cron job request with SlackChannel and SlackThreadTs variables
	mgr := manager.AssertTestJobManager(nil, t)
	qc := common.NewQueryContext(nil, "")
	jobReq, err := makeTestCronJobRequest(t, mgr, "SlackChannel", "old-channel", "SlackThreadTs", "")
	require.NoError(t, err)

	webServer := web.NewStubWebServer()
	ctrl := NewJobRequestController(mgr, webServer)

	// WHEN triggering with a JSON body containing updated params
	body := `{"params":{"SlackChannel":"C-NEW","SlackThreadTs":"9999.000"}}`
	reader := io.NopCloser(strings.NewReader(body))
	ctx := web.NewStubContext(&http.Request{
		Body:   reader,
		Header: map[string][]string{"content-type": {"application/json"}},
	})
	ctx.Params["id"] = jobReq.ID
	err = ctrl.triggerJobRequest(ctx)

	// THEN it should succeed and the params should be updated
	require.NoError(t, err)
	loaded, err := mgr.GetJobRequest(qc, jobReq.ID)
	require.NoError(t, err)
	require.Equal(t, "C-NEW", fmt.Sprintf("%v", loaded.NameValueParams["SlackChannel"]))
	require.Equal(t, "9999.000", fmt.Sprintf("%v", loaded.NameValueParams["SlackThreadTs"]))
}

func Test_ShouldRestartJobRequest(t *testing.T) {
	// GIVEN job-request controller
	mgr := manager.AssertTestJobManager(nil, t)
	jobDef, err := addJobRequest(mgr)
	require.NoError(t, err)

	err = mgr.CancelJobRequest(common.NewQueryContext(nil, ""), jobDef.ID)
	require.NoError(t, err)
	webServer := web.NewStubWebServer()
	ctrl := NewJobRequestController(mgr, webServer)

	// WHEN restarting job-request
	reader := io.NopCloser(strings.NewReader(""))
	ctx := web.NewStubContext(&http.Request{Body: reader, Header: map[string][]string{"content-type": {"application/json"}}})
	ctx.Params["id"] = jobDef.ID
	err = ctrl.restartJobRequest(ctx)

	// THEN it should not fail
	require.NoError(t, err)
}

func Test_ShouldGetRecentDeadIDs(t *testing.T) {
	// GIVEN job-request controller
	mgr := manager.AssertTestJobManager(nil, t)
	webServer := web.NewStubWebServer()
	ctrl := NewJobRequestController(mgr, webServer)
	req, err := addJobRequest(mgr)
	require.NoError(t, err)
	reader := io.NopCloser(strings.NewReader(""))
	ctx := web.NewStubContext(&http.Request{Body: reader, Header: map[string][]string{"content-type": {"application/json"}}})
	ctx.Set(web.DBUser, &common.User{ID: req.UserID})

	// WHEN fetching recently completed job-ids
	err = ctrl.getDeadIDs(ctx)

	// THEN it should not fail
	require.NoError(t, err)
	ids := ctx.Result.([]string)
	require.NotNil(t, ids)
}

func getTestJobDefinition(mgr *manager.JobManager) (*types.JobDefinition, error) {
	// GIVEN job-request controller
	jobStatsRegistry := stats.NewJobStatsRegistry()
	webServer := web.NewStubWebServer()
	jobDefCtrl := NewJobDefinitionController(mgr, jobStatsRegistry, webServer)
	reader := io.NopCloser(strings.NewReader(""))
	req := &http.Request{Body: reader, URL: &url.URL{}}
	ctx := web.NewStubContext(req)

	// WHEN querying job definitions
	err := jobDefCtrl.queryJobDefinitions(ctx)
	// THEN it should not fail and return jobs
	if err != nil {
		return nil, err
	}
	jobs := ctx.Result.(*PaginatedResult).Records.([]*types.JobDefinition)
	return jobs[0], nil
}

func addJobRequest(mgr *manager.JobManager) (*types.JobRequest, error) {
	jobDef, err := getTestJobDefinition(mgr)
	if err != nil {
		return nil, err
	}
	jobReq, err := types.NewJobRequestFromDefinition(jobDef)
	if err != nil {
		return nil, err
	}
	jobReq, err = mgr.SaveJobRequest(common.NewQueryContext(nil, ""), jobReq)
	if err != nil {
		return nil, err
	}
	return jobReq, nil
}
