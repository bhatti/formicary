package repository

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	common "plexobject.com/formicary/internal/types"

	"plexobject.com/formicary/queen/types"
)

// Get operation should fail if job-definition doesn't exist
func Test_ShouldGetJobDefinitionWithNonExistingId(t *testing.T) {
	// GIVEN a job-definition repository
	repo, err := NewTestJobDefinitionRepository()
	require.NoError(t, err)

	// WHEN finding non-existing job-definition
	_, err = repo.Get(common.NewQueryContext("", "", ""), "missing_id")

	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

// GetByType operation should fail if given job-type doesn't exist
func Test_ShouldGetByTypeJobDefinitionWithNonExistingType(t *testing.T) {
	// GIVEN a job-definition repository
	repo, err := NewTestJobDefinitionRepository()
	require.NoError(t, err)

	// WHEN finding non-existing job-definition by type
	_, err = repo.GetByType(qc, "non-existing-type")

	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

// test setting max concurrency for non-existing job
func Test_ShouldSetConcurrencyForJobDefinitionWithNonExistingType(t *testing.T) {
	// GIVEN a job-definition repository
	repo, err := NewTestJobDefinitionRepository()
	require.NoError(t, err)
	// WHEN setting concurrency for non-existing job-definition
	err = repo.SetMaxConcurrency("id", 3)
	// THEN it should fail
	require.Error(t, err)
}

// test setting max concurrency for existing job
func Test_ShouldSetConcurrencyForJobDefinitionWithExistingType(t *testing.T) {
	// GIVEN a job-definition repository
	repo, err := NewTestJobDefinitionRepository()
	require.NoError(t, err)

	// AND existing job-definition
	job := newTestJobDefinition("job-with-max-concurrency")
	saved, err := repo.Save(qc, job)
	require.NoError(t, err)

	// WHEN setting concurrency
	concurrency := rand.Intn(15) + 3
	err = repo.SetMaxConcurrency(saved.ID, concurrency)

	// THEN it should not fail
	require.NoError(t, err)

	// WHEN retrieving job by id
	loaded, err := repo.Get(qc, saved.ID)

	// THEN it should match saved concurrency
	require.NoError(t, err)
	require.Equal(t, concurrency, loaded.MaxConcurrency)
}

// Deleting non-existing job-definition should fail
func Test_ShouldDeleteByTypeJobDefinitionWithNonExistingType(t *testing.T) {
	// GIVEN a job-definition repository
	repo, err := NewTestJobDefinitionRepository()
	require.NoError(t, err)

	// WHEN deleting non-existing job-definition
	err = repo.Delete(qc, "non-existing-job")

	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to delete")
}

// Pausing non-existing job-definition should fail
func Test_ShouldPauseByTypeJobDefinitionWithNonExistingType(t *testing.T) {
	// GIVEN a job-definition repository
	repo, err := NewTestJobDefinitionRepository()
	require.NoError(t, err)

	// WHEN pausing non-existing job-definition
	err = repo.SetPaused("non-existing-job", true)

	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to set pause")
}

// Saving job-definition without job-type should fail
func Test_ShouldSaveJobDefinitionWithoutJobType(t *testing.T) {
	// GIVEN a job-definition repository
	repo, err := NewTestJobDefinitionRepository()
	require.NoError(t, err)

	// WHEN saving a job without job-type
	job := types.NewJobDefinition("")
	job.UserID = qc.UserID
	job.OrganizationID = qc.OrganizationID
	_, err = repo.Save(qc, job)
	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "jobType is not specified")
}

// Saving valid job-definition without config should succeed
func Test_ShouldSaveValidJobDefinitionWithoutConfig(t *testing.T) {
	// GIVEN a job-definition repository
	repo, err := NewTestJobDefinitionRepository()
	require.NoError(t, err)

	// Saving valid job
	job := newTestJobDefinition("valid-job-without-config")
	saved, err := repo.Save(qc, job)
	// THEN it should not fail
	require.NoError(t, err)

	// WHEN retrieving job by id
	loaded, err := repo.Get(qc, saved.ID)
	// THEN it should should match
	require.NoError(t, err)
	require.NoError(t, loaded.Equals(job))

	// WHEN retrieving job by job-type
	loaded, err = repo.GetByType(qc, job.JobType)
	// THEN it should should match
	require.NoError(t, err)
	require.NoError(t, loaded.Equals(job))
}

// Saving a job plugin and query
func Test_ShouldSaveValidPlugin(t *testing.T) {
	// GIVEN a job-definition repository
	repo, err := NewTestJobDefinitionRepository()
	require.NoError(t, err)
	repo.Clear()
	// WHEN creating a plugin job and regular jobs
	for i := 0; i < 10; i++ {
		job := newTestJobDefinition(fmt.Sprintf("test-plugin-%d", i))
		if i%2 == 0 {
			job.PublicPlugin = true
			job.SemVersion = "1.0"
		} else {
			job.SemVersion = "0.0"
		}
		_, err = repo.Save(qc, job)
		require.NoError(t, err)
	}

	// WHEN Querying jobs without filters
	params := make(map[string]interface{})
	_, total, err := repo.Query(qc, params, 0, 100, []string{"job_type desc"})
	// THEN it should match expected count
	require.NoError(t, err)
	require.Equal(t, int64(10), total)

	// WHEN querying jobs by plugin
	params["public_plugin"] = true
	res, total, err := repo.Query(qc, params, 0, 100, []string{"job_type desc"})
	// THEN it should match expected count
	require.NoError(t, err)
	require.Equal(t, int64(5), total)
	require.Equal(t, 5, len(res))
	require.True(t, res[0].PublicPlugin)
	require.Equal(t, "1.0", res[0].SemVersion)
}

// Saving a job with sem-version
func Test_ShouldSaveValidJobDefinitionWithSemVersion(t *testing.T) {
	// GIVEN a job-definition repository
	repo, err := NewTestJobDefinitionRepository()
	require.NoError(t, err)
	repo.Clear()
	// WHEN creating a plugin job
	job := newTestJobDefinition("TestPlugin")
	job.PublicPlugin = true
	_, _ = job.AddVariable("jk1", "jv1")
	for i := 0; i < 10; i++ {
		job.SemVersion = fmt.Sprintf("1.%d-dev1", i)
		_, err = repo.Save(qc, job)
		require.NoError(t, err)
		// WHEN saving plugin with same dev version
		_, err = repo.Save(qc, job)
		// THEN it should not fail
		require.NoError(t, err)

		// major.minor
		job.SemVersion = fmt.Sprintf("1.%d", i)
		// WHEN saving job with semantic version
		_, err = repo.Save(qc, job)
		// THEN it should not fail
		require.NoError(t, err)

		// WHEN saving plugin with same version
		_, err = repo.Save(qc, job)
		// THEN it should fail
		require.Error(t, err)
	}

	for i := 0; i < 10; i++ {
		// WHEN retrieving job by type and version
		semVersion := fmt.Sprintf("1.%d", i)
		loaded, err := repo.GetByTypeAndSemanticVersion(qc, job.JobType, semVersion)
		// THEN it should not fail and match job
		require.NoError(t, err)
		require.NoError(t, loaded.Equals(job))
		if i < 9 {
			_ = repo.Delete(qc, loaded.ID)
		}
	}

	for i := 0; i < 10; i++ {
		// WHEN Retrieving job by type
		semVersion := fmt.Sprintf("1.%d", i)
		loaded, err := repo.GetByType(qc, job.JobType+":"+semVersion)
		// THEN it should not fail and match job
		require.NoError(t, err)
		require.NoError(t, loaded.Equals(job))

		// AND sem-version should match
		require.Equal(t, semVersion, loaded.SemVersion)

		if i < 9 {
			require.False(t, loaded.Active) // old version becomes inactive
		} else {
			require.True(t, loaded.Active)
		}
	}
}

// Saving a job with config should succeed
func Test_ShouldSaveValidJobDefinitionWithConfig(t *testing.T) {
	// GIVEN a job-definition repository
	repo, err := NewTestJobDefinitionRepository()
	require.NoError(t, err)
	repo.Clear()
	// Creating a job
	job := newTestJobDefinition("valid-job-with-config-override")
	_, _ = job.AddVariable("jk1", "jv1")
	_, _ = job.AddVariable("jk2", true)
	_, _ = job.AddConfig("a1", "b1", false)

	// WHEN saving job
	saved, err := repo.Save(qc, job)
	// THEN it should not fail
	require.NoError(t, err)

	// WHEN retrieving job by id
	loaded, err := repo.Get(qc, saved.ID)
	// THEN it should match
	require.NoError(t, err)
	require.NoError(t, loaded.Equals(job))

	// WHEN retrieving job by job-type
	loaded, err = repo.GetByType(qc, job.JobType)
	// THEN it should match
	require.NoError(t, err)
	require.NoError(t, loaded.Equals(job))
}

// Updating a job definition should succeed
func Test_ShouldUpdateValidJobDefinition(t *testing.T) {
	// GIVEN a job-definition repository
	repo, err := NewTestJobDefinitionRepository()
	require.NoError(t, err)
	repo.Clear()
	job := newTestJobDefinition("test-job-for-update")

	// WHEN saving job
	saved, err := repo.Save(qc, job)
	// THEN it should not fail
	require.NoError(t, err)
	numConfig := len(job.Variables)

	// WHEN retrieving job by id
	loaded, err := repo.Get(qc, saved.ID)
	// THEN it should match
	require.NoError(t, err)
	require.NoError(t, loaded.Equals(job))
	require.Equal(t, numConfig, len(loaded.Variables))

	// WHEN updating variables and tasks
	job.RemoveVariable(job.Variables[0].Name)
	job.Tasks[0].TaskType = "new_task1"
	job.Tasks[2].OnExitCode["completed"] = "task4"
	job.Tasks[2].Variables = job.Tasks[2].Variables[1:]
	task4 := types.NewTaskDefinition("task4", common.Shell)
	task4.AlwaysRun = true
	_, _ = task4.AddVariable("t4k1", "v1")
	_, _ = task4.AddVariable("t4k2", []string{"i", "j", "k"})
	job.AddTask(task4)

	// AND saving it again
	saved, err = repo.Save(qc, job)
	// THEN it should not fail
	require.NoError(t, err)

	// WHEN retrieving job by id
	loaded, err = repo.Get(qc, saved.ID)
	// THEN it should match
	require.NoError(t, err)
	require.NoError(t, loaded.Equals(job))
}

// Pausing a persistent job-definition should succeed
func Test_ShouldPausingPersistentJobDefinition(t *testing.T) {
	// GIVEN a job-definition repository
	repo, err := NewTestJobDefinitionRepository()
	require.NoError(t, err)

	// AND an existing job
	job := types.NewJobDefinition("test.test-job-for-pause")
	job.UserID = qc.UserID
	job.OrganizationID = qc.OrganizationID
	task1 := types.NewTaskDefinition("task1", common.Shell)
	job.AddTask(task1)
	job.UpdateRawYaml()
	saved, err := repo.Save(qc, job)
	require.NoError(t, err)

	// WHEN pausing job by id
	err = repo.SetPaused(saved.ID, true)
	// THEN it should not fail
	require.NoError(t, err)
}

// Persisting persistent job-definition with encrypted config separately
func Test_ShouldSaveJobDefinitionWithSecretConfigSeparately(t *testing.T) {
	// GIVEN a job-definition repository
	repo, err := NewTestJobDefinitionRepository()
	require.NoError(t, err)
	//repo.Clear()

	// WHEN adding a job-definition with secret keys
	job := types.NewJobDefinition("my-test-job")
	job.UserID = qc.UserID
	job.OrganizationID = qc.OrganizationID
	_, _ = job.AddVariable("v1", "first")
	_, _ = job.AddVariable("v2", 200)
	_, _ = job.AddConfig("k0", "some-secret", true)
	_, _ = job.AddConfig("k1", "my-test-secret", true)
	_, _ = job.AddConfig("k2", "plain", false)
	_, _ = job.AddConfig("k3", 1011, true)
	task1 := types.NewTaskDefinition("task1", common.Shell)
	job.AddTask(task1)
	job.UpdateRawYaml()

	// AND saving the job
	saved, err := repo.Save(qc, job)
	require.NoError(t, err)

	_ = repo.DeleteConfig(qc, job.ID, saved.GetConfig("k0").ID)
	_, _ = repo.SaveConfig(qc, job.ID, "k1", "new-value", true)
	_, _ = repo.SaveConfig(qc, job.ID, "k4", "another", true)
	_, _ = repo.SaveConfig(qc, job.ID, "k5", 500, false)

	// THEN retrieving job will return with the configs
	loaded, err := repo.Get(qc, saved.ID)
	// AND it should fail
	require.NoError(t, err)
	require.NoError(t, loaded.Equals(job))
	require.Equal(t, 2, len(loaded.Variables))
	require.Equal(t, 5, len(loaded.Configs))
	require.Equal(t, "first", loaded.GetVariable("v1"))
	require.Equal(t, int64(200), loaded.GetVariable("v2"))
	require.Equal(t, "new-value", loaded.GetConfig("k1").Value)
	require.Equal(t, "plain", loaded.GetConfig("k2").Value)
	require.Equal(t, "1011", loaded.GetConfig("k3").Value)
	require.Equal(t, "another", loaded.GetConfig("k4").Value)
	require.Equal(t, "500", loaded.GetConfig("k5").Value)
}

// Persisting persistent job-definition with encrypted config
func Test_ShouldSaveJobDefinitionWithSecretConfig(t *testing.T) {
	// GIVEN a job-definition repository
	repo, err := NewTestJobDefinitionRepository()
	require.NoError(t, err)
	//repo.Clear()

	// WHEN adding a job-definition with secret keys
	job := types.NewJobDefinition("my-job")
	job.UserID = qc.UserID
	job.OrganizationID = qc.OrganizationID
	_, _ = job.AddVariable("v1", "first")
	_, _ = job.AddVariable("v2", 200)
	_, _ = job.AddConfig("k1", "my-test-secret", true)
	_, _ = job.AddConfig("k2", "plain", false)
	_, _ = job.AddConfig("k3", 1011, true)
	task1 := types.NewTaskDefinition("task1", common.Shell)
	job.AddTask(task1)
	job.UpdateRawYaml()

	// AND saving the job
	saved, err := repo.Save(qc, job)
	require.NoError(t, err)

	// THEN retrieving job will return with the configs
	loaded, err := repo.Get(qc, saved.ID)
	// AND it should fail
	require.NoError(t, err)
	require.NoError(t, loaded.Equals(job))
	require.Equal(t, 2, len(loaded.Variables))
	require.Equal(t, 3, len(loaded.Configs))
	require.Equal(t, "first", loaded.GetVariable("v1"))
	require.Equal(t, int64(200), loaded.GetVariable("v2"))
	require.Equal(t, "my-test-secret", loaded.GetConfig("k1").Value)
	require.Equal(t, "plain", loaded.GetConfig("k2").Value)
	require.Equal(t, "1011", loaded.GetConfig("k3").Value)
}

// Deleting a persistent job-definition should succeed
func Test_ShouldDeletePersistentJobDefinition(t *testing.T) {
	// GIVEN a job-definition repository
	repo, err := NewTestJobDefinitionRepository()
	require.NoError(t, err)

	// AND an existing job
	job := types.NewJobDefinition("test.test-job-for-delete")
	job.UserID = qc.UserID
	job.OrganizationID = qc.OrganizationID
	task1 := types.NewTaskDefinition("task1", common.Shell)
	job.AddTask(task1)
	job.UpdateRawYaml()
	saved, err := repo.Save(qc, job)
	require.NoError(t, err)

	// WHEN deleting job by id
	err = repo.Delete(common.NewQueryContext("", "", ""), saved.ID)
	// THEN it should not fail
	require.NoError(t, err)

	// WHEN retrieving deleted job
	loaded, err := repo.Get(qc, saved.ID)
	// THEN it should be inactive
	require.NoError(t, err)
	require.Equal(t, false, loaded.Active)
}

// Querying job-definitions by job-type
func Test_ShouldJobDefinitionQueryByJobType(t *testing.T) {
	// GIVEN a job-definition repository
	repo, err := NewTestJobDefinitionRepository()
	require.NoError(t, err)

	repo.Clear()
	jobs := make([]*types.JobDefinition, 0)
	// AND a set of jobs in the database
	for i := 0; i < 10; i++ {
		job := newTestJobDefinition(fmt.Sprintf("query-job-%v", i))
		saved, err := repo.Save(qc, job)
		require.NoError(t, err)
		jobs = append(jobs, saved)
	}

	// WHEN Querying jobs without filters
	params := make(map[string]interface{})
	_, total, err := repo.Query(qc, params, 0, 100, []string{"job_type desc"})
	// THEN it should match expected count
	require.NoError(t, err)
	require.Equal(t, int64(10), total)

	// WHEN querying jobs by job-type
	params["job_type"] = jobs[0].JobType
	res, total, err := repo.Query(qc, params, 0, 100, []string{"job_type desc"})
	// THEN it should match expected count
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, 1, len(res))
	require.NoError(t, res[0].Equals(jobs[0]))
}

