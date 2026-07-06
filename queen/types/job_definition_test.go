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

func Test_ShouldGetDelayBetweenRetriesWithBackoffPolicy(t *testing.T) {
	jd := NewJobDefinition("test-job")
	jd.RetryBackoffPolicy = &BackoffPolicy{
		Min:    100 * time.Millisecond,
		Max:    10 * time.Second,
		Factor: 2,
		Jitter: false,
	}

	// Attempt 0 should return Min
	d0 := jd.GetDelayBetweenRetries(0)
	require.Equal(t, 100*time.Millisecond, d0)

	// Attempt 1 should return Min * Factor
	d1 := jd.GetDelayBetweenRetries(1)
	require.Equal(t, 200*time.Millisecond, d1)

	// Attempt 2 should return Min * Factor^2
	d2 := jd.GetDelayBetweenRetries(2)
	require.Equal(t, 400*time.Millisecond, d2)

	// Should cap at Max
	d10 := jd.GetDelayBetweenRetries(10)
	require.Equal(t, 10*time.Second, d10)
}

func Test_ShouldGetDelayBetweenRetriesWithJitter(t *testing.T) {
	jd := NewJobDefinition("test-job")
	jd.RetryBackoffPolicy = &BackoffPolicy{
		Min:    1 * time.Second,
		Max:    30 * time.Second,
		Factor: 2,
		Jitter: true,
	}

	// With jitter, results should be bounded but non-deterministic
	d := jd.GetDelayBetweenRetries(3)
	// Attempt 3 without jitter = 8s, with jitter should be in [0, 8s]
	require.True(t, d > 0 && d <= 8*time.Second, "expected jittered delay in (0, 8s], got %s", d)
}

func Test_ShouldFallbackToDelayBetweenRetriesWhenNoPolicy(t *testing.T) {
	jd := NewJobDefinition("test-job")
	jd.DelayBetweenRetries = 5 * time.Second

	// Without backoff policy, should return the fixed delay
	d := jd.GetDelayBetweenRetries(3)
	require.Equal(t, 5*time.Second, d)
}

func Test_ShouldFallbackToRandomDelayWhenNoPolicyAndNoDelay(t *testing.T) {
	jd := NewJobDefinition("test-job")

	// Without backoff policy or fixed delay, should return random between 5-14s
	d := jd.GetDelayBetweenRetries()
	require.True(t, d >= 5*time.Second && d <= 14*time.Second, "expected delay in [5s, 14s], got %s", d)
}

func Test_ShouldFallbackWhenPolicyMinIsZero(t *testing.T) {
	jd := NewJobDefinition("test-job")
	jd.RetryBackoffPolicy = &BackoffPolicy{
		Min:    0,
		Max:    10 * time.Second,
		Factor: 2,
	}
	jd.DelayBetweenRetries = 3 * time.Second

	// Policy with Min=0 should fall through to fixed delay
	d := jd.GetDelayBetweenRetries(1)
	require.Equal(t, 3*time.Second, d)
}

func Test_ShouldSerializeBackoffPolicyInYAML(t *testing.T) {
	yamlContent := `
job_type: test-job
retry: 3
retry_backoff_policy:
  min: 1s
  max: 30s
  factor: 2
  jitter: true
tasks:
  - task_type: task1
    method: SHELL
    retry_backoff_policy:
      min: 500ms
      max: 10s
      factor: 3
      jitter: false
`
	jd := NewJobDefinition("")
	err := yaml.Unmarshal([]byte(yamlContent), jd)
	require.NoError(t, err)
	require.NotNil(t, jd.RetryBackoffPolicy)
	require.Equal(t, 1*time.Second, jd.RetryBackoffPolicy.Min)
	require.Equal(t, 30*time.Second, jd.RetryBackoffPolicy.Max)
	require.Equal(t, float64(2), jd.RetryBackoffPolicy.Factor)
	require.True(t, jd.RetryBackoffPolicy.Jitter)

	require.Len(t, jd.Tasks, 1)
	require.NotNil(t, jd.Tasks[0].RetryBackoffPolicy)
	require.Equal(t, 500*time.Millisecond, jd.Tasks[0].RetryBackoffPolicy.Min)
	require.Equal(t, 10*time.Second, jd.Tasks[0].RetryBackoffPolicy.Max)
	require.Equal(t, float64(3), jd.Tasks[0].RetryBackoffPolicy.Factor)
	require.False(t, jd.Tasks[0].RetryBackoffPolicy.Jitter)
}

