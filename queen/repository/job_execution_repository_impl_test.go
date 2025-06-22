package repository

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"math/rand"
	"sync"
	"testing"
	"time"

	common "plexobject.com/formicary/internal/types"

	"plexobject.com/formicary/queen/types"
)

// Get operation should fail if job-execution doesn't exist
func Test_ShouldNotGetJobExecutionWithNonExistingId(t *testing.T) {
	repo, err := NewTestJobExecutionRepository()
	require.NoError(t, err)
	// WHEN finding non-existing id
	_, err = repo.Get("missing_id")
	// THEN it should fail
	require.Error(t, err)
}

// Saving job-execution without job-type should fail
func Test_ShouldNotSaveJobExecutionWithoutJobType(t *testing.T) {
	repo, err := NewTestJobExecutionRepository()
	require.NoError(t, err)
	req, err := types.NewJobRequestFromDefinition(types.NewJobDefinition(""))
	require.NoError(t, err)
	job := types.NewJobExecution(req.ToInfo())
	// WHEN saving job without job-type
	_, err = repo.Save(job)
	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "jobType is not specified")
}

// Updates state of job execution
func Test_ShouldUpdateStateOfJobExecution(t *testing.T) {
	// GIVEN repositories
	jobRequestRepository, err := NewTestJobRequestRepository()
	require.NoError(t, err)
	jobExecutionRepository, err := NewTestJobExecutionRepository()
	require.NoError(t, err)
	qc, err := NewTestQC()
	require.NoError(t, err)

	// WHEN creating jobExec-execution
	_, jobExec, err := NewTestJobExecution(qc, "valid-jobExec-without-config")
	require.NoError(t, err)

	// THEN should be able to save valid jobExec
	savedJobExec, err := jobExecutionRepository.Save(jobExec)
	require.NoError(t, err)

	// AND setting status to EXECUTING for job-request should succeed
	_ = jobRequestRepository.SetReadyToExecute(
		jobExec.JobRequestID, savedJobExec.ID, "")
	_, _ = jobExec.AddContext("jk8", 88)

	// AND setting status to EXECUTING for job-execution should succeed
	err = jobExecutionRepository.UpdateJobRequestAndExecutionState(
		savedJobExec.ID, common.READY, common.EXECUTING)
	require.NoError(t, err)

	// AND changing job-context should succeed
	err = jobExecutionRepository.UpdateJobContext(savedJobExec.ID, jobExec.Contexts)
	require.NoError(t, err)

	// WHEN loading saved job-execution
	loaded, err := jobExecutionRepository.Get(jobExec.ID)
	require.NoError(t, err)

	// THEN job-context should match
	require.Equal(t, jobExec.GetContext("jk8").Value, loaded.GetContext("jk8").Value)

	// WHEN changing context again and setting status to finalized
	_, _ = jobExec.AddContext("jk9", 99)
	err = jobExecutionRepository.FinalizeJobRequestAndExecutionState(
		savedJobExec.ID, common.EXECUTING, common.FAILED, "failed", "", 10, 100, 1)
	require.NoError(t, err)
	err = jobExecutionRepository.UpdateJobContext(savedJobExec.ID, jobExec.Contexts)
	require.NoError(t, err)

	// THEN loaded job-execution should match context
	loaded, err = jobExecutionRepository.Get(jobExec.ID)
	require.NoError(t, err)
	require.Equal(t, jobExec.GetContext("jk9").Value, loaded.GetContext("jk9").Value)

	// Cannot change state from terminal
	err = jobExecutionRepository.FinalizeJobRequestAndExecutionState(
		savedJobExec.ID, common.FAILED, common.PENDING, "failed", "", 10, 0, 0)
	// should fail to change state from terminal
	require.Error(t, err)

	// should fail setting status after terminal state
	err = jobExecutionRepository.UpdateJobRequestAndExecutionState(
		savedJobExec.ID, common.READY, common.EXECUTING)
	require.Error(t, err)

	loaded, err = jobExecutionRepository.Get(jobExec.ID)
	require.NoError(t, err)
	require.Equal(t, 5, len(loaded.Contexts))
}

