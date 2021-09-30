package repository

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	common "plexobject.com/formicary/internal/types"

	"plexobject.com/formicary/queen/types"
)

// Fetching a non-existing job should fail
func Test_ShouldGetJobRequestNonExistingId(t *testing.T) {
	// GIVEN a job-resource repository
	repo, err := NewTestJobRequestRepository()
	require.NoError(t, err)
	qc, err := NewTestQC()
	require.NoError(t, err)

	// WHEN fetching non-existing request-id
	_, err = repo.Get(qc, 143242)

	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

// Creating a job request without job-definition should fail
func Test_ShouldJobRequestWithoutJobDefinition(t *testing.T) {
	req, err := types.NewJobRequestFromDefinition(&types.JobDefinition{})
	require.NoError(t, err)
	req.JobType = "some-type"
	req.JobDefinitionID = ""

	// WHEN validating request without job-definition
	err = req.ValidateBeforeSave()

	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "jobDefinitionID is not specified")
}

// Creating a job-request without job-type should fail
func Test_ShouldJobRequestWithoutJobType(t *testing.T) {
	req, err := types.NewJobRequestFromDefinition(&types.JobDefinition{})
	require.NoError(t, err)
	req.JobDefinitionID = "some-id"
	// WHEN validating request without job-type
	err = req.ValidateBeforeSave()
	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "jobType is not specified")
}

// Creating a job-request without job-state should fail
func Test_ShouldJobRequestWithoutJobState(t *testing.T) {
	req, err := types.NewJobRequestFromDefinition(&types.JobDefinition{})
	require.NoError(t, err)
	req.JobType = "some-type"
	req.JobDefinitionID = "some-id"
	req.JobState = ""

	// WHEN validating job-request without job-state
	err = req.ValidateBeforeSave()
	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "jobState is not specified")
}

// Creating a job request without scheduled-date should fail
func Test_ShouldJobRequestWithoutScheduleDate(t *testing.T) {
	req, err := types.NewJobRequestFromDefinition(&types.JobDefinition{})
	require.NoError(t, err)
	req.JobDefinitionID = "some-id"
	req.JobType = "some-type"
	req.JobState = "PENDING"
	req.ScheduledAt = time.Time{}
	// WHEN validating job-request without scheduled-date
	err = req.ValidateBeforeSave()
	// THEN it should fail
	require.Error(t, err)
	require.Contains(t, err.Error(), "scheduledAt is not specified")
}

// Saving a job-request should succeed
func Test_ShouldSaveValidJobRequest(t *testing.T) {
	// GIVEN a job-resource repository
	repo, err := NewTestJobRequestRepository()
	require.NoError(t, err)
	repo.Clear()
	qc, err := NewTestQC()
	require.NoError(t, err)

	// AND a job-definition in the database
	job, err := SaveTestJobDefinition(qc, "test-job-for-request-without-params", "")
	require.NoError(t, err)

	// WHEN saving request
	req, err := types.NewJobRequestFromDefinition(job)
	require.NoError(t, err)
	req.UserID = qc.User.ID
	req.OrganizationID = qc.User.OrganizationID
	_, _ = req.AddParam("jk1", "jv1")
	_, _ = req.AddParam("jk2", map[string]int{"a": 1, "b": 2})
	saved, err := repo.Save(req)

	// THEN it should not fail
	require.NoError(t, err)

	// WHEN retrieving by id
	loaded, err := repo.Get(qc, req.ID)

	// THEN it should not fail
	require.NoError(t, err)
	require.NoError(t, loaded.Equals(saved))

	// WHEN updating job request
	req.ClearParams()
	_, _ = req.AddParam("jk1", "jv1")
	_, _ = req.AddParam("jk3", 3)
	_, _ = req.AddParam("jk4", true)
	req.JobState = common.READY
	saved, err = repo.Save(req)
	// THEN it should not fail
	require.NoError(t, err)

	// WHEN retrieving by id
	loaded, err = repo.Get(qc, req.ID)
	// THEN it should not fail
	require.NoError(t, err)
	require.NoError(t, loaded.Equals(req))

	params, err := repo.GetParams(req.ID)
	require.NoError(t, err)
	require.Equal(t, 3, len(params))
}

