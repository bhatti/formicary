package scheduler

import (
	"context"
	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/internal/metrics"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/notify"
	"plexobject.com/formicary/queen/types"
	"testing"
	"time"

	"plexobject.com/formicary/internal/artifacts"
	"plexobject.com/formicary/internal/health"
	"plexobject.com/formicary/internal/queue"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/resource"
	"plexobject.com/formicary/queen/stats"
)

func Test_ShouldStartAndStopJobScheduler(t *testing.T) {
	serverCfg := testServerConfig()
	// GIVEN job scheduler
	scheduler := newTestJobScheduler(t, serverCfg)

	// WHEN scheduler is started
	err := scheduler.Start(context.Background())
	// THEN it should not fail
	require.NoError(t, err)
	// WHEN scheduler is stopped
	err = scheduler.Stop(context.Background())
	// THEN it should not fail
	require.NoError(t, err)
}

func newTestJobScheduler(t *testing.T, serverCfg *config.ServerConfig) *JobScheduler {
	queueClient, _ := queue.NewStubClient(&serverCfg.CommonConfig)

	auditRecordRepository, err := repository.NewTestAuditRecordRepository()
	require.NoError(t, err)

	jobDefinitionRepository, err := repository.NewTestJobDefinitionRepository()
	require.NoError(t, err)
	jobRequestRepository, err := repository.NewTestJobRequestRepository()
	require.NoError(t, err)
	jobExecutionRepository, err := repository.NewTestJobExecutionRepository()
	require.NoError(t, err)
	artifactRepository, err := repository.NewTestArtifactRepository()
	require.NoError(t, err)
	errorRepository, err := repository.NewTestErrorCodeRepository()
	require.NoError(t, err)
	userRepository, err := repository.NewTestUserRepository()
	require.NoError(t, err)
	orgRepository, err := repository.NewTestOrganizationRepository()
	require.NoError(t, err)
	emailVerificationRepository, err := repository.NewTestEmailVerificationRepository()
	require.NoError(t, err)
	subscriptionRepository, err := repository.NewTestSubscriptionRepository()
	require.NoError(t, err)
	resourceManager := resource.New(serverCfg, queueClient)
	// Create resource manager for keeping track of ants
	healthMonitor, err := health.New(&serverCfg.CommonConfig, queueClient)
	require.NoError(t, err)

	artifactService, err := artifacts.NewStub(nil)
	require.NoError(t, err)
	artifactManager, err := manager.NewArtifactManager(
		serverCfg,
		artifactRepository,
		artifactService)
	require.NoError(t, err)

	jobStatsRegistry := stats.NewJobStatsRegistry()

	metricsRegistry := metrics.New()

	notifier, err := notify.New(
		serverCfg,
		make(map[common.NotifyChannel]types.Sender),
		emailVerificationRepository)
	require.NoError(t, err)
	userManager, err := manager.NewUserManager(
		serverCfg,
		auditRecordRepository,
		userRepository,
		orgRepository,
		emailVerificationRepository,
		subscriptionRepository,
		notifier,
	)
	require.NoError(t, err)
	jobManager, err := manager.NewJobManager(
		serverCfg,
		auditRecordRepository,
		jobDefinitionRepository,
		jobRequestRepository,
		jobExecutionRepository,
		userManager,
		resourceManager,
		artifactManager,
		jobStatsRegistry,
		metricsRegistry,
		queueClient,
		notifier,
	)
	require.NoError(t, err)

	scheduler := New(
		serverCfg,
		queueClient,
		jobManager,
		artifactManager,
		errorRepository,
		userRepository,
		orgRepository,
		resourceManager,
		healthMonitor,
		metricsRegistry,
	)
	return scheduler
}

func testServerConfig() *config.ServerConfig {
	serverCfg := &config.ServerConfig{}
	serverCfg.S3.AccessKeyID = "admin"
	serverCfg.S3.SecretAccessKey = "password"
	serverCfg.Pulsar.URL = "test"
	serverCfg.Jobs.JobSchedulerLeaderInterval = 2 * time.Second
	serverCfg.Jobs.JobSchedulerCheckPendingJobsInterval = 2 * time.Second
	serverCfg.Jobs.OrphanRequestsTimeout = 5 * time.Second
	serverCfg.Jobs.OrphanRequestsUpdateInterval = 2 * time.Second
	serverCfg.Jobs.MissingCronJobsInterval = 2 * time.Second

	serverCfg.Notify.EmailJobsTemplateFile = "../../public/views/notify/email_notify_job.html"
	serverCfg.Notify.SlackJobsTemplateFile = "../../public/views/notify/slack_notify_job.txt"
	serverCfg.Notify.VerifyEmailTemplateFile = "../../public/views/notify/verify_email.html"
	return serverCfg
}