// Saving valid job-execution without config should succeed
func Test_ShouldSaveValidJobExecutionWithoutContext(t *testing.T) {
	// GIVEN repositories
	repo, err := NewTestJobExecutionRepository()
	require.NoError(t, err)
	qc, err := NewTestQC()
	require.NoError(t, err)

	// WHEN creating a job-execution without context
	_, jobExec, err := NewTestJobExecution(qc, "valid-jobExec-without-context")
	require.NoError(t, err)

	// THEN Saving valid jobExec should succeed
	savedExec, err := repo.Save(jobExec)
	require.NoError(t, err)
	err = saveTestArtifacts(qc.User, savedExec)
	require.NoError(t, err)

	// Retrieving jobExec by id
	loaded, err := repo.Get(savedExec.ID)
	require.NoError(t, err)

	// Comparing savedExec object
	require.NoError(t, loaded.Equals(jobExec))

	for _, next := range loaded.Tasks {
		require.NotNil(t, next.Artifacts)
		require.Equal(t, 5, len(next.Artifacts))
	}

	savedExec.JobState = common.FAILED
	savedExec, err = repo.Save(savedExec)
	require.NoError(t, err)

	loaded, err = repo.Get(savedExec.ID)
	require.NoError(t, err)
	require.Equal(t, common.FAILED, loaded.JobState)
	require.Equal(t, int64(900), loaded.CPUSecs)

	// Should not be able to reset job
	err = repo.ResetStateToReady(savedExec.ID)
	require.Error(t, err)

	err = repo.FinalizeJobRequestAndExecutionState(
		savedExec.ID,
		common.PENDING,
		common.COMPLETED,
		"",
		"",
		10,
		100,
		0)
	require.Error(t, err)
}

// Saving a job with context should succeed
func Test_ShouldSaveValidJobExecutionWithContext(t *testing.T) {
	// GIVEN repositories
	repo, err := NewTestJobExecutionRepository()
	require.NoError(t, err)
	qc, err := NewTestQC()
	require.NoError(t, err)

	// Creating a job-execution
	// WHEN creating a job-execution with context
	_, jobExec, err := NewTestJobExecution(qc, "valid-job-with-context")
	require.NoError(t, err)
	_, _ = jobExec.AddContext("jk1", "jv1")
	_, _ = jobExec.AddContext("jk2", map[string]int{"a": 1, "b": 2})

	// THEN Saving job-execution should succeed
	saved, err := repo.Save(jobExec)
	require.NoError(t, err)

	// AND retrieving job-execution by id should succeed
	loaded, err := repo.Get(saved.ID)
	require.NoError(t, err)
	require.NoError(t, loaded.Equals(jobExec))
}

// Test saving task execution concurrently for deadlock detection
func Test_ShouldSaveTaskExecutionConcurrently(t *testing.T) {
	// GIVEN repositories
	repo, err := NewTestJobExecutionRepository()
	require.NoError(t, err)
	qc, err := NewTestQC()
	require.NoError(t, err)

	var wg sync.WaitGroup
	var lock sync.RWMutex
	errors := make([]string, 0)
	// WHEN creating a job-execution in different go-routines concurrently
	for i := 0; i < 10; i++ {
		var jobExec *types.JobExecution
		lock.Lock()
		_, jobExec, err = NewTestJobExecution(qc, "valid-job-with-config")
		jobExec, err = repo.Save(jobExec)
		lock.Unlock()
		require.NoError(t, err)
		for j := 0; j < len(jobExec.Tasks); j++ {
			wg.Add(1)
			go func(task *types.TaskExecution) {
				lock.Lock()
				defer lock.Unlock()
				defer wg.Done()
				_, _ = task.AddContext("nk1", rand.Int())
				task.TaskState = common.EXECUTING
				_, err := repo.SaveTask(task)
				if err != nil {
					errors = append(errors, fmt.Sprintf("unexpected error %v while saving %v", err, task.String()))
				}
				_, _ = task.AddContext("nk2", rand.Int())
				task.TaskState = common.FAILED
				_, err = repo.SaveTask(task)
				if err != nil {
					errors = append(errors, fmt.Sprintf("unexpected error %v while saving %v", err, task.String()))
				}
			}(jobExec.Tasks[j])
		}
	}
	wg.Wait()
	// THEN no errors should be raised
	for _, err := range errors {
		t.Fatalf(err)
	}
}

