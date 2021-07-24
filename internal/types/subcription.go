package types

import (
	"fmt"
	"time"
)

// Kind defines enum for kind of subscription
type Kind string

const (
	// IndividualSubscription subscription
	IndividualSubscription Kind = "INDIVIDUAL"
	// OrganizationSubscription subscription
	OrganizationSubscription Kind = "ORGANIZATION"
)

// Period defines enum for period of subscription
type Period string

const (
	// Weekly period
	Weekly Period = "WEEKLY"
	// Monthly period
	Monthly Period = "MONTHLY"
	// SemiAnnual period
	SemiAnnual Period = "SEMI_ANNUAL"
	// Annual period
	Annual Period = "ANNUAL"
)

// Policy defines enum for policy of subscription
type Policy string

const (
	// Freemium policy
	Freemium Policy = "FREEMIUM"
	// Basic policy
	Basic Policy = "BASIC"
	// Enterprise policy
	Enterprise Policy = "ENTERPRISE"
	// Unlimited policy
	Unlimited Policy = "UNLIMITED"
)

// Subscription defines subscription
type Subscription struct {
	//gorm.Model
	// ID defines UUID for primary key
	ID string `json:"id" gorm:"primary_key"`
	// UserID defines user
	UserID string `json:"user_id"`
	// OrganizationID defines org
	OrganizationID string `json:"organization_id"`
	// Kind defines type of subscription
	Policy Policy `json:"policy"`
	// Kind defines type of subscription
	Kind Kind `json:"kind"`
	// Period of subscription
	Period Period `json:"period"`
	// Price of subscription in cents
	Price int64 `json:"price"`
	// CPUQuota  allowed cpu seconds
	CPUQuota int64 `json:"cpu_quota"`
	// DiskQuota allowed disk Mbytes
	DiskQuota int64 `json:"disk_quota"`
	// StartedAt started-at
	StartedAt time.Time `json:"started_at"`
	// EndedAt ended-at
	EndedAt time.Time `json:"ended_at"`
	// Active flag
	Active bool `json:"active"`
	// CreatedAt creation time
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt update time
	UpdatedAt time.Time `json:"updated_at"`
	// RemainingCPUQuota  cpu seconds
	RemainingCPUQuota int64 `json:"remaining_cpu_quota" gorm:"-"`
	// RemainingDiskQuota disk Mbytes
	RemainingDiskQuota int64 `json:"remaining_disk_quota" gorm:"-"`
	// LoadedAt
	LoadedAt time.Time `json:"-" gorm:"-"`
}

// TableName overrides default table name
func (Subscription) TableName() string {
	return "formicary_subscriptions"
}

// NewSubscription creates new instance
func NewSubscription(kind Kind, period Period) *Subscription {
	return &Subscription{
		Kind:      kind,
		Period:    period,
		Active:    true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// NewFreemiumSubscription creates new instance
func NewFreemiumSubscription(userID string, orgID string) *Subscription {
	kind := IndividualSubscription
	if orgID != "" {
		kind = OrganizationSubscription
	}
	return &Subscription{
		UserID:         userID,
		OrganizationID: orgID,
		Kind:           kind,
		Period:         Monthly,
		Policy:         Freemium,
		CPUQuota:       1000,
		DiskQuota:      1000,
		Active:         true,
		StartedAt:      time.Now(),
		EndedAt:        time.Now().Add(time.Hour * 24 * 30),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
}

func (ec *Subscription) String() string {
	return fmt.Sprintf("Subscription policy=%s period=%s user=%s org=%s cpu=%d disk=%d start=%s end=%s",
		ec.Policy, ec.Period, ec.UserID, ec.OrganizationID, ec.CPUQuota, ec.DiskQuota, ec.StartedString(), ec.EndedString())
}

// Validate validates audit-record
func (ec *Subscription) Validate() error {
	if ec.Kind == "" {
		return fmt.Errorf("kind is not specified")
	}
	if ec.Policy == "" {
		return fmt.Errorf("policy is not specified")
	}
	if ec.Period == "" {
		return fmt.Errorf("period is not specified")
	}
	if ec.UserID == "" && ec.OrganizationID == "" {
		return fmt.Errorf("userID and organizationID is not specified")
	}
	if ec.Price == 0 && ec.Policy != Freemium {
		return fmt.Errorf("price is not specified")
	}
	if ec.CPUQuota == 0 {
		return fmt.Errorf("cpu-quota is not specified")
	}
	if ec.DiskQuota == 0 {
		return fmt.Errorf("disk-quota is not specified")
	}
	if ec.StartedAt.IsZero() {
		return fmt.Errorf("started-at is not specified")
	}
	if ec.EndedAt.IsZero() {
		return fmt.Errorf("ended-at is not specified")
	}
	return nil
}

// StartedString formatted date
func (ec *Subscription) StartedString() string {
	return ec.StartedAt.Format("Jan _2")
}

// EndedString formatted date
func (ec *Subscription) EndedString() string {
	return ec.EndedAt.Format("Jan _2")
}

// Expired checks expiration
func (ec *Subscription) Expired() bool {
	return !ec.Active || ec.EndedAt.Unix() < time.Now().Unix()
}
