package tasklet

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/events"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/types"
	"strconv"
	"sync"
	"time"
)

// JobWaiter waits for the job
type JobWaiter struct {
	sync.Mutex
	jobManager   *manager.JobManager
	requestIDs   []uint64
	requests     map[uint64]*types.JobRequest
	queryContext *common.QueryContext
	done         chan struct{}
}

// NewJobWaiter constructor
func NewJobWaiter(
	jobManager *manager.JobManager,
	taskReq *common.TaskRequest) (*JobWaiter, error) {
	queryContext := common.NewQueryContextFromIDs(taskReq.UserID, taskReq.OrganizationID)

	requestIDs, err := buildJobIDs(taskReq)
	if err != nil {
		return nil, err
	}
	waiter := &JobWaiter{
		queryContext: queryContext,
		jobManager:   jobManager,
		requestIDs:   requestIDs,
		requests:     make(map[uint64]*types.JobRequest),
		done:         make(chan struct{}),
	}
	return waiter, nil
}

// Await sleeps for a timeout or if context is cancelled
func (jw *JobWaiter) Await(ctx context.Context, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-jw.done:
		return nil
	}
}

// BuildTaskResponse builds task response
func (jw *JobWaiter) BuildTaskResponse(
	taskReq *common.TaskRequest) (taskResp *common.TaskResponse, err error) {
	if !jw.completed() {
		return nil, fmt.Errorf("could not wait for all jobs, received only %d requests from %d pending jobs",
			len(jw.requests), len(jw.requestIDs))
	}

	taskResp = common.NewTaskResponse(taskReq)
	taskResp.Status = common.COMPLETED
	for _, req := range jw.requests {
		jobExecution, err := jw.jobManager.GetJobExecution(req.JobExecutionID)
		if err != nil {
			return nil, fmt.Errorf("failed to find job-execution for '%d', state '%s', type '%s', execution-id '%s' due to %v",
				req.ID, req.JobState, req.JobType, req.JobExecutionID, err)
		}

		if !taskResp.Status.Failed() {
			taskResp.Status = jobExecution.JobState
		}

		taskResp.ErrorMessage = jobExecution.ErrorMessage
		taskResp.ErrorCode = jobExecution.ErrorCode
		taskResp.ExitCode = jobExecution.ExitCode
		taskResp.ExitMessage = jobExecution.ExitMessage

		taskResp.AddContext(fmt.Sprintf("Request_%d_ErrorMessage", req.ID), jobExecution.ErrorMessage)
		taskResp.AddContext(fmt.Sprintf("Request_%d_ErrorCode", req.ID), jobExecution.ErrorCode)
		taskResp.AddContext(fmt.Sprintf("Request_%d_ExitCode", req.ID), jobExecution.ExitCode)
		taskResp.AddContext(fmt.Sprintf("Request_%d_ExitMessage", req.ID), jobExecution.ExitMessage)
		taskResp.AddContext(fmt.Sprintf("Request_%d_JobExecutionID", req.ID), jobExecution.ID)

		for _, c := range jobExecution.Contexts {
			v, err := c.GetParsedValue()
			if err == nil {
				taskResp.AddContext(c.Name, v)
			}
		}

		for _, task := range jobExecution.Tasks {
			for _, art := range task.Artifacts {
				taskResp.AddArtifact(art)
			}
		}
	}
	return taskResp, nil
}

// Poll checks if pending requests are completed
func (jw *JobWaiter) Poll() (bool, error) {
	jw.Lock()
	defer jw.Unlock()
	for _, id := range jw.requestIDs {
		if jw.requests[id] == nil {
			req, err := jw.jobManager.GetJobRequest(jw.queryContext, id)
			if err != nil {
				return false, err
			}
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
			}).Debug("polling to check completed tasks")
	}

	return jw.completed(), nil
}

// UpdateFromJobLifecycleEvent checks if received job that it's waiting
func (jw *JobWaiter) UpdateFromJobLifecycleEvent(
	jobExecutionLifecycleEvent *events.JobExecutionLifecycleEvent) error {
	if jw.matchesJobIDs(jobExecutionLifecycleEvent) {
		jw.Lock()
		defer jw.Unlock()
		jw.done <- struct{}{}
	}
	return nil
}

/////////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////
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

func buildJobIDs(taskReq *common.TaskRequest) (jobIDs []uint64, err error) {
	waitingTaskTypes := taskReq.ExecutorOpts.AwaitForkedTasks
	jobIDs = make([]uint64, len(waitingTaskTypes))
	for i, taskType := range waitingTaskTypes {
		reqKey := taskType + forkedJobIDSuffix
		v := taskReq.Variables[reqKey]
		if v.Value == nil {
			return nil, fmt.Errorf("failed to find job-id for %s", reqKey)
		}
		switch v.Value.(type) {
		case uint64:
			jobIDs[i] = v.Value.(uint64)
		default:
			jobIDs[i], err = strconv.ParseUint(fmt.Sprintf("%v", v.Value), 0, 64)
			if err != nil {
				return nil, fmt.Errorf("failed to parse job-id %v k due to %v", v, err)
			}
		}
	}
	return
}

func (jw *JobWaiter) completed() bool {
	return len(jw.requests) == len(jw.requestIDs)
}
