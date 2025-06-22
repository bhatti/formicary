package types

import (
	"errors"
	"fmt"
	"time"
)

// OrganizationConfig defines common configuration for the organization that can be shared by all jobs defined by users within that organization.
type OrganizationConfig struct {
	//gorm.Model
	// Inheriting name, value, type
	NameTypeValue
	// ID defines UUID for primary key
	ID string `yaml:"-" json:"id" gorm:"primary_key"`
	// OrganizationID defines foreign key for Organization
	OrganizationID string `yaml:"-" json:"organization_id"`
	// CreatedAt job creation time
	CreatedAt time.Time `yaml:"-" json:"created_at"`
	// UpdatedAt job update time
	UpdatedAt time.Time         `yaml:"-" json:"updated_at"`
	Errors    map[string]string `yaml:"-" json:"-" gorm:"-"`
}

// TableName overrides default table name
func (OrganizationConfig) TableName() string {
	return "formicary_org_configs"
}

func (u *OrganizationConfig) String() string {
	if u.Secret {
		return fmt.Sprintf("%s=*****", u.Name)
	}
	return fmt.Sprintf("%s=%s", u.Name, u.Value)
}

// Validate validates organization
func (u *OrganizationConfig) Validate() (err error) {
	u.Errors = make(map[string]string)
	if u.OrganizationID == "" {
		err = errors.New("org-id is not specified")
		u.Errors["OrganizationID"] = err.Error()
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

	return
}

// AfterLoad initializes org
func (u *OrganizationConfig) AfterLoad(key []byte) error {
	return u.Decrypt(key)
}

// ValidateBeforeSave validates job-resource
func (u *OrganizationConfig) ValidateBeforeSave(key []byte) error {
	if err := u.Validate(); err != nil {
		return err
	}
	return u.Encrypt(key)
}

// NewOrganizationConfig creates new job config
func NewOrganizationConfig(orgID string, name string, value interface{}, secret bool) (*OrganizationConfig, error) {
	nv, err := NewNameTypeValue(name, value, secret)
	if err != nil {
		return nil, err
	}
	return &OrganizationConfig{
		NameTypeValue:  nv,
		OrganizationID: orgID,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}, nil
}
