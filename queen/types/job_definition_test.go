package types

import (
	"encoding/json"
	"io/ioutil"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v3"
	"plexobject.com/formicary/internal/crypto"

	common "plexobject.com/formicary/internal/types"
)

// Verify table names for job-definition and config
func Test_ShouldJobDefinitionTableNames(t *testing.T) {
	job := NewJobDefinition("io.formicary.test-job")
	require.Equal(t, "formicary_job_definitions", job.TableName())
	variable, _ := job.AddVariable("jk1", "jv1")
	require.Equal(t, "formicary_job_definition_variables", variable.TableName())
}

var testEncryptedKey = crypto.SHA256Key("test-key")

// Validate happy path of Validate with proper job-definition
func Test_ShouldValidateGoodJobDefinition(t *testing.T) {
	// GIVEN - job definition is created
	// AND a job is populated with required fields
	job := newTestJobDefinition("test-job")
	// WHEN validating
	err := job.ValidateBeforeSave(testEncryptedKey)
	// THEN it should not fail
	require.NoError(t, err)

	// AND first task should match
	firstTask, err := job.GetFirstTask()
	require.NoError(t, err)
	require.Equal(t, "task1", firstTask.TaskType)

	require.NotNil(t, job.GetLastAlwaysRunTasks())
	require.NotNil(t, job.GetLastTask())
	require.NotNil(t, job.GetLastAlwaysRunTasks())
	// AND dynamic task should match
	task, _, err := job.GetDynamicTask("task1", nil)
	require.NoError(t, err)
	require.Equal(t, "task1", task.TaskType)
	config := job.GetDynamicConfigAndVariables(nil)
	require.Equal(t, "jv1", config["jk1"].Value)
	require.Equal(t, "", job.CronAndScheduleTime())
}

// Validate job with single task
func Test_ShouldBeAbleToCreateJobDefinitionWithSingleTask(t *testing.T) {
	// GIVEN - job definition is created
	// AND a job is populated with a single task
	job := NewJobDefinition("test-job")
	task1 := NewTaskDefinition("task1", common.Shell)
	job.Tasks = append(job.Tasks, task1)
	job.UpdateRawYaml()
	// WHEN validating job
	err := job.ValidateBeforeSave(testEncryptedKey)

	// THEN it should not fail
	require.NoError(t, err)

	// WHEN finding first task
	firstTask, err := job.GetFirstTask()
	// THEN it should not fail and match
	require.NoError(t, err)
	require.Equal(t, "task1", firstTask.TaskType)
}

// Validate job with leaf nodes (without exit-codes)
func Test_ShouldJobDefinitionValidateWithMultipleLeafTasks(t *testing.T) {
	// GIVEN - job definition is created
	job := NewJobDefinition("io.formicary.test-job")
	task1 := NewTaskDefinition("task1", common.Shell)
	task2 := NewTaskDefinition("task2", common.Shell)
	job.AddTask(task1)
	job.AddTask(task2)
	job.UpdateRawYaml()
	// WHEN validating with multiple leaf tasks
	err := job.ValidateBeforeSave(testEncryptedKey)

	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "multiple leaf tasks")
}

// Validate job with looping tasks (pointing to each other)
func Test_ShouldJobDefinitionValidateWithLoopingTasks(t *testing.T) {
	// GIVEN - job definition is created
	job := NewJobDefinition("io.formicary.test-job")
	task1 := NewTaskDefinition("task1", common.Shell)
	task1.OnExitCode["completed"] = "task2"
	task2 := NewTaskDefinition("task2", common.Shell)
	task2.OnExitCode["completed"] = "task1"
	job.AddTask(task1)
	job.AddTask(task2)
	job.UpdateRawYaml()
	// WHEN validating with looping tasks
	err := job.ValidateBeforeSave(testEncryptedKey)

	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "could not find starting task")
}

// Validate job with empty on-exit
func Test_ShouldJobDefinitionValidateWithEmptyOnExit(t *testing.T) {
	// GIVEN - job definition is created
	job := NewJobDefinition("io.formicary.test-job")
	task1 := NewTaskDefinition("task1", common.Shell)
	task1.OnExitCode["completed"] = ""
	task1.OnExitCode["failed"] = ""
	task2 := NewTaskDefinition("task2", common.Shell)
	job.AddTask(task1)
	job.AddTask(task2)
	job.UpdateRawYaml()
	// WHEN validating with empty on-exit
	err := job.ValidateBeforeSave(testEncryptedKey)

	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty task")
}