// Updating state of job-request should succeed
func Test_ShouldUpdateStateOfJobRequest(t *testing.T) {
	// GIVEN a job-resource repository
	repo, err := NewTestJobRequestRepository()
	require.NoError(t, err)
	qc, err := NewTestQC()
	require.NoError(t, err)
	// AND a job-definition
	job, err := SaveTestJobDefinition(qc, "test-job-for-state-update", "")
	require.NoError(t, err)

	// WHEN updating state of non-existing request should fail
	err = repo.UpdateJobState(7891, "PENDING", "READY", "", "", 0, 0)
	// THEN it should fail
	require.Error(t, err)

	// WHEN saving a job-request
	req, err := types.NewJobRequestFromDefinition(job)
	require.NoError(t, err)
	req.UserID = qc.User.ID
	req.OrganizationID = qc.User.OrganizationID
	// Saving request
	_, err = repo.Save(req)
	require.NoError(t, err)

	// AND Updating state with non-matching old state should fail
	err = repo.UpdateJobState(req.ID, "BLAH", "READY", "", "", 0, 0)
	// THEN it should fail
	require.Error(t, err)

	// WHEN Updating state with valid old state
	err = repo.UpdateJobState(req.ID, "PENDING", "READY", "", "", 0, 0)
	// THEN it should not fail
	require.NoError(t, err)

	// WHEN finding request by id
	loaded, err := repo.Get(qc, req.ID)
	// THEN it should not fail
	require.NoError(t, err)
	require.Equal(t, common.READY, loaded.JobState)
}

// SetReadyToExecute should update state of job to READY
func Test_ShouldUpdateStateToExecutingForJobRequest(t *testing.T) {
	// GIVEN a job-resource repository
	repo, err := NewTestJobRequestRepository()
	require.NoError(t, err)
	qc, err := NewTestQC()
	require.NoError(t, err)
	// AND a job-definition
	job, err := SaveTestJobDefinition(qc, "test-job-for-state-update", "")
	require.NoError(t, err)

	// WHEN marking nonexisting request as executing
	err = repo.SetReadyToExecute(7891, "123234", "")
	// THEN it should fail
	require.Error(t, err)

	// WHEN saving job-request
	req, err := types.NewJobRequestFromDefinition(job)
	require.NoError(t, err)
	req.UserID = qc.User.ID
	req.OrganizationID = qc.User.OrganizationID
	_, err = repo.Save(req)
	require.NoError(t, err)

	// WHEN marking job ready to execute for unknown job-execution-id
	err = repo.SetReadyToExecute(req.ID, "xxx", "")
	// THEN it should fail
	require.Error(t, err)

	// Saving job-execution
	jobExec, err := saveTestJobExecutionForRequest(req, job)
	require.NoError(t, err)

	// WHEN marking the request from PENDING to READY
	err = repo.SetReadyToExecute(req.ID, jobExec.ID, "")
	// THEN it should not fail
	require.NoError(t, err)

	// WHEN marking the request to READY again
	err = repo.SetReadyToExecute(req.ID, jobExec.ID, "")
	// THEN it should fail
	require.Error(t, err)

	// WHEN loading the request
	loaded, err := repo.Get(qc, req.ID)
	// THEN it should not fail and has READY state
	require.NoError(t, err)
	require.Equal(t, common.READY, loaded.JobState)
	require.Equal(t, jobExec.ID, loaded.JobExecutionID)
}

// Updating priority of job-request should succeed
func Test_ShouldUpdatePriorityOfJobRequest(t *testing.T) {
	// GIVEN a job-resource repository
	repo, err := NewTestJobRequestRepository()
	require.NoError(t, err)
	qc, err := NewTestQC()
	require.NoError(t, err)
	// AND a job-definition
	job, err := SaveTestJobDefinition(qc, "test-job-for-priority-update", "")
	require.NoError(t, err)

	// WHEN updating priority of non-existing request
	err = repo.UpdatePriority(qc, 7891, 20)
	// THEN it should fail
	require.Error(t, err)

	// GIVE a saved a job-request
	req, err := types.NewJobRequestFromDefinition(job)
	require.NoError(t, err)
	req.UserID = qc.User.ID
	req.OrganizationID = qc.User.OrganizationID
	_, err = repo.Save(req)
	require.NoError(t, err)

	// WHEN updating priority
	err = repo.UpdatePriority(qc, req.ID, 20)
	// THEN it should not fail
	require.NoError(t, err)

	// WHEN loading request
	loaded, err := repo.Get(qc, req.ID)

	// THEN it should match the priority
	require.NoError(t, err)
	require.Equal(t, 20, loaded.JobPriority)
}