// Saving task execution with retries
func Test_ShouldAddTaskExecutionToJobExecutionWithRetries(t *testing.T) {
	// GIVEN repositories
	repo, err := NewTestJobExecutionRepository()
	require.NoError(t, err)
	qc, err := NewTestQC()
	require.NoError(t, err)

	job, err := SaveTestJobDefinition(qc, "valid-job-with-config", "")
	require.NoError(t, err)

	// WHEN creating a job-execution
	now := time.Now()
	_, jobExec, err := NewTestJobExecution(qc, job.JobType)
	require.NoError(t, err)
	jobExec.StartedAt = now
	jobExec.EndedAt = &now

	for _, task := range jobExec.Tasks {
		task.StartedAt = time.Now().Add(-10 * time.Second)
		task.EndedAt = &now
		task.TaskState = common.COMPLETED
		require.Equal(t, int64(9), task.ExecutionCostSecs())
	}

	// THEN saving job-execution should succeed
	savedExec, err := repo.Save(jobExec)
	require.NoError(t, err)
	// Retrieving job-execution by id
	loaded, err := repo.Get(savedExec.ID)
	require.NoError(t, err)
	require.NoError(t, loaded.Equals(jobExec))
	require.Equal(t, len(jobExec.Tasks), len(loaded.Tasks))
	require.Equal(t, int64(81), loaded.ExecutionCostSecs())
	{
		_, oldTask := jobExec.GetTask("", job.Tasks[0].TaskType)
		previousExecutionCostSecs := oldTask.ExecutionCostSecs()
		loaded.DeleteTask(oldTask.ID)
		require.NoError(t, repo.DeleteTask(oldTask.ID))
		newTask := loaded.AddTask(job.Tasks[0])
		newTask.AddPreviousExecutionCostSecs(oldTask.ID, previousExecutionCostSecs)
		newTask.StartedAt = time.Now().Add(-15 * time.Second)
		newTask.EndedAt = &now
		newTask.TaskState = common.COMPLETED
		require.Equal(t, int64(9), newTask.GetPreviousExecutionCostSecs())
		_, err = repo.SaveTask(newTask)
		require.NoError(t, err)
	}

	loaded, err = repo.Get(savedExec.ID)
	require.NoError(t, err)
	_, loadedTask := loaded.GetTask("", job.Tasks[0].TaskType)
	require.NotNil(t, loadedTask)
	require.Equal(t, int64(9), loadedTask.GetPreviousExecutionCostSecs())
	require.Equal(t, int64(9), loadedTask.ExecutionCostSecs())
	require.Equal(t, int64(81), loaded.ExecutionCostSecs())
}

