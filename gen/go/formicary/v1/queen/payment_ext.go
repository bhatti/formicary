// SPDX-License-Identifier: AGPL-3.0-or-later
// Hand-written extension methods for proto-generated Payment.
// This file is NEVER overwritten by buf generate.

package queen

import "fmt"

// TableName implements the GORM Tabler interface.
func (*Payment) TableName() string { return "formicary_payments" }

// Validate checks required fields on the payment.
func (p *Payment) Validate() error {
	if p.UserId == "" {
		return fmt.Errorf("userId is not specified")
	}
	if p.Amount == 0 {
		return fmt.Errorf("amount is not specified")
	}
	if p.StartedAt == nil {
		return fmt.Errorf("started-at is not specified")
	}
	if p.EndedAt == nil {
		return fmt.Errorf("ended-at is not specified")
	}
	return nil
}

// ValidateBeforeSave validates the payment before persistence.
func (p *Payment) ValidateBeforeSave() error {
	return p.Validate()
}
