package repository

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	common "plexobject.com/formicary/internal/types"

	"github.com/oklog/ulid/v2"
	"gorm.io/gorm"
	"plexobject.com/formicary/queen/types"
)

var _ JobRequestRepository = &JobRequestRepositoryImpl{}

// JobRequestRepositoryImpl implements JobRequestRepository using gorm O/R mapping
type JobRequestRepositoryImpl struct {
	db     *gorm.DB
	dbType string
}

// NewJobRequestRepositoryImpl creates new instance for job-request-repository
func NewJobRequestRepositoryImpl(db *gorm.DB, dbType string) (*JobRequestRepositoryImpl, error) {
	return &JobRequestRepositoryImpl{db: db, dbType: dbType}, nil
}

// GetParams by id
func (jrr *JobRequestRepositoryImpl) GetParams(
	id string) (params []*types.JobRequestParam, err error) {
	if id == "" {
		return nil, common.NewValidationError(
			fmt.Errorf("id is not specified for job-request"))
	}
	params = make([]*types.JobRequestParam, 0)
	tx := jrr.db.Where("job_request_id = ?", id).Limit(1000).Order("name")
	res := tx.Find(&params)
	if res.Error != nil {
		err = res.Error
		return nil, err
	}
	return
}

// Get method finds JobRequest by id
func (jrr *JobRequestRepositoryImpl) Get(
	qc *common.QueryContext,
	id string) (*types.JobRequest, error) {
	if id == "" {
		return nil, common.NewValidationError(
			fmt.Errorf("id is not specified for job-request"))
	}
	var req types.JobRequest
	res := qc.AddOrgElseUserWhere(jrr.db, true).Preload("Params").Where("id = ?", id).First(&req)
	if res.Error != nil {
		return nil, common.NewNotFoundError(res.Error)
	}
	err := req.AfterLoad()
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"Component": "JobRequestRepositoryImpl",
			"Error":     err,
		}).Warn("failed to initialize request after loading")
	}
	sort.Slice(req.Params, func(i, j int) bool { return req.Params[i].Name < req.Params[j].Name })
	return &req, nil
}

// GetByUserKey JobRequest by user-key
func (jrr *JobRequestRepositoryImpl) GetByUserKey(
	qc *common.QueryContext,
	userKey string) (*types.JobRequest, error) {
	var req types.JobRequest
	res := qc.AddOrgElseUserWhere(jrr.db, true).Preload("Params").Where("user_key = ?", userKey).First(&req)
	if res.Error != nil {
		return nil, common.NewNotFoundError(res.Error)
	}
	err := req.AfterLoad()
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"Component": "JobRequestRepositoryImpl",
			"Error":     err,
		}).Warn("failed to initialize request after loading")
	}
	sort.Slice(req.Params, func(i, j int) bool { return req.Params[i].Name < req.Params[j].Name })
	return &req, nil
}

// Clear - for testing
func (jrr *JobRequestRepositoryImpl) Clear() {
	clearDB(jrr.db)
}

// UpdateJobState sets state of job-request
func (jrr *JobRequestRepositoryImpl) UpdateJobState(
	id string,
	oldState common.RequestState,
	newState common.RequestState,
	errorMessage string,
	errorCode string,
	scheduleDelay time.Duration,
	retried int,
	pausedCount int) error {
	var job types.JobRequest
	tx := jrr.db.Model(&job).Where("id = ?", id)
	if oldState != "" {
		if !oldState.CanTransitionTo(newState) {
			return fmt.Errorf("job %s cannot transition from %s to %s", id, oldState, newState)
		}
		tx = tx.Where("job_state = ?", oldState)
	}
	updates := map[string]interface{}{"job_state": newState, "updated_at": time.Now()}
	if errorMessage != "" || newState == common.COMPLETED {
		updates["error_message"] = errorMessage
		updates["error_code"] = errorCode
	}
	// Free the unique user_key slot when job reaches a terminal state so the
	// same issue can be re-submitted after failure.
	if newState.IsTerminal() {
		updates["user_key"] = ulid.Make().String()
	}
	// Clear hard_restart once the job is executing — the FSM has already read it
	// via DoesRequireFullRestart and any further restarts should be soft by default.
	if newState == common.EXECUTING {
		updates["hard_restart"] = false
	}
	if newState == common.PENDING || newState == common.PAUSED { // not common.MANUAL_APPROVAL_REQUIRED {
		if scheduleDelay > 0 {
			updates["scheduled_at"] = time.Now().Add(scheduleDelay)
		}
		if retried > 0 {
			updates["retried"] = retried
		}
		if pausedCount > 0 {
			updates["paused_count"] = pausedCount
		}
	}
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component":    "JobRequestRepositoryImpl",
			"Method":       "UpdateJobState",
			"Updates":      updates,
			"ID":           id,
			"OldState":     oldState,
			"NewState":     newState,
			"ErrorMessage": errorMessage,
			"ErrorCode":    errorCode,
			"Delay":        scheduleDelay,
			"Retried":      retried,
		}).Debugf("updating job state")
	}

	res := tx.Updates(updates)
	if res.Error != nil {
		return common.NewNotFoundError(res.Error)
	}
	if res.RowsAffected != 1 {
		return common.NewNotFoundError(
			fmt.Errorf("failed to update request state to %v with id %v", newState, id))
	}
	return nil
}

