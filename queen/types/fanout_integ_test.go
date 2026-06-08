// SPDX-License-Identifier: AGPL-3.0-or-later

package types

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"
	common "plexobject.com/formicary/internal/types"
)

// ---------------------------------------------------------------------------
// Task fan-out mode (no fork_job_type)
// ---------------------------------------------------------------------------

// Test_ShouldGetDynamicTaskForTaskFanOutExample verifies the task fan-out YAML example
// (docs/examples/fan-out-task-regions.yaml) parses correctly:
//   - setup task parses cleanly
//   - deploy task carries fan_out with correct fields, Method=FAN_OUT_JOB,
//     ExecutionMethod=SHELL (preserved from the original method)
//   - executor opts carry the fan_out config
func Test_ShouldGetDynamicTaskForTaskFanOutExample(t *testing.T) {
	b, err := ioutil.ReadFile("../../docs/examples/fan-out-task-regions.yaml")
	require.NoError(t, err)

	job, err := NewJobDefinitionFromYaml(b)
	require.NoError(t, err)
	require.Equal(t, "fan-out-task-regions", job.JobType)

	_, _, err = job.GetDynamicTask("setup", map[string]common.VariableValue{})
	require.NoError(t, err, "setup task must parse cleanly")

	deployTask, deployOpts, err := job.GetDynamicTask("deploy", map[string]common.VariableValue{})
	require.NoError(t, err, "deploy task must parse without error")
	require.NotNil(t, deployTask.FanOut, "fan_out must be present on deploy task")
	require.Equal(t, "regions", deployTask.FanOut.Source)
	require.Equal(t, "region", deployTask.FanOut.ItemVar)
	require.Equal(t, 2, deployTask.FanOut.MaxParallel)
	require.False(t, deployTask.FanOut.FailFast)
	require.False(t, deployTask.FanOut.IsJobFanOut(), "task fan-out has no fork_job_type")
	require.Equal(t, common.FanOutJob, deployTask.Method, "method rewritten to FAN_OUT_JOB")
	require.Equal(t, common.Shell, deployTask.FanOut.ExecutionMethod, "original SHELL preserved")

	require.NotNil(t, deployOpts.FanOut, "fan_out must flow into executor opts")
	require.Equal(t, "regions", deployOpts.FanOut.Source)
	require.Equal(t, common.FanOutJob, deployOpts.Method)

	_, _, err = job.GetDynamicTask("report", map[string]common.VariableValue{})
	require.NoError(t, err, "report task must parse cleanly")
}

// Test_ShouldGetDynamicTaskForTaskFanOutFixture verifies the task fan-out fixture
// (fixtures/fan_out_task_job.yaml) produces correct executor opts.
func Test_ShouldGetDynamicTaskForTaskFanOutFixture(t *testing.T) {
	b, err := ioutil.ReadFile("../../fixtures/fan_out_task_job.yaml")
	require.NoError(t, err)

	job, err := NewJobDefinitionFromYaml(b)
	require.NoError(t, err)
	require.Equal(t, "io.formicary.test.fan-out-task", job.JobType)

	processTask, processOpts, err := job.GetDynamicTask("process", map[string]common.VariableValue{})
	require.NoError(t, err)
	require.NotNil(t, processTask.FanOut)
	require.Equal(t, "items", processTask.FanOut.Source)
	require.Equal(t, "item", processTask.FanOut.ItemVar)
	require.Equal(t, 2, processTask.FanOut.MaxParallel)
	require.False(t, processTask.FanOut.FailFast)
	require.Equal(t, common.FanOutJob, processTask.Method)
	require.Equal(t, common.Shell, processTask.FanOut.ExecutionMethod)
	require.NotNil(t, processOpts.FanOut)
	require.False(t, processOpts.FanOut.IsJobFanOut())
}

