package diagrams

import (
	"github.com/stretchr/testify/require"
	"os"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/types"
	"strings"
	"testing"
)

func Test_ShouldCreateMermaidForForkJob(t *testing.T) {
	// GIVEN job definition defined in yaml
	b, err := os.ReadFile("../../docs/examples/parallel-video-encoding.yaml")
	require.NoError(t, err)
	definition, err := types.NewJobDefinitionFromYaml(b)
	require.NoError(t, err)
	request, err := types.NewJobRequestFromDefinition(definition)
	require.NoError(t, err)

	// AND job-execution
	jobExec := types.NewJobExecution(request)
	jobExec.AddTask(definition.GetTask("validate")).TaskState = common.COMPLETED
	jobExec.AddTask(definition.GetTask("download")).TaskState = common.COMPLETED
	jobExec.AddTask(definition.GetTask("split")).TaskState = common.COMPLETED
	jobExec.AddTask(definition.GetTask("fork-encode1")).TaskState = common.COMPLETED
	jobExec.AddTask(definition.GetTask("fork-encode2")).TaskState = common.COMPLETED
	jobExec.AddTask(definition.GetTask("fork-await")).TaskState = common.EXECUTING
	jobExec.JobState = common.EXECUTING

	// WHEN job definition and execution is passed to generate mermaid config
	generator, err := NewMermaid(definition, nil)
	require.NoError(t, err)
	mermaidConf, err := generator.GenerateMermaid()

	// THEN a valid mermaid config is created
	require.NoError(t, err)
	require.Contains(t, mermaidConf, "flowchart TD")
	require.Contains(t, mermaidConf, "validate --> download")
	require.Contains(t, mermaidConf, "download --> split")
	require.Contains(t, mermaidConf, "split --> fork_encode1")
	require.Contains(t, mermaidConf, "fork_encode1 --> fork_encode2")
	require.Contains(t, mermaidConf, "fork_encode2 --> fork_await")
}

func Test_ShouldCreateMermaidForTacoJob(t *testing.T) {
	// GIVEN job definition defined in yaml
	b, err := os.ReadFile("../../docs/examples/taco-job.yaml")
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

	// WHEN job definition and execution is passed to generate mermaid config
	generator, err := NewMermaid(definition, jobExec)
	require.NoError(t, err)
	mermaidConf, err := generator.GenerateMermaid()

	// THEN a valid mermaid config is created
	require.NoError(t, err)
	require.Contains(t, mermaidConf, "flowchart TD")
	require.Contains(t, mermaidConf, "START --> allocate")
	require.Contains(t, mermaidConf, "check_date ==> monday") // Bold arrow for executed path
	require.Contains(t, mermaidConf, "monday ==> deallocate")
	require.Contains(t, mermaidConf, "deallocate --> END")
	require.Contains(t, mermaidConf, "class END class_error") // Failed job should have error styling
}

func Test_ShouldCreateMermaidForBasicJobFromYAML(t *testing.T) {
	// GIVEN job definition defined in yaml
	b, err := os.ReadFile("../../fixtures/basic-job.yaml")
	require.NoError(t, err)
	definition, err := types.NewJobDefinitionFromYaml(b)
	require.NoError(t, err)
	request, err := types.NewJobRequestFromDefinition(definition)
	require.NoError(t, err)

	// AND job-execution
	jobExec := types.NewJobExecution(request)
	jobExec.AddTask(definition.Tasks[0]).TaskState = common.FAILED
	jobExec.JobState = common.FAILED

	// WHEN job definition and execution is passed to generate mermaid config
	generator, err := NewMermaid(definition, jobExec)
	require.NoError(t, err)
	mermaidConf, err := generator.GenerateMermaid()

	// THEN a valid mermaid config is created
	require.NoError(t, err)
	require.Contains(t, mermaidConf, "flowchart TD")
	require.Contains(t, mermaidConf, "START --> task1")
}

func Test_ShouldCreateMermaidForSimpleHappyJob(t *testing.T) {
	// GIVEN job definition and execution
	definition := newTestJob("test1", 10)
	exec := newTestJobExecution(definition)

	// WHEN job definition and execution is passed to generate mermaid config
	generator, err := NewMermaid(definition, exec)
	require.NoError(t, err)
	mermaidConf, err := generator.GenerateMermaid()

	// THEN a valid mermaid config is created
	require.NoError(t, err)
	require.Contains(t, mermaidConf, "flowchart TD")
	require.Contains(t, mermaidConf, "START --> task1")
	require.Contains(t, mermaidConf, "task1 ==> task2") // Bold arrows for executed paths
	require.Contains(t, mermaidConf, "task2 ==> task3")
	require.Contains(t, mermaidConf, "task9 --> END")
	require.Contains(t, mermaidConf, "class START class_primary")
	require.Contains(t, mermaidConf, "class END class_success")
	require.Contains(t, mermaidConf, "classDef class_success") // CSS class definitions
}