// Saving task execution separately after creating job execution
func Test_ShouldAddTaskExecutionToJobExecution(t *testing.T) {
	// GIVEN repositories
	repo, err := NewTestJobExecutionRepository()
	require.NoError(t, err)
	qc, err := NewTestQC()
	require.NoError(t, err)

	// WHEN creating a job-execution
	_, jobExec, err := NewTestJobExecution(qc, "valid-job-with-config")
	require.NoError(t, err)
	_, _ = jobExec.AddContext("jk1", "jv1")
	_, _ = jobExec.AddContext("jk2", map[string]int{"a": 1, "b": 2})

	// THEN saving job-execution should succeed
	savedExec, err := repo.Save(jobExec)
	require.NoError(t, err)

	// Retrieving job-execution by id
	loaded, err := repo.Get(savedExec.ID)
	require.NoError(t, err)
	require.NoError(t, loaded.Equals(jobExec))
	require.Equal(t, len(jobExec.Tasks), len(loaded.Tasks))
	now := time.Now()
	// WHEN creating a new task for the job-execution
	task := loaded.AddTask(types.NewTaskDefinition("new_task", common.Shell))
	task.StartedAt = time.Now().Add(-10 * time.Second)
	task.EndedAt = &now
	_, _ = task.AddContext("nk1", "new-task")
	_, _ = task.AddContext("nk2", []int{1, 2, 3})

	// THEN saving task should succeed
	_, err = repo.SaveTask(task)
	require.NoError(t, err)
	loaded, err = repo.Get(savedExec.ID)
	require.NoError(t, err)
	require.Equal(t, len(jobExec.Tasks)+1, len(loaded.Tasks))
	require.Equal(t, int64(0), loaded.CPUSecs)

	_, loadedTask := loaded.GetTask("", "new_task")
	require.NotNil(t, loadedTask)

	require.Equal(t, common.RequestState("READY"), loadedTask.TaskState)
	require.Equal(t, 2, len(loadedTask.Contexts))
	require.Equal(t, "new-task", loadedTask.GetContext("nk1").Value)

	// WHEN updating task again with different state and context
	loadedTask.TaskState = common.FAILED
	loadedTask.ExitCode = "101"
	loadedTask.DeleteContext("nk1")
	_, _ = loadedTask.AddContext("nk3", "final")
	_, _ = loadedTask.AddContext("nk4", 10)
	_, err = repo.SaveTask(loadedTask)
	require.NoError(t, err)

	// THEN job-execution after loading should have the context
	loaded, err = repo.Get(savedExec.ID)
	require.NoError(t, err)
	require.Equal(t, len(jobExec.Tasks)+1, len(loaded.Tasks))

	// AND should find new task
	_, loadedTask = loaded.GetTask(loadedTask.ID, loadedTask.TaskType)
	require.NotNil(t, loadedTask)

	require.Equal(t, common.RequestState("FAILED"), loadedTask.TaskState)
	require.Equal(t, 3, len(loadedTask.Contexts))
	require.Equal(t, "final", loadedTask.GetContext("nk3").Value)

	// WHEN changing state from terminal state
	loadedTask.TaskState = common.PAUSED

	_, err = repo.SaveTask(loadedTask)

	// THEN it should not fail
	require.NoError(t, err)

	// WHEN changing state from terminal state
	loadedTask.TaskState = common.PAUSED

	_, err = repo.SaveTask(loadedTask)

	// THEN it should fail -- TODO fix test - should fail
	require.NoError(t, err)
}

// Updating task execution state
func Test_ShouldUpdateStateForTaskExecution(t *testing.T) {
	// GIVEN repositories
	repo, err := NewTestJobExecutionRepository()
	require.NoError(t, err)
	qc, err := NewTestQC()
	require.NoError(t, err)

	// WHEN Creating a job-execution
	_, jobExec, err := NewTestJobExecution(qc, "valid-job-with-config")
	require.NoError(t, err)
	_, _ = jobExec.AddContext("jk1", "jv1")
	_, _ = jobExec.AddContext("jk2", map[string]int{"a": 1, "b": 2})

	// AND saving job-execution
	savedExec, err := repo.Save(jobExec)
	require.NoError(t, err)

	// THEN state of job-execution after loading by id should match the saved state
	loaded, err := repo.Get(savedExec.ID)
	require.NoError(t, err)
	require.NoError(t, loaded.Equals(jobExec))
	require.Equal(t, len(jobExec.Tasks), len(loaded.Tasks))

	// WHEN changing state of task from READY to FAILED
	err = repo.UpdateTaskState(loaded.Tasks[0].ID, common.READY, common.FAILED)
	// THEN it should succeed
	require.NoError(t, err)

	// BUT changing from terminal FAILED to PENDING should not succeed
	err = repo.UpdateTaskState(loaded.Tasks[0].ID, common.FAILED, common.PENDING)
	require.Error(t, err)
}

