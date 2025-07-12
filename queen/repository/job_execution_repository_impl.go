package repository

import (
	"fmt"
	"sort"
	"time"

	common "plexobject.com/formicary/internal/types"

	"github.com/oklog/ulid/v2"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"plexobject.com/formicary/queen/types"
)

// JobExecutionRepositoryImpl implements JobExecutionRepository using gorm O/R mapping
type JobExecutionRepositoryImpl struct {
	db     *gorm.DB
	dbType string
}

// NewJobExecutionRepositoryImpl creates new instance for job-execution-repository
func NewJobExecutionRepositoryImpl(db *gorm.DB, dbType string) (*JobExecutionRepositoryImpl, error) {
	return &JobExecutionRepositoryImpl{db: db, dbType: dbType}, nil
}

// Get method finds JobExecution by id
func (jer *JobExecutionRepositoryImpl) Get(
	id string) (*types.JobExecution, error) {
	if id == "" {
		return nil, common.NewValidationError(
			fmt.Errorf("id is not specified for job-execution"))
	}
	var job types.JobExecution
	res := jer.db.Preload("Tasks", "active = ?", true).
		Preload("Contexts").
		Preload("Tasks.Contexts").
		Preload("Tasks.Artifacts").
		Where("id = ?", id).
		Where("active = ?", true).
		First(&job)
	if res.Error != nil {
		return nil, common.NewNotFoundError(res.Error)
	}
	if err := job.AfterLoad(); err != nil {
		return nil, common.NewValidationError(err)
	}
	// sort tasks by update time
	sort.Slice(job.Tasks, func(i, j int) bool { return job.Tasks[i].TaskOrder < job.Tasks[j].TaskOrder })
	sort.Slice(job.Contexts, func(i, j int) bool { return job.Contexts[i].Name < job.Contexts[j].Name })
	for _, t := range job.Tasks {
		sort.Slice(t.Contexts, func(i, j int) bool { return t.Contexts[i].Name < t.Contexts[j].Name })
	}
	return &job, nil
}

// ResetStateToReady resets state to ready
func (jer *JobExecutionRepositoryImpl) ResetStateToReady(id string) error {
	return jer.db.Transaction(func(tx *gorm.DB) error {
		updates := map[string]interface{}{"job_state": common.READY, "updated_at": time.Now()}
		// check in-clause
		res := tx.Model(&types.JobExecution{}).Where("id = ?", id).
			Where("job_state NOT IN ?", common.TerminalStates).
			Updates(updates)
		if res.Error != nil {
			return common.NewNotFoundError(res.Error)
		}
		if res.RowsAffected != 1 {
			return common.NewNotFoundError(
				fmt.Errorf("failed to update job execution state to READY with id %v", id))
		}

		return nil
	})
}

// calcCost
func (jer *JobExecutionRepositoryImpl) calcCost(
	id string) (sum int64) {
	var sql string
	if jer.dbType == "sqlite" {
		sql = "SELECT sum((count_services+1) * (ended_at-started_at)) from formicary_task_executions where job_execution_id = ?"
	} else {
		sql = "SELECT sum((count_services+1) * (ended_at-started_at)) from formicary_task_executions where job_execution_id = ?"
	}

	args := []interface{}{id}
	err := jer.db.Raw(sql, args...).Row().Scan(&sum)
	if err != nil {
		log.WithFields(log.Fields{
			"Component": "JobExecutionRepositoryImpl",
			"ID":        id,
			"Error":     err,
		}).
			Warnf("failed to calculate cost of job-execution")
	}
	return
}

