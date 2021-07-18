package repository

import (
	"fmt"
	"math/rand"
	"plexobject.com/formicary/internal/crypto"
	"plexobject.com/formicary/internal/events"
	"plexobject.com/formicary/queen/types/subscription"
	"time"

	"gorm.io/gorm"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/types"
)

var testFactory *Factory

//const testDB = "mysql"
//const testDBSource = "formicary_user_dev:formicary_pass@tcp(localhost:3306)/formicary_dev?charset=utf8mb4&parseTime=true&loc=Local"

const testDB = "sqlite"
const testDBSource = "/tmp/formicary_sqlite.db"
//const testDBSource = ":memory:"

// NewTestFactory Creating a test database connection
func NewTestFactory() (*Factory, error) {
	if testFactory == nil {
		var err error
		serverCfg := &config.ServerConfig{}
		serverCfg.S3.AccessKeyID = "admin"
		serverCfg.S3.SecretAccessKey = "password"
		serverCfg.S3.Bucket = "test-bucket"
		serverCfg.Pulsar.URL = "test"
		serverCfg.Redis.Host = "test"
		serverCfg.DB.DBType = testDB
		serverCfg.DB.DataSource = fmt.Sprintf("%s_%d", testDBSource, rand.Int())
		serverCfg.DB.MaxIdleConns = 10
		serverCfg.DB.MaxOpenConns = 20
		serverCfg.DB.MaxOpenConns = 20
		serverCfg.DB.EncryptionKey = string(crypto.SHA256Key("test-key"))
		serverCfg.DB.ConnMaxIdleTime = 1 * time.Hour
		serverCfg.DB.ConnMaxLifeTime = 4 * time.Hour
		if err = serverCfg.Validate(); err != nil {
			return nil, err
		}
		testFactory, err = NewFactory(serverCfg)
		//testFactory, err = NewFactory("sqlite", defaultSqliteMemTestDB)
		if err != nil {
			return nil, err
		}
		if testFactory.db == nil {
			return nil, fmt.Errorf("failed to find test database %v", err)
		}
	}
	return testFactory, nil
}

// NewTestLogEventRepository Creating a test repository for log-event
func NewTestLogEventRepository() (*LogEventRepositoryImpl, error) {
	f, err := NewTestFactory()
	if err != nil {
		return nil, err
	}
	return f.LogEventRepository, nil
}

// NewTestAuditRecordRepository Creating a test repository for audit-record
func NewTestAuditRecordRepository() (*AuditRecordRepositoryImpl, error) {
	f, err := NewTestFactory()
	if err != nil {
		return nil, err
	}
	return f.AuditRecordRepository, nil
}

// NewTestErrorCodeRepository Creating a test repository for error-code
func NewTestErrorCodeRepository() (*ErrorCodeRepositoryCached, error) {
	f, err := NewTestFactory()
	if err != nil {
		return nil, err
	}
	return f.ErrorCodeRepository, nil
}

// NewTestJobDefinitionRepository Creating a test repository for job-definition
func NewTestJobDefinitionRepository() (JobDefinitionRepository, error) {
	f, err := NewTestFactory()
	if err != nil {
		return nil, err
	}
	return f.JobDefinitionRepository, nil
}

// NewTestJobRequestRepository Creating a test repository for job-request
func NewTestJobRequestRepository() (*JobRequestRepositoryImpl, error) {
	f, err := NewTestFactory()
	if err != nil {
		return nil, err
	}
	return f.JobRequestRepository, nil
}

// NewTestJobExecutionRepository Creating a test repository for job-execution
func NewTestJobExecutionRepository() (*JobExecutionRepositoryImpl, error) {
	f, err := NewTestFactory()
	if err != nil {
		return nil, err
	}
	return f.JobExecutionRepository, nil
}

// NewTestArtifactRepository Creating a test repository for artifact
func NewTestArtifactRepository() (*ArtifactRepositoryImpl, error) {
	f, err := NewTestFactory()
	if err != nil {
		return nil, err
	}
	return f.ArtifactRepository, nil
}

// NewTestJobResourceRepository Creating a test repository for resources
func NewTestJobResourceRepository() (*JobResourceRepositoryImpl, error) {
	f, err := NewTestFactory()
	if err != nil {
		return nil, err
	}
	return f.JobResourceRepository, nil
}

// NewTestUserRepository a test repository for users
func NewTestUserRepository() (UserRepository, error) {
	f, err := NewTestFactory()
	if err != nil {
		return nil, err
	}
	return f.UserRepository, nil
}

// NewTestOrganizationRepository Creating a test repository for resources
func NewTestOrganizationRepository() (OrganizationRepository, error) {
	f, err := NewTestFactory()
	if err != nil {
		return nil, err
	}
	return f.OrgRepository, nil
}

// NewTestSystemConfigRepository Creating a test repository for system config
func NewTestSystemConfigRepository() (*SystemConfigRepositoryImpl, error) {
	f, err := NewTestFactory()
	if err != nil {
		return nil, err
	}
	return f.SystemConfigRepository, nil
}

// NewTestOrgConfigRepository Creating a test repository for system config
func NewTestOrgConfigRepository() (*OrganizationConfigRepositoryImpl, error) {
	f, err := NewTestFactory()
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
	db.Where("id != ''").Delete(subscription.Subscription{})
	db.Where("id != ''").Delete(subscription.Payment{})
}