// Test delete config with different versions
func Test_ShouldDeleteJobDefinitionConfigWithMultipleVersions(t *testing.T) {
	// GIVEN a job-definition repository
	repo, err := NewTestJobDefinitionRepository()
	require.NoError(t, err)

	repo.Clear()

	// AND a set of jobs in the database
	for i := 0; i < 10; i++ {
		job := types.NewJobDefinition("io.formicary.test.version-job")
		job.UserID = "test-user"
		job.OrganizationID = "test-org"
		if i == 0 {
			_, _ = job.AddConfig("cf1", "v1", false)
			_, _ = job.AddConfig("cf2", "v2", false)
			_, _ = job.AddConfig("cf3", "v3", false)
		}
		maxTasks := rand.Intn(3) + 2
		for j := 0; j < maxTasks; j++ {
			task := types.NewTaskDefinition(fmt.Sprintf("task%v", j), common.Shell)
			if j < maxTasks-1 {
				task.OnExitCode["completed"] = fmt.Sprintf("task%v", j+1)
			}
			job.AddTask(task)
		}
		job.UpdateRawYaml()
		saved, err := repo.Save(qc, job)
		require.NoError(t, err)
		require.Equal(t, int32(i), saved.Version)

		if i == 5 {
			err = repo.DeleteConfig(qc, job.ID, job.Configs[0].ID)
			require.NoError(t, err)
		}

		// verify
		loaded, err := repo.GetByType(qc, job.JobType)
		require.NoError(t, err)
		require.Equal(t, saved.ID, loaded.ID)
		require.Equal(t, int32(i), loaded.Version)
		if i < 5 {
			require.Equal(t, 3, len(loaded.Configs))
		} else {
			require.Equal(t, 2, len(loaded.Configs))
		}
	}
}

