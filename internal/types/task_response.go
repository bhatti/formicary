package types

import (
	"encoding/json"
	"fmt"
	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/crypto"
	"runtime/debug"
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
	return fmt.Sprintf("Container-Startup: %s, PreScript: %s, Artifacts-Download: %s, Script: %s, Post-Script: %s, Artifacts-Upload: %s, Container-Shutdown: %s",
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

// TaskResponse outlines the outcome of a task execution, encompassing its status, context, generated artifacts,
// and additional outputs.
// swagger:ignore
type TaskResponse struct {
	JobRequestID    string                 `json:"job_request_id"`
	TaskExecutionID string                 `json:"task_execution_id"`
	JobType         string                 `json:"job_type"`
	JobTypeVersion  string                 `json:"job_type_version"`
	TaskType        string                 `json:"task_type"`
	CoRelationID    string                 `json:"co_relation_id"`
	Status          RequestState           `json:"status"`
	AntID           string                 `json:"ant_id"`
	Host            string                 `json:"host"`
	Namespace       string                 `json:"namespace"`
	Tags            []string               `json:"tags"`
	ErrorMessage    string                 `json:"error_message"`
	ErrorCode       string                 `json:"error_code"`
	ExitCode        string                 `json:"exit_code"`
	ExitMessage     string                 `json:"exit_message"`
	FailedCommand   string                 `json:"failed_command"`
	TaskContext     map[string]interface{} `json:"task_context"`
	JobContext      map[string]interface{} `json:"job_context"`
	Artifacts       []*Artifact            `json:"artifacts"`
	Warnings        []string               `json:"warnings"`
	Stdout          []string               `json:"stdout"`
	CostFactor      float64                `json:"cost_factor"`
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
		CoRelationID:    req.CoRelationID,
		Tags:            []string{},
		Status:          UNKNOWN,
		TaskContext:     make(map[string]interface{}),
		JobContext:      make(map[string]interface{}),
		Artifacts:       make([]*Artifact, 0),
		Warnings:        make([]string, 0),
	}
}

// String defines description of task response
func (res *TaskResponse) String() string {
	return fmt.Sprintf("ID=%s CoRelID=%s TaskType=%s Status=%s Exit=%s TaskContext=%d Artifacts=%d Error=%s %s",
		res.JobRequestID, res.CoRelationID, res.TaskType, res.Status, res.ExitCode, len(res.TaskContext),
		len(res.Artifacts), res.ErrorCode, res.ErrorMessage)
}

// Validate validates
func (res *TaskResponse) Validate() error {
	if res.TaskExecutionID == "" {
		return fmt.Errorf("taskExecutionID is not specified in task-response")
	}
	if res.JobType == "" {
		return fmt.Errorf("jobType is not specified in task-response")
	}
	if res.TaskType == "" {
		return fmt.Errorf("taskType is not specified in task-response")
	}
	if res.Status == "" {
		return fmt.Errorf("status is not specified in task-response")
	}
	//if res.AntID == "" {
	//	return fmt.Errorf("antID is not specified in task-response")
	//}
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

// Marshal converts task request to byte array
func (res *TaskResponse) Marshal(
	encryptionKeyStr string,
) (b []byte, err error) {
	if err := res.Validate(); err != nil {
		return nil, err
	}
	b, err = json.Marshal(res)
	if err != nil {
		return nil, err
	}
	if encryptionKeyStr != "" {
		encryptionKey := crypto.SHA256Key(encryptionKeyStr)
		if b, err = crypto.Encrypt(encryptionKey, b); err != nil {
			return nil, err
		}
		return b, nil
	}
	return b, nil
}

// UnmarshalTaskResponse converts byte array to task response
func UnmarshalTaskResponse(
	encryptionKeyStr string,
	payload []byte) (res *TaskResponse, err error) {
	if encryptionKeyStr != "" {
		encryptionKey := crypto.SHA256Key(encryptionKeyStr)
		var dec []byte
		if dec, err = crypto.Decrypt(encryptionKey, payload); err != nil {
			return nil, err
		}
		payload = dec
	}

	res = &TaskResponse{}
	if err := json.Unmarshal(payload, res); err != nil {
		logrus.WithFields(
			logrus.Fields{
				"Component": "UnmarshalTaskResponse",
				"Payload":   string(payload),
				"Error":     err,
			}).Error("failed to unmarshal task response")
		return nil, err
	}

	if err := res.Validate(); err != nil {
		debug.PrintStack()
		logrus.WithFields(
			logrus.Fields{
				"Component":       "UnmarshalTaskResponse",
				"RequestID":       res.JobRequestID,
				"JobType":         res.JobType,
				"TaskType":        res.TaskType,
				"TaskExecutionID": res.TaskExecutionID,
				"Payload":         string(payload),
				"Error":           err,
			}).Error("failed to validate task response")
		return nil, err
	}
	return res, nil
}
