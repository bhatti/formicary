package types

import (
	"fmt"
	"strings"
	"time"
)

// TaskResponseTimings timings in response
type TaskResponseTimings struct {
	ReceivedAt                     time.Time `json:"received_at"`
	PodStartedAt                   time.Time `json:"pod_started_at"`
	PreScriptFinishedAt            time.Time `json:"pre_script_finished_at"`
	DependentArtifactsDownloadedAt time.Time `json:"dependent_artifacts_downloaded_at"`
	ScriptFinishedAt               time.Time `json:"script_finished_at"`
	PostScriptFinishedAt           time.Time `json:"post_script_finished_at"`
	ArtifactsUploadedAt            time.Time `json:"artifacts_uploaded_at"`
	PodShutdownAt                  time.Time `json:"pod_shutdown_at"`
}

func (t TaskResponseTimings) String() string {
	return fmt.Sprintf("POD-Startup: %s, PreScript: %s, Artifacts-Download: %s, Script: %s, Post-Script: %s, Artifacts-Upload: %s, POD-Shutdown: %s",
		t.PodStartupDuration(),
		t.PreScriptDuration(),
		t.DependentArtifactsDownloadedDuration(),
		t.ScriptFinishedDuration(),
		t.PostScriptFinishedDuration(),
		t.ArtifactsUploadedDuration(),
		t.PodShutdownDuration())
}

// PodStartupDuration time
func (t TaskResponseTimings) PodStartupDuration() time.Duration {
	return t.PodStartedAt.Sub(t.ReceivedAt)
}

// PreScriptDuration time
func (t TaskResponseTimings) PreScriptDuration() time.Duration {
	return t.PreScriptFinishedAt.Sub(t.PodStartedAt)
}

// DependentArtifactsDownloadedDuration time
func (t TaskResponseTimings) DependentArtifactsDownloadedDuration() time.Duration {
	return t.DependentArtifactsDownloadedAt.Sub(t.PreScriptFinishedAt)
}

// ScriptFinishedDuration time
func (t TaskResponseTimings) ScriptFinishedDuration() time.Duration {
	return t.ScriptFinishedAt.Sub(t.DependentArtifactsDownloadedAt)
}

// PostScriptFinishedDuration time
func (t TaskResponseTimings) PostScriptFinishedDuration() time.Duration {
	return t.PostScriptFinishedAt.Sub(t.ScriptFinishedAt)
}

// ArtifactsUploadedDuration time
func (t TaskResponseTimings) ArtifactsUploadedDuration() time.Duration {
	return t.ArtifactsUploadedAt.Sub(t.PostScriptFinishedAt)
}

// PodShutdownDuration time
func (t TaskResponseTimings) PodShutdownDuration() time.Duration {
	return t.PodShutdownAt.Sub(t.ArtifactsUploadedAt)
}

// TaskResponse defines structure for response from ant
// swagger:ignore
type TaskResponse struct {
	JobRequestID    uint64                 `json:"job_request_id"`
	TaskExecutionID string                 `json:"task_id"`
	JobType         string                 `json:"job_type"`
	JobTypeVersion  string                 `json:"job_type_version"`
	TaskType        string                 `json:"task_type"`
	Status          RequestState           `json:"status"`
	AntID           string                 `json:"ant_id"`
	Host            string                 `json:"host"`
	Namespace       string                 `json:"namespace"`
	Tags            []string               `json:"tags"`
	ErrorMessage    string                 `json:"error_message"`
	ErrorCode       string                 `json:"error_code"`
	ExitCode        string                 `json:"exit_code"`
	ExitMessage     string                 `json:"exit_message"`
	TaskContext     map[string]interface{} `json:"task_context"`
	JobContext      map[string]interface{} `json:"job_context"`
	Artifacts       []*Artifact            `json:"artifacts"`
	Warnings        []string               `json:"warnings"`
	AppliedCost     float64                `json:"applied_cost"`
	Timings         TaskResponseTimings    `json:"timings"`
}

// NewTaskResponse creates new instance
func NewTaskResponse(req *TaskRequest) *TaskResponse {
	return &TaskResponse{
		JobRequestID:    req.JobRequestID,
		JobType:         req.JobType,
		JobTypeVersion:  req.JobTypeVersion,
		TaskExecutionID: req.TaskExecutionID,
		TaskType:        req.TaskType,
		Tags:            []string{},
		Status:          COMPLETED,
		TaskContext:     make(map[string]interface{}),
		JobContext:      make(map[string]interface{}),
		Artifacts:       make([]*Artifact, 0),
		Warnings:        make([]string, 0),
	}
}

// String defines description of task response
func (res *TaskResponse) String() string {
	return fmt.Sprintf("ID=%d TaskType=%s Status=%s Exit=%s TaskContext=%d Artifacts=%d Error=%s %s",
		res.JobRequestID, res.TaskType, res.Status, res.ExitCode, len(res.TaskContext),
		len(res.Artifacts), res.ErrorCode, res.ErrorMessage)
}

// Validate validates
func (res *TaskResponse) Validate() error {
	if res.JobRequestID == 0 {
		return fmt.Errorf("requestID is not specified")
	}
	if res.TaskExecutionID == "" {
		return fmt.Errorf("taskExecutionID is not specified")
	}
	if res.JobType == "" {
		return fmt.Errorf("jobType is not specified")
	}
	if res.TaskType == "" {
		return fmt.Errorf("taskType is not specified")
	}
	if res.Status == "" {
		return fmt.Errorf("status is not specified")
	}
	if res.AntID == "" {
		return fmt.Errorf("antID is not specified")
	}
	if res.Host == "" {
		return fmt.Errorf("host is not specified")
	}
	return nil
}

// AddContext adds context
func (res *TaskResponse) AddContext(name string, value interface{}) {
	res.TaskContext[name] = value
}

// AddJobContext adds context for job
func (res *TaskResponse) AddJobContext(name string, value interface{}) {
	res.JobContext[name] = value
}

// AddArtifact adds artifact
func (res *TaskResponse) AddArtifact(artifact *Artifact) {
	res.Artifacts = append(res.Artifacts, artifact)
}

// AdditionalError adds additional errors
func (res *TaskResponse) AdditionalError(warning string, fatal bool) {
	warning = strings.TrimSpace(warning)
	if warning == "" {
		return
	}
	if fatal && res.Status != FAILED {
		res.ErrorMessage = warning
		res.Status = FAILED
	} else {
		res.Warnings = append(res.Warnings, warning)
	}
}
