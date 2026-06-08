// SPDX-License-Identifier: AGPL-3.0-or-later

package tasklet

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/events"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/types"
)

// JobWaiter waits for one or more forked child jobs to reach a terminal state.
type JobWaiter struct {
	sync.Mutex
	ctx          context.Context
	antID        string
	jobManager   *manager.JobManager
	requestIDs   []string
	requests     map[string]*types.JobRequest
	queryContext *common.QueryContext
	subWorkflow  *common.SubWorkflowConfig
	// notify is a buffered-1 channel. UpdateFromJobLifecycleEvent does a non-blocking
	// send so the signal is never lost even if RunAndWait is not yet sleeping.
	notify       chan struct{}
	logTimestamp time.Time
	logInterval  time.Duration
}

// NewJobWaiter constructor
func NewJobWaiter(
	ctx context.Context,
	antID string,
	jobManager *manager.JobManager,
	taskReq *common.TaskRequest) (*JobWaiter, error) {
	queryContext := common.NewQueryContextFromIDs(taskReq.UserID, taskReq.OrganizationID)

	requestIDs, err := buildJobIDs(taskReq)
	if err != nil {
		return nil, err
	}
	waiter := &JobWaiter{
		ctx:          ctx,
		antID:        antID,
		queryContext: queryContext,
		jobManager:   jobManager,
		requestIDs:   requestIDs,
		requests:     make(map[string]*types.JobRequest),
		subWorkflow:  taskReq.ExecutorOpts.SubWorkflow,
		notify:       make(chan struct{}, 1),
		logTimestamp: time.Unix(0, 0),
	}
	waiter.logInterval = time.Second * 15
	if val, err := time.ParseDuration(taskReq.Variables["log_interval"].String()); err == nil {
		waiter.logInterval = val
	}
	if len(requestIDs) == 0 {
		logrus.WithFields(
			logrus.Fields{
				"Component":  "JobWaiter",
				"RequestIDs": requestIDs,
				"Request":    taskReq,
			}).Warnf("no request ids found")
	}
	return waiter, nil
}

// RequestIDs returns the list of child request IDs being awaited.
func (jw *JobWaiter) RequestIDs() []string {
	return jw.requestIDs
}

// BuildTaskResponse assembles a task response from all completed child job executions.
// When sub_workflow.output_variables is set, only mapped variables are promoted to the
// parent context with their renamed keys. Without output_variables all child context
// variables are copied verbatim.
func (jw *JobWaiter) BuildTaskResponse(
	taskReq *common.TaskRequest) (taskResp *common.TaskResponse, err error) {
	if !jw.completed() {
		return nil, fmt.Errorf("could not wait for all jobs, received only %d requests from %d pending jobs",
			len(jw.requests), len(jw.requestIDs))
	}

	taskResp = common.NewTaskResponse(taskReq)
	taskResp.Status = common.COMPLETED
	taskResp.AntID = jw.antID
	taskResp.Host = "server"
	for _, req := range jw.requests {
		jobExecution, err := jw.jobManager.GetJobExecution(req.JobExecutionID)
		if err != nil {
			return nil, fmt.Errorf("failed to find job-execution for '%s', state '%s', type '%s', execution-id '%s' due to %v",
				req.ID, req.JobState, req.JobType, req.JobExecutionID, err)
		}

		if !taskResp.Status.Failed() {
			taskResp.Status = jobExecution.JobState
		}

		// Only capture error details from the first failing child so the reported
		// error is deterministic (map iteration order is non-deterministic in Go).
		if jobExecution.JobState.Failed() && taskResp.ErrorMessage == "" {
			taskResp.ErrorMessage = jobExecution.ErrorMessage
			taskResp.ErrorCode = jobExecution.ErrorCode
			taskResp.ExitCode = jobExecution.ExitCode
			taskResp.ExitMessage = jobExecution.ExitMessage
		}

		taskResp.AddContext(fmt.Sprintf("Request_%s_ErrorMessage", req.ID), jobExecution.ErrorMessage)
		taskResp.AddContext(fmt.Sprintf("Request_%s_ErrorCode", req.ID), jobExecution.ErrorCode)
		taskResp.AddContext(fmt.Sprintf("Request_%s_ExitCode", req.ID), jobExecution.ExitCode)
		taskResp.AddContext(fmt.Sprintf("Request_%s_ExitMessage", req.ID), jobExecution.ExitMessage)
		taskResp.AddContext(fmt.Sprintf("Request_%s_JobExecutionID", req.ID), jobExecution.ID)

		jw.applyOutputMap(taskResp, jobExecution.Contexts)

		for _, task := range jobExecution.Tasks {
			for _, art := range task.Artifacts {
				taskResp.AddArtifact(art)
			}
		}
	}
	return taskResp, nil
}

