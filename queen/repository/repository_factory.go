package repository

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"plexobject.com/formicary/internal/events"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/types"

	"github.com/iancoleman/strcase"
	log "github.com/sirupsen/logrus"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/driver/sqlserver"
	"gorm.io/gorm"
	"plexobject.com/formicary/queen/config"
)

// Factory acts as locator for repositories that are used to access database objects
type Factory struct {
	db                      *gorm.DB
	ArtifactRepository      *ArtifactRepositoryImpl
	AuditRecordRepository   *AuditRecordRepositoryImpl
	LogEventRepository      *LogEventRepositoryImpl
	ErrorCodeRepository     *ErrorCodeRepositoryCached
	JobDefinitionRepository JobDefinitionRepository
	JobRequestRepository    *JobRequestRepositoryImpl
	JobExecutionRepository  *JobExecutionRepositoryImpl
	JobResourceRepository   *JobResourceRepositoryImpl
	SystemConfigRepository  *SystemConfigRepositoryImpl
	OrgConfigRepository     *OrganizationConfigRepositoryImpl
	UserRepository          UserRepository
	OrgRepository           OrganizationRepository
	SubscriptionRepository  SubscriptionRepository
}

// NewFactory creates new repository factory
// See https://gorm.io/docs/v2_release_note.html -- go get gorm.io/gorm
func NewFactory(serverCfg *config.ServerConfig) (factory *Factory, err error) {
	log.WithFields(log.Fields{
		"Component":      "RepositoryFactory",
		"DbType":         serverCfg.DB.DBType,
		"DataSourceName": serverCfg.DB.DataSource,
	}).Infof("Connecting...")
	var db *gorm.DB
	opts := &gorm.Config{
		PrepareStmt: true,
		//NamingStrategy: schema.NamingStrategy{
		//	TablePrefix: "formicary_",
		//},
	}
	if serverCfg.DB.DBType == "mysql" {
		db, err = gorm.Open(mysql.Open(serverCfg.DB.DataSource), opts)
	} else if serverCfg.DB.DBType == "postgres" {
		db, err = gorm.Open(postgres.Open(serverCfg.DB.DataSource), opts)
	} else if serverCfg.DB.DBType == "sqlserver" {
		db, err = gorm.Open(sqlserver.Open(serverCfg.DB.DataSource), opts)
	} else if serverCfg.DB.DBType == "sqlite" {
		db, err = gorm.Open(sqlite.Open(serverCfg.DB.DataSource), opts)
	} else {
		return nil, fmt.Errorf("unsupported database type=%s source=%s", serverCfg.DB.DBType, serverCfg.DB.DataSource)
	}
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(serverCfg.DB.MaxIdleConns)
	sqlDB.SetMaxOpenConns(serverCfg.DB.MaxOpenConns)
	sqlDB.SetConnMaxIdleTime(serverCfg.DB.ConnMaxIdleTime * time.Hour)
	sqlDB.SetConnMaxLifetime(serverCfg.DB.ConnMaxLifeTime * time.Hour)

	artifactRepository, err := NewArtifactRepositoryImpl(db)
	if err != nil {
		return nil, err
	}
	auditRecordRepository, err := NewAuditRecordRepositoryImpl(db)
	if err != nil {
		return nil, err
	}
	logEventRepository, err := NewLogEventRepositoryImpl(db)
	if err != nil {
		return nil, err
	}
	errorCodeRepository, err := NewErrorCodeRepositoryImpl(db)
	if err != nil {
		return nil, err
	}
	errorCodeRepositoryCached, err := NewErrorCodeRepositoryCached(serverCfg, errorCodeRepository)
	if err != nil {
		return nil, err
	}
	jobDefinitionRepository, err := NewJobDefinitionRepositoryImpl(&serverCfg.DB, db)
	if err != nil {
		return nil, err
	}
	jobDefinitionRepositoryCached, err := NewJobDefinitionRepositoryCached(serverCfg, jobDefinitionRepository)
	if err != nil {
		return nil, err
	}

	jobRequestRepository, err := NewJobRequestRepositoryImpl(db, serverCfg.DB.DBType)
	if err != nil {
		return nil, err
	}
	jobExecutionRepository, err := NewJobExecutionRepositoryImpl(db)
	if err != nil {
		return nil, err
	}
	jobResourceRepository, err := NewJobResourceRepositoryImpl(db)
	if err != nil {
		return nil, err
	}
	systemConfigRepository, err := NewSystemConfigRepositoryImpl(db)
	if err != nil {
		return nil, err
	}
	orgConfigRepository, err := NewOrganizationConfigRepositoryImpl(&serverCfg.DB, db)
	if err != nil {
		return nil, err
	}
	userRepository, err := NewUserRepositoryImpl(db)
	if err != nil {
		return nil, err
	}
	cachedUserRepository, err := NewUserRepositoryCached(serverCfg, userRepository)
	if err != nil {
		return nil, err
	}
	orgRepository, err := NewOrganizationRepositoryImpl(&serverCfg.DB, db)
	if err != nil {
		return nil, err
	}

	subscriptionRepository, err := NewSubscriptionRepositoryImpl(db)
	if err != nil {
		return nil, err
	}

	cachedOrgRepository, err := NewOrganizationRepositoryCached(serverCfg, orgRepository)
	if err != nil {
		return nil, err
	}

	if serverCfg.DB.DBType == "sqlite" {
		if err = migrate(db); err != nil {
			return nil, err
		}
		if org, err := cachedOrgRepository.Create(
			common.NewQueryContext("", "", ""),
			common.NewOrganization("", "formicary.org", "org.formicary")); err == nil {
			_, _ = cachedUserRepository.Create(common.NewUser(
				org.ID, "admin", "admin", true))
			_, _ = cachedUserRepository.Create(common.NewUser(
				org.ID, "bhatti", "bhatti", false))
			_, _ = errorCodeRepository.Save(common.NewErrorCode(
				"*", "job timed out", "ERR_JOB_TIMEOUT"))
			_, _ = errorCodeRepository.Save(common.NewErrorCode(
				"*", "task timed out", "ERR_TASK_TIMEOUT"))
			_, _ = errorCodeRepository.Save(common.NewErrorCode(
				"*", "failed to schedule job", "ERR_JOB_SCHEDULE"))
			_, _ = errorCodeRepository.Save(common.NewErrorCode(
				"*", "failed to launch job", "ERR_JOB_LAUNCH"))
			_, _ = errorCodeRepository.Save(common.NewErrorCode(
				"*", "failed to execute job", "ERR_JOB_EXECUTE"))
			_, _ = errorCodeRepository.Save(common.NewErrorCode(
				"*", "failed to cancel job", "ERR_JOB_CANCELLED"))
			_, _ = errorCodeRepository.Save(common.NewErrorCode(
				"*", "ant workers unavailable", "ERR_ANTS_UNAVAILABLE"))
			_, _ = errorCodeRepository.Save(common.NewErrorCode(
				"*", "failed to execute task", "ERR_TASK_EXECUTE"))
			_, _ = errorCodeRepository.Save(common.NewErrorCode(
				"*", "failed to find next task", "ERR_INVALID_NEXT_TASK"))
			_, _ = errorCodeRepository.Save(common.NewErrorCode(
				"*", "failed to find container", "ERR_CONTAINER_NOT_FOUND"))
			_, _ = errorCodeRepository.Save(common.NewErrorCode(
				"*", "failed to stop container", "ERR_CONTAINER_STOPPED_FAILED"))
			_, _ = errorCodeRepository.Save(common.NewErrorCode(
				"*", "failed to execute task by ant worker",
				"ERR_ANT_EXECUTION_FAILED"))
			_, _ = errorCodeRepository.Save(common.NewErrorCode(
				"*", "failed to marshal object", "ERR_MARSHALING_FAILED"))
			_, _ = errorCodeRepository.Save(common.NewErrorCode(
				"*", "restart job", "ERR_RESTART_JOB"))
			_, _ = errorCodeRepository.Save(common.NewErrorCode(
				"*", "restart task", "ERR_RESTART_TASK"))
			_, _ = errorCodeRepository.Save(common.NewErrorCode(
				"*", "filtered scheduled job", "ERR_FILTERED_JOB"))
			_, _ = errorCodeRepository.Save(common.NewErrorCode(
				"*", "validation error", "ERR_VALIDATION"))
			_, _ = errorCodeRepository.Save(common.NewErrorCode(
				"*", "ant resources not available", "ERR_ANT_RESOURCES"))
			_, _ = errorCodeRepository.Save(common.NewErrorCode(
				"*", "fatal error", "ERR_FATAL"))
			_, _ = errorCodeRepository.Save(common.NewErrorCode(
				"*", "resource quota exceeded", "ERR_QUOTA_EXCEEDED"))
		}
	}

	f := &Factory{
		db:                      db,
		ArtifactRepository:      artifactRepository,
		LogEventRepository:      logEventRepository,
		AuditRecordRepository:   auditRecordRepository,
		ErrorCodeRepository:     errorCodeRepositoryCached,
		JobDefinitionRepository: jobDefinitionRepositoryCached,
		JobRequestRepository:    jobRequestRepository,
		JobExecutionRepository:  jobExecutionRepository,
		JobResourceRepository:   jobResourceRepository,
		SystemConfigRepository:  systemConfigRepository,
		OrgConfigRepository:     orgConfigRepository,
		UserRepository:          cachedUserRepository,
		OrgRepository:           cachedOrgRepository,
		SubscriptionRepository:  subscriptionRepository,
	}
	return f, nil
}

