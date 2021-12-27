package types

import (
	common "plexobject.com/formicary/internal/types"
	"time"
)

// JobDefinitionVariable defines variables for job definition
type JobDefinitionVariable struct {
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
	UpdatedAt time.Time `yaml:"-" json:"updated_at"`
}

// TableName overrides default table name
func (JobDefinitionVariable) TableName() string {
	return "formicary_job_definition_variables"
}

// NewJobDefinitionVariable creates new job variable
func NewJobDefinitionVariable(name string, value interface{}) (*JobDefinitionVariable, error) {
	nv, err := common.NewNameTypeValue(name, value, false)
	if err != nil {
		return nil, err
	}
	return &JobDefinitionVariable{
		NameTypeValue: nv,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}, nil
}
