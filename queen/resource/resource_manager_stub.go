package resource

import (
	"context"
	"fmt"
	"sort"

	"plexobject.com/formicary/internal/events"
	"plexobject.com/formicary/queen/types"

	common "plexobject.com/formicary/internal/types"
)

// ManagerStub for resources
type ManagerStub struct {
	Registry map[string]*common.AntRegistration
	Events   []*events.ContainerLifecycleEvent
}

// NewStub - creates new stub implementation
func NewStub() *ManagerStub {
	return &ManagerStub{
		Registry: make(map[string]*common.AntRegistration),
		Events:   make([]*events.ContainerLifecycleEvent, 0),
	}
}

// Start stub
func (rm *ManagerStub) Start(_ context.Context) (err error) {
	return nil
}

// Stop stub
func (rm *ManagerStub) Stop(_ context.Context) (err error) {
	return nil
}

// Register stub
func (rm *ManagerStub) Register(
	_ context.Context,
	reg *common.AntRegistration) error {
	rm.Registry[reg.AntID] = reg
	return nil
}

// Unregister stub
func (rm *ManagerStub) Unregister(
	_ context.Context,
	id string) (bool, error) {
	delete(rm.Registry, id)
	return true, nil
}

// Registrations returns all registered ants
func (rm *ManagerStub) Registrations() (regs []*common.AntRegistration) {
	regs = make([]*common.AntRegistration, 0)
	for _, reg := range rm.Registry {
		regs = append(regs, reg)
	}
	return
}

// Registration for ants
func (rm *ManagerStub) Registration(id string) *common.AntRegistration {
	return rm.Registry[id]
}

// HasAntsForJobTags - checks if antRegistrations are available for tags
func (rm *ManagerStub) HasAntsForJobTags(
	methods []common.TaskMethod,
	tags []string) error {
	reg, err := rm.getMatchingReservation(
		methods,
		tags)
	if err != nil {
		return nil
	}
	if reg == nil {
		return fmt.Errorf("no matching mocked ant for methods=%v tags=%v", methods, tags)
	}
	return nil
}

// CheckJobResources checks job resources for all tasks
func (rm *ManagerStub) CheckJobResources(
	job *types.JobDefinition) (reservations []*common.AntReservation, err error) {
	reservations = make([]*common.AntReservation, 0)
	var reservationsByTask map[string]*common.AntReservation
	if reservationsByTask, err = rm.doReserveJobResources("", job, true); err != nil {
		return nil, err
	}
	for _, reservation := range reservationsByTask {
		reservations = append(reservations, reservation)
	}
	sort.Slice(reservations, func(i, j int) bool { return reservations[i].AntID < reservations[j].AntID })
	return
}

// ReserveJobResources reserves resources for all tasks within the job
func (rm *ManagerStub) ReserveJobResources(
	requestID string,
	def *types.JobDefinition) (reservations map[string]*common.AntReservation, err error) {
	return rm.doReserveJobResources(requestID, def, false)
}

// ReleaseJobResources release resources for all tasks within the job
func (rm *ManagerStub) ReleaseJobResources(requestID string) (err error) {
	for _, reg := range rm.Registry {
		delete(reg.Allocations, requestID)
	}
	return nil
}

// Reserve - reserves ant for a request
// Note: This is used for a task request to route request to a particular ant, and we must
// match an ant that supports all tags (as opposed to HasAntsForJobTags)
func (rm *ManagerStub) Reserve(
	requestID string,
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
func (rm *ManagerStub) Release(reservation *common.AntReservation) (err error) {
	if reservation == nil {
		return fmt.Errorf("reservation is not specified")
	}

	reg := rm.Registry[reservation.AntID]
	if reg == nil {
		return fmt.Errorf("failed to deallocate, ants=%s request=%s task=%s is no longer registered",
			reservation.AntID, reservation.JobRequestID, reservation.TaskType)
	}
	delete(reg.Allocations, reservation.JobRequestID)
	return nil
}

// TerminateContainer for remote ant
func (rm *ManagerStub) TerminateContainer(
	_ context.Context, _ string, _ string, _ common.TaskMethod) (err error) {
	return nil
}

// CountContainerEvents returns counts of events
func (rm *ManagerStub) CountContainerEvents() (res map[common.TaskMethod]int) {
	res = make(map[common.TaskMethod]int)
	for _, event := range rm.Events {
		res[event.Method] = res[event.Method] + 1
	}
	return
}

// GetContainerEvents returns all events
func (rm *ManagerStub) GetContainerEvents(
	_ int, _ int, _ string) (res []*events.ContainerLifecycleEvent, total int) {
	return rm.Events, len(rm.Events)
}

// ///////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////
func (rm *ManagerStub) doReserve(
	requestID string,
	taskType string,
	method common.TaskMethod,
	tags []string,
	dryRun bool) (*common.AntReservation, error) {
	reg, err := rm.getMatchingReservation([]common.TaskMethod{method}, tags)
	if err != nil {
		return nil, err
	}

	res := common.NewAntReservation(
		reg.AntID,
		reg.AntTopic,
		requestID,
		taskType,
		reg.EncryptionKey,
		reg.CurrentLoad,
		reg.TotalExecuted)
	if !dryRun {
		reg.Allocations[requestID] = common.NewAntAllocation(
			reg.AntID,
			reg.AntTopic,
			requestID,
			taskType)
	}
	return res, nil
}

// Reserve resources for the job
func (rm *ManagerStub) doReserveJobResources(
	requestID string,
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

func (rm *ManagerStub) getMatchingReservation(
	methods []common.TaskMethod,
	tags []string) (*common.AntRegistration, error) {
	if methods == nil || len(methods) == 0 {
		return nil, fmt.Errorf("methods not specified for ant-registration")
	}

	// Matching methods
	for _, reg := range rm.Registry {
		found := 0
	regLoop:
		for _, regMethod := range reg.Methods {
			for _, method := range methods {
				if regMethod == method {
					found++
					break regLoop
				}
			}
		}
		if len(tags) > 0 {
		tagLoop:
			for _, regTag := range reg.Tags {
				for _, tag := range tags {
					if regTag == tag {
						found++
						break tagLoop
					}
				}
			}
			if found == 2 && len(reg.Allocations) < reg.MaxCapacity {
				return reg, nil
			}
		}
		return reg, nil
	}

	return nil, fmt.Errorf("no matching mock ant for methods=%v tags=%v registry=%v", methods, tags, rm.Registry)
}
