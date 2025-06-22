package http

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"plexobject.com/formicary/internal/types"
	cutils "plexobject.com/formicary/internal/utils"
	"plexobject.com/formicary/internal/web"
	"reflect"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/oklog/ulid/v2"
	"plexobject.com/formicary/ants/executor"
	"plexobject.com/formicary/internal/async"
)

// CommandRunner command runner for http
type CommandRunner struct {
	executor.BaseCommandRunner
	client    web.HTTPClient
	variables map[string]types.VariableValue
	cancel    context.CancelFunc
	future    async.Awaiter
	running   bool
}

// NewCommandRunner constructor
func NewCommandRunner(
	ctx context.Context,
	client web.HTTPClient,
	e *executor.BaseExecutor,
	cmd string,
	helper bool,
	variables map[string]types.VariableValue) (*CommandRunner, error) {
	base := executor.NewBaseCommandRunner(e, cmd, helper)

	runner := CommandRunner{
		client:            client,
		BaseCommandRunner: base,
		variables:         variables,
		running:           true,
		cancel:            func() {},
	}
	runner.Host, _ = os.Hostname()
	runner.ID = ulid.Make().String()

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

	if len(stdout) > 0 && (scr.ExecutorOptions.Debug || !scr.IsHelper(ctx)) {
		_, _ = scr.Trace.Write(stdout, types.StdoutTags)
	}

	if len(stderr) > 0 && (scr.ExecutorOptions.Debug || !scr.IsHelper(ctx)) {
		_, _ = scr.Trace.Write(stderr, types.StderrTags)
	}

	if err == nil {
		logrus.WithFields(logrus.Fields{
			"Component": "HTTPCommandRunner",
			"ID":        scr.ID,
			"Name":      scr.Name,
			"Endpoint":  scr.Command,
			"StdoutLen": len(stdout),
			"HTTPCode":  scr.ExitCode,
			"Elapsed":   scr.BaseExecutor.Elapsed(),
			"Memory":    cutils.MemUsageMiBString(),
		}).Info("succeeded in executing http request")
		if scr.ExecutorOptions.Debug || !scr.IsHelper(ctx) {
			_ = scr.BaseExecutor.WriteTraceSuccess(ctx, fmt.Sprintf(
				"✅ %s Duration=%v",
				scr.Command, scr.BaseExecutor.Elapsed()))
		}
	} else if err != nil {
		scr.ExitMessage = err.Error()
		logrus.WithFields(logrus.Fields{
			"Component": "HTTPCommandRunner",
			"ID":        scr.ID,
			"Name":      scr.Name,
			"StderrLen": len(stderr),
			"Endpoint":  scr.Command,
			"HTTPCode":  scr.ExitCode,
			"Error":     err,
			"Elapsed":   scr.BaseExecutor.Elapsed(),
			"Memory":    cutils.MemUsageMiBString(),
		}).Warn("failed to execute http request")
		if scr.ExecutorOptions.Debug || !scr.IsHelper(ctx) {
			_ = scr.BaseExecutor.WriteTraceError(ctx, fmt.Sprintf(
				"❌ %s failed HTTPCode=%d Error=%s Duration=%v",
				scr.Command, scr.ExitCode, err, scr.BaseExecutor.Elapsed(),
			))
		}
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
	if scr.ExecutorOptions.Debug || !scr.IsHelper(ctx) {
		_ = scr.BaseExecutor.WriteTrace(ctx,
			fmt.Sprintf("🔄 $ %s", scr.Command))
	}
	var j []byte
	switch scr.Method {
	case types.HTTPGet:
		out, scr.ExitCode, err = scr.client.Get(
			ctx,
			scr.Command,
			scr.Headers,
			scr.QueryParams)
	case types.HTTPPostForm:
		out, scr.ExitCode, err = scr.client.PostForm(
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
		out, scr.ExitCode, err = scr.client.PostJSON(
			ctx,
			scr.Command,
			scr.Headers,
			scr.QueryParams,
			j)
	case types.HTTPPutJSON:
		j, err = variablesAsJSON(scr.variables)
		if err != nil {
			return nil, err
		}
		if scr.Headers == nil || len(scr.Headers) == 0 {
			scr.Headers = map[string]string{"Content-type": "application/json"}
		}
		out, scr.ExitCode, err = scr.client.PutJSON(
			ctx,
			scr.Command,
			scr.Headers,
			scr.QueryParams,
			j)
	case types.HTTPDelete:
		j, err = variablesAsJSON(scr.variables)
		if err != nil {
			return nil, err
		}
		out, scr.ExitCode, err = scr.client.Delete(
			ctx,
			scr.Command,
			scr.Headers,
			j)
	default:
		return nil, fmt.Errorf("unsupported http protocol %s", scr.Method)
	}
	return
}

func variablesAsJSON(variables map[string]types.VariableValue) ([]byte, error) {
	params := variablesAsFormParams(variables)
	return json.Marshal(params)
}

func variablesAsFormParams(variables map[string]types.VariableValue) map[string]string {
	params := make(map[string]string)
	for k, v := range variables {
		if reflect.TypeOf(v.Value).String() == "string" {
			params[k] = v.Value.(string)
		} else {
			params[k] = fmt.Sprintf("%v", v.Value)
		}
	}
	return params
}