// Test_ShouldRoundTripTaskFanOutViaYaml verifies fan_out survives a Yaml() → re-parse round-trip
// (simulating what happens after DB load via GetDynamicTaskWithQuerier).
func Test_ShouldRoundTripTaskFanOutViaYaml(t *testing.T) {
	b, err := ioutil.ReadFile("../../fixtures/fan_out_task_job.yaml")
	require.NoError(t, err)

	original, err := NewJobDefinitionFromYaml(b)
	require.NoError(t, err)

	reloaded, err := NewJobDefinitionFromYaml([]byte(original.Yaml()))
	require.NoError(t, err)

	var processTask *TaskDefinition
	for _, task := range reloaded.Tasks {
		if task.TaskType == "process" {
			processTask = task
			break
		}
	}
	require.NotNil(t, processTask, "process task must survive round-trip")
	require.NotNil(t, processTask.FanOut, "fan_out must survive round-trip")
	require.Equal(t, "items", processTask.FanOut.Source)
	require.Equal(t, common.FanOutJob, processTask.Method)
	require.False(t, processTask.FanOut.IsJobFanOut())
}

// ---------------------------------------------------------------------------
// Job fan-out mode (fork_job_type is set)
// ---------------------------------------------------------------------------

// Test_ShouldGetDynamicTaskForJobFanOutExample verifies the job fan-out YAML example
// (docs/examples/fan-out-job-etl.yaml) parses correctly:
//   - process-datasets task carries fan_out with fork_job_type set
//   - IsJobFanOut() returns true
//   - sub_workflow output variables survive into the task definition
func Test_ShouldGetDynamicTaskForJobFanOutExample(t *testing.T) {
	b, err := ioutil.ReadFile("../../docs/examples/fan-out-job-etl.yaml")
	require.NoError(t, err)

	job, err := NewJobDefinitionFromYaml(b)
	require.NoError(t, err)
	require.Equal(t, "fan-out-job-etl", job.JobType)

	_, _, err = job.GetDynamicTask("setup", map[string]common.VariableValue{})
	require.NoError(t, err, "setup task must parse cleanly")

	processTask, processOpts, err := job.GetDynamicTask("process-datasets", map[string]common.VariableValue{})
	require.NoError(t, err, "process-datasets task must parse cleanly")
	require.NotNil(t, processTask.FanOut)
	require.True(t, processTask.FanOut.IsJobFanOut(), "job fan-out: IsJobFanOut must be true")
	require.Equal(t, "io.formicary.examples.child-etl", processTask.FanOut.ForkJobType)
	require.Equal(t, "1.0", processTask.FanOut.ForkJobVersion)
	require.Equal(t, "datasets", processTask.FanOut.Source)
	require.Equal(t, "dataset", processTask.FanOut.ItemVar)
	require.Equal(t, 2, processTask.FanOut.MaxParallel)
	require.False(t, processTask.FanOut.FailFast)
	require.Equal(t, common.FanOutJob, processTask.Method)

	// sub_workflow must survive into the task definition
	require.NotNil(t, processTask.SubWorkflow)
	require.True(t, processTask.SubWorkflow.WaitForCompletion)
	om, err := processTask.SubWorkflow.OutputMap()
	require.NoError(t, err)
	require.Equal(t, "etl_row_count", om["row_count"])

	// Both fan_out and sub_workflow must flow into executor opts
	require.NotNil(t, processOpts.FanOut)
	require.True(t, processOpts.FanOut.IsJobFanOut())
	require.NotNil(t, processOpts.SubWorkflow)

	_, _, err = job.GetDynamicTask("summarize", map[string]common.VariableValue{})
	require.NoError(t, err, "summarize task must parse cleanly")
}

// Test_ShouldGetDynamicTaskForJobFanOutFixture verifies the job fan-out fixture
// (fixtures/fan_out_job_fork_job.yaml) parses and produces correct executor opts.
func Test_ShouldGetDynamicTaskForJobFanOutFixture(t *testing.T) {
	b, err := ioutil.ReadFile("../../fixtures/fan_out_job_fork_job.yaml")
	require.NoError(t, err)

	job, err := NewJobDefinitionFromYaml(b)
	require.NoError(t, err)
	require.Equal(t, "io.formicary.test.fan-out-fork-job", job.JobType)

	processTask, processOpts, err := job.GetDynamicTask("process", map[string]common.VariableValue{})
	require.NoError(t, err)
	require.NotNil(t, processTask.FanOut)
	require.True(t, processTask.FanOut.IsJobFanOut(), "fixture must be job fan-out mode")
	require.Equal(t, "io.formicary.test.child-job", processTask.FanOut.ForkJobType)
	require.Equal(t, "datasets", processTask.FanOut.Source)
	require.Equal(t, "dataset", processTask.FanOut.ItemVar)
	require.Equal(t, 2, processTask.FanOut.MaxParallel)
	require.False(t, processTask.FanOut.FailFast)
	require.Equal(t, common.FanOutJob, processTask.Method)
	require.NotNil(t, processOpts.FanOut)
	require.True(t, processOpts.FanOut.IsJobFanOut())
}