// GetResourceUsageByOrgUser - Finds usage between time by user and organization
func (jer *JobExecutionRepositoryImpl) GetResourceUsageByOrgUser(
	ranges []types.DateRange,
	limit int) ([]types.ResourceUsage, error) {
	res := make([]types.ResourceUsage, 0)
	if ranges == nil || len(ranges) == 0 {
		return res, nil
	}
	sql := "SELECT user_id, organization_id, COUNT(*) as count, SUM(cpu_secs) as value FROM formicary_job_executions WHERE updated_at >= ? AND updated_at <= ? group by user_id, organization_id order by value desc limit ?"
	for _, r := range ranges {
		rows, err := jer.db.Raw(sql, r.StartDate, r.EndDate, limit).Rows()
		if err != nil {
			return nil, err
		}
		defer func() {
			_ = rows.Close()
		}()
		for rows.Next() {
			usage := types.ResourceUsage{}
			if err = jer.db.ScanRows(rows, &usage); err != nil {
				return nil, err
			}
			usage.ResourceType = types.CPUResource
			usage.StartDate = r.StartDate
			usage.EndDate = r.EndDate
			usage.ValueUnit = "seconds"
			res = append(res, usage)
		}
	}
	return res, nil
}

// GetResourceUsage - Finds usage between time
func (jer *JobExecutionRepositoryImpl) GetResourceUsage(
	qc *common.QueryContext,
	ranges []types.DateRange) ([]types.ResourceUsage, error) {
	res := make([]types.ResourceUsage, 0)
	if ranges == nil || len(ranges) == 0 {
		return res, nil
	}
	orgSQL, orgArg := qc.AddOrgUserWhereSQL(true)
	sql := "SELECT COUNT(*) as count, SUM(cpu_secs) as value FROM formicary_job_executions WHERE updated_at >= ? AND updated_at <= ? AND " +
		orgSQL
	for _, r := range ranges {
		rows, err := jer.db.Raw(sql, r.StartDate, r.EndDate, orgArg).Rows()
		if err != nil {
			return nil, err
		}
		defer func() {
			_ = rows.Close()
		}()
		for rows.Next() {
			usage := types.ResourceUsage{}
			if err = jer.db.ScanRows(rows, &usage); err != nil {
				return nil, err
			}
			usage.ResourceType = types.CPUResource
			usage.UserID = qc.GetUserID()
			usage.OrganizationID = qc.GetOrganizationID()
			usage.StartDate = r.StartDate
			usage.EndDate = r.EndDate
			usage.ValueUnit = "seconds"
			res = append(res, usage)
		}
	}
	return res, nil
}

// FinalizeJobRequestAndExecutionState updates final state of job-execution and job-request
func (jer *JobExecutionRepositoryImpl) FinalizeJobRequestAndExecutionState(
	id string,
	oldState common.RequestState,
	newState common.RequestState,
	errorMessage string,
	errorCode string,
	cpuSecs int64,
	scheduleDelay time.Duration,
	retried int,
) error {
	if !newState.CanFinalize() {
		return common.NewValidationError(
			fmt.Errorf("new state %s is not terminal", newState))
	}
	return jer.db.Transaction(func(tx *gorm.DB) error {
		// saving job request along with job-execution in a same transaction
		updates := map[string]interface{}{
			"job_state":     newState,
			"error_message": errorMessage,
			"error_code":    errorCode,
			"retried":       retried,
			"updated_at":    time.Now(),
		}
		if newState == common.PAUSED { // not common.MANUAL_APPROVAL_REQUIRED {
			if scheduleDelay > 0 {
				updates["scheduled_at"] = time.Now().Add(scheduleDelay)
			}
		}
		res := tx.Model(&types.JobRequest{}).
			Where("job_execution_id = ?", id).
			Where("job_state = ?", oldState).Updates(updates)
		if res.Error != nil {
			return common.NewNotFoundError(res.Error)
		}
		if res.RowsAffected != 1 {
			rows := res.RowsAffected
			var req types.JobRequest
			res = tx.Where("job_execution_id = ?", id).First(&req)
			return common.NewNotFoundError(
				fmt.Errorf("failed to update job request state from %s to %s -- db-state '%s', rows %d",
					oldState, newState, req.JobState, rows))
		}

		// check in-clause
		res = tx.Model(&types.JobExecution{}).Where("id = ?", id).
			Where("job_state = ?", oldState).
			Where("job_state NOT IN ?", common.TerminalStates).
			Updates(map[string]interface{}{
				"job_state":     newState,
				"error_message": errorMessage,
				"error_code":    errorCode,
				"ended_at":      time.Now(),
				"cpu_secs":      cpuSecs,
				"updated_at":    time.Now(),
			})
		if res.Error != nil {
			return common.NewNotFoundError(res.Error)
		}
		if res.RowsAffected != 1 {
			return common.NewNotFoundError(
				fmt.Errorf("failed to update job execution state from %v to %v with id %v",
					oldState, newState, id))
		}

		return nil
	})
}