// Test saving config with different versions
func Test_ShouldSaveJobDefinitionConfigWithMultipleVersions(t *testing.T) {
	// GIVEN a job-definition repository
	repo, err := NewTestJobDefinitionRepository()
	require.NoError(t, err)

	repo.Clear()

	// AND a set of jobs in the database
	for i := 0; i < 10; i++ {
		job := types.NewJobDefinition("io.formicary.test.version-job")
		job.UserID = "test-user"
		job.OrganizationID = "test-org"
		if i == 0 {
			_, _ = job.AddConfig("cf1", "v1", true)
			_, _ = job.AddConfig("cf2", "v2", false)
			_, _ = job.AddConfig("cf3", "v3", false)
		}
		maxTasks := rand.Intn(3) + 2
		for j := 0; j < maxTasks; j++ {
			task := types.NewTaskDefinition(fmt.Sprintf("task%v", j), common.Shell)
			if j < maxTasks-1 {
				task.OnExitCode["completed"] = fmt.Sprintf("task%v", j+1)
			}
			job.AddTask(task)
		}
		job.UpdateRawYaml()
		saved, err := repo.Save(qc, job)
		require.NoError(t, err)
		require.Equal(t, int32(i), saved.Version)

		if i == 5 {
			_, err := repo.SaveConfig(qc, job.ID, "cf4", "v4", false)
			require.NoError(t, err)
		}

		// verify
		loaded, err := repo.GetByType(qc, job.JobType)
		require.NoError(t, err)
		require.Equal(t, saved.ID, loaded.ID)
		require.Equal(t, int32(i), loaded.Version)
		if i < 5 {
			require.Equal(t, 3, len(loaded.Configs))
		} else {
			require.Equal(t, 4, len(loaded.Configs))
		}
	}
}

