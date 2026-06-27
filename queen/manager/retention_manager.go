package manager

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"plexobject.com/formicary/internal/metrics"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/repository"
)

// RetentionManager purges old job history based on per-job-definition retention policy.
// Retention applies only to terminal states (FAILED, COMPLETED, CANCELLED). EXECUTING and other
// non-terminal states are never touched.
type RetentionManager struct {
	jobDefinitionRepository repository.JobDefinitionRepository
	jobRequestRepository    repository.JobRequestRepository
	metricsRegistry         *metrics.Registry
}

// NewRetentionManager creates a new RetentionManager.
func NewRetentionManager(
	jobDefinitionRepository repository.JobDefinitionRepository,
	jobRequestRepository repository.JobRequestRepository,
	metricsRegistry *metrics.Registry,
) (*RetentionManager, error) {
	if jobDefinitionRepository == nil {
		return nil, fmt.Errorf("jobDefinitionRepository is required")
	}
	if jobRequestRepository == nil {
		return nil, fmt.Errorf("jobRequestRepository is required")
	}
	if metricsRegistry == nil {
		return nil, fmt.Errorf("metricsRegistry is required")
	}
	return &RetentionManager{
		jobDefinitionRepository: jobDefinitionRepository,
		jobRequestRepository:    jobRequestRepository,
		metricsRegistry:         metricsRegistry,
	}, nil
}

// PurgeAll iterates all active job definitions and hard-deletes history older than each
// definition's configured retention window. Runs in batches of 500 to avoid long-lock
// transactions. Returns total job_requests deleted across all types and states.
func (rm *RetentionManager) PurgeAll(ctx context.Context) (int64, error) {
	start := time.Now()
	qc := common.NewQueryContextFromIDs("", "").WithAdmin()

	// Paginate job definitions to avoid a hard cap silently missing entries.
	const pageSize = 1000
	var totalPurged int64
	terminalStates := []common.RequestState{common.FAILED, common.COMPLETED, common.CANCELLED}

	for offset := 0; ; offset += pageSize {
		jobDefs, _, err := rm.jobDefinitionRepository.Query(qc, map[string]interface{}{"active": true}, offset, pageSize, []string{})
		if err != nil {
			return totalPurged, fmt.Errorf("retention: failed to load job definitions at offset %d: %w", offset, err)
		}
		if len(jobDefs) == 0 {
			break
		}

		for _, jd := range jobDefs {
			for _, state := range terminalStates {
				days := jd.GetRetentionDays(state)
				if days <= 0 {
					continue
				}
				olderThan := time.Now().AddDate(0, 0, -days)
				purged, purgeErr := rm.purgeState(ctx, jd.JobType, state, olderThan)
				if purgeErr != nil {
					logrus.WithFields(logrus.Fields{
						"Component": "RetentionManager",
						"JobType":   jd.JobType,
						"State":     state,
						"Error":     purgeErr,
					}).Warn("retention purge batch failed, continuing")
					continue
				}
				if purged > 0 {
					rm.metricsRegistry.Incr("retention_purged_records_total", map[string]string{
						"JobType": jd.JobType,
						"State":   string(state),
					})
					logrus.WithFields(logrus.Fields{
						"Component": "RetentionManager",
						"JobType":   jd.JobType,
						"State":     state,
						"Purged":    purged,
						"OlderThan": olderThan.Format(time.RFC3339),
					}).Info("retention purge complete for job type")
				}
				totalPurged += purged
			}
		}

		if len(jobDefs) < pageSize {
			break
		}
	}

	elapsed := time.Since(start)
	logrus.WithFields(logrus.Fields{
		"Component":   "RetentionManager",
		"TotalPurged": totalPurged,
		"ElapsedMs":   elapsed.Milliseconds(),
	}).Info("retention purge run complete")

	if totalPurged > 10000 {
		logrus.WithField("TotalPurged", totalPurged).
			Warn("retention purge deleted >10k records — consider shortening retention windows or running more frequently")
	}

	return totalPurged, nil
}

// purgeState drains all batches for a single (jobType, state) pair.
func (rm *RetentionManager) purgeState(
	ctx context.Context,
	jobType string,
	state common.RequestState,
	olderThan time.Time,
) (int64, error) {
	const batchSize = 500
	var total int64
	for {
		if ctx.Err() != nil {
			return total, ctx.Err()
		}
		n, err := rm.jobRequestRepository.PurgeOldRequests(jobType, state, olderThan, batchSize)
		if err != nil {
			return total, err
		}
		total += n
		if n < batchSize {
			break
		}
	}
	return total, nil
}