// Test_ShouldRoundTripJobFanOutViaYaml verifies job fan-out survives Yaml() → re-parse round-trip.
func Test_ShouldRoundTripJobFanOutViaYaml(t *testing.T) {
	b, err := ioutil.ReadFile("../../fixtures/fan_out_job_fork_job.yaml")
	require.NoError(t, err)

	original, err := NewJobDefinitionFromYaml(b)
	require.NoError(t, err)

	reloaded, err := NewJobDefinitionFromYaml([]byte(original.Yaml()))
	require.NoError(t, err)

	var processTask *TaskDefinition
	for _, task := range reloaded.Tasks {
		if task.TaskType == "process" {
			processTask = task
			break
		}
	}
	require.NotNil(t, processTask, "process task must survive round-trip")
	require.NotNil(t, processTask.FanOut, "fan_out must survive round-trip")
	require.True(t, processTask.FanOut.IsJobFanOut(), "IsJobFanOut must survive round-trip")
	require.Equal(t, "io.formicary.test.child-job", processTask.FanOut.ForkJobType)
	require.Equal(t, common.FanOutJob, processTask.Method)
}

// ---------------------------------------------------------------------------
// Existing fan-out-deploy example (kept for backward compat)
// ---------------------------------------------------------------------------

// Test_ShouldGetDynamicTaskForFanOutExample verifies the original fan-out-deploy.yaml example.
func Test_ShouldGetDynamicTaskForFanOutExample(t *testing.T) {
	b, err := ioutil.ReadFile("../../docs/examples/fan-out-deploy.yaml")
	require.NoError(t, err)

	job, err := NewJobDefinitionFromYaml(b)
	require.NoError(t, err)
	require.Equal(t, "fan-out-deploy", job.JobType)

	_, _, err = job.GetDynamicTask("setup", map[string]common.VariableValue{})
	require.NoError(t, err, "setup task must parse without error")

	deployTask, deployOpts, err := job.GetDynamicTask("deploy", map[string]common.VariableValue{})
	require.NoError(t, err, "deploy task must parse without error")
	require.NotNil(t, deployTask.FanOut, "fan_out must be present on deploy task")
	require.Equal(t, "regions", deployTask.FanOut.Source)
	require.Equal(t, "region", deployTask.FanOut.ItemVar)
	require.Equal(t, 2, deployTask.FanOut.MaxParallel)
	require.False(t, deployTask.FanOut.FailFast)
	require.Equal(t, common.FanOutJob, deployTask.Method, "task method must be rewritten to FAN_OUT_JOB")
	require.Equal(t, common.Shell, deployTask.FanOut.ExecutionMethod, "original method captured in ExecutionMethod")

	require.NotNil(t, deployOpts.FanOut, "fan_out must be in executor opts")
	require.Equal(t, "regions", deployOpts.FanOut.Source)
	require.Equal(t, common.FanOutJob, deployOpts.Method)

	_, _, err = job.GetDynamicTask("report", map[string]common.VariableValue{})
	require.NoError(t, err, "report task must parse without error")
}

// Test_ShouldGetDynamicTaskForFanOutFixture verifies the original fan_out_job.yaml fixture.
func Test_ShouldGetDynamicTaskForFanOutFixture(t *testing.T) {
	b, err := ioutil.ReadFile("../../fixtures/fan_out_job.yaml")
	require.NoError(t, err)

	job, err := NewJobDefinitionFromYaml(b)
	require.NoError(t, err)
	require.Equal(t, "io.formicary.test.fan-out-deploy", job.JobType)

	deployTask, deployOpts, err := job.GetDynamicTask("deploy", map[string]common.VariableValue{})
	require.NoError(t, err)
	require.NotNil(t, deployTask.FanOut)
	require.Equal(t, "regions", deployTask.FanOut.Source)
	require.Equal(t, "region", deployTask.FanOut.ItemVar)
	require.Equal(t, 2, deployTask.FanOut.MaxParallel)
	require.False(t, deployTask.FanOut.FailFast)
	require.Equal(t, common.FanOutJob, deployTask.Method)
	require.Equal(t, common.Shell, deployTask.FanOut.ExecutionMethod)
	require.NotNil(t, deployOpts.FanOut)
	require.Equal(t, common.FanOutJob, deployOpts.Method)
}

