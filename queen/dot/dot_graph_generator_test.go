package dot

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"plexobject.com/formicary/internal/crypto"
	"testing"

	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/types"
)

var testEncryptedKey = crypto.SHA256Key("test-key")

func Test_ShouldCreateDotForTacoJob(t *testing.T) {
	// GIVEN job jobDefinition defined in yaml
	b, err := ioutil.ReadFile("../../docs/examples/taco-job.yaml")
	require.NoError(t, err)
	definition, err := types.NewJobDefinitionFromYaml(b)
	require.NoError(t, err)
	request, err := types.NewJobRequestFromDefinition(definition)
	require.NoError(t, err)

	// AND job-execution
	jobExec := types.NewJobExecution(request)
	jobExec.AddTask(definition.GetTask("allocate")).TaskState = common.COMPLETED
	checkTask := jobExec.AddTask(definition.GetTask("check-date"))
	checkTask.TaskState = common.FAILED
	checkTask.ExitCode = "1"
	jobExec.AddTask(definition.GetTask("monday")).TaskState = common.COMPLETED
	jobExec.AddTask(definition.GetTask("deallocate")).TaskState = common.COMPLETED
	jobExec.JobState = common.FAILED

	// WHEN job jobDefinition and execution is passed to generate dot config
	generator, err := New(definition, jobExec)
	require.NoError(t, err)
	dotConf, err := generator.GenerateDot()
	// THEN a valid dot config is created
	require.NoError(t, err)
	require.Contains(t, dotConf, `"start" -> "allocate"`)
	require.Contains(t, dotConf, `"check-date" -> "monday"`)
	require.Contains(t, dotConf, `"monday" -> "deallocate"`)
	require.Contains(t, dotConf, `"deallocate" -> "end"`)
	fmt.Printf("%s\n", dotConf)
}

func Test_ShouldCreateDotForBasicJobFromYAML(t *testing.T) {
	// GIVEN job jobDefinition defined in yaml
	b, err := ioutil.ReadFile("../../fixtures/basic-job.yaml")
	require.NoError(t, err)
	definition, err := types.NewJobDefinitionFromYaml(b)
	require.NoError(t, err)
	request, err := types.NewJobRequestFromDefinition(definition)
	require.NoError(t, err)

	// AND job-execution
	jobExec := types.NewJobExecution(request)
	jobExec.AddTask(definition.Tasks[0]).TaskState = common.FAILED
	jobExec.JobState = common.FAILED

	// WHEN job jobDefinition and execution is passed to generate dot config
	generator, err := New(definition, jobExec)
	require.NoError(t, err)
	dotConf, err := generator.GenerateDot()
	// THEN a valid dot config is created
	require.NoError(t, err)
	require.Contains(t, dotConf, `"start" -> "task1"`)
}

func Test_ShouldCreateDotForSimpleHappyJob(t *testing.T) {
	// GIVEN job jobDefinition and execution
	definition := newTestJob("test1", 10)
	exec := newTestJobExecution(definition)
	// WHEN job jobDefinition and execution is passed to generate dot config
	generator, err := New(definition, exec)
	// THEN a valid dot config is created
	require.NoError(t, err)
	dotConf, err := generator.GenerateDot()
	require.NoError(t, err)
	require.Contains(t, dotConf, `"start" -> "task1"`)
	require.Contains(t, dotConf, `"task1" -> "task2"`)
	require.Contains(t, dotConf, `"task2" -> "task3"`)
	require.Contains(t, dotConf, `"task9" -> "end"`)
}

func Test_ShouldCreateDotImageForSimpleHappyJob(t *testing.T) {
	// GIVEN job jobDefinition and execution
	definition := newTestJob("test1", 10)
	exec := newTestJobExecution(definition)
	// WHEN job jobDefinition and execution is passed to generate dot config
	generator, err := New(definition, exec)
	// THEN a valid dot config is created
	require.NoError(t, err)
	b, err := generator.GenerateDotImage()
	// THEN a valid dot config is created
	require.NoError(t, err)

	tmpFile, err := ioutil.TempFile(os.TempDir(), "dot-")
	require.NoError(t, err)

	// Remember to clean up the file afterwards
	defer func() {
		_ = os.Remove(tmpFile.Name())
	}()

	_, err = tmpFile.Write(b)
	if err != nil {
		_ = tmpFile.Close()
		t.Fatalf("Failed to writ temp file %v", err)
	}
	_ = tmpFile.Close()
}