// Test Update same job-definition with different versions
func Test_ShouldSaveJobDefinitionWithMultipleVersions(t *testing.T) {
	// GIVEN a job-definition repository
	repo, err := NewTestJobDefinitionRepository()
	require.NoError(t, err)

	repo.Clear()

	// AND a set of jobs in the database
	for i := 0; i < 10; i++ {
		job := types.NewJobDefinition("io.formicary.test.version-job")
		job.UserID = "test-user"
		job.OrganizationID = "test-org"
		if i == 0 {
			_, _ = job.AddConfig("cf1", "v1", false)
			_, _ = job.AddConfig("cf2", "v2", false)
			_, _ = job.AddConfig("cf3", "v3", false)
		}
		maxTasks := rand.Intn(3) + 2
		for j := 0; j < maxTasks; j++ {
			maxConfig := rand.Intn(3)
			task := types.NewTaskDefinition(fmt.Sprintf("task%v", j), common.Shell)
			if j < maxTasks-1 {
				task.OnExitCode["completed"] = fmt.Sprintf("task%v", j+1)
			}
			for k := 0; k < maxConfig; k++ {
				_, _ = task.AddVariable(fmt.Sprintf("k%v", k), k)
			}
			job.AddTask(task)
		}
		maxConfig := rand.Intn(3)
		for j := 0; j < maxConfig; j++ {
			_, _ = job.AddVariable(fmt.Sprintf("k%v", j), j)
		}
		job.UpdateRawYaml()
		saved, err := repo.Save(qc, job)
		require.NoError(t, err)
		require.Equal(t, int32(i), saved.Version)

		// verify
		loaded, err := repo.GetByType(qc, job.JobType)
		require.NoError(t, err)
		require.Equal(t, saved.ID, loaded.ID)
		require.Equal(t, int32(i), loaded.Version)
		require.Equal(t, 3, len(loaded.Configs))
	}

	// WHEN querying to jobs
	params := make(map[string]interface{})
	params["job_type"] = "io.formicary.test.version-job"
	res, total, err := repo.Query(qc, params, 0, 100, []string{"job_type desc"})
	// THEN only one job should be active with latest version
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, 1, len(res))
	require.Equal(t, int32(9), res[0].Version)
}