// UpdateRunningTimestamp sets updated time of job-request
func (jrr *JobRequestRepositoryImpl) UpdateRunningTimestamp(
	id string) error {
	var job types.JobRequest
	// check in-clause
	tx := jrr.db.Model(&job).
		Where("id = ?", id).
		Where("job_state IN ?", common.RunningStates)
	res := tx.Updates(map[string]interface{}{"updated_at": time.Now()})
	if res.Error != nil {
		return common.NewNotFoundError(res.Error)
	}
	if res.RowsAffected != 1 {
		return common.NewNotFoundError(
			fmt.Errorf("failed to update request timestamp for %v", id))
	}
	return nil
}

// Save persists job-request
func (jrr *JobRequestRepositoryImpl) Save(
	qc *common.QueryContext,
	req *types.JobRequest) (*types.JobRequest, error) {
	err := req.ValidateBeforeSave()
	if err != nil {
		return nil, common.NewValidationError(err)
	}
	req.OrganizationID = qc.GetOrganizationID()
	req.UserID = qc.GetUserID()
	err = jrr.db.Transaction(func(tx *gorm.DB) error {
		newReq := false
		if req.ID == "" {
			req.ID = ulid.Make().String()
			req.CreatedAt = time.Now()
			req.UpdatedAt = time.Now()
			req.JobState = common.PENDING
			if req.UserKey == "" {
				req.UserKey = ulid.Make().String()
			}
			newReq = true
		} else {
			old, err := jrr.Get(qc, req.ID)
			if err != nil {
				return err
			}
			if !old.Editable(qc.GetUserID(), qc.GetOrganizationID()) {
				logrus.WithFields(logrus.Fields{
					"Component":  "JobRequestRepositoryImpl",
					"JobRequest": req,
					"QC":         qc,
				}).Warnf("invalid owner %s / %s didn't match query context",
					req.UserID, req.OrganizationID)
				return common.NewPermissionError(
					fmt.Errorf("cannot access job request %s", req.ID))
			}
			req.UpdatedAt = time.Now()
			jrr.clearOrphanJobParams(tx, req)
		}
		if req.UserKey == "" {
			req.UserKey = ulid.Make().String()
		}
		var res *gorm.DB

		for _, c := range req.Params {
			if c.ID == "" {
				c.ID = ulid.Make().String()
			}
			c.JobRequestID = req.ID
		}

		if newReq {
			res = tx.Create(req)
		} else {
			res = tx.Save(req)
		}

		if res.Error != nil {
			return res.Error
		}

		//tx.Model(job).Association("Params").Replace(job.Params)
		return nil
	})
	return req, err
}

// SetReadyToExecute marks job as ready to execute so that job can be picked up by job launcher
func (jrr *JobRequestRepositoryImpl) SetReadyToExecute(
	id string,
	jobExecutionID string,
	lastJobExecutionID string) error {
	if id == "" {
		return common.NewValidationError("id is not specified for request to set ready to execute")
	}
	if jobExecutionID == "" {
		return common.NewValidationError("job-execution-id is not specified for request to set ready to execute")
	}
	var totalExecutionCount int64
	res := jrr.db.Model(&types.JobExecution{}).
		Where("id = ?", jobExecutionID).Count(&totalExecutionCount)
	if res.Error != nil {
		return common.NewNotFoundError(res.Error)
	}
	if totalExecutionCount == 0 {
		return common.NewNotFoundError(
			fmt.Errorf("failed to find job-execution for job-execution '%s'", jobExecutionID))
	}

	var job types.JobRequest
	tx := jrr.db.Model(&job).
		Where("id = ?", id).
		Where("job_state IN ?", []string{string(common.PENDING), string(common.PAUSED)}) // not MANUAL_APPROVAL_REQUIRED

	res = tx.Updates(map[string]interface{}{
		"job_state":             common.READY,
		"job_execution_id":      jobExecutionID,
		"last_job_execution_id": lastJobExecutionID,
		"updated_at":            time.Now(),
	})
	if res.Error != nil {
		return common.NewNotFoundError(res.Error)
	}
	if res.RowsAffected != 1 {
		old, err := jrr.Get(common.NewQueryContext(nil, ""), id)
		if err != nil {
			return common.NewNotFoundError(err)
		}
		return common.NewNotFoundError(
			fmt.Errorf("failed to mark job as READY because old status was %v for request-id %s",
				old.JobState, id))
	}
	return nil
}

// UpdatePriority update priority
func (jrr *JobRequestRepositoryImpl) UpdatePriority(
	qc *common.QueryContext,
	id string,
	priority int32) error {
	var job types.JobRequest
	tx := qc.AddOrgElseUserWhere(jrr.db.Model(&job), false).Where("id = ?", id)
	res := tx.Updates(map[string]interface{}{"job_priority": priority, "updated_at": time.Now()})
	if res.Error != nil {
		return common.NewNotFoundError(res.Error)
	}
	if res.RowsAffected != 1 {
		return common.NewNotFoundError(
			fmt.Errorf("failed to update priority for job-request with id %v", id))
	}
	return nil
}

// Cancel a job
func (jrr *JobRequestRepositoryImpl) Cancel(
	qc *common.QueryContext,
	id string) error {
	req, err := jrr.Get(qc, id)
	if err != nil {
		return nil
	}
	if req.JobState.IsTerminal() {
		return common.NewConflictError(fmt.Sprintf("request %s is already in terminal state %s",
			req.ID, req.JobState))
	}
	return jrr.db.Transaction(func(db *gorm.DB) error {
		tx := qc.AddOrgElseUserWhere(db, false).Model(&types.JobRequest{}).
			Where("id = ? AND job_state NOT IN ?", id, common.TerminalStates)
		res := tx.Updates(map[string]interface{}{
			"job_state":     common.CANCELLED,
			"error_code":    common.ErrorJobCancelled,
			"error_message": "cancelled by job request",
			"updated_at":    time.Now()})
		if res.Error != nil {
			return common.NewNotFoundError(res.Error)
		}
		if res.RowsAffected != 1 {
			return common.NewNotFoundError(
				fmt.Errorf("failed to mark job-request as cancel with id %v", id))
		}
		if req.JobExecutionID != "" {
			tx := qc.AddOrgElseUserWhere(db, false).Model(&types.JobExecution{}).Where("id = ?", req.JobExecutionID)
			res := tx.Updates(map[string]interface{}{
				"job_state":     common.CANCELLED,
				"error_message": "job request cancelled",
				"error_code":    common.ErrorJobCancelled,
				"updated_at":    time.Now(),
				"cpu_secs":      time.Now().Unix() - req.ScheduledAt.Unix(), // TODO verify
				"ended_at":      time.Now(),
			})

			if res.Error != nil {
				return common.NewNotFoundError(res.Error)
			}
		}
		return nil
	})
}