// Validate job with non-existing task on-exit
func Test_ShouldJobDefinitionValidateWithNonExistingTaskOnExit(t *testing.T) {
	// GIVEN - job definition is created
	job := NewJobDefinition("test-job")
	task1 := NewTaskDefinition("task1", common.Shell)
	task1.OnExitCode["completed"] = "task2"
	task1.OnExitCode["failed"] = "x2"
	task2 := NewTaskDefinition("task2", common.Shell)
	job.AddTask(task1)
	job.AddTask(task2)
	job.UpdateRawYaml()
	// WHEN validating with non-existing on-exit task
	err := job.ValidateBeforeSave(testEncryptedKey)
	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "not defined")
}

// Validate job with non-reachable task on-exit
func Test_ShouldJobDefinitionValidateWithNonReachableTaskOnExit(t *testing.T) {
	// GIVEN - job definition is created
	job := NewJobDefinition("test-job")
	task1 := NewTaskDefinition("task1", common.Shell)
	task1.OnExitCode["completed"] = "task2"
	task2 := NewTaskDefinition("task2", common.Shell)
	task2.OnExitCode["completed"] = "task1"
	task3 := NewTaskDefinition("task3", common.Shell)
	job.AddTask(task1)
	job.AddTask(task2)
	job.AddTask(task3)
	job.UpdateRawYaml()
	// WHEN validating with non-reachable on-exit task
	err := job.ValidateBeforeSave(testEncryptedKey)
	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "not reachable")
}

// Validate job with duplicate tasks
func Test_ShouldJobDefinitionValidateWithDuplicateTasks(t *testing.T) {
	// GIVEN - job definition is created
	job := NewJobDefinition("test-job")
	task1 := NewTaskDefinition("task1", common.Shell)
	task1.OnExitCode["completed"] = "task2"
	job.AddTask(task1)
	job.Tasks = append(job.Tasks, task1)
	job.UpdateRawYaml()
	// WHEN validating with duplicate tasks
	err := job.ValidateBeforeSave(testEncryptedKey)
	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate tasks")
}

// Validate should fail if job type is empty
func Test_ShouldJobDefinitionValidateWithoutJobType(t *testing.T) {
	// GIVEN - job definition is created
	job := NewJobDefinition("")
	// WHEN validating without job-type
	err := job.ValidateBeforeSave(testEncryptedKey)
	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "jobType is not specified")
}

// Validate should fail if built without tasks
func Test_ShouldJobDefinitionValidateWithoutTasks(t *testing.T) {
	// GIVEN - job definition is created
	job := NewJobDefinition("type")
	// WHEN validating without tasks
	err := job.ValidateBeforeSave(testEncryptedKey)
	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "tasks are not specified")
}

// Validate should fail with invalid task-type
func Test_ShouldJobDefinitionValidateWithInvalidTaskType(t *testing.T) {
	// GIVEN - job definition is created
	job := NewJobDefinition("name")
	task1 := NewTaskDefinition("", "method")
	job.Tasks = append(job.Tasks, task1)
	job.UpdateRawYaml()

	// WHEN validating with task without type
	err := job.ValidateBeforeSave(testEncryptedKey)
	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "taskType is not specified")
}

// Validate should succeed with missing task-method
func Test_ShouldJobDefinitionValidateWithMissingTaskMethod(t *testing.T) {
	// GIVEN - job definition is created
	job := NewJobDefinition("name")
	task1 := NewTaskDefinition("type", "")
	job.AddTask(task1)
	job.RawYaml = "blah"
	// WHEN validating with task without method
	err := job.ValidateBeforeSave(testEncryptedKey)
	// THEN it should not fail
	require.NoError(t, err)
}

// Validate should succeed if built without methods
func Test_ShouldJobDefinitionValidateWithoutMethods(t *testing.T) {
	// GIVEN - job definition is created
	job := NewJobDefinition("name")
	task1 := NewTaskDefinition("type", "")
	job.AddTask(task1)
	job.RawYaml = "blah"
	// WHEN validating with task without methods
	err := job.ValidateBeforeSave(testEncryptedKey)
	// THEN it should not fail
	require.NoError(t, err)
}

