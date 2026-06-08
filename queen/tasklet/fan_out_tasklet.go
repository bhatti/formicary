// SPDX-License-Identifier: AGPL-3.0-or-later

package tasklet

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"plexobject.com/formicary/internal/events"
	"plexobject.com/formicary/internal/queue"
	"plexobject.com/formicary/internal/tasklet"
	"plexobject.com/formicary/internal/tracing"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/resource"
	qtypes "plexobject.com/formicary/queen/types"
	"plexobject.com/formicary/queen/utils"
)

// FanOutTasklet handles tasks that have fan_out configured.
//
// Two modes:
//
//   - Task fan-out (default): when fan_out has no fork_job_type, dispatches one
//     TaskRequest per array item directly to an ant worker using the task's original
//     execution method (SHELL, KUBERNETES, etc.). Uses ResourceManager.Reserve +
//     QueueClient.SendReceive, identical to TaskSupervisor.invoke.
//
//   - Job fan-out: when fan_out.fork_job_type is set, spawns one child JobRequest per
//     array item using the existing FORK_JOB machinery (JobForkTasklet pattern). Supports
//     full sub_workflow input/output variable mapping and cascade cancellation.
//
// Results are aggregated into a single TaskResponse: each child's context keys are
// prefixed with "{item_var}_{index}_".  No child jobs or definitions are created in
// task fan-out mode; all children share the parent JobExecutionID.
type FanOutTasklet struct {
	*tasklet.BaseTasklet
	serverCfg       *config.ServerConfig
	resourceManager resource.Manager
	jobManager      *manager.JobManager
}

// NewFanOutTasklet constructor
func NewFanOutTasklet(
	serverCfg *config.ServerConfig,
	requestRegistry tasklet.RequestRegistry,
	resourceManager resource.Manager,
	jobManager *manager.JobManager,
	queueClient queue.Client,
	requestTopic string,
) *FanOutTasklet {
	id := serverCfg.Common.ID + "-fan-out-tasklet"
	registration := common.AntRegistration{
		AntID:        id,
		AntTopic:     requestTopic,
		MaxCapacity:  serverCfg.Jobs.MaxFanOutTaskletCapacity,
		Tags:         []string{},
		Methods:      []common.TaskMethod{common.FanOutJob},
		Allocations:  make(map[string]*common.AntAllocation),
		AutoRefresh:  true,
		CreatedAt:    time.Now(),
		AntStartedAt: time.Now(),
	}
	t := &FanOutTasklet{
		serverCfg:       serverCfg,
		resourceManager: resourceManager,
		jobManager:      jobManager,
	}
	t.BaseTasklet = tasklet.NewBaseTasklet(
		id,
		&serverCfg.Common,
		queueClient,
		nil,
		requestRegistry,
		requestTopic,
		serverCfg.Common.GetRegistrationTopic(),
		&registration,
		t,
	)
	return t
}

// TerminateContainer terminates container
func (t *FanOutTasklet) TerminateContainer(
	_ context.Context,
	_ *common.TaskRequest) (*common.TaskResponse, error) {
	return nil, fmt.Errorf("cannot terminate container")
}

// ListContainers list containers
func (t *FanOutTasklet) ListContainers(
	_ context.Context,
	req *common.TaskRequest) (*common.TaskResponse, error) {
	taskResp := common.NewTaskResponse(req)
	taskResp.AntID = t.ID
	taskResp.Host = "server"
	taskResp.Status = common.COMPLETED
	taskResp.AddContext("containers", make([]*events.ContainerLifecycleEvent, 0))
	return taskResp, nil
}

// PreExecute checks if request can be executed
func (t *FanOutTasklet) PreExecute(
	_ context.Context,
	_ *common.TaskRequest) bool {
	return true
}

