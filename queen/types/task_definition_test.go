package types

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"testing"

	common "plexobject.com/formicary/internal/types"

	"gopkg.in/yaml.v3"
	"plexobject.com/formicary/queen/utils"
)

// Verify table names for task-definition and config
func Test_ShouldTaskDefinitionTableNames(t *testing.T) {
	task := NewTaskDefinition("task", common.Shell)
	require.Equal(t, "formicary_task_definitions", task.TableName())
	variable, _ := task.AddVariable("k1", "v1")
	require.Equal(t, "formicary_task_definition_variables", variable.TableName())
}

// Validate task-definition with proper initialization
func Test_ShouldTaskDefinitionHappyPath(t *testing.T) {
	task := NewTaskDefinition("task", common.Shell)
	task.OnExitCode["completed"] = "task2"

	// WHEN validating valid task-definition
	err := task.ValidateBeforeSave()

	// THEN it should not fail
	require.NoError(t, err)
}

// Setting script
func Test_ShouldSetScriptForTaskDefinition(t *testing.T) {
	task := NewTaskDefinition("task", common.Shell)
	task.OnExitCode["completed"] = "task2"

	task.Script = []string{"c1", "c2", "c3"}
	require.Equal(t, "c1,c2,c3,", task.ScriptString())
}

// Setting variables
func Test_ShouldSetVariablesForTaskDefinition(t *testing.T) {
	task := NewTaskDefinition("task", common.Shell)
	task.AddVariable("job_version", "v1")
	task.AddVariable("artifact_ids", "v2")
	task.AddVariable("except", "true")
	task.AddVariable("v4", "1")
	require.Equal(t, "job_version=v1 artifact_ids=v2 except=true v4=1 ", task.VariablesString())
	require.Nil(t, task.GetVariable("k3"))
	require.Equal(t, "v2", task.GetVariable("artifact_ids").Value)
	require.Equal(t, 1, len(task.FilteredVariables()))
	task.AfterLoad()
}

// Setting except
func Test_ShouldSetExceptForTaskDefinition(t *testing.T) {
	task := NewTaskDefinition("task", common.Shell)
	require.False(t, task.IsExcept())
}

// Setting always-run
func Test_ShouldSetAlwaysRunForTaskDefinition(t *testing.T) {
	task := NewTaskDefinition("task", common.Shell)
	task.SetAlwaysRun()
	require.False(t, task.IsExcept())
}

// Test validate without task-type
func Test_ShouldFailValidateTaskDefinitionWithoutTaskType(t *testing.T) {
	task1 := NewTaskDefinition("", common.Shell)

	// WHEN validating without task-type
	err := task1.ValidateBeforeSave()

	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "taskType")
}

// Test validate without task-type
func Test_ShouldValidateTaskDefinitionWithoutTaskMethod(t *testing.T) {
	task1 := NewTaskDefinition("type", "")
	// WHEN validating without method
	err := task1.ValidateBeforeSave()

	// THEN it should fail
	require.NoError(t, err)
	require.Equal(t, common.Kubernetes, task1.Method)
}

// Verify serialization of empty exit code
func Test_ShouldLoadSaveEmptyOnExitCode(t *testing.T) {
	task := NewTaskDefinition("task", common.Shell)
	// WHEN saving empty exit code
	m, err := task.LoadOnExitCode()
	require.NoError(t, err)
	require.Equal(t, 0, len(m))
	serialized, err := task.SaveOnExitCode()

	// it should return empty serialized exit codes
	require.NoError(t, err)
	require.Equal(t, "", serialized)
}

// Verify serialization of valid exit code
func Test_ShouldLoadSaveOnExitCodeWithValidData(t *testing.T) {
	task := NewTaskDefinition("task", common.Shell)
	// Adding some exit codes
	task.OnExitCode["completed"] = "task2"
	task.OnExitCode["failed"] = "task3"
	task.OnExitCode["blah"] = "task4"

	require.Equal(t, 3, len(task.OnExitCode))

	serialized, err := task.SaveOnExitCode()
	// it should return non-empty serialized exit codes
	require.NoError(t, err)
	require.NotEqual(t, "", serialized)

	codes, err := task.LoadOnExitCode()
	require.NoError(t, err)
	require.Equal(t, 3, len(codes))
}

// Test parse yaml tag for task-type
func Test_ShouldParseYamlTaskTag(t *testing.T) {
	b, err := ioutil.ReadFile("../../fixtures/basic-job.yaml")
	require.NoError(t, err)
	taskNames := []string{"task1", "task2", "task3"}
	for _, name := range taskNames {
		// WHEN parsing dynamic task
		ser := utils.ParseYamlTag(string(b), fmt.Sprintf("task_type: %s", name))
		task := NewTaskDefinition("", "")
		err = yaml.Unmarshal([]byte(ser), task)
		require.NoError(t, err)
		require.Equal(t, name, task.TaskType)
		if name == "task1" {
			require.Equal(t, "NetworkBandwidth", task.Resources.ResourceType)
			require.Equal(t, "connection", task.Resources.ExtractConfig.ContextPrefix)
		}
	}
}
