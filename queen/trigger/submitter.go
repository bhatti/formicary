// SPDX-License-Identifier: AGPL-3.0-or-later

package trigger

import (
	"context"
	"strings"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"plexobject.com/formicary/internal/tracing"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/types"
)

// Submitter creates a JobRequest from a trigger EvalResult and saves it.
type Submitter struct {
	jobManager       *manager.JobManager
	triggerStateRepo repository.TriggerStateRepository
}

// NewSubmitter creates a Submitter backed by the given JobManager.
func NewSubmitter(jm *manager.JobManager, repo repository.TriggerStateRepository) *Submitter {
	return &Submitter{jobManager: jm, triggerStateRepo: repo}
}

// Submit creates and saves a JobRequest. Returns nil (no error) when the request
// was deduplicated (UserKey already exists), so callers must check for nil return.
func (s *Submitter) Submit(ctx context.Context, jobDef *types.JobDefinition, triggerName string, result *EvalResult) (*types.JobRequest, error) {
	ctx, span := tracing.Tracer("formicary.trigger").Start(ctx, "trigger.submit_job",
		trace.WithAttributes(
			attribute.String("job.type", jobDef.JobType),
			attribute.String("trigger.name", triggerName),
			attribute.String("trigger.dedup_key", result.DedupKey),
		),
	)
	defer func() { span.End() }()

	req, err := types.NewJobRequestFromDefinition(jobDef)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	for k, v := range result.Params {
		if _, err = req.AddParam(k, v); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, err
		}
	}
	if result.DedupKey != "" {
		req.UserKey = result.DedupKey
	}

	qc := common.NewQueryContextFromIDs(jobDef.UserID, jobDef.OrganizationID)
	saved, err := s.jobManager.SaveJobRequest(qc, req)
	if err != nil {
		if isDuplicateKeyError(err) {
			logrus.WithFields(logrus.Fields{
				"Component":   "TriggerSubmitter",
				"JobType":     jobDef.JobType,
				"TriggerName": triggerName,
				"DedupKey":    result.DedupKey,
			}).Infof("trigger deduped: job request with UserKey already exists")
			return nil, nil
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	logrus.WithFields(logrus.Fields{
		"Component":   "TriggerSubmitter",
		"JobType":     jobDef.JobType,
		"TriggerName": triggerName,
		"RequestID":   saved.ID,
	}).Infof("trigger fired, job request created")

	// Record the fire time so the dashboard can show "Last Fired".
	if s.triggerStateRepo != nil {
		if err := s.triggerStateRepo.RecordFired(jobDef.ID, triggerName); err != nil {
			logrus.WithFields(logrus.Fields{
				"Component":   "TriggerSubmitter",
				"JobType":     jobDef.JobType,
				"TriggerName": triggerName,
			}).Warnf("failed to record trigger fire time: %v", err)
		}
	}

	return saved, nil
}

func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "Duplicate entry") ||
		strings.Contains(msg, "UNIQUE constraint failed") ||
		strings.Contains(msg, "duplicate key value")
}
