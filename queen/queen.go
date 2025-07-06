package queen

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/metrics"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/email"
	"plexobject.com/formicary/queen/notify"
	"plexobject.com/formicary/queen/slack"
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
	repoFactory, err := repository.NewLocator(serverCfg)
	if err != nil {
		return err
	}

	// Create web server
	webServer, err := web.NewDefaultWebServer(&serverCfg.Common)
	if err != nil {
		return err
	}

	// Create messaging client
	queueClient, err := queue.NewClientManager().GetClient(ctx, &serverCfg.Common)
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

	artifactService, err := artifacts.New(serverCfg.Common.S3)
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
		repoFactory.LogEventRepository,
		repoFactory.ArtifactRepository,
		artifactService)
	if err != nil {
		return err
	}

	notifier, err := notify.New(
		serverCfg,
		repoFactory.LogEventRepository,
		repoFactory.EmailVerificationRepository)
	if err != nil {
		return err
	}

	userManager, err := manager.NewUserManager(
		serverCfg,
		repoFactory.AuditRecordRepository,
		repoFactory.UserRepository,
		repoFactory.OrgRepository,
		repoFactory.OrgConfigRepository,
		repoFactory.InvitationRepository,
		repoFactory.EmailVerificationRepository,
		repoFactory.SubscriptionRepository,
		repoFactory.JobExecutionRepository,
		repoFactory.ArtifactRepository,
		notifier,
	)
	if err != nil {
		return err
	}

	emailSender, err := email.New(serverCfg, userManager)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"Component": "Queen",
			"ID":        serverCfg.Common.ID,
			"Error":     err,
		}).Warnf("failed to create email-sender")
	} else {
		notifier.AddSender(common.EmailChannel, emailSender)
	}

	slackSender, err := slack.New(serverCfg, userManager)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"Component": "Queen",
			"ID":        serverCfg.Common.ID,
			"Error":     err,
		}).Warnf("failed to create slack-sender")
	} else {
		notifier.AddSender(common.SlackChannel, slackSender)
	}

	jobManager, err := manager.NewJobManager(
		ctx,
		serverCfg,
		repoFactory.AuditRecordRepository,
		repoFactory.JobDefinitionRepository,
		repoFactory.JobRequestRepository,
		repoFactory.JobExecutionRepository,
		userManager,
		resourceManager,
		artifactManager,
		jobStatsRegistry,
		metricsRegistry,
		queueClient,
		notifier,
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
			userManager,
			resourceManager,
			repoFactory.ErrorCodeRepository,
			healthMonitor,
			metricsRegistry,
		)
		if err = jobScheduler.Start(ctx); err != nil {
			return err
		}
	}

	// request registry keeps track of requests and is used by tasklet
	requestRegistry := ctasklet.NewRequestRegistry(&serverCfg.Common, metricsRegistry)

	// starts artifact-expiration tasklet
	if err = tasklet.NewArtifactExpirationTasklet(
		serverCfg,
		requestRegistry,
		artifactManager,
		queueClient,
		serverCfg.Common.GetExpireArtifactsTaskletTopic(),
	).Start(ctx); err != nil {
		return fmt.Errorf("failed to create artifact expiration tasklet due to %w", err)
	}

	// starts artifact-expiration tasklet
	if err = tasklet.NewManualTasklet(
		serverCfg,
		requestRegistry,
		queueClient,
	).Start(ctx); err != nil {
		return fmt.Errorf("failed to create manual tasklet due to %w", err)
	}

	// starts messaging tasklet
	if err = tasklet.NewMessagingTasklet(
		serverCfg,
		requestRegistry,
		jobManager,
		queueClient,
		serverCfg.Common.GetMessagingTaskletTopic(),
	).Start(ctx); err != nil {
		return fmt.Errorf("failed to create messaging tasklet due to %w", err)
	}

	// starts job-fork tasklet that runs on the server side to fork jobs
	if err = tasklet.NewJobForkTasklet(
		serverCfg,
		requestRegistry,
		jobManager,
		queueClient,
		serverCfg.Common.GetForkJobTaskletTopic(),
	).Start(ctx); err != nil {
		return fmt.Errorf("failed to create fork-job tasklet due to %w", err)
	}

	// starts job-fork-await tasklet that runs on the server side to wait for forked jobs
	if err = tasklet.NewJobForkWaitTasklet(
		serverCfg,
		requestRegistry,
		jobManager,
		queueClient,
		serverCfg.Common.GetWaitForkJobTaskletTopic(),
	).Start(ctx); err != nil {
		return fmt.Errorf("failed to create fork-job tasklet due to %w", err)
	}

	// Register job launcher that listen to event topic and starts executing job in goroutine
	jobLauncher := launcher.New(
		serverCfg,
		queueClient,
		jobManager,
		artifactManager,
		userManager,
		resourceManager,
		repoFactory.ErrorCodeRepository,
		metricsRegistry)
	if err = jobLauncher.Start(ctx); err != nil {
		return err
	}

	// listen for signal to cleanly shutdown by finishing the work first before exit
	serverCfg.Common.AddSignalHandlerForShutdown(func() {
		go func() {
			// stop job scheduler from processing more jobs
			_ = jobScheduler.Stop(ctx)
			// wait until all current jobs are done
			for jobLauncher.CountProcessingJobs() > 0 {
				logrus.WithFields(logrus.Fields{
					"Component":      "Queen",
					"ID":             serverCfg.Common.ID,
					"InProgressJobs": jobLauncher.CountProcessingJobs(),
				}).Warnf("shutting down, waiting for job launcher to finish jobs...")
				time.Sleep(1 * time.Second)
			}
			// in the end stop web server
			webServer.Stop()
			logrus.WithFields(logrus.Fields{
				"Component": "Queen",
				"ID":        serverCfg.Common.ID,
			}).Warnf("shutting down, finished waiting for job launcher to finish jobs, exiting...")
		}()
	})

	if serverCfg.HasEmbeddedAnt() {
		if err = serverCfg.EmbeddedAnt.Validate(); err != nil {
			logrus.WithFields(logrus.Fields{
				"Tags":    serverCfg.EmbeddedAnt.Tags,
				"Methods": serverCfg.EmbeddedAnt.Methods,
			}).WithError(serverCfg.EmbeddedAnt.Validate()).Error("Embedded ants could not be configured")
			return err
		}
		serverCfg.EmbeddedAnt.Common = serverCfg.Common
		// Create embedded ants manager
		embeddedAntsManager, _ := NewEmbeddedAntsManager(serverCfg)
		// Start embedded ants first (they run asynchronously)
		if err = embeddedAntsManager.Start(queueClient, artifactService); err != nil {
			logrus.WithFields(logrus.Fields{
				"Error": err,
			}).Error("Failed to start embedded ants")
			// Don't fail the entire server startup for embedded ants
		} else {
			logrus.WithFields(logrus.Fields{
				"Tags":    serverCfg.EmbeddedAnt.Tags,
				"Methods": serverCfg.EmbeddedAnt.Methods,
			}).Info("Embedded ants configured")

			// Set up graceful shutdown for embedded ants
			go func() {
				<-ctx.Done()
				logrus.Info("Shutting down embedded ants...")
				if err := embeddedAntsManager.Stop(); err != nil {
					logrus.WithFields(logrus.Fields{
						"Error": err,
					}).Error("Error stopping embedded ants")
				}
			}()
		}
	}

	logrus.WithFields(logrus.Fields{
		"ID":   serverCfg.Common.ID,
		"Port": serverCfg.Common.HTTPPort}).
		Infof("starting server...")
	// starts web server for APIs
	if err = server.StartWebServer(
		ctx,
		serverCfg,
		repoFactory,
		userManager,
		jobManager,
		dashboardStats,
		resourceManager,
		requestRegistry,
		artifactManager,
		jobStatsRegistry,
		healthMonitor,
		queueClient,
		webServer,
		web.New(&serverCfg.Common)); err != nil {
		return err
	}
	return nil
}

