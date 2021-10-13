package registry

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"plexobject.com/formicary/internal/metrics"

	cutils "plexobject.com/formicary/internal/utils"

	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/ants/config"
	"plexobject.com/formicary/ants/executor/utils"
	"plexobject.com/formicary/internal/events"
	"plexobject.com/formicary/internal/queue"

	"plexobject.com/formicary/internal/types"
)

// ContainerStatus status of container
type ContainerStatus string

const (
	// ContainerNonExistent - container does not exist
	ContainerNonExistent ContainerStatus = "NON_EXISTENT"
	// ContainerExistsWithGoodAnt - container is being run by another good ant
	ContainerExistsWithGoodAnt ContainerStatus = "EXISTS_WITH_GOOD_ANT"
	// OrphanContainer - container is running but ant died or restarted
	OrphanContainer ContainerStatus = "ORPHAN_CONTAINER"
)

// AntContainersRegistry keeps track of running containers
type AntContainersRegistry struct {
	id                              string
	antCfg                          *config.AntConfig
	queueClient                     queue.Client
	metricsRegistry                 *metrics.Registry
	registrations                   map[string]*types.AntRegistration          // ant-id => registration
	containersEvents                map[string]*events.ContainerLifecycleEvent // method+container-name: container event
	lock                            sync.RWMutex
	registrationSubscriberID        string
	containersLifecycleSubscriberID string
}

// NewAntContainersRegistry constructor
func NewAntContainersRegistry(
	antCfg *config.AntConfig,
	queueClient queue.Client,
	metricsRegistry *metrics.Registry,
) *AntContainersRegistry {
	return &AntContainersRegistry{
		id:               antCfg.ID + "-container-registry",
		antCfg:           antCfg,
		queueClient:      queueClient,
		metricsRegistry:  metricsRegistry,
		registrations:    make(map[string]*types.AntRegistration),          // ant-id => registration
		containersEvents: make(map[string]*events.ContainerLifecycleEvent), // method + container-name: container event
	}
}

// Start subscription for monitoring registrations
func (r *AntContainersRegistry) Start(ctx context.Context) (err error) {
	if r.registrationSubscriberID, err = r.subscribeToRegistration(ctx, r.antCfg.GetRegistrationTopic()); err != nil {
		return err
	}
	if r.containersLifecycleSubscriberID, err = r.subscribeToContainersLifecycleEvents(ctx, r.antCfg.GetContainerLifecycleTopic()); err != nil {
		_ = r.Stop(ctx)
		return err
	}
	r.registerAlreadyRunningContainers(ctx)
	return nil
}

// Stop unsubscribes registrations and background ticker
func (r *AntContainersRegistry) Stop(ctx context.Context) (err error) {
	err1 := r.queueClient.UnSubscribe(
		ctx,
		r.antCfg.GetRegistrationTopic(),
		r.registrationSubscriberID)
	err2 := r.queueClient.UnSubscribe(
		ctx,
		r.antCfg.GetContainerLifecycleTopic(),
		r.containersLifecycleSubscriberID)
	return cutils.ErrorsAny(err1, err2)
}

// GetContainerEvents returns all events
func (r *AntContainersRegistry) GetContainerEvents() (all []*events.ContainerLifecycleEvent) {
	r.lock.RLock()
	all = make([]*events.ContainerLifecycleEvent, len(r.containersEvents))
	i := 0
	for _, v := range r.containersEvents {
		all[i] = v
		i++
	}
	r.lock.RUnlock()
	sort.Slice(all, func(i, j int) bool { return all[i].CreatedAt.Unix() > all[j].CreatedAt.Unix() })
	return
}

// UpdateContainer adds container to local registry upon start and removes it upon end
func (r *AntContainersRegistry) UpdateContainer(
	_ context.Context,
	containerEvent *events.ContainerLifecycleEvent) error {
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component":      "AntContainersRegistry",
			"ContainerEvent": containerEvent,
		}).Debug("received container lifecycle")
	}

	// update mapping of ant-id => registration
	r.lock.Lock()
	defer r.lock.Unlock()
	if containerEvent.ContainerState == types.STARTED {
		r.containersEvents[containerEvent.Key()] = containerEvent
		r.metricsRegistry.Incr(
			"container_started_total",
			map[string]string{
				"ORG": containerEvent.Labels[types.OrgID],
			})
	} else if containerEvent.ContainerState.IsTerminal() {
		r.metricsRegistry.Incr(
			"container_ended_total",
			map[string]string{
				"ORG": containerEvent.Labels[types.OrgID],
			})
		delete(r.containersEvents, containerEvent.Key())
	} else {
		return fmt.Errorf("unsupported container event %s", containerEvent)
	}

	return nil
}

// GetContainerEvent checks if container exists
func (r *AntContainersRegistry) GetContainerEvent(
	method types.TaskMethod,
	containerName string) *events.ContainerLifecycleEvent {
	r.lock.RLock()
	defer r.lock.RUnlock()
	key := events.ContainerLifecycleEventKey(method, containerName)
	return r.containersEvents[key]
}