// Execute runs the fan-out logic.
func (t *FanOutTasklet) Execute(
	ctx context.Context,
	taskReq *common.TaskRequest) (*common.TaskResponse, error) {
	fanOut := taskReq.ExecutorOpts.FanOut
	if fanOut == nil {
		return taskReq.ErrorResponse(
			fmt.Errorf("fan_out configuration is missing from task %s", taskReq.TaskType)), nil
	}
	if err := fanOut.Validate(); err != nil {
		return taskReq.ErrorResponse(err), nil
	}

	items, err := resolveFanOutSource(taskReq, fanOut.Source)
	if err != nil {
		return taskReq.ErrorResponse(err), nil
	}
	if len(items) == 0 {
		logrus.WithFields(logrus.Fields{
			"Component": "FanOutTasklet",
			"TaskType":  taskReq.TaskType,
			"Source":    fanOut.Source,
		}).Warn("fan_out: source array is empty, completing immediately")
		taskResp := common.NewTaskResponse(taskReq)
		taskResp.Status = common.COMPLETED
		taskResp.AntID = t.ID
		taskResp.Host = "server"
		taskResp.AddContext("FanOutItemCount", 0)
		return taskResp, nil
	}

	started := time.Now()
	logrus.WithFields(logrus.Fields{
		"Component":   "FanOutTasklet",
		"JobType":     taskReq.JobType,
		"TaskType":    taskReq.TaskType,
		"Source":      fanOut.Source,
		"ItemCount":   len(items),
		"MaxParallel": fanOut.MaxParallel,
		"FailFast":    fanOut.FailFast,
		"Mode":        fanOutMode(fanOut),
	}).Info("fan_out: starting task expansion")

	var results []fanOutResult
	if fanOut.IsJobFanOut() {
		results, err = t.dispatchJobsAndWait(ctx, taskReq, fanOut, items)
	} else {
		results, err = t.dispatchTasksAndWait(ctx, taskReq, fanOut, items)
	}
	if err != nil {
		return taskReq.ErrorResponse(fmt.Errorf("fan_out: %w", err)), nil
	}

	taskResp := t.buildAggregatedResponse(taskReq, fanOut.ItemVar, results)
	taskResp.AddContext("FanOutItemCount", len(items))
	taskResp.AddContext("FanOutSource", fanOut.Source)
	taskResp.AddContext("FanOutMode", fanOutMode(fanOut))

	logrus.WithFields(logrus.Fields{
		"Component": "FanOutTasklet",
		"TaskType":  taskReq.TaskType,
		"ItemCount": len(items),
		"Status":    taskResp.Status,
		"Elapsed":   time.Since(started),
	}).Info("fan_out: completed")

	return taskResp, nil
}

// fanOutResult holds one child task's or child job's outcome.
type fanOutResult struct {
	index    int
	itemVal  string
	response *common.TaskResponse
	err      error
}

// dispatchTasksAndWait fans out to ant workers directly as TaskRequests (task fan-out mode).
// Runs at most max_parallel concurrently. On fail_fast, cancels context on first failure.
func (t *FanOutTasklet) dispatchTasksAndWait(
	ctx context.Context,
	taskReq *common.TaskRequest,
	fanOut *common.FanOutConfig,
	items []interface{},
) ([]fanOutResult, error) {
	n := len(items)
	results := make([]fanOutResult, n)

	concurrency := fanOut.MaxParallel
	if concurrency <= 0 || concurrency > n {
		concurrency = n
	}
	sem := make(chan struct{}, concurrency)

	execCtx, cancelExec := context.WithCancel(ctx)
	defer cancelExec()

	var wg sync.WaitGroup
	var firstFailure error
	var failMu sync.Mutex

	for i, item := range items {
		wg.Add(1)
		go func(idx int, itm interface{}) {
			defer wg.Done()

			select {
			case sem <- struct{}{}:
			case <-execCtx.Done():
				results[idx] = fanOutResult{index: idx, err: execCtx.Err()}
				return
			}
			defer func() { <-sem }()

			itemStr := fmt.Sprintf("%v", itm)
			resp, dispErr := t.dispatchSingleTask(execCtx, taskReq, fanOut, itemStr, idx)
			results[idx] = fanOutResult{index: idx, itemVal: itemStr, response: resp, err: dispErr}

			if dispErr != nil || (resp != nil && resp.Status.Failed()) {
				failMu.Lock()
				if firstFailure == nil {
					if dispErr != nil {
						firstFailure = dispErr
					} else {
						firstFailure = fmt.Errorf("item %d (%s) failed: %s", idx, itemStr, resp.ErrorMessage)
					}
					if fanOut.FailFast {
						cancelExec()
					}
				}
				failMu.Unlock()
			}
		}(i, item)
	}

	wg.Wait()
	return results, nil
}