// ///////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////
func buildHealthMonitor(
	ctx context.Context,
	serverCfg *config.ServerConfig,
	queueClient queue.Client) (healthMonitor *health.Monitor, err error) {
	// Create resource manager for keeping track of ants
	healthMonitor, err = health.New(&serverCfg.Common, queueClient)
	if err != nil {
		return nil, err
	}

	for i, endpoint := range serverCfg.Common.Queue.Endpoints {
		if monitor, err := health.NewHostPortMonitor(
			fmt.Sprintf("%s-%d", serverCfg.Common.Queue.Provider, i), endpoint); err != nil {
			return nil, err
		} else {
			healthMonitor.Register(ctx, monitor)
		}
	}
	if serverCfg.Common.Redis.Host != "" {
		var redisMonitor health.Monitorable
		if redisMonitor, err = health.NewHostPortMonitor("redis",
			fmt.Sprintf("%s:%d", serverCfg.Common.Redis.Host, serverCfg.Common.Redis.Port)); err != nil {
			return nil, err
		}
		healthMonitor.Register(ctx, redisMonitor)
	}

	if serverCfg.DB.Type != "sqlite" {
		var dbMonitor health.Monitorable
		if dbMonitor, err = health.NewHostPortMonitor("database", serverCfg.DB.DataSource); err != nil {
			return nil, err
		}
		healthMonitor.Register(ctx, dbMonitor)
	}
	var s3Monitor health.Monitorable
	if s3Monitor, err = health.NewHostPortMonitor("S3", serverCfg.Common.S3.Endpoint); err != nil {
		return nil, err
	}
	healthMonitor.Register(ctx, s3Monitor)

	if err = healthMonitor.Start(ctx); err != nil {
		return nil, err
	}
	return healthMonitor, nil
}