// FindActiveChildRequests returns non-terminal child job requests marked for cascade
// cancellation (cascade_cancel = true) for the given parent job request ID.
func (jrr *JobRequestRepositoryImpl) FindActiveChildRequests(parentID string) ([]*types.JobRequest, error) {
	if parentID == "" {
		return nil, common.NewValidationError(fmt.Errorf("parentID is not specified"))
	}
	var requests []*types.JobRequest
	res := jrr.db.Where(
		"parent_id = ? AND cascade_cancel = ? AND job_state NOT IN ?",
		parentID, true, common.TerminalStates,
	).Find(&requests)
	if res.Error != nil {
		return nil, res.Error
	}
	return requests, nil
}

// Delete - deletes job-request
func (jrr *JobRequestRepositoryImpl) Delete(
	qc *common.QueryContext,
	id string) error {
	_, err := jrr.Get(qc, id)
	if err != nil {
		return err
	}
	return jrr.db.Transaction(func(db *gorm.DB) error {
		_ = db.Exec("DELETE FROM formicary_job_request_params WHERE job_request_id = ?", id)
		res := db.Exec("DELETE FROM formicary_job_requests WHERE id = ?", id)
		return res.Error
	})
}

// DeletePendingCronByJobType - delete pending cron job
func (jrr *JobRequestRepositoryImpl) DeletePendingCronByJobType(
	qc *common.QueryContext,
	jobType string) error {
	sql := "SELECT id FROM formicary_job_requests WHERE job_type = ? AND job_state = ? AND cron_triggered = ?"
	args := []interface{}{jobType, common.PENDING, true}
	if !qc.IsAdmin() {
		if qc.HasOrganization() {
			sql += " AND organization_id = ?"
			args = append(args, qc.GetOrganizationID())
		} else if qc.GetUserID() != "" {
			sql += " AND user_id = ?"
			args = append(args, qc.GetUserID())
		}
	}
	rows, err := jrr.db.Raw(sql, args...).Limit(100).Rows()
	if err != nil {
		return err
	}
	defer func() {
		_ = rows.Close()
	}()
	ids := make([]string, 0)

	for rows.Next() {
		var id jobRequestID
		if err = jrr.db.ScanRows(rows, &id); err != nil {
			return err
		}
		ids = append(ids, id.ID)
	}
	if len(ids) == 0 {
		return nil
	}
	return jrr.db.Transaction(func(db *gorm.DB) error {
		res := jrr.db.Exec("DELETE FROM formicary_job_request_params WHERE job_request_id IN ?", ids)
		if res.Error != nil {
			return common.NewNotFoundError(res.Error)
		}
		res = jrr.db.Exec("DELETE FROM formicary_job_requests WHERE id IN ? AND job_state = ?", ids, common.PENDING)
		if res.Error != nil {
			return common.NewNotFoundError(res.Error)
		}
		return nil
	})
}