// dispatchJobsAndWait fans out to child JobRequests (job fan-out mode).
// Spawns N child jobs using the FORK_JOB pattern: each child is a real JobRequest
// with cascade_cancel=true and the item value injected as item_var.
// Uses the same JobWaiter.RunAndWait pattern as JobForkWaitTasklet.
func (t *FanOutTasklet) dispatchJobsAndWait(
	ctx context.Context,
	taskReq *common.TaskRequest,
	fanOut *common.FanOutConfig,
	items []interface{},
) ([]fanOutResult, error) {
	n := len(items)
	results := make([]fanOutResult, n)

	concurrency := fanOut.MaxParallel
	if concurrency <= 0 || concurrency > n {
		concurrency = n
	}
	sem := make(chan struct{}, concurrency)

	execCtx, cancelExec := context.WithCancel(ctx)
	defer cancelExec()

	// Phase 1: spawn all child jobs (respecting max_parallel semaphore).
	childIDs := make([]string, n)
	var wgSpawn sync.WaitGroup
	var spawnErr error
	var spawnMu sync.Mutex

	for i, item := range items {
		wgSpawn.Add(1)
		go func(idx int, itm interface{}) {
			defer wgSpawn.Done()

			select {
			case sem <- struct{}{}:
			case <-execCtx.Done():
				return
			}
			defer func() { <-sem }()

			itemStr := fmt.Sprintf("%v", itm)
			childID, err := t.spawnChildJob(execCtx, taskReq, fanOut, itemStr, idx)
			if err != nil {
				spawnMu.Lock()
				if spawnErr == nil {
					spawnErr = fmt.Errorf("fan_out[%d] spawn failed: %w", idx, err)
					if fanOut.FailFast {
						cancelExec()
					}
				}
				spawnMu.Unlock()
				return
			}
			childIDs[idx] = childID
			results[idx] = fanOutResult{index: idx, itemVal: itemStr}
		}(i, item)
	}
	wgSpawn.Wait()

	if spawnErr != nil {
		return results, spawnErr
	}

	// Phase 2: wait for all spawned child jobs via JobWaiter (same as JobForkWaitTasklet).
	// Build a fake await request that references all child job IDs.
	awaitReq := t.buildAwaitRequest(taskReq, fanOut, childIDs)

	waiter, err := NewJobWaiter(execCtx, t.ID, t.jobManager, awaitReq)
	if err != nil {
		return results, fmt.Errorf("fan_out: failed to create job waiter: %w", err)
	}

	waitCtx := execCtx
	var cancelWait context.CancelFunc
	if taskReq.Timeout > 0 {
		waitCtx, cancelWait = context.WithTimeout(execCtx, taskReq.Timeout)
		defer cancelWait()
	}
	topic := t.Config.GetJobExecutionLifecycleTopic()
	if waitErr := waiter.RunAndWait(waitCtx,
		func(h func(*events.JobExecutionLifecycleEvent) error) error {
			return t.EventBus.Subscribe(topic, h)
		},
		func(h func(*events.JobExecutionLifecycleEvent) error) {
			_ = t.EventBus.Unsubscribe(topic, h)
		},
	); waitErr != nil {
		return results, fmt.Errorf("fan_out: waiting for child jobs: %w", waitErr)
	}

	// Translate completed child jobs into fanOutResult entries.
	aggregatedResp, buildErr := waiter.BuildTaskResponse(awaitReq)
	if buildErr != nil {
		return results, fmt.Errorf("fan_out: building aggregated response: %w", buildErr)
	}
	// Wrap the aggregated waiter response as a single result for the caller.
	results = []fanOutResult{{
		index:    0,
		response: aggregatedResp,
	}}
	return results, nil
}