// Test querying job-execution by job-type
func Test_ShouldQueryJobExecutionQueryByJobType(t *testing.T) {
	// GIVEN repositories
	repo, err := NewTestJobExecutionRepository()
	require.NoError(t, err)
	repo.clear()
	qc, err := NewTestQC()
	require.NoError(t, err)
	jobExecs := make([]*types.JobExecution, 10)
	// AND creating a set of job-executions
	for i := 0; i < 10; i++ {
		_, exec, err := NewTestJobExecution(qc, fmt.Sprintf("query-job-%v", i))
		require.NoError(t, err)
		savedExec, err := repo.Save(exec)
		require.NoError(t, err)
		err = saveTestArtifacts(qc.User, savedExec)
		require.NoError(t, err)
		jobExecs[i] = exec
	}

	// WHEN querying job-executions without filters
	params := make(map[string]interface{})
	_, total, err := repo.Query(params, 0, 100, []string{"job_type desc"})
	// THEN it should return expected totals
	require.NoError(t, err)
	require.Equal(t, int64(10), total)

	// WHEN querying job-executions by job-type
	params["job_type"] = jobExecs[0].JobType
	res, total, err := repo.Query(params, 0, 100, []string{"job_type desc"})
	// THEN it should return only one record
	require.NoError(t, err)
	require.Equal(t, 1, len(res))
	require.Equal(t, int64(1), total)
	require.NoError(t, res[0].Equals(jobExecs[0]))

	for _, job := range res {
		for _, next := range job.Tasks {
			require.Nil(t, next.Artifacts, next)
		}
	}
}

// Test GetResourceUsage usage by org/user
func Test_ShouldJobExecutionAccountingByOrgUser(t *testing.T) {
	// GIVEN repositories
	jobRequestRepository, err := NewTestJobRequestRepository()
	require.NoError(t, err)
	jobExecutionRepository, err := NewTestJobExecutionRepository()
	require.NoError(t, err)
	jobExecutionRepository.clear()
	qc, err := NewTestQC()
	require.NoError(t, err)

	jobs := make([]*types.JobExecution, 10)
	// AND creating a set of job-executions
	for i := 0; i < 10; i++ {
		_, jobExec, err := NewTestJobExecution(qc, fmt.Sprintf("jobExec-exec-account-%v", i))
		require.NoError(t, err)
		jobExec.StartedAt = time.Now().Add(-10 * time.Second)
		saved, err := jobExecutionRepository.Save(jobExec)
		require.NoError(t, err)
		err = jobRequestRepository.SetReadyToExecute(saved.JobRequestID, saved.ID, "")
		require.NoError(t, err)
		jobs[i] = saved
		err = jobExecutionRepository.FinalizeJobRequestAndExecutionState(
			saved.ID,
			saved.JobState,
			common.COMPLETED,
			"",
			"",
			10,
			100,
			0)
		require.NoError(t, err)
	}
	// WHEN querying getting usage with nil range
	usage, err := jobExecutionRepository.GetResourceUsageByOrgUser(nil, 10000)
	// THEN no errors and zero result should return
	require.NoError(t, err)
	require.Equal(t, 0, len(usage))
	// WHEN querying getting usage with full range
	usage, err = jobExecutionRepository.GetResourceUsageByOrgUser([]types.DateRange{{
		StartDate: time.Now().Add(-1 * time.Minute),
		EndDate:   time.Now().Add(1 * time.Minute),
	}}, 10000)
	// THEN no errors and zero result should return
	require.NoError(t, err)
	require.Equal(t, 1, len(usage))
	require.Equal(t, 10, usage[0].Count)
	require.Equal(t, int64(100), usage[0].Value)
}

