package types

import (
	"errors"
	"fmt"
	common "plexobject.com/formicary/internal/types"
	"time"
)

// JobDefinitionConfig defines variables for job definition
type JobDefinitionConfig struct {
	//gorm.Model
	// Inheriting name, value, type
	common.NameTypeValue
	// ID defines UUID for primary key
	ID string `yaml:"-" json:"id" gorm:"primary_key"`
	// JobDefinitionID defines foreign key for JobDefinition
	JobDefinitionID string `yaml:"-" json:"job_definition_id"`
	// CreatedAt job creation time
	CreatedAt time.Time `yaml:"-" json:"created_at"`
	// UpdatedAt job update time
	UpdatedAt time.Time         `yaml:"-" json:"updated_at"`
	Errors    map[string]string `yaml:"-" json:"-" gorm:"-"`
}

// TableName overrides default table name
func (JobDefinitionConfig) TableName() string {
	return "formicary_job_definition_configs"
}

// NewJobDefinitionConfig creates new job variable
func NewJobDefinitionConfig(name string, value interface{}, secret bool) (*JobDefinitionConfig, error) {
	nv, err := common.NewNameTypeValue(name, value, secret)
	if err != nil {
		return nil, err
	}
	return &JobDefinitionConfig{
		NameTypeValue: nv,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}, nil
}

func (u *JobDefinitionConfig) String() string {
	return fmt.Sprintf("%s=%s", u.Name, u.Value)
}

// ValidateBeforeSave validates before save
func (u *JobDefinitionConfig) ValidateBeforeSave(key []byte) error {
	if err := u.Validate(); err != nil {
		return err
	}
	return u.Encrypt(key)
}

// Validate validates job-config
func (u *JobDefinitionConfig) Validate() (err error) {
	u.Errors = make(map[string]string)
	if u.ID != "" && u.JobDefinitionID == "" {
		err = errors.New("job-definition-id is not specified")
		u.Errors["JobDefinitionID"] = err.Error()
	}
	if u.Name == "" {
		err = errors.New("name is not specified")
		u.Errors["Name"] = err.Error()
	}
	if u.Kind == "" {
		err = errors.New("type is not specified")
		u.Errors["Kind"] = err.Error()
	}
	if u.Value == "" {
		err = errors.New("value is not specified")
		u.Errors["Value"] = err.Error()
	}
	if len(u.Value) > maxConfigValueLength {
		err = errors.New("value is too big")
		u.Errors["Value"] = err.Error()
	}
	return
}