// PurgeOldRequests hard-deletes job requests and all cascade data for the given job type
// and terminal state where updated_at < olderThan. Returns the count of requests deleted.
// All ID collection and deletes happen inside a single transaction to avoid TOCTOU races
// where a job transitions to EXECUTING after the initial SELECT.
func (jrr *JobRequestRepositoryImpl) PurgeOldRequests(
	jobType string,
	state common.RequestState,
	olderThan time.Time,
	limit int) (int64, error) {
	if !state.IsTerminal() {
		return 0, fmt.Errorf("PurgeOldRequests: state %s is not terminal, refusing to purge", state)
	}
	if limit <= 0 {
		limit = 500
	}

	type idRow struct{ ID string }
	var deleted int64

	err := jrr.db.Transaction(func(tx *gorm.DB) error {
		// Collect request IDs inside the transaction so the state check and deletes are atomic.
		var reqRows []idRow
		if err := tx.Raw(
			"SELECT id FROM formicary_job_requests WHERE job_type = ? AND job_state = ? AND updated_at < ? LIMIT ?",
			jobType, state, olderThan, limit,
		).Scan(&reqRows).Error; err != nil {
			return err
		}
		if len(reqRows) == 0 {
			return nil
		}
		reqIDs := make([]string, len(reqRows))
		for i, r := range reqRows {
			reqIDs[i] = r.ID
		}

		// Collect job execution IDs.
		var execRows []idRow
		if err := tx.Raw(
			"SELECT id FROM formicary_job_executions WHERE job_request_id IN ?", reqIDs,
		).Scan(&execRows).Error; err != nil {
			return err
		}
		execIDs := make([]string, len(execRows))
		for i, r := range execRows {
			execIDs[i] = r.ID
		}

		// Collect task execution IDs.
		taskIDs := make([]string, 0)
		if len(execIDs) > 0 {
			var taskRows []idRow
			if err := tx.Raw(
				"SELECT id FROM formicary_task_executions WHERE job_execution_id IN ?", execIDs,
			).Scan(&taskRows).Error; err != nil {
				return err
			}
			for _, r := range taskRows {
				taskIDs = append(taskIDs, r.ID)
			}
		}

		// Delete in FK dependency order (children before parents).
		if len(taskIDs) > 0 {
			// Approval votes/deadlines reference task_executions via FK; delete explicitly
			// to be safe across all DB engines (SQLite requires PRAGMA foreign_keys=ON).
			if err := tx.Exec("DELETE FROM formicary_approval_votes WHERE task_execution_id IN ?", taskIDs).Error; err != nil {
				return err
			}
			if err := tx.Exec("DELETE FROM formicary_approval_deadlines WHERE task_execution_id IN ?", taskIDs).Error; err != nil {
				return err
			}
			if err := tx.Exec("DELETE FROM formicary_job_resource_uses WHERE task_execution_id IN ?", taskIDs).Error; err != nil {
				return err
			}
			if err := tx.Exec("DELETE FROM formicary_task_execution_context WHERE task_execution_id IN ?", taskIDs).Error; err != nil {
				return err
			}
			if err := tx.Exec("DELETE FROM formicary_task_executions WHERE id IN ?", taskIDs).Error; err != nil {
				return err
			}
		}
		if len(execIDs) > 0 {
			if err := tx.Exec("DELETE FROM formicary_log_events WHERE job_execution_id IN ?", execIDs).Error; err != nil {
				return err
			}
			if err := tx.Exec("DELETE FROM formicary_job_execution_context WHERE job_execution_id IN ?", execIDs).Error; err != nil {
				return err
			}
			if err := tx.Exec("DELETE FROM formicary_job_executions WHERE id IN ?", execIDs).Error; err != nil {
				return err
			}
		}
		// Artifacts reference job_request_id; delete before the request row itself.
		if err := tx.Exec("DELETE FROM formicary_artifacts WHERE job_request_id IN ?", reqIDs).Error; err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM formicary_job_request_params WHERE job_request_id IN ?", reqIDs).Error; err != nil {
			return err
		}
		res := tx.Exec("DELETE FROM formicary_job_requests WHERE id IN ?", reqIDs)
		if res.Error != nil {
			return res.Error
		}
		deleted = res.RowsAffected
		return nil
	})
	return deleted, err
}

// Trigger triggers a scheduled job
func (jrr *JobRequestRepositoryImpl) Trigger(
	qc *common.QueryContext,
	id string) error {
	// TODO check for cron schedule
	sql := "UPDATE formicary_job_requests SET scheduled_at = ?, updated_at = ?, user_key = ? WHERE id = ? AND cron_triggered = ? AND job_state = ?"
	args := []interface{}{time.Now(), time.Now(), ulid.Make().String(), id, true, common.PENDING}
	if !qc.IsAdmin() {
		if qc.HasOrganization() {
			sql += " AND organization_id = ?"
			args = append(args, qc.GetOrganizationID())
		} else if qc.GetUserID() != "" {
			sql += " AND user_id = ?"
			args = append(args, qc.GetUserID())
		}
	}
	res := jrr.db.Exec(sql, args...)
	if res.Error != nil {
		return common.NewNotFoundError(res.Error)
	}
	return nil
}

// Restart the job; hard=true sets hard_restart so DoesRequireFullRestart returns true
// and all tasks re-run from scratch instead of only the failed ones.
// newJobDefinitionID, when non-empty, re-pins the request to a different job definition version.
func (jrr *JobRequestRepositoryImpl) Restart(
	qc *common.QueryContext,
	id string,
	hard bool,
	newJobDefinitionID string) error {
	setClauses := "job_state = ?, last_job_execution_id = job_execution_id, " +
		"job_execution_id = NULL, error_code = NULL, error_message = NULL, schedule_attempts = 0, " +
		"retried = retried + 1, hard_restart = ?, scheduled_at = ?, updated_at = ?"
	args := []interface{}{common.PENDING, hard, time.Now(), time.Now()}
	if newJobDefinitionID != "" {
		setClauses += ", job_definition_id = ?"
		args = append(args, newJobDefinitionID)
	}
	sql := "UPDATE formicary_job_requests SET " + setClauses + " WHERE id = ? AND job_state NOT IN ?"
	args = append(args, id, common.NotRestartableStates)
	if !qc.IsAdmin() {
		if qc.HasOrganization() {
			sql += " AND organization_id = ?"
			args = append(args, qc.GetOrganizationID())
		} else if qc.GetUserID() != "" {
			sql += " AND user_id = ?"
			args = append(args, qc.GetUserID())
		}
	}
	res := jrr.db.Exec(sql, args...)
	if res.Error != nil {
		return common.NewNotFoundError(res.Error)
	}
	if res.RowsAffected != 1 {
		return common.NewNotFoundError(
			fmt.Errorf("failed to restart job-request with id %v", id))
	}
	return nil
}

// IncrementScheduleAttempts and optionally bump schedule time and decrement priority for jobs that are not ready
func (jrr *JobRequestRepositoryImpl) IncrementScheduleAttempts(
	id string,
	scheduleSecs time.Duration,
	decrPriority int,
	errorMessage string) error {
	res := jrr.db.Exec(
		"UPDATE formicary_job_requests SET schedule_attempts = schedule_attempts + 1, "+
			"scheduled_at = ?, job_priority = job_priority - ?, error_message = ?, updated_at = ? WHERE id = ?",
		time.Now().Add(scheduleSecs), decrPriority, errorMessage, time.Now(), id)
	if res.Error != nil {
		return common.NewNotFoundError(res.Error)
	}
	if res.RowsAffected != 1 {
		return common.NewNotFoundError(
			fmt.Errorf("failed to update schedule_attempts for %s", id))
	}
	return nil
}

