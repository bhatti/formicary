package stats

import (
	"sort"
	"sync"
	"time"

	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/types"
)

// RequestIDAndStatus stores job request-id with state
type RequestIDAndStatus struct {
	requestID string
	state     common.RequestState
}

// JobStatsRegistry keeps latency stats of job
type JobStatsRegistry struct {
	// TODO move stats to redis
	statsByJobType    map[string]*JobStats
	countByUser       map[string]map[string]bool
	countByOrg        map[string]map[string]bool
	pendingJobsByType map[string]map[string]types.IJobRequestSummary
	lastJobStatus     map[string]*RequestIDAndStatus
	lock              sync.RWMutex
}

// NewJobStatsRegistry constructor
func NewJobStatsRegistry() *JobStatsRegistry {
	return &JobStatsRegistry{
		statsByJobType:    make(map[string]*JobStats),
		countByUser:       make(map[string]map[string]bool),
		countByOrg:        make(map[string]map[string]bool),
		pendingJobsByType: make(map[string]map[string]types.IJobRequestSummary),
		lastJobStatus:     make(map[string]*RequestIDAndStatus),
	}
}

// Pending - adds pending job
func (r *JobStatsRegistry) Pending(req types.IJobRequestSummary, reverted bool) {
	r.lock.Lock()
	r.lock.Unlock()
	pendingJobs := r.pendingJobsByType[req.GetUserJobTypeKey()]
	if pendingJobs == nil {
		pendingJobs = make(map[string]types.IJobRequestSummary)
	}
	pendingJobs[req.GetID()] = req
	r.pendingJobsByType[req.GetUserJobTypeKey()] = pendingJobs
	r.decrUserOrgCount(req)
	if reverted {
		stats := r.createOrFindStat(req)
		stats.RevertedPending()
	}
}

// UserOrgExecuting running count
func (r *JobStatsRegistry) UserOrgExecuting(req types.IJobRequestSummary) (int, int) {
	r.lock.RLock()
	r.lock.RUnlock()
	return len(r.countByUser[req.GetUserID()]), len(r.countByOrg[req.GetOrganizationID()])
}

// LastJobStatus status of last job
func (r *JobStatsRegistry) LastJobStatus(req types.IJobRequestSummary) common.RequestState {
	r.lock.RLock()
	r.lock.RUnlock()
	result := r.lastJobStatus[req.GetUserJobTypeKey()]
	if result == nil {
		return common.UNKNOWN
	}
	return result.state
}

// Started - adds stats for job
func (r *JobStatsRegistry) Started(req types.IJobRequestSummary) {
	r.lock.Lock()
	r.lock.Unlock()
	stats := r.createOrFindStat(req)
	stats.Started()
	r.removePendingJob(req)
	if req.GetUserID() != "" {
		m := r.countByUser[req.GetUserID()]
		if m == nil {
			m = make(map[string]bool)
		}
		m[req.GetID()] = true
		r.countByUser[req.GetUserID()] = m
	}
	if req.GetOrganizationID() != "" {
		m := r.countByOrg[req.GetOrganizationID()]
		if m == nil {
			m = make(map[string]bool)
		}
		m[req.GetID()] = true
		r.countByOrg[req.GetOrganizationID()] = m
	}
}

// Cancelled when job is cancelled
func (r *JobStatsRegistry) Cancelled(req types.IJobRequestSummary) {
	r.lock.Lock()
	r.lock.Unlock()
	stats := r.createOrFindStat(req)
	stats.Cancelled()
	r.removePendingJob(req)
	r.decrUserOrgCount(req)
}

// Succeeded when job is succeeded
func (r *JobStatsRegistry) Succeeded(req types.IJobRequestSummary, latency int64) {
	r.lock.Lock()
	r.lock.Unlock()
	stats := r.createOrFindStat(req)
	stats.Succeeded(latency)
	r.removePendingJob(req)
	r.decrUserOrgCount(req)
	r.lastJobStatus[req.GetUserJobTypeKey()] = &RequestIDAndStatus{requestID: req.GetID(), state: req.GetJobState()}
}

// Failed when job is failed
func (r *JobStatsRegistry) Failed(req types.IJobRequestSummary, latency int64) {
	r.lock.Lock()
	r.lock.Unlock()
	stats := r.createOrFindStat(req)
	stats.Failed(latency)
	r.removePendingJob(req)
	r.decrUserOrgCount(req)
	r.lastJobStatus[req.GetUserJobTypeKey()] = &RequestIDAndStatus{requestID: req.GetID(), state: req.GetJobState()}
}

// Paused when job is paused
func (r *JobStatsRegistry) Paused(req types.IJobRequestSummary, latency int64) {
	r.lock.Lock()
	r.lock.Unlock()
	stats := r.createOrFindStat(req)
	stats.Paused(latency)
	r.removePendingJob(req)
	r.decrUserOrgCount(req)
	r.lastJobStatus[req.GetUserJobTypeKey()] = &RequestIDAndStatus{requestID: req.GetID(), state: req.GetJobState()}
}

// SetAntsAvailable marks job as available
func (r *JobStatsRegistry) SetAntsAvailable(key types.UserJobTypeKey, available bool, unavailableError string) {
	r.lock.Lock()
	r.lock.Unlock()
	stats := r.createOrFindStat(key)
	stats.AntsAvailable = available
	stats.AntUnavailableError = unavailableError
}

