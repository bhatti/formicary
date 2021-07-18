package types

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Organization represents a user organization that may have one or more users.
// It is used multi-tenancy support in the platform.
type Organization struct {
	//gorm.Model
	// ID defines UUID for primary key
	ID string `yaml:"-" json:"id" gorm:"primary_key"`
	// ParentID defines parent org
	ParentID string `json:"parent_id"`
	// OwnerUserID defines owner user
	OwnerUserID string `json:"owner_user_id"`
	// BundleID defines package or bundle
	BundleID string `json:"bundle_id"`
	// OrgUnit defines org-unit
	OrgUnit string `json:"org_unit"`
	// Salt for password
	Salt string `json:"salt"`
	// MaxConcurrency defines max number of jobs that can be run concurrently by org
	MaxConcurrency int `yaml:"max_concurrency,omitempty" json:"max_concurrency"`
	// LicensePolicy defines license policy
	LicensePolicy string `yaml:"external_id" json:"license_policy"`
	// Configs defines config properties of org
	Configs []*OrganizationConfig `yaml:"-" json:"-" gorm:"ForeignKey:OrganizationID" gorm:"auto_preload" gorm:"constraint:OnUpdate:CASCADE"`
	// Active is used to soft delete org
	Active bool `yaml:"-" json:"-"`
	// CreatedAt created time
	CreatedAt time.Time `yaml:"-" json:"created_at"`
	// UpdatedAt update time
	UpdatedAt time.Time         `yaml:"-" json:"updated_at"`
	Errors    map[string]string `yaml:"-" json:"-" gorm:"-"`
}

// NewOrganization creates new instance of org
func NewOrganization(
	orgUnit string,
	bundle string) *Organization {
	return &Organization{
		BundleID:       bundle,
		OrgUnit:        orgUnit,
		MaxConcurrency: 1,
		Active:         true,
		Configs:        make([]*OrganizationConfig, 0),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
}

// TableName overrides default table name
func (Organization) TableName() string {
	return "formicary_orgs"
}

// String provides short summary of job
func (u *Organization) String() string {
	return fmt.Sprintf("Org=%s", u.OrgUnit)
}

// AddConfig adds resource config
func (u *Organization) AddConfig(
	name string,
	value interface{},
	secret bool) (*OrganizationConfig, error) {
	config, err := NewOrganizationConfig(u.ID, name, value, secret)
	if err != nil {
		return nil, err
	}
	matched := false
	for _, next := range u.Configs {
		if next.Name == name {
			next.Value = config.Value
			next.Type = config.Type
			next.Secret = config.Secret
			config = next
			matched = true
			break
		}
	}
	if !matched {
		config.OrganizationID = u.ID
		u.Configs = append(u.Configs, config)
	}
	return config, nil
}

// DeleteConfig removes resource config
func (u *Organization) DeleteConfig(name string) *OrganizationConfig {
	for i, c := range u.Configs {
		if c.Name == name {
			u.Configs = append(u.Configs[:i], u.Configs[i+1:]...)
			return c
		}
	}
	return nil
}

// ConfigString - text view of configs
func (u *Organization) ConfigString() string {
	var b strings.Builder
	for _, c := range u.Configs {
		b.WriteString(c.Name + "=" + c.Value + ",")
	}
	return b.String()
}

// GetConfig gets config
func (u *Organization) GetConfig(name string) *OrganizationConfig {
	for _, next := range u.Configs {
		if next.Name == name {
			return next
		}
	}
	return nil
}

// GetConfigByID gets config
func (u *Organization) GetConfigByID(configID string) *OrganizationConfig {
	for _, next := range u.Configs {
		if next.ID == configID {
			return next
		}
	}
	return nil
}

// Equals compares other job-resource for equality
func (u *Organization) Equals(other *Organization) error {
	if other == nil {
		return fmt.Errorf("found nil other job")
	}
	if err := u.Validate(); err != nil {
		return err
	}
	if err := other.Validate(); err != nil {
		return err
	}

	if u.OrgUnit != other.OrgUnit {
		return fmt.Errorf("expected jobType %v but was %v", u.OrgUnit, other.OrgUnit)
	}
	if len(u.Configs) != len(other.Configs) {
		return fmt.Errorf("expected number of org configs %v but was %v\nconfigs: %v\ntheirs: %v",
			len(u.Configs), len(other.Configs), u.ConfigString(), other.ConfigString())
	}
	return nil
}

// AfterLoad initializes org
func (u *Organization) AfterLoad(key []byte) error {
	for _, cfg := range u.Configs {
		if err := cfg.Decrypt(key); err != nil {
			return err
		}
	}
	return nil
}

// Validate validates job-resource
func (u *Organization) Validate() (err error) {
	if u.OrgUnit == "" {
		return errors.New("org-unit is not specified")
	}
	if u.BundleID == "" {
		return errors.New("bundle is not specified")
	}

	return
}

// ValidateBeforeSave validates job-resource
func (u *Organization) ValidateBeforeSave(key []byte) error {
	if err := u.Validate(); err != nil {
		return err
	}
	for _, cfg := range u.Configs {
		if err := cfg.Encrypt(key); err != nil {
			return err
		}
	}
	return nil
}
