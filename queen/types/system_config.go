package types

import (
	"errors"
	"fmt"
	"time"
)

// SystemConfig defines internal configuration shared by the formicary platform.
type SystemConfig struct {
	//gorm.Model
	// ID defines UUID for primary key
	ID string `json:"id" gorm:"primary_key"`
	// Scope defines scope such as default or org-unit
	Scope string `json:"scope"`
	// Kind defines kind of config property
	Kind string `json:"kind"`
	// Name defines name of config property
	Name string `json:"name"`
	// Value defines value of config property
	Value string `json:"value"`
	// Secret for encryption
	Secret bool `value:"secret" json:"secret"`
	// CreatedAt job creation time
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt job update time
	UpdatedAt time.Time         `json:"updated_at"`
	Errors    map[string]string `yaml:"-" json:"-" gorm:"-"`
}

// TableName overrides default table name
func (SystemConfig) TableName() string {
	return "formicary_system_config"
}

// NewSystemConfig constructor
func NewSystemConfig(scope string, kind string, name string, value string) *SystemConfig {
	return &SystemConfig{
		Scope: scope,
		Kind:  kind,
		Name:  name,
		Value: value,
	}
}

// ShortID returns short id
func (c *SystemConfig) ShortID() string {
	if len(c.ID) > 8 {
		return "..." + c.ID[len(c.ID)-8:]
	}
	return c.ID
}

func (c *SystemConfig) String() string {
	return fmt.Sprintf("%s=%s", c.Name, c.Value)
}

// Equals compares other config for equality
func (c *SystemConfig) Equals(other *SystemConfig) error {
	if other == nil {
		return fmt.Errorf("found nil other config")
	}
	if err := c.ValidateBeforeSave(); err != nil {
		return err
	}
	if err := other.ValidateBeforeSave(); err != nil {
		return err
	}

	if c.Scope != other.Scope {
		return fmt.Errorf("expected scope %v but was %v", c.Scope, other.Scope)
	}
	if c.Kind != other.Kind {
		return fmt.Errorf("expected kind %v but was %v", c.Kind, other.Kind)
	}
	if c.Name != other.Name {
		return fmt.Errorf("expected name %v but was %v", c.Name, other.Name)
	}
	if c.Value != other.Value {
		return fmt.Errorf("expected value %v but was %v", c.Value, other.Value)
	}
	return nil
}

// Validate validates config
func (c *SystemConfig) Validate() (err error) {
	c.Errors = make(map[string]string)
	if c.Scope == "" {
		err = errors.New("scope is not specified")
		c.Errors["Scope"] = err.Error()
	}
	if c.Kind == "" {
		err = errors.New("kind is not specified")
		c.Errors["Kind"] = err.Error()
	}
	if c.Name == "" {
		err = errors.New("name is not specified in sys-config")
		c.Errors["Name"] = err.Error()
	}
	if c.Value == "" {
		err = errors.New("value is not specified in sys-config")
		c.Errors["Value"] = err.Error()
	}

	return
}

// ValidateBeforeSave validates system config
func (c *SystemConfig) ValidateBeforeSave() error {
	return c.Validate()
}