// Querying job-requests should succeed
func Test_ShouldQueryJobRequest(t *testing.T) {
	// GIVEN a job-resource repository
	repo, err := NewTestJobRequestRepository()
	require.NoError(t, err)
	repo.Clear()
	qc, err := NewTestQC()
	require.NoError(t, err)
	job, err := SaveTestJobDefinition(qc, "simple-job-for-query", "")
	require.NoError(t, err)

	// AND a set of job requests in the database
	requests := make(map[uint64]*types.JobRequest, 0)
	for i := 0; i < 10; i++ {
		req, err := types.NewJobRequestFromDefinition(job)
		require.NoError(t, err)
		req.JobPriority = 0
		max := rand.Intn(5) + 2
		for j := 0; j < max; j++ {
			_, _ = req.AddParam(fmt.Sprintf("k%v", j), j)
		}
		req.UserID = qc.User.ID
		req.OrganizationID = qc.User.OrganizationID
		saved, err := repo.Save(req)
		require.NoError(t, err)
		requests[saved.ID] = saved
	}

	// WHEN querying all requests
	params := make(map[string]interface{})
	_, total, err := repo.Query(qc, params, 0, 100, []string{"job_priority"})

	// THEN it should return matching results
	require.NoError(t, err)
	require.Equal(t, int64(10), total)

	// WHEN querying a specific job-request
	params["job_type"] = "io.formicary.test.simple-job-for-query"
	res, total, err := repo.Query(qc, params, 0, 100, []string{"job_priority"})
	// THEN it should return matching results
	require.NoError(t, err)
	require.Equal(t, int64(10), total)
	require.Equal(t, 10, len(res))
	for _, next := range res {
		require.NoError(t, next.Equals(requests[next.ID]))
	}
}

// Updating job request and querying with multiple operators should succeed
func Test_ShouldUpdateAndQueryWithMultipleOperators(t *testing.T) {
	// GIVEN a job-resource repository
	repo, err := NewTestJobRequestRepository()
	require.NoError(t, err)
	repo.Clear()
	qc, err := NewTestQC()
	require.NoError(t, err)

	// AND a set of job requests in the database
	requests := make([]*types.JobRequest, 10)
	for i := 0; i < 10; i++ {
		job, err := SaveTestJobDefinition(qc, fmt.Sprintf("job-for-requests-with-params-%v", i), "")
		require.NoError(t, err)
		req, err := types.NewJobRequestFromDefinition(job)
		require.NoError(t, err)
		req.UserID = qc.User.ID
		req.OrganizationID = qc.User.OrganizationID
		max := rand.Intn(5) + 2
		for j := 0; j < max; j++ {
			_, _ = req.AddParam(fmt.Sprintf("k%v", j), j)
		}
		// Saving job request
		_, err = repo.Save(req)
		require.NoError(t, err)
		max = rand.Intn(5) + 2
		for j := 0; j < max; j++ {
			_, _ = req.AddParam(fmt.Sprintf("k%v", j), j)
		}
		// Updating job request
		saved, err := repo.Save(req)
		require.NoError(t, err)
		requests[i] = saved
	}

	// WHEN querying job requests by job type
	params := make(map[string]interface{})
	params["job_type"] = requests[0].JobType
	res, total, err := repo.Query(qc, params, 0, 100, []string{"job_type desc"})
	// THEN it should return matching results
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, 1, len(res))

	// WHEN querying similar job-types
	params = make(map[string]interface{})
	params["job_type:like"] = "job-for-requests-with-params-"
	res, total, err = repo.Query(qc, params, 0, 100, []string{"job_type desc"})
	// THEN it should return matching results
	require.NoError(t, err)
	require.Equal(t, int64(10), total)
	require.Equal(t, 10, len(res))

	// WHEN querying job-requests using IN operator
	params = make(map[string]interface{})
	params["job_type:in"] = requests[0].JobType + "," + requests[1].JobType
	_, total, err = repo.Query(qc, params, 0, 100, []string{"job_type desc"})
	// THEN it should return matching results
	require.NoError(t, err)
	require.Equal(t, int64(2), total)

	// WHEN querying jobs using equal operator
	params = make(map[string]interface{})
	params["job_type:="] = requests[0].JobType
	_, total, err = repo.Query(qc, params, 0, 100, []string{"job_type desc"})
	// THEN it should return matching results
	require.NoError(t, err)
	require.Equal(t, int64(1), total)

	// WHEN querying job requests using not equal operator
	params = make(map[string]interface{})
	params["job_type:!="] = requests[0].JobType
	_, total, err = repo.Query(qc, params, 0, 100, []string{"job_type desc"})
	// THEN it should return matching results
	require.NoError(t, err)
	require.Equal(t, int64(9), total)

	// WHEN querying job requests suing Greater Than operator
	params = make(map[string]interface{})
	params["job_type:>"] = requests[0].JobType
	_, total, err = repo.Query(qc, params, 0, 100, []string{"job_type desc"})
	// THEN it should return matching results
	require.NoError(t, err)
	require.Equal(t, int64(9), total)

	// WHEN querying job requests using Less Than operator
	params = make(map[string]interface{})
	params["job_type:<"] = requests[9].JobType
	_, total, err = repo.Query(qc, params, 0, 100, []string{"job_type desc"})
	// THEN it should return matching results
	require.NoError(t, err)
	require.Equal(t, int64(9), total)
}

