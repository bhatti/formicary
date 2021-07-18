package types

import (
	"errors"
	"fmt"
	"plexobject.com/formicary/internal/types"
	"strings"
	"time"

	"plexobject.com/formicary/internal/utils"
)

// BasicResource defines common properties of a resource, which can be used to implement mutex/semaphores for a job.
// These mutex/semaphores can represent external resources that job requires and can be used to determine
// concurrency of jobs. For example, a job may need a license key to connect to a third party service and
// it may only accept upto five connections that can be allocated via resources.
type BasicResource struct {
	// ResourceType defines type of resource such as Device, CPU, Memory
	ResourceType string `yaml:"resource_type" json:"resource_type" gorm:"resource_type"`
	// Description of resource
	Description string `yaml:"description,omitempty" json:"description" gorm:"description"`
	// Platform can be OS platform or target runtime
	Platform string `yaml:"platform,omitempty" json:"platform" gorm:"platform"`
	// Category can be used to represent grouping of resources
	Category string `yaml:"category,omitempty" json:"category" gorm:"category"`
	// Tags can be used as tags for resource matching
	Tags           []string `yaml:"tags,omitempty" json:"tags" gorm:"-"`
	TagsSerialized string   `yaml:"-" json:"-" gorm:"tags_serialized"`
	// Value consumed, e.g. it will be 1 for mutex, semaphore but can be higher number for other quota system
	Value int `yaml:"value" json:"value" gorm:"-"`
	// ExtractConfig -- extracts config from resource and copies to job context
	ExtractConfig ResourceCriteriaConfig `yaml:"extract_config" json:"extract_config" gorm:"-"`
}

func (br *BasicResource) String() string {
	return br.ResourceType
}

// ResourceCriteriaConfig defines properties to extract from a resource into job variables.
// swagger:ignore
type ResourceCriteriaConfig struct {
	Properties    []string `yaml:"properties" json:"properties" gorm:"-"`
	ContextPrefix string   `yaml:"context_prefix" json:"context_prefix" gorm:"-"`
}

// JobResource represents a virtual resource, which can be used to implement mutex/semaphores for a job.
// Job Resources can be used for allocating computing resources such as devices, CPUs, memory, connections, licences, etc.
// You can use them as mutex, semaphores or quota system to determine concurrency of jobs.
// For example, a job may need a license key to connect to a third party service and it may only accept upto
// five connections that can be allocated via resources.
type JobResource struct {
	//gorm.Model
	BasicResource
	// ID defines UUID for primary key
	ID string `yaml:"-" json:"id" gorm:"primary_key"`
	// ExternalID defines external-id of the resource if exists
	ExternalID string `yaml:"external_id" json:"external_id"`
	// ValidStatus - health status of resource
	ValidStatus bool `yaml:"valid_status" json:"valid_status"`
	// Quota can be used to represent mutex (max 1), semaphores (max limit) or other kind of quota.
	// Note: mutex/semaphores can only take one resource by quota may take any value
	Quota int `yaml:"quota" json:"quota"`
	// LeaseTimeout specifies max time to wait for release of resource otherwise it's automatically released.
	LeaseTimeout time.Duration `yaml:"lease_timeout,omitempty" json:"lease_timeout"`
	// Paused is used to stop further processing of job and it can be used during maintenance, upgrade or debugging.
	Paused bool `yaml:"-" json:"paused"`
	// Configs defines config properties of job
	Configs []*JobResourceConfig `yaml:"-" json:"-" gorm:"ForeignKey:JobResourceID" gorm:"auto_preload" gorm:"constraint:OnUpdate:CASCADE"`
	// Uses defines use of resources
	Uses []*JobResourceUse `yaml:"-" json:"-" gorm:"ForeignKey:JobResourceID"`
	// OrganizationID defines org who submitted the job
	OrganizationID string `json:"organization_id"`
	// UserID defines user who submitted the job
	UserID string `json:"user_id"`
	// CreatedAt job creation time
	CreatedAt time.Time `yaml:"-" json:"created_at"`
	// UpdatedAt job update time
	UpdatedAt time.Time `yaml:"-" json:"updated_at"`
	// Active is used to soft delete job resource
	Active bool `yaml:"-" json:"-"`
	// Following are transient properties
	NameValueConfigs map[string]interface{} `yaml:"resource_variables,omitempty" json:"resource_variables" gorm:"-"`
	Errors           map[string]string      `yaml:"-" json:"-" gorm:"-"`
}

// NewJobResource creates new instance of job-resource
func NewJobResource(
	resourceType string,
	quota int) *JobResource {
	return &JobResource{
		BasicResource: BasicResource{
			ResourceType: resourceType,
		},
		ValidStatus:      true,
		Active:           true,
		Quota:            quota,
		LeaseTimeout:     60 * time.Second,
		Configs:          make([]*JobResourceConfig, 0),
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
		NameValueConfigs: make(map[string]interface{}),
	}
}

// TableName overrides default table name
func (JobResource) TableName() string {
	return "formicary_job_resources"
}

// MatchTag matches by tag
func (jr *JobResource) MatchTag(other []string) error {
	return utils.MatchTagsArray(jr.Tags, other)
}

// ShortID short id
func (jr *JobResource) ShortID() string {
	if len(jr.ID) > 8 {
		return "..." + jr.ID[len(jr.ID)-8:]
	}
	return jr.ID
}

// RemainingQuota available
func (jr *JobResource) RemainingQuota() int {
	used := 0
	for _, use := range jr.Uses {
		used += use.Value
	}
	return jr.Quota - used
}