// Poll checks whether all pending child requests have reached a terminal state.
func (jw *JobWaiter) Poll() (completed bool, err error) {
	if jw.ctx.Err() != nil {
		return false, jw.ctx.Err()
	}
	jw.Lock()
	defer jw.Unlock()
	statuses := make(map[string]common.RequestState)
	for _, id := range jw.requestIDs {
		if jw.requests[id] == nil {
			req, err := jw.jobManager.GetJobRequest(jw.queryContext, id)
			if err != nil {
				return false, err
			}
			statuses[req.ID] = req.JobState
			if req.JobState.IsTerminal() {
				jw.requests[id] = req
			}
		}
	}

	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(
			logrus.Fields{
				"Component":         "JobWaiter",
				"RequestIDs":        jw.requestIDs,
				"CompletedRequests": len(jw.requests),
				"Statuses":          statuses,
			}).Debug("polling to check completed tasks")
	}

	completed = jw.completed()
	now := time.Now()
	if !completed && now.Unix()-jw.logTimestamp.Unix() > int64(jw.logInterval.Seconds()) {
		jw.logTimestamp = now
		logrus.WithFields(
			logrus.Fields{
				"Component":         "JobWaiter",
				"RequestIDs":        jw.requestIDs,
				"CompletedRequests": len(jw.requests),
				"Statuses":          statuses,
			}).Infof("waiting for completed tasks")
	}
	return
}

// UpdateFromJobLifecycleEvent signals the waiter when a tracked child job completes.
// The send is non-blocking: if the channel already holds a pending signal, it is a
// no-op — RunAndWait will still wake and re-poll on the next iteration.
func (jw *JobWaiter) UpdateFromJobLifecycleEvent(
	jobExecutionLifecycleEvent *events.JobExecutionLifecycleEvent) error {
	if jw.matchesJobIDs(jobExecutionLifecycleEvent) {
		select {
		case jw.notify <- struct{}{}:
		default:
		}
	}
	return nil
}

// RunAndWait subscribes to job lifecycle events, polls until all child jobs complete,
// unsubscribes, and returns any poll error. Returns ctx.Err() if the context is
// cancelled before all children complete.
func (jw *JobWaiter) RunAndWait(
	ctx context.Context,
	subscribe func(handler func(*events.JobExecutionLifecycleEvent) error) error,
	unsubscribe func(handler func(*events.JobExecutionLifecycleEvent) error),
) error {
	if err := subscribe(jw.UpdateFromJobLifecycleEvent); err != nil {
		return fmt.Errorf("failed to subscribe to lifecycle events: %w", err)
	}
	defer unsubscribe(jw.UpdateFromJobLifecycleEvent)

	sleep := 1 * time.Second
	for {
		done, err := jw.Poll()
		if err != nil {
			return err
		}
		if done {
			return nil
		}

		// Drain any stale notification before sleeping so we don't
		// skip a signal that arrived between Poll and the select below.
		select {
		case <-jw.notify:
		default:
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-jw.notify:
			// lifecycle event arrived — re-poll immediately
		case <-time.After(sleep):
		}

		if sleep*2 <= 10*time.Second {
			sleep *= 2
		}
	}
}

// ///////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////