/////////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////
func migrate(db *gorm.DB) error {
	db.DisableForeignKeyConstraintWhenMigrating = true
	if err := db.AutoMigrate(&types.JobDefinition{}); err != nil {
		return err
	}
	if err := db.AutoMigrate(&types.JobDefinitionVariable{}); err != nil {
		return err
	}
	if err := db.AutoMigrate(&types.JobDefinitionConfig{}); err != nil {
		return err
	}
	if err := db.AutoMigrate(types.TaskDefinition{}); err != nil {
		return err
	}
	if err := db.AutoMigrate(&types.TaskDefinitionVariable{}); err != nil {
		return err
	}
	if err := db.AutoMigrate(&types.JobRequest{}); err != nil {
		return err
	}
	if err := db.AutoMigrate(&types.JobRequestParam{}); err != nil {
		return err
	}
	if err := db.AutoMigrate(&types.JobExecution{}); err != nil {
		return err
	}
	if err := db.AutoMigrate(&types.JobExecutionContext{}); err != nil {
		return err
	}
	if err := db.AutoMigrate(&types.TaskExecution{}); err != nil {
		return err
	}
	if err := db.AutoMigrate(&types.TaskExecutionContext{}); err != nil {
		return err
	}
	if err := db.AutoMigrate(&types.JobResource{}); err != nil {
		return err
	}
	if err := db.AutoMigrate(&types.JobResourceUse{}); err != nil {
		return err
	}
	if err := db.AutoMigrate(&types.JobResourceConfig{}); err != nil {
		return err
	}
	if err := db.AutoMigrate(&types.SystemConfig{}); err != nil {
		return err
	}
	if err := db.AutoMigrate(&common.Artifact{}); err != nil {
		return err
	}
	if err := db.AutoMigrate(&types.AuditRecord{}); err != nil {
		return err
	}
	if err := db.AutoMigrate(&common.ErrorCode{}); err != nil {
		return err
	}
	if err := db.AutoMigrate(&common.User{}); err != nil {
		return err
	}
	if err := db.AutoMigrate(&types.UserToken{}); err != nil {
		return err
	}
	if err := db.AutoMigrate(&types.UserSession{}); err != nil {
		return err
	}
	if err := db.AutoMigrate(&types.UserInvitation{}); err != nil {
		return err
	}
	if err := db.AutoMigrate(&common.Organization{}); err != nil {
		return err
	}
	if err := db.AutoMigrate(&common.OrganizationConfig{}); err != nil {
		return err
	}
	if err := db.AutoMigrate(&events.LogEvent{}); err != nil {
		return err
	}
	if err := db.AutoMigrate(&common.Subscription{}); err != nil {
		return err
	}
	if err := db.AutoMigrate(&common.Payment{}); err != nil {
		return err
	}

	log.Infof("Migrated test database...")
	return nil
}