// UpdateJobRequestAndExecutionState updates intermediate state of job-execution and job-request
func (jer *JobExecutionRepositoryImpl) UpdateJobRequestAndExecutionState(
	id string,
	oldState common.RequestState,
	newState common.RequestState, currentTask string) error {
	if newState.IsTerminal() { // TODO possibly add PAUSED
		return common.NewValidationError(
			fmt.Errorf("new state %s cannot be terminal", newState))
	}
	return jer.db.Transaction(func(tx *gorm.DB) error {
		updates := map[string]interface{}{"job_state": newState, "updated_at": time.Now()}
		if newState.IsTerminal() {
			updates["current_task"] = ""
		} else if currentTask != "" {
			updates["current_task"] = currentTask
		}
		// saving job request along with job-execution in a same transaction
		res := tx.Model(&types.JobRequest{}).
			Where("job_execution_id = ?", id).
			Where("job_state = ?", oldState).Updates(updates)
		if res.Error != nil {
			return common.NewNotFoundError(res.Error)
		}
		if res.RowsAffected != 1 {
			var req types.JobRequest
			_ = tx.Model(&types.JobRequest{}).
				Where("job_execution_id = ?", id).First(&req)
			return common.NewNotFoundError(
				fmt.Errorf("failed to update job request state with job_execution_id %s from %s to %s because it has %s state", id, oldState, newState, req.JobState))
		}

		// check in-clause
		res = tx.Model(&types.JobExecution{}).Where("id = ?", id).
			Where("job_state = ?", oldState).
			Where("job_state NOT IN ?", common.TerminalStates).
			Updates(updates)
		if res.Error != nil {
			return common.NewNotFoundError(res.Error)
		}
		if res.RowsAffected != 1 {
			return common.NewNotFoundError(
				fmt.Errorf("failed to update job execution state from %v to %v with id %v",
					oldState, newState, id))
		}

		return nil
	})
}

// UpdateJobContext updates context of job-execution
func (jer *JobExecutionRepositoryImpl) UpdateJobContext(
	id string,
	contexts []*types.JobExecutionContext) error {
	return jer.db.Transaction(func(tx *gorm.DB) error {
		err := jer.createJobContexts(tx, id, contexts)
		if err != nil {
			return err
		}
		return nil
	})
}

// UpdateTaskState sets state of task-execution
func (jer *JobExecutionRepositoryImpl) UpdateTaskState(
	id string,
	oldState common.RequestState,
	newState common.RequestState) error {
	return jer.db.Transaction(func(tx *gorm.DB) error {
		var task types.TaskExecution
		updates := map[string]interface{}{"task_state": newState, "updated_at": time.Now()}
		// check in-clause
		res := tx.Model(&task).Where("id = ?", id).
			Where("task_state = ?", oldState).
			Where("task_state NOT IN ?", common.TerminalStates).
			Updates(updates)
		if res.Error != nil {
			return common.NewNotFoundError(res.Error)
		}
		if res.RowsAffected != 1 {
			return common.NewNotFoundError(
				fmt.Errorf("failed to update task execution state to %v with id %v",
					newState, id))
		}
		return nil
	})
}