// Test Querying job-types and cron triggers
func Test_ShouldGetJobTypesAndCronTriggerForJobDefinition(t *testing.T) {
	// GIVEN a job-definition repository
	repo, err := NewTestJobDefinitionRepository()
	require.NoError(t, err)

	repo.Clear()

	// AND a set of jobs in the database
	for i := 0; i < 10; i++ {
		job := newTestJobDefinition(fmt.Sprintf("job-def-trigger-type-%v", i))
		if i%2 == 0 {
			job.CronTrigger = "0 19 * * *"
			job.OrganizationID = "one"
		} else {
			job.OrganizationID = "two"
		}
		_, err := repo.Save(common.NewQueryContext(job.UserID, job.OrganizationID, ""), job)
		require.NoError(t, err)
	}
	// WHEN finding job-types and cron trigger
	all, err := repo.GetJobTypesAndCronTrigger(common.NewQueryContext("", "", "").WithAdmin())
	// THEN it should not fail and return matching job-types
	require.NoError(t, err)
	require.Equal(t, 10, len(all))
	all, err = repo.GetJobTypesAndCronTrigger(common.NewQueryContext("", "one", ""))
	require.NoError(t, err)
	require.Equal(t, 5, len(all))
}

func Test_ShouldSaveJobDefinitionFromYaml(t *testing.T) {
	// GIVEN a job-definition repository
	repo, err := NewTestJobDefinitionRepository()
	require.NoError(t, err)
	repo.Clear()

	files := []string{
		"../../fixtures/test_job.yaml",
		"../../fixtures/basic-job.yaml",
		"../../fixtures/kube-build.yaml",
		"../../fixtures/encoding-job.yaml",
		"../../fixtures/fork_job.yaml",
	}
	for _, file := range files {
		b, err := ioutil.ReadFile(file)
		require.NoError(t, err)
		job, err := types.NewJobDefinitionFromYaml(b)
		require.NoError(t, err)
		saved, err := repo.Save(common.NewQueryContext("", "", ""), job)
		require.NoError(t, err)
		loaded, err := repo.GetByType(common.NewQueryContext("", "", ""), saved.JobType)
		require.NoError(t, err)
		for _, next := range loaded.Tasks {
			params := map[string]interface{}{
				"Token":             "tok1",
				"IsWindowsPlatform": true,
				"Platform":          "LINUX",
				"OSVersion":         "20.04.1",
				"Nonce":             1,
				"JobID":             1,
			}
			dynTask, opts, err := job.GetDynamicTask(next.TaskType, params)
			require.NoError(t, err)
			require.NotEqual(t, "", dynTask.Method)
			require.NotEqual(t, "", opts.Method)
		}
		require.Equal(t, len(loaded.Tasks), len(job.Tasks))
	}
}