// Test query for triggered jobs that are active
func Test_ShouldFindJobTimes(t *testing.T) {
	// GIVEN a job-resource repository
	repo, err := NewTestJobRequestRepository()
	require.NoError(t, err)
	repo.Clear()
	qc, err := NewTestQC()
	require.NoError(t, err)

	now := time.Now()

	// AND a set of job requests in the database
	jobTypes := make([]string, 0)
	for i := 0; i < 10; i++ {
		jobType := fmt.Sprintf("time-job-%v-%v", i, now.Format("15:04:05"))
		jobTypes = append(jobTypes, jobType)
		job, err := SaveTestJobDefinition(qc, jobType, "")
		require.NoError(t, err)
		jobStates := []common.RequestState{common.PENDING, common.READY, common.STARTED,
			common.EXECUTING, common.COMPLETED, common.FAILED}
		// See https://github.com/hashicorp/cronexpr

		for j, jobState := range jobStates {
			for k := 0; k < 2; k++ { // 10 * 2 * 6
				req, err := types.NewJobRequestFromDefinition(job)
				require.NoError(t, err)
				_, err = repo.Save(req)
				require.NoError(t, err)
				req.JobState = jobState
				req.JobPriority = j
				// updating state and priority because by default state is PENDING
				_, err = repo.Save(req)
				require.NoError(t, err)
			}
		}
	}

	// WHEN finding job-types
	recs, err := repo.GetJobTimes(1000)

	// THEN it should not fail
	require.NoError(t, err)
	require.Equal(t, 120, len(recs))
}

// Test query for triggered jobs that are active
func Test_ShouldFindActiveCronScheduledJobs(t *testing.T) {
	// GIVEN a job-resource repository
	repo, err := NewTestJobRequestRepository()
	require.NoError(t, err)
	repo.Clear()
	qc, err := NewTestQC()
	require.NoError(t, err)

	now := time.Now()
	cronTriggers := []string{"0 0 * * * * *", "0 0 0 * * * *", "0 0 0 * * 0 *",
		"0 * * * * * *", "0 0 * * * * *", "0 * * * * * *"}
	jobTypes := make([]types.JobTypeCronTrigger, 0)
	// AND a set of test requests in the database
	for i := 0; i < 10; i++ {
		jobType := fmt.Sprintf("cron-job-%v-%v", i, now.Format("15:04:05"))
		job, err := SaveTestJobDefinition(qc, jobType, cronTriggers[i%len(cronTriggers)])
		require.NoError(t, err)
		jobTypes = append(jobTypes, types.NewJobTypeCronTrigger(job))
		jobStates := []common.RequestState{common.PENDING, common.READY, common.STARTED,
			common.EXECUTING, common.COMPLETED, common.FAILED}
		// See https://github.com/hashicorp/cronexpr

		for j, jobState := range jobStates {
			for k := 0; k < 2; k++ { // 10 * 2 * 6
				req, err := types.NewJobRequestFromDefinition(job)
				require.NoError(t, err)
				_, _ = req.AddParam("p1", "v1")
				_, _ = req.AddParam("p2", "v2")
				req.UserID = qc.User.ID
				req.OrganizationID = qc.User.OrganizationID
				_, err = repo.Save(req)
				require.NoError(t, err)
				req.JobState = jobState
				req.JobPriority = j
				// Updating state and priority because by default state is PENDING
				_, err = repo.Save(req)
				require.NoError(t, err)
				err = repo.Trigger(qc, req.ID)
				require.NoError(t, err)
			}
		}
	}

	params := make(map[string]interface{})
	// WHEN counting requests
	total, err := repo.Count(qc, params)
	// THEN it should match expected count
	require.NoError(t, err)
	require.Equal(t, int64(120), total)

	// WHEN finding active requests by jobTypes
	infos, err := repo.FindActiveCronScheduledJobsByJobType(jobTypes)
	// THEN it should match expected count
	require.NoError(t, err)
	require.Equal(t, 10, len(infos))

	// WHEN finding active requests by userKeys
	infos, err = repo.FindActiveCronScheduledJobsByJobType(jobTypes)
	// THEN it should match expected count
	require.NoError(t, err)
	require.Equal(t, 10, len(infos))
	err = repo.DeletePendingCronByJobType(qc, jobTypes[0].JobType)
	require.NoError(t, err)
}

