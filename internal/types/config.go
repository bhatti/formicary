// SPDX-License-Identifier: AGPL-3.0-or-later

package types

import (
	"errors"
	"fmt"
	"time"
)

// ConfigurableType discriminates the owner kind in the polymorphic configs table.
type ConfigurableType string

const (
	// ConfigurableTypeOrg owns configs scoped to an organization.
	ConfigurableTypeOrg ConfigurableType = "organizations"
	// ConfigurableTypeUser owns configs scoped to an individual user.
	ConfigurableTypeUser ConfigurableType = "users"
)

// Config is a named configuration property that can belong to either an organization
// or a user, stored in the single polymorphic `formicary_configs` table.
type Config struct {
	NameTypeValue
	// ID primary key
	ID string `yaml:"-" json:"id" gorm:"primary_key"`
	// ConfigurableID is the owner's primary key (org ID or user ID).
	ConfigurableID string `yaml:"-" json:"configurable_id"`
	// ConfigurableType discriminates the owner kind.
	ConfigurableType ConfigurableType `yaml:"-" json:"configurable_type"`
	// CreatedAt creation time
	CreatedAt time.Time `yaml:"-" json:"created_at"`
	// UpdatedAt last update time
	UpdatedAt time.Time         `yaml:"-" json:"updated_at"`
	Errors    map[string]string `yaml:"-" json:"-" gorm:"-"`
}

// TableName overrides the default table name.
func (Config) TableName() string {
	return "formicary_configs"
}

func (c *Config) String() string {
	if c.Secret {
		return fmt.Sprintf("%s=*****", c.Name)
	}
	return fmt.Sprintf("%s=%s", c.Name, c.Value)
}

// Validate validates the config before save.
func (c *Config) Validate() (err error) {
	c.Errors = make(map[string]string)
	if c.ConfigurableID == "" {
		err = errors.New("configurable_id is not specified")
		c.Errors["ConfigurableID"] = err.Error()
	}
	if c.ConfigurableType == "" {
		err = errors.New("configurable_type is not specified")
		c.Errors["ConfigurableType"] = err.Error()
	}
	if c.Name == "" {
		err = errors.New("name is not specified")
		c.Errors["Name"] = err.Error()
	}
	if c.Kind == "" {
		err = errors.New("type is not specified")
		c.Errors["Kind"] = err.Error()
	}
	if c.Value == "" {
		err = errors.New("value is not specified")
		c.Errors["Value"] = err.Error()
	}
	return
}

// AfterLoad decrypts the value after loading from the database.
func (c *Config) AfterLoad(key []byte) error {
	return c.Decrypt(key)
}

// ValidateBeforeSave validates and encrypts before saving to the database.
func (c *Config) ValidateBeforeSave(key []byte) error {
	if err := c.Validate(); err != nil {
		return err
	}
	return c.Encrypt(key)
}

// NewOrgConfig creates a config owned by an organization.
func NewOrgConfig(orgID string, name string, value interface{}, secret bool) (*Config, error) {
	nv, err := NewNameTypeValue(name, value, secret)
	if err != nil {
		return nil, err
	}
	return &Config{
		NameTypeValue:    nv,
		ConfigurableID:   orgID,
		ConfigurableType: ConfigurableTypeOrg,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}, nil
}

// NewUserConfig creates a config owned by a user.
func NewUserConfig(userID string, name string, value interface{}, secret bool) (*Config, error) {
	nv, err := NewNameTypeValue(name, value, secret)
	if err != nil {
		return nil, err
	}
	return &Config{
		NameTypeValue:    nv,
		ConfigurableID:   userID,
		ConfigurableType: ConfigurableTypeUser,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}, nil
}