// Test Query with different operators
func Test_ShouldJobDefinitionQueryWithDifferentOperators(t *testing.T) {
	// GIVEN a job-definition repository
	repo, err := NewTestJobDefinitionRepository()
	require.NoError(t, err)
	repo.Clear()

	// AND a set of job definitions in the database
	jobs := make([]*types.JobDefinition, 0)
	for i := 0; i < 10; i++ {
		job := newTestJobDefinition(fmt.Sprintf("job-def-query-operator-%v", i))
		saved, err := repo.Save(qc, job)
		require.NoError(t, err)
		jobs = append(jobs, saved)
	}

	// WHEN querying using LIKE
	params := make(map[string]interface{})
	params["job_type:like"] = "job-def-query-operator"
	_, total, err := repo.Query(qc, params, 0, 100, []string{"job_type desc"})
	// THEN it should match expected total
	require.NoError(t, err)
	require.Equal(t, int64(10), total)

	// WHEN querying using IN operator
	params = make(map[string]interface{})
	params["job_type:in"] = jobs[0].JobType + "," + jobs[1].JobType
	_, total, err = repo.Query(qc, params, 0, 100, []string{"job_type desc"})
	// THEN it should match expected total
	require.NoError(t, err)
	require.Equal(t, int64(2), total)

	// WHEN querying using exact operator
	params = make(map[string]interface{})
	params["job_type:="] = jobs[0].JobType
	_, total, err = repo.Query(qc, params, 0, 100, []string{"job_type desc"})
	// THEN it should match expected total
	require.NoError(t, err)
	require.Equal(t, int64(1), total)

	// WHEN querying using not equal operator
	params = make(map[string]interface{})
	params["job_type:!="] = jobs[0].JobType
	_, total, err = repo.Query(qc, params, 0, 100, []string{"job_type desc"})
	// THEN it should match expected total
	require.NoError(t, err)
	require.Equal(t, int64(9), total)

	// WHEN querying using GreaterThan operator
	params = make(map[string]interface{})
	params["job_type:>"] = jobs[0].JobType
	_, total, err = repo.Query(qc, params, 0, 100, []string{"job_type desc"})
	// THEN it should match expected total
	require.NoError(t, err)
	require.Equal(t, int64(9), total)

	// WHEN querying using LessThan operator
	params = make(map[string]interface{})
	params["job_type:<"] = jobs[9].JobType
	_, total, err = repo.Query(qc, params, 0, 100, []string{"job_type desc"})
	// THEN it should match expected total
	require.NoError(t, err)
	require.Equal(t, int64(9), total)

	// WHEN querying by another user
	_, total, err = repo.Query(common.NewQueryContext("a", "b", ""), params, 0, 100, []string{"job_type desc"})
	if err != nil {
		t.Fatalf("unexpected query error %v", err)
	}
	// THEN it should not find data for other user
	require.NoError(t, err)
	require.Equal(t, int64(0), total)
}

