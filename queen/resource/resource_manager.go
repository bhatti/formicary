package resource

import (
	"context"
	"fmt"
	"plexobject.com/formicary/internal/events"
	"plexobject.com/formicary/internal/math"
	"sort"
	"time"

	"plexobject.com/formicary/internal/utils"

	"plexobject.com/formicary/queen/types"

	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/queue"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
)

// Manager interface defines methods for checking capacity and allocation of ants
// Note: This code has high accounting/complexity so test it thoroughly
type Manager interface {
	Registrations() []*common.AntRegistration
	Registration(id string) *common.AntRegistration
	HasAntsForJobTags(
		methods []common.TaskMethod,
		tags []string) error
	Reserve(
		requestID uint64,
		taskType string,
		method common.TaskMethod,
		tags []string) (*common.AntReservation, error)
	ReserveJobResources(
		requestID uint64,
		def *types.JobDefinition) (reservations map[string]*common.AntReservation, err error)
	Release(reservation *common.AntReservation) (err error)
	ReleaseJobResources(requestID uint64) (err error)
	CheckJobResources(job *types.JobDefinition) ([]*common.AntReservation, error)
	GetContainerEvents(offset int, limit int, sortBy string) (all []*events.ContainerLifecycleEvent, total int)
	TerminateContainer(ctx context.Context, id string, antID string, method common.TaskMethod) (err error)
	CountContainerEvents() map[common.TaskMethod]int
}

// ManagerImpl for resources
type ManagerImpl struct {
	id                string
	serverCfg         *config.ServerConfig
	queueClient       queue.Client
	registrationTopic string
	state             *State
	ticker            *time.Ticker
}

// New - creates new ManagerImpl for resources
func New(
	serverCfg *config.ServerConfig,
	queueClient queue.Client) *ManagerImpl {
	registrationTopic := serverCfg.GetRegistrationTopic()
	return &ManagerImpl{
		id:                serverCfg.ID + "-resource-manager",
		serverCfg:         serverCfg,
		queueClient:       queueClient,
		registrationTopic: registrationTopic,
		state:             NewState(serverCfg, queueClient),
	}
}

// Start subscription for monitoring antRegistrations
// TODO Start() -- subscribe to lifecycle events to release resources
func (rm *ManagerImpl) Start(ctx context.Context) (err error) {
	if err = rm.subscribeToRegistration(ctx, rm.serverCfg.GetRegistrationTopic()); err != nil {
		return err
	}
	if err = rm.subscribeToJobLifecycleEvent(ctx, rm.serverCfg.GetJobExecutionLifecycleTopic()); err != nil {
		_ = rm.Stop(ctx)
		return err
	}
	if err = rm.subscribeToTaskLifecycleEvent(ctx, rm.serverCfg.GetTaskExecutionLifecycleTopic()); err != nil {
		_ = rm.Stop(ctx)
		return err
	}
	if err = rm.subscribeToContainersLifecycleEvents(ctx, rm.serverCfg.GetContainerLifecycleTopic()); err != nil {
		_ = rm.Stop(ctx)
		return err
	}

	rm.startReaperTicker(ctx)
	return nil
}

// Stop unsubscribes antRegistrations and background ticker
func (rm *ManagerImpl) Stop(ctx context.Context) (err error) {
	if rm.ticker != nil {
		rm.ticker.Stop()
	}
	err1 := rm.queueClient.UnSubscribe(
		ctx,
		rm.serverCfg.GetRegistrationTopic(),
		rm.id,
	)
	err2 := rm.queueClient.UnSubscribe(
		ctx,
		rm.serverCfg.GetJobExecutionLifecycleTopic(),
		rm.id,
	)
	err3 := rm.queueClient.UnSubscribe(
		ctx,
		rm.serverCfg.GetTaskExecutionLifecycleTopic(),
		rm.id,
	)
	err4 := rm.queueClient.UnSubscribe(
		ctx,
		rm.serverCfg.GetContainerLifecycleTopic(),
		rm.id,
	)
	return utils.ErrorsAny(err1, err2, err3, err4)
}

// Registrations returns all registered ants
func (rm *ManagerImpl) Registrations() (regs []*common.AntRegistration) {
	return rm.state.getRegistrations()
}

// Registration for ants
func (rm *ManagerImpl) Registration(id string) *common.AntRegistration {
	return rm.state.getRegistration(id)
}

// HasAntsForJobTags - checks if antRegistrations are available for tags
// Note: A job collects all tags used by tasks but we won't actually use them at the same
// time
func (rm *ManagerImpl) HasAntsForJobTags(
	methods []common.TaskMethod,
	tags []string) error {
	if methods == nil || len(methods) == 0 {
		return fmt.Errorf("methods not specified for ant-registration")
	}

	// Matching methods
	for _, method := range methods {
		if exists, total := rm.state.hasAntsByMethod(method); !exists {
			return fmt.Errorf("no ant for method='%s' antsByMethods=%d", method, total)
		}
	}

	// Matching tags
	for _, tag := range tags { // tag => [ant-id:true]
		antIDs, totalAntsByTags := rm.state.getAntsByTag(tag)
		if len(antIDs) == 0 {
			return fmt.Errorf("no ant for tag='%s' ants-by-tags=%d", tag, totalAntsByTags)
		}
		matched := false

		errors := make([]string, 0)
		for _, antID := range antIDs {
			registration := rm.state.getRegistrationByAnt(antID)
			allocations := rm.state.getAllocationsByAnt(antID) // ant-id => [request-id: allocation]
			if registration == nil || allocations == nil {
				continue // shouldn't happen
			}

			// For backpressure we won't try to schedule a job if ants are overloaded
			if float64(len(allocations)) <= float64(registration.MaxCapacity) {
				matched = true
				break
			} else {
				errors = append(errors,
					fmt.Sprintf("AntID=%s Tag=%s Capacity (%d) > Allocations (%d)",
						antID, tag, registration.MaxCapacity, len(allocations)))
			}
		}
		if !matched {
			return fmt.Errorf("no matching ant for tag='%s' ants-by-tags=%d errors=%v",
				tag, totalAntsByTags, errors)
		}
	}
	return nil
}