// GetJobsPendingManualApproval get jobs pending manual approval
func (jer *JobExecutionRepositoryImpl) GetJobsPendingManualApproval(
	qc *common.QueryContext,
	limit int,
	offset int) ([]*types.JobExecution, error) {

	jobs := make([]*types.JobExecution, 0)

	tx := jer.db.Preload("Tasks", "task_state = ?", common.MANUAL_APPROVAL_REQUIRED).
		Preload("Contexts").
		Where("job_state = ? AND active = ?", common.MANUAL_APPROVAL_REQUIRED, true).
		Order("updated_at ASC").
		Limit(limit).Offset(offset)

	// Add organization/user filtering
	if !qc.IsAdmin() {
		if qc.HasOrganization() {
			tx = tx.Where("organization_id = ?", qc.GetOrganizationID())
		} else if qc.GetUserID() != "" {
			tx = tx.Where("user_id = ?", qc.GetUserID())
		}
	}

	res := tx.Find(&jobs)
	if res.Error != nil {
		return nil, res.Error
	}

	for _, job := range jobs {
		if err := job.AfterLoad(); err != nil {
			log.WithFields(log.Fields{
				"Component": "JobExecutionRepositoryImpl",
				"JobID":     job.ID,
				"Error":     err,
			}).Warn("Failed to load job execution after retrieval")
		}
	}

	return jobs, nil
}

// ResumeFromManualApproval resumes job execution after manual approval
// This updates job-execution state and can be used for more complex resume logic if needed
func (jer *JobExecutionRepositoryImpl) ResumeFromManualApproval(request types.ReviewTaskRequest) error {
	if request.RequestID == "" {
		return common.NewValidationError(fmt.Errorf("job request is not defined"))
	}
	if request.TaskType == "" {
		return common.NewValidationError(fmt.Errorf("task type is not defined"))
	}
	if request.Status == "" {
		return common.NewValidationError(fmt.Errorf("status is not defined"))
	}
	if request.Status != common.APPROVED && request.Status != common.REJECTED {
		return common.NewValidationError(fmt.Errorf("status is not valid: %s", request.Status))
	}

	var taskState common.RequestState
	if request.Status == common.COMPLETED {
		taskState = common.COMPLETED
	} else {
		taskState = common.FAILED
	}
	return jer.db.Transaction(func(tx *gorm.DB) error {
		now := time.Now()

		// 1. Update the approved task execution
		taskRes := tx.Model(&types.TaskExecution{}).
			Where("job_execution_id = ? AND task_type = ? AND task_state = ?",
				request.ExecutionID, request.TaskType, common.MANUAL_APPROVAL_REQUIRED).
			Updates(map[string]interface{}{
				"task_state":         taskState,
				"exit_code":          request.Status,
				"exit_message":       fmt.Sprintf("Manually %s by %s", request.Status, request.ReviewedBy),
				"manual_reviewed_by": request.ReviewedBy,
				"manual_reviewed_at": now,
				"ended_at":           now,
				"updated_at":         now,
			})

		if taskRes.Error != nil {
			return taskRes.Error
		}

		// 2. Update job execution to PENDING state for scheduler pickup
		jobRes := tx.Model(&types.JobExecution{}).
			Where("id = ? AND job_state = ?", request.ExecutionID, common.MANUAL_APPROVAL_REQUIRED).
			Updates(map[string]interface{}{
				"job_state":     common.PENDING,
				"error_code":    "",
				"error_message": "",
				"updated_at":    now,
			})

		if jobRes.Error != nil {
			return jobRes.Error
		}

		if jobRes.RowsAffected != 1 {
			return fmt.Errorf("failed to update job execution state for job %s", request.ExecutionID)
		}

		log.WithFields(log.Fields{
			"Component":      "JobExecutionRepositoryImpl",
			"JobExecutionID": request.ExecutionID,
			"TaskType":       request.TaskType,
			"ReviewedBy":     request.ReviewedBy,
		}).Info("Job execution resumed from manual approval")

		return nil
	})
}