// Test evaluation of shouldSkip property
func Test_ShouldEvaluateSkipIf(t *testing.T) {
	// GIVEN - job definition is created
	job := newTestJobDefinition("name")
	data := map[string]common.VariableValue{
		"Count": common.NewVariableValue(10, false),
		"Flag":  common.NewVariableValue(true, false),
	}
	require.False(t, job.ShouldSkip(data))
	job.shouldSkip = "{{if and (gt .Count 5) .Flag}}true{{end}}"
	require.True(t, job.ShouldSkip(data))
	job.shouldSkip = "{{if and (gt .Count 5) (eq .Flag true)}}true{{end}}"
	require.True(t, job.ShouldSkip(data))

	data = map[string]common.VariableValue{
		"Count": common.NewVariableValue(1, false),
		"Flag":  common.NewVariableValue(true, false),
	}
	require.False(t, job.ShouldSkip(data))
	data = map[string]common.VariableValue{
		"Count": common.NewVariableValue(10, false),
		"Flag":  common.NewVariableValue(false, false),
	}
	require.False(t, job.ShouldSkip(data))
}

// Test properties after serialization using YAML
func Test_ShouldSerializeYamlJobDefinition(t *testing.T) {
	// GIVEN - job definition is created
	job := newTestJobDefinition("name")
	// AND valid job
	err := job.ValidateBeforeSave(testEncryptedKey)
	require.NoError(t, err)
	err = job.Validate()
	require.NoError(t, err)

	// WHEN marshaling job into yaml
	ser, err := yaml.Marshal(job)

	// THEN it should not fail
	require.NoError(t, err)

	// WHEN unmarshalling job from yaml
	loaded, err := NewJobDefinitionFromYaml(ser)
	// THEN it should not fail
	require.NoError(t, err)

	// AND should be valid
	err = loaded.Validate()
	require.NoError(t, err)
	err = loaded.ValidateBeforeSave(testEncryptedKey)
	require.NoError(t, err)
	require.NoError(t, job.Equals(loaded))
}

// Test plugin version
func Test_ShouldValidatePluginVersion(t *testing.T) {
	// GIVEN - job definition is created
	job := newTestJobDefinition("name")
	job.PublicPlugin = true
	// WHEN validating plugin job
	err := job.Validate()
	// THEN it should fail with version error
	require.Error(t, err)
	require.Contains(t, err.Error(), "no major/minor")

	// Setting semantic version to garbage
	job.SemVersion = "blah"
	err = job.Validate()
	// THEN it should fail with version error
	require.Error(t, err)
	require.Contains(t, err.Error(), "no major/minor")

	// WHEN using major version
	job.SemVersion = "1"
	err = job.Validate()
	// THEN it should fail with version error
	require.Error(t, err)
	require.Contains(t, err.Error(), "no major/minor")

	// WHEN using garbage minor
	job.SemVersion = "1.blah"
	_, err = job.CheckSemVersion()
	require.Error(t, err)
	// AND WHEN validation
	err = job.Validate()
	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "bad last digit")

	// WHEN validating using garbage patch
	job.SemVersion = "1.0.blah"
	err = job.Validate()

	// THEN validation should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "bad last digit")

	// WHEN using garbage RC for semantic version
	job.SemVersion = "1.0.blah-rc1"
	err = job.Validate()
	// THEN validation should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "bad last digit")

	// WHEN using valid RC
	job.SemVersion = "1.0.rc1"
	err = job.Validate()
	// THEN it should not fail
	require.NoError(t, err)
	require.Equal(t, "000000001.000000000", job.NormalizedSemVersion())

	// WHEN using valid dev
	job.SemVersion = "1.0.1-dev"
	err = job.Validate()
	// THEN it should not fail
	require.NoError(t, err)
	require.Equal(t, "000000001.000000000.000000001", job.NormalizedSemVersion())

	// WHEN using valid dev
	job.SemVersion = "1.0.123rc1"
	err = job.Validate()
	// THEN it should not fail
	require.NoError(t, err)
	require.Equal(t, "000000001.000000000.000000123", job.NormalizedSemVersion())

	// WHEN using valid dev
	job.SemVersion = "1.0.123-rc1"
	err = job.Validate()
	// THEN it should not fail
	require.NoError(t, err)
	require.Equal(t, "000000001.000000000.000000123", job.NormalizedSemVersion())

	// WHEN using valid dev
	job.SemVersion = "1.0.dev"
	err = job.Validate()
	// THEN it should not fail
	require.NoError(t, err)
	require.Equal(t, "000000001.000000000", job.NormalizedSemVersion())

	// WHEN using valid dev
	job.SemVersion = "1.0.123dev1"
	err = job.Validate()
	// THEN it should not fail
	require.NoError(t, err)
	require.Equal(t, "000000001.000000000.000000123", job.NormalizedSemVersion())

	// WHEN using valid dev
	job.SemVersion = "1.0.123-dev2"
	err = job.Validate()
	// THEN it should not fail
	require.NoError(t, err)
	require.Equal(t, "000000001.000000000.000000123", job.NormalizedSemVersion())

	// WHEN using valid dev
	job.SemVersion = "1.2.3.4"
	err = job.Validate()
	// THEN it should not fail
	require.NoError(t, err)
	require.Equal(t, "000000001.000000002.000000003", job.NormalizedSemVersion())
}

