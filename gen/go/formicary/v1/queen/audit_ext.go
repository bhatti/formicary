// SPDX-License-Identifier: AGPL-3.0-or-later
// Hand-written extension methods for proto-generated AuditRecord.
// This file is NEVER overwritten by buf generate.

package queen

import (
	"fmt"
	"time"
)

// TableName implements the GORM Tabler interface.
func (*AuditRecord) TableName() string { return "formicary_audit_records" }

// Validate checks required fields on the audit record.
func (ar *AuditRecord) Validate() error {
	if ar.Kind == "" {
		return fmt.Errorf("kind is not specified")
	}
	if ar.UserId == "" && ar.OrganizationId == "" {
		return fmt.Errorf("userId or organizationId must be specified")
	}
	return nil
}

// ValidateBeforeSave validates the audit record before persistence.
func (ar *AuditRecord) ValidateBeforeSave() error {
	return ar.Validate()
}

// NewAuditRecord creates a new AuditRecord for the given kind, target, user, org, and remote IP.
func NewAuditRecord(kind string, targetID string, userID string, orgID string, remoteIP string) *AuditRecord {
	return &AuditRecord{
		Id:             ulid(),
		Kind:           kind,
		TargetId:       targetID,
		UserId:         userID,
		OrganizationId: orgID,
		RemoteIp:       remoteIP,
		CreatedAt:      nowTimestamp(),
	}
}

// NewJobAuditRecord creates an audit record for a job event.
func NewJobAuditRecord(kind string, jobType string, requestID string, userID string, orgID string, remoteIP string) *AuditRecord {
	rec := NewAuditRecord(kind, requestID, userID, orgID, remoteIP)
	rec.JobType = jobType
	return rec
}

// Summary returns a short human-readable description of the audit record.
func (ar *AuditRecord) Summary() string {
	t := time.Unix(0, 0)
	if ar.CreatedAt != nil {
		t = ar.CreatedAt.AsTime()
	}
	return fmt.Sprintf("AuditRecord[kind=%s target=%s user=%s org=%s time=%s]",
		ar.Kind, ar.TargetId, ar.UserId, ar.OrganizationId, t.Format(time.RFC3339))
}
