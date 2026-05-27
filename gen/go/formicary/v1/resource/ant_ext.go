// SPDX-License-Identifier: AGPL-3.0-or-later
// Hand-written extension methods for proto-generated AntRegistration, AntAllocation, AntReservation.
// This file is NEVER overwritten by buf generate.

package resource

import (
	"fmt"
	"strings"
	"time"
)

// ──────────────────────────────────────────────────────────────────────────────
// AntRegistration
// ──────────────────────────────────────────────────────────────────────────────

// TableName implements the GORM Tabler interface.
func (*AntRegistration) TableName() string { return "formicary_ant_registrations" }

// IsAlive returns true if the ant registration was created/updated within the TTL window.
func (ar *AntRegistration) IsAlive(ttl time.Duration) bool {
	if ar.AntStartedAt == nil {
		return false
	}
	return time.Since(ar.AntStartedAt.AsTime()) < ttl
}

// HasMethod returns true if this ant supports the given task method.
func (ar *AntRegistration) HasMethod(method string) bool {
	for _, m := range ar.Methods {
		if strings.EqualFold(m, method) {
			return true
		}
	}
	return false
}

// HasTag returns true if this ant has the given tag.
func (ar *AntRegistration) HasTag(tag string) bool {
	for _, t := range ar.Tags {
		if strings.EqualFold(t, tag) {
			return true
		}
	}
	return false
}

// AvailableCapacity returns the number of additional tasks this ant can accept.
func (ar *AntRegistration) AvailableCapacity() int32 {
	return ar.MaxCapacity - ar.CurrentLoad
}

// Summary returns a short human-readable description of the registration.
func (ar *AntRegistration) Summary() string {
	return fmt.Sprintf("AntRegistration[id=%s methods=%v load=%d/%d]",
		ar.AntId, ar.Methods, ar.CurrentLoad, ar.MaxCapacity)
}