type jobRequestID struct {
	ID string
}

type jobRequestIDState struct {
	ID       string
	JobState common.RequestState
}

// RecentIDs returns job -ids
func (jrr *JobRequestRepositoryImpl) RecentIDs(
	limit int) (res map[string]common.RequestState, err error) {
	sql := "SELECT id, job_state FROM formicary_job_requests ORDER BY updated_at DESC limit ?"
	args := []interface{}{limit}
	rows, err := jrr.db.Raw(sql, args...).Limit(limit).Rows()
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	res = make(map[string]common.RequestState)

	for rows.Next() {
		var id jobRequestIDState
		if err = jrr.db.ScanRows(rows, &id); err != nil {
			return nil, err
		}
		res[id.ID] = id.JobState
	}

	return res, nil
}

// RecentLiveIDs returns recently alive - executing/pending/starting job-ids
func (jrr *JobRequestRepositoryImpl) RecentLiveIDs(
	limit int) ([]string, error) {
	sql := "SELECT id FROM formicary_job_requests WHERE job_state NOT IN ? ORDER BY updated_at DESC limit ?"
	jobStates := common.TerminalStates
	args := []interface{}{jobStates, limit}
	rows, err := jrr.db.Raw(sql, args...).Limit(limit).Rows()
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	ids := make([]string, 0)

	for rows.Next() {
		var id jobRequestID
		if err = jrr.db.ScanRows(rows, &id); err != nil {
			return nil, err
		}
		ids = append(ids, id.ID)
	}

	return ids, nil
}

// RecentDeadIDs returns recently completed job-ids
func (jrr *JobRequestRepositoryImpl) RecentDeadIDs(
	limit int,
	fromOffset time.Duration,
	toOffset time.Duration,
) ([]string, error) {
	sql := "SELECT id FROM formicary_job_requests WHERE job_state IN ? AND updated_at > ? AND updated_at < ? ORDER BY updated_at DESC limit ?"
	jobStates := common.TerminalStates
	now := time.Now()
	args := []interface{}{jobStates, now.Add(fromOffset * -1), now.Add(toOffset * -1), limit}
	rows, err := jrr.db.Raw(sql, args...).Limit(limit).Rows()
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	ids := make([]string, 0)

	for rows.Next() {
		var id jobRequestID
		if err = jrr.db.ScanRows(rows, &id); err != nil {
			return nil, err
		}
		ids = append(ids, id.ID)
	}

	return ids, nil

}

// JobCountsByDays calculates stats for all job-types/statuses within days
func (jrr *JobRequestRepositoryImpl) JobCountsByDays(
	qc *common.QueryContext,
	limit int,
) (stats []*types.JobCounts, err error) {
	var args []interface{}
	var sql string
	scopeSelect := ""
	scopeWhere := ""
	if qc.IsAdmin() {
	} else if qc.HasOrganization() {
		scopeSelect = "organization_id,"
		scopeWhere = "WHERE organization_id = ? "
		args = append(args, qc.GetOrganizationID())
	} else if qc.GetUserID() != "" {
		scopeSelect = "user_id,"
		scopeWhere = "WHERE user_id = ? "
		args = append(args, qc.GetUserID())
	}

	if jrr.dbType == "sqlite" {
		sql = "SELECT " + scopeSelect + "job_type, job_state, count(*) AS counts, date(updated_at) as day " +
			" FROM formicary_job_requests " + scopeWhere +
			" GROUP BY " + scopeSelect + "job_type, job_state, day ORDER BY day desc limit ?"
	} else {
		sql = "SELECT " + scopeSelect + "job_type, job_state, count(*) AS counts, cast(updated_at as date) as start_time, " +
			" cast(updated_at as date) as end_time FROM formicary_job_requests " + scopeWhere +
			" GROUP BY " + scopeSelect + "job_type, job_state, start_time ORDER BY start_time desc limit ?"
	}
	args = append(args, limit)
	rows, err := jrr.db.Raw(sql, args...).Rows()
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	stats = make([]*types.JobCounts, 0)

	for rows.Next() {
		stat := types.JobCounts{}
		if err = jrr.db.ScanRows(rows, &stat); err != nil {
			return nil, err
		}
		if stat.Day != "" {
			stat.StartTime, _ = time.Parse("2006-01-02", stat.Day)
			stat.EndTime, _ = time.Parse("2006-01-02", stat.Day)
		} else {
			stat.Day = stat.GetStartTime().Format("2006-01-02")
		}
		stats = append(stats, &stat)
	}
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component": "JobRequestRepositoryImpl",
			"Method":    "JobCountsByDays",
			"SQL":       sql,
			"Args":      args,
			"Stats":     len(stats),
		}).Debugf("job counts by days")
	}

	return stats, nil
}

