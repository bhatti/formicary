package subscription

import (
	"fmt"
	"plexobject.com/formicary/internal/types"
	"time"
)

// Payment for subscription
type Payment struct {
	//gorm.Model
	// ID defines UUID for primary key
	ID string `json:"id" gorm:"primary_key"`
	// SubscriptionID foreign-key
	SubscriptionID string `json:"subscription_id"`
	// ExternalID external id
	ExternalID string `json:"external_id"`
	// UserID defines user who submitted the job
	UserID string `json:"user_id"`
	// OrganizationID defines org
	OrganizationID string `json:"organization_id"`
	// State
	State types.RequestState `json:"state"`
	// PaymentMethod method
	PaymentMethod types.RequestState `json:"payment_method"`
	// Notes
	Notes string `json:"notes"`
	// Amount of subscription in cents
	Amount uint64 `json:"amount"`
	// RefundedAt refunded-at
	RefundedAt *time.Time `json:"refunded_at"`
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
func (Payment) TableName() string {
	return "formicary_payments"
}

// NewPayment creates new instance
func NewPayment(s Subscription) *Payment {
	return &Payment{
		UserID:         s.UserID,
		OrganizationID: s.OrganizationID,
		State:          types.PENDING,
		Amount:         s.Price,
		Active:         true,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
}

// Validate validates audit-record
func (ec *Payment) Validate() error {
	if ec.UserID == "" {
		return fmt.Errorf("userID is not specified")
	}
	if ec.Amount == 0 {
		return fmt.Errorf("amount is not specified")
	}
	if ec.StartedAt.IsZero() {
		return fmt.Errorf("started-at is not specified")
	}
	if ec.EndedAt.IsZero() {
		return fmt.Errorf("ended-at is not specified")
	}
	return nil
}