// applyOutputMap promotes child execution-context variables to the task response.
// When sub_workflow.output_variables is set only mapped variables are included (with
// their renamed parent-context keys). Missing or unparseable keys are logged as warnings.
// Without output_variables all variables are copied verbatim.
func (jw *JobWaiter) applyOutputMap(
	taskResp *common.TaskResponse,
	contexts []*types.JobExecutionContext,
) {
	if jw.subWorkflow != nil && len(jw.subWorkflow.OutputVariables) > 0 {
		outputMap, err := jw.subWorkflow.OutputMap()
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"Component": "JobWaiter",
				"Error":     err,
			}).Warn("sub_workflow: invalid output_variables configuration, copying all context vars")
			// Fall through to copy-all path on configuration error.
			for _, c := range contexts {
				if v, parseErr := c.GetParsedValue(); parseErr == nil {
					taskResp.AddContext(c.Name, v)
				} else {
					logrus.WithFields(logrus.Fields{
						"Component": "JobWaiter",
						"ChildKey":  c.Name,
						"Error":     parseErr,
					}).Warn("sub_workflow: failed to parse child context variable")
				}
			}
			return
		}
		// Build a lookup of child context names that were successfully mapped.
		found := make(map[string]struct{}, len(contexts))
		for _, c := range contexts {
			if parentName, ok := outputMap[c.Name]; ok {
				if v, parseErr := c.GetParsedValue(); parseErr == nil {
					taskResp.AddContext(parentName, v)
					found[c.Name] = struct{}{}
				} else {
					logrus.WithFields(logrus.Fields{
						"Component": "JobWaiter",
						"ChildKey":  c.Name,
						"ParentKey": parentName,
						"Error":     parseErr,
					}).Warn("sub_workflow: failed to parse child context variable for output_variables")
				}
			}
		}
		// Warn about expected output_variables keys absent from the child context.
		for childName, parentName := range outputMap {
			if _, ok := found[childName]; !ok {
				logrus.WithFields(logrus.Fields{
					"Component": "JobWaiter",
					"ChildKey":  childName,
					"ParentKey": parentName,
				}).Warn("sub_workflow: output_variables key not found in child execution context")
			}
		}
		logrus.WithFields(logrus.Fields{
			"Component":           "JobWaiter",
			"OutputVariableCount": len(jw.subWorkflow.OutputVariables),
			"MappedCount":         len(found),
		}).Debug("sub_workflow: applied output_variables to task response")
	} else {
		for _, c := range contexts {
			if v, parseErr := c.GetParsedValue(); parseErr == nil {
				taskResp.AddContext(c.Name, v)
			} else {
				logrus.WithFields(logrus.Fields{
					"Component": "JobWaiter",
					"ChildKey":  c.Name,
					"Error":     parseErr,
				}).Warn("sub_workflow: failed to parse child context variable")
			}
		}
	}
}

func (jw *JobWaiter) matchesJobIDs(jobExecutionLifecycleEvent *events.JobExecutionLifecycleEvent) bool {
	if !jobExecutionLifecycleEvent.JobState.IsTerminal() {
		return false
	}
	for _, n := range jw.requestIDs {
		if jobExecutionLifecycleEvent.JobRequestID == n {
			return true
		}
	}
	return false
}

func buildJobIDs(taskReq *common.TaskRequest) (jobIDs []string, err error) {
	waitingTaskTypes := taskReq.ExecutorOpts.AwaitForkedTasks
	if len(waitingTaskTypes) == 0 {
		return nil, fmt.Errorf("no task types defined for await_forked_tasks")
	}
	jobIDs = make([]string, len(waitingTaskTypes))
	for i, taskType := range waitingTaskTypes {
		reqKey := taskType + forkedJobIDSuffix
		v := taskReq.Variables[reqKey]
		if v.Value == nil {
			return nil, fmt.Errorf("failed to find job-id for %s", reqKey)
		}
		switch val := v.Value.(type) {
		case string:
			if val == "" {
				return nil, fmt.Errorf("empty job-id for %s", reqKey)
			}
			jobIDs[i] = val
		default:
			s := fmt.Sprintf("%v", v.Value)
			if s == "" || s == "<nil>" {
				return nil, fmt.Errorf("invalid job-id for %s: %v", reqKey, v.Value)
			}
			jobIDs[i] = s
		}
	}
	return
}

func (jw *JobWaiter) completed() bool {
	return len(jw.requests) == len(jw.requestIDs)
}