// CheckIfAlreadyRunning checks if container is already running
func (r *AntContainersRegistry) CheckIfAlreadyRunning(
	method types.TaskMethod,
	containerName string) (ContainerStatus, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()
	key := events.ContainerLifecycleEventKey(method, containerName)
	containerEvent := r.containersEvents[key]
	if containerEvent == nil {
		return ContainerNonExistent, nil
	}
	registration := r.registrations[containerEvent.AntID]
	if registration == nil {
		return OrphanContainer,
			fmt.Errorf("method %s container %s is already running but ant %s no longer listening, last status %s at %s",
				method, containerName, containerEvent.AntID, containerEvent.ContainerState, containerEvent.CreatedAt)
	}
	if registration.AntStartedAt.Unix() < containerEvent.CreatedAt.Unix() {
		return ContainerExistsWithGoodAnt,
			fmt.Errorf("method %s container %s is already running by the ant %s so it should respond, last status %s at %s",
				method, containerName, containerEvent.AntID, containerEvent.ContainerState, containerEvent.CreatedAt)
	}
	return OrphanContainer,
		fmt.Errorf("method %s container %s is already running by the ant %s but it was restarted, last status %s at %s",
			method, containerName, containerEvent.AntID, containerEvent.ContainerState, containerEvent.CreatedAt)
}

/////////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////
func (r *AntContainersRegistry) registerAnt(
	_ context.Context,
	registration *types.AntRegistration) error {
	registration.ReceivedAt = time.Now()
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component": "AntContainersRegistry",
			"AntID":     registration.AntID,
			"Capacity":  registration.MaxCapacity,
			"Load":      registration.CurrentLoad,
			"Methods":   registration.Methods,
			"Tags":      registration.Tags,
		}).Debug("received ant registration")
	}

	// update mapping of ant-id => registration
	r.lock.Lock()
	defer r.lock.Unlock()
	// update mapping of ant-id => registration
	r.registrations[registration.AntID] = registration

	return nil
}

func (r *AntContainersRegistry) subscribeToContainersLifecycleEvents(
	ctx context.Context,
	containerTopic string) (string, error) {
	return r.queueClient.Subscribe(
		ctx,
		containerTopic,
		false, // shared subscription
		func(ctx context.Context, event *queue.MessageEvent) error {
			defer event.Ack()
			containerEvent, err := events.UnmarshalContainerLifecycleEvent(event.Payload)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"Component": "AntContainersRegistry",
					"Payload":   string(event.Payload),
					"Target":    r.id,
					"Error":     err}).Error("failed to unmarshal registration by container registry")
				return err
			}
			if err := r.UpdateContainer(ctx, containerEvent); err != nil {
				logrus.WithFields(logrus.Fields{
					"Component":      "AntContainersRegistry",
					"ContainerEvent": containerEvent,
					"Target":         r.id,
					"Error":          err}).Error("failed to register ant")
			}
			return nil
		},
		make(map[string]string),
	)
}
func (r *AntContainersRegistry) subscribeToRegistration(
	ctx context.Context,
	registrationTopic string) (string, error) {
	return r.queueClient.Subscribe(
		ctx,
		registrationTopic,
		false, // shared subscription
		func(ctx context.Context, event *queue.MessageEvent) error {
			defer event.Ack()
			registration, err := types.UnmarshalAntRegistration(event.Payload)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					"Component":         "AntContainersRegistry",
					"RegistrationTopic": registrationTopic,
					"Registration":      registration,
					"Payload":           string(event.Payload),
					"Target":            r.id,
					"Error":             err}).Error("failed to unmarshal registration by ant container registry")
				return err
			}
			if err := r.registerAnt(ctx, registration); err != nil {
				logrus.WithFields(logrus.Fields{
					"Component":    "AntContainersRegistry",
					"Registration": registration,
					"Target":       r.id,
					"Error":        err}).Error("failed to register ant")
			}
			return nil
		},
		make(map[string]string),
	)
}

func (r *AntContainersRegistry) registerAlreadyRunningContainers(ctx context.Context) {
	added := 0
	containersByMethod := utils.AllRunningContainers(ctx, r.antCfg)
	for k, containers := range containersByMethod {
		for _, container := range containers {
			antID := container.GetLabels()[types.AntID]
			if antID == "" {
				continue
			}
			userID := container.GetLabels()[types.UserID]
			state := types.STARTED
			if container.GetState().Done() {
				state = types.COMPLETED
			} else if container.GetState().ReadyOrRunning() {
				state = types.STARTED
			} else {
				logrus.WithFields(logrus.Fields{
					"Component": "AntContainersRegistry",
					"Container": container,
					"State":     state,
				}).Warn("unknown container state")
			}
			containerEvent := events.NewContainerLifecycleEvent(
				"AntContainersRegistry",
				userID,
				antID,
				k,
				container.GetName(),
				container.GetID(),
				state,
				container.GetLabels(),
				container.GetStartedAt(),
				container.GetEndedAt(),
			)
			_ = r.UpdateContainer(ctx, containerEvent)
			added++
		}
	}
	if added > 0 {
		logrus.WithFields(logrus.Fields{
			"Component":          "AntContainersRegistry",
			"ContainersByMethod": len(containersByMethod),
			"ContainersEvents":   len(r.containersEvents),
			"Added":              added,
		}).Info("added already running containers")
	}
}
