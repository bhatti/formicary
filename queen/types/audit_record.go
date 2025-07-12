package types

import (
	"fmt"
	common "plexobject.com/formicary/internal/types"
	"time"
)

// AuditKind defines enum for state of request or execution
type AuditKind string

const (
	// JobDefinitionUpdated created
	JobDefinitionUpdated AuditKind = "JOB_DEFINITION_UPDATED"
	// JobResourceUpdated created
	JobResourceUpdated AuditKind = "JOB_RESOURCE_UPDATED"
	// JobRequestCreated created
	JobRequestCreated AuditKind = "JOB_REQUEST_CREATED"
	// JobRequestCompleted completed
	JobRequestCompleted AuditKind = "JOB_REQUEST_COMPLETED"
	// JobRequestPaused completed
	JobRequestPaused AuditKind = "JOB_REQUEST_PAUSED"
	// JobRequestApproved manual
	JobRequestApproved AuditKind = "JOB_REQUEST_APPROVED"
	// JobRequestRejected manual
	JobRequestRejected AuditKind = "JOB_REQUEST_REJECTED"
	// JobRequestFailed failed
	JobRequestFailed AuditKind = "JOB_REQUEST_FAILED"
	// JobRequestCancelled cancelled
	JobRequestCancelled AuditKind = "JOB_REQUEST_CANCELLED"
	// JobRequestRestarted restarted
	JobRequestRestarted AuditKind = "JOB_REQUEST_RESTARTED"
	// JobRequestTriggered restarted
	JobRequestTriggered AuditKind = "JOB_REQUEST_TRIGGERED"

	// UserUpdated updated
	UserUpdated AuditKind = "USER_UPDATED"
	// UserSignup updated
	UserSignup AuditKind = "USER_SIGNUP"
	// UserLogin updated
	UserLogin AuditKind = "USER_LOGIN"
	// UserLogout updated
	UserLogout AuditKind = "USER_LOGOUT"
	// TokenCreated updated
	TokenCreated AuditKind = "TOKEN_CREATED"
	// InvitationCreated updated
	InvitationCreated AuditKind = "INVITATION_CREATED"
	// OrganizationUpdated updated
	OrganizationUpdated AuditKind = "ORGANIZATION_UPDATED"
	// SubscriptionUpdated updated
	SubscriptionUpdated AuditKind = "SUBSCRIPTION_UPDATED"
	// EmailVerificationCreated created
	EmailVerificationCreated AuditKind = "EMAIL_VERIFICATION_CREATED"
	// EmailVerificationVerified verified
	EmailVerificationVerified AuditKind = "EMAIL_VERIFICATION_VERIFIED"
	// OrgConfigUpdated updated
	OrgConfigUpdated AuditKind = "ORG_CONFIG_UPDATED"
)

// AuditRecord defines audit-record
type AuditRecord struct {
	//gorm.Model
	// ID defines UUID for primary key
	ID string `json:"id" gorm:"primary_key"`
	// TargetID defines target id
	TargetID string `json:"target_id"`
	// UserID defines user who submitted the job
	UserID string `json:"user_id"`
	// OrganizationID defines org who submitted the job
	OrganizationID string `json:"organization_id"`
	// Kind defines type of audit record
	Kind AuditKind `json:"kind"`
	// JobType - job-type
	JobType string `json:"job_type"`
	// RemoteIP defines remote ip-address
	RemoteIP string `json:"remote_ip"`
	// URL defines access url
	URL string `json:"url"`
	// Error message
	Error string `json:"error"`
	// Message defines audit message
	Message string `json:"message"`
	// CreatedAt job creation time
	CreatedAt time.Time `json:"created_at"`
}

// TableName overrides default table name
func (AuditRecord) TableName() string {
	return "formicary_audit_records"
}

// NewAuditRecord creates new instance of audit-record
func NewAuditRecord(kind AuditKind, msg string) *AuditRecord {
	return &AuditRecord{
		Kind:      kind,
		Message:   msg,
		CreatedAt: time.Now(),
	}
}

// NewAuditRecordFromJobResource creates new instance of audit-record
func NewAuditRecordFromJobResource(res *JobResource, qc *common.QueryContext) *AuditRecord {
	return &AuditRecord{
		Kind:           JobResourceUpdated,
		Message:        fmt.Sprintf("job-resource updated %s", res),
		UserID:         qc.GetUserID(),
		OrganizationID: qc.GetOrganizationID(),
		TargetID:       res.ID,
		JobType:        res.ResourceType,
		RemoteIP:       qc.IPAddress,
		CreatedAt:      time.Now(),
	}
}

// NewAuditRecordFromJobDefinition creates new instance of audit-record
func NewAuditRecordFromJobDefinition(job *JobDefinition, kind AuditKind, qc *common.QueryContext) *AuditRecord {
	return &AuditRecord{
		Kind:           kind,
		Message:        fmt.Sprintf("job-definition created %s", job),
		UserID:         qc.GetUserID(),
		OrganizationID: qc.GetOrganizationID(),
		TargetID:       job.ID,
		JobType:        job.JobType,
		RemoteIP:       qc.IPAddress,
		CreatedAt:      time.Now(),
	}
}

