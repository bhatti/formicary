// SPDX-License-Identifier: AGPL-3.0-or-later
// Hand-written extension methods for proto-generated JobResource, JobResourceConfig, JobResourceUse.
// This file is NEVER overwritten by buf generate.

package queen

import (
	"errors"
	"fmt"
	"time"
)

// TableName implements the GORM Tabler interface.
func (*JobResource) TableName() string { return "formicary_job_resources" }

// TableName implements the GORM Tabler interface.
func (*JobResourceConfig) TableName() string { return "formicary_job_resource_config" }

// TableName implements the GORM Tabler interface.
func (*JobResourceUse) TableName() string { return "formicary_job_resource_uses" }

// ──────────────────────────────────────────────────────────────────────────────
// JobResource
// ──────────────────────────────────────────────────────────────────────────────

// AddConfig adds or updates a named configuration property on the resource.
func (jr *JobResource) AddConfig(name string, value string, kind string, secret bool) *JobResourceConfig {
	for _, c := range jr.Configs {
		if c.Name == name {
			c.Value = value
			c.Kind = kind
			c.Secret = secret
			return c
		}
	}
	cfg := &JobResourceConfig{
		Id:            ulid(),
		JobResourceId: jr.Id,
		Name:          name,
		Value:         value,
		Kind:          kind,
		Secret:        secret,
		CreatedAt:     nowTimestamp(),
		UpdatedAt:     nowTimestamp(),
	}
	jr.Configs = append(jr.Configs, cfg)
	return cfg
}

// GetConfig returns a named configuration property, or nil if not found.
func (jr *JobResource) GetConfig(name string) *JobResourceConfig {
	for _, c := range jr.Configs {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// DeleteConfig removes a named configuration property from the resource.
func (jr *JobResource) DeleteConfig(name string) bool {
	for i, c := range jr.Configs {
		if c.Name == name {
			jr.Configs = append(jr.Configs[:i], jr.Configs[i+1:]...)
			return true
		}
	}
	return false
}

// Validate checks required fields on the job resource.
func (jr *JobResource) Validate() error {
	if jr.Basic == nil || jr.Basic.ResourceType == "" {
		return errors.New("resourceType is not specified")
	}
	if jr.Quota == 0 {
		return errors.New("quota is not specified")
	}
	return nil
}

// ValidateBeforeSave validates the job resource before persistence.
func (jr *JobResource) ValidateBeforeSave() error {
	if err := jr.Validate(); err != nil {
		return err
	}
	for _, c := range jr.Configs {
		if err := c.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// LeaseTimeout returns the configured lease timeout as a time.Duration.
func (jr *JobResource) LeaseTimeout() time.Duration {
	if jr.LeaseTimeoutNs > 0 {
		return time.Duration(jr.LeaseTimeoutNs)
	}
	return 60 * time.Second
}

// Summary returns a short human-readable description of the resource.
func (jr *JobResource) Summary() string {
	rt := ""
	if jr.Basic != nil {
		rt = jr.Basic.ResourceType
	}
	return fmt.Sprintf("JobResource[type=%s quota=%d]", rt, jr.Quota)
}

// ──────────────────────────────────────────────────────────────────────────────
// JobResourceConfig
// ──────────────────────────────────────────────────────────────────────────────

// Validate checks required fields on the resource config.
func (rc *JobResourceConfig) Validate() error {
	rc.Errors = make(map[string]string)
	var err error
	if rc.JobResourceId == "" {
		err = errors.New("job-resource-id is not specified")
		rc.Errors["JobResourceId"] = err.Error()
	}
	if rc.Name == "" {
		err = errors.New("name is not specified")
		rc.Errors["Name"] = err.Error()
	}
	if rc.Kind == "" {
		err = errors.New("type is not specified")
		rc.Errors["Kind"] = err.Error()
	}
	if rc.Value == "" {
		err = errors.New("value is not specified")
		rc.Errors["Value"] = err.Error()
	}
	return err
}

// ──────────────────────────────────────────────────────────────────────────────
// JobResourceUse
// ──────────────────────────────────────────────────────────────────────────────

// Validate checks required fields on the resource use record.
func (ru *JobResourceUse) Validate() error {
	if ru.Value == 0 {
		return errors.New("value is not specified")
	}
	if ru.JobResourceId == "" {
		return errors.New("job-resource-id is not specified")
	}
	if ru.JobRequestId == 0 {
		return errors.New("job-request-id is not specified")
	}
	if ru.TaskExecutionId == "" {
		return errors.New("task-id is not specified")
	}
	if ru.ExpiresAt == nil {
		return errors.New("expiration is not specified")
	}
	return nil
}

// ValidateBeforeSave validates the resource use before persistence.
func (ru *JobResourceUse) ValidateBeforeSave() error {
	return ru.Validate()
}

// Summary returns a short human-readable description of the resource use.
func (ru *JobResourceUse) Summary() string {
	return fmt.Sprintf("JobResourceUse[resourceID=%s jobRequestID=%d taskID=%s user=%s value=%d]",
		ru.JobResourceId, ru.JobRequestId, ru.TaskExecutionId, ru.UserId, ru.Value)
}
