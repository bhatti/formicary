package repository

import (
	"fmt"
	"plexobject.com/formicary/internal/acl"
	"reflect"
	"regexp"
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
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"plexobject.com/formicary/queen/config"
)

// Locator provides access to repositories that are used to access database objects
type Locator struct {
	db                          *gorm.DB
	ArtifactRepository          *ArtifactRepositoryImpl
	LogEventRepository          *LogEventRepositoryImpl
	ErrorCodeRepository         *ErrorCodeRepositoryCached
	JobDefinitionRepository     JobDefinitionRepository
	JobRequestRepository        *JobRequestRepositoryImpl
	JobExecutionRepository      *JobExecutionRepositoryImpl
	JobResourceRepository       *JobResourceRepositoryImpl
	SystemConfigRepository      *SystemConfigRepositoryImpl
	ConfigRepository            *ConfigRepositoryImpl
	UserRepository              UserRepository
	OrgRepository               OrganizationRepository
	InvitationRepository        InvitationRepository
	SubscriptionRepository      SubscriptionRepository
	EmailVerificationRepository EmailVerificationRepository
	AuditRecordRepository       AuditRecordRepository
	TriggerStateRepository      TriggerStateRepository
	SlackRegCodeRepository      SlackRegCodeRepository
	BannerRepository            BannerRepository
	DB                          *gorm.DB
}

// sqliteDSN builds a SQLite DSN with WAL mode and performance pragmas appended.
// WAL allows concurrent readers with a single writer, _busy_timeout prevents
// SQLITE_BUSY errors under concurrent load, and NORMAL synchronous mode is safe
// with WAL while being significantly faster than the FULL default.
func sqliteDSN(serverCfg *config.ServerConfig) string {
	if !serverCfg.DB.SQLiteWALMode {
		return serverCfg.DB.DataSource
	}
	ds := serverCfg.DB.DataSource
	sep := "?"
	if strings.Contains(ds, "?") {
		sep = "&"
	}
	pragmas := fmt.Sprintf("%s_journal_mode=WAL&_busy_timeout=%d&_synchronous=%s&_foreign_keys=on",
		sep,
		serverCfg.DB.SQLiteBusyTimeout,
		serverCfg.DB.SQLiteSynchronous,
	)
	if serverCfg.DB.SQLiteCacheSize > 0 {
		pragmas += fmt.Sprintf("&_cache_size=-%d", serverCfg.DB.SQLiteCacheSize)
	}
	return ds + pragmas
}

