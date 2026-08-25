package resource

import (
	"context"
	"fmt"
	"plexobject.com/formicary/internal/events"
	"plexobject.com/formicary/internal/math"
	"sort"
	"strings"
	"sync"
	"time"

	"plexobject.com/formicary/internal/utils"

	"plexobject.com/formicary/queen/types"

	"github.com/sirupsen/logrus"
	"plexobject.com/formicary/internal/queue"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
	queenhealth "plexobject.com/formicary/queen/health"
)

// Manager interface defines methods for checking capacity and allocation of ants
// Note: This code has high accounting/complexity so test it thoroughly
type Manager interface {
	Register(
		ctx context.Context,
		registration *common.AntRegistration) error
	Unregister(
		ctx context.Context,
		id string) (bool, error)
	Registrations() []*common.AntRegistration
	// RegistrationsByOrg returns only ants belonging to the given org.
	// When orgID is empty (admin or auth-disabled), all registrations are returned.
	RegistrationsByOrg(orgID string) []*common.AntRegistration
	Registration(id string) *common.AntRegistration
	HasAntsForJobTags(
		methods []common.TaskMethod,
		tags []string,
		orgID string) error
	Reserve(
		requestID string,
		taskType string,
		method common.TaskMethod,
		tags []string,
		orgID string) (*common.AntReservation, error)
	ReserveJobResources(
		requestID string,
		orgID string,
		def *types.JobDefinition) (reservations map[string]*common.AntReservation, err error)
	Release(reservation *common.AntReservation) (err error)
	ReleaseJobResources(requestID string) (err error)
	CheckJobResources(job *types.JobDefinition) ([]*common.AntReservation, error)
	GetContainerEvents(offset int, limit int, sortBy string) (all []*events.ContainerLifecycleEvent, total int)
	// GetContainerEventsByOrg returns only container events whose AntID suffix matches orgID.
	// When orgID is empty, all events are returned (same as GetContainerEvents).
	GetContainerEventsByOrg(orgID string, offset int, limit int, sortBy string) (all []*events.ContainerLifecycleEvent, total int)
	TerminateContainer(ctx context.Context, id string, antID string, method common.TaskMethod) (err error)
	CountContainerEvents() map[common.TaskMethod]int
	SetBannerBridge(b *queenhealth.BannerHealthBridge)
}

// ManagerImpl for resources
type ManagerImpl struct {
	id                string
	serverCfg         *config.ServerConfig
	queueClient       queue.Client
	registrationTopic string
	state             *State
	ticker            *time.Ticker
	stopped           bool
	bannerBridge      *queenhealth.BannerHealthBridge
	lock              sync.RWMutex

	registrationSubscriptionID           string
	jobExecutionLifecycleSubscriptionID  string
	taskExecutionLifecycleSubscriptionID string
	containerLifecycleSubscriptionID     string
}

// SetBannerBridge configures the bridge that writes org-scoped banners on ant registration changes.
func (rm *ManagerImpl) SetBannerBridge(b *queenhealth.BannerHealthBridge) {
	rm.lock.Lock()
	defer rm.lock.Unlock()
	rm.bannerBridge = b
}

// New - creates new ManagerImpl for resources
func New(
	serverCfg *config.ServerConfig,
	queueClient queue.Client) *ManagerImpl {
	registrationTopic := serverCfg.Common.GetRegistrationTopic()
	return &ManagerImpl{
		id:                serverCfg.Common.ID + "-resource-manager",
		serverCfg:         serverCfg,
		queueClient:       queueClient,
		registrationTopic: registrationTopic,
		state:             NewState(serverCfg, queueClient),
	}
}

// Start subscription for monitoring antRegistrations
// TODO Start() -- subscribe to lifecycle events to release resources
func (rm *ManagerImpl) Start(ctx context.Context) (err error) {
	if rm.registrationSubscriptionID, err = rm.subscribeToRegistration(ctx, rm.serverCfg.Common.GetRegistrationTopic()); err != nil {
		return err
	}
	if rm.jobExecutionLifecycleSubscriptionID, err = rm.subscribeToJobLifecycleEvent(ctx, rm.serverCfg.Common.GetJobExecutionLifecycleTopic()); err != nil {
		_ = rm.Stop(ctx)
		return err
	}
	if rm.taskExecutionLifecycleSubscriptionID, err = rm.subscribeToTaskLifecycleEvent(ctx, rm.serverCfg.Common.GetTaskExecutionLifecycleTopic()); err != nil {
		_ = rm.Stop(ctx)
		return err
	}
	if rm.containerLifecycleSubscriptionID, err = rm.subscribeToContainersLifecycleEvents(ctx, rm.serverCfg.Common.GetContainerLifecycleTopic()); err != nil {
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
		rm.serverCfg.Common.GetRegistrationTopic(),
		rm.registrationSubscriptionID)
	err2 := rm.queueClient.UnSubscribe(
		ctx,
		rm.serverCfg.Common.GetJobExecutionLifecycleTopic(),
		rm.jobExecutionLifecycleSubscriptionID)
	err3 := rm.queueClient.UnSubscribe(
		ctx,
		rm.serverCfg.Common.GetTaskExecutionLifecycleTopic(),
		rm.taskExecutionLifecycleSubscriptionID)
	err4 := rm.queueClient.UnSubscribe(
		ctx,
		rm.serverCfg.Common.GetContainerLifecycleTopic(),
		rm.containerLifecycleSubscriptionID)
	rm.lock.Lock()
	rm.stopped = true
	rm.lock.Unlock()
	return utils.ErrorsAny(err1, err2, err3, err4)
}