// add where clause to query from generic params
func addQueryParamsWhere(params map[string]interface{}, tx *gorm.DB) *gorm.DB {
	for k, v := range params {
		k = strcase.ToSnake(k)
		keyParts := strings.Split(k, ":")
		if reflect.TypeOf(v).String() == "string" &&
			(strings.HasSuffix(keyParts[0], "_date") || strings.HasSuffix(keyParts[0], "_at")) {
			if date, err := time.Parse(time.RFC3339, v.(string)); err == nil {
				v = date
			}
		}
		if len(keyParts) > 1 {
			if strings.HasPrefix(keyParts[1], "like") || strings.HasPrefix(keyParts[1], "contain") {
				tx = tx.Where(fmt.Sprintf("%v LIKE ?", keyParts[0]), fmt.Sprintf("%%%v%%", v))
			} else if strings.HasPrefix(keyParts[1], "in") {
				tx = tx.Where(fmt.Sprintf("%v IN (?)", keyParts[0]), strings.Split(v.(string), ","))
			} else if strings.HasPrefix(keyParts[1], "!") || strings.HasPrefix(keyParts[1], "<>") {
				tx = tx.Where(fmt.Sprintf("%v <> ?", keyParts[0]), strings.Split(v.(string), ","))
			} else if strings.HasPrefix(keyParts[1], "<") {
				tx = tx.Where(fmt.Sprintf("%v < ?", keyParts[0]), v)
			} else if strings.HasPrefix(keyParts[1], "<=") {
				tx = tx.Where(fmt.Sprintf("%v <= ?", keyParts[0]), v)
			} else if strings.HasPrefix(keyParts[1], ">") {
				tx = tx.Where(fmt.Sprintf("%v > ?", keyParts[0]), v)
			} else if strings.HasPrefix(keyParts[1], ">=") {
				tx = tx.Where(fmt.Sprintf("%v >= ?", keyParts[0]), v)
			} else {
				tx = tx.Where(fmt.Sprintf("%v = ?", keyParts[0]), v)
			}
		} else {
			tx = tx.Where(fmt.Sprintf("%v = ?", k), v)
		}
	}
	return tx
}