// JobCounts calculates stats for all job-types/statuses/error-codes within given range
func (jrr *JobRequestRepositoryImpl) JobCounts(
	qc *common.QueryContext,
	start time.Time,
	end time.Time) ([]*types.JobCounts, error) {
	args := []interface{}{start, end}
	scopeSelect := ""
	scopeWhere := ""
	if qc.IsAdmin() {
	} else if qc.HasOrganization() {
		scopeSelect = "organization_id,"
		scopeWhere = "AND organization_id = ? "
		args = append(args, qc.GetOrganizationID())
	} else if qc.GetUserID() != "" {
		scopeSelect = "user_id,"
		scopeWhere = "AND user_id = ? "
		args = append(args, qc.GetUserID())
	}
	var sql string
	if jrr.dbType == "sqlite" {
		sql = "SELECT " + scopeSelect + "job_type, job_state, error_code, count(*) AS counts, min(updated_at) as start_time_string, " +
			"max(updated_at) as end_time_string FROM formicary_job_requests WHERE updated_at >= ? AND updated_at <= ? " +
			scopeWhere + " GROUP BY " + scopeSelect + "job_type, job_state, error_code ORDER BY job_type "
	} else {
		sql = "SELECT " + scopeSelect + "job_type, job_state, error_code, count(*) AS counts, min(updated_at) as start_time, " +
			"max(updated_at) as end_time FROM formicary_job_requests WHERE updated_at >= ? AND updated_at <= ? " +
			scopeWhere + " GROUP BY " + scopeSelect + "job_type, job_state, error_code ORDER BY job_type "
	}

	rows, err := jrr.db.Raw(sql, args...).Rows()
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	stats := make([]*types.JobCounts, 0)

	for rows.Next() {
		stat := types.JobCounts{}
		if err = jrr.db.ScanRows(rows, &stat); err != nil {
			return nil, err
		}
		stat.Day = stat.GetStartTime().Format("2006-01-02")
		stats = append(stats, &stat)
	}
	sort.Slice(stats, func(i, j int) bool { return stats[i].GetEndTime().Unix() > stats[j].GetEndTime().Unix() })
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component": "JobRequestRepositoryImpl",
			"Method":    "JobCounts",
			"SQL":       sql,
			"Args":      args,
			"Stats":     len(stats),
		}).Debugf("job counts")
	}
	return stats, nil
}

// CountByOrgAndState returns counts
func (jrr *JobRequestRepositoryImpl) CountByOrgAndState(
	org string,
	state common.RequestState) (totalRecords int64, err error) {
	res := jrr.db.Model(&types.JobRequest{}).
		Where("organization_id = ?", org).
		Where("job_state = ?", state).Count(&totalRecords)
	if res.Error != nil {
		err = res.Error
	}
	return
}

// CountByJobTypeAndState counts jobs matching the given job-type and one or more states.
// States are passed as variadic common.RequestState; if none are provided, all states match.
// Returns 0 on error so it is safe to use inside skip_if templates.
func (jrr *JobRequestRepositoryImpl) CountByJobTypeAndState(
	jobType string, states ...common.RequestState) (int64, error) {
	var totalRecords int64
	tx := jrr.db.Model(&types.JobRequest{}).Where("job_type = ?", jobType)
	if len(states) > 0 {
		stateVals := make([]string, len(states))
		for i, s := range states {
			stateVals[i] = string(s)
		}
		tx = tx.Where("job_state IN ?", stateVals)
	}
	res := tx.Count(&totalRecords)
	if res.Error != nil {
		return 0, res.Error
	}
	return totalRecords, nil
}

// FindActiveCronScheduledJobsByJobType queries scheduled jobs that are either running or waiting to be run
func (jrr *JobRequestRepositoryImpl) FindActiveCronScheduledJobsByJobType(
	jobTypesAndTrigger []types.JobTypeCronTrigger,
) ([]*types.JobRequestInfo, error) {
	// initialize job-types, user-ids, and keys
	jobTypes := make([]string, len(jobTypesAndTrigger))
	userIDs := make([]string, 0)
	userKeys := make([]string, len(jobTypesAndTrigger))

	for i, typeAndTrigger := range jobTypesAndTrigger {
		jobTypes[i] = typeAndTrigger.JobType
		if typeAndTrigger.UserID != "" {
			userIDs = append(userIDs, typeAndTrigger.UserID)
		}
		_, userKeys[i] = types.GetCronScheduleTimeAndUserKey(typeAndTrigger.OrganizationOrUserID(), typeAndTrigger.JobType, typeAndTrigger.CronTrigger)
	}
	jobStates := []common.RequestState{common.PENDING, common.PAUSED, common.MANUAL_APPROVAL_REQUIRED,
		common.READY, common.STARTED, common.EXECUTING}
	args := []interface{}{true, jobTypes, jobStates}

	sql := "SELECT id, job_type, job_version, organization_id, user_id, job_priority, job_state, schedule_attempts, scheduled_at, created_at, " +
		" job_definition_id, job_execution_id, last_job_execution_id, current_task, cron_triggered, retried FROM formicary_job_requests WHERE " +
		" cron_triggered = ? AND ((job_type IN ? AND job_state IN ?"
	if len(userIDs) > 0 {
		sql += " AND user_id IN ?"
		args = append(args, userIDs)
	}
	sql += ") OR user_key IN ?) order by updated_at desc"
	args = append(args, userKeys)

	rows, err := jrr.db.Raw(sql, args...).Rows()
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	infos := make([]*types.JobRequestInfo, 0)

	duplicates := make(map[string]bool)
	for rows.Next() {
		info := types.JobRequestInfo{}
		if err = jrr.db.ScanRows(rows, &info); err != nil {
			return nil, err
		}
		if !duplicates[info.GetUserJobTypeKey()] {
			infos = append(infos, &info)
			duplicates[info.GetUserJobTypeKey()] = true
		}
	}
	return infos, nil
}