// CheckJobResources checks job resources for all tasks
func (rm *ManagerImpl) CheckJobResources(
	job *types.JobDefinition) (reservations []*common.AntReservation, err error) {
	reservations = make([]*common.AntReservation, 0)
	var reservationsByTask map[string]*common.AntReservation
	if reservationsByTask, err = rm.doReserveJobResources(0, job, true); err != nil {
		return nil, err
	}
	for _, reservation := range reservationsByTask {
		reservations = append(reservations, reservation)
	}
	sort.Slice(reservations, func(i, j int) bool { return reservations[i].AntID < reservations[j].AntID })
	return
}

// ReserveJobResources reserves resources for all tasks within the job
func (rm *ManagerImpl) ReserveJobResources(
	requestID uint64,
	def *types.JobDefinition) (reservations map[string]*common.AntReservation, err error) {
	return rm.doReserveJobResources(requestID, def, false)
}

// ReleaseJobResources release resources for all tasks within the job
func (rm *ManagerImpl) ReleaseJobResources(requestID uint64) (err error) {
	return rm.state.releaseJob(requestID)
}

// Reserve - reserves ant for a request
// Note: This is used for a task request to route request to a particular ant and we must
// match a ant that supports all tags (as opposed to HasAntsForJobTags)
func (rm *ManagerImpl) Reserve(
	requestID uint64,
	taskType string,
	method common.TaskMethod,
	tags []string) (*common.AntReservation, error) {
	return rm.doReserve(
		requestID,
		taskType,
		method,
		tags,
		false)
}

// Release deallocates ant for a request
func (rm *ManagerImpl) Release(reservation *common.AntReservation) (err error) {
	if reservation == nil {
		return fmt.Errorf("reservation is not specified")
	}

	if rm.state.getRegistrationByAnt(reservation.AntID) == nil {
		return fmt.Errorf("failed to deallocate, ants=%s request=%d task=%s is no longer registered",
			reservation.AntID, reservation.JobRequestID, reservation.TaskType)
	}

	err = rm.state.release(reservation)

	if err != nil {
		logrus.WithFields(logrus.Fields{
			"AntID":     reservation.AntID,
			"TaskType":  reservation.TaskType,
			"RequestID": reservation.JobRequestID,
			"Error":     err,
		}).Warn("Error releasing ant for task request")
	} else {
		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			logrus.WithFields(logrus.Fields{
				"AntID":     reservation.AntID,
				"TaskType":  reservation.TaskType,
				"RequestID": reservation.JobRequestID,
				"Error":     err,
			}).Debug("releasing ant for task request")
		}
	}

	return err
}

// TerminateContainer for remote ant
func (rm *ManagerImpl) TerminateContainer(ctx context.Context, id string, antID string, method common.TaskMethod) (err error) {
	return rm.state.terminateContainer(ctx, id, antID, method)
}

// CountContainerEvents returns counts of events
func (rm *ManagerImpl) CountContainerEvents() map[common.TaskMethod]int {
	return rm.state.countContainerEvents()
}

// GetContainerEvents returns all events
func (rm *ManagerImpl) GetContainerEvents(offset int, limit int, sortBy string) (res []*events.ContainerLifecycleEvent, total int) {
	all := rm.state.getContainerEvents(sortBy)
	total = len(all)
	res = make([]*events.ContainerLifecycleEvent, math.Min(limit, len(all)))
	i := 0
	for j, cnt := range all {
		if j < offset {
			continue
		} else if i >= len(res) {
			break
		}
		res[i] = cnt
		i++
	}
	return
}

/////////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////
func (rm *ManagerImpl) register(
	ctx context.Context,
	registration *common.AntRegistration) error {
	registration.ReceivedAt = time.Now()
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component": "ResourceManager",
			"AntID":     registration.AntID,
			"Capacity":  registration.MaxCapacity,
			"Load":      registration.CurrentLoad,
			"Methods":   registration.Methods,
			"Tags":      registration.Tags,
		}).Debug("received ant registration")
	}
	// update mapping of ant-id => registration
	rm.state.addRegistration(ctx, registration)

	return nil
}

func (rm *ManagerImpl) doReserve(
	requestID uint64,
	taskType string,
	method common.TaskMethod,
	tags []string,
	dryRun bool) (*common.AntReservation, error) {
	if method == "" {
		return nil, fmt.Errorf("method not specified")
	}
	return rm.state.reserve(requestID, taskType, method, tags, dryRun)
}

// Reserve resources for the job
func (rm *ManagerImpl) doReserveJobResources(
	requestID uint64,
	def *types.JobDefinition,
	dryRun bool) (reservations map[string]*common.AntReservation, err error) {
	reservations = make(map[string]*common.AntReservation)
	var alloc *common.AntReservation
	for _, task := range def.Tasks {
		// reserve another ant
		alloc, err = rm.doReserve(
			requestID,
			task.TaskType,
			task.Method,
			task.Tags,
			dryRun)
		if err == nil {
			reservations[task.TaskType] = alloc
		} else {
			if !dryRun {
				// release all allocations so far and return with error
				_ = rm.ReleaseJobResources(requestID)
			}
			return
		}
	}
	return
}
