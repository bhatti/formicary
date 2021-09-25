package repository

import (
	"fmt"
	"gorm.io/gorm"
	"math/rand"
	"plexobject.com/formicary/internal/events"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/types"
)

var testLocator *Locator

const testDB = "sqlite"
const testDBSource = "/tmp/formicary_test_db.sqlite"

// NewTestLocator uses test database for repositories
func NewTestLocator() (*Locator, error) {
	if testLocator == nil {
		var err error
		serverCfg := config.TestServerConfig()
		serverCfg.DB.DBType = testDB
		serverCfg.DB.DataSource = fmt.Sprintf("%s_%d", testDBSource, rand.Int())
		if err = serverCfg.Validate(); err != nil {
			return nil, err
		}
		testLocator, err = NewLocator(serverCfg)
		//testLocator, err = NewLocator("sqlite", defaultSqliteMemTestDB)
		if err != nil {
			return nil, err
		}
		if testLocator.db == nil {
			return nil, fmt.Errorf("failed to find test database %v", err)
		}
	}
	return testLocator, nil
}

// NewTestLogEventRepository Creating a test repository for log-event
func NewTestLogEventRepository() (*LogEventRepositoryImpl, error) {
	f, err := NewTestLocator()
	if err != nil {
		return nil, err
	}
	return f.LogEventRepository, nil
}

// NewTestAuditRecordRepository Creating a test repository for audit-record
func NewTestAuditRecordRepository() (AuditRecordRepository, error) {
	f, err := NewTestLocator()
	if err != nil {
		return nil, err
	}
	return f.AuditRecordRepository, nil
}

// NewTestErrorCodeRepository Creating a test repository for error-code
func NewTestErrorCodeRepository() (*ErrorCodeRepositoryCached, error) {
	f, err := NewTestLocator()
	if err != nil {
		return nil, err
	}
	return f.ErrorCodeRepository, nil
}

// NewTestJobDefinitionRepository Creating a test repository for job-definition
func NewTestJobDefinitionRepository() (JobDefinitionRepository, error) {
	f, err := NewTestLocator()
	if err != nil {
		return nil, err
	}
	return f.JobDefinitionRepository, nil
}

// NewTestJobRequestRepository Creating a test repository for job-request
func NewTestJobRequestRepository() (*JobRequestRepositoryImpl, error) {
	f, err := NewTestLocator()
	if err != nil {
		return nil, err
	}
	return f.JobRequestRepository, nil
}

// NewTestJobExecutionRepository Creating a test repository for job-execution
func NewTestJobExecutionRepository() (*JobExecutionRepositoryImpl, error) {
	f, err := NewTestLocator()
	if err != nil {
		return nil, err
	}
	return f.JobExecutionRepository, nil
}

// NewTestArtifactRepository Creating a test repository for artifact
func NewTestArtifactRepository() (*ArtifactRepositoryImpl, error) {
	f, err := NewTestLocator()
	if err != nil {
		return nil, err
	}
	return f.ArtifactRepository, nil
}

// NewTestJobResourceRepository Creating a test repository for resources
func NewTestJobResourceRepository() (*JobResourceRepositoryImpl, error) {
	f, err := NewTestLocator()
	if err != nil {
		return nil, err
	}
	return f.JobResourceRepository, nil
}

// NewTestUserRepository a test repository for users
func NewTestUserRepository() (UserRepository, error) {
	f, err := NewTestLocator()
	if err != nil {
		return nil, err
	}
	return f.UserRepository, nil
}

// NewTestOrganizationRepository Creating a test repository for org
func NewTestOrganizationRepository() (OrganizationRepository, error) {
	f, err := NewTestLocator()
	if err != nil {
		return nil, err
	}
	return f.OrgRepository, nil
}

// NewTestInvitationRepository Creating a test repository for user-invitation
func NewTestInvitationRepository() (InvitationRepository, error) {
	f, err := NewTestLocator()
	if err != nil {
		return nil, err
	}
	return f.InvitationRepository, nil
}

// NewTestSubscriptionRepository Creating a test repository for subscription
func NewTestSubscriptionRepository() (SubscriptionRepository, error) {
	f, err := NewTestLocator()
	if err != nil {
		return nil, err
	}
	return f.SubscriptionRepository, nil
}

// NewTestSystemConfigRepository Creating a test repository for system config
func NewTestSystemConfigRepository() (*SystemConfigRepositoryImpl, error) {
	f, err := NewTestLocator()
	if err != nil {
		return nil, err
	}
	return f.SystemConfigRepository, nil
}

// NewTestEmailVerificationRepository Creating a test repository for email verification
func NewTestEmailVerificationRepository() (EmailVerificationRepository, error) {
	f, err := NewTestLocator()
	if err != nil {
		return nil, err
	}
	return f.EmailVerificationRepository, nil
}

// NewTestOrgConfigRepository Creating a test repository for system config
func NewTestOrgConfigRepository() (*OrganizationConfigRepositoryImpl, error) {
	f, err := NewTestLocator()
	if err != nil {
		return nil, err
	}
	return f.OrgConfigRepository, nil
}

/////////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////
// clearDB - for testing purpose clear data before each test
func clearDB(db *gorm.DB) {
	db.Where("id != ''").Delete(common.Artifact{})
	db.Where("id != ''").Delete(types.AuditRecord{})
	db.Where("id != ''").Delete(common.ErrorCode{})
	db.Where("id != ''").Delete(types.TaskExecutionContext{})
	db.Where("id != ''").Delete(types.TaskExecution{})
	db.Where("id != ''").Delete(types.JobExecutionContext{})
	db.Where("id != ''").Delete(types.JobExecution{})
	db.Where("id != ''").Delete(types.JobRequestParam{})
	db.Where("id != ''").Delete(types.JobRequest{})
	db.Where("id != ''").Delete(types.TaskDefinitionVariable{})
	db.Where("id != ''").Delete(types.TaskDefinition{})
	db.Where("id != ''").Delete(types.JobDefinitionVariable{})
	db.Where("id != ''").Delete(types.JobDefinitionConfig{})
	db.Where("id != ''").Delete(types.JobResourceUse{})
	db.Where("id != ''").Delete(types.JobResourceConfig{})
	db.Where("id != ''").Delete(types.JobResource{})
	db.Where("id != ''").Delete(common.OrganizationConfig{})
	db.Where("id != ''").Delete(types.SystemConfig{})
	db.Where("id != ''").Delete(types.JobDefinition{})
	db.Where("id != ''").Delete(common.User{})
	db.Where("id != ''").Delete(common.Organization{})
	db.Where("id != ''").Delete(events.LogEvent{})
	db.Where("id != ''").Delete(common.Subscription{})
	db.Where("id != ''").Delete(common.Payment{})
	db.Where("id != ''").Delete(types.EmailVerification{})
}
