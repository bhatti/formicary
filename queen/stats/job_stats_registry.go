package stats

import (
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/types"
	"sort"
	"sync"
	"time"
)

// RequestIDAndStatus stores job request-id with state
type RequestIDAndStatus struct {
	requestID uint64
	state     common.RequestState
}

// JobStatsRegistry keeps latency stats of job
type JobStatsRegistry struct {
	// TODO move stats to redis
	// TODO add request-ids for each user/org
	statsByJobType    map[string]*JobStats
	countByUser       map[string]int
	countByOrg        map[string]int
	pendingJobsByType map[string]map[uint64]types.IJobRequestSummary
	lastJobStatus     map[string]*RequestIDAndStatus
	lock              sync.RWMutex
}

// NewJobStatsRegistry constructor
func NewJobStatsRegistry() *JobStatsRegistry {
	return &JobStatsRegistry{
		statsByJobType:    make(map[string]*JobStats),
		countByUser:       make(map[string]int),
		countByOrg:        make(map[string]int),
		pendingJobsByType: make(map[string]map[uint64]types.IJobRequestSummary),
		lastJobStatus:     make(map[string]*RequestIDAndStatus),
	}
}

// Pending - adds pending job
func (r *JobStatsRegistry) Pending(req types.IJobRequestSummary) {
	r.lock.Lock()
	r.lock.Unlock()
	pendingJobs := r.pendingJobsByType[req.GetUserJobTypeKey()]
	if pendingJobs == nil {
		pendingJobs = make(map[uint64]types.IJobRequestSummary)
	}
	pendingJobs[req.GetID()] = req
	r.pendingJobsByType[req.GetUserJobTypeKey()] = pendingJobs
}

// UserOrgExecuting running count
func (r *JobStatsRegistry) UserOrgExecuting(req types.IJobRequestSummary) (int, int) {
	r.lock.RLock()
	r.lock.RUnlock()
	return r.countByUser[req.GetUserID()], r.countByOrg[req.GetOrganizationID()]
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
		r.countByUser[req.GetUserID()] = r.countByUser[req.GetUserID()] + 1
	}
	if req.GetOrganizationID() != "" {
		r.countByOrg[req.GetOrganizationID()] = r.countByOrg[req.GetOrganizationID()] + 1
	}
}

// Cancelled when job is cancelled
func (r *JobStatsRegistry) Cancelled(req types.IJobRequestSummary) {
	r.lock.Lock()
	r.lock.Unlock()
	stats := r.createOrFindStat(req)
	stats.Cancelled()
	r.removePendingJob(req)
	if req.GetUserID() != "" && r.countByUser[req.GetUserID()] > 0 {
		r.countByUser[req.GetUserID()] = r.countByUser[req.GetUserID()] - 1
	}
	if req.GetOrganizationID() != "" && r.countByOrg[req.GetOrganizationID()] > 0 {
		r.countByOrg[req.GetOrganizationID()] = r.countByOrg[req.GetOrganizationID()] - 1
	}
}

// Succeeded when job is succeeded
func (r *JobStatsRegistry) Succeeded(req types.IJobRequestSummary, latency int64) {
	r.lock.Lock()
	r.lock.Unlock()
	stats := r.createOrFindStat(req)
	stats.Succeeded(latency)
	r.removePendingJob(req)
	if req.GetUserID() != "" && r.countByUser[req.GetUserID()] > 0 {
		r.countByUser[req.GetUserID()] = r.countByUser[req.GetUserID()] - 1
	}
	if req.GetOrganizationID() != "" && r.countByOrg[req.GetOrganizationID()] > 0 {
		r.countByOrg[req.GetOrganizationID()] = r.countByOrg[req.GetOrganizationID()] - 1
	}
	r.lastJobStatus[req.GetUserJobTypeKey()] = &RequestIDAndStatus{requestID: req.GetID(), state: req.GetJobState()}
}

// Failed when job is failed
func (r *JobStatsRegistry) Failed(req types.IJobRequestSummary, latency int64) {
	r.lock.Lock()
	r.lock.Unlock()
	stats := r.createOrFindStat(req)
	stats.Failed(latency)
	r.removePendingJob(req)
	if req.GetUserID() != "" && r.countByUser[req.GetUserID()] > 0 {
		r.countByUser[req.GetUserID()] = r.countByUser[req.GetUserID()] - 1
	}
	if req.GetOrganizationID() != "" && r.countByOrg[req.GetOrganizationID()] > 0 {
		r.countByOrg[req.GetOrganizationID()] = r.countByOrg[req.GetOrganizationID()] - 1
	}
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

// SetPaused marks job as paused
func (r *JobStatsRegistry) SetPaused(key types.UserJobTypeKey, paused bool) {
	r.lock.Lock()
	r.lock.Unlock()
	stats := r.createOrFindStat(key)
	stats.JobPaused = paused
}

// BuildWaitEstimate return estimated time for job
func (r *JobStatsRegistry) BuildWaitEstimate(key types.UserJobTypeKey) (q JobWaitEstimate) {
	q.PendingJobIDs = make([]uint64, 0)
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
		for k, job := range pendingJobs {
			if job.GetScheduledAt().After(maxTime) {
				continue // ignore future job
			}
			q.PendingJobIDs = append(q.PendingJobIDs, k)
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
		if qc.Matches(stat.JobKey.GetUserID(), stat.JobKey.GetOrganizationID()) {
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
				JobPaused:            stat.JobPaused,
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