// Test GetResourceUsage usage
func Test_ShouldJobExecutionAccounting(t *testing.T) {
	// GIVEN repositories
	jobRequestRepository, err := NewTestJobRequestRepository()
	require.NoError(t, err)
	jobExecutionRepository, err := NewTestJobExecutionRepository()
	require.NoError(t, err)
	jobExecutionRepository.clear()
	qc, err := NewTestQC()
	require.NoError(t, err)

	jobsExecs := make([]*types.JobExecution, 10)
	// AND creating a set of job-executions
	for i := 0; i < 10; i++ {
		_, jobExec, err := NewTestJobExecution(qc, fmt.Sprintf("jobExec-exec-account-%v", i))
		require.NoError(t, err)
		jobExec.StartedAt = time.Now().Add(-10 * time.Second)
		saved, err := jobExecutionRepository.Save(jobExec)
		require.NoError(t, err)
		err = jobRequestRepository.SetReadyToExecute(saved.JobRequestID, saved.ID, "")
		require.NoError(t, err)
		jobsExecs[i] = saved
		err = jobExecutionRepository.FinalizeJobRequestAndExecutionState(
			saved.ID,
			saved.JobState,
			common.COMPLETED,
			"",
			"",
			10,
			100,
			0)
		require.NoError(t, err)
	}
	// WHEN querying getting usage with nil range
	usage, err := jobExecutionRepository.GetResourceUsage(qc, nil)
	// THEN no errors and zero result should return
	require.NoError(t, err)
	require.Equal(t, 0, len(usage))
	// WHEN querying getting usage with full range
	usage, err = jobExecutionRepository.GetResourceUsage(qc, []types.DateRange{{
		StartDate: time.Now().Add(-1 * time.Minute),
		EndDate:   time.Now().Add(1 * time.Minute),
	}})
	// THEN no errors and zero result should return
	require.NoError(t, err)
	require.Equal(t, 1, len(usage))
	require.Equal(t, 10, usage[0].Count)
	require.Equal(t, int64(100), usage[0].Value)
}

// Test Query with different operators
func Test_ShouldJobExecutionQueryWithDifferentOperators(t *testing.T) {
	// GIVEN repositories
	repo, err := NewTestJobExecutionRepository()
	require.NoError(t, err)
	repo.clear()
	qc, err := NewTestQC()
	require.NoError(t, err)

	jobExecs := make([]*types.JobExecution, 10)
	// AND creating a set of job-executions
	for i := 0; i < 10; i++ {
		_, jobExec, err := NewTestJobExecution(qc, fmt.Sprintf("job-exec-query-operator-%v", i))
		require.NoError(t, err)
		saved, err := repo.Save(jobExec)
		require.NoError(t, err)
		jobExecs[i] = saved
	}

	// WHEN querying using LIKE
	params := make(map[string]interface{})
	params["job_type:like"] = "job-exec-query-operator"
	_, total, err := repo.Query(params, 0, 100, []string{"job_type desc"})

	// THEN it should return all records
	require.NoError(t, err)
	require.Equal(t, int64(10), total)

	// WHEN querying using IN operator
	params = make(map[string]interface{})
	params["job_type:in"] = jobExecs[0].JobType + "," + jobExecs[1].JobType
	_, total, err = repo.Query(params, 0, 100, []string{"job_type desc"})
	// THEN it should return 2 records
	require.NoError(t, err)
	require.Equal(t, int64(2), total)

	// WHEN querying using exact operator
	params = make(map[string]interface{})
	params["job_type:="] = jobExecs[0].JobType
	_, total, err = repo.Query(params, 0, 100, []string{"job_type desc"})
	// THEN it should return 1 record
	require.NoError(t, err)
	require.Equal(t, int64(1), total)

	// WHEN querying using not equal operator
	params = make(map[string]interface{})
	params["job_type:!="] = jobExecs[0].JobType
	_, total, err = repo.Query(params, 0, 100, []string{"job_type desc"})
	// THEN it should return 9 records
	require.NoError(t, err)
	require.Equal(t, int64(9), total)

	// WHEN querying using GreaterThan operator
	params = make(map[string]interface{})
	params["job_type:>"] = jobExecs[0].JobType
	_, total, err = repo.Query(params, 0, 100, []string{"job_type desc"})
	// THEN it should return 9 records
	require.NoError(t, err)
	require.Equal(t, int64(9), total)

	// WHEN querying using LessThan operator
	params = make(map[string]interface{})
	params["job_type:<"] = jobExecs[9].JobType
	_, total, err = repo.Query(params, 0, 100, []string{"job_type desc"})
	// THEN it should return 9 records
	require.NoError(t, err)
	require.Equal(t, int64(9), total)
}

