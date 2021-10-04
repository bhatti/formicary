package types

import (
	"context"
	"encoding/json"
	"fmt"
	"plexobject.com/formicary/internal/crypto"
	"plexobject.com/formicary/internal/utils"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// TaskAction defines enum for action of request
type TaskAction string

// ForkedJob constant
const ForkedJob = "ForkedJob"

const (
	// EXECUTE action
	EXECUTE TaskAction = "EXECUTE"
	// CANCEL action
	CANCEL TaskAction = "CANCEL"
	// TERMINATE action
	TERMINATE TaskAction = "TERMINATE_CONTAINER "
	// LIST action
	LIST TaskAction = "LIST_CONTAINERS"
)

// TaskRequest defines structure for incoming requests for task
// swagger:ignore
type TaskRequest struct {
	UserID          string                   `json:"user_id" yaml:"user_id"`
	OrganizationID  string                   `json:"organization_id" yaml:"organization_id"`
	JobDefinitionID string                   `json:"job_definition_id" yaml:"job_definition_id"`
	JobRequestID    uint64                   `json:"job_request_id" yaml:"job_request_id"`
	JobType         string                   `json:"job_type" yaml:"job_type"`
	JobTypeVersion  string                   `json:"job_type_version" yaml:"job_type_version"`
	JobExecutionID  string                   `json:"job_execution_id" yaml:"job_execution_id"`
	TaskExecutionID string                   `json:"task_execution_id" yaml:"task_execution_id"`
	TaskType        string                   `json:"task_type" yaml:"task_type"`
	Platform        string                   `json:"platform" yaml:"platform"`
	Action          TaskAction               `json:"action" yaml:"action"`
	JobRetry        int                      `json:"job_retry" yaml:"job_retry"`
	TaskRetry       int                      `json:"task_retry" yaml:"task_retry"`
	AllowFailure    bool                     `json:"allow_failure" yaml:"allow_failure"`
	Tags            []string                 `json:"tags" yaml:"tags"`
	BeforeScript    []string                 `json:"before_script" yaml:"before_script"`
	AfterScript     []string                 `json:"after_script" yaml:"after_script"`
	Script          []string                 `json:"script" yaml:"script"`
	Timeout         time.Duration            `json:"timeout" yaml:"timeout"`
	Variables       map[string]VariableValue `json:"variables" yaml:"variables"`
	ExecutorOpts    *ExecutorOptions         `json:"executor_opts" yaml:"executor_opts"`
	AdminUser       bool                     `json:"admin_user" yaml:"admin_user"`

	// Transient local properties for keeping track of request by ants
	StartedAt time.Time          `json:"-"`
	Status    RequestState       `json:"-"`
	Cancel    context.CancelFunc `json:"-"`
	Cancelled bool               `json:"-"`
}

// Key of task
func (req *TaskRequest) Key() string {
	return TaskKey(req.JobRequestID, req.TaskType)
}

// KeyPath key path of job
func (req *TaskRequest) KeyPath() string {
	return fmt.Sprintf("%sjob-%d/%s", utils.NormalizePrefix(req.UserID), req.JobRequestID, req.TaskType)
}

// TaskKey builds task key
func TaskKey(requestID uint64, taskType string) string {
	return fmt.Sprintf("%d-%s", requestID, taskType)
}

// String defines description of task request
func (req *TaskRequest) String() string {
	return fmt.Sprintf("ID=%d JobType=%s TaskType=%s Action=%s",
		req.JobRequestID, req.JobType, req.TaskType, req.Action)
}

// AddVariable adds variable or parameter to request
func (req *TaskRequest) AddVariable(name string, value interface{}, secret bool) {
	req.Variables[name] = NewVariableValue(value, secret)
}

// CacheArtifactID returns artifact-id for caching
func (req *TaskRequest) CacheArtifactID(prefix string, key string) string {
	if !req.ExecutorOpts.Cache.Valid() {
		return ""
	}
	userOrg := req.OrganizationID // share within org
	if userOrg == "" {
		userOrg = req.UserID
	}
	return utils.CacheArtifactID(
		prefix,
		userOrg,
		req.JobType,
		key,
	)
}

// GetMaskFields returns sensitive fields that will be filtered
func (req *TaskRequest) GetMaskFields() (res []string) {
	res = make([]string, 0)
	for _, v := range req.Variables {
		if v.Secret {
			res = append(res, fmt.Sprintf("%s", v.Value))
		}
	}
	return
}

// Mask string to hide sensitive data
func (req *TaskRequest) Mask(s string) string {
	if s == "" {
		return s
	}
	maskVars := req.GetMaskFields()
	if len(maskVars) == 0 {
		return s
	}
	for _, next := range maskVars {
		s = strings.ReplaceAll(s, next, "[MASKED]")
	}
	return s
}

// Validate validates
func (req *TaskRequest) Validate() error {
	if req.Action == "" {
		return fmt.Errorf("action is not specified")
	}
	if req.Action == EXECUTE || req.Action == CANCEL {
		if req.JobRequestID == 0 {
			return fmt.Errorf("requestID is not specified")
		}
		if req.JobExecutionID == "" {
			return fmt.Errorf("jobExecutionID is not specified")
		}
		if req.TaskExecutionID == "" {
			return fmt.Errorf("taskExecutionID is not specified")
		}
		if req.JobType == "" {
			return fmt.Errorf("jobType is not specified")
		}
		if req.TaskType == "" {
			return fmt.Errorf("taskType is not specified")
		}
		if req.ExecutorOpts.Method.RequiresScript() {
			if req.Script == nil || len(req.Script) == 0 {
				return fmt.Errorf("script is not specified")
			}
		}
		if err := req.ExecutorOpts.Validate(); err != nil {
			return err
		}
	}
	if req.BeforeScript == nil {
		req.BeforeScript = make([]string, 0)
	}
	if req.AfterScript == nil {
		req.AfterScript = make([]string, 0)
	}
	return nil
}

// Marshal converts task request to byte array
func (req *TaskRequest) Marshal(
	encryptionKeyStr string,
) (b []byte, err error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	b, err = json.Marshal(req)
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

// UnmarshalTaskRequest converts byte array to task request
func UnmarshalTaskRequest(
	encryptionKeyStr string,
	payload []byte) (req *TaskRequest, err error) {
	if encryptionKeyStr != "" {
		encryptionKey := crypto.SHA256Key(encryptionKeyStr)
		var dec []byte
		if dec, err = crypto.Decrypt(encryptionKey, payload); err != nil {
			return nil, err
		}
		payload = dec
	}

	req = &TaskRequest{}
	if err := json.Unmarshal(payload, req); err != nil {
		logrus.WithFields(
			logrus.Fields{
				"Component": "UnmarshalTaskRequest",
				"Payload":   string(payload),
				"Error":     err,
			}).Error("failed to unmarshal task request")
		return nil, err
	}

	if err := req.Validate(); err != nil {
		logrus.WithFields(
			logrus.Fields{
				"Component":       "UnmarshalTaskRequest",
				"RequestID":       req.JobRequestID,
				"JobType":         req.JobType,
				"TaskType":        req.TaskType,
				"TaskExecutionID": req.TaskExecutionID,
				"Params":          req.Variables,
				"Error":           err,
			}).Error("failed to validate task request")
		return nil, err
	}
	return req, nil
}
