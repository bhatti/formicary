package http

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/web"
	"reflect"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/twinj/uuid"
	"plexobject.com/formicary/ants/executor"
	"plexobject.com/formicary/internal/async"
)

// CommandRunner command runner for http
type CommandRunner struct {
	executor.BaseCommandRunner
	client         web.HTTPClient
	variables      map[string]interface{}
	cancel         context.CancelFunc
	future         async.Awaiter
	httpStatusCode int
	running        bool
}

// NewCommandRunner constructor
func NewCommandRunner(
	ctx context.Context,
	client web.HTTPClient,
	e *executor.BaseExecutor,
	cmd string,
	helper bool,
	variables map[string]interface{}) (*CommandRunner, error) {
	base := executor.NewBaseCommandRunner(e, cmd, helper)

	runner := CommandRunner{
		client:            client,
		BaseCommandRunner: base,
		variables:         variables,
		running:           true,
		cancel:            func() {},
	}
	runner.Host, _ = os.Hostname()
	runner.ID = uuid.NewV4().String()

	handler := func(ctx context.Context, _ interface{}) (interface{}, error) {
		return runner.run(ctx)
	}
	runner.future = async.Execute(ctx, handler, async.NoAbort, nil)
	return &runner, nil
}

// Await - awaits for completion
func (scr *CommandRunner) Await(ctx context.Context) (stdout []byte, stderr []byte, err error) {
	var res interface{}
	res, err = scr.future.Await(ctx)
	if res == nil {
		res = make([]byte, 0)
	}
	stdout = res.([]byte)
	stderr = make([]byte, 0)
	scr.Err = err
	scr.ExitCode = scr.httpStatusCode

	if scr.ExecutorOptions.Debug || !scr.IsHelper(ctx) {
		_, _ = scr.Trace.Write(stdout)
	}

	if err == nil {
		logrus.WithFields(logrus.Fields{
			"Component": "HTTPCommandRunner",
			"ID":        scr.ID,
			"Name":      scr.Name,
			"Endpoint":  scr.Command,
			"HTTPCode":  scr.httpStatusCode,
			"Elapsed":   scr.BaseExecutor.Elapsed(),
		}).Info("succeeded in executing http request")
		_ = scr.BaseExecutor.WriteTraceSuccess(fmt.Sprintf(
			"✅ %s Duration=%v",
			scr.Command, scr.BaseExecutor.Elapsed()))
	} else if err != nil {
		scr.ExitMessage = err.Error()
		logrus.WithFields(logrus.Fields{
			"Component": "HTTPCommandRunner",
			"ID":        scr.ID,
			"Name":      scr.Name,
			"Endpoint":  scr.Command,
			"HTTPCode":  scr.httpStatusCode,
			"Error":     err,
			"Elapsed":   scr.BaseExecutor.Elapsed(),
		}).Warn("failed to execute http request")
		_ = scr.BaseExecutor.WriteTraceError(fmt.Sprintf(
			"❌ %s failed HTTPCode=%d Error=%s Duration=%v",
			scr.Command, scr.ExitCode, err, scr.BaseExecutor.Elapsed()))
	}
	return
}

// Stop - stops runner
func (scr *CommandRunner) Stop(context.Context, time.Duration) error {
	if scr.running && scr.cancel != nil {
		scr.cancel()
	}
	scr.running = false
	return nil
}

// IsRunning checks if runner is active
func (scr *CommandRunner) IsRunning(context.Context) (bool, error) {
	return scr.running, nil
}

/////////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////
func (scr *CommandRunner) run(ctx context.Context) (out interface{}, err error) {
	ctx, scr.cancel = context.WithCancel(ctx)
	defer func() {
		scr.running = false
	}()
	_ = scr.BaseExecutor.WriteTrace(
		fmt.Sprintf("🔄 $ %s", scr.Command))
	var j []byte
	switch scr.Method {
	case types.HTTPGet:
		out, scr.httpStatusCode, err = scr.client.Get(
			ctx,
			scr.Command,
			scr.Headers)
	case types.HTTPPostForm:
		out, scr.httpStatusCode, err = scr.client.PostForm(
			ctx,
			scr.Command,
			scr.Headers,
			variablesAsFormParams(scr.variables))
	case types.HTTPPostJSON:
		j, err = variablesAsJSON(scr.variables)
		if err != nil {
			return nil, err
		}
		if scr.Headers == nil || len(scr.Headers) == 0 {
			scr.Headers = map[string]string{"Content-type": "application/json"}
		}
		out, scr.httpStatusCode, err = scr.client.PostJSON(
			ctx,
			scr.Command,
			scr.Headers,
			j)
	case types.HTTPPutJSON:
		j, err = variablesAsJSON(scr.variables)
		if err != nil {
			return nil, err
		}
		if scr.Headers == nil || len(scr.Headers) == 0 {
			scr.Headers = map[string]string{"Content-type": "application/json"}
		}
		out, scr.httpStatusCode, err = scr.client.PutJSON(
			ctx,
			scr.Command,
			scr.Headers,
			j)
	case types.HTTPDelete:
		j, err = variablesAsJSON(scr.variables)
		if err != nil {
			return nil, err
		}
		out, scr.httpStatusCode, err = scr.client.Delete(
			ctx,
			scr.Command,
			scr.Headers,
			j)
	default:
		return nil, fmt.Errorf("unsupported http protocol %s", scr.Method)
	}
	return
}

func variablesAsJSON(variables map[string]interface{}) ([]byte, error) {
	params := variablesAsFormParams(variables)
	return json.Marshal(params)
}

func variablesAsFormParams(variables map[string]interface{}) map[string]string {
	params := make(map[string]string)
	for k, v := range variables {
		if reflect.TypeOf(v).String() == "string" {
			params[k] = v.(string)
		} else {
			params[k] = fmt.Sprintf("%v", v)
		}
	}
	return params
}
