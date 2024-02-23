package ants

import (
	"context"
	"fmt"
	"plexobject.com/formicary/ants/executor/utils"
	"plexobject.com/formicary/internal/metrics"
	"time"

	"github.com/sirupsen/logrus"

	"plexobject.com/formicary/ants/controller"
	"plexobject.com/formicary/ants/registry"
	"plexobject.com/formicary/internal/tasklet"

	"plexobject.com/formicary/internal/artifacts"

	"plexobject.com/formicary/ants/config"
	"plexobject.com/formicary/ants/handler"
	"plexobject.com/formicary/internal/queue"
	"plexobject.com/formicary/internal/web"
)

// Start starts ant
func Start(ctx context.Context, antCfg *config.AntConfig) error {
	requestTopic := antCfg.Common.GetRequestTopic()

	metricsRegistry := metrics.New()

	// request registry keeps track of requests and is used by request handler
	requestRegistry := tasklet.NewRequestRegistry(&antCfg.Common, metricsRegistry)

	webClient := web.New(&antCfg.Common)

	webServer, err := web.NewDefaultWebServer(&antCfg.Common)
	if err != nil {
		return fmt.Errorf("failed to create web server due to %w", err)
	}

	// Create messaging client
	queueClient, err := queue.NewMessagingClient(&antCfg.Common)
	if err != nil {
		return fmt.Errorf("failed to connect to pulsar due to %w", err)
	}

	artifactService, err := artifacts.New(&antCfg.Common.S3)
	if err != nil {
		return fmt.Errorf("failed to connect to minio due to %w", err)
	}

	executor := handler.NewRequestExecutor(
		antCfg,
		queueClient,
		webClient,
		artifactService)

	// starts ant container registry
	antContainersRegistry := registry.NewAntContainersRegistry(
		antCfg,
		queueClient,
		metricsRegistry)
	if err = antContainersRegistry.Start(ctx); err != nil {
		return fmt.Errorf("failed to create containers registry due to %w", err)
	}

	// starts subscriber to listen for incoming requests
	if err = handler.NewRequestHandler(
		antCfg,
		queueClient,
		webClient,
		requestRegistry,
		antContainersRegistry,
		metricsRegistry,
		executor,
		requestTopic).Start(ctx); err != nil {
		return fmt.Errorf("failed to create request handler due to %w", err)
	}

	// start reaper to terminate long-running containers
	containerReaper := utils.NewContainersReaper(
		antCfg,
		queueClient,
		webClient,
		metricsRegistry)
	if err = containerReaper.Start(ctx); err != nil {
		return err
	}

	// listen for signal to cleanly shutdown by finishing the work first before exit
	antCfg.Common.AddSignalHandlerForShutdown(func() {
		go func() {
			// base tasklet will automatically stop registering the ant so that it won't receive any work
			// wait until all current requests are done

			// ant needs additional wait time in case other work inflight needs to be finished
			// Also, note the correct way to shutdown is to start new ants and then shutdown old ants so
			// that we don't miss any work
			for i := 0; i < antCfg.PollAttemptsBeforeShutdown; i++ {
				for requestRegistry.Count() > 0 {
					logrus.WithFields(logrus.Fields{
						"Component":          "Ant",
						"I":                  i,
						"ID":                 antCfg.Common.ID,
						"InProgressRequests": requestRegistry.Count(),
					}).Warnf("shutting down, waiting for requests to finish...")
					time.Sleep(1 * time.Second)
				}
				time.Sleep(antCfg.PollIntervalBeforeShutdown)
			}
			_ = containerReaper.Stop(context.Background())
			// in the end stop web server
			webServer.Stop()
			logrus.WithFields(logrus.Fields{
				"Component": "Queen",
				"ID":        antCfg.Common.ID,
			}).Warnf("shutting down, finished waiting for requests, exiting...")
		}()
	})
	// starts the web server for APIs
	controller.StartWebServer(
		antCfg,
		webServer)
	return nil
}