func Test_ShouldCreateMermaidWithAlwaysRun(t *testing.T) {
	// GIVEN job definition and execution
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

	// WHEN job definition and execution is passed to generate mermaid config
	generator, err := NewMermaid(job, jobExec)
	require.NoError(t, err)
	mermaidConf, err := generator.GenerateMermaid()

	// THEN a valid mermaid config is created
	require.NoError(t, err)
	require.Contains(t, mermaidConf, "flowchart TD")
	require.Contains(t, mermaidConf, "START --> allocate")
	require.Contains(t, mermaidConf, "allocate ==> check")
	require.Contains(t, mermaidConf, "check ==> taco_tuesday")
	require.Contains(t, mermaidConf, "taco_tuesday ==> party")

	// Check that always-run task uses trapezoid shape
	require.Contains(t, mermaidConf, `deallocate[/"deallocate"\]`)
}

func Test_ShouldCreateMermaidForCustomizedExitCode(t *testing.T) {
	// GIVEN job definition and execution
	definition := newTestJob("test1", 10)
	// AND configure first task with custom exit codes
	definition.GetTask("task1").OnExitCode["SKIP"] = "task3"

	cleanup := definition.AddTask(types.NewTaskDefinition("cleanup", common.Shell))
	// AND configure last task as always run
	definition.GetTask("task9").OnExitCode["completed"] = cleanup.TaskType
	cleanup.AlwaysRun = true

	exec := newTestJobExecution(definition)

	_, task8 := exec.GetTask("", "task8")
	task8.TaskState = common.FAILED
	_, task9 := exec.GetTask("", "task9")
	task9.TaskState = common.READY
	exec.JobState = common.FAILED

	// WHEN job definition and execution is passed to generate mermaid config
	generator, err := NewMermaid(definition, exec)
	require.NoError(t, err)
	mermaidConf, err := generator.GenerateMermaid()

	// THEN a valid mermaid config is created
	require.NoError(t, err)
	require.Contains(t, mermaidConf, "flowchart TD")
	require.Contains(t, mermaidConf, "START --> task1")
	require.Contains(t, mermaidConf, "task1 ==> task2")
	require.Contains(t, mermaidConf, "task2 ==> task3")
	require.Contains(t, mermaidConf, "task3 ==> task4")

	// Check that cleanup task uses trapezoid shape for always-run
	require.Contains(t, mermaidConf, `cleanup[/"cleanup"\]`)
}

func Test_ShouldCreateMermaidForManualApprovalTask(t *testing.T) {
	// GIVEN job definition with manual approval task
	definition := types.NewJobDefinition("test.manual-approval")
	definition.AddTasks(
		types.NewTaskDefinition("build", common.Shell).
			AddExitCode("completed", "approve"),
		types.NewTaskDefinition("approve", common.Manual).
			AddExitCode("approved", "deploy").
			AddExitCode("rejected", "cleanup"),
		types.NewTaskDefinition("deploy", common.Shell).
			AddExitCode("completed", "cleanup"),
		types.NewTaskDefinition("cleanup", common.Shell).SetAlwaysRun(),
	)
	definition.RawYaml = "test"

	req, err := types.NewJobRequestFromDefinition(definition)
	require.NoError(t, err)

	jobExec := types.NewJobExecution(req.ToInfo())
	jobExec.AddTask(definition.GetTask("build")).TaskState = common.COMPLETED
	approveTask := jobExec.AddTask(definition.GetTask("approve"))
	approveTask.TaskState = common.MANUAL_APPROVAL_REQUIRED
	jobExec.JobState = common.EXECUTING

	// WHEN job definition and execution is passed to generate mermaid config
	generator, err := NewMermaid(definition, jobExec)
	require.NoError(t, err)
	mermaidConf, err := generator.GenerateMermaid()

	// THEN a valid mermaid config is created
	require.NoError(t, err)
	require.Contains(t, mermaidConf, "flowchart TD")
	require.Contains(t, mermaidConf, "build ==> approve")

	// Check manual approval task has lock icon and proper styling
	require.Contains(t, mermaidConf, `approve["approve`)         // Manual tasks get special shape
	require.Contains(t, mermaidConf, "🔒")                        // Lock icon for manual approval
	require.Contains(t, mermaidConf, "class approve class_info") // Info styling for manual approval
}