// Test next task for job definition
func Test_ShouldNextTaskForJobDefinition(t *testing.T) {
	// GIVEN - job definition is created
	job := newTestJobDefinition("name")
	err := job.Validate()
	require.NoError(t, err)

	// WHEN finding first task
	firstTask, err := job.GetFirstTask()
	// THEN it should match expected type
	require.NoError(t, err)
	require.Equal(t, "task1", firstTask.TaskType)

	// WHEN finding next task to execute
	next, _, err := job.GetNextTask(firstTask, "completed", "")
	require.NoError(t, err)

	// THEN it should return valid task
	require.NotNil(t, next)
}

// Test next task for job definition based on exit-code
func Test_ShouldNextTaskFromExitCodeForJobDefinition(t *testing.T) {
	// GIVEN - job definition is created
	job := newTestJobDefinition("name")
	err := job.Validate()
	require.NoError(t, err)

	// WHEN finding first task
	firstTask, err := job.GetFirstTask()
	// THEN it should match expected type
	require.NoError(t, err)
	require.Equal(t, "task1", firstTask.TaskType)

	//OnExitCode["400"] = "COMPLETED"
	//OnExitCode["500"] = "FAILED"
	//OnExitCode["600"] = "HARD_ERROR"

	// WHEN finding next task to execute based on 400 - COMPLETED
	next, _, err := job.GetNextTask(firstTask, "completed", "400")
	// THEN completed it should return valid task
	require.NotNil(t, next)

	// WHEN finding next task to execute based on 500 - FAILED
	next, _, err = job.GetNextTask(firstTask, "completed", "500")
	require.NotNil(t, next)
	require.Nil(t, err)

	// WHEN finding next task to execute based on 600 - FATAL
	next, _, err = job.GetNextTask(firstTask, "completed", "600")
	require.NoError(t, err)
	require.Equal(t, "task2", next.TaskType)
	next, _, err = job.GetNextTask(firstTask, "failed", "600")
	require.NoError(t, err)
	require.Nil(t, next)
}

// Test properties after serialization using JSON
func Test_ShouldSerializeJsonJobDefinition(t *testing.T) {
	// GIVEN - job definition is created
	job := newTestJobDefinition("name")

	// WHEN job is marshalled
	ser, err := json.Marshal(job)

	// THEN it should not fail
	require.NoError(t, err)

	// WHEN job is unmarshalled from json
	loaded := NewJobDefinition("")
	err = json.Unmarshal(ser, loaded)
	// THEN it should not fail
	require.NoError(t, err)

	// WHEN validating with RawYaml
	loaded.RawYaml = string(ser)

	// it should not fail and match job
	err = loaded.ValidateBeforeSave(testEncryptedKey)
	require.NoError(t, err)
	require.NoError(t, job.Equals(loaded))
}

// Test pipe job
func Test_ShouldParsePipeString(t *testing.T) {
	// GIVEN job-definition loaded from pipeline yaml
	b, err := ioutil.ReadFile("../../docs/examples/io.formicary.tokens.yaml")
	require.NoError(t, err)
	job, err := NewJobDefinitionFromYaml(b)
	require.NoError(t, err)
	require.NotNil(t, job.ReportStdoutTask())
	task, _, err := job.GetDynamicTask(
		"etherscan-contracts",
		map[string]common.VariableValue{"JobRetry": common.NewVariableValue(1, false)},
	)
	require.NoError(t, err)
	require.True(t, len(task.Script) > 1)
	require.NotNil(t, job.ReportStdoutTask())
	require.False(t, strings.Contains(task.Script[0], "&lt;"), task.Script[0])
	task = job.GetTask("analyze")
	require.True(t, task.ReportStdout)
	task, _, err = job.GetDynamicTask(
		"santiment",
		map[string]common.VariableValue{"JobRetry": common.NewVariableValue(1, false)},
	)
}

