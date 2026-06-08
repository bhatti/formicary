// SPDX-License-Identifier: AGPL-3.0-or-later

package manager

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/types"
)

// Test_ShouldSaveAndLoadTaskFanOutDefinition verifies that a task fan-out job definition
// (fan_out without fork_job_type) can be saved through the full manager path and reloaded
// with all fan_out fields intact.
func Test_ShouldSaveAndLoadTaskFanOutDefinition(t *testing.T) {
	qc, err := repository.NewTestQC()
	require.NoError(t, err)

	b, err := os.ReadFile("../../fixtures/fan_out_task_job.yaml")
	require.NoError(t, err)

	job, err := types.NewJobDefinitionFromYaml(b)
	require.NoError(t, err)

	serverCfg := config.TestServerConfig()
	jobManager, _, err := newTestJobManager(serverCfg)
	require.NoError(t, err)

	saved, err := jobManager.SaveJobDefinition(qc, job)
	require.NoError(t, err)
	require.NotEmpty(t, saved.ID)

	// Reload from DB and verify fan_out survives round-trip.
	loaded, err := jobManager.GetJobDefinition(qc, saved.ID)
	require.NoError(t, err)
	require.Equal(t, "io.formicary.test.fan-out-task", loaded.JobType)

	// GetDynamicTask must re-parse from raw_yaml and populate FanOut in opts.
	processTask, processOpts, err := loaded.GetDynamicTask("process", nil)
	require.NoError(t, err)
	require.NotNil(t, processTask.FanOut, "fan_out must survive DB round-trip")
	require.Equal(t, "items", processTask.FanOut.Source)
	require.Equal(t, "item", processTask.FanOut.ItemVar)
	require.Equal(t, 2, processTask.FanOut.MaxParallel)
	require.False(t, processTask.FanOut.IsJobFanOut(), "task fan-out: no fork_job_type")
	require.NotNil(t, processOpts.FanOut, "FanOut must be in executor opts after DB load")
}

// Test_ShouldSaveAndLoadJobFanOutDefinition verifies that a job fan-out definition
// (fan_out with fork_job_type) can be saved through the full manager path and reloaded
// with all fan_out fields intact, including fork_job_type and IsJobFanOut=true.
func Test_ShouldSaveAndLoadJobFanOutDefinition(t *testing.T) {
	qc, err := repository.NewTestQC()
	require.NoError(t, err)

	b, err := os.ReadFile("../../fixtures/fan_out_job_fork_job.yaml")
	require.NoError(t, err)

	job, err := types.NewJobDefinitionFromYaml(b)
	require.NoError(t, err)

	serverCfg := config.TestServerConfig()
	jobManager, _, err := newTestJobManager(serverCfg)
	require.NoError(t, err)

	saved, err := jobManager.SaveJobDefinition(qc, job)
	require.NoError(t, err)
	require.NotEmpty(t, saved.ID)

	loaded, err := jobManager.GetJobDefinition(qc, saved.ID)
	require.NoError(t, err)
	require.Equal(t, "io.formicary.test.fan-out-fork-job", loaded.JobType)

	processTask, processOpts, err := loaded.GetDynamicTask("process", nil)
	require.NoError(t, err)
	require.NotNil(t, processTask.FanOut, "fan_out must survive DB round-trip")
	require.True(t, processTask.FanOut.IsJobFanOut(), "IsJobFanOut must be true after DB load")
	require.Equal(t, "io.formicary.test.child-job", processTask.FanOut.ForkJobType)
	require.Equal(t, "datasets", processTask.FanOut.Source)
	require.NotNil(t, processOpts.FanOut)
	require.True(t, processOpts.FanOut.IsJobFanOut())
}