func Test_ShouldCreateMermaidForForkAndAwaitJobs(t *testing.T) {
	// GIVEN job definition with fork and await
	definition := types.NewJobDefinition("test.fork-await")
	definition.AddTasks(
		types.NewTaskDefinition("setup", common.Shell).
			AddExitCode("completed", "fork-parallel"),
		types.NewTaskDefinition("fork-parallel", common.ForkJob).
			AddExitCode("completed", "await-parallel"),
		types.NewTaskDefinition("await-parallel", common.AwaitForkedJob).
			AddExitCode("completed", "finalize"),
		types.NewTaskDefinition("finalize", common.Shell),
	)
	definition.RawYaml = "test"

	req, err := types.NewJobRequestFromDefinition(definition)
	require.NoError(t, err)

	jobExec := types.NewJobExecution(req.ToInfo())
	jobExec.AddTask(definition.GetTask("setup")).TaskState = common.COMPLETED
	jobExec.AddTask(definition.GetTask("fork-parallel")).TaskState = common.COMPLETED
	jobExec.AddTask(definition.GetTask("await-parallel")).TaskState = common.EXECUTING
	jobExec.JobState = common.EXECUTING

	// WHEN job definition and execution is passed to generate mermaid config
	generator, err := NewMermaid(definition, jobExec)
	require.NoError(t, err)
	mermaidConf, err := generator.GenerateMermaid()

	// THEN a valid mermaid config is created
	require.NoError(t, err)
	require.Contains(t, mermaidConf, "flowchart TD")
	require.Contains(t, mermaidConf, "setup ==> fork_parallel")
	require.Contains(t, mermaidConf, "fork_parallel ==> await_parallel")

	// Check await job uses trapezoid shape
	require.Contains(t, mermaidConf, `await_parallel[/"await-parallel"\]`)
}

func Test_ShouldCreateMermaidWithDecisionNodes(t *testing.T) {
	// GIVEN job definition with decision-like task (multiple exit codes)
	definition := types.NewJobDefinition("test.decision")
	definition.AddTasks(
		types.NewTaskDefinition("check-env", common.Shell).
			AddExitCode("dev", "deploy-dev").
			AddExitCode("staging", "deploy-staging").
			AddExitCode("prod", "deploy-prod"),
		types.NewTaskDefinition("deploy-dev", common.Shell),
		types.NewTaskDefinition("deploy-staging", common.Shell),
		types.NewTaskDefinition("deploy-prod", common.Shell),
	)
	definition.RawYaml = "test"

	req, err := types.NewJobRequestFromDefinition(definition)
	require.NoError(t, err)

	jobExec := types.NewJobExecution(req.ToInfo())
	checkTask := jobExec.AddTask(definition.GetTask("check-env"))
	checkTask.TaskState = common.COMPLETED
	checkTask.ExitCode = "staging"
	jobExec.AddTask(definition.GetTask("deploy-staging")).TaskState = common.COMPLETED
	jobExec.JobState = common.COMPLETED

	// WHEN job definition and execution is passed to generate mermaid config
	generator, err := NewMermaid(definition, jobExec)
	require.NoError(t, err)
	mermaidConf, err := generator.GenerateMermaid()

	// THEN a valid mermaid config is created
	require.NoError(t, err)
	require.Contains(t, mermaidConf, "flowchart TD")
	require.Contains(t, mermaidConf, "check_env ==> deploy_staging") // Executed path is bold
	require.Contains(t, mermaidConf, "check_env -.-> deploy_dev")    // Potential paths are dotted
	require.Contains(t, mermaidConf, "check_env -.-> deploy_prod")
}

func Test_ShouldValidateMermaidSyntax(t *testing.T) {
	// GIVEN simple job definition
	definition := newTestJob("syntax-test", 5)
	exec := newTestJobExecution(definition)

	// WHEN mermaid config is generated
	generator, err := NewMermaid(definition, exec)
	require.NoError(t, err)
	mermaidConf, err := generator.GenerateMermaid()
	require.NoError(t, err)

	// THEN the config should have valid mermaid syntax
	lines := strings.Split(mermaidConf, "\n")

	// Should start with flowchart declaration
	require.True(t, strings.HasPrefix(lines[0], "flowchart TD"))

	// Should contain valid node definitions
	nodeDefCount := 0
	connectionCount := 0
	classDefCount := 0
	classAssignCount := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "flowchart") {
			continue
		}

		if strings.Contains(line, "[") && strings.Contains(line, "]") {
			nodeDefCount++
		} else if strings.Contains(line, "-->") || strings.Contains(line, "==>") || strings.Contains(line, "-.->") {
			connectionCount++
		} else if strings.HasPrefix(line, "classDef") {
			classDefCount++
		} else if strings.HasPrefix(line, "class ") {
			classAssignCount++
		}
	}

	// Should have reasonable number of each element
	require.Greater(t, nodeDefCount, 0, "Should have node definitions")
	require.Greater(t, connectionCount, 0, "Should have connections")
	require.Greater(t, classDefCount, 0, "Should have class definitions")
	require.Greater(t, classAssignCount, 0, "Should have class assignments")
}