// Test iterate loop
func Test_ShouldBuildIterateJob(t *testing.T) {
	// GIVEN job-definition loaded from pipeline yaml
	b, err := ioutil.ReadFile("../../docs/examples/iterate-job.yaml")
	require.NoError(t, err)
	job, err := NewJobDefinitionFromYaml(b)
	require.NoError(t, err)
	require.Equal(t, 5, len(job.Tasks))
	task, _, err := job.GetDynamicTask(
		"task-1",
		map[string]common.VariableValue{"JobRetry": common.NewVariableValue(1, false)},
	)
	require.NoError(t, err)
	require.Equal(t, 1, len(task.Script))
	task, _, err = job.GetDynamicTask(
		"task-4",
		map[string]common.VariableValue{},
	)
	require.NoError(t, err)
	require.Equal(t, 1, len(task.Script))
}

// Test json serialization of yaml job definition
func Test_ShouldSerializeSensorFromYAML(t *testing.T) {
	// GIVEN job-definition loaded from pipeline yaml
	b, err := ioutil.ReadFile("../../docs/examples/sensor-job.yaml")
	require.NoError(t, err)
	job, err := NewJobDefinitionFromYaml(b)
	require.NoError(t, err)
	require.Equal(t, 3, len(job.Tasks))
	require.Equal(t, 0, job.Tasks[0].TaskOrder)
	require.Equal(t, 1, job.Tasks[1].TaskOrder)
	require.Equal(t, 2, job.Tasks[2].TaskOrder)
	started := time.Now()
	task, _, err := job.GetDynamicTask(
		"first",
		map[string]common.VariableValue{
			"JobElapsedSecs": common.NewVariableValue(uint(time.Since(started).Seconds()), false),
		},
	)
	require.NoError(t, err)
	require.NotEqual(t, "", task.Script[0])
}

// Test json serialization of yaml job definition
func Test_ShouldETLSerializeFromYAML(t *testing.T) {
	// GIVEN job-definition loaded from pipeline yaml
	b, err := ioutil.ReadFile("../../docs/examples/etl-sum-job.yaml")
	require.NoError(t, err)
	job, err := NewJobDefinitionFromYaml(b)
	require.NoError(t, err)
	require.Equal(t, 3, len(job.Tasks))
	require.Equal(t, 0, job.Tasks[0].TaskOrder)
	require.Equal(t, 1, job.Tasks[1].TaskOrder)
	require.Equal(t, 2, job.Tasks[2].TaskOrder)
	task, _, err := job.GetDynamicTask(
		"extract",
		map[string]common.VariableValue{
			"data_string": common.NewVariableValue("{\"1001\": 301.27, \"1002\": 433.21, \"1003\": 502.22}", false),
		},
	)
	require.NoError(t, err)
	require.Equal(t, "{\"1001\": 301.27, \"1002\": 433.21, \"1003\": 502.22}", task.Variables[0].Value)
}

// Test json serialization of yaml job definition
func Test_ShouldSerializeHelloWorldFromYAML(t *testing.T) {
	// GIVEN job-definition loaded from pipeline yaml
	b, err := ioutil.ReadFile("../../docs/examples/hello_world.yaml")
	require.NoError(t, err)
	job, err := NewJobDefinitionFromYaml(b)
	require.NoError(t, err)
	require.Equal(t, 3, len(job.Tasks))
	require.Equal(t, 0, job.Tasks[0].TaskOrder)
	require.Equal(t, 1, job.Tasks[1].TaskOrder)
	require.Equal(t, 2, job.Tasks[2].TaskOrder)
	task, _, err := job.GetDynamicTask(
		"combine",
		map[string]common.VariableValue{"JobRetry": common.NewVariableValue(1, false)},
	)
	require.NoError(t, err)
	require.Equal(t, 3, len(task.Script))
	task, _, err = job.GetDynamicTask(
		"combine",
		map[string]common.VariableValue{"JobRetry": common.NewVariableValue(4, false)},
	)
	require.NoError(t, err)
	require.Equal(t, 2, len(task.Script))
	task, _, err = job.GetDynamicTask(
		"combine",
		map[string]common.VariableValue{},
	)
	require.Error(t, err) // should fail without JobRetry
}

// Test json serialization of forked job definition
func Test_ShouldSerializeForkedJobDefinitionIntoJSON(t *testing.T) {
	// GIVEN job-definition loaded from pipeline yaml
	b, err := ioutil.ReadFile("../../fixtures/fork_job.yaml")
	require.NoError(t, err)
	job, err := NewJobDefinitionFromYaml(b)
	require.NoError(t, err)
	require.Equal(t, 2, len(job.Notify[common.EmailChannel].Recipients))
	require.Equal(t, common.NotifyWhenAlways, job.Notify[common.EmailChannel].When)
	job.RawYaml = string(b)
	err = job.ValidateBeforeSave(testEncryptedKey)
	require.NoError(t, err)

	require.Equal(t, "{\"email\":{\"recipients\":[\"support@formicary.io\",\"bhatti@plexobject.com\"],\"when\":\"always\"}}", job.NotifySerialized)
	// WHEN marshaling job
	b, err = json.Marshal(job)

	// THEN it should not fail
	require.NoError(t, err)
}