// spawnChildJob creates one child JobRequest for job fan-out mode.
func (t *FanOutTasklet) spawnChildJob(
	ctx context.Context,
	parentReq *common.TaskRequest,
	fanOut *common.FanOutConfig,
	itemVal string,
	idx int,
) (string, error) {
	qc := common.NewQueryContextFromIDs(parentReq.UserID, parentReq.OrganizationID)
	jobDef, err := t.jobManager.GetJobDefinitionByType(
		qc,
		fanOut.ForkJobType,
		fanOut.ForkJobVersion,
	)
	if err != nil {
		return "", fmt.Errorf("job type %q not found: %w", fanOut.ForkJobType, err)
	}

	req, err := qtypes.NewJobRequestFromDefinition(jobDef)
	if err != nil {
		return "", fmt.Errorf("failed to build job request: %w", err)
	}
	req.JobVersion = fanOut.ForkJobVersion
	req.ParentID = parentReq.JobRequestID
	req.CascadeCancel = true
	_, _ = req.AddParam(common.ForkedJob, true)

	// Inject the item variable so child tasks can reference it.
	_, _ = req.AddParam(fanOut.ItemVar, itemVal)
	_, _ = req.AddParam(fmt.Sprintf("FanOutIndex_%d", idx), idx)

	// Apply sub_workflow input_params if configured.
	if sw := parentReq.ExecutorOpts.SubWorkflow; sw != nil && len(sw.InputParams) > 0 {
		templateData := common.VariableValuesToMap(common.MaskVariableValues(parentReq.Variables))
		// item_var takes precedence: inject it before template resolution.
		templateData[fanOut.ItemVar] = itemVal
		inputMap, mapErr := sw.InputMap()
		if mapErr != nil {
			return "", mapErr
		}
		for childParam, templateExpr := range inputMap {
			resolved, resolveErr := utils.ParseTemplate(templateExpr, templateData)
			if resolveErr != nil {
				return "", fmt.Errorf("sub_workflow.input_params: param %q: %w", childParam, resolveErr)
			}
			if _, addErr := req.AddParam(childParam, resolved); addErr != nil {
				return "", fmt.Errorf("sub_workflow.input_params: set param %q: %w", childParam, addErr)
			}
		}
	}

	saved, err := t.jobManager.SaveJobRequest(qc, req)
	if err != nil {
		return "", fmt.Errorf("failed to save child job request: %w", err)
	}

	logrus.WithFields(logrus.Fields{
		"Component":       "FanOutTasklet",
		"ParentRequestID": parentReq.JobRequestID,
		"ChildRequestID":  saved.ID,
		"ForkJobType":     fanOut.ForkJobType,
		"ItemVar":         fanOut.ItemVar,
		"Item":            itemVal,
		"Index":           idx,
	}).Info("fan_out: spawned child job")

	return saved.ID, nil
}

// buildAwaitRequest builds a synthetic TaskRequest whose Variables map contains the
// "{taskType}.ForkedJobID" keys that JobWaiter.buildJobIDs expects.
func (t *FanOutTasklet) buildAwaitRequest(
	parentReq *common.TaskRequest,
	fanOut *common.FanOutConfig,
	childIDs []string,
) *common.TaskRequest {
	awaitTaskType := fmt.Sprintf("fan-out-%s", parentReq.TaskType)
	awaitTasks := make([]string, 0, len(childIDs))
	vars := make(map[string]common.VariableValue, len(childIDs))

	for i, id := range childIDs {
		if id == "" {
			continue
		}
		key := fmt.Sprintf("%s-%d%s", awaitTaskType, i, forkedJobIDSuffix)
		vars[key] = common.NewVariableValue(id, false)
		awaitTasks = append(awaitTasks, fmt.Sprintf("%s-%d", awaitTaskType, i))
	}

	awaitReq := &common.TaskRequest{
		JobDefinitionID: parentReq.JobDefinitionID,
		JobRequestID:    parentReq.JobRequestID,
		JobExecutionID:  parentReq.JobExecutionID,
		TaskExecutionID: parentReq.TaskExecutionID,
		JobType:         parentReq.JobType,
		JobTypeVersion:  parentReq.JobTypeVersion,
		TaskType:        awaitTaskType,
		UserID:          parentReq.UserID,
		OrganizationID:  parentReq.OrganizationID,
		Timeout:         parentReq.Timeout,
		Action:          common.EXECUTE,
		ExecutorOpts:    common.NewExecutorOptions("", common.AwaitForkedJob),
		Variables:       vars,
	}
	awaitReq.ExecutorOpts.AwaitForkedTasks = awaitTasks
	awaitReq.ExecutorOpts.SubWorkflow = parentReq.ExecutorOpts.SubWorkflow
	return awaitReq
}

