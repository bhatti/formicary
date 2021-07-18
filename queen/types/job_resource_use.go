package types

import (
	"errors"
	"fmt"
	"time"
)

// JobResourceUse defines use of a job resource
type JobResourceUse struct {
	//gorm.Model
	// ID defines UUID for primary key
	ID string `yaml:"-" json:"id" gorm:"primary_key"`
	// JobResourceID defines foreign key for JobResource
	JobResourceID string `yaml:"-" json:"job_resource_id"`
	// JobRequestID - id for the job that consumed it
	JobRequestID uint64 `yaml:"-" json:"job_request_id"`
	// TaskExecutionID - id for the task that consumed it
	TaskExecutionID string `yaml:"-" json:"task_execution_id"`
	// UserID - user id who consumed it
	UserID string `yaml:"-" json:"user_id"`
	// Value consumed, e.g. it will be 1 for mutex, semaphore but can be higher number for other quota system
	Value int `yaml:"value" json:"value"`
	// Aborted if resource was take forcefully
	Aborted bool `yaml:"aborted" json:"aborted"`
	// Active is used to soft delete job resource use
	Active bool `yaml:"-" json:"-"`
	// CreatedAt job creation time
	CreatedAt time.Time `yaml:"-" json:"created_at"`
	// UpdatedAt job update time
	UpdatedAt time.Time `yaml:"-" json:"updated_at"`
	// ExpiresAt - expiration time
	ExpiresAt time.Time `yaml:"-" json:"expires_at"`
	// RemainingQuota
	RemainingQuota int `yaml:"-" json:"-" gorm:"-"`
}

// NewJobResourceUse constructor
func NewJobResourceUse(
	resourceID string,
	requestID uint64,
	taskID string,
	userID string,
	value int,
	expiration time.Time) *JobResourceUse {
	return &JobResourceUse{
		JobResourceID:   resourceID,
		JobRequestID:    requestID,
		TaskExecutionID: taskID,
		UserID:          userID,
		Value:           value,
		ExpiresAt:       expiration,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
}

// TableName overrides default table name
func (JobResourceUse) TableName() string {
	return "formicary_job_resource_uses"
}

// String provides short summary of task
func (jru *JobResourceUse) String() string {
	return fmt.Sprintf("ResourceID=%s JobRequestID=%d TaskExecutionID=%s User=%s Value=%d",
		jru.JobResourceID, jru.JobRequestID, jru.TaskExecutionID, jru.UserID, jru.Value)
}

// Validate validates task
func (jru *JobResourceUse) Validate() error {
	if jru.Value == 0 {
		return errors.New("value is not specified")
	}
	if jru.JobResourceID == "" {
		return errors.New("job-resource-id is not specified")
	}
	if jru.JobRequestID == 0 {
		return errors.New("job-request-id is not specified")
	}
	if jru.TaskExecutionID == "" {
		return errors.New("task-id is not specified")
	}
	if jru.ExpiresAt.IsZero() {
		return errors.New("expiration is not specified")
	}
	return nil
}

// ValidateBeforeSave validates task
func (jru *JobResourceUse) ValidateBeforeSave() error {
	if err := jru.Validate(); err != nil {
		return err
	}
	return nil
}