// Test json serialization of job definition
func Test_ShouldSerializeJobDefinitionIntoJSON(t *testing.T) {
	// GIVEN job-definition loaded from pipeline yaml
	b, err := ioutil.ReadFile("../../fixtures/kube-build.yaml")
	require.NoError(t, err)
	job, err := NewJobDefinitionFromYaml(b)
	require.NoError(t, err)
	job.RawYaml = string(b)
	err = job.ValidateBeforeSave(testEncryptedKey)
	require.NoError(t, err)

	b, err = json.Marshal(job)
	require.NoError(t, err)
}

// Test build task from yaml with template config
func Test_ShouldGetDynamicTaskForJobDefinition(t *testing.T) {
	yamlConfigs := []string{
		"../../fixtures/basic-job.yaml",
		"../../fixtures/encoding-job.yaml",
		"../../fixtures/kube-build.yaml",
		"../../fixtures/cron-kube-build.yaml",
		"../../fixtures/test_job.yaml",
		"../../fixtures/docker_build.yaml"}
	for _, yamlConfig := range yamlConfigs {
		// GIVEN job definition defined in yaml
		b, err := ioutil.ReadFile(yamlConfig)
		require.NoError(t, err)

		// WHEN creating job definition from yaml
		job, err := NewJobDefinitionFromYaml(b)

		// THEN it should not fail
		require.NoError(t, err)
		job.RawYaml = string(b)

		// AND it should be valid
		err = job.ValidateBeforeSave(testEncryptedKey)
		require.NoError(t, err)

		// WHEN marshaling job into json
		b, err = json.Marshal(job)
		err = job.ValidateBeforeSave(testEncryptedKey)
		// THEN it should not fail
		require.NoError(t, err)

		for _, task := range job.Tasks {
			// WHEN finding dynamic task by type and params
			params := map[string]common.VariableValue{
				"Token":             common.NewVariableValue("tok1", false),
				"IsWindowsPlatform": common.NewVariableValue(true, false),
				"Platform":          common.NewVariableValue("IOS", false),
				"OSVersion":         common.NewVariableValue("13.2", false),
				"Language":          common.NewVariableValue("GO", false),
				"IsMpeg4":           common.NewVariableValue(true, false),
				"Nonce":             common.NewVariableValue(1, false),
			}
			dynTask, _, err := job.GetDynamicTask(task.TaskType, params)
			require.NoError(t, err)

			// dynamic task should not be nil
			require.NotNil(t, dynTask)
			require.Equal(t, task.TaskType, dynTask.TaskType)
		}
	}
}

// Test yaml deserialize
func Test_ShouldYamlDeserializeForJobDefinition(t *testing.T) {
	files := []string{
		"../../fixtures/test_job.yaml",
		"../../fixtures/basic-job.yaml",
		"../../fixtures/kube-build.yaml",
		"../../fixtures/encoding-job.yaml"}
	tasks := []int{3, 9, 5, 6}
	configs := []int{4, 3, 4, 4}
	for i, file := range files {
		b, err := ioutil.ReadFile(file)
		require.NoError(t, err)

		// GIVEN job definition from YAML
		job, err := NewJobDefinitionFromYaml(b)
		require.NoError(t, err)

		// WHEN validating job
		err = job.ValidateBeforeSave(testEncryptedKey)

		// THEN it should not fail and match config
		require.NoError(t, err)
		require.Equal(t, configs[i], len(job.Variables))
		require.Equal(t, tasks[i], len(job.Tasks))
	}
}

// Test build job config with for loop
func Test_ShouldParseLoopJobDefinition(t *testing.T) {
	// GIVEN a job loaded from YAML file
	b, err := ioutil.ReadFile("../../docs/examples/loop-job.yaml")
	require.NoError(t, err)
	job, err := NewJobDefinitionFromYaml(b)
	require.NoError(t, err)
	require.NotNil(t, job)
	params := map[string]common.VariableValue{}
	require.True(t, rangeRegex.FindStringIndex(string(b)) != nil)
	task, _, err := job.GetDynamicTask("t3", params)
	require.Equal(t, 17, len(task.Script))
}