// GetJobTimes returns job times for recent jobs
func (jrr *JobRequestRepositoryImpl) GetJobTimes(
	limit int) ([]*types.JobTime, error) {
	sql := "SELECT r.id, r.organization_id, r.user_id, r.job_type, r.job_version, r.job_state, " +
		" r.job_priority, r.scheduled_at, r.created_at , x.started_at, x.ended_at " +
		"FROM formicary_job_requests r LEFT OUTER JOIN formicary_job_executions x " +
		"ON r.job_execution_id = x.id ORDER BY r.updated_at DESC LIMIT ?"
	rows, err := jrr.db.Raw(sql, limit).Limit(limit).Rows()
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	times := make([]*types.JobTime, 0)

	for rows.Next() {
		jt := types.JobTime{}
		if err = jrr.db.ScanRows(rows, &jt); err != nil {
			return nil, err
		}
		times = append(times, &jt)
	}
	return times, nil
}

// NextSchedulableJobsByTypes queries basic job id/state for pending/ready state from parameter
func (jrr *JobRequestRepositoryImpl) NextSchedulableJobsByTypes(
	jobTypes []string,
	state []common.RequestState,
	limit int) ([]*types.JobRequestInfo, error) {
	sql := "SELECT id, job_type, job_version, organization_id, user_id, job_priority, job_state, schedule_attempts, scheduled_at, created_at, " +
		" job_definition_id, job_execution_id, last_job_execution_id, cron_triggered, current_task, retried FROM formicary_job_requests WHERE job_type in " +
		" (SELECT job_type FROM formicary_job_definitions WHERE disabled is false AND active is true AND " +
		" (user_id = formicary_job_requests.user_id OR organization_id = formicary_job_requests.organization_id)) " +
		" AND job_state IN ? AND scheduled_at <= ? "

	args := []interface{}{state, time.Now()}

	if len(jobTypes) > 0 {
		sql += " AND job_type IN ?"
		args = append(args, jobTypes)
	}
	sql += " ORDER BY job_priority DESC, created_at LIMIT ?"
	args = append(args, limit)

	rows, err := jrr.db.Raw(sql, args...).Rows()
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()
	infos := make([]*types.JobRequestInfo, 0)

	for rows.Next() {
		info := types.JobRequestInfo{}
		if err = jrr.db.ScanRows(rows, &info); err != nil {
			return nil, err
		}
		infos = append(infos, &info)
	}
	return infos, nil
}

// RequeueOrphanRequests queries jobs with EXECUTING/STARTED status and puts them back to PENDING
func (jrr *JobRequestRepositoryImpl) RequeueOrphanRequests(
	staleInterval time.Duration) (total int64, err error) {
	date := time.Now().Add(-staleInterval)
	res := jrr.db.Exec(
		"UPDATE formicary_job_requests SET job_state = ?, scheduled_at = ?, updated_at = ? "+
			" WHERE job_state IN ? AND updated_at < ?",
		common.PENDING,
		time.Now(),
		time.Now(),
		[]common.RequestState{common.STARTED, common.READY, common.EXECUTING},
		date)
	if res.Error != nil {
		return 0, res.Error
	}
	if res.RowsAffected == 0 && logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component": "JobRequestRepositoryImpl",
			"Date":      date,
		}).Debugf("didn't find any orphan jobs")
	}
	return res.RowsAffected, nil
}

// QueryOrphanRequests queries orphan jobs that haven't been updated
func (jrr *JobRequestRepositoryImpl) QueryOrphanRequests(
	limit int,
	offset int,
	staleInterval time.Duration) (jobRequests []*types.JobRequest, err error) {
	jobRequests = make([]*types.JobRequest, 0)
	date := time.Now().Add(-staleInterval * time.Second)
	tx := jrr.db.Preload("Params").Limit(limit).Offset(offset).
		Where("job_state IN ? AND updated_at < ?", common.OrphanStates, date).
		Order("updated_at")
	res := tx.Find(&jobRequests)
	if res.Error != nil {
		err = res.Error
		return nil, err
	}
	for _, j := range jobRequests {
		err = j.AfterLoad()
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"Component": "JobRequestRepositoryImpl",
				"Error":     err,
			}).Warn("failed to initialize request after loading for orphan requests")
		}
	}
	return
}

// clearOrphanJobParams remove orphan request params
func (jrr *JobRequestRepositoryImpl) clearOrphanJobParams(tx *gorm.DB, req *types.JobRequest) {
	paramIDs := make([]string, len(req.Params))
	for i, c := range req.Params {
		paramIDs[i] = c.ID
	}

	tx.Where("id NOT IN ? AND job_request_id = ?", paramIDs, req.ID).Delete(types.JobRequestParam{})
}

// Query finds matching job-request by parameters
func (jrr *JobRequestRepositoryImpl) Query(
	qc *common.QueryContext,
	params map[string]interface{},
	page int,
	pageSize int,
	order []string) (jobRequests []*types.JobRequest, totalRecords int64, err error) {
	jobState := params["job_state"]
	jobRequests = make([]*types.JobRequest, 0)
	tx := qc.AddOrgElseUserWhere(jrr.db, true).Preload("Params").Limit(pageSize).Offset(page * pageSize)
	tx = jrr.addQuery(params, tx)
	if len(order) == 0 {
		if jobState == common.WAITING || jobState == common.READY || jobState == common.PENDING ||
			jobState == common.PAUSED || jobState == common.MANUAL_APPROVAL_REQUIRED {
			tx = tx.Order("job_priority DESC").Order("created_at")
		} else {
			tx = tx.Order("updated_at DESC")
		}
	} else {
		for _, ord := range order {
			tx = tx.Order(ord)
		}
	}
	res := tx.Find(&jobRequests)
	if res.Error != nil {
		err = res.Error
		return nil, 0, err
	}
	for _, j := range jobRequests {
		err = j.AfterLoad()
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"Component": "JobRequestRepositoryImpl",
				"Error":     err,
			}).Warn("failed to initialize request after loading for query")
		}
	}
	if jobState != nil {
		params["job_state"] = jobState
	}
	totalRecords, _ = jrr.Count(qc, params)
	return
}

