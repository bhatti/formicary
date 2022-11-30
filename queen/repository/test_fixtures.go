package repository

import (
	"fmt"
	"github.com/twinj/uuid"
	"math/rand"
	"plexobject.com/formicary/internal/acl"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/types"
	"time"
)

// NewTestQC creates a test user/org for testing
func NewTestQC() (*common.QueryContext, error) {
	orgRepository, err := NewTestOrganizationRepository()
	if err != nil {
		return nil, err
	}
	userRepository, err := NewTestUserRepository()

	org := common.NewOrganization("", uuid.NewV4().String(), uuid.NewV4().String())
	org, err = orgRepository.Create(common.NewQueryContext(nil, ""), org)
	if err != nil {
		return nil, err
	}

	user := common.NewUser(org.ID, uuid.NewV4().String(), uuid.NewV4().String(), uuid.NewV4().String()+"@formicary.io", acl.NewRoles(""))
	saved, err := userRepository.Create(user)
	if err != nil {
		return nil, err
	}
	saved.Organization = org
	return common.NewQueryContext(saved, ""), nil
}

// NewTestJobDefinition creates new job-definition
func NewTestJobDefinition(user *common.User, name string) *types.JobDefinition {
	job := types.NewJobDefinition("io.formicary.test." + name)
	job.UserID = user.ID
	job.OrganizationID = user.OrganizationID
	_, _ = job.AddVariable("jk1", "jv1")
	_, _ = job.AddVariable("jk2", map[string]int{"a": 1, "b": 2})
	_, _ = job.AddVariable("jk3", "jv3")
	_, _ = job.AddConfig("name", "value", false)
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
		task.Method = common.Kubernetes
		if i%2 == 1 {
			task.AlwaysRun = true
		}
		job.AddTask(task)
	}
	job.UpdateRawYaml()

	return job
}

// SaveTestJobDefinition saves job-definition in test database
func SaveTestJobDefinition(qc *common.QueryContext, name string, cronTrigger string) (*types.JobDefinition, error) {
	repo, err := NewTestJobDefinitionRepository()
	if err != nil {
		return nil, fmt.Errorf("unexpected error %w while creating a job repository", err)
	}
	job := NewTestJobDefinition(qc.User, name)
	job.CronTrigger = cronTrigger
	return repo.Save(qc, job)
}

// Saving job-execution for given request
func saveTestJobExecutionForRequest(req *types.JobRequest, job *types.JobDefinition) (*types.JobExecution, error) {
	repo, err := NewTestJobExecutionRepository()
	if err != nil {
		return nil, err
	}
	jobExec := types.NewJobExecution(req.ToInfo())
	_, _ = jobExec.AddContext("jk1", "jv1")
	_, _ = jobExec.AddContext("jk2", map[string]int{"a": 1, "b": 2})
	_, _ = jobExec.AddContext("jk3", "jv3")
	for _, t := range job.Tasks {
		task := jobExec.AddTask(t)
		_, _ = task.AddContext("tk1", "v1")
		_, _ = task.AddContext("tk2", []string{"i", "j", "k"})
	}
	return repo.Save(jobExec)
}

// SaveTestJobRequests adds test job-requests
func SaveTestJobRequests(qc *common.QueryContext, jobName string) error {
	repo, err := NewTestJobRequestRepository()
	if err != nil {
		return err
	}
	errorCodes := []string{"ERR_CODE1", "ERR_CODE2"}
	// GIVEN a set of test job requests in the database
	for i := 0; i < 20; i++ {
		job, err := SaveTestJobDefinition(qc, fmt.Sprintf("%s-%v", jobName, i), "")
		if err != nil {
			return err
		}
		for j := 0; j < 15; j++ {
			req, err := types.NewJobRequestFromDefinition(job)
			if err != nil {
				return err
			}
			req.OrganizationID = qc.GetOrganizationID()
			req.UserID = qc.GetUserID()
			req.JobPriority = rand.Intn(100)
			_, err = repo.Save(qc, req)
			if err != nil {
				return err
			}
			if j < 5 {
				if j%2 == 0 {
					req.JobState = common.PENDING
				} else {
					req.JobState = common.READY
				}
			} else if j%2 == 0 {
				req.JobState = common.COMPLETED
			} else {
				req.JobState = common.FAILED
				req.ErrorCode = errorCodes[i%len(errorCodes)]
			}
			req.UserID = qc.User.ID
			req.OrganizationID = qc.User.OrganizationID

			// updating state and error code
			_, err = repo.Save(qc, req)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// NewTestJobExecution creates a test job execution
func NewTestJobExecution(qc *common.QueryContext, name string) (*types.JobRequest, *types.JobExecution, error) {
	job, err := SaveTestJobDefinition(qc, name, "")
	if err != nil {
		return nil, nil, err
	}
	jobRequestRepo, err := NewTestJobRequestRepository()
	if err != nil {
		return nil, nil, err
	}
	req, err := types.NewJobRequestFromDefinition(job)
	if err != nil {
		return nil, nil, err
	}
	_, _ = req.AddParam("jk1", "jv1")
	_, _ = req.AddParam("jk2", map[string]int{"a": 1, "b": 2})
	_, _ = req.AddParam("jk3", true)
	_, _ = req.AddParam("jk4", 50.10)
	_, err = jobRequestRepo.Save(qc, req)
	if err != nil {
		return nil, nil, err
	}
	now := time.Now()
	jobExec := types.NewJobExecution(req.ToInfo())
	_, _ = jobExec.AddContext("jk1", "jv1")
	_, _ = jobExec.AddContext("jk2", map[string]int{"a": 1, "b": 2})
	_, _ = jobExec.AddContext("jk3", "jv3")
	for i, t := range job.Tasks {
		task := jobExec.AddTask(t)
		task.StartedAt = now.Add(time.Duration(i) * time.Second)
		endedAt := now.Add(time.Duration(i+100) * time.Second)
		task.EndedAt = &endedAt
		_, _ = task.AddContext("tk1", "v1")
		_, _ = task.AddContext("tk2", []string{"i", "j", "k"})
	}
	return req, jobExec, nil
}

func newTestArtifact(user *common.User, expire time.Time) *common.Artifact {
	art := common.NewArtifact(
		"bucket",
		"name",
		"group",
		"kind",
		123,
		"sha",
		54)
	art.ID = uuid.NewV4().String()
	art.AddMetadata("n1", "v1")
	art.AddMetadata("n2", "v2")
	art.AddMetadata("n3", "1")
	art.AddTag("t1", "v1")
	art.AddTag("t2", "v2")
	art.ExpiresAt = expire
	art.JobExecutionID = "job"
	art.TaskExecutionID = "task"
	art.Bucket = "test"
	art.UserID = user.ID
	art.OrganizationID = user.OrganizationID
	return art
}

// SaveFile artifacts
func saveTestArtifacts(user *common.User, jobExec *types.JobExecution) error {
	artifactRepository, err := NewTestArtifactRepository()
	if err != nil {
		return err
	}
	for _, task := range jobExec.Tasks {
		for i := 0; i < 5; i++ {
			art := newTestArtifact(user, time.Now().Add(time.Hour))
			art.JobRequestID = jobExec.JobRequestID
			art.JobExecutionID = jobExec.ID
			art.TaskExecutionID = task.ID
			_, err := artifactRepository.Save(art)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
