// SPDX-License-Identifier: AGPL-3.0-or-later
// Hand-written extension methods for proto-generated JobRequest and JobRequestParam.
// This file is NEVER overwritten by buf generate.

package queen

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// TableName implements the GORM Tabler interface.
func (*JobRequest) TableName() string { return "formicary_job_requests" }

// TableName implements the GORM Tabler interface.
func (*JobRequestParam) TableName() string { return "formicary_job_request_params" }

// ──────────────────────────────────────────────────────────────────────────────
// Param cache (keyed by request ID, stores map[name]*JobRequestParam)
// ──────────────────────────────────────────────────────────────────────────────

var jrParamCache sync.Map // map[string]map[string]*JobRequestParam

func jrParams(id string) map[string]*JobRequestParam {
	if v, ok := jrParamCache.Load(id); ok {
		return v.(map[string]*JobRequestParam)
	}
	m := make(map[string]*JobRequestParam)
	jrParamCache.Store(id, m)
	return m
}

// AfterLoad initialises the param lookup cache from the persisted Params slice.
func (jr *JobRequest) AfterLoad() error {
	cache := make(map[string]*JobRequestParam, len(jr.Params))
	for _, p := range jr.Params {
		cache[p.Name] = p
	}
	jrParamCache.Store(jr.Id, cache)
	return nil
}

// ──────────────────────────────────────────────────────────────────────────────
// State-machine helpers
// ──────────────────────────────────────────────────────────────────────────────

// Pending returns true if the request is in PENDING state.
func (jr *JobRequest) Pending() bool { return jr.JobState == "PENDING" }

// Running returns true if the request is currently executing.
func (jr *JobRequest) Running() bool {
	return jr.JobState == "EXECUTING" || jr.JobState == "STARTED" || jr.JobState == "RUNNING"
}

// Waiting returns true if the request is waiting.
func (jr *JobRequest) Waiting() bool { return jr.JobState == "WAITING" }

// Done returns true if the request has reached a terminal state.
func (jr *JobRequest) Done() bool { return isTerminalState(jr.JobState) }

// Completed returns true if the job completed successfully.
func (jr *JobRequest) Completed() bool { return jr.JobState == "COMPLETED" }

// Failed returns true if the job failed.
func (jr *JobRequest) Failed() bool { return jr.JobState == "FAILED" || jr.JobState == "FATAL" }

// IsTerminal returns true if the job has reached a terminal state.
func (jr *JobRequest) IsTerminal() bool { return isTerminalState(jr.JobState) }

// NotTerminal returns true if the job has not yet reached a terminal state.
func (jr *JobRequest) NotTerminal() bool { return !isTerminalState(jr.JobState) }

// CanRestart returns true if the job can be restarted.
func (jr *JobRequest) CanRestart() bool { return canRestart(jr.JobState) }

// CanCancel returns true if the job can be cancelled.
func (jr *JobRequest) CanCancel() bool { return canCancel(jr.JobState) }

// CanApprove returns true if the job awaits manual approval.
func (jr *JobRequest) CanApprove() bool { return canApprove(jr.JobState) }

// Editable returns true if the given user/org can modify this request.
func (jr *JobRequest) Editable(userID string, organizationID string) bool {
	if jr.OrganizationId != "" || organizationID != "" {
		return jr.OrganizationId == organizationID
	}
	return jr.UserId == userID
}

// ──────────────────────────────────────────────────────────────────────────────
// Param management
// ──────────────────────────────────────────────────────────────────────────────

// AddParam adds or updates a parameter on the request.
func (jr *JobRequest) AddParam(name string, value interface{}) (*JobRequestParam, error) {
	strVal := fmt.Sprintf("%v", value)
	cache := jrParams(jr.Id)
	if existing, ok := cache[name]; ok {
		existing.Value = strVal
		return existing, nil
	}
	p := &JobRequestParam{
		Id:           ulid(),
		JobRequestId: jr.Id,
		Name:         name,
		Value:        strVal,
		Kind:         kindOf(value),
	}
	jr.Params = append(jr.Params, p)
	cache[name] = p
	return p, nil
}

// GetParam returns a parameter value by name, or empty string if not found.
func (jr *JobRequest) GetParam(name string) string {
	cache := jrParams(jr.Id)
	if p, ok := cache[name]; ok {
		return p.Value
	}
	return ""
}

// GetParamOrDefault returns a parameter value or the provided default.
func (jr *JobRequest) GetParamOrDefault(name string, def string) string {
	if v := jr.GetParam(name); v != "" {
		return v
	}
	return def
}

// SetParams replaces all parameters on the request (updates both slice and cache).
func (jr *JobRequest) SetParams(params []*JobRequestParam) {
	jr.Params = params
	cache := make(map[string]*JobRequestParam, len(params))
	for _, p := range params {
		cache[p.Name] = p
	}
	jrParamCache.Store(jr.Id, cache)
}

