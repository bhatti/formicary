// SPDX-License-Identifier: AGPL-3.0-or-later

package tasklet

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/internal/acl"
	"plexobject.com/formicary/internal/metrics"
	"plexobject.com/formicary/internal/queue"
	"plexobject.com/formicary/internal/tasklet"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/resource"
)

func newTestFanOutTasklet(t *testing.T) (*FanOutTasklet, *manager.JobManager) {
	t.Helper()
	jobManager := manager.AssertTestJobManager(nil, t)
	cfg := config.TestServerConfig()
	queueClient, _ := queue.NewClientManager().GetClient(context.Background(), &cfg.Common)
	resourceManager := resource.NewStub()
	resourceManager.Registry["ant-1"] = &common.AntRegistration{
		AntID:       "ant-1",
		AntTopic:    "ant-1-topic",
		MaxCapacity: 100,
		Tags:        make([]string, 0),
		Methods:     []common.TaskMethod{common.Shell},
		Allocations: make(map[string]*common.AntAllocation),
	}
	requestRegistry := tasklet.NewRequestRegistry(&cfg.Common, metrics.New())

	// Wire the queue client to return COMPLETED for any TaskRequest sent via SendReceive.
	if channelClient, ok := queueClient.(*queue.ClientChannel); ok {
		channelClient.SetSendReceivePayloadFunc(func(_ context.Context, inReq *queue.SendReceiveRequest) ([]byte, error) {
			var req common.TaskRequest
			if err := json.Unmarshal(inReq.Payload, &req); err != nil {
				return nil, err
			}
			resp := common.NewTaskResponse(&req)
			resp.AntID = "ant-1"
			resp.Host = "test"
			resp.Status = common.COMPLETED
			// Echo the item_var back as a context variable so callers can verify injection.
			if v, ok := req.Variables["region"]; ok {
				resp.AddContext("deployed_region", v.Value)
			}
			if v, ok := req.Variables["dataset"]; ok {
				resp.AddContext("processed_dataset", v.Value)
			}
			return json.Marshal(resp)
		})
	}

	ft := NewFanOutTasklet(cfg, requestRegistry, resourceManager, jobManager, queueClient, "fan-out-topic")
	return ft, jobManager
}

func buildFanOutTaskRequest(regions []string) *common.TaskRequest {
	arr, _ := json.Marshal(regions)
	req := &common.TaskRequest{
		JobType:         "test-job",
		TaskType:        "deploy",
		JobRequestID:    "req-001",
		JobExecutionID:  "exec-001",
		TaskExecutionID: "task-001",
		UserID:          "user-1",
		OrganizationID:  "",
		Action:          common.EXECUTE,
		Script:          []string{"echo deploying to {{.region}}"},
		ExecutorOpts:    common.NewExecutorOptions("", common.FanOutJob),
		Variables:       map[string]common.VariableValue{},
	}
	req.Variables["regions"] = common.NewVariableValue(string(arr), false)
	req.ExecutorOpts.FanOut = &common.FanOutConfig{
		Source:          "regions",
		ItemVar:         "region",
		MaxParallel:     2,
		FailFast:        false,
		ExecutionMethod: common.Shell,
	}
	return req
}

func Test_FanOutTasklet_TerminateContainerReturnsError(t *testing.T) {
	ft, _ := newTestFanOutTasklet(t)
	_, err := ft.TerminateContainer(context.Background(), nil)
	require.Error(t, err)
}

func Test_FanOutTasklet_PreExecuteReturnsTrue(t *testing.T) {
	ft, _ := newTestFanOutTasklet(t)
	require.True(t, ft.PreExecute(context.Background(), nil))
}

func Test_FanOutTasklet_ListContainersReturnsCompleted(t *testing.T) {
	ft, _ := newTestFanOutTasklet(t)
	req := &common.TaskRequest{ExecutorOpts: common.NewExecutorOptions("", common.FanOutJob)}
	resp, err := ft.ListContainers(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, common.COMPLETED, resp.Status)
}

func Test_FanOutTasklet_MissingFanOutConfigFails(t *testing.T) {
	ft, _ := newTestFanOutTasklet(t)
	req := &common.TaskRequest{
		JobType:      "test",
		TaskType:     "deploy",
		ExecutorOpts: common.NewExecutorOptions("", common.FanOutJob),
	}
	// No FanOut set on opts
	resp, err := ft.Execute(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, common.FAILED, resp.Status)
	require.Contains(t, resp.ErrorMessage, "fan_out configuration is missing")
}

func Test_FanOutTasklet_EmptySourceCompletesImmediately(t *testing.T) {
	ft, _ := newTestFanOutTasklet(t)
	req := buildFanOutTaskRequest([]string{}) // empty list
	resp, err := ft.Execute(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, common.COMPLETED, resp.Status)
	require.Equal(t, 0, resp.TaskContext["FanOutItemCount"])
}

