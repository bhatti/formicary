package queen

import (
	"context"
	"fmt"
	"plexobject.com/formicary/internal/ant_config"
	"plexobject.com/formicary/internal/artifacts"
	"plexobject.com/formicary/internal/queue"
	"sync"
	"time"

	"plexobject.com/formicary/ants"
	"plexobject.com/formicary/queen/config"

	log "github.com/sirupsen/logrus"
)

// EmbeddedAntsManager manages embedded ants that run alongside the queen
type EmbeddedAntsManager struct {
	serverConfig  *config.ServerConfig
	ant           *EmbeddedAnt
	mutex         sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	shutdownGroup sync.WaitGroup
}

// EmbeddedAnt represents a single embedded ant instance
type EmbeddedAnt struct {
	ID        string
	Config    *ant_config.AntConfig
	Status    string
	StartedAt time.Time
	cancel    context.CancelFunc
}

// NewEmbeddedAntsManager creates a new embedded ants manager
func NewEmbeddedAntsManager(serverConfig *config.ServerConfig) (*EmbeddedAntsManager, error) {
	ctx, cancel := context.WithCancel(context.Background())
	if !serverConfig.HasEmbeddedAnt() {
		cancel()
		return nil, fmt.Errorf("embedded ant config is not defined")
	}
	return &EmbeddedAntsManager{
		serverConfig: serverConfig,
		ctx:          ctx,
		cancel:       cancel,
	}, nil
}

// Start starts all embedded ants asynchronously
func (eam *EmbeddedAntsManager) Start(queueClient queue.Client, artifactService artifacts.Service) error {
	if !eam.serverConfig.HasEmbeddedAnt() {
		return nil
	}

	eam.mutex.Lock()
	defer eam.mutex.Unlock()

	if err := eam.startAnt(eam.serverConfig.EmbeddedAnt, queueClient, artifactService); err != nil {
		log.WithFields(log.Fields{
			"AntID": &eam.serverConfig.EmbeddedAnt.Common.ID,
			"Error": err,
		}).Error("Failed to start embedded ant")
		// Continue starting other ants even if one fails
	}

	return nil
}

// startAnt starts a single embedded ant
func (eam *EmbeddedAntsManager) startAnt(antConfig *ant_config.AntConfig,
	queueClient queue.Client, artifactService artifacts.Service) error {
	if !eam.serverConfig.HasEmbeddedAnt() {
		return fmt.Errorf("ant is not configured")
	}

	// Create context for this ant
	antCtx, antCancel := context.WithCancel(eam.ctx)

	eam.ant = &EmbeddedAnt{
		ID:        antConfig.Common.ID,
		Config:    antConfig,
		Status:    "starting",
		StartedAt: time.Now(),
		cancel:    antCancel,
	}

	// Start the ant in a goroutine
	eam.shutdownGroup.Add(1)
	go func(ant *EmbeddedAnt) {
		defer eam.shutdownGroup.Done()
		defer func() {
			eam.mutex.Lock()
			ant.Status = "stopped"
			eam.mutex.Unlock()
		}()

		log.WithFields(log.Fields{
			"AntID":       ant.ID,
			"Tags":        ant.Config.Tags,
			"Methods":     ant.Config.Methods,
			"MaxCapacity": ant.Config.MaxCapacity,
		}).Info("Starting embedded ant")

		eam.mutex.Lock()
		ant.Status = "running"
		eam.mutex.Unlock()

		// Start the ant using the existing ants.Start function
		if err := ants.StartEmbedded(antCtx, ant.Config, queueClient, artifactService); err != nil {
			log.WithFields(log.Fields{
				"AntID": ant.ID,
				"Error": err,
			}).Error("Embedded ant topped started with error")
		}
	}(eam.ant)

	// Give the ant a moment to start
	time.Sleep(100 * time.Millisecond)

	log.WithFields(log.Fields{
		"AntID": eam.ant.ID,
		"Tags":  eam.ant.Config.Tags,
	}).Info("Embedded ant started")

	return nil
}

// Stop stops all embedded ants gracefully
func (eam *EmbeddedAntsManager) Stop() error {
	log.Info("Stopping embedded ants...")

	if eam.ant == nil {
		log.Info("No embedded ants to stop")
		return nil
	}

	// Cancel all ant contexts
	eam.cancel()

	// Wait for all ants to stop with timeout
	done := make(chan struct{})
	go func() {
		eam.shutdownGroup.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.WithFields(log.Fields{
			"ID": eam.ant.ID,
		}).Info("Embedded ants stopped gracefully")
	case <-time.After(30 * time.Second):
		log.WithFields(log.Fields{
			"ID": eam.ant.ID,
		}).Warn("Timeout waiting for embedded ants to stop")
	}

	return nil
}

// GetStatus returns the status of all embedded ants
func (eam *EmbeddedAntsManager) GetStatus() *EmbeddedAnt {
	if eam.ant == nil {
		return nil
	}

	eam.mutex.RLock()
	defer eam.mutex.RUnlock()

	// Create a copy to avoid race conditions
	return &EmbeddedAnt{
		ID:        eam.ant.ID,
		Status:    eam.ant.Status,
		StartedAt: eam.ant.StartedAt,
	}
}