// Updating a job execution should succeed
func Test_ShouldUpdateValidJobExecution(t *testing.T) {
	// GIVEN repositories
	repo, err := NewTestJobExecutionRepository()
	require.NoError(t, err)
	repo.clear()
	qc, err := NewTestQC()
	require.NoError(t, err)

	_, jobExec, err := NewTestJobExecution(qc, "test-jobExec-for-update")
	require.NoError(t, err)

	// AND previously savedExec jobExec execution
	savedExec, err := repo.Save(jobExec)
	require.NoError(t, err)

	// WHEN retrieving jobExec by id
	loaded, err := repo.Get(savedExec.ID)
	require.NoError(t, err)
	// THEN it should be equal to the saved execution
	require.NoError(t, loaded.Equals(jobExec))

	// WHEN updating context
	jobExec.DeleteContext("jk1")
	jobExec.Tasks[0].TaskType = "new_task1"
	_, _ = jobExec.Tasks[1].AddContext("new_tk", false)
	_ = jobExec.Tasks[2].DeleteContext(jobExec.Tasks[2].Contexts[0].Name)

	// AND saving jobExec
	savedExec, err = repo.Save(jobExec)
	require.NoError(t, err)

	// THEN retrieving jobExec by id should match saved execution
	loaded, err = repo.Get(savedExec.ID)
	require.NoError(t, err)
	require.NoError(t, loaded.Equals(jobExec))
	for i := 0; i < len(jobExec.Tasks); i++ {
		_, loadedTask := loaded.GetTask("", jobExec.Tasks[i].TaskType)
		require.NoError(t, jobExec.Tasks[i].Equals(loadedTask))
	}
}

// Deleting a job task should succeed
func Test_ShouldDeleteTaskValidJobExecution(t *testing.T) {
	// GIVEN repositories
	repo, err := NewTestJobExecutionRepository()
	require.NoError(t, err)
	repo.clear()
	qc, err := NewTestQC()
	require.NoError(t, err)

	_, job, err := NewTestJobExecution(qc, "test-job-for-delete-task")
	require.NoError(t, err)

	// AND previously saved job-execution
	saved, err := repo.Save(job)
	require.NoError(t, err)

	// Retrieving job by id should succeed
	_, err = repo.Get(saved.ID)
	require.NoError(t, err)

	// WHEN Deleting task
	err = repo.DeleteTask(job.Tasks[0].ID)
	require.NoError(t, err)

	// THEN retrieving job execution by id should fail
	loaded, err := repo.Get(saved.ID)
	require.NoError(t, err)
	if len(loaded.Tasks) != len(saved.Tasks)-1 {
		t.Fatalf("unexpected number of tasks after deleting first task %v", loaded)
	}
}

// Deleting a job execution should succeed
func Test_ShouldDeleteValidJobExecution(t *testing.T) {
	// GIVEN repositories
	repo, err := NewTestJobExecutionRepository()
	require.NoError(t, err)
	repo.clear()
	qc, err := NewTestQC()
	require.NoError(t, err)

	_, job, err := NewTestJobExecution(qc, "test-job-for-delete")
	require.NoError(t, err)

	// AND previously saved job
	saved, err := repo.Save(job)
	require.NoError(t, err)

	// Retrieving job by id
	_, err = repo.Get(saved.ID)
	require.NoError(t, err)

	// WHEN deleting job by id
	err = repo.Delete(job.ID)
	// THEN it should succeed
	require.NoError(t, err)

	// AND retrieving job execution by id should fail
	_, err = repo.Get(saved.ID)
	require.Error(t, err)
}
