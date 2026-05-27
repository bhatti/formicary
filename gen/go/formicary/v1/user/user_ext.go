// SPDX-License-Identifier: AGPL-3.0-or-later
// Hand-written extension methods for proto-generated User, Organization,
// Subscription, and OrganizationConfig types.
// This file is NEVER overwritten by buf generate.

package user

import (
	"fmt"
	"math/rand"
	"time"

	ulidpkg "github.com/oklog/ulid/v2"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func ulid() string {
	entropy := ulidpkg.Monotonic(rand.New(rand.NewSource(time.Now().UnixNano())), 0) //nolint:gosec
	return ulidpkg.MustNew(ulidpkg.Timestamp(time.Now()), entropy).String()
}

func nowTimestamp() *timestamppb.Timestamp {
	return timestamppb.New(time.Now())
}

// ──────────────────────────────────────────────────────────────────────────────
// User
// ──────────────────────────────────────────────────────────────────────────────

// TableName implements the GORM Tabler interface.
func (*User) TableName() string { return "formicary_users" }

// HasOrganization returns true if the user belongs to an organization.
func (u *User) HasOrganization() bool {
	return u.OrganizationId != ""
}

// Validate checks required fields on the user.
func (u *User) Validate() error {
	u.Errors = make(map[string]string)
	var err error
	if u.Username == "" {
		err = fmt.Errorf("username is not specified")
		u.Errors["Username"] = err.Error()
	}
	if u.Email == "" {
		err = fmt.Errorf("email is not specified")
		u.Errors["Email"] = err.Error()
	}
	return err
}

// ValidateBeforeSave validates the user before persistence.
func (u *User) ValidateBeforeSave() error {
	return u.Validate()
}

// ──────────────────────────────────────────────────────────────────────────────
// Organization
// ──────────────────────────────────────────────────────────────────────────────

// TableName implements the GORM Tabler interface.
func (*Organization) TableName() string { return "formicary_orgs" }

// AddConfig adds or updates a named configuration property on the organization.
func (o *Organization) AddConfig(name string, value string, kind string, secret bool) *OrganizationConfig {
	for _, c := range o.Configs {
		if c.Name == name {
			c.Value = value
			c.Kind = kind
			c.Secret = secret
			c.UpdatedAt = nowTimestamp()
			return c
		}
	}
	cfg := &OrganizationConfig{
		Id:             ulid(),
		OrganizationId: o.Id,
		Name:           name,
		Value:          value,
		Kind:           kind,
		Secret:         secret,
		CreatedAt:      nowTimestamp(),
		UpdatedAt:      nowTimestamp(),
	}
	o.Configs = append(o.Configs, cfg)
	return cfg
}

// GetConfig returns a named configuration property, or nil if not found.
func (o *Organization) GetConfig(name string) *OrganizationConfig {
	for _, c := range o.Configs {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// DeleteConfig removes a named configuration property from the organization.
func (o *Organization) DeleteConfig(name string) bool {
	for i, c := range o.Configs {
		if c.Name == name {
			o.Configs = append(o.Configs[:i], o.Configs[i+1:]...)
			return true
		}
	}
	return false
}

// Validate checks required fields on the organization.
func (o *Organization) Validate() error {
	o.Errors = make(map[string]string)
	var err error
	if o.OrgUnit == "" {
		err = fmt.Errorf("orgUnit is not specified")
		o.Errors["OrgUnit"] = err.Error()
	}
	if o.BundleId == "" {
		err = fmt.Errorf("bundleId is not specified")
		o.Errors["BundleId"] = err.Error()
	}
	return err
}

// ValidateBeforeSave validates the organization before persistence.
func (o *Organization) ValidateBeforeSave() error {
	return o.Validate()
}

// ──────────────────────────────────────────────────────────────────────────────
// OrganizationConfig
// ──────────────────────────────────────────────────────────────────────────────

// TableName implements the GORM Tabler interface.
func (*OrganizationConfig) TableName() string { return "formicary_org_configs" }

// Validate checks required fields on the org config.
func (oc *OrganizationConfig) Validate() error {
	oc.Errors = make(map[string]string)
	var err error
	if oc.Name == "" {
		err = fmt.Errorf("name is not specified")
		oc.Errors["Name"] = err.Error()
	}
	if oc.Value == "" {
		err = fmt.Errorf("value is not specified")
		oc.Errors["Value"] = err.Error()
	}
	return err
}

// ──────────────────────────────────────────────────────────────────────────────
// Subscription
// ──────────────────────────────────────────────────────────────────────────────

// TableName implements the GORM Tabler interface.
func (*Subscription) TableName() string { return "formicary_subscriptions" }

// Validate checks required fields on the subscription.
func (s *Subscription) Validate() error {
	if s.Kind == "" {
		return fmt.Errorf("kind is not specified")
	}
	if s.Policy == "" {
		return fmt.Errorf("policy is not specified")
	}
	if s.Period == "" {
		return fmt.Errorf("period is not specified")
	}
	if s.UserId == "" && s.OrganizationId == "" {
		return fmt.Errorf("userId and organizationId are not specified")
	}
	if s.Price == 0 && s.Policy != "FREEMIUM" {
		return fmt.Errorf("price is not specified")
	}
	if s.CpuQuota == 0 {
		return fmt.Errorf("cpu-quota is not specified")
	}
	if s.DiskQuota == 0 {
		return fmt.Errorf("disk-quota is not specified")
	}
	if s.StartedAt == nil {
		return fmt.Errorf("started-at is not specified")
	}
	if s.EndedAt == nil {
		return fmt.Errorf("ended-at is not specified")
	}
	return nil
}

// ValidateBeforeSave validates the subscription before persistence.
func (s *Subscription) ValidateBeforeSave() error {
	return s.Validate()
}

// Expired returns true if the subscription is no longer active.
func (s *Subscription) Expired() bool {
	if !s.Active || s.EndedAt == nil {
		return true
	}
	return s.EndedAt.AsTime().Before(time.Now())
}

// StartedString returns a formatted started-at date.
func (s *Subscription) StartedString() string {
	if s.StartedAt == nil {
		return ""
	}
	return s.StartedAt.AsTime().Format("Jan _2")
}

// EndedString returns a formatted ended-at date.
func (s *Subscription) EndedString() string {
	if s.EndedAt == nil {
		return ""
	}
	return s.EndedAt.AsTime().Format("Jan _2")
}

// Summary returns a short human-readable description of the subscription.
func (s *Subscription) Summary() string {
	return fmt.Sprintf("Subscription policy=%s period=%s user=%s org=%s cpu=%d disk=%d start=%s end=%s",
		s.Policy, s.Period, s.UserId, s.OrganizationId, s.CpuQuota, s.DiskQuota,
		s.StartedString(), s.EndedString())
}