// SaveTask saves task execution
func (jer *JobExecutionRepositoryImpl) SaveTask(
	task *types.TaskExecution) (*types.TaskExecution, error) {
	err := task.ValidateBeforeSave()
	if err != nil {
		return nil, common.NewValidationError(err)
	}
	now := time.Now()
	err = jer.db.Transaction(func(tx *gorm.DB) error {
		var res *gorm.DB
		newTask := false
		if task.ID != "" {
			task.UpdatedAt = time.Now()
			if task.TaskState.CanFinalize() {
				task.EndedAt = &now
			}
		} else {
			task.StartedAt = now
			task.UpdatedAt = now
			task.ID = ulid.Make().String()
			newTask = true
		}
		task.Active = true
		contextIDS := make([]string, 0)
		for _, c := range task.Contexts {
			if c.ID != "" {
				contextIDS = append(contextIDS, c.ID)
			}
		}
		if newTask {
			res = tx.Omit("Contexts").Create(task)
		} else {
			// Cannot change terminal state
			// check in-clause and omit
			tx.Where("task_execution_id = ? AND id NOT IN ?", task.ID, contextIDS).
				Delete(types.TaskExecutionContext{})
			res = tx.
				Where("id = ? AND task_state NOT IN ?", task.ID, common.TerminalStates).
				Omit("Contexts").
				Save(task)
		}
		if res.Error != nil {
			return common.NewNotFoundError(res.Error)
		}
		if res.RowsAffected != 1 {
			return common.NewNotFoundError(
				fmt.Errorf("failed to update task rows %v", task.String()))
		}
		// Replace causes deadlock
		//tx.Model(task).Association("Contexts").Replace(task.Contexts)
		for _, c := range task.Contexts {
			c.TaskExecutionID = task.ID
			if c.ID == "" {
				c.ID = ulid.Make().String()
				res = tx.Create(c)
			} else {
				res = tx.Save(c)
			}
			if res.Error != nil {
				return res.Error
			}
		}
		return nil
	})
	return task, err
}

// Save persists job-execution
func (jer *JobExecutionRepositoryImpl) Save(
	jobExec *types.JobExecution) (*types.JobExecution, error) {
	if jobExec == nil {
		return nil, common.NewValidationError(fmt.Errorf("job-execution is nil"))
	}
	err := jobExec.ValidateBeforeSave()
	now := time.Now()
	if err != nil {
		return nil, common.NewValidationError(err)
	}
	if jobExec.StartedAt.IsZero() {
		jobExec.StartedAt = now
	}
	err = jer.db.Transaction(func(tx *gorm.DB) error {
		var res *gorm.DB
		newJob := false
		if jobExec.ID != "" {
			jobExec.UpdatedAt = now
			jer.clearOrphanJobTasks(tx, jobExec)
			jer.clearOrphanJobContexts(tx, jobExec.ID, jobExec.Contexts)
			if log.IsLevelEnabled(log.DebugLevel) {
				log.WithFields(log.Fields{
					"Component":    "JobExecutionRepositoryImpl",
					"jobExecution": jobExec.String(),
				}).
					Debug("saving job-execution...")
			}
		} else {
			jobExec.UpdatedAt = now
			jobExec.ID = ulid.Make().String()
			if log.IsLevelEnabled(log.DebugLevel) {
				log.WithFields(log.Fields{
					"Component":    "JobExecutionRepositoryImpl",
					"jobExecution": jobExec.String(),
				}).
					Debug("creating job-execution...")
			}
			newJob = true
		}
		jobExec.Active = true
		for _, c := range jobExec.Contexts {
			if c.ID == "" {
				c.ID = ulid.Make().String()
			}
			c.JobExecutionID = jobExec.ID
		}
		for _, t := range jobExec.Tasks {
			t.JobExecutionID = jobExec.ID
			t.Active = true
		}
		if jobExec.JobState.CanFinalize() {
			jobExec.CPUSecs = jobExec.ExecutionCostSecs()
			if !newJob {
				cpuSecs := jer.calcCost(jobExec.ID)
				if cpuSecs > 0 && cpuSecs > jobExec.CPUSecs {
					jobExec.CPUSecs = cpuSecs
				}
			}
			jobExec.EndedAt = &now
		}
		if newJob {
			res = tx.Omit("Tasks", "Contexts").Create(jobExec)
		} else {
			res = tx.Omit("Tasks", "Contexts").Save(jobExec)
		}
		if res.Error != nil {
			return res.Error
		}
		err = tx.Model(jobExec).Association("Contexts").Replace(jobExec.Contexts)
		if err != nil {
			return err
		}
		for _, t := range jobExec.Tasks {
			newTask := false
			if t.ID == "" {
				t.ID = ulid.Make().String()
				newTask = true
			}
			for _, c := range t.Contexts {
				if c.ID == "" {
					c.ID = ulid.Make().String()
				}
				c.TaskExecutionID = t.ID
			}
			if newTask {
				res = tx.Omit("Contexts").Create(t)
			} else {
				res = tx.Omit("Contexts").Save(t)
			}
			if res.Error != nil {
				return res.Error
			}
			err = tx.Model(t).Association("Contexts").Replace(t.Contexts)
			if err != nil {
				return err
			}
		}
		return nil
	})
	return jobExec, err
}