// Test request infos for jobs that can be scheduled
func Test_ShouldNextSchedulableJobs(t *testing.T) {
	now := time.Now()
	// GIVEN a job-resource repository
	jobRequestRepository, err := NewTestJobRequestRepository()
	require.NoError(t, err)
	jobRequestRepository.Clear()
	qc, err := NewTestQC()
	require.NoError(t, err)
	start := time.Now()

	// WHEN scheduling jobs with empty database
	infos, err := jobRequestRepository.NextSchedulableJobsByType(make([]string, 0), common.PENDING, 100)
	// THEN it should match 0 count
	require.NoError(t, err)
	require.Equal(t, 0, len(infos))

	// GIVEN a test requests in the database
	for i := 0; i < 5; i++ {
		job, err := SaveTestJobDefinition(qc, fmt.Sprintf("job-for-infos-%v-%v", i, now.Format("15:04:05")), "")
		require.NoError(t, err)
		// half will be saved as PENDING and half as READY
		for j := 0; j < 20; j++ {
			req, err := types.NewJobRequestFromDefinition(job)
			require.NoError(t, err)
			req.OrganizationID = fmt.Sprintf("org_%d", i)
			req.UserID = fmt.Sprintf("user_%d_%d", i, j)
			// first two outer rows should leave 10 * 2 = pending jobs
			req.ScheduledAt = start.Add(time.Duration(i-1) * time.Second)
			_, err = jobRequestRepository.Save(req)
			require.NoError(t, err)
			if j%2 == 0 {
				req.JobState = common.PENDING
			} else {
				req.JobState = common.READY
			}
			req.JobPriority = j
			// updating state and priority
			_, err = jobRequestRepository.Save(req)
			require.NoError(t, err)
		}
	}

	params := make(map[string]interface{})

	// WHEN counting request
	total, err := jobRequestRepository.Count(common.NewQueryContext(nil, ""), params)
	// THEN it should match expected count
	require.NoError(t, err)
	require.Equal(t, int64(100), total)

	// WHEN counting request by organization
	total, err = jobRequestRepository.Count(common.NewQueryContextFromIDs("", "org_0"), params)
	// THEN it should match expected count
	require.NoError(t, err)
	require.Equal(t, int64(20), total)

	// WHEN querying PENDING requests
	jobs, total, err := jobRequestRepository.Query(
		common.NewQueryContext(nil, ""),
		map[string]interface{}{"job_state": common.PENDING}, 0, 20, []string{})
	// THEN it should match expected count
	require.NoError(t, err)
	require.Equal(t, int64(50), total)
	require.Equal(t, 20, len(jobs))

	// WHEN scheduling jobs (top pending jobs)
	infos, err = jobRequestRepository.NextSchedulableJobsByType(make([]string, 0), common.PENDING, 10)
	// THEN it should match expected count
	require.NoError(t, err)
	require.Equal(t, 10, len(infos))
	for _, info := range infos {
		err = jobRequestRepository.UpdateJobState(info.ID, info.JobState, common.READY, "", "", 0, 0)
		require.NoError(t, err)
	}

	// WHEN scheduling next top pending jobs
	infos, err = jobRequestRepository.NextSchedulableJobsByType(make([]string, 0), common.PENDING, 10)
	// THEN it should match expected count
	require.NoError(t, err)
	require.Equal(t, 10, len(infos))
	for _, info := range infos {
		err = jobRequestRepository.UpdateJobState(info.ID, info.JobState, common.READY, "", "", 0, 0)
		require.NoError(t, err)
	}

	// WHEN scheduling next top pending jobs
	infos, err = jobRequestRepository.NextSchedulableJobsByType(make([]string, 0), common.PENDING, 10)
	// THEN it should return 0 records
	require.NoError(t, err)
	require.Equal(t, 0, len(infos))
}

