package manager

import (
	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/internal/artifacts"
	"plexobject.com/formicary/internal/metrics"
	"plexobject.com/formicary/internal/queue"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/notify"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/resource"
	"plexobject.com/formicary/queen/stats"
	"testing"
)

// AssertTestUserManager for testing
func AssertTestUserManager(serverCfg *config.ServerConfig, t *testing.T) *UserManager {
	mgr, err := TestUserManager(serverCfg)
	require.NoError(t, err)
	return mgr
}

// TestUserManager for testing
func TestUserManager(serverCfg *config.ServerConfig) (userManager *UserManager, err error) {
	if serverCfg == nil {
		serverCfg = config.TestServerConfig()
	}
	err = serverCfg.Validate()
	if err != nil {
		return nil, err
	}
	userRepository, err := repository.NewTestUserRepository()
	if err != nil {
		return nil, err
	}
	orgRepository, err := repository.NewTestOrganizationRepository()
	if err != nil {
		return nil, err
	}
	orgConfigRepository, err := repository.NewTestOrgConfigRepository()
	if err != nil {
		return nil, err
	}

	auditRecordRepository, err := repository.NewTestAuditRecordRepository()
	if err != nil {
		return nil, err
	}

	emailVerificationRepository, err := repository.NewTestEmailVerificationRepository()
	if err != nil {
		return nil, err
	}
	subscriptionRepository, err := repository.NewTestSubscriptionRepository()
	if err != nil {
		return nil, err
	}
	invRepository, err := repository.NewTestInvitationRepository()
	if err != nil {
		return nil, err
	}
	jobExecRepository, err := repository.NewTestJobExecutionRepository()
	if err != nil {
		return nil, err
	}
	artifactRepository, err := repository.NewTestArtifactRepository()
	if err != nil {
		return nil, err
	}
	notifier, err := notify.New(
		serverCfg,
		emailVerificationRepository)
	if err != nil {
		return nil, err
	}
	userManager, err = NewUserManager(
		serverCfg,
		auditRecordRepository,
		userRepository,
		orgRepository,
		orgConfigRepository,
		invRepository,
		emailVerificationRepository,
		subscriptionRepository,
		jobExecRepository,
		artifactRepository,
		notifier,
	)
	return
}

// AssertTestArtifactManager for testing
func AssertTestArtifactManager(serverCfg *config.ServerConfig, t *testing.T) *ArtifactManager {
	mgr, err := TestArtifactManager(serverCfg)
	require.NoError(t, err)
	return mgr
}

// TestArtifactManager for testing
func TestArtifactManager(serverCfg *config.ServerConfig) (manager *ArtifactManager, err error) {
	if serverCfg == nil {
		serverCfg = config.TestServerConfig()
	}
	err = serverCfg.Validate()
	if err != nil {
		return nil, err
	}
	artifactRepository, err := repository.NewTestArtifactRepository()
	if err != nil {
		return nil, err
	}
	artifactService, err := artifacts.NewStub(nil)
	if err != nil {
		return nil, err
	}
	return NewArtifactManager(
		serverCfg,
		artifactRepository,
		artifactService)
}

// TestResourceManager for testing
func TestResourceManager(serverCfg *config.ServerConfig) resource.Manager {
	if serverCfg == nil {
		serverCfg = config.TestServerConfig()
	}
	queueClient := queue.NewStubClient(&serverCfg.CommonConfig)
	return resource.New(serverCfg, queueClient)
}

// AssertTestJobManager for testing
func AssertTestJobManager(serverCfg *config.ServerConfig, t *testing.T) *JobManager {
	mgr, err := TestJobManager(serverCfg)
	require.NoError(t, err)
	return mgr
}

// TestJobManager for testing
func TestJobManager(serverCfg *config.ServerConfig) (manager *JobManager, err error) {
	if serverCfg == nil {
		serverCfg = config.TestServerConfig()
	}
	err = serverCfg.Validate()
	if err != nil {
		return nil, err
	}
	auditRecordRepository, err := repository.NewTestAuditRecordRepository()
	if err != nil {
		return nil, err
	}
	jobDefinitionRepository, err := repository.NewTestJobDefinitionRepository()
	if err != nil {
		return nil, err
	}
	jobRequestRepository, err := repository.NewTestJobRequestRepository()
	if err != nil {
		return nil, err
	}
	jobExecutionRepository, err := repository.NewTestJobExecutionRepository()
	if err != nil {
		return nil, err
	}
	emailVerificationRepository, err := repository.NewTestEmailVerificationRepository()
	if err != nil {
		return nil, err
	}

	artifactManager, err := TestArtifactManager(serverCfg)
	if err != nil {
		return nil, err
	}

	notifier, err := notify.New(
		serverCfg,
		emailVerificationRepository)
	if err != nil {
		return nil, err
	}
	userManager, err := TestUserManager(serverCfg)
	if err != nil {
		return nil, err
	}
	queueClient := queue.NewStubClient(&serverCfg.CommonConfig)
	resourceManager := resource.New(serverCfg, queueClient)
	return NewJobManager(
		serverCfg,
		auditRecordRepository,
		jobDefinitionRepository,
		jobRequestRepository,
		jobExecutionRepository,
		userManager,
		resourceManager,
		artifactManager,
		stats.NewJobStatsRegistry(),
		metrics.New(),
		queueClient,
		notifier,
	)
}