// Count counts records by query
func (jrr *JobRequestRepositoryImpl) Count(
	qc *common.QueryContext,
	params map[string]interface{}) (totalRecords int64, err error) {
	tx := jrr.db.Model(&types.JobRequest{})
	tx = qc.AddOrgElseUserWhere(tx, true)
	tx = jrr.addQuery(params, tx)
	res := tx.Count(&totalRecords)
	if res.Error != nil {
		err = res.Error
	}
	return
}

func (jrr *JobRequestRepositoryImpl) addQuery(params map[string]interface{}, tx *gorm.DB) *gorm.DB {
	q := params["q"]
	if q != nil {
		reqID, _ := strconv.ParseInt(fmt.Sprintf("%s", q), 10, 64)
		qs := fmt.Sprintf("%%%s%%", q)
		tx = tx.Where("id = ? OR job_type LIKE ? OR description LIKE ? OR user_id LIKE ? OR organization_id LIKE ? OR quick_search LIKE ?",
			reqID, qs, qs, qs, qs, qs)
	}
	jobState := params["job_state"]
	if jobState != nil {
		delete(params, "job_state")
		if jobState == "RUNNING" {
			tx = tx.Where("job_state IN ?", common.RunningStates)
		} else if jobState == "WAITING" {
			tx = tx.Where("job_state IN ?", common.WaitingStates)
		} else if jobState == "DONE" {
			tx = tx.Where("job_state IN ?", common.TerminalStates)
		} else {
			tx = tx.Where("job_state = ?", jobState)
		}
	}
	return addQueryParamsWhere(filterParams(params, "q"), tx)
}

// JobSubmissionSummary holds aggregated job submission counts per user/org/job_type.
type JobSubmissionSummary struct {
	UserID         string `json:"user_id"`
	OrganizationID string `json:"organization_id"`
	JobType        string `json:"job_type"`
	SubmittedCount int64  `json:"submitted_count"`
	SucceededCount int64  `json:"succeeded_count"`
	FailedCount    int64  `json:"failed_count"`
}

// jobSubmissionOrderCols is the allowlist of sortable columns for QueryJobSubmissions.
var jobSubmissionOrderCols = map[string]bool{
	"user_id": true, "organization_id": true, "job_type": true,
	"submitted_count": true, "succeeded_count": true, "failed_count": true,
}

// validatedJobSubmissionOrder returns safe ORDER BY clauses or an error.
func validatedJobSubmissionOrder(order []string) ([]string, error) {
	safe := make([]string, 0, len(order))
	for _, ord := range order {
		parts := strings.Fields(strings.ToLower(ord))
		if len(parts) == 0 || len(parts) > 2 {
			return nil, fmt.Errorf("invalid order clause: %q", ord)
		}
		if !jobSubmissionOrderCols[parts[0]] {
			return nil, fmt.Errorf("sort column not allowed: %q", parts[0])
		}
		if len(parts) == 2 && parts[1] != "asc" && parts[1] != "desc" {
			return nil, fmt.Errorf("invalid sort direction: %q", parts[1])
		}
		safe = append(safe, ord)
	}
	return safe, nil
}

// QueryJobSubmissions aggregates job submission counts from formicary_job_requests,
// grouped by user_id/organization_id/job_type.
// submitted_count = COUNT(*); succeeded_count = COMPLETED; failed_count = FAILED.
// Params: organization_id (equality), from/to (created_at range, RFC3339 or YYYY-MM-DD).
func (jrr *JobRequestRepositoryImpl) QueryJobSubmissions(
	params map[string]interface{},
	page int,
	pageSize int,
	order []string) (records []*JobSubmissionSummary, totalRecords int64, err error) {
	records = make([]*JobSubmissionSummary, 0)

	safeOrder, err := validatedJobSubmissionOrder(order)
	if err != nil {
		return nil, 0, err
	}

	// Translate from/to into the colon-operator keys that addQueryParamsWhere understands,
	// then delegate all WHERE-clause building to the shared helper.
	normalized := make(map[string]interface{})
	for k, v := range params {
		switch k {
		case "from":
			normalized["created_at:>="] = v
		case "to":
			normalized["created_at:<="] = v
		default:
			normalized[k] = v
		}
	}

	base := jrr.db.Table("formicary_job_requests")
	base = addQueryParamsWhere(normalized, base)

	// Count distinct (user, org, job_type) groups.
	// Use a multi-char separator so values containing '|' cannot produce false collisions.
	countRes := base.Session(&gorm.Session{}).
		Select("COUNT(DISTINCT COALESCE(user_id,'') || '||SEP||' || COALESCE(organization_id,'') || '||SEP||' || COALESCE(job_type,''))").
		Scan(&totalRecords)
	if countRes.Error != nil {
		return nil, 0, countRes.Error
	}

	q := base.Session(&gorm.Session{}).
		Select("user_id, organization_id, job_type, " +
			"COUNT(*) as submitted_count, " +
			"SUM(CASE WHEN job_state = 'COMPLETED' THEN 1 ELSE 0 END) as succeeded_count, " +
			"SUM(CASE WHEN job_state = 'FAILED' THEN 1 ELSE 0 END) as failed_count").
		Group("user_id, organization_id, job_type")

	if len(safeOrder) == 0 {
		q = q.Order("submitted_count DESC")
	} else {
		for _, ord := range safeOrder {
			q = q.Order(ord)
		}
	}
	q = q.Limit(pageSize).Offset(page * pageSize)
	res := q.Scan(&records)
	if res.Error != nil {
		return nil, 0, res.Error
	}
	return records, totalRecords, nil
}