// Test build job config with shouldSkip and cron
func Test_ShouldParseFilterCronJobDefinition(t *testing.T) {
	// GIVEN a job loaded from YAML file
	b, err := ioutil.ReadFile("../../fixtures/hello_world_scheduled.yaml")
	require.NoError(t, err)
	job, err := NewJobDefinitionFromYaml(b)
	require.NoError(t, err)
	require.NotNil(t, job)
	require.NotEqual(t, "", job.SkipIf())
	params := map[string]common.VariableValue{
		"Target": common.NewVariableValue("charlie", false),
	}
	require.True(t, job.ShouldSkip(params))
	params = map[string]common.VariableValue{
		"Target": common.NewVariableValue("bob", false),
	}
	require.False(t, job.ShouldSkip(params))
	require.NotEqual(t, "", job.CronAndScheduleTime())
	date, userKey := job.GetCronScheduleTimeAndUserKey()
	require.NotNil(t, date)
	require.NotEqual(t, "", userKey)
	job.Disabled = true
	date, userKey = job.GetCronScheduleTimeAndUserKey()
	require.Nil(t, date)
	require.Equal(t, "", userKey)
}

// Test secret config
func Test_ShouldEncryptSecretConfigForJobDefinition(t *testing.T) {
	// GIVEN a job
	job := NewJobDefinition("test-job")
	_, _ = job.AddConfig("k1", "my-secret", true)
	_, _ = job.AddConfig("k2", "plain", false)
	_, _ = job.AddConfig("k3", 101, true)

	key := crypto.SHA256Key("my key")

	// WHEN encrypting config
	err1 := job.GetConfig("k1").Encrypt(key)
	err2 := job.GetConfig("k2").Encrypt(key)
	err3 := job.GetConfig("k3").Encrypt(key)
	// THEN it should not fail
	require.NoError(t, err1)
	require.NoError(t, err2)
	require.NoError(t, err3)

	// AND secret config should begin with _ENCRYPTED_
	require.Contains(t, job.GetConfig("k1").Value, "_ENCRYPTED_")
	require.Equal(t, "plain", job.GetConfig("k2").Value)
	require.Contains(t, job.GetConfig("k3").Value, "_ENCRYPTED_")

	// WHEN decrypting config
	err1 = job.GetConfig("k1").Decrypt(key)
	err2 = job.GetConfig("k2").Decrypt(key)
	err3 = job.GetConfig("k3").Decrypt(key)
	// THEN it should not fail
	require.NoError(t, err1)
	require.NoError(t, err2)
	require.NoError(t, err3)

	// AND secret config should match original value
	require.Equal(t, "my-secret", job.GetConfig("k1").Value)
	require.Equal(t, "plain", job.GetConfig("k2").Value)
	require.Equal(t, "101", job.GetConfig("k3").Value)
}

// Test build job config from yaml with template
func Test_ShouldGetDynamicConfigForJobDefinition(t *testing.T) {
	// GIVEN a job loaded from YAML file
	b, err := ioutil.ReadFile("../../fixtures/kube-build.yaml")
	require.NoError(t, err)
	job, err := NewJobDefinitionFromYaml(b)
	if err != nil {
		require.NoError(t, err)
	}
	require.NotNil(t, job)

	// WHEN getting dynamic config
	params := map[string]interface{}{
		"Token":             "tok1",
		"IsWindowsPlatform": true,
		"Platform":          "LINUX",
		"OSVersion":         "20.04.1",
	}
	cfg := job.GetDynamicConfigAndVariables(params)
	// THEN it should not fail and match expected values
	require.Equal(t, 7, len(cfg))
	require.Equal(t, "jv1", cfg["jk1"].Value)
	require.Equal(t, "License", job.Resources.ResourceType)
	require.Equal(t, "my-job", job.Resources.ExtractConfig.ContextPrefix)
}

func Test_ShouldParseTimeout(t *testing.T) {
	// GIVEN a job loaded from YAML
	b, err := ioutil.ReadFile("../../docs/examples/messaging-job.yaml")
	require.NoError(t, err)
	job, err := NewJobDefinitionFromYaml(b)
	if err != nil {
		require.NoError(t, err)
	}
	require.NotNil(t, job)

	// WHEN fetching dynamic task
	params := map[string]common.VariableValue{}
	for _, next := range job.Variables {
		if vv, err := next.GetVariableValue(); err == nil {
			params[next.Name] = vv
		}
	}
	task, _, err := job.GetDynamicTask("trigger", params)

	// THEN it should not fail
	require.NoError(t, err)
	require.Equal(t, 1*time.Minute, job.Timeout)
	require.Equal(t, 0, len(task.Variables))
	require.Equal(t, common.TaskMethod("MESSAGING"), task.Method)
}

