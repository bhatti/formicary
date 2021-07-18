package supervisor

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"plexobject.com/formicary/queen/config"

	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/queue"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/fsm"
)

// TaskSupervisor for executing task
type TaskSupervisor struct {
	serverCfg        *config.ServerConfig
	taskStateMachine *fsm.TaskExecutionStateMachine
	cancel           context.CancelFunc
}

// NewTaskSupervisor creates supervisor for task execution
func NewTaskSupervisor(
	serverCfg *config.ServerConfig,
	stateMachine *fsm.TaskExecutionStateMachine) *TaskSupervisor {
	return &TaskSupervisor{
		serverCfg:        serverCfg,
		taskStateMachine: stateMachine,
		cancel:           func() {},
	}
}

// Execute - creates periodic ticker for scheduling pending jobs
func (ts *TaskSupervisor) Execute(
	ctx context.Context) error {
	return ts.execute(ctx)
}

/////////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////
// executing job
func (ts *TaskSupervisor) execute(
	ctx context.Context) (err error) {
	started := time.Now()
	if ts.taskStateMachine.TaskDefinition.Timeout > 0 {
		// timeout will be handled by ant but here we are adding additional check with a bit more buffer
		ctx, ts.cancel = context.WithTimeout(
			ctx,
			ts.taskStateMachine.TaskDefinition.Timeout+1*time.Minute)
	} else if ts.serverCfg.MaxTaskTimeout > 0 {
		ctx, ts.cancel = context.WithTimeout(
			ctx,
			ts.serverCfg.MaxTaskTimeout)
	} else {
		ctx, ts.cancel = context.WithCancel(ctx)
	}

	defer ts.cancel()

	// If this is continuation from last execution and task was completed successfully then use it
	if ts.taskStateMachine.TaskExecution.TaskState.Completed() {
		logrus.WithFields(ts.taskStateMachine.LogFields("TaskSupervisor")).
			Infof("task %s already completed so won't run it again",
				ts.taskStateMachine.TaskDefinition.TaskType)

		return nil
	}

	// we will save task state in the end
	defer func() {
		if !ts.taskStateMachine.TaskExecution.TaskState.IsTerminal() {
			if err == nil && ctx.Err() != nil {
				err = fmt.Errorf("%v (timeout=%s)", ctx.Err(), time.Now().Sub(started).String())
			} else if err == nil {
				err = fmt.Errorf("unknown error executing task")
			}
			ts.taskStateMachine.SetFailed(err)
		}
		// save final state
		saveErr := ts.taskStateMachine.FinalizeTaskState(ctx)
		if ts.taskStateMachine.TaskExecution.Failed() {
			logrus.WithFields(ts.taskStateMachine.LogFields("TaskSupervisor", err, saveErr)).
				Warnf("failed to run task '%s'!", ts.taskStateMachine.TaskDefinition.TaskType)
		} else {
			logrus.WithFields(ts.taskStateMachine.LogFields("TaskSupervisor", err, saveErr)).
				Infof("completed task successfully '%s'!", ts.taskStateMachine.TaskDefinition.TaskType)
		}
	}()

	// PrepareExecution validates ant reservation and initialize previous task execution if needed
	if err = ts.taskStateMachine.PrepareExecution(ctx); err != nil {
		// task is updated with FAILED
		// changing job state from EXECUTING to FAILED
		return fmt.Errorf("failed to prepare task for execution due to %v", err)
	}

	logrus.WithFields(ts.taskStateMachine.LogFields("TaskSupervisor")).
		Infof("starting task %s...", ts.taskStateMachine.TaskDefinition.TaskType)

	// mark task as executing
	if err = ts.taskStateMachine.SetTaskToExecuting(ctx); err == nil {
		err = ts.tryExecuteTask(ctx)
	}

	return
}

