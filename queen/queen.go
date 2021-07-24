package queen

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/metrics"
	"plexobject.com/formicary/internal/types"
	"time"

	"plexobject.com/formicary/internal/artifacts"
	ctasklet "plexobject.com/formicary/internal/tasklet"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/stats"
	"plexobject.com/formicary/queen/tasklet"

	"plexobject.com/formicary/internal/health"

	"plexobject.com/formicary/internal/queue"
	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/launcher"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/resource"
	"plexobject.com/formicary/queen/scheduler"
	"plexobject.com/formicary/queen/server"
)

// Start starts all services for formicary server
func Start(ctx context.Context, serverCfg *config.ServerConfig) error {
	repoFactory, err := repository.NewFactory(serverCfg)
	if err != nil {
		return err
	}

	// Create web server
	webServer, err := web.NewDefaultWebServer(&serverCfg.CommonConfig)
	if err != nil {
		return err
	}

	// Create messaging client
	queueClient, err := queue.NewMessagingClient(&serverCfg.CommonConfig)
	if err != nil {
		return err
	}

	// Create resource manager for keeping track of ants
	resourceManager := resource.New(serverCfg, queueClient)
	if err = resourceManager.Start(ctx); err != nil {
		return err
	}

	healthMonitor, err := buildHealthMonitor(ctx, serverCfg, queueClient)
	if err != nil {
		return err
	}

	artifactService, err := artifacts.New(&serverCfg.S3)
	if err != nil {
		return err
	}

	jobStatsRegistry := stats.NewJobStatsRegistry()

	dashboardStats := manager.NewDashboardManager(
		serverCfg,
		repoFactory,
		jobStatsRegistry,
		resourceManager,
		healthMonitor)

	metricsRegistry := metrics.New()

	artifactManager, err := manager.NewArtifactManager(
		serverCfg,
		repoFactory.ArtifactRepository,
		artifactService)
	if err != nil {
		return err
	}

	jobManager, err := manager.NewJobManager(
		serverCfg,
		repoFactory.AuditRecordRepository,
		repoFactory.JobDefinitionRepository,
		repoFactory.JobRequestRepository,
		repoFactory.JobExecutionRepository,
		repoFactory.UserRepository,
		repoFactory.OrgRepository,
		resourceManager,
		artifactManager,
		jobStatsRegistry,
		metricsRegistry,
		queueClient,
	)
	if err != nil {
		return err
	}

	// JobScheduler needs to run as a leader so that it can properly manage resources
	// DisableJobScheduler can be used to disable job scheduler if multiple instances of
	// queen servers are running that can execute jobs but only one of them can schedule jobs.
	var jobScheduler *scheduler.JobScheduler
	if !serverCfg.Jobs.DisableJobScheduler {
		// Create job scheduler for scheduling pending jobs
		jobScheduler = scheduler.New(
			serverCfg,
			queueClient,
			jobManager,
			artifactManager,
			repoFactory.ErrorCodeRepository,
			repoFactory.UserRepository,
			repoFactory.OrgRepository,
			resourceManager,
			healthMonitor,
			metricsRegistry,
		)
		if err = jobScheduler.Start(ctx); err != nil {
			return err
		}
	}

	// request registry keeps track of requests and is used by tasklet
	requestRegistry := ctasklet.NewRequestRegistry(&serverCfg.CommonConfig, metricsRegistry)

	// starts job-fork tasklet that runs on the server side to fork jobs
	if err = tasklet.NewJobForkTasklet(
		serverCfg,
		requestRegistry,
		jobManager,
		queueClient,
		serverCfg.GetForkJobTaskletTopic(),
	).Start(ctx); err != nil {
		return fmt.Errorf("failed to create fork-job tasklet %v", err)
	}

	// starts job-fork-await tasklet that runs on the server side to wait for forked jobs
	if err = tasklet.NewJobForkWaitTasklet(
		serverCfg,
		requestRegistry,
		jobManager,
		queueClient,
		serverCfg.GetWaitForkJobTaskletTopic(),
	).Start(ctx); err != nil {
		return fmt.Errorf("failed to create fork-job tasklet %v", err)
	}

	// Register job launcher that listen to event topic and starts executing job in goroutine
	jobLauncher := launcher.New(
		serverCfg,
		queueClient,
		jobManager,
		artifactManager,
		repoFactory.ErrorCodeRepository,
		repoFactory.UserRepository,
		repoFactory.OrgRepository,
		resourceManager,
		metricsRegistry)
	if err = jobLauncher.Start(ctx); err != nil {
		return err
	}

	// listen for signal to cleanly shutdown by finishing the work first before exit
	serverCfg.AddSignalHandlerForShutdown(func() {
		go func() {
			// stop job scheduler from processing more jobs
			_ = jobScheduler.Stop(ctx)
			// wait until all current jobs are done
			for jobLauncher.CountProcessingJobs() > 0 {
				logrus.WithFields(logrus.Fields{
					"Component":      "Queen",
					"ID":             serverCfg.ID,
					"InProgressJobs": jobLauncher.CountProcessingJobs(),
				}).Warnf("shutting down, waiting for job launcher to finish jobs...")
				time.Sleep(1 * time.Second)
			}
			// in the end stop web server
			webServer.Stop()
			logrus.WithFields(logrus.Fields{
				"Component": "Queen",
				"ID":        serverCfg.ID,
			}).Warnf("shutting down, finished waiting for job launcher to finish jobs, exiting...")
		}()
	})

	// starts web server for APIs
	if err = server.StartWebServer(
		ctx,
		serverCfg,
		repoFactory,
		jobManager,
		dashboardStats,
		resourceManager,
		artifactManager,
		jobStatsRegistry,
		healthMonitor,
		queueClient,
		webServer); err != nil {
		return err
	}
	return nil
}

/////////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////
func buildHealthMonitor(
	ctx context.Context,
	serverCfg *config.ServerConfig,
	queueClient queue.Client) (healthMonitor *health.Monitor, err error) {
	// Create resource manager for keeping track of ants
	healthMonitor, err = health.New(&serverCfg.CommonConfig, queueClient)
	if err != nil {
		return nil, err
	}

	if serverCfg.MessagingProvider == types.PulsarMessagingProvider {
		var pulsarMonitor health.Monitorable
		if pulsarMonitor, err = health.NewHostPortMonitor("pulsar", serverCfg.Pulsar.URL); err != nil {
			return nil, err
		}
		healthMonitor.Register(ctx, pulsarMonitor)
	} else {
		var redisMonitor health.Monitorable
		if redisMonitor, err = health.NewHostPortMonitor("redis",
			fmt.Sprintf("%s:%d", serverCfg.Redis.Host, serverCfg.Redis.Port)); err != nil {
			return nil, err
		}
		healthMonitor.Register(ctx, redisMonitor)
	}

	if serverCfg.DB.DBType != "sqlite" {
		var dbMonitor health.Monitorable
		if dbMonitor, err = health.NewHostPortMonitor("database", serverCfg.DB.DataSource); err != nil {
			return nil, err
		}
		healthMonitor.Register(ctx, dbMonitor)
	}
	var s3Monitor health.Monitorable
	if s3Monitor, err = health.NewHostPortMonitor("S3", serverCfg.S3.Endpoint); err != nil {
		return nil, err
	}
	healthMonitor.Register(ctx, s3Monitor)

	if err = healthMonitor.Start(ctx); err != nil {
		return nil, err
	}
	return healthMonitor, nil
}