// NewAuditRecordFromEmailVerification creates new instance of audit-record
func NewAuditRecordFromEmailVerification(ev *EmailVerification, kind AuditKind, qc *common.QueryContext) *AuditRecord {
	return &AuditRecord{
		Kind:           kind,
		Message:        fmt.Sprintf("email verificaton %s, email %s, code %s", kind, ev.Email, ev.EmailCode),
		UserID:         qc.GetUserID(),
		OrganizationID: qc.GetOrganizationID(),
		TargetID:       ev.ID,
		RemoteIP:       qc.IPAddress,
		CreatedAt:      time.Now(),
	}
}

// NewAuditRecordFromJobDefinitionConfig creates new instance of audit-record
func NewAuditRecordFromJobDefinitionConfig(cfg *JobDefinitionConfig, kind AuditKind, qc *common.QueryContext) *AuditRecord {
	return &AuditRecord{
		Kind:           kind,
		Message:        fmt.Sprintf("job-definition config created %s", cfg),
		UserID:         qc.GetUserID(),
		OrganizationID: qc.GetOrganizationID(),
		TargetID:       cfg.ID,
		RemoteIP:       qc.IPAddress,
		CreatedAt:      time.Now(),
	}
}

// NewAuditRecordFromJobRequest creates new instance of audit-record
func NewAuditRecordFromJobRequest(job IJobRequest, kind AuditKind, qc *common.QueryContext) *AuditRecord {
	return &AuditRecord{
		Kind:           kind,
		Message:        fmt.Sprintf("job-request %s", job),
		UserID:         qc.GetUserID(),
		OrganizationID: qc.GetOrganizationID(),
		RemoteIP:       qc.IPAddress,
		JobType:        job.GetJobType(),
		TargetID:       fmt.Sprintf("%s", job.GetID()),
		CreatedAt:      time.Now(),
	}
}

// NewAuditRecordFromInvite creates new instance of audit-record
func NewAuditRecordFromInvite(inv *UserInvitation, qc *common.QueryContext) *AuditRecord {
	return &AuditRecord{
		Kind:           InvitationCreated,
		Message:        fmt.Sprintf("User invitation updated %v", inv),
		UserID:         qc.GetUserID(),
		OrganizationID: qc.GetOrganizationID(),
		TargetID:       inv.ID,
		RemoteIP:       qc.IPAddress,
		CreatedAt:      time.Now(),
	}
}

// NewAuditRecordFromToken creates new instance of audit-record
func NewAuditRecordFromToken(token *UserToken, qc *common.QueryContext) *AuditRecord {
	return &AuditRecord{
		Kind:           TokenCreated,
		Message:        fmt.Sprintf("API token updated %v", token),
		UserID:         qc.GetUserID(),
		OrganizationID: qc.GetOrganizationID(),
		TargetID:       token.ID,
		RemoteIP:       qc.IPAddress,
		CreatedAt:      time.Now(),
	}
}

// NewAuditRecordFromUser creates new instance of audit-record
func NewAuditRecordFromUser(user *common.User, kind AuditKind, qc *common.QueryContext) *AuditRecord {
	return &AuditRecord{
		Kind:           kind,
		Message:        fmt.Sprintf("user updated %s", user),
		UserID:         qc.GetUserID(),
		OrganizationID: qc.GetOrganizationID(),
		TargetID:       user.ID,
		RemoteIP:       qc.IPAddress,
		CreatedAt:      time.Now(),
	}
}

// NewAuditRecordFromSubscription creates new instance of audit-record
func NewAuditRecordFromSubscription(subscription *common.Subscription, qc *common.QueryContext) *AuditRecord {
	return &AuditRecord{
		Kind:           SubscriptionUpdated,
		Message:        fmt.Sprintf("subscription added %s", subscription),
		UserID:         qc.GetUserID(),
		OrganizationID: qc.GetOrganizationID(),
		TargetID:       subscription.UserID,
		RemoteIP:       qc.IPAddress,
		CreatedAt:      time.Now(),
	}
}

// NewAuditRecordFromOrganization creates new instance of audit-record
func NewAuditRecordFromOrganization(org *common.Organization, qc *common.QueryContext) *AuditRecord {
	return &AuditRecord{
		Kind:           OrganizationUpdated,
		Message:        fmt.Sprintf("organization updated %s", org),
		UserID:         qc.GetUserID(),
		OrganizationID: qc.GetOrganizationID(),
		TargetID:       org.ID,
		RemoteIP:       qc.IPAddress,
		CreatedAt:      time.Now(),
	}
}

// NewAuditRecordFromOrganizationConfig creates new instance of audit-record
func NewAuditRecordFromOrganizationConfig(cfg *common.OrganizationConfig, qc *common.QueryContext) *AuditRecord {
	return &AuditRecord{
		Kind:           OrgConfigUpdated,
		Message:        fmt.Sprintf("organization config updated %s", cfg),
		UserID:         qc.GetUserID(),
		OrganizationID: cfg.OrganizationID,
		TargetID:       cfg.ID,
		RemoteIP:       qc.IPAddress,
		CreatedAt:      time.Now(),
	}
}

// Validate validates audit-record
func (ec *AuditRecord) Validate() error {
	if ec.Kind == "" {
		return fmt.Errorf("kind is not specified")
	}
	if ec.Message == "" {
		return fmt.Errorf("message is not specified")
	}
	return nil
}

// ValidateBeforeSave validation before saving
func (ec *AuditRecord) ValidateBeforeSave() error {
	if err := ec.Validate(); err != nil {
		return err
	}
	return nil
}