// This method checks if task was completed in last job run then reuse it
// Otherwise it submits the request to remote ant
func (ts *TaskSupervisor) tryExecuteTask(
	ctx context.Context) (err error) {

	if ts.taskStateMachine.TaskDefinition.IsExcept() {
		_, _ = ts.taskStateMachine.TaskExecution.AddContext(
			"Except", ts.taskStateMachine.TaskDefinition.Except)
		ts.taskStateMachine.TaskExecution.TaskState = common.COMPLETED
		ts.taskStateMachine.TaskExecution.ExitCode = "SKIPPED"
		ts.taskStateMachine.TaskExecution.ExitMessage = "Skipped task due to except flag"
		return nil
	}

	// Build task request
	var taskReq *common.TaskRequest
	if taskReq, err = ts.taskStateMachine.BuildTaskRequest(); err != nil {
		return fmt.Errorf("failed to build request for '%s' due to %v",
			ts.taskStateMachine.TaskDefinition.TaskType, err)
	}

	// Reuse previous task state if completed successfully
	if ts.taskStateMachine.CanReusePreviousResult() {
		if taskResp, err := ts.taskStateMachine.BuildTaskResponseFromPreviousResult(); err == nil {
			_, _ = ts.taskStateMachine.TaskExecution.AddContext(
				"ReusedPreviousResultFromTaskExecution",
				ts.taskStateMachine.LastTaskExecution.ID)
			return ts.taskStateMachine.UpdateTaskFromResponse(taskReq, taskResp)
		}
	}

	executing := false

	// Try running the task with retry loop - by default it will run once if no retry is set
	for ; ts.taskStateMachine.CanRetry() || executing; ts.taskStateMachine.TaskExecution.Retried++ {
		taskReq.Retry = ts.taskStateMachine.TaskExecution.Retried
		// send request and wait synchronously for response
		taskResp, err := ts.invoke(ctx, taskReq)
		if err == nil {
			err = ts.taskStateMachine.UpdateTaskFromResponse(taskReq, taskResp)
			executing = taskResp.Status == common.EXECUTING
			// error will be nil if status is COMPLETED
			if (err == nil && !executing) ||
				ts.taskStateMachine.TaskExecution.Retried == ts.taskStateMachine.TaskDefinition.Retry ||
				taskResp.Status == common.FATAL {
				break
			}
			sleepDuration := ts.taskStateMachine.TaskDefinition.GetDelayBetweenRetries()
			logrus.WithFields(
				ts.taskStateMachine.LogFields(
					"TaskSupervisor",
				)).Warnf("retrying task=%s status=%s retried=%d wait=%s ...",
				ts.taskStateMachine.TaskDefinition.TaskType,
				taskResp.Status,
				ts.taskStateMachine.TaskExecution.Retried,
				sleepDuration)
			time.Sleep(sleepDuration)
		} else {
			break
		}
	}
	return err
}

// invoking task request
func (ts *TaskSupervisor) invoke(
	ctx context.Context,
	taskReq *common.TaskRequest) (taskResp *common.TaskResponse, err error) {
	var b []byte
	if b, err = common.MarshalTaskRequest(ts.taskStateMachine.Reservation.EncryptionKey, taskReq); err != nil {
		return nil, fmt.Errorf("failed to marshal %s due to %v", taskReq, err)
	}
	var event *queue.MessageEvent
	if event, err = ts.taskStateMachine.QueueClient.SendReceive(
		ctx,
		ts.taskStateMachine.Reservation.AntTopic,
		make(map[string]string),
		b,
		taskReq.ResponseTopic); err != nil {
		return nil, err
	}
	taskResp = common.NewTaskResponse(taskReq)
	err = json.Unmarshal(event.Payload, taskResp)

	if ts.taskStateMachine.TaskDefinition.IsFatalError(taskResp.ExitCode) {
		if taskResp.ErrorCode != "" {
			taskResp.AddContext("ErrorCode", taskResp.ErrorCode)
		}
		taskResp.ErrorCode = common.ErrorFatal
		taskResp.Status = common.FATAL
		logrus.WithFields(logrus.Fields{
			"Component": "TaskSupervisor",
			"Task":      ts.taskStateMachine.TaskDefinition,
			"TaskResp":  taskResp,
		}).Warnf("marking response with fatal error")
	}
	event.Ack() // auto-ack
	return
}