// Test_ShouldSaveAndLoadFanOutWithSubWorkflow verifies that fan_out + sub_workflow
// combination survives the full manager save/load path.
func Test_ShouldSaveAndLoadFanOutWithSubWorkflow(t *testing.T) {
	qc, err := repository.NewTestQC()
	require.NoError(t, err)

	b, err := os.ReadFile("../../docs/examples/fan-out-job-etl.yaml")
	require.NoError(t, err)

	job, err := types.NewJobDefinitionFromYaml(b)
	require.NoError(t, err)

	serverCfg := config.TestServerConfig()
	jobManager, _, err := newTestJobManager(serverCfg)
	require.NoError(t, err)

	saved, err := jobManager.SaveJobDefinition(qc, job)
	require.NoError(t, err)
	require.NotEmpty(t, saved.ID)

	loaded, err := jobManager.GetJobDefinition(qc, saved.ID)
	require.NoError(t, err)

	processTask, processOpts, err := loaded.GetDynamicTask("process-datasets", nil)
	require.NoError(t, err)
	require.NotNil(t, processTask.FanOut)
	require.True(t, processTask.FanOut.IsJobFanOut())
	require.Equal(t, "io.formicary.examples.child-etl", processTask.FanOut.ForkJobType)

	// sub_workflow must also survive DB round-trip.
	require.NotNil(t, processTask.SubWorkflow)
	require.True(t, processTask.SubWorkflow.WaitForCompletion)
	om, err := processTask.SubWorkflow.OutputMap()
	require.NoError(t, err)
	require.Equal(t, "etl_row_count", om["row_count"])

	require.NotNil(t, processOpts.FanOut)
	require.NotNil(t, processOpts.SubWorkflow)
}

// Test_ShouldSaveAndLoadFanOutDeployExample verifies the original fan-out-deploy.yaml
// example (docs) survives the full manager save/load path.
func Test_ShouldSaveAndLoadFanOutDeployExample(t *testing.T) {
	qc, err := repository.NewTestQC()
	require.NoError(t, err)

	b, err := os.ReadFile("../../docs/examples/fan-out-deploy.yaml")
	require.NoError(t, err)

	job, err := types.NewJobDefinitionFromYaml(b)
	require.NoError(t, err)

	serverCfg := config.TestServerConfig()
	jobManager, _, err := newTestJobManager(serverCfg)
	require.NoError(t, err)

	saved, err := jobManager.SaveJobDefinition(qc, job)
	require.NoError(t, err)
	require.NotEmpty(t, saved.ID)

	loaded, err := jobManager.GetJobDefinition(qc, saved.ID)
	require.NoError(t, err)
	require.Equal(t, "fan-out-deploy", loaded.JobType)

	deployTask, deployOpts, err := loaded.GetDynamicTask("deploy", nil)
	require.NoError(t, err)
	require.NotNil(t, deployTask.FanOut)
	require.Equal(t, "regions", deployTask.FanOut.Source)
	require.False(t, deployTask.FanOut.IsJobFanOut())
	require.NotNil(t, deployOpts.FanOut)
}

// Test_FanOut_ManagerBuildDynamicParamsIncludesJobVariables verifies the complete
// path from DB load → GetDynamicConfigAndVariables → fan-out source resolution.
// This is the exact path that FanOutTasklet.Execute depends on via:
//   BuildTaskRequest() → buildDynamicParams() → buildDynamicConfigs() → GetDynamicConfigAndVariables()
// The bug was: ReloadFromYaml in postProcessJob would return an empty job when
// the YAML had {{.region}}-style template vars, discarding all job_variables.
func Test_FanOut_ManagerBuildDynamicParamsIncludesJobVariables(t *testing.T) {
	qc, err := repository.NewTestQC()
	require.NoError(t, err)

	serverCfg := config.TestServerConfig()
	jobManager, _, err := newTestJobManager(serverCfg)
	require.NoError(t, err)

	// YAML with template vars in scripts — this triggers the ReloadFromYaml bug path.
	jobYaml := `job_type: io.formicary.test.fanout-dynparams
job_variables:
  regions: '["us-east-1","us-west-2","eu-west-1"]'
  batch_size: "100"
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
      - echo "Deploying to {{.region}} batch={{.batch_size}}"
    on_completed: done
  - task_type: done
    method: SHELL
    script:
      - echo "done count={{.FanOutItemCount}}"
`
	job, err := types.NewJobDefinitionFromYaml([]byte(jobYaml))
	require.NoError(t, err)

	saved, err := jobManager.SaveJobDefinition(qc, job)
	require.NoError(t, err)

	loaded, err := jobManager.GetJobDefinition(qc, saved.ID)
	require.NoError(t, err)

	// Simulate what buildDynamicConfigs produces (the exact path taken by the engine).
	vars := loaded.GetDynamicConfigAndVariables(nil)

	// The fan-out source variable must be present — this is what FanOutTasklet reads.
	regionsVar, ok := vars["regions"]
	require.True(t, ok,
		"job_variables 'regions' must survive DB round-trip with template vars in scripts")
	require.Equal(t, `["us-east-1","us-west-2","eu-west-1"]`, regionsVar.Value,
		"regions value must match what was saved in job_variables")

	// Other job_variables must also survive.
	batchVar, ok := vars["batch_size"]
	require.True(t, ok, "job_variables 'batch_size' must survive DB round-trip")
	require.Equal(t, "100", batchVar.Value)

	// Fan-out task must be loadable with the vars the engine provides.
	_, opts, err := loaded.GetDynamicTask("deploy", vars)
	require.NoError(t, err)
	require.NotNil(t, opts.FanOut, "fan_out config must survive DB round-trip")
	require.Equal(t, "regions", opts.FanOut.Source)
	require.Equal(t, "region", opts.FanOut.ItemVar)
	require.Equal(t, 2, opts.FanOut.MaxParallel)
}