// DeleteTask job-execution
func (jer *JobExecutionRepositoryImpl) DeleteTask(
	id string) error {
	var task types.TaskExecution
	res := jer.db.Model(&task).
		Where("id = ?", id).
		Where("active = ?", true).
		Updates(map[string]interface{}{"active": false, "updated_at": time.Now()})
	if res.Error != nil {
		return common.NewNotFoundError(res.Error)
	}
	if res.RowsAffected != 1 {
		return common.NewNotFoundError(
			fmt.Errorf("failed to delete task execution with id %v, rows %v",
				id, res.RowsAffected))
	}
	return nil
}

// Delete job-execution
func (jer *JobExecutionRepositoryImpl) Delete(
	id string) error {
	old, err := jer.Get(id)
	if err != nil {
		return err
	}
	return jer.db.Transaction(func(tx *gorm.DB) error {
		for _, t := range old.Tasks {
			var task types.TaskExecution
			res := jer.db.Model(&task).
				Where("id = ?", t.ID).
				Where("active = ?", true).
				Updates(map[string]interface{}{"active": false, "updated_at": time.Now()})
			if res.Error != nil {
				return common.NewNotFoundError(res.Error)
			}
			if res.RowsAffected != 1 {
				return common.NewNotFoundError(
					fmt.Errorf("failed to delete task execution with id %v, rows %v",
						t.ID, res.RowsAffected))
			}
		}
		var job types.JobExecution
		res := jer.db.Model(&job).
			Where("id = ?", id).
			Where("active = ?", true).
			Updates(map[string]interface{}{"active": false, "updated_at": time.Now()})
		if res.Error != nil {
			return common.NewNotFoundError(
				fmt.Errorf("failed to delete job execution with id %v due to %v", id, res.Error))
		}
		if res.RowsAffected != 1 {
			return common.NewNotFoundError(
				fmt.Errorf("failed to delete job execution with id %v, rows %v", id, res.RowsAffected))
		}
		return nil
	})
}

// //////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////a
// loadTaskIDsByJobID - loads tasks by job-id
func (jer *JobExecutionRepositoryImpl) loadTasksIDByJobID(
	tx *gorm.DB,
	jobID string) (map[string]bool, error) {
	tasks := make([]*types.TaskExecution, 0)
	taskIDs := make(map[string]bool)

	res := tx.Select("id").Where("job_execution_id = ?", jobID).Find(&tasks)
	if res.Error != nil {
		return nil, res.Error
	}
	for _, t := range tasks {
		taskIDs[t.ID] = true
	}
	return taskIDs, nil
}