func Test_ShouldCreateDotWithAlwaysRun(t *testing.T) {
	// GIVEN job jobDefinition and execution
	job := types.NewJobDefinition("test.test-run")

	job.AddTasks(
		types.NewTaskDefinition("allocate", common.Shell).
			AddExitCode("completed", "check").
			AddExitCode("failed", "failed-allocate"),
		types.NewTaskDefinition("failed-allocate", common.Shell).
			AddExitCode("completed", "deallocate"),
		types.NewTaskDefinition("check", common.Shell).
			AddExitCode("monday", "manic-monday").
			AddExitCode("tuesday", "taco-tuesday").
			AddExitCode("wednesday", "party"),
		types.NewTaskDefinition("manic-monday", common.Shell).
			AddExitCode("completed", "deallocate"),
		types.NewTaskDefinition("taco-tuesday", common.Shell).
			AddExitCode("completed", "party"),
		types.NewTaskDefinition("party", common.Shell).
			AddExitCode("completed", "deallocate"),
		types.NewTaskDefinition("deallocate", common.Shell).SetAlwaysRun(),
	)
	job.UpdateRawYaml()
	err := job.ValidateBeforeSave(testEncryptedKey)
	require.NoError(t, err)
	req, err := types.NewJobRequestFromDefinition(job)
	require.NoError(t, err)
	jobExec := types.NewJobExecution(req.ToInfo())
	jobExec.AddTask(job.GetTask("allocate")).SetStatus(common.COMPLETED)
	jobExec.AddTask(job.GetTask("check")).SetStatus("tuesday")
	jobExec.AddTask(job.GetTask("taco-tuesday")).SetStatus(common.COMPLETED)
	jobExec.AddTasks(job.Tasks[3])
	jobExec.JobState = common.COMPLETED

	// WHEN job jobDefinition and execution is passed to generate dot config
	generator, err := New(job, jobExec)
	// THEN a valid dot config is created
	require.NoError(t, err)
	dotConf, err := generator.GenerateDot()
	// THEN a valid dot config is created
	require.NoError(t, err)
	require.Contains(t, dotConf, `"start" -> "allocate"`)
	require.Contains(t, dotConf, `"allocate" -> "check"`)
	require.Contains(t, dotConf, `"check" -> "taco-tuesday"`)
	require.Contains(t, dotConf, `"taco-tuesday" -> "party"`)
}

func Test_ShouldCreateDotForCustomizedExitCode(t *testing.T) {
	// GIVEN job jobDefinition and execution
	definition := newTestJob("test1", 10)
	// AND configure first task with custom exit codes
	definition.GetTask("task1").OnExitCode["SKIP"] = "task3"

	cleanup := definition.AddTask(types.NewTaskDefinition("cleanup", common.Shell))
	// AND configure last task as always run
	definition.GetTask("task9").OnExitCode["completed"] = cleanup.TaskType
	cleanup.AlwaysRun = true

	exec := newTestJobExecution(definition)

	exec.GetTask("task8").TaskState = common.FAILED
	exec.GetTask("task9").TaskState = common.READY
	exec.JobState = common.FAILED

	// WHEN job jobDefinition and execution is passed to generate dot config
	generator, err := New(definition, exec)
	// THEN a valid dot config is created
	require.NoError(t, err)
	dotConf, err := generator.GenerateDot()

	// THEN a valid dot config is created
	require.NoError(t, err)
	require.Contains(t, dotConf, `"start" -> "task1"`)
	require.Contains(t, dotConf, `"task1" -> "task2"`)
	require.Contains(t, dotConf, `"task2" -> "task3"`)
	require.Contains(t, dotConf, `"task3" -> "task4"`)
}

func newTestJob(name string, max int) *types.JobDefinition {
	job := types.NewJobDefinition("io.formicary.test." + name)

	for i := 1; i < max; i++ {
		task := types.NewTaskDefinition(fmt.Sprintf("task%d", i), common.Shell)
		if i < max-1 {
			task.OnExitCode[common.COMPLETED] = fmt.Sprintf("task%d", i+1)
		}
		prefix := fmt.Sprintf("t%d", i)
		task.BeforeScript = []string{prefix + "_cmd1", prefix + "_cmd2", prefix + "_cmd3"}
		task.AfterScript = []string{prefix + "_cmd1", prefix + "_cmd2", prefix + "_cmd3"}
		task.Script = []string{prefix + "_cmd1", prefix + "_cmd2", prefix + "_cmd3"}
		task.Headers = map[string]string{prefix + "_h1": "1", prefix + "_h2": "true", prefix + "_h3": "three"}
		_, _ = task.AddVariable(prefix+"k1", "v1")
		task.Method = common.Docker
		if i%2 == 0 {
			task.AlwaysRun = true
		}
		job.AddTask(task)
	}
	job.UpdateRawYaml()
	return job
}

func newTestJobExecution(job *types.JobDefinition) *types.JobExecution {
	req, _ := types.NewJobRequestFromDefinition(job)

	jobExec := types.NewJobExecution(req.ToInfo())
	for _, t := range job.Tasks {
		task := jobExec.AddTask(t)
		task.TaskState = common.COMPLETED
	}
	jobExec.JobState = common.COMPLETED
	return jobExec
}