// Test Query dead ids
func Test_ShouldQueryDeadIDs(t *testing.T) {
	// GIVEN a job-resource repository
	repo, err := NewTestJobRequestRepository()
	require.NoError(t, err)
	repo.Clear()
	qc, err := NewTestQC()
	require.NoError(t, err)
	states := []string{"PENDING", "READY", "STARTED", "EXECUTING", "FAILED", "COMPLETED", "CANCELLED",
		"PENDING", "READY", "STARTED", "EXECUTING", "FAILED", "COMPLETED", "CANCELLED"}
	// AND a test requests in the database
	for i := 0; i < 20; i++ {
		job, err := SaveTestJobDefinition(qc, "dead-job", "")
		require.NoError(t, err)
		for _, state := range states {
			req, err := types.NewJobRequestFromDefinition(job)
			require.NoError(t, err)
			_, err = repo.Save(req)
			require.NoError(t, err)

			// updating state
			req.JobState = common.NewRequestState(state)
			req.UserID = qc.User.ID
			req.OrganizationID = qc.User.OrganizationID
			_, err = repo.Save(req)
			require.NoError(t, err)
		}
	}

	params := make(map[string]interface{})

	// WHEN querying job-requests
	recs, total, err := repo.Query(qc, params, 0, 500, make([]string, 0))
	// THEN it should match expected count
	require.NoError(t, err)
	require.Equal(t, int64(20*len(states)), total)
	require.Equal(t, 20*len(states), len(recs))

	// WHEN querying COMPLETED jobs
	params["job_state"] = "COMPLETED"
	recs, total, err = repo.Query(qc, params, 0, 500, make([]string, 0))
	// THEN it should match expected count
	require.NoError(t, err)
	require.Equal(t, int64(40), total)
	require.Equal(t, 40, len(recs))

	// WHEN searching recently completed jobs
	ids, err := repo.RecentDeadIDs(10)
	// THEN it should match expected count
	require.NoError(t, err)
	require.Equal(t, 10, len(ids))
}

// Test Query by aggregate states
func Test_ShouldQueryAggregateStatesJobRequests(t *testing.T) {
	// GIVEN a job-resource repository
	repo, err := NewTestJobRequestRepository()
	require.NoError(t, err)
	repo.Clear()
	qc, err := NewTestQC()
	require.NoError(t, err)
	job, err := SaveTestJobDefinition(qc, "job-for-aggregate-states", "")
	require.NoError(t, err)
	states := []string{"PENDING", "READY", "STARTED", "EXECUTING", "FAILED", "COMPLETED", "CANCELLED",
		"PENDING", "READY", "STARTED", "EXECUTING", "FAILED", "COMPLETED", "CANCELLED"}
	params := make(map[string]interface{})
	// AND a set of test requests in the database
	for i := 0; i < 20; i++ {
		for j, state := range states {
			req, err := types.NewJobRequestFromDefinition(job)
			require.NoError(t, err)
			req.OrganizationID = fmt.Sprintf("org_%d", i)
			req.UserID = fmt.Sprintf("user_%d_%d", i, j)
			req.JobPriority = rand.Intn(100)
			_, err = repo.Save(req)
			require.NoError(t, err)

			// updating state
			req.JobState = common.NewRequestState(state)
			_, err = repo.Save(req)
			require.NoError(t, err)
		}
	}

	params["job_state"] = "PENDING"
	params["job_type"] = job.JobType

	// WHEN querying jobs by job-type and PENDING
	recs, total, err := repo.Query(common.NewQueryContext(nil, ""), params, 0, 10, make([]string, 0))
	for _, rec := range recs {
		require.Equal(t, common.PENDING, rec.JobState)
	}
	// THEN it should match expected count
	require.NoError(t, err)
	require.Equal(t, int64(40), total)
	require.Equal(t, 10, len(recs))

	// WHEN querying job requests by DONE state
	params["job_state"] = "DONE"
	recs, total, err = repo.Query(common.NewQueryContext(nil, ""), params, 0, 50, make([]string, 0))
	for _, rec := range recs {
		if rec.JobState != "FAILED" && rec.JobState != "COMPLETED" && rec.JobState != "CANCELLED" {
			t.Fatalf("unexpected state of req %v", rec)
		}
	}
	// THEN it should match expected count
	require.NoError(t, err)
	require.Equal(t, int64(120), total)
	require.Equal(t, 50, len(recs))

	// WHEN querying jobs by RUNNING state
	params["job_state"] = "RUNNING"
	recs, total, err = repo.Query(common.NewQueryContext(nil, ""), params, 0, 100, make([]string, 0))
	for _, rec := range recs {
		if rec.JobState != "STARTED" && rec.JobState != "EXECUTING" {
			t.Fatalf("unexpected state of req %v", rec)
		}
	}
	// THEN it should match expected count
	require.NoError(t, err)
	require.Equal(t, int64(80), total)
	require.Equal(t, 80, len(recs))

	// WHEN querying state by WAITING
	params["job_state"] = "WAITING"
	recs, total, err = repo.Query(common.NewQueryContext(nil, ""), params, 0, 100, make([]string, 0))
	for _, rec := range recs {
		if rec.JobState != "PENDING" && rec.JobState != "READY" {
			t.Fatalf("unexpected state of req %v", rec)
		}
	}
	// THEN it should match expected count
	require.NoError(t, err)
	require.Equal(t, int64(80), total)
	require.Equal(t, 80, len(recs))
}