// ---------------------------------------------------------------------------
// Field-level parsing
// ---------------------------------------------------------------------------

// Test_ShouldParseJobFanOutMode verifies that fork_job_type in fan_out
// correctly activates job fan-out mode and IsJobFanOut returns true.
func Test_ShouldParseJobFanOutMode(t *testing.T) {
	yml := `
job_type: test-job-fan-out
tasks:
  - task_type: process
    method: FORK_JOB
    fan_out:
      source: datasets
      item_var: dataset
      fork_job_type: io.formicary.etl-child
      fork_job_version: "1.0"
      max_parallel: 3
      fail_fast: true
`
	job, err := NewJobDefinitionFromYaml([]byte(yml))
	require.NoError(t, err)

	task, opts, err := job.GetDynamicTask("process", map[string]common.VariableValue{})
	require.NoError(t, err)
	require.NotNil(t, task.FanOut)
	require.True(t, task.FanOut.IsJobFanOut(), "IsJobFanOut must return true when fork_job_type is set")
	require.Equal(t, "io.formicary.etl-child", task.FanOut.ForkJobType)
	require.Equal(t, "1.0", task.FanOut.ForkJobVersion)
	require.Equal(t, 3, task.FanOut.MaxParallel)
	require.True(t, task.FanOut.FailFast)
	require.Equal(t, common.FanOutJob, task.Method)
	require.NotNil(t, opts.FanOut)
	require.True(t, opts.FanOut.IsJobFanOut())
}

// Test_ShouldParseFanOutWithSubWorkflowOutputVariables verifies fan_out and
// sub_workflow can be combined: job fan-out with output variable mapping.
func Test_ShouldParseFanOutWithSubWorkflowOutputVariables(t *testing.T) {
	yml := `
job_type: test-fan-out-with-output
tasks:
  - task_type: run-all
    method: FORK_JOB
    fork_job_type: io.formicary.child
    fan_out:
      source: items
      item_var: item
      fork_job_type: io.formicary.child
    sub_workflow:
      output_variables:
        - name: child_result
          value: run_all_result
      wait_for_completion: true
`
	job, err := NewJobDefinitionFromYaml([]byte(yml))
	require.NoError(t, err)

	task, opts, err := job.GetDynamicTask("run-all", map[string]common.VariableValue{})
	require.NoError(t, err)
	require.NotNil(t, task.FanOut)
	require.True(t, task.FanOut.IsJobFanOut())
	require.NotNil(t, task.SubWorkflow)
	require.True(t, task.SubWorkflow.WaitForCompletion)

	om, err := task.SubWorkflow.OutputMap()
	require.NoError(t, err)
	require.Equal(t, "run_all_result", om["child_result"])

	require.NotNil(t, opts.FanOut)
	require.NotNil(t, opts.SubWorkflow)
}

// Test_ShouldRoundTripFanOutWithJobModeViaJSON verifies job fan-out survives
// Yaml() → re-parse, simulating the DB load path (GetDynamicTaskWithQuerier uses
// raw_yaml on every execution).
func Test_ShouldRoundTripFanOutWithJobModeViaJSON(t *testing.T) {
	b, err := ioutil.ReadFile("../../fixtures/fan_out_job.yaml")
	require.NoError(t, err)

	original, err := NewJobDefinitionFromYaml(b)
	require.NoError(t, err)

	reloaded, err := NewJobDefinitionFromYaml([]byte(original.Yaml()))
	require.NoError(t, err)

	var deployTask *TaskDefinition
	for _, task := range reloaded.Tasks {
		if task.TaskType == "deploy" {
			deployTask = task
			break
		}
	}
	require.NotNil(t, deployTask, "deploy task must survive round-trip")
	require.NotNil(t, deployTask.FanOut, "fan_out must survive round-trip")
	require.Equal(t, "regions", deployTask.FanOut.Source)
	require.Equal(t, common.FanOutJob, deployTask.Method)
}