// Test_FanOut_ManagerFanOutJobEtlDynamicParams verifies the ETL job fan-out example
// resolves datasets from job_variables after DB round-trip.
func Test_FanOut_ManagerFanOutJobEtlDynamicParams(t *testing.T) {
	qc, err := repository.NewTestQC()
	require.NoError(t, err)

	b, err := os.ReadFile("../../docs/examples/fan-out-job-etl.yaml")
	require.NoError(t, err)

	serverCfg := config.TestServerConfig()
	jobManager, _, err := newTestJobManager(serverCfg)
	require.NoError(t, err)

	job, err := types.NewJobDefinitionFromYaml(b)
	require.NoError(t, err)

	saved, err := jobManager.SaveJobDefinition(qc, job)
	require.NoError(t, err)

	loaded, err := jobManager.GetJobDefinition(qc, saved.ID)
	require.NoError(t, err)

	vars := loaded.GetDynamicConfigAndVariables(nil)
	datasetsVar, ok := vars["datasets"]
	require.True(t, ok, "datasets must survive DB round-trip for fan-out-job-etl")
	require.NotEmpty(t, datasetsVar.Value)

	_, opts, err := loaded.GetDynamicTask("process-datasets", vars)
	require.NoError(t, err)
	require.NotNil(t, opts.FanOut)
	require.Equal(t, "datasets", opts.FanOut.Source)
	require.True(t, opts.FanOut.IsJobFanOut())
}

// Test_FanOut_ManagerFanOutTaskRegionsDynamicParams verifies the task-regions example
// resolves regions from job_variables after DB round-trip.
func Test_FanOut_ManagerFanOutTaskRegionsDynamicParams(t *testing.T) {
	qc, err := repository.NewTestQC()
	require.NoError(t, err)

	b, err := os.ReadFile("../../docs/examples/fan-out-task-regions.yaml")
	require.NoError(t, err)

	serverCfg := config.TestServerConfig()
	jobManager, _, err := newTestJobManager(serverCfg)
	require.NoError(t, err)

	job, err := types.NewJobDefinitionFromYaml(b)
	require.NoError(t, err)

	saved, err := jobManager.SaveJobDefinition(qc, job)
	require.NoError(t, err)

	loaded, err := jobManager.GetJobDefinition(qc, saved.ID)
	require.NoError(t, err)

	vars := loaded.GetDynamicConfigAndVariables(nil)
	regionsVar, ok := vars["regions"]
	require.True(t, ok, "regions must survive DB round-trip for fan-out-task-regions")
	require.NotEmpty(t, regionsVar.Value)

	_, opts, err := loaded.GetDynamicTask("deploy", vars)
	require.NoError(t, err)
	require.NotNil(t, opts.FanOut)
	require.Equal(t, "regions", opts.FanOut.Source)
}