// clearOrphanJobTasks -- tasks and their contexts that are no longer active
func (jer *JobExecutionRepositoryImpl) clearOrphanJobTasks(
	tx *gorm.DB,
	job *types.JobExecution) {
	oldTaskIDs, err := jer.loadTasksIDByJobID(tx, job.ID)
	if err != nil {
		log.WithFields(log.Fields{
			"Component": "JobExecutionRepositoryImpl",
			"Error":     err,
		}).Warnf("failed to load old tasks")
		return
	}

	deletedTaskIDs := make([]string, 0)
	taskIDs := make([]string, len(job.Tasks))

	for i, t := range job.Tasks {
		if oldTaskIDs[t.ID] == false {
			deletedTaskIDs = append(deletedTaskIDs, t.ID)
		}
		taskIDs[i] = t.ID
	}

	// delete all task contexts that are deleted
	// check in-clause
	tx.Where("task_execution_id IN ?", deletedTaskIDs).Delete(types.TaskExecutionContext{})

	// Delete all tasks that are not in the list
	// check in-clause
	tx.Where("id IN ? AND job_execution_id = ?", deletedTaskIDs, job.ID).Delete(types.TaskExecution{})

	for _, t := range job.Tasks {
		contextIDs := make([]string, len(t.Contexts))
		for i, c := range t.Contexts {
			contextIDs[i] = c.ID
		}
		// check in-clause
		tx.Where("id NOT IN ? AND task_execution_id = ?", contextIDs, t.ID).Delete(types.TaskExecutionContext{})
	}
}

// clearOrphanJobContexts -- contexts that are no longer active
func (jer *JobExecutionRepositoryImpl) clearOrphanJobContexts(
	tx *gorm.DB,
	id string,
	newContexts []*types.JobExecutionContext) {
	contextIDs := make([]string, len(newContexts))
	for i, c := range newContexts {
		contextIDs[i] = c.ID
	}
	// check in-clause
	tx.Where("id NOT IN ? AND job_execution_id = ?", contextIDs, id).Delete(types.JobExecutionContext{})
}

func (jer *JobExecutionRepositoryImpl) createJobContexts(
	tx *gorm.DB,
	id string,
	contexts []*types.JobExecutionContext) error {
	//jer.clearOrphanJobContexts(tx, id, contexts)
	for _, c := range contexts {
		c.JobExecutionID = id
		var res *gorm.DB
		if c.ID == "" {
			c.ID = ulid.Make().String()
			res = tx.Create(c) // TODO check deadlock
		} else {
			res = tx.Save(c)
		}
		if res.Error != nil {
			return res.Error
		}
	}
	return nil
}

// Query finds matching job-execution by parameters
func (jer *JobExecutionRepositoryImpl) Query(
	params map[string]interface{},
	page int,
	pageSize int,
	order []string) (jobs []*types.JobExecution, totalRecords int64, err error) {
	jobs = make([]*types.JobExecution, 0)
	tx := jer.db.Preload("Tasks").
		Preload("Contexts").
		Preload("Tasks.Contexts").
		Limit(pageSize).
		Offset(page*pageSize).
		Where("active = ?", true)

	tx = addQueryParamsWhere(params, tx)
	for _, ord := range order {
		tx = tx.Order(ord)
	}
	res := tx.Find(&jobs)
	if res.Error != nil {
		return nil, 0, err
	}
	for _, j := range jobs {
		if err = j.AfterLoad(); err != nil {
			return
		}
	}
	totalRecords, _ = jer.Count(params)
	return
}

// Count counts records by query
func (jer *JobExecutionRepositoryImpl) Count(
	params map[string]interface{}) (totalRecords int64, err error) {
	tx := jer.db.Model(&types.JobExecution{}).Where("active = ?", true)
	tx = addQueryParamsWhere(params, tx)
	res := tx.Count(&totalRecords)
	if res.Error != nil {
		return 0, err
	}
	return
}

// clear - for testing
func (jer *JobExecutionRepositoryImpl) clear() {
	clearDB(jer.db)
}