// Test Query Orphan jobs
func Test_ShouldFindOrphanJobRequests(t *testing.T) {
	now := time.Now().Unix()
	// GIVEN a job-resource repository
	repo, err := NewTestJobRequestRepository()
	require.NoError(t, err)
	repo.Clear()
	qc, err := NewTestQC()
	require.NoError(t, err)
	states := []string{"PENDING", "READY", "STARTED", "EXECUTING", "FAILED", "COMPLETED", "CANCELLED"}

	// GIVEN a set of test requests in the database
	for i := 0; i < 10; i++ {
		job, err := SaveTestJobDefinition(qc, fmt.Sprintf("job-for-stale-%v-%v", i, now), "")
		require.NoError(t, err)
		for j, state := range states {
			req, err := types.NewJobRequestFromDefinition(job)
			require.NoError(t, err)
			req.OrganizationID = fmt.Sprintf("org_%d", i)
			req.UserID = fmt.Sprintf("user_%d_%d", i, j)
			_, err = repo.Save(req)
			require.NoError(t, err)

			// updating state
			req.JobState = common.NewRequestState(state)
			_, err = repo.Save(req)
			require.NoError(t, err)
		}
	}

	// WHEN querying jobs
	_, total, err := repo.Query(
		common.NewQueryContext(nil, ""),
		map[string]interface{}{}, 0, 10000, []string{})
	// THEN it should match expected count
	require.NoError(t, err)
	require.Equal(t, int64(70), total)

	// WHEN querying RUNNING state
	_, total, err = repo.Query(
		common.NewQueryContext(nil, ""),
		map[string]interface{}{"job_state": "RUNNING"}, 0, 10000, []string{})
	// THEN it should match expected count
	require.NoError(t, err)
	require.Equal(t, int64(20), total) // READY + STARTED + EXECUTING

	// WHEN querying orphan requests
	recs, err := repo.QueryOrphanRequests(10000, 0, 5)
	// THEN it should match expected count
	require.NoError(t, err)
	require.Equal(t, 0, len(recs))

	// Marking job's updated-time to epoch time so that those are considered as orphans/stale requests
	res := repo.db.Exec("update formicary_job_requests set updated_at = '2020-12-16 00:00:00.000'")
	require.Equal(t, int64(70), res.RowsAffected)

	// WHEN querying orphan requests
	recs, err = repo.QueryOrphanRequests(10000, 0, 30)
	// THEN it should match expected count
	require.NoError(t, err)
	require.Equal(t, 30, len(recs))

	for _, rec := range recs {
		// WHEN restarting orphan requests
		err = repo.Restart(common.NewQueryContextFromIDs(rec.UserID, ""), rec.ID)
		require.NoError(t, err)
		// AND incrementing schedule attempts
		err = repo.IncrementScheduleAttempts(
			rec.ID, time.Duration(rand.Intn(100))*time.Second, rand.Intn(100), "blah")

		// THEN it should not fail
		require.NoError(t, err)
	}
}

// Test fix orphan jobs
func Test_ShouldFixOrphanJobRequests(t *testing.T) {
	now := time.Now().Unix()
	// GIVEN a job-resource repository
	repo, err := NewTestJobRequestRepository()
	require.NoError(t, err)
	repo.Clear()
	qc, err := NewTestQC()
	require.NoError(t, err)
	states := []string{"PENDING", "READY", "STARTED", "EXECUTING", "FAILED", "COMPLETED", "CANCELLED"}

	// GIVEN a set of test requests in the database
	for i := 0; i < 10; i++ {
		job, err := SaveTestJobDefinition(qc, fmt.Sprintf("job-for-stale-%v-%v", i, now), "")
		require.NoError(t, err)
		for j, state := range states {
			req, err := types.NewJobRequestFromDefinition(job)
			require.NoError(t, err)
			req.OrganizationID = fmt.Sprintf("org_%d", i)
			req.UserID = fmt.Sprintf("user_%d_%d", i, j)
			_, err = repo.Save(req)
			require.NoError(t, err)

			// updating state
			req.JobState = common.NewRequestState(state)
			_, err = repo.Save(req)
			require.NoError(t, err)
		}
	}

	// WHEN querying jobs
	_, total, err := repo.Query(
		common.NewQueryContext(nil, ""),
		map[string]interface{}{}, 0, 10000, []string{})
	// THEN it should match expected count
	require.NoError(t, err)
	require.Equal(t, int64(70), total)

	// marking updated-at of job-requests to epoch time so that those can be re-queued
	res := repo.db.Exec("update formicary_job_requests set updated_at = '2020-12-16 00:00:00.000'")
	require.Equal(t, int64(70), res.RowsAffected)
	total, err = repo.RequeueOrphanRequests(30)
	// THEN it should match expected count
	require.NoError(t, err)
	require.Equal(t, int64(30), total)
}