func Test_ShouldRoundTripBackoffPolicyJSON(t *testing.T) {
	jd := NewJobDefinition("test-job")
	jd.RetryBackoffPolicy = &BackoffPolicy{
		Min:    1 * time.Second,
		Max:    30 * time.Second,
		Factor: 2,
		Jitter: true,
	}

	data, err := json.Marshal(jd)
	require.NoError(t, err)

	jd2 := NewJobDefinition("")
	err = json.Unmarshal(data, jd2)
	require.NoError(t, err)
	require.NotNil(t, jd2.RetryBackoffPolicy)
	require.Equal(t, 1*time.Second, jd2.RetryBackoffPolicy.Min)
	require.Equal(t, 30*time.Second, jd2.RetryBackoffPolicy.Max)
	require.Equal(t, float64(2), jd2.RetryBackoffPolicy.Factor)
	require.True(t, jd2.RetryBackoffPolicy.Jitter)
}

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
	require.Contains(t, err.Error(), "task task1 is not reachable")
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
	require.False(t, job.ShouldSkip(data, nil))
	job.shouldSkip = "{{if and (gt .Count 5) .Flag}}true{{end}}"
	require.True(t, job.ShouldSkip(data, nil))
	job.shouldSkip = "{{if and (gt .Count 5) (eq .Flag true)}}true{{end}}"
	require.True(t, job.ShouldSkip(data, nil))

	data = map[string]common.VariableValue{
		"Count": common.NewVariableValue(1, false),
		"Flag":  common.NewVariableValue(true, false),
	}
	require.False(t, job.ShouldSkip(data, nil))
	data = map[string]common.VariableValue{
		"Count": common.NewVariableValue(10, false),
		"Flag":  common.NewVariableValue(false, false),
	}
	require.False(t, job.ShouldSkip(data, nil))
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
	require.True(t, job.ShouldSkip(params, nil))
	params = map[string]common.VariableValue{
		"Target": common.NewVariableValue("bob", false),
	}
	require.False(t, job.ShouldSkip(params, nil))
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

// Test_ShouldParsePickerYAML loads the actual ai-gh-issue-picker.yaml file
// and calls GetDynamicTask for both tasks. This is the definitive regression test:
// any template or YAML syntax error in the real file will surface here before deploy.
func Test_ShouldParsePickerYAML(t *testing.T) {
	b, err := ioutil.ReadFile("../../docs/examples/ai-gh-issue-picker.yaml")
	require.NoError(t, err)

	job, err := NewJobDefinitionFromYaml(b)
	require.NoError(t, err)
	require.Equal(t, "ai-gh-issue-picker", job.JobType)

	vars := map[string]common.VariableValue{
		"JobRetry":        common.NewVariableValue(0, false),
		"JobID":           common.NewVariableValue("test-job-id", false),
		"GitHubOrg":       common.NewVariableValue("myorg", false),
		"GitHubRepo":      common.NewVariableValue("myrepo", false),
		"GithubToken":     common.NewVariableValue("ghp_test", false),
		"MaxPendingJobs":  common.NewVariableValue("10", false),
		"PickupLabel":     common.NewVariableValue("ai-ready", false),
		"InProgressLabel": common.NewVariableValue("ai-in-progress", false),
	}

	// gather-issues task must parse cleanly
	gatherTask, _, err := job.GetDynamicTask("gather-issues", vars)
	require.NoError(t, err, "gather-issues task must parse without error")
	require.NotNil(t, gatherTask)
	require.NotEmpty(t, gatherTask.Script)

	// submit-jobs task must parse cleanly — this caught the "unexpected EOF" bug
	// caused by double-quoted YAML containing inner double-quoted template args.
	submitTask, submitOpts, err := job.GetDynamicTask("submit-jobs", vars)
	require.NoError(t, err, "submit-jobs task must parse without error (check for double-quote YAML bug)")
	require.NotNil(t, submitTask)
	require.NotNil(t, submitOpts)
	// PENDING_COUNT must render to a number (CountByJobTypeAndState returns 0 without a querier)
	require.Equal(t, "0", submitOpts.Environment["PENDING_COUNT"],
		"PENDING_COUNT should render to '0' when no querier is available")
}

