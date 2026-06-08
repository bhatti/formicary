package types

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"
	common "plexobject.com/formicary/internal/types"
)

func Test_ShouldGetDynamicTaskForSubWorkflowExample(t *testing.T) {
	b, err := ioutil.ReadFile("../../docs/examples/sub-workflow-etl.yaml")
	require.NoError(t, err)

	job, err := NewJobDefinitionFromYaml(b)
	require.NoError(t, err)
	require.Equal(t, "io.formicary.examples.parent-etl", job.JobType)

	vars := map[string]common.VariableValue{
		"source_table": common.NewVariableValue("orders", false),
		"batch_size":   common.NewVariableValue("500", false),
	}

	// validate-inputs task must parse cleanly
	task, _, err := job.GetDynamicTask("validate-inputs", vars)
	require.NoError(t, err, "validate-inputs task must parse without error")
	require.NotEmpty(t, task.Script)

	// run-child-etl task must carry sub_workflow
	forkTask, _, err := job.GetDynamicTask("run-child-etl", vars)
	require.NoError(t, err, "run-child-etl task must parse without error")
	require.NotNil(t, forkTask.SubWorkflow)
	om, err := forkTask.SubWorkflow.OutputMap()
	require.NoError(t, err)
	require.Equal(t, "etl_row_count", om["row_count"])
	require.True(t, forkTask.SubWorkflow.WaitForCompletion)

	// report-results task must parse cleanly
	_, _, err = job.GetDynamicTask("report-results", vars)
	require.NoError(t, err, "report-results task must parse without error")
}

func Test_ShouldGetDynamicTaskForSubWorkflowChildExample(t *testing.T) {
	b, err := ioutil.ReadFile("../../docs/examples/sub-workflow-etl-child.yaml")
	require.NoError(t, err)

	job, err := NewJobDefinitionFromYaml(b)
	require.NoError(t, err)
	require.Equal(t, "io.formicary.examples.child-etl", job.JobType)
	require.Equal(t, "1.0", job.SemVersion)
}

func Test_ShouldGetDynamicTaskForSubWorkflowFixture(t *testing.T) {
	b, err := ioutil.ReadFile("../../fixtures/sub_workflow_job.yaml")
	require.NoError(t, err)

	job, err := NewJobDefinitionFromYaml(b)
	require.NoError(t, err)

	vars := map[string]common.VariableValue{
		"source_table": common.NewVariableValue("orders", false),
		"batch_size":   common.NewVariableValue("500", false),
	}

	task, _, err := job.GetDynamicTask("run-child-etl", vars)
	require.NoError(t, err)
	require.NotNil(t, task.SubWorkflow)
	om, err := task.SubWorkflow.OutputMap()
	require.NoError(t, err)
	require.Equal(t, "etl_row_count", om["row_count"])
	require.True(t, task.SubWorkflow.WaitForCompletion)
}
