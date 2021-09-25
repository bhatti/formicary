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
	MaxConcurrency int    `yaml:"max_concurrency,omitempty" json:"max_concurrency"`
	// StickyMessage defines an error message that needs user attention
	StickyMessage  string `json:"sticky_message" gorm:"sticky_message"`
	// LicensePolicy defines license policy
	LicensePolicy string `yaml:"external_id" json:"license_policy"`
	// Configs defines config properties of org
	Configs []*OrganizationConfig `yaml:"-" json:"-" gorm:"ForeignKey:OrganizationID" gorm:"auto_preload" gorm:"constraint:OnUpdate:CASCADE"`
	// Subscription defines quota policy and usage period
	Subscription *Subscription `json:"subscription" gorm:"ForeignKey:OrganizationID" gorm:"auto_preload" gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	// Active is used to softly delete org
	Active bool `yaml:"-" json:"-"`
	// CreatedAt created time
	CreatedAt time.Time `yaml:"-" json:"created_at"`
	// UpdatedAt update time
	UpdatedAt time.Time         `yaml:"-" json:"updated_at"`
	Errors    map[string]string `yaml:"-" json:"-" gorm:"-"`
}

// NewOrganization creates new instance of org
func NewOrganization(
	ownerID string,
	orgUnit string,
	bundle string) *Organization {
	return &Organization{
		OwnerUserID:    ownerID,
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
func (o *Organization) String() string {
	return fmt.Sprintf("Org=%s", o.OrgUnit)
}

// AddConfig adds resource config
func (o *Organization) AddConfig(
	name string,
	value interface{},
	secret bool) (*OrganizationConfig, error) {
	config, err := NewOrganizationConfig(o.ID, name, value, secret)
	if err != nil {
		return nil, err
	}
	matched := false
	for _, next := range o.Configs {
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
		config.OrganizationID = o.ID
		o.Configs = append(o.Configs, config)
	}
	return config, nil
}

// DeleteConfig removes resource config
func (o *Organization) DeleteConfig(name string) *OrganizationConfig {
	for i, c := range o.Configs {
		if c.Name == name {
			o.Configs = append(o.Configs[:i], o.Configs[i+1:]...)
			return c
		}
	}
	return nil
}

// ConfigString - text view of configs
func (o *Organization) ConfigString() string {
	var b strings.Builder
	for _, c := range o.Configs {
		b.WriteString(c.Name + "=" + c.Value + ",")
	}
	return b.String()
}

// GetConfig gets config
func (o *Organization) GetConfig(name string) *OrganizationConfig {
	for _, next := range o.Configs {
		if next.Name == name {
			return next
		}
	}
	return nil
}

// GetConfigString gets config value as string
func (o *Organization) GetConfigString(name string) string {
	for _, next := range o.Configs {
		if next.Name == name {
			return next.Value
		}
	}
	return ""
}

// GetConfigByID gets config
func (o *Organization) GetConfigByID(configID string) *OrganizationConfig {
	for _, next := range o.Configs {
		if next.ID == configID {
			return next
		}
	}
	return nil
}

// Equals compares other job-resource for equality
func (o *Organization) Equals(other *Organization) error {
	if other == nil {
		return fmt.Errorf("found nil other job")
	}
	if err := o.Validate(); err != nil {
		return err
	}
	if err := other.Validate(); err != nil {
		return err
	}

	if o.OrgUnit != other.OrgUnit {
		return fmt.Errorf("expected jobType %v but was %v", o.OrgUnit, other.OrgUnit)
	}
	if len(o.Configs) != len(other.Configs) {
		return fmt.Errorf("expected number of org configs %v but was %v\nconfigs: %v\ntheirs: %v",
			len(o.Configs), len(other.Configs), o.ConfigString(), other.ConfigString())
	}
	return nil
}

// AfterLoad initializes org
func (o *Organization) AfterLoad(key []byte) error {
	for _, cfg := range o.Configs {
		if err := cfg.Decrypt(key); err != nil {
			return err
		}
	}
	return nil
}

// Validate validates job-resource
func (o *Organization) Validate() (err error) {
	o.Errors = make(map[string]string)
	if o.OrgUnit == "" {
		err = errors.New("org-unit is not specified")
		o.Errors["OrgUnit"] = err.Error()
	}
	if len(o.OrgUnit) > 100 {
		err = errors.New("org-unit is too long")
		o.Errors["OrgUnit"] = err.Error()
	}
	if o.BundleID == "" {
		err = errors.New("bundle is not specified")
		o.Errors["BundleID"] = err.Error()
	}
	if len(o.BundleID) > 100 {
		err = errors.New("bundle is too long")
		o.Errors["BundleID"] = err.Error()
	}
	if o.MaxConcurrency == 0 {
		o.MaxConcurrency = 3
	}

	return
}

// ValidateBeforeSave validates job-resource
func (o *Organization) ValidateBeforeSave(key []byte) error {
	if err := o.Validate(); err != nil {
		return err
	}
	for _, cfg := range o.Configs {
		if err := cfg.Encrypt(key); err != nil {
			return err
		}
	}
	return nil
}