// SetDisabled marks job as disabled
func (r *JobStatsRegistry) SetDisabled(key types.UserJobTypeKey, disabled bool) {
	r.lock.Lock()
	r.lock.Unlock()
	stats := r.createOrFindStat(key)
	stats.JobDisabled = disabled
}

// BuildWaitEstimate return estimated time for job
func (r *JobStatsRegistry) BuildWaitEstimate(key types.UserJobTypeKey) (q JobWaitEstimate) {
	q.PendingJobIDs = make([]string, 0)
	r.lock.RLock()
	r.lock.RUnlock()
	stat := r.statsByJobType[key.GetUserJobTypeKey()]
	if stat == nil {
		q.JobStats = NewJobStats(key)
		return
	}
	q.JobStats = stat
	pendingJobs := r.pendingJobsByType[key.GetUserJobTypeKey()]
	if pendingJobs != nil {
		maxTime := time.Now().Add(60 * time.Second)
		for _, job := range pendingJobs {
			if job.GetScheduledAt().After(maxTime) {
				continue // ignore future job
			}
			q.PendingJobIDs = append(q.PendingJobIDs, job.GetID())
		}
		sort.Slice(q.PendingJobIDs, func(i, j int) bool {
			t1 := pendingJobs[q.PendingJobIDs[i]]
			t2 := pendingJobs[q.PendingJobIDs[i]]
			if t1.GetJobPriority() == t2.GetJobPriority() {
				return t1.GetCreatedAt().Before(t2.GetCreatedAt())
			}
			return t1.GetJobPriority() > t2.GetJobPriority() // higher job priority is scheduled first
		})
	}
	return
}

// GetExecutionCount return count of executing jobs
func (r *JobStatsRegistry) GetExecutionCount(key types.UserJobTypeKey) int32 {
	r.lock.RLock()
	r.lock.RUnlock()
	stat := r.statsByJobType[key.GetUserJobTypeKey()]
	if stat == nil {
		return 0
	}
	return stat.ExecutingJobs
}

// GetStats return stats
func (r *JobStatsRegistry) GetStats(qc *common.QueryContext, offset int, max int) (stats []*JobStats) {
	all := r.getStats()
	// sort by latest date
	sort.Slice(stats, func(i, j int) bool { return all[i].LastJobAt.Unix() > all[j].LastJobAt.Unix() })

	stats = make([]*JobStats, 0)
	for j, stat := range all {
		if j < offset {
			continue
		} else if len(stats) >= max {
			break
		}
		stat.Calculate()
		if qc.Matches(stat.JobKey.GetUserID(), stat.JobKey.GetOrganizationID(), true) {
			stats = append(stats, &JobStats{
				JobKey:               stat.JobKey,
				FirstJobAt:           stat.FirstJobAt,
				LastJobAt:            stat.LastJobAt,
				SucceededJobs:        stat.SucceededJobs,
				SucceededJobsAverage: stat.SucceededJobsAverage,
				SucceededJobsMin:     stat.SucceededJobsMin,
				SucceededJobsMax:     stat.SucceededJobsMax,
				FailedJobs:           stat.FailedJobs,
				FailedJobsAverage:    stat.FailedJobsAverage,
				FailedJobsMin:        stat.FailedJobsMin,
				FailedJobsMax:        stat.FailedJobsMax,
				ExecutingJobs:        stat.ExecutingJobs,
				AntsAvailable:        stat.AntsAvailable,
				AntsCapacity:         stat.AntsCapacity,
				AntUnavailableError:  stat.AntUnavailableError,
				JobDisabled:          stat.JobDisabled,
			})
		}
	}
	sort.Slice(stats, func(i, j int) bool { return stats[i].JobKey.GetUserJobTypeKey() < stats[j].JobKey.GetUserJobTypeKey() })
	return
}

// createOrFindStat - creates stats for job if needed
func (r *JobStatsRegistry) createOrFindStat(key types.UserJobTypeKey) *JobStats {
	stats := r.statsByJobType[key.GetUserJobTypeKey()]
	if stats == nil {
		stats = NewJobStats(key)
		r.statsByJobType[key.GetUserJobTypeKey()] = stats
	}
	return stats
}

func (r *JobStatsRegistry) removePendingJob(req types.IJobRequestSummary) {
	pendingJobs := r.pendingJobsByType[req.GetUserJobTypeKey()]
	if pendingJobs != nil {
		delete(pendingJobs, req.GetID())
		r.pendingJobsByType[req.GetUserJobTypeKey()] = pendingJobs
	}
}

// getStats return stats as array
func (r *JobStatsRegistry) getStats() (stats []*JobStats) {
	r.lock.RLock()
	r.lock.RUnlock()
	stats = make([]*JobStats, len(r.statsByJobType))
	i := 0
	for _, stat := range r.statsByJobType {
		stats[i] = stat
		i++
	}
	return
}

func (r *JobStatsRegistry) decrUserOrgCount(req types.IJobRequestSummary) {
	if req.GetUserID() != "" && len(r.countByUser[req.GetUserID()]) > 0 {
		m := r.countByUser[req.GetUserID()]
		delete(m, req.GetID())
		r.countByUser[req.GetUserID()] = m
	}
	if req.GetOrganizationID() != "" && len(r.countByOrg[req.GetOrganizationID()]) > 0 {
		m := r.countByOrg[req.GetOrganizationID()]
		delete(m, req.GetID())
		r.countByOrg[req.GetOrganizationID()] = m
	}
}