// String provides short summary of job
func (jr *JobResource) String() string {
	return fmt.Sprintf("ResourceType=%s Remaining=%d Tags=%v Platform=%s",
		jr.ResourceType, jr.RemainingQuota(), jr.Tags, jr.Platform)
}

// ConfigString - text view of configs
func (jr *JobResource) ConfigString() string {
	var b strings.Builder
	for _, c := range jr.Configs {
		b.WriteString(c.Name + "=" + c.Value + ",")
	}
	return b.String()
}

// AddConfig adds resource config
func (jr *JobResource) AddConfig(
	name string,
	value interface{}) (*JobResourceConfig, error) {
	config, err := NewJobResourceConfig(name, value, false)
	if err != nil {
		return nil, err
	}
	matched := false
	for _, next := range jr.Configs {
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
		config.JobResourceID = jr.ID
		jr.Configs = append(jr.Configs, config)
	}
	jr.NameValueConfigs[name] = value
	return config, nil
}

// DeleteConfig removes resource config
func (jr *JobResource) DeleteConfig(name string) *JobResourceConfig {
	delete(jr.NameValueConfigs, name)
	for i, next := range jr.Configs {
		if next.Name == name {
			jr.Configs = append(jr.Configs[:i], jr.Configs[i+1:]...)
			return next
		}
	}
	return nil
}

// GetConfig gets job config
func (jr *JobResource) GetConfig(name string) *JobResourceConfig {
	for _, next := range jr.Configs {
		if next.Name == name {
			return next
		}
	}
	return nil
}

// GetConfigByID gets config
func (jr *JobResource) GetConfigByID(configID string) *JobResourceConfig {
	for _, next := range jr.Configs {
		if next.ID == configID {
			return next
		}
	}
	return nil
}

// Equals compares other job-resource for equality
func (jr *JobResource) Equals(other *JobResource) error {
	if other == nil {
		return fmt.Errorf("found nil other job")
	}
	if err := jr.ValidateBeforeSave(); err != nil {
		return err
	}
	if err := other.ValidateBeforeSave(); err != nil {
		return err
	}

	if jr.ResourceType != other.ResourceType {
		return fmt.Errorf("expected jobType %v but was %v", jr.ResourceType, other.ResourceType)
	}
	if len(jr.Configs) != len(other.Configs) {
		return fmt.Errorf("expected number of job configs %v but was %v\nconfigs: %v\ntheirs: %v",
			len(jr.Configs), len(other.Configs), jr.ConfigString(), other.ConfigString())
	}
	return nil
}

// AfterLoad initializes job-resource
func (jr *JobResource) AfterLoad() error {
	jr.NameValueConfigs = make(map[string]interface{})
	jr.Tags = utils.SplitTags(jr.TagsSerialized)
	for _, c := range jr.Configs {
		v, err := c.GetParsedValue()
		if err != nil {
			return err
		}
		jr.NameValueConfigs[c.Name] = v
	}
	return nil
}

// Validate validates job-resource
func (jr *JobResource) Validate() (err error) {
	jr.Errors = make(map[string]string)
	if jr.ResourceType == "" {
		err = errors.New("resource-type is not specified")
		jr.Errors["ResourceType"] = err.Error()
	}
	if jr.Quota <= 0 {
		err = errors.New("resource quota must be defined for max resource usage")
		jr.Errors["Quota"] = err.Error()
	}
	if jr.LeaseTimeout == 0 {
		err = errors.New("lease-timeout is not specified")
		jr.Errors["LeaseTimeout"] = err.Error()
	}

	return
}

// ValidateBeforeSave validates job-resource
func (jr *JobResource) ValidateBeforeSave() error {
	if err := jr.Validate(); err != nil {
		return err
	}

	if jr.Tags != nil {
		jr.TagsSerialized = strings.Join(jr.Tags, ",")
	} else {
		jr.TagsSerialized = ""
	}
	for n, v := range jr.NameValueConfigs {
		if _, err := jr.AddConfig(n, v); err != nil {
			return err
		}
	}
	return nil
}

// JobResourceConfig defines configuration for job resource
type JobResourceConfig struct {
	//gorm.Model
	// Inheriting name, value, type
	types.NameTypeValue
	// ID defines UUID for primary key
	ID string `yaml:"-" json:"id" gorm:"primary_key"`
	// JobResourceID defines foreign key for JobResource
	JobResourceID string `yaml:"-" json:"job_resource_id"`
	// CreatedAt job creation time
	CreatedAt time.Time `yaml:"-" json:"created_at"`
	// UpdatedAt job update time
	UpdatedAt time.Time         `yaml:"-" json:"updated_at"`
	Errors    map[string]string `yaml:"-" json:"-" gorm:"-"`
}

// TableName overrides default table name
func (JobResourceConfig) TableName() string {
	return "formicary_job_resource_config"
}

// NewJobResourceConfig creates new job config
func NewJobResourceConfig(name string, value interface{}, secret bool) (*JobResourceConfig, error) {
	nv, err := types.NewNameTypeValue(name, value, secret)
	if err != nil {
		return nil, err
	}
	return &JobResourceConfig{
		NameTypeValue: nv,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}, nil
}

// Validate validates resource-config
func (u *JobResourceConfig) Validate() (err error) {
	u.Errors = make(map[string]string)
	if u.JobResourceID == "" {
		err = errors.New("job-resource-id is not specified")
		u.Errors["JobResourceID"] = err.Error()
	}
	if u.Name == "" {
		err = errors.New("name is not specified")
		u.Errors["Name"] = err.Error()
	}
	if u.Type == "" {
		err = errors.New("type is not specified")
		u.Errors["Type"] = err.Error()
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