// Creating a test job
func newTestJobDefinition(name string) *types.JobDefinition {
	job := types.NewJobDefinition("io.formicary.test." + name)
	job.UserID = "test-user"
	job.OrganizationID = "test-org"
	_, _ = job.AddVariable("jk1", "jv1")
	_, _ = job.AddVariable("jk2", map[string]int{"a": 1, "b": 2})
	_, _ = job.AddVariable("jk3", "jv3")
	for i := 1; i < 10; i++ {
		task := types.NewTaskDefinition(fmt.Sprintf("task%d", i), common.Shell)
		if i < 9 {
			task.OnExitCode["completed"] = fmt.Sprintf("task%d", i+1)
		}
		prefix := fmt.Sprintf("t%d", i)
		task.BeforeScript = []string{prefix + "_cmd1", prefix + "_cmd2", prefix + "_cmd3"}
		task.AfterScript = []string{prefix + "_cmd1", prefix + "_cmd2", prefix + "_cmd3"}
		task.Script = []string{prefix + "_cmd1", prefix + "_cmd2", prefix + "_cmd3"}
		task.Headers = map[string]string{prefix + "_h1": "1", prefix + "_h2": "true", prefix + "_h3": "three"}
		_, _ = task.AddVariable(prefix+"k1", "v1")
		_, _ = task.AddVariable(prefix+"k2", []string{"i", "j", "k"})
		_, _ = task.AddVariable(prefix+"k3", "v3")
		_, _ = task.AddVariable(prefix+"k4", 14.123)
		_, _ = task.AddVariable(prefix+"k5", true)
		_, _ = task.AddVariable(prefix+"k6", 50)
		_, _ = task.AddVariable(prefix+"k7", map[string]string{"i": "a", "j": "b", "k": "c"})
		_, _ = task.AddVariable(prefix+"k8", 4.881)
		task.Method = common.Docker
		if i%2 == 1 {
			task.AlwaysRun = true
		}
		job.AddTask(task)
	}
	job.UpdateRawYaml()

	return job
}