func Test_ShouldParseVariables(t *testing.T) {
	// GIVEN a job loaded from YAML
	b, err := ioutil.ReadFile("../../docs/examples/trivy-scan-job.yaml")
	require.NoError(t, err)
	job, err := NewJobDefinitionFromYaml(b)
	if err != nil {
		require.NoError(t, err)
	}
	require.NotNil(t, job)

	// WHEN fetching dynamic task
	params := map[string]common.VariableValue{}
	for _, next := range job.Variables {
		if vv, err := next.GetVariableValue(); err == nil {
			params[next.Name] = vv
		}
	}
	task, _, err := job.GetDynamicTask("scan", params)

	// THEN it should not fail
	require.NoError(t, err)
	require.Equal(t, 3, len(task.Variables))
	require.Equal(t, common.TaskMethod("KUBERNETES"), task.Method)
}

func newTestJobDefinition(name string) *JobDefinition {
	job := NewJobDefinition("io.formicary.test." + name)
	_, _ = job.AddVariable("jk1", "jv1")
	_, _ = job.AddVariable("jk2", map[string]int{"a": 1, "b": 2})
	_, _ = job.AddVariable("jk3", "jv3")
	job.Resources.ResourceType = "License"
	job.Resources.Value = 2
	job.Resources.Platform = "Vendor"
	job.Resources.Tags = []string{"vendor-a-api"}
	job.Resources.ExtractConfig.ContextPrefix = "my-job"
	job.Resources.ExtractConfig.Properties = []string{"license-id", "expiration"}

	task1 := NewTaskDefinition("task1", common.Shell)
	task1.OnExitCode["completed"] = "task2"
	task1.BeforeScript = []string{"t1_cmd1", "t1_cmd2", "t1_cmd3"}
	task1.AfterScript = []string{"t1_cmd1", "t1_cmd2", "t1_cmd3"}
	task1.Script = []string{"t1_cmd1", "t1_cmd2", "t1_cmd3"}
	task1.Headers = map[string]string{"t1_h1": "1", "t1_h2": "true", "t1_h3": "three"}
	task1.Method = common.Docker
	task1.Resources.ResourceType = "NetworkBandwidth"
	task1.Resources.Value = 10
	task1.Resources.Platform = "LINUX"
	task1.Resources.Tags = []string{"gig", "network"}
	task1.Resources.ExtractConfig.ContextPrefix = "connection"
	task1.Resources.ExtractConfig.Properties = []string{"network-id", "route"}
	task1.OnExitCode["completed"] = "task2"
	task1.OnExitCode["600"] = "FATAL"

	task2 := NewTaskDefinition("task2", common.Shell)
	task2.OnExitCode["completed"] = "task3"
	task2.AllowFailure = true
	_, _ = task2.AddVariable("t2k1", "v1")
	_, _ = task2.AddVariable("t2k2", []string{"i", "j", "k"})
	_, _ = task2.AddVariable("t2k3", "v3")
	_, _ = task2.AddVariable("t2k4", 14.123)
	task2.BeforeScript = []string{"t2_cmd1", "t2_cmd2", "t2_cmd3"}
	task2.Script = []string{"t2_cmd1", "t2_cmd2", "t2_cmd3"}
	task2.Method = common.Docker

	task3 := NewTaskDefinition("task3", common.Shell)
	task3.AlwaysRun = true
	_, _ = task3.AddVariable("t3k1", true)
	_, _ = task3.AddVariable("t3k2", 50)
	_, _ = task3.AddVariable("t3k3", map[string]string{"i": "a", "j": "b", "k": "c"})
	_, _ = task3.AddVariable("t3k4", 4.881)
	task3.BeforeScript = []string{"t3_cmd1", "t3_cmd2", "t3_cmd3"}
	task3.Script = []string{"t3_cmd1", "t3_cmd2", "t3_cmd3"}
	task3.Method = common.Docker
	task3.AlwaysRun = true
	job.AddTask(task1)
	job.AddTask(task2)
	job.AddTask(task3)
	job.UpdateRawYaml()
	_ = job.ValidateBeforeSave(testEncryptedKey)
	_ = job.AfterLoad(testEncryptedKey)
	return job
}
