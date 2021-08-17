package launcher

import (
	"context"
	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/internal/artifacts"
	"plexobject.com/formicary/internal/metrics"
	"plexobject.com/formicary/internal/queue"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/notify"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/resource"
	"plexobject.com/formicary/queen/stats"
	"plexobject.com/formicary/queen/types"
	"testing"
)

func Test_ShouldStartAndStopJobLauncher(t *testing.T) {
	serverCfg := testTestServerConfig()
	err := serverCfg.Validate()
	require.NoError(t, err)
	// GIVEN job launcher
	launcher := newTestLauncher(t, serverCfg, err)

	// WHEN starting launcher
	err = launcher.Start(context.Background())

	// THEN it should not fail
	require.NoError(t, err)

	// WHEN stopping launcher
	err = launcher.Stop(context.Background())
	// THEN it should not fail
	require.NoError(t, err)
}

func newTestLauncher(t *testing.T, serverCfg *config.ServerConfig, err error) *JobLauncher {
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
	artifactService, err := artifacts.NewStub(nil)
	require.NoError(t, err)
	jobStatsRegistry := stats.NewJobStatsRegistry()
	metricsRegistry := metrics.New()
	artifactManager, err := manager.NewArtifactManager(
		serverCfg,
		artifactRepository,
		artifactService)
	require.NoError(t, err)

	notifier, err := notify.New(serverCfg, make(map[string]types.Sender))
	require.NoError(t, err)
	resourceManager := resource.New(serverCfg, queueClient)
	jobManager, err := manager.NewJobManager(
		serverCfg,
		auditRecordRepository,
		jobDefinitionRepository,
		jobRequestRepository,
		jobExecutionRepository,
		userRepository,
		orgRepository,
		resourceManager,
		artifactManager,
		jobStatsRegistry,
		metricsRegistry,
		queueClient,
		notifier,
	)
	require.NoError(t, err)
	return New(serverCfg,
		queueClient,
		jobManager,
		artifactManager,
		errorRepository,
		userRepository,
		orgRepository,
		resourceManager,
		metricsRegistry,
	)
}

func testTestServerConfig() *config.ServerConfig {
	serverCfg := &config.ServerConfig{}
	serverCfg.S3.AccessKeyID = "admin"
	serverCfg.S3.SecretAccessKey = "password"
	serverCfg.S3.Bucket = "buc"
	serverCfg.Pulsar.URL = "test"
	serverCfg.Redis.Host = "localhost"
	serverCfg.Email.JobsTemplateFile = "../../public/views/email/notify_job.html"
	return serverCfg
}