// NewLocator creates new repository locator
// See https://gorm.io/docs/v2_release_note.html -- go get gorm.io/gorm
func NewLocator(serverCfg *config.ServerConfig) (locator *Locator, err error) {
	maskRegex := regexp.MustCompile(`.*@`)
	log.WithFields(log.Fields{
		"Component":      "RepositoryLocator",
		"Kind":           serverCfg.DB.Type,
		"DataSourceName": maskRegex.ReplaceAllString(serverCfg.DB.DataSource, "*****"),
	}).Infof("Connecting...")
	var db *gorm.DB
	opts := &gorm.Config{
		PrepareStmt: true,
		Logger:      logger.Default.LogMode(logger.Silent),
		//NamingStrategy: schema.NamingStrategy{
		//	TablePrefix: "formicary_",
		//},
	}
	if serverCfg.DB.Type == "mysql" {
		db, err = gorm.Open(mysql.Open(serverCfg.DB.DataSource), opts)
	} else if serverCfg.DB.Type == "postgres" {
		db, err = gorm.Open(postgres.Open(serverCfg.DB.DataSource), opts)
	} else if serverCfg.DB.Type == "sqlite" {
		db, err = gorm.Open(sqlite.Open(sqliteDSN(serverCfg)), opts)
	} else {
		return nil, fmt.Errorf("unsupported database type=%s source=%s", serverCfg.DB.Type, serverCfg.DB.DataSource)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database type=%s source=%s due to %w",
			serverCfg.DB.Type, serverCfg.DB.DataSource, err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(serverCfg.DB.MaxIdleConns)
	sqlDB.SetMaxOpenConns(serverCfg.DB.MaxOpenConns)
	sqlDB.SetConnMaxIdleTime(serverCfg.DB.ConnMaxIdleTime * time.Hour)
	sqlDB.SetConnMaxLifetime(serverCfg.DB.ConnMaxLifeTime * time.Hour)

	// audit records
	artifactRepository, err := NewArtifactRepositoryImpl(db)
	if err != nil {
		return nil, err
	}
	auditRecordRepository, err := NewAuditRecordRepositoryImpl(db)
	if err != nil {
		return nil, err
	}

	// logging events
	logEventRepository, err := NewLogEventRepositoryImpl(db)
	if err != nil {
		return nil, err
	}

	// error codes
	errorCodeRepository, err := NewErrorCodeRepositoryImpl(db)
	if err != nil {
		return nil, err
	}
	errorCodeRepositoryCached, err := NewErrorCodeRepositoryCached(serverCfg, errorCodeRepository)
	if err != nil {
		return nil, err
	}

	// jobs related repositories
	jobDefinitionRepository, err := NewJobDefinitionRepositoryImpl(&serverCfg.DB, db)
	if err != nil {
		return nil, err
	}
	jobDefinitionRepositoryCached, err := NewJobDefinitionRepositoryCached(serverCfg, jobDefinitionRepository)
	if err != nil {
		return nil, err
	}
	jobRequestRepository, err := NewJobRequestRepositoryImpl(db, serverCfg.DB.Type)
	if err != nil {
		return nil, err
	}
	jobExecutionRepository, err := NewJobExecutionRepositoryImpl(db, serverCfg.DB.Type)
	if err != nil {
		return nil, err
	}
	jobResourceRepository, err := NewJobResourceRepositoryImpl(db)
	if err != nil {
		return nil, err
	}

	// config repository
	systemConfigRepository, err := NewSystemConfigRepositoryImpl(db)
	if err != nil {
		return nil, err
	}

	// user repository
	userRepository, err := NewUserRepositoryImpl(db)
	if err != nil {
		return nil, err
	}
	cachedUserRepository, err := NewUserRepositoryCached(serverCfg, userRepository)
	if err != nil {
		return nil, err
	}

	// organization repository
	orgRepository, err := NewOrganizationRepositoryImpl(&serverCfg.DB, db,
		func(
			qc *common.QueryContext,
			id string,
			kind UpdateKind,
			obj interface{}) {
			cachedUserRepository.ClearCacheForOrg(qc.GetOrganizationID())
		})
	if err != nil {
		return nil, err
	}
	cachedOrgRepository, err := NewOrganizationRepositoryCached(serverCfg, orgRepository)
	if err != nil {
		return nil, err
	}
	orgConfigRepository, err := NewConfigRepositoryImpl(&serverCfg.DB, db,
		func(
			qc *common.QueryContext,
			id string,
			kind UpdateKind,
			obj interface{}) {
			cachedOrgRepository.ClearCacheFor(qc.GetOrganizationID(), "")
		})
	if err != nil {
		return nil, err
	}
	invRepository, err := NewInvitationRepositoryImpl(&serverCfg.DB, db)
	if err != nil {
		return nil, err
	}

	// subscription repository
	subscriptionRepository, err := NewSubscriptionRepositoryImpl(
		db, func(
			qc *common.QueryContext,
			id string,
			kind UpdateKind,
			obj interface{}) {
			cachedOrgRepository.ClearCacheFor(qc.GetOrganizationID(), "")
			cachedUserRepository.ClearCacheFor(qc.GetUserID(), "")
		},
	)
	if err != nil {
		return nil, err
	}

	// audit records
	cachedAuditRepository, err := NewAuditRecordRepositoryCached(serverCfg, auditRecordRepository)
	if err != nil {
		return nil, err
	}

	// email verification
	emailVerificationRepository, err := NewEmailVerificationRepositoryImpl(db)
	if err != nil {
		return nil, err
	}
	cachedEmailVerificationRepository, err := NewEmailVerificationRepositoryCached(
		serverCfg,
		emailVerificationRepository)
	if err != nil {
		return nil, err
	}
	triggerStateRepository, err := NewTriggerStateRepositoryImpl(db)
	if err != nil {
		return nil, err
	}

	slackRegCodeRepository, err := NewSlackRegCodeRepositoryImpl(db)
	if err != nil {
		return nil, err
	}

	bannerRepository, err := NewBannerRepositoryImpl(db)
	if err != nil {
		return nil, err
	}

	// Run GORM AutoMigrate when goose has NOT already set up the schema.
	// In Docker/production the entrypoint runs goose first (migrate.sh), so AutoMigrate is skipped.
	// In unit/integration tests (SQLite) and non-goose Postgres environments this creates/updates
	// the schema. AutoMigrate is idempotent and works across all supported DB types.
	if !gooseMigrated(db) {
		if err = migrate(db); err != nil {
			return nil, err
		}
	}

	// Seed fixture data for local dev and tests — SQLite only.
	if serverCfg.DB.Type == "sqlite" {
		qc := common.NewQueryContext(nil, "")
		if org, err := cachedOrgRepository.Create(
			qc,
			common.NewOrganization("", "formicary.org", "org.formicary")); err == nil {
			_, _ = cachedUserRepository.Create(common.NewUser(
				org.ID, "admin", "admin", "support@formicary.io", acl.NewRoles("Admin[]")))
			_, _ = cachedUserRepository.Create(common.NewUser(
				org.ID, "bhatti", "bhatti", "bhatti@formicary.io", acl.NewRoles("Admin[]")))
			_, _ = errorCodeRepository.Save(qc, common.NewErrorCode(
				"*", "job timed out", "", "ERR_JOB_TIMEOUT"))
			_, _ = errorCodeRepository.Save(qc, common.NewErrorCode(
				"*", "task timed out", "", "ERR_TASK_TIMEOUT"))
			_, _ = errorCodeRepository.Save(qc, common.NewErrorCode(
				"*", "failed to schedule job", "", "ERR_JOB_SCHEDULE"))
			_, _ = errorCodeRepository.Save(qc, common.NewErrorCode(
				"*", "failed to launch job", "", "ERR_JOB_LAUNCH"))
			_, _ = errorCodeRepository.Save(qc, common.NewErrorCode(
				"*", "failed to execute job", "", "ERR_JOB_EXECUTE"))
			_, _ = errorCodeRepository.Save(qc, common.NewErrorCode(
				"*", "failed to cancel job", "", "ERR_JOB_CANCELLED"))
			_, _ = errorCodeRepository.Save(qc, common.NewErrorCode(
				"*", "ant workers unavailable", "", "ERR_ANTS_UNAVAILABLE"))
			_, _ = errorCodeRepository.Save(qc, common.NewErrorCode(
				"*", "failed to execute task", "", "ERR_TASK_EXECUTE"))
			_, _ = errorCodeRepository.Save(qc, common.NewErrorCode(
				"*", "failed to find next task", "", "ERR_INVALID_NEXT_TASK"))
			_, _ = errorCodeRepository.Save(qc, common.NewErrorCode(
				"*", "failed to find container", "", "ERR_CONTAINER_NOT_FOUND"))
			_, _ = errorCodeRepository.Save(qc, common.NewErrorCode(
				"*", "failed to stop container", "", "ERR_CONTAINER_STOPPED_FAILED"))
			_, _ = errorCodeRepository.Save(qc, common.NewErrorCode(
				"*", "failed to execute task by ant worker", "", "ERR_ANT_EXECUTION_FAILED"))
			_, _ = errorCodeRepository.Save(qc, common.NewErrorCode(
				"*", "failed to marshal object", "", "ERR_MARSHALING_FAILED"))
			_, _ = errorCodeRepository.Save(qc, common.NewErrorCode(
				"*", "restart job", "", "ERR_RESTART_JOB"))
			_, _ = errorCodeRepository.Save(qc, common.NewErrorCode(
				"*", "pause job", "", "ERR_PAUSE_JOB"))
			_, _ = errorCodeRepository.Save(qc, common.NewErrorCode(
				"*", "restart task", "", "ERR_RESTART_TASK"))
			_, _ = errorCodeRepository.Save(qc, common.NewErrorCode(
				"*", "filtered scheduled job", "", "ERR_FILTERED_JOB"))
			_, _ = errorCodeRepository.Save(qc, common.NewErrorCode(
				"*", "validation error", "", "ERR_VALIDATION"))
			_, _ = errorCodeRepository.Save(qc, common.NewErrorCode(
				"*", "ant resources not available", "", "ERR_ANT_RESOURCES"))
			_, _ = errorCodeRepository.Save(qc, common.NewErrorCode(
				"*", "fatal error", "", "ERR_FATAL"))
			_, _ = errorCodeRepository.Save(qc, common.NewErrorCode(
				"*", "resource quota exceeded", "", "ERR_QUOTA_EXCEEDED"))
		}
	}

	f := &Locator{
		db:                          db,
		DB:                          db,
		ArtifactRepository:          artifactRepository,
		LogEventRepository:          logEventRepository,
		AuditRecordRepository:       cachedAuditRepository,
		ErrorCodeRepository:         errorCodeRepositoryCached,
		JobDefinitionRepository:     jobDefinitionRepositoryCached,
		JobRequestRepository:        jobRequestRepository,
		JobExecutionRepository:      jobExecutionRepository,
		JobResourceRepository:       jobResourceRepository,
		SystemConfigRepository:      systemConfigRepository,
		ConfigRepository:            orgConfigRepository,
		UserRepository:              cachedUserRepository,
		OrgRepository:               cachedOrgRepository,
		InvitationRepository:        invRepository,
		SubscriptionRepository:      subscriptionRepository,
		EmailVerificationRepository: cachedEmailVerificationRepository,
		TriggerStateRepository:      triggerStateRepository,
		SlackRegCodeRepository:      slackRegCodeRepository,
		BannerRepository:            bannerRepository,
	}
	return f, nil
}

// ///////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////

// allowedQueryColumns is the allowlist of snake_case column names that may be
// interpolated into SQL WHERE clauses via addQueryParamsWhere.
// Column names from URL query params are validated against this set before
// being used in SQL to prevent column-name injection.
var allowedQueryColumns = map[string]bool{
	// common
	"id": true, "created_at": true, "updated_at": true,
	// log events
	"level": true, "source": true, "ant_id": true, "job_type": true,
	"job_request_id": true, "job_execution_id": true, "task_execution_id": true,
	"task_type": true, "user_id": true, "tags": true,
	// jobs
	"job_state": true, "org_id": true, "organization_id": true,
	"public_plugin": true, "sem_version": true, "disabled": true, "paused": true,
	"platform": true, "tags_str": true, "cron_triggered": true,
	// job requests
	"scheduled_at": true, "cron_expression": true,
	// artifacts
	"artifact_id": true, "sha256": true, "kind": true, "name": true, "content_type": true,
	// configs
	"scope": true, "value": true,
	// users
	"username": true, "email": true, "verified": true, "active": true, "locked": true,
	// orgs
	"org_unit": true, "bundle_id": true,
	// audit
	"audit_kind": true, "remote_ip": true,
	// resources
	"resource_type": true, "max_quota": true, "quota": true,
	// error codes
	"task_type_error": true, "job_type_error": true, "exit_code": true, "error_code": true,
	// subscriptions
	"subscription_state": true,
	// invitations
	"accepted": true, "invitation_code": true,
	// email verification
	"email_code": true,
	// trigger states
	"trigger_type": true, "external_id": true,
}

// gooseMigrated returns true when the goose version table is present in the DB,
// indicating that the schema was already set up by goose (i.e. running in Docker/production).
// Uses GORM's db-agnostic HasTable so it works on SQLite, Postgres, and MySQL.
func gooseMigrated(db *gorm.DB) bool {
	return db.Migrator().HasTable("goose_db_version")
}

func migrate(db *gorm.DB) error {
	db.DisableForeignKeyConstraintWhenMigrating = true
	// Rename serialized_perms → additive_perms if the old column still exists.
	// Idempotent: HasColumn checks prevent any error on already-migrated databases.
	if db.Migrator().HasColumn(&common.User{}, "serialized_perms") &&
		!db.Migrator().HasColumn(&common.User{}, "additive_perms") {
		if err := db.Migrator().RenameColumn(&common.User{}, "serialized_perms", "additive_perms"); err != nil {
			return fmt.Errorf("failed to rename serialized_perms to additive_perms: %w", err)
		}
	}
	if err := db.AutoMigrate(
		&types.JobDefinition{},
		&types.JobDefinitionVariable{},
		&types.JobDefinitionConfig{},
		&types.TaskDefinition{},
		&types.TaskDefinitionVariable{},
		&types.JobRequest{},
		&types.JobRequestParam{},
		&types.JobExecution{},
		&types.JobExecutionContext{},
		&types.TaskExecution{},
		&types.TaskExecutionContext{},
		&types.JobResource{},
		&types.JobResourceUse{},
		&types.JobResourceConfig{},
		&types.SystemConfig{},
		&common.Artifact{},
		&types.AuditRecord{},
		&common.ErrorCode{},
		&common.User{},
		&types.UserToken{},
		&types.UserSession{},
		&types.UserInvitation{},
		&common.Organization{},
		&common.Config{},
		&events.LogEvent{},
		&common.Subscription{},
		&common.Payment{},
		&types.EmailVerification{},
		&types.TriggerState{},
		&common.SlackRegCode{},
		&common.Banner{},
		&types.ApprovalPolicy{},
		&types.ApprovalVote{},
		&types.ApprovalDeadline{},
	); err != nil {
		return fmt.Errorf("AutoMigrate failed: %w", err)
	}
	log.Infof("AutoMigrate completed")
	return nil
}

// add where clause to query from generic params
func addQueryParamsWhere(params map[string]interface{}, tx *gorm.DB) *gorm.DB {
	for k, v := range params {
		k = strcase.ToSnake(k)
		keyParts := strings.Split(k, ":")
		// Validate the column name against the allowlist to prevent SQL injection.
		if !allowedQueryColumns[keyParts[0]] {
			continue
		}
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
				// check in-clause
				tx = tx.Where(fmt.Sprintf("%v IN ?", keyParts[0]), strings.Split(v.(string), ","))
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