// Registrations returns all registered ants
func (rm *ManagerImpl) Registrations() (regs []*common.AntRegistration) {
	return rm.state.getRegistrations()
}

// RegistrationsByOrg returns ants belonging to the given org (or all when orgID is empty).
func (rm *ManagerImpl) RegistrationsByOrg(orgID string) []*common.AntRegistration {
	if orgID == "" {
		return rm.state.getRegistrations()
	}
	all := rm.state.getRegistrations()
	filtered := make([]*common.AntRegistration, 0, len(all))
	for _, r := range all {
		if r.OrgID == orgID || r.OrgID == "" {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// Registration for ants
func (rm *ManagerImpl) Registration(id string) *common.AntRegistration {
	return rm.state.getRegistration(id)
}

// HasAntsForJobTags - checks if live antRegistrations are available for methods and tags.
// Note: A job collects all tags used by tasks, but we won't actually use them at the same time.
// orgID is used to pre-filter to org-scoped ants when auth is enabled; "" means no filter.
func (rm *ManagerImpl) HasAntsForJobTags(
	methods []common.TaskMethod,
	tags []string,
	orgID string) error {
	if methods == nil || len(methods) == 0 {
		return fmt.Errorf("methods not specified for ant-registration")
	}
	aliveTimeout := rm.serverCfg.Jobs.AntRegistrationAliveTimeout

	// Matching methods — only consider ants that are alive and support the method.
	// When orgID is set, restrict to org-scoped candidates (with unscoped fallback).
	for _, method := range methods {
		antIDs := rm.state.getAntIDsByMethodAndOrg(method, orgID)
		liveCount := 0
		for _, antID := range antIDs {
			reg := rm.state.getRegistrationByAnt(antID)
			if reg != nil && reg.Supports(method, nil, aliveTimeout) {
				liveCount++
			}
		}
		if liveCount == 0 {
			logrus.WithFields(logrus.Fields{
				"Methods": methods,
				"Tags":    tags,
				"OrgID":   orgID,
				"Dump":    rm.state.dump(false),
			}).Warnf("no live ant for method: %s", method)
			return fmt.Errorf("no live ant for method='%s'", method)
		}
	}

	// Matching tags — only consider live ants that pass the same org filter used in reserve().
	// getAntIDsByMethodAndOrg applies the scoped→unscoped fallback; we replicate that here
	// per-tag so HasAntsForJobTags and reserve() are consistent: if HasAntsForJobTags says
	// "OK" for a (tag, orgID) pair then reserve() will also be able to find a matching ant.
	for _, tag := range tags {
		antIDs, totalAntsByTags := rm.state.getAntsByTag(tag)
		if len(antIDs) == 0 {
			logrus.WithFields(logrus.Fields{
				"Methods": methods,
				"Tags":    tags,
				"OrgID":   orgID,
				"Dump":    rm.state.dump(false),
			}).Warnf("failed to find ant by tags: %s", tag)
			return fmt.Errorf("no ant for tag='%s' ants-by-tags=%d", tag, totalAntsByTags)
		}

		// Build the org-filtered candidate set for this tag using the same scoped→unscoped
		// fallback logic as reserve(), so both functions agree on what constitutes a match.
		orgMatchedIDs := rm.state.filterAntIDsByOrg(antIDs, orgID)

		matched := false
		errors := make([]string, 0)
		for _, antID := range orgMatchedIDs {
			registration := rm.state.getRegistrationByAnt(antID)
			allocations := rm.state.getAllocationsByAnt(antID)
			if registration == nil || allocations == nil {
				continue
			}
			if !registration.IsAlive(aliveTimeout) {
				errors = append(errors, fmt.Sprintf("AntID=%s stale (last seen %s)", antID, registration.ReceivedAt.Format("15:04:05")))
				continue
			}
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
			orgCandidateCount := len(orgMatchedIDs)
			return fmt.Errorf("no matching live ant for tag='%s' org='%s' "+
				"org-candidate-ants=%d global-ants-with-tag=%d errors=%v",
				tag, orgID, orgCandidateCount, totalAntsByTags, errors)
		}
	}
	return nil
}

// CheckJobResources checks job resources for all tasks
func (rm *ManagerImpl) CheckJobResources(
	job *types.JobDefinition) (reservations []*common.AntReservation, err error) {
	reservations = make([]*common.AntReservation, 0)
	var reservationsByTask map[string]*common.AntReservation
	if reservationsByTask, err = rm.doReserveJobResources("", "", job, true); err != nil {
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
	requestID string,
	orgID string,
	def *types.JobDefinition) (reservations map[string]*common.AntReservation, err error) {
	return rm.doReserveJobResources(requestID, orgID, def, false)
}

// ReleaseJobResources release resources for all tasks within the job
func (rm *ManagerImpl) ReleaseJobResources(requestID string) (err error) {
	return rm.state.releaseJob(requestID)
}

// Reserve - reserves ant for a request
// Note: This is used for a task request to route request to a particular ant, and we must
// match an ant that supports all tags (as opposed to HasAntsForJobTags)
func (rm *ManagerImpl) Reserve(
	requestID string,
	taskType string,
	method common.TaskMethod,
	tags []string,
	orgID string) (*common.AntReservation, error) {
	return rm.doReserve(
		requestID,
		taskType,
		method,
		tags,
		orgID,
		false)
}

// Release deallocates ant for a request
func (rm *ManagerImpl) Release(reservation *common.AntReservation) (err error) {
	if reservation == nil {
		return fmt.Errorf("reservation is not specified")
	}

	if rm.state.getRegistrationByAnt(reservation.AntID) == nil {
		return fmt.Errorf("failed to deallocate, ant-id=%s request=%s task=%s is no longer registered",
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
	return rm.GetContainerEventsByOrg("", offset, limit, sortBy)
}

// GetContainerEventsByOrg returns container events filtered by org.
// When orgID is empty, all events are returned.
// Events are matched by the ant that ran them: AntID has suffix "@<orgID>" when org-scoped.
func (rm *ManagerImpl) GetContainerEventsByOrg(orgID string, offset int, limit int, sortBy string) (res []*events.ContainerLifecycleEvent, total int) {
	all := rm.state.getContainerEvents(sortBy)
	if orgID != "" {
		filtered := make([]*events.ContainerLifecycleEvent, 0, len(all))
		suffix := "@" + orgID
		for _, e := range all {
			if strings.HasSuffix(e.AntID, suffix) || !strings.Contains(e.AntID, "@") {
				filtered = append(filtered, e)
			}
		}
		all = filtered
	}
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

// Register adds registration
func (rm *ManagerImpl) Register(
	ctx context.Context,
	registration *common.AntRegistration) error {
	registration.ReceivedAt = time.Now()
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component":     "ResourceManager",
			"AntID":         registration.AntID,
			"Capacity":      registration.MaxCapacity,
			"Load":          registration.CurrentLoad,
			"TotalExecuted": registration.TotalExecuted,
			"Methods":       registration.Methods,
			"Tags":          registration.Tags,
		}).Debug("register ant worker")
	}
	// update mapping of ant-id => registration
	rm.state.addRegistration(ctx, registration)

	if rm.bannerBridge != nil && registration.OrgID != "" {
		rm.bannerBridge.SyncAntHealth(registration)
		rm.bannerBridge.SyncNoAntForOrg(registration.OrgID, true)
	}

	return nil
}

// Unregister removes registration
func (rm *ManagerImpl) Unregister(
	_ context.Context,
	id string) (bool, error) {
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component": "ResourceManager",
			"AntID":     id,
		}).Debug("unregister ant worker")
	}
	// capture orgID before removal so we can update the banner state
	var orgID string
	if rm.bannerBridge != nil {
		if reg := rm.state.getRegistrationByAnt(id); reg != nil {
			orgID = reg.OrgID
		}
	}

	_, _, count := rm.state.removeRegistration(id)

	if rm.bannerBridge != nil && orgID != "" {
		remaining := rm.RegistrationsByOrg(orgID)
		hasExternal := false
		for _, r := range remaining {
			if r.HasExternalMethods() {
				hasExternal = true
				break
			}
		}
		rm.bannerBridge.SyncNoAntForOrg(orgID, hasExternal)
	}

	return count > 0, nil
}

// ///////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////
func (rm *ManagerImpl) isStopped() bool {
	rm.lock.RLock()
	defer rm.lock.RUnlock()
	return rm.stopped
}

func (rm *ManagerImpl) doReserve(
	requestID string,
	taskType string,
	method common.TaskMethod,
	tags []string,
	orgID string,
	dryRun bool) (*common.AntReservation, error) {
	if method == "" {
		return nil, fmt.Errorf("method not specified")
	}
	return rm.state.reserve(requestID, taskType, method, tags, orgID, dryRun)
}

// Reserve resources for the job
func (rm *ManagerImpl) doReserveJobResources(
	requestID string,
	orgID string,
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
			orgID,
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
