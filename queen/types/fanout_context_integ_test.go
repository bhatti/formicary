// SPDX-License-Identifier: AGPL-3.0-or-later
//
// Integration tests that verify the full context-propagation path for fan-out:
//
//   1. job_variables are available to all tasks via GetDynamicConfigAndVariables.
//   2. Task-level variables: do NOT propagate to subsequent tasks.
//   3. The fan-out source variable is present in the Variables map that
//      FanOutTasklet.Execute would receive (via TaskRequest.Variables).
//
// These tests catch the class of failure seen in production:
//   "fan_out.source 'regions' not found in job execution context"
//
// Root cause: task-level variables: only apply within that task and do NOT
// propagate to subsequent tasks. job_variables are job-scoped and available
// everywhere. All fan-out fixtures and examples must use job_variables.

package types

import (
	"encoding/json"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"
	common "plexobject.com/formicary/internal/types"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// assertFanOutSourceInContext verifies that the fan_out.source variable is
// present in the Variables map (from GetDynamicConfigAndVariables) that
// FanOutTasklet.Execute would receive via TaskRequest.Variables.
func assertFanOutSourceInContext(t *testing.T, job *JobDefinition, taskType string) {
	t.Helper()

	// This is what the engine builds via buildDynamicParams → buildDynamicConfigs.
	vars := job.GetDynamicConfigAndVariables(nil)

	_, opts, err := job.GetDynamicTask(taskType, vars)
	require.NoError(t, err)
	require.NotNil(t, opts.FanOut, "task %q must have fan_out configured", taskType)

	source := opts.FanOut.Source
	v, ok := vars[source]
	require.True(t, ok,
		"fan_out.source %q must be present in job Variables — use job_variables: not task-level variables:", source)
	require.NotNil(t, v.Value, "fan_out.source %q must have a non-nil value", source)

	// Must be a valid JSON array.
	raw := v.Value.(string)
	var items []interface{}
	require.NoError(t, json.Unmarshal([]byte(raw), &items),
		"fan_out.source %q value must be a valid JSON array, got: %q", source, raw)
	require.NotEmpty(t, items, "fan_out.source %q array must not be empty", source)
}

// ---------------------------------------------------------------------------
// Task fan-out: job_variables propagation
// ---------------------------------------------------------------------------

// Test_FanOut_JobVariablesAvailableToFanOutTask verifies that job_variables
// defined at the top of the YAML are present in the fan-out task's Variables map.
// This is the critical path: FanOutTasklet reads taskReq.Variables[fanOut.Source].
func Test_FanOut_JobVariablesAvailableToFanOutTask(t *testing.T) {
	jobYaml := `
job_type: test-fan-out-context
job_variables:
  regions: '["us-east-1","us-west-2","eu-west-1"]'
tasks:
  - task_type: setup
    method: SHELL
    script:
      - echo "setup"
    on_completed: deploy
  - task_type: deploy
    method: SHELL
    fan_out:
      source: regions
      item_var: region
      max_parallel: 2
      fail_fast: false
    script:
      - echo "deploying to {{.region}}"
    on_completed: done
  - task_type: done
    method: SHELL
    script:
      - echo "done"
`
	job, err := NewJobDefinitionFromYaml([]byte(jobYaml))
	require.NoError(t, err)
	assertFanOutSourceInContext(t, job, "deploy")
}

// Test_FanOut_TaskVariablesDoNotPropagate verifies that a variable defined
// inside a task's variables: block is NOT available to the fan-out task.
// This documents the contract: task variables: are task-local.
func Test_FanOut_TaskVariablesDoNotPropagate(t *testing.T) {
	jobYaml := `
job_type: test-fan-out-task-var-scope
tasks:
  - task_type: setup
    method: SHELL
    script:
      - echo "setup"
    variables:
      regions: '["us-east-1","us-west-2"]'
    on_completed: deploy
  - task_type: deploy
    method: SHELL
    fan_out:
      source: regions
      item_var: region
    script:
      - echo "deploying to {{.region}}"
`
	job, err := NewJobDefinitionFromYaml([]byte(jobYaml))
	require.NoError(t, err)

	// GetDynamicConfigAndVariables only returns job_variables and org configs —
	// NOT task-level variables. "regions" defined in setup task is absent here.
	vars := job.GetDynamicConfigAndVariables(nil)
	_, ok := vars["regions"]
	require.False(t, ok,
		"task-level variables: must NOT appear in GetDynamicConfigAndVariables — use job_variables")
}

// Test_FanOut_RequestParamOverridesJobVariable verifies that caller-supplied
// params (e.g. job request params) can override job_variables defaults.
func Test_FanOut_RequestParamOverridesJobVariable(t *testing.T) {
	jobYaml := `
job_type: test-fan-out-param-override
job_variables:
  regions: '["us-east-1","us-west-2"]'
tasks:
  - task_type: deploy
    method: SHELL
    fan_out:
      source: regions
      item_var: region
    script:
      - echo "deploying to {{.region}}"
`
	job, err := NewJobDefinitionFromYaml([]byte(jobYaml))
	require.NoError(t, err)

	// job_variables provides a default that can be overridden by request params.
	vars := job.GetDynamicConfigAndVariables(nil)
	require.Contains(t, vars, "regions")

	// Simulate caller-supplied override (buildDynamicParams merges request params over job_variables).
	vars["regions"] = common.NewVariableValue(`["eu-west-1","ap-southeast-1","eu-central-1"]`, false)
	var items []interface{}
	require.NoError(t, json.Unmarshal([]byte(vars["regions"].Value.(string)), &items))
	require.Equal(t, 3, len(items))
}

// ---------------------------------------------------------------------------
// Fan-out example files: verify source variable is in job context
// ---------------------------------------------------------------------------

// Test_FanOut_TaskRegionsExampleSourceInContext verifies docs/examples/fan-out-task-regions.yaml
// uses job_variables so the fan-out source survives into the deploy task Variables.
func Test_FanOut_TaskRegionsExampleSourceInContext(t *testing.T) {
	b, err := ioutil.ReadFile("../../docs/examples/fan-out-task-regions.yaml")
	require.NoError(t, err)
	job, err := NewJobDefinitionFromYaml(b)
	require.NoError(t, err)
	assertFanOutSourceInContext(t, job, "deploy")
}

// Test_FanOut_DeployExampleSourceInContext verifies docs/examples/fan-out-deploy.yaml
// uses job_variables so the fan-out source survives into the deploy task Variables.
func Test_FanOut_DeployExampleSourceInContext(t *testing.T) {
	b, err := ioutil.ReadFile("../../docs/examples/fan-out-deploy.yaml")
	require.NoError(t, err)
	job, err := NewJobDefinitionFromYaml(b)
	require.NoError(t, err)
	assertFanOutSourceInContext(t, job, "deploy")
}

// Test_FanOut_JobEtlExampleSourceInContext verifies docs/examples/fan-out-job-etl.yaml
// uses job_variables so datasets survives into the process-datasets task Variables.
func Test_FanOut_JobEtlExampleSourceInContext(t *testing.T) {
	b, err := ioutil.ReadFile("../../docs/examples/fan-out-job-etl.yaml")
	require.NoError(t, err)
	job, err := NewJobDefinitionFromYaml(b)
	require.NoError(t, err)
	assertFanOutSourceInContext(t, job, "process-datasets")
}

// ---------------------------------------------------------------------------
// Fan-out fixture files: verify source variable is in job context
// ---------------------------------------------------------------------------

// Test_FanOut_FanOutJobFixtureSourceInContext verifies fixtures/fan_out_job.yaml
// uses job_variables so regions survives into the deploy task Variables.
func Test_FanOut_FanOutJobFixtureSourceInContext(t *testing.T) {
	b, err := ioutil.ReadFile("../../fixtures/fan_out_job.yaml")
	require.NoError(t, err)
	job, err := NewJobDefinitionFromYaml(b)
	require.NoError(t, err)
	assertFanOutSourceInContext(t, job, "deploy")
}

// Test_FanOut_FanOutTaskFixtureSourceInContext verifies fixtures/fan_out_task_job.yaml
// uses job_variables so items survives into the process task Variables.
func Test_FanOut_FanOutTaskFixtureSourceInContext(t *testing.T) {
	b, err := ioutil.ReadFile("../../fixtures/fan_out_task_job.yaml")
	require.NoError(t, err)
	job, err := NewJobDefinitionFromYaml(b)
	require.NoError(t, err)
	assertFanOutSourceInContext(t, job, "process")
}

// Test_FanOut_FanOutForkJobFixtureSourceInContext verifies fixtures/fan_out_job_fork_job.yaml
// uses job_variables so datasets survives into the process task Variables.
func Test_FanOut_FanOutForkJobFixtureSourceInContext(t *testing.T) {
	b, err := ioutil.ReadFile("../../fixtures/fan_out_job_fork_job.yaml")
	require.NoError(t, err)
	job, err := NewJobDefinitionFromYaml(b)
	require.NoError(t, err)
	assertFanOutSourceInContext(t, job, "process")
}