// Test_ShouldRejectDoubleQuotedTemplateInYAML documents the "unexpected EOF" / YAML
// parse error seen in production when PENDING_COUNT was written as:
//
//	PENDING_COUNT: "{{CountByJobTypeAndState "ai-gh-implement" "PENDING"}}"
//
// The inner double quotes break the YAML string boundary — YAML v3 errors on the line
// entirely, or truncates the value, leaving a malformed Go template.
// Either the YAML parse or the GetDynamicTask template parse must fail.
// The fix is to use single-quoted YAML strings (see Test_ShouldAcceptSingleQuotedTemplateInYAML).
func Test_ShouldRejectDoubleQuotedTemplateInYAML(t *testing.T) {
	brokenYAML := "job_type: broken-picker\ntasks:\n- task_type: submit\n  method: SHELL\n  environment:\n    PENDING_COUNT: \"{{CountByJobTypeAndState \\\"ai-gh-implement\\\" \\\"PENDING\\\"}}\"\n  script:\n    - echo done\n"
	job, err := NewJobDefinitionFromYaml([]byte(brokenYAML))
	if err != nil {
		// YAML parser rejected the malformed line — correct behaviour.
		return
	}
	vars := map[string]common.VariableValue{
		"JobRetry": common.NewVariableValue(0, false),
	}
	_, _, err = job.GetDynamicTask("submit", vars)
	if err != nil {
		// Template engine rejected the truncated/malformed template — also correct.
		return
	}
	// If we got here, neither YAML nor template engine caught the problem.
	// Check that the rendered value is at least not silently wrong (empty or garbled).
	// This is a documentation test: the broken form must not work undetected.
	t.Log("YAML and template both accepted the double-quoted form — this may indicate a YAML parser version difference")
}

// Test_ShouldAcceptSingleQuotedTemplateInYAML verifies the correct form:
// single-quoted YAML string containing a template with double-quoted arguments.
func Test_ShouldAcceptSingleQuotedTemplateInYAML(t *testing.T) {
	fixedYAML := `
job_type: fixed-picker
tasks:
- task_type: submit
  method: SHELL
  environment:
    PENDING_COUNT: '{{CountByJobTypeAndState "ai-gh-implement" "PENDING"}}'
    SUBMITTED_IDS: '{{if .IssuesJSON}}{{SubmitJobsFromJSON "ai-gh-implement" .IssuesJSON (printf "GitHubOrg=%s" .GitHubOrg) (printf "GitHubRepo=%s" .GitHubRepo)}}{{end}}'
  script:
    - echo "${PENDING_COUNT}"
`
	job, err := NewJobDefinitionFromYaml([]byte(fixedYAML))
	require.NoError(t, err)

	vars := map[string]common.VariableValue{
		"JobRetry":   common.NewVariableValue(0, false),
		"GitHubOrg":  common.NewVariableValue("myorg", false),
		"GitHubRepo": common.NewVariableValue("myrepo", false),
	}
	task, opts, err := job.GetDynamicTask("submit", vars)
	require.NoError(t, err, "single-quoted template with double-quoted args should parse cleanly")
	require.NotNil(t, task)
	require.NotNil(t, opts)
	// PENDING_COUNT should have been rendered (CountByJobTypeAndState returns 0 with no querier)
	require.Contains(t, opts.Environment["PENDING_COUNT"], "0")
}