// dispatchSingleTask reserves an ant worker, builds a child TaskRequest for one
// fan-out item, and sends it via SendReceive, blocking until the response arrives.
// This mirrors TaskSupervisor.invoke exactly, including tracing.
func (t *FanOutTasklet) dispatchSingleTask(
	ctx context.Context,
	parentReq *common.TaskRequest,
	fanOut *common.FanOutConfig,
	itemVal string,
	idx int,
) (*common.TaskResponse, error) {
	method := fanOut.ExecutionMethod
	if method == "" || !method.IsValid() {
		return nil, fmt.Errorf("fan_out[%d]: execution_method is not set or invalid", idx)
	}

	reservation, err := t.resourceManager.Reserve(
		parentReq.JobRequestID,
		parentReq.TaskType,
		method,
		parentReq.Tags,
	)
	if err != nil {
		return nil, fmt.Errorf("fan_out[%d]: failed to reserve ant for method %s: %w", idx, method, err)
	}
	defer func() {
		_ = t.resourceManager.Release(reservation)
	}()

	// Build child request: same as parent but with item injected and method restored.
	//
	// Registry key = JobRequestID + "-" + TaskType (see TaskRequest.Key()).
	// All fan-out children share the same JobRequestID (parent's). To avoid registry
	// collisions we give each child a unique TaskType with a per-index suffix.
	// The ant executor doesn't care about the exact TaskType value; it is only used
	// as a dedup key in the registry and as a log label.
	childTaskType := fmt.Sprintf("%s-fo-%d", parentReq.TaskType, idx)

	// Deep-copy ExecutorOptions so concurrent goroutines don't race on shared pointer
	// fields (MainContainer, HelperContainer, Affinity) or the Artifacts.Paths slice
	// backing array (the ant appends to Paths during execution).
	childOpts := cloneExecutorOpts(parentReq.ExecutorOpts)
	childOpts.Method = method
	childOpts.FanOut = nil // prevent recursion

	childReq := &common.TaskRequest{
		JobDefinitionID: parentReq.JobDefinitionID,
		JobRequestID:    parentReq.JobRequestID,
		JobExecutionID:  parentReq.JobExecutionID,
		TaskExecutionID: parentReq.TaskExecutionID,
		JobType:         parentReq.JobType,
		JobTypeVersion:  parentReq.JobTypeVersion,
		TaskType:        childTaskType,
		CoRelationID:    parentReq.CoRelationID,
		UserID:          parentReq.UserID,
		OrganizationID:  parentReq.OrganizationID,
		Timeout:         parentReq.Timeout,
		Action:          parentReq.Action,
		Tags:            parentReq.Tags,
		ExecutorOpts:    &childOpts,
		Variables:       make(map[string]common.VariableValue, len(parentReq.Variables)+1),
	}
	// Copy parent variables, then inject item variable (item_var takes precedence).
	for k, v := range parentReq.Variables {
		childReq.Variables[k] = v
	}
	childReq.Variables[fanOut.ItemVar] = common.NewVariableValue(itemVal, false)

	// Render scripts using the raw (un-rendered) templates preserved in FanOutConfig.
	// Queen-side GetDynamicTaskWithQuerier runs template expansion without the item var,
	// so parentReq.Script already has "<no value>" for per-item placeholders.
	// fanOut.RawScript holds the original templates (e.g., `echo "Deploying to {{.region}}"`)
	// captured before queen-side rendering ran, allowing correct per-item substitution here.
	templateData := common.VariableValuesToMap(childReq.Variables)
	if len(fanOut.RawScript) > 0 {
		childReq.Script = renderScripts(fanOut.RawScript, templateData)
	} else {
		childReq.Script = parentReq.Script
	}
	if len(fanOut.RawBeforeScript) > 0 {
		childReq.BeforeScript = renderScripts(fanOut.RawBeforeScript, templateData)
	} else {
		childReq.BeforeScript = parentReq.BeforeScript
	}
	if len(fanOut.RawAfterScript) > 0 {
		childReq.AfterScript = renderScripts(fanOut.RawAfterScript, templateData)
	} else {
		childReq.AfterScript = parentReq.AfterScript
	}

	b, err := childReq.Marshal(reservation.EncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("fan_out[%d]: failed to marshal child request: %w", idx, err)
	}

	// Mirror TaskSupervisor: start a trace span per child dispatch.
	ctx, span := tracing.Tracer("formicary.queen").Start(ctx, "fan_out.task.dispatch",
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.String("task.type", childReq.TaskType),
			attribute.String("job.type", childReq.JobType),
			attribute.String("job.request_id", childReq.JobRequestID),
			attribute.String("fan_out.item", itemVal),
			attribute.Int("fan_out.index", idx),
			attribute.String("messaging.destination", reservation.AntTopic),
		),
	)
	defer span.End()

	props := queue.NewMessageHeaders(
		queue.DisableBatchingKey, "true",
		queue.MessageTarget, reservation.AntID,
		"RequestID", parentReq.JobRequestID,
		"TaskType", parentReq.TaskType,
		"FanOutIndex", fmt.Sprintf("%d", idx),
		"UserID", parentReq.UserID,
	)
	tracing.InjectContext(ctx, props)

	req := &queue.SendReceiveRequest{
		OutTopic: reservation.AntTopic,
		InTopic:  t.serverCfg.GetResponseTopicTaskReply(),
		Payload:  b,
		Timeout:  parentReq.Timeout,
		Props:    props,
	}

	res, err := t.QueueClient.SendReceive(ctx, req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, fmt.Errorf("fan_out[%d]: SendReceive failed: %w", idx, err)
	}
	if res.Event == nil {
		err = fmt.Errorf("fan_out[%d]: nil response event", idx)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	taskResp, err := common.UnmarshalTaskResponse(reservation.EncryptionKey, res.Event.Payload)
	if err != nil {
		return nil, fmt.Errorf("fan_out[%d]: failed to unmarshal response: %w", idx, err)
	}

	logrus.WithFields(logrus.Fields{
		"Component": "FanOutTasklet",
		"Index":     idx,
		"ItemVar":   fanOut.ItemVar,
		"Item":      itemVal,
		"Status":    taskResp.Status,
		"AntID":     reservation.AntID,
	}).Debug("fan_out: child task completed")

	return taskResp, nil
}