func Test_FanOutTasklet_TaskFanOut_AllItemsDispatched(t *testing.T) {
	ft, _ := newTestFanOutTasklet(t)
	regions := []string{"us-east-1", "us-west-2", "eu-west-1"}
	req := buildFanOutTaskRequest(regions)

	resp, err := ft.Execute(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, common.COMPLETED, resp.Status)
	require.Equal(t, 3, resp.TaskContext["FanOutItemCount"])

	// Each item should have its status reflected under "{item_var}_{index}_status".
	require.Equal(t, string(common.COMPLETED), resp.TaskContext["region_0_status"])
	require.Equal(t, string(common.COMPLETED), resp.TaskContext["region_1_status"])
	require.Equal(t, string(common.COMPLETED), resp.TaskContext["region_2_status"])
}

func Test_FanOutTasklet_TaskFanOut_ItemVarInjected(t *testing.T) {
	ft, _ := newTestFanOutTasklet(t)
	regions := []string{"ap-southeast-1"}
	req := buildFanOutTaskRequest(regions)

	resp, err := ft.Execute(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, common.COMPLETED, resp.Status)
	// The mock echoes region value back as "deployed_region"; it should appear with prefix.
	require.Equal(t, "ap-southeast-1", resp.TaskContext["region_0_deployed_region"])
}

func Test_FanOutTasklet_TaskFanOut_MissingSourceVariableFails(t *testing.T) {
	ft, _ := newTestFanOutTasklet(t)
	req := &common.TaskRequest{
		JobType:         "test",
		TaskType:        "deploy",
		JobRequestID:    "req-002",
		JobExecutionID:  "exec-002",
		TaskExecutionID: "task-002",
		UserID:          "user-1",
		Action:          common.EXECUTE,
		ExecutorOpts:    common.NewExecutorOptions("", common.FanOutJob),
		Variables:       map[string]common.VariableValue{},
	}
	req.ExecutorOpts.FanOut = &common.FanOutConfig{
		Source:          "nonexistent_var",
		ItemVar:         "region",
		ExecutionMethod: common.Shell,
	}
	resp, err := ft.Execute(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, common.FAILED, resp.Status)
	require.Contains(t, resp.ErrorMessage, "nonexistent_var")
}

func Test_FanOutTasklet_TaskFanOut_FanOutModeInContext(t *testing.T) {
	ft, _ := newTestFanOutTasklet(t)
	req := buildFanOutTaskRequest([]string{"us-east-1"})
	resp, err := ft.Execute(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, common.COMPLETED, resp.Status)
	require.Equal(t, "task", resp.TaskContext["FanOutMode"])
}

func Test_FanOutTasklet_JobFanOut_SpawnsChildJobs(t *testing.T) {
	ft, jobManager := newTestFanOutTasklet(t)
	user := common.NewUser("", "fanout@formicary.io", "fan-out", "", acl.NewRoles(""))
	user.ID = "fanout-user"
	qc := common.NewQueryContext(user, "")
	job := repository.NewTestJobDefinition(user, "child-job")
	_, err := jobManager.SaveJobDefinition(qc, job)
	require.NoError(t, err)

	datasets := []string{"ds-a", "ds-b"}
	dataArr, _ := json.Marshal(datasets)

	req := &common.TaskRequest{
		JobType:         "parent-job",
		TaskType:        "process",
		JobRequestID:    "req-job-100",
		JobExecutionID:  "exec-job-100",
		TaskExecutionID: "task-job-100",
		UserID:          user.ID,
		OrganizationID:  user.OrganizationID,
		Action:          common.EXECUTE,
		ExecutorOpts:    common.NewExecutorOptions("", common.FanOutJob),
		Variables:       map[string]common.VariableValue{},
	}
	req.Variables["datasets"] = common.NewVariableValue(string(dataArr), false)
	req.ExecutorOpts.FanOut = &common.FanOutConfig{
		Source:      "datasets",
		ItemVar:     "dataset",
		MaxParallel: 2,
		ForkJobType: "io.formicary.test.child-job",
	}

	// Job fan-out spawns child jobs and then waits for them via JobWaiter.
	// In unit tests the children stay PENDING (no engine running) so the waiter
	// will time out. We poll until all expected child jobs appear, then cancel.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// Run Execute in background — it will block on JobWaiter until ctx times out.
	go func() {
		_, _ = ft.Execute(ctx, req)
	}()
	// Poll until both child jobs are visible in the DB, then cancel.
	for {
		select {
		case <-ctx.Done():
			goto check
		default:
		}
		allRequests, _, findErr := jobManager.QueryJobRequests(
			qc,
			map[string]interface{}{"job_type": "io.formicary.test.child-job"},
			0, 100, []string{},
		)
		if findErr == nil && len(allRequests) >= len(datasets) {
			cancel()
			goto check
		}
		// Yield before re-checking to avoid a busy-wait.
		select {
		case <-ctx.Done():
			goto check
		case <-time.After(10 * time.Millisecond):
		}
	}
check:

	// Verify two child jobs were created with correct params.
	allRequests, _, findErr := jobManager.QueryJobRequests(
		qc,
		map[string]interface{}{"job_type": "io.formicary.test.child-job"},
		0, 100, []string{},
	)
	require.NoError(t, findErr)

	for _, ds := range datasets {
		found := false
		for _, r := range allRequests {
			if p := r.GetParam("dataset"); p != nil && fmt.Sprintf("%v", p.Value) == ds {
				require.True(t, r.CascadeCancel, "cascade_cancel must be true on fan-out child job")
				require.Equal(t, "req-job-100", r.ParentID)
				found = true
				break
			}
		}
		require.True(t, found, "expected child job for dataset %s", ds)
	}
}