// ClearParams removes all parameters.
func (jr *JobRequest) ClearParams() {
	jr.Params = nil
	jrParamCache.Store(jr.Id, make(map[string]*JobRequestParam))
}

// ParamString returns a compact string representation of all params.
func (jr *JobRequest) ParamString() string {
	var b strings.Builder
	for _, p := range jr.Params {
		b.WriteString(p.Name + "=" + p.Value + " ")
	}
	return b.String()
}

// GetParamsJSON returns params serialized as a JSON object.
func (jr *JobRequest) GetParamsJSON() (string, error) {
	m := make(map[string]interface{}, len(jr.Params))
	for _, p := range jr.Params {
		m[p.Name] = p.Value
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// SetParamsJSON deserializes a JSON object into params.
func (jr *JobRequest) SetParamsJSON(raw string) error {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return err
	}
	for k, v := range m {
		if _, err := jr.AddParam(k, v); err != nil {
			return err
		}
	}
	return nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Formatting helpers
// ──────────────────────────────────────────────────────────────────────────────

// ElapsedDuration returns human-readable elapsed time since the request was created.
func (jr *JobRequest) ElapsedDuration() string {
	if jr.CreatedAt == nil {
		return ""
	}
	return time.Since(jr.CreatedAt.AsTime()).String()
}

// ShortUserID returns the first 8 chars of the user ID.
func (jr *JobRequest) ShortUserID() string {
	if len(jr.UserId) > 8 {
		return jr.UserId[0:8] + "..."
	}
	return jr.UserId
}

// ShortJobType returns the first 20 chars of the job type.
func (jr *JobRequest) ShortJobType() string {
	if len(jr.JobType) > 20 {
		return jr.JobType[0:20] + "..."
	}
	return jr.JobType
}

// JobTypeAndVersion returns "type:version" or just "type" if no version.
func (jr *JobRequest) JobTypeAndVersion() string {
	if jr.JobVersion == "" {
		return jr.JobType
	}
	return jr.JobType + ":" + jr.JobVersion
}

// UpdatedAtString formats the UpdatedAt timestamp.
func (jr *JobRequest) UpdatedAtString() string {
	if jr.UpdatedAt == nil {
		return ""
	}
	return jr.UpdatedAt.AsTime().Format("2006-01-02 15:04:05")
}

// ScheduledAtString formats the ScheduledAt timestamp.
func (jr *JobRequest) ScheduledAtString() string {
	if jr.ScheduledAt == nil {
		return ""
	}
	return jr.ScheduledAt.AsTime().Format("2006-01-02 15:04:05")
}

// GetUserJobTypeKey returns a unique key combining org/user and job type.
func (jr *JobRequest) GetUserJobTypeKey() string {
	return getUserJobTypeKey(jr.OrganizationId, jr.UserId, jr.JobType, jr.JobVersion)
}

// getUserJobTypeKey is defined in ext_helpers.go.

// Summary returns a short human-readable description of the request.
func (jr *JobRequest) Summary() string {
	return fmt.Sprintf("JobRequest[id=%s type=%s state=%s user=%s]",
		jr.Id, jr.JobType, jr.JobState, jr.UserId)
}

// ──────────────────────────────────────────────────────────────────────────────
// IJobRequest implementation helpers (for compatibility)
// ──────────────────────────────────────────────────────────────────────────────

// IncrRetried increments and returns the retry count.
func (jr *JobRequest) IncrRetried() int32 {
	jr.Retried++
	return jr.Retried
}

// SetJobState updates the job state.
func (jr *JobRequest) SetJobState(state string) {
	jr.JobState = state
}

// SetJobExecutionID sets the current job execution ID.
func (jr *JobRequest) SetJobExecutionID(id string) {
	jr.JobExecutionId = id
}

// ──────────────────────────────────────────────────────────────────────────────
// Validation
// ──────────────────────────────────────────────────────────────────────────────

// Validate checks required fields on the job request.
func (jr *JobRequest) Validate() error {
	jr.Errors = make(map[string]string)
	var err error
	if jr.JobType == "" {
		err = errors.New("jobType is not specified")
		jr.Errors["JobType"] = err.Error()
	}
	if jr.UserKey == "" && jr.CronTriggered {
		err = errors.New("userKey is required for cron-triggered jobs")
		jr.Errors["UserKey"] = err.Error()
	}
	return err
}

// ValidateBeforeSave validates the job request before persistence.
func (jr *JobRequest) ValidateBeforeSave() error {
	return jr.Validate()
}

// ──────────────────────────────────────────────────────────────────────────────
// Quick-search update
// ──────────────────────────────────────────────────────────────────────────────

// UpdateQuickSearch rebuilds the quick-search text field from all param values.
// This enables substring search across job parameters without JSONB.
func (jr *JobRequest) UpdateQuickSearch() {
	var parts []string
	for _, p := range jr.Params {
		if p.Value != "" {
			parts = append(parts, p.Value)
		}
	}
	jr.QuickSearch = strings.Join(parts, " ")
}

// kindOf and getUserJobTypeKey are defined in ext_helpers.go.