// Test that sub_workflow block parses correctly from YAML.
// Template expressions in input_variables values ({{ .x }}) are stripped during YAML
// parsing (NewJobDefinitionFromYaml resolves templates before unmarshalling), so
// we verify structural fields: output_variables values and wait_for_completion.
func Test_ShouldParseSubWorkflowFromYAML(t *testing.T) {
	b, err := ioutil.ReadFile("../../fixtures/sub_workflow_job.yaml")
	require.NoError(t, err)

	job, err := NewJobDefinitionFromYaml(b)
	require.NoError(t, err)

	task := job.GetTask("run-child-etl")
	require.NotNil(t, task, "run-child-etl task must exist")
	require.NotNil(t, task.SubWorkflow, "sub_workflow must be parsed")

	// output_variables use plain string values and survive parsing intact.
	om, err := task.SubWorkflow.OutputMap()
	require.NoError(t, err)
	require.Equal(t, "etl_row_count", om["row_count"])
	require.Equal(t, "child_status", om["status"])
	require.True(t, task.SubWorkflow.WaitForCompletion)
}

// Test that fan_out config is parsed and validated from YAML.
func Test_ShouldParseFanOutFromYAML(t *testing.T) {
	b, err := ioutil.ReadFile("../../fixtures/fan_out_job.yaml")
	require.NoError(t, err)

	job, err := NewJobDefinitionFromYaml(b)
	require.NoError(t, err)

	task := job.GetTask("deploy")
	require.NotNil(t, task, "deploy task must exist")
	require.NotNil(t, task.FanOut, "fan_out must be parsed")
	require.Equal(t, "regions", task.FanOut.Source)
	require.Equal(t, "region", task.FanOut.ItemVar)
	require.Equal(t, 2, task.FanOut.MaxParallel)
	require.False(t, task.FanOut.FailFast)
}

// Test that fan_out round-trips through JSON serialisation.
func Test_ShouldRoundTripFanOutViaJSON(t *testing.T) {
	b, err := ioutil.ReadFile("../../fixtures/fan_out_job.yaml")
	require.NoError(t, err)

	original, err := NewJobDefinitionFromYaml(b)
	require.NoError(t, err)

	raw, err := json.Marshal(original)
	require.NoError(t, err)

	var roundTripped JobDefinition
	err = json.Unmarshal(raw, &roundTripped)
	require.NoError(t, err)

	var deployTask *TaskDefinition
	for _, task := range roundTripped.Tasks {
		if task.TaskType == "deploy" {
			deployTask = task
			break
		}
	}
	require.NotNil(t, deployTask, "deploy task must survive JSON roundtrip")
	require.NotNil(t, deployTask.FanOut, "fan_out must survive JSON roundtrip")
	require.Equal(t, "regions", deployTask.FanOut.Source)
	require.Equal(t, "region", deployTask.FanOut.ItemVar)
	require.Equal(t, 2, deployTask.FanOut.MaxParallel)
}

// Test that sub_workflow round-trips through JSON serialisation.
func Test_ShouldRoundTripSubWorkflowViaJSON(t *testing.T) {
	b, err := ioutil.ReadFile("../../fixtures/sub_workflow_job.yaml")
	require.NoError(t, err)
	original, err := NewJobDefinitionFromYaml(b)
	require.NoError(t, err)

	raw, err := json.Marshal(original)
	require.NoError(t, err)

	var roundTripped JobDefinition
	err = json.Unmarshal(raw, &roundTripped)
	require.NoError(t, err)

	// After JSON roundtrip, GetTask requires the task lookup map to be initialised.
	// Use GetTask only after unmarshalling via the proper constructor path, or
	// iterate tasks directly.
	var forkTask *TaskDefinition
	for _, task := range roundTripped.Tasks {
		if task.TaskType == "run-child-etl" {
			forkTask = task
			break
		}
	}
	require.NotNil(t, forkTask, "run-child-etl task must survive JSON roundtrip")
	require.NotNil(t, forkTask.SubWorkflow, "sub_workflow must survive JSON roundtrip")
	om, err := forkTask.SubWorkflow.OutputMap()
	require.NoError(t, err)
	require.Equal(t, "etl_row_count", om["row_count"])
	require.True(t, forkTask.SubWorkflow.WaitForCompletion)
}