// ---------------------------------------------------------------------------
// resolveFanOutSource: unit tests for the source-resolution helper.
// These cover the exact failure mode seen in production:
//   fan_out.source 'regions' not found in job execution context
// ---------------------------------------------------------------------------

// Test_ResolveFanOutSource_JsonArrayVariable verifies that a Variables entry
// holding a JSON-encoded string array is parsed correctly.
func Test_ResolveFanOutSource_JsonArrayVariable(t *testing.T) {
	req := &common.TaskRequest{
		Variables: map[string]common.VariableValue{
			"regions": common.NewVariableValue(`["us-east-1","us-west-2","eu-west-1"]`, false),
		},
		ExecutorOpts: common.NewExecutorOptions("", common.FanOutJob),
	}
	items, err := resolveFanOutSource(req, "regions")
	require.NoError(t, err)
	require.Len(t, items, 3)
	require.Equal(t, "us-east-1", items[0])
	require.Equal(t, "us-west-2", items[1])
	require.Equal(t, "eu-west-1", items[2])
}

// Test_ResolveFanOutSource_MissingVariableFails verifies the exact error that
// was seen in production when job_variables was not set.
func Test_ResolveFanOutSource_MissingVariableFails(t *testing.T) {
	req := &common.TaskRequest{
		Variables:    map[string]common.VariableValue{},
		ExecutorOpts: common.NewExecutorOptions("", common.FanOutJob),
	}
	_, err := resolveFanOutSource(req, "regions")
	require.Error(t, err)
	require.Contains(t, err.Error(), "regions")
	require.Contains(t, err.Error(), "not found")
}

// Test_ResolveFanOutSource_InvalidJsonFails verifies the error when the
// source variable exists but is not a valid JSON array.
func Test_ResolveFanOutSource_InvalidJsonFails(t *testing.T) {
	req := &common.TaskRequest{
		Variables: map[string]common.VariableValue{
			"regions": common.NewVariableValue("not-valid-json", false),
		},
		ExecutorOpts: common.NewExecutorOptions("", common.FanOutJob),
	}
	_, err := resolveFanOutSource(req, "regions")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not a valid JSON array")
}

// Test_ResolveFanOutSource_NativeSlicePassthrough verifies that a Variables
// entry that is already a []interface{} (e.g. from in-memory construction)
// is returned directly without re-parsing.
func Test_ResolveFanOutSource_NativeSlicePassthrough(t *testing.T) {
	req := &common.TaskRequest{
		Variables:    map[string]common.VariableValue{},
		ExecutorOpts: common.NewExecutorOptions("", common.FanOutJob),
	}
	req.Variables["items"] = common.VariableValue{
		Value:  []interface{}{"a", "b", "c"},
		Secret: false,
	}
	items, err := resolveFanOutSource(req, "items")
	require.NoError(t, err)
	require.Len(t, items, 3)
	require.Equal(t, "a", items[0])
}

// Test_ResolveFanOutSource_DatasetsVariable verifies the job fan-out case
// (datasets → child ETL jobs) uses the same resolution path.
func Test_ResolveFanOutSource_DatasetsVariable(t *testing.T) {
	req := &common.TaskRequest{
		Variables: map[string]common.VariableValue{
			"datasets": common.NewVariableValue(`["sales_2024","inventory_2024","orders_2024"]`, false),
		},
		ExecutorOpts: common.NewExecutorOptions("", common.FanOutJob),
	}
	items, err := resolveFanOutSource(req, "datasets")
	require.NoError(t, err)
	require.Len(t, items, 3)
	require.Equal(t, "sales_2024", items[0])
}
