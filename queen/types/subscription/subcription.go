package subscription

import (
	"fmt"
	"time"
)

// Kind defines enum for kind of subscription
type Kind string

const (
	// FreemiumSubscription subscription
	FreemiumSubscription Kind = "FREEMIUM"
	// IndividualSubscription subscription
	IndividualSubscription Kind = "INDIVIDUAL"
	// OrganizationSubscription subscription
	OrganizationSubscription Kind = "ORGANIZATION"
)

// Period defines enum for period of subscription
type Period string

const (
	// Monthly period
	Monthly Period = "MONTHLY"
	// SemiAnnual period
	SemiAnnual Period = "SEMI_ANNUAL"
	// Annual period
	Annual Period = "ANNUAL"
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
	Kind Kind `json:"kind"`
	// Period of subscription
	Period Period `json:"period"`
	// Price of subscription in cents
	Price uint64 `json:"price"`
	// CPUQuota  allowed cpu seconds
	CPUQuota uint64 `json:"cpu_quota"`
	// DiskQuota allowed disk bytes
	DiskQuota uint64 `json:"disk_quota"`
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

// Validate validates audit-record
func (ec *Subscription) Validate() error {
	if ec.Kind == "" {
		return fmt.Errorf("kind is not specified")
	}
	if ec.Period == "" {
		return fmt.Errorf("period is not specified")
	}
	if ec.UserID == "" {
		return fmt.Errorf("userID is not specified")
	}
	if ec.Price == 0 {
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