// buildAggregatedResponse merges N child task responses into one parent TaskResponse.
// Each child's context keys are prefixed with "{itemVar}_{index}_".
// For job fan-out, the waiter result is merged into a response seeded from taskReq so
// that CoRelationID, TaskType, and other identity fields match what TaskSupervisor expects
// when it correlates the reply via SendReceive.
func (t *FanOutTasklet) buildAggregatedResponse(
	taskReq *common.TaskRequest,
	itemVar string,
	results []fanOutResult,
) *common.TaskResponse {
	// Job fan-out: waiter built context/artifacts from child jobs; seed a new response
	// from the ORIGINAL taskReq (not awaitReq) so CoRelationID and TaskType are correct.
	if len(results) == 1 && results[0].index == 0 && results[0].itemVal == "" && results[0].response != nil {
		waiterResp := results[0].response
		resp := common.NewTaskResponse(taskReq)
		resp.Status = waiterResp.Status
		resp.ErrorMessage = waiterResp.ErrorMessage
		resp.ErrorCode = waiterResp.ErrorCode
		resp.ExitCode = waiterResp.ExitCode
		resp.ExitMessage = waiterResp.ExitMessage
		resp.AntID = t.ID
		resp.Host = "server"
		for k, v := range waiterResp.TaskContext {
			resp.AddContext(k, v)
		}
		for k, v := range waiterResp.JobContext {
			resp.JobContext[k] = v
		}
		for _, art := range waiterResp.Artifacts {
			resp.AddArtifact(art)
		}
		return resp
	}

	taskResp := common.NewTaskResponse(taskReq)
	taskResp.Status = common.COMPLETED
	taskResp.AntID = t.ID
	taskResp.Host = "server"

	for _, r := range results {
		prefix := fmt.Sprintf("%s_%d_", itemVar, r.index)

		if r.err != nil {
			if !taskResp.Status.Failed() {
				taskResp.Status = common.FAILED
				taskResp.ErrorMessage = r.err.Error()
			}
			taskResp.AddContext(fmt.Sprintf("%serror", prefix), r.err.Error())
			continue
		}
		if r.response == nil {
			continue
		}
		if r.response.Status.Failed() && !taskResp.Status.Failed() {
			taskResp.Status = r.response.Status
			taskResp.ErrorMessage = r.response.ErrorMessage
			taskResp.ErrorCode = r.response.ErrorCode
		}

		taskResp.AddContext(fmt.Sprintf("%sstatus", prefix), string(r.response.Status))
		taskResp.AddContext(fmt.Sprintf("%serror_code", prefix), r.response.ErrorCode)
		taskResp.AddContext(fmt.Sprintf("%serror_message", prefix), r.response.ErrorMessage)
		taskResp.AddContext(fmt.Sprintf("%sexit_code", prefix), r.response.ExitCode)

		for k, v := range r.response.TaskContext {
			taskResp.AddContext(prefix+k, v)
		}
		for _, art := range r.response.Artifacts {
			taskResp.AddArtifact(art)
		}
	}
	return taskResp
}