// Getting job count by days should return counts of rows by job-types and error-codes
func Test_ShouldGetJobCountsByDaysWithDifferentJobTypesStatusesAndErrorCodes(t *testing.T) {
	// GIVEN a job-resource repository
	repo, err := NewTestJobRequestRepository()
	require.NoError(t, err)
	repo.Clear()
	qc, err := NewTestQC()
	require.NoError(t, err)

	// WHEN counting an empty database
	params := make(map[string]interface{})
	total, err := repo.Count(qc, params)
	// THEN it should match 0 count
	require.NoError(t, err)
	require.Equal(t, int64(0), total)

	// WHEN getting counts from empty database
	counts, err := repo.JobCountsByDays(common.NewQueryContext(nil, ""), 10)
	// THEN it should match 0 count
	require.NoError(t, err)
	require.Equal(t, 0, len(counts))

	require.NoError(t, SaveTestJobRequests(qc, "job-counts"))

	params["job_state"] = "DONE"

	// WHEN counting by DONE state
	total, err = repo.Count(common.NewQueryContext(nil, ""), params)
	// THEN it should match expected count
	require.NoError(t, err)
	require.Equal(t, int64(200), total)

	params["job_state"] = "DONE"
	// WHEN querying by DONE state
	_, total, err = repo.Query(
		common.NewQueryContext(nil, ""),
		params, 0, 1000, make([]string, 0))
	// THEN it should match expected count
	require.NoError(t, err)
	require.Equal(t, int64(200), total)

	// WHEN getting job counts by days
	counts, err = repo.JobCountsByDays(common.NewQueryContext(nil, ""), 10)
	// THEN it should match expected count
	require.NoError(t, err)
	require.Equal(t, 10, len(counts))

	// WHEN counting by org-id
	counts, err = repo.JobCountsByDays(common.NewQueryContextFromIDs("", "org_0"), 10)
	// THEN it should not fail
	require.NoError(t, err)
}

// Getting job count should return counts of rows by job-types and error-codes
func Test_ShouldGetJobCountsWithDifferentJobTypesStatusesAndErrorCodes(t *testing.T) {
	start := time.Now()
	end := start.AddDate(0, 0, 1)
	// GIVEN a job-resource repository
	repo, err := NewTestJobRequestRepository()
	require.NoError(t, err)
	repo.Clear()
	qc, err := NewTestQC()
	require.NoError(t, err)
	params := make(map[string]interface{})

	// WHEN counting an empty database
	total, err := repo.Count(qc, params)
	// THEN it should match 0count
	require.NoError(t, err)
	require.Equal(t, int64(0), total)

	// WHEN getting counts from empty database
	counts, err := repo.JobCounts(common.NewQueryContext(nil, ""), start, end)
	// THEN it should match 0count
	require.NoError(t, err)
	require.Equal(t, 0, len(counts))

	require.NoError(t, SaveTestJobRequests(qc, "job-counts-with-status"))

	params["job_state"] = "DONE"

	// WHEN querying jobs
	_, total, err = repo.Query(qc, params, 0, 100, make([]string, 0))
	// THEN it should match expected count
	require.NoError(t, err)
	require.Equal(t, int64(200), total)

	// WHEN counting jobs
	total, err = repo.Count(qc, params)
	// THEN it should match expected count
	require.NoError(t, err)
	require.Equal(t, int64(300), total)

	start = time.Unix(0, 0)
	end = time.Now().Add(24 * time.Hour)

	// WHEN getting counts
	_, err = repo.JobCounts(common.NewQueryContext(nil, ""), start, end)
	// THEN it should not fail
	require.NoError(t, err)

	// WHEN getting counts by org
	_, err = repo.JobCounts(common.NewQueryContextFromIDs("", "org_0"), start, end)
	// THEN it should not fail
	require.NoError(t, err)
}