// ---------------------------------------------------------------------------
// Validation
// ---------------------------------------------------------------------------

// Test_ShouldRejectFanOutMissingSource checks that missing source is caught.
func Test_ShouldRejectFanOutMissingSource(t *testing.T) {
	yml := `
job_type: test-fan-out-invalid
tasks:
  - task_type: t1
    method: SHELL
    fan_out:
      item_var: region
    script:
      - echo hello
`
	_, err := NewJobDefinitionFromYaml([]byte(yml))
	require.Error(t, err)
	require.Contains(t, err.Error(), "source")
}

// Test_ShouldRejectFanOutMissingItemVar checks that missing item_var is caught.
func Test_ShouldRejectFanOutMissingItemVar(t *testing.T) {
	yml := `
job_type: test-fan-out-invalid
tasks:
  - task_type: t1
    method: SHELL
    fan_out:
      source: regions
    script:
      - echo hello
`
	_, err := NewJobDefinitionFromYaml([]byte(yml))
	require.Error(t, err)
	require.Contains(t, err.Error(), "item_var")
}

// Test_ShouldRejectFanOutNegativeMaxParallel checks that negative max_parallel is caught.
func Test_ShouldRejectFanOutNegativeMaxParallel(t *testing.T) {
	yml := `
job_type: test-fan-out-invalid
tasks:
  - task_type: t1
    method: SHELL
    fan_out:
      source: regions
      item_var: region
      max_parallel: -1
    script:
      - echo hello
`
	_, err := NewJobDefinitionFromYaml([]byte(yml))
	require.Error(t, err)
}

// Test_ShouldOverrideMethodToFanOutJobWhenFanOutSet verifies that setting fan_out
// on a SHELL task causes its method to be rewritten to FAN_OUT_JOB.
func Test_ShouldOverrideMethodToFanOutJobWhenFanOutSet(t *testing.T) {
	yml := `
job_type: test-fan-out-method
tasks:
  - task_type: t1
    method: SHELL
    fan_out:
      source: items
      item_var: item
    script:
      - echo hello
`
	job, err := NewJobDefinitionFromYaml([]byte(yml))
	require.NoError(t, err)
	task, opts, err := job.GetDynamicTask("t1", map[string]common.VariableValue{})
	require.NoError(t, err)
	require.Equal(t, common.FanOutJob, task.Method)
	require.Equal(t, common.Shell, task.FanOut.ExecutionMethod)
	require.Equal(t, common.FanOutJob, opts.Method)
}

// Test_ShouldValidateBeforeSaveTaskFanOut verifies that fan-out job definitions
// pass ValidateBeforeSave (no panic, no error — the save path is clean).
func Test_ShouldValidateBeforeSaveTaskFanOut(t *testing.T) {
	b, err := ioutil.ReadFile("../../fixtures/fan_out_task_job.yaml")
	require.NoError(t, err)

	job, err := NewJobDefinitionFromYaml(b)
	require.NoError(t, err)
	require.NoError(t, job.ValidateBeforeSave(nil))
}

// Test_ShouldValidateBeforeSaveJobFanOut verifies that job fan-out definitions
// pass ValidateBeforeSave.
func Test_ShouldValidateBeforeSaveJobFanOut(t *testing.T) {
	b, err := ioutil.ReadFile("../../fixtures/fan_out_job_fork_job.yaml")
	require.NoError(t, err)

	job, err := NewJobDefinitionFromYaml(b)
	require.NoError(t, err)
	require.NoError(t, job.ValidateBeforeSave(nil))
}

// Test_ShouldValidateBeforeSaveFanOutDeployExample verifies the docs example passes save validation.
func Test_ShouldValidateBeforeSaveFanOutDeployExample(t *testing.T) {
	b, err := ioutil.ReadFile("../../docs/examples/fan-out-deploy.yaml")
	require.NoError(t, err)

	job, err := NewJobDefinitionFromYaml(b)
	require.NoError(t, err)
	require.NoError(t, job.ValidateBeforeSave(nil))
}