// resolveFanOutSource reads the source variable from taskReq.Variables and
// parses it as a JSON array.
func resolveFanOutSource(taskReq *common.TaskRequest, source string) ([]interface{}, error) {
	v, ok := taskReq.Variables[source]
	if !ok || v.Value == nil {
		return nil, fmt.Errorf("fan_out.source %q not found in job execution context", source)
	}
	if arr, ok2 := v.Value.([]interface{}); ok2 {
		return arr, nil
	}
	raw := fmt.Sprintf("%v", v.Value)
	var items []interface{}
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil, fmt.Errorf("fan_out.source %q is not a valid JSON array: %w", source, err)
	}
	return items, nil
}

func fanOutMode(fanOut *common.FanOutConfig) string {
	if fanOut.IsJobFanOut() {
		return "job"
	}
	return "task"
}

// cloneExecutorOpts returns a deep copy of opts safe for concurrent use by fan-out
// child goroutines. Each child appends to Artifacts.Paths on the ant side, so the
// slice must not share its backing array. Pointer fields (MainContainer,
// HelperContainer, Affinity) are copied by value to prevent concurrent mutations
// from one child affecting another's executor options.
func cloneExecutorOpts(opts *common.ExecutorOptions) common.ExecutorOptions {
	c := *opts // shallow copy of value fields

	// Clone Artifacts.Paths so ant-side appends don't race across children.
	if opts.Artifacts.Paths != nil {
		paths := make([]string, len(opts.Artifacts.Paths))
		copy(paths, opts.Artifacts.Paths)
		c.Artifacts.Paths = paths
	}

	// Deep-copy pointer fields so concurrent goroutines don't share mutable state.
	if opts.MainContainer != nil {
		mc := *opts.MainContainer
		c.MainContainer = &mc
	}
	if opts.HelperContainer != nil {
		hc := *opts.HelperContainer
		c.HelperContainer = &hc
	}
	if opts.Affinity != nil {
		af := *opts.Affinity
		c.Affinity = &af
	}

	// Clone maps — shallow copies share the same underlying map.
	if opts.NodeSelector != nil {
		ns := make(map[string]string, len(opts.NodeSelector))
		for k, v := range opts.NodeSelector {
			ns[k] = v
		}
		c.NodeSelector = ns
	}
	if opts.PodLabels != nil {
		pl := make(map[string]string, len(opts.PodLabels))
		for k, v := range opts.PodLabels {
			pl[k] = v
		}
		c.PodLabels = pl
	}
	if opts.PodAnnotations != nil {
		pa := make(map[string]string, len(opts.PodAnnotations))
		for k, v := range opts.PodAnnotations {
			pa[k] = v
		}
		c.PodAnnotations = pa
	}
	if opts.NodeTolerations != nil {
		nt := make(common.NodeTolerations, len(opts.NodeTolerations))
		for k, v := range opts.NodeTolerations {
			nt[k] = v
		}
		c.NodeTolerations = nt
	}

	return c
}

// renderScripts applies Go template rendering to each script line using templateData.
// Lines that fail to render are kept as-is; a warning is logged so operators can
// identify broken templates rather than receiving silent `<no value>` output.
func renderScripts(scripts []string, templateData map[string]interface{}) []string {
	if len(scripts) == 0 {
		return scripts
	}
	out := make([]string, len(scripts))
	for i, s := range scripts {
		if rendered, err := utils.ParseTemplate(s, templateData); err == nil {
			out[i] = rendered
		} else {
			logrus.WithFields(logrus.Fields{
				"Component": "FanOutTasklet",
				"Script":    s,
				"Error":     err,
			}).Warn("fan_out: failed to render script template, using raw string")
			out[i] = s
		}
	}
	return out
}
