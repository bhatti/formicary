package resource

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"

	"plexobject.com/formicary/internal/events"
	"plexobject.com/formicary/internal/queue"
	"plexobject.com/formicary/queen/config"

	"github.com/sirupsen/logrus"
	common "plexobject.com/formicary/internal/types"
)

var taskTimeout = time.Second * 30

// State for resources
type State struct {
	serverCfg                      *config.ServerConfig
	queueClient                    queue.Client
	antRegistrations               map[string]*common.AntRegistration          // ant-id => registration
	antByTag                       map[string]map[string]bool                  // tag => [ant-id: true]
	antByMethod                    map[common.TaskMethod]map[string]bool       // method=> [ant-id: true]
	antsByRequest                  map[string]map[string]bool                  // request-id => [ant-id: true]
	allocationsByAnt               map[string]map[string]*common.AntAllocation // ant-id => [request-id: allocation]
	containersEvents               map[string]*events.ContainerLifecycleEvent  // method+container-name: container event
	containersEventKeysByRequestID map[string]map[string]bool                  // for removing containers by request
	lock                           sync.RWMutex
}

// NewState - creates new State for resources
func NewState(
	serverCfg *config.ServerConfig,
	queueClient queue.Client) *State {
	return &State{
		serverCfg:                      serverCfg,
		queueClient:                    queueClient,
		antRegistrations:               make(map[string]*common.AntRegistration),          // ant-id => registration
		antByTag:                       make(map[string]map[string]bool),                  // tag => [ant-id: true]
		antByMethod:                    make(map[common.TaskMethod]map[string]bool),       // method=> [ant-id: true]
		antsByRequest:                  make(map[string]map[string]bool),                  // request-id => [ant-id: true]
		allocationsByAnt:               make(map[string]map[string]*common.AntAllocation), // ant-id => [request-id: allocation]
		containersEvents:               make(map[string]*events.ContainerLifecycleEvent),  // method + container-name: container event
		containersEventKeysByRequestID: make(map[string]map[string]bool),                  // for removing containers by request
	}
}

func (s *State) getRegistrationByAnt(antID string) *common.AntRegistration {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.antRegistrations[antID] // ant-id => registration
}

func (s *State) reserve(
	requestID string,
	taskType string,
	method common.TaskMethod,
	tags []string,
	dryRun bool) (*common.AntReservation, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	methodAnts := s.antByMethod[method] // method=> [ant-id:true]

	if methodAnts == nil || len(methodAnts) == 0 {
		return nil, fmt.Errorf("no ants available for method '%s', "+
			"ants-by-methods=%d, total-registered-ants=%d",
			method, len(s.antByMethod), len(s.antRegistrations))
	}

	tagAnts := make(map[string]int)

	for _, outerTag := range tags {
		if strings.TrimSpace(outerTag) == "" {
			continue
		}

		antIDs := s.antByTag[outerTag] // tag => [ant-id:true]
		if antIDs == nil || len(antIDs) == 0 {
			return nil, fmt.Errorf("no ants available for tag '%s', "+
				"tags=%v, ants-by-tags=%d, total-registered-ants=%d",
				outerTag, tags, len(s.antByTag), len(s.antRegistrations))
		}

		for antID := range antIDs {
			tagAnts[antID] = 1
		}
	}

	// creating available ants to keep track of ants for both method and tags
	availableAnts := make(map[string]int)
	// The ants must both support method and tags
	for k := range methodAnts {
		if len(tags) == 0 {
			availableAnts[k] = 2
		} else {
			availableAnts[k] = 1 + tagAnts[k]
		}
	}

	// Select ant with the least workload
	reservations := make([]*common.AntReservation, 0)
	for antID, count := range availableAnts {
		if count != 2 { // 1 for method + 1 for tag
			continue
		}

		// Checking registration
		registration := s.antRegistrations[antID]
		allocationsByAnt := s.allocationsByAnt[registration.AntID] // ant-id => [request-id: allocation]
		if registration == nil || allocationsByAnt == nil {
			continue // shouldn't happen
		}

		// matching all tags for the ant
		// Note: we won't check capacity here as it's already checked in HasAntsForJobTags
		if registration.Supports(method, tags, s.serverCfg.Jobs.AntRegistrationAliveTimeout) {
			reservations = append(reservations,
				common.NewAntReservation(
					registration.AntID,
					registration.AntTopic,
					requestID,
					taskType,
					registration.EncryptionKey,
					calculateLoad(allocationsByAnt),
					registration.TotalExecuted,
				))
		}
	}

	// reservations must be available to continue
	if len(reservations) == 0 {
		return nil, fmt.Errorf("no ants could be reserved for method=%s, tags=%v available=%v "+
			"ants-by-methods=%d ants-by-tags=%d",
			method, tags, availableAnts, len(s.antByMethod), len(s.antByTag))
	}

	if !dryRun {
		// sorting based on least current load
		sort.Slice(reservations, func(i, j int) bool {
			return reservations[i].CurrentLoad < reservations[j].CurrentLoad
		})
	}
	// least load is first
	reservation := common.NewAntReservation(
		reservations[0].AntID,
		reservations[0].AntTopic,
		reservations[0].JobRequestID,
		reservations[0].TaskType,
		reservations[0].EncryptionKey,
		reservations[0].CurrentLoad,
		reservations[0].TotalExecuted,
	)
	reservation.TotalReservations = len(reservations)
	if !dryRun {
		s.addAllocationsByAnt(requestID, taskType, reservation)

		s.addAntsByRequest(requestID, reservation.AntID)

		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			logrus.WithFields(logrus.Fields{
				"Component":         "ResourceManager",
				"Load":              reservation.CurrentLoad,
				"AntID":             reservation.AntID,
				"RequestID":         requestID,
				"TaskType":          taskType,
				"Method":            method,
				"TotalReservations": reservation.TotalReservations,
				"Tags":              tags,
			}).Debug("reserving ant to task request")
		}
	}

	return reservation, nil
}

func calculateLoad(allocationsByAnt map[string]*common.AntAllocation) (load int) {
	for _, alloc := range allocationsByAnt {
		load += len(alloc.TaskTypes)
	}
	return
}

func (s *State) addAllocationsByAnt(
	requestID string,
	taskType string,
	reservation *common.AntReservation) {
	allocationsByAnt := s.allocationsByAnt[reservation.AntID] // ant-id => [request-id: allocation]
	allocation := allocationsByAnt[requestID]
	if allocation == nil {
		allocation = common.NewAntAllocation(
			reservation.AntID,
			reservation.AntTopic,
			requestID,
			taskType)
	}

	allocation.TaskTypes[taskType] = common.RESERVED
	allocationsByAnt[requestID] = allocation
	s.allocationsByAnt[reservation.AntID] = allocationsByAnt
	reservation.CurrentLoad = len(allocationsByAnt)
}

func (s *State) addAntsByRequest(requestID string, antID string) {
	requestAnts := s.antsByRequest[requestID]
	if requestAnts == nil {
		requestAnts = make(map[string]bool)
	}
	requestAnts[antID] = true
	s.antsByRequest[requestID] = requestAnts
}

func (s *State) releaseJob(requestID string) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	requestAnts := s.antsByRequest[requestID]
	if requestAnts == nil {
		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			logrus.WithFields(logrus.Fields{
				"RequestID": requestID,
			}).Debugf("release failed because ants not found for job request")
		}
		return nil
	}
	for antID := range requestAnts {
		allocationsByAnt := s.allocationsByAnt[antID] // ant-id => [request-id : allocation]
		if allocationsByAnt == nil {
			continue
		}
		allocationsByRequest := allocationsByAnt[requestID]
		if allocationsByRequest == nil {
			continue
		}
		delete(allocationsByAnt, requestID)
		s.allocationsByAnt[antID] = allocationsByAnt
	}

	// free all containers by request id
	delete(s.antsByRequest, requestID)
	keys := s.containersEventKeysByRequestID[requestID]
	if keys != nil {
		for k := range keys {
			delete(s.containersEvents, k)
		}
	}
	delete(s.containersEventKeysByRequestID, requestID)

	return nil
}

func (s *State) release(
	reservation *common.AntReservation) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	allocationsByAnt := s.allocationsByAnt[reservation.AntID] // ant-id => [request-id : allocation]
	if allocationsByAnt == nil {
		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			logrus.WithFields(logrus.Fields{
				"RequestID": reservation.JobRequestID,
				"AntID":     reservation.AntID,
				"TaskType":  reservation.TaskType,
			}).Debugf("release failed because no ants not found for task request")
		}
		return nil
	}

	allocationsByRequest := allocationsByAnt[reservation.JobRequestID]
	if allocationsByRequest == nil {
		if logrus.IsLevelEnabled(logrus.DebugLevel) {
			logrus.WithFields(logrus.Fields{
				"RequestID": reservation.JobRequestID,
				"AntID":     reservation.AntID,
				"TaskType":  reservation.TaskType,
			}).Debugf("release failed because ants no longer allocated")
		}
		return nil
	}

	delete(allocationsByRequest.TaskTypes, reservation.TaskType)

	if len(allocationsByRequest.TaskTypes) == 0 {
		delete(allocationsByAnt, reservation.JobRequestID)
		s.allocationsByAnt[reservation.AntID] = allocationsByAnt
		delete(s.antsByRequest, reservation.JobRequestID)
	}
	return nil
}

func (s *State) getAllocationsByAnt(
	antID string) map[string]*common.AntAllocation {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.allocationsByAnt[antID] // ant-id => [request-id: allocation]
}

func (s *State) hasAntsByMethod(method common.TaskMethod) (exists bool, total int) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	ants := s.antByMethod[method] // method=> [ant-id:true]
	total = len(ants)
	if ants == nil {
		exists = false
	} else {
		exists = len(ants) > 0
	}
	return
}

func (s *State) getAntsByTag(
	tag string) (antIDs []string, total int) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	antIDs = make([]string, 0)
	ants := s.antByTag[tag] // tag => [ant-id:true]
	total = len(ants)
	if ants != nil {
		for id := range ants {
			antIDs = append(antIDs, id)
		}
	}
	return
}

// getRegistrations returns all registered ants
func (s *State) getRegistrations() (regs []*common.AntRegistration) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	regs = make([]*common.AntRegistration, len(s.antRegistrations))
	i := 0
	for _, r := range s.antRegistrations {
		regs[i] = r
		i++
	}
	sort.Slice(regs, func(i, j int) bool { return regs[i].AntID < regs[j].AntID })

	return
}

// getRegistration for ant
func (s *State) getRegistration(
	id string) *common.AntRegistration {
	s.lock.RLock()
	defer s.lock.RUnlock()
	for _, r := range s.antRegistrations {
		if r.AntID == id {
			return r
		}
	}
	return nil
}

// countContainerEvents returns counts of events
func (s *State) countContainerEvents() (res map[common.TaskMethod]int) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	res = make(map[common.TaskMethod]int)
	for _, e := range s.containersEvents {
		res[e.Method] = res[e.Method] + 1
	}
	return
}

// getContainerEvents returns all events
func (s *State) getContainerEvents(
	sortBy string) (all []*events.ContainerLifecycleEvent) {
	s.lock.RLock()
	all = make([]*events.ContainerLifecycleEvent, len(s.containersEvents))
	i := 0
	for _, e := range s.containersEvents {
		all[i] = e
		i++
	}
	s.lock.RUnlock()
	sort.Slice(all, func(i, j int) bool {
		if sortBy == "time" {
			return all[i].CreatedAt.Unix() > all[j].CreatedAt.Unix()
		} else if sortBy == "key" {
			return all[i].Key() < all[j].Key()
		} else if sortBy == "elapsed" {
			return all[i].Elapsed() > all[j].Elapsed()
		}
		return all[i].ContainerName < all[j].ContainerName
	})
	return
}

// updateContainer
func (s *State) updateContainer(
	cnt *events.ContainerLifecycleEvent) {
	s.lock.Lock()
	defer s.lock.Unlock()
	keys := s.containersEventKeysByRequestID[cnt.RequestID()]
	if keys == nil {
		keys = make(map[string]bool)
	}
	if cnt.ContainerState.Done() {
		delete(s.containersEvents, cnt.Key())
		delete(keys, cnt.Key())
	} else {
		s.containersEvents[cnt.Key()] = cnt
		keys[cnt.Key()] = true
	}
	s.containersEventKeysByRequestID[cnt.RequestID()] = keys
}

// terminateContainer for remote ant
func (s *State) terminateContainer(
	ctx context.Context,
	id string,
	antID string,
	method common.TaskMethod) (err error) {
	registration := s.getRegistrationByAnt(antID)
	if registration == nil {
		return fmt.Errorf("failed to find ant with id %s", antID)
	}
	taskReq := &common.TaskRequest{
		JobExecutionID:  "TERMINATE_000",
		TaskExecutionID: "TERMINATE_000",
		JobType:         "DefaultResourceManager",
		TaskType:        "DefaultResourceManager",
		Action:          common.TERMINATE,
		ExecutorOpts:    common.NewExecutorOptions(id, method),
		StartedAt:       time.Now(),
	}
	var b []byte
	var taskResp *common.TaskResponse
	if b, err = taskReq.Marshal(registration.EncryptionKey); err == nil {
		req := &queue.SendReceiveRequest{
			OutTopic: registration.AntTopic,
			InTopic:  s.serverCfg.GetResponseTopicAntRegistration(),
			Payload:  b,
			Props:    make(map[string]string),
			Timeout:  taskTimeout,
		}
		if res, err := s.queueClient.SendReceive(ctx, req); err == nil {
			defer res.Ack() // auto-ack
			taskResp, err = common.UnmarshalTaskResponse(registration.EncryptionKey, res.Event.Payload)
			if err == nil && taskResp.Status.Failed() {
				err = fmt.Errorf("failed to terminate %s by %s due to %s", id, antID, taskResp.ErrorMessage)
			}
		} else {
			logrus.WithFields(logrus.Fields{
				"Component": "ResourceManager",
				"AntID":     registration.AntID,
				"OutTopic":  registration.AntTopic,
				"InTopic":   s.serverCfg.GetResponseTopicAntRegistration(),
			}).WithError(err).Errorf("failed to send request for termination")
		}
	} else {
		logrus.WithFields(logrus.Fields{
			"Component": "ResourceManager",
			"InTopic":   s.serverCfg.GetResponseTopicAntRegistration(),
			"OutTopic":  registration.AntTopic,
			"AntID":     registration.AntID,
			"Error":     err,
		}).Error("failed to terminate execution containers because message could not be sent")
	}
	return
}

// addContainers for adding containers after ant registration
func (s *State) addContainers(
	ctx context.Context,
	registration *common.AntRegistration) {
	taskReq := &common.TaskRequest{
		JobExecutionID:  "LIST_000",
		TaskExecutionID: "LIST_000",
		JobType:         "DefaultResourceManager",
		TaskType:        "DefaultResourceManager",
		Action:          common.LIST,
		ExecutorOpts:    common.NewExecutorOptions("", registration.Methods[0]),
		StartedAt:       time.Now(),
	}
	if b, err := taskReq.Marshal(registration.EncryptionKey); err == nil {
		req := &queue.SendReceiveRequest{
			OutTopic: registration.AntTopic,
			InTopic:  s.serverCfg.GetResponseTopicAntRegistration(),
			Payload:  b,
			Props:    make(map[string]string),
			Timeout:  taskTimeout,
		}
		if res, err := s.queueClient.SendReceive(ctx, req); err == nil {
			defer res.Ack() // auto-ack
			if res.Event == nil {
				logrus.WithFields(logrus.Fields{
					"Component": "ResourceManager",
					"AntID":     registration.AntID,
					"Topic":     registration.AntTopic,
				}).Errorf("received nil event from %s to %s",
					registration.AntTopic, s.serverCfg.GetResponseTopicAntRegistration())
				debug.PrintStack()
				return
			}
			taskResp, err := common.UnmarshalTaskResponse(registration.EncryptionKey, res.Event.Payload)
			if err == nil {
				jsonContainers := taskResp.TaskContext["containers"]
				containers := make([]*events.ContainerLifecycleEvent, 0)
				if jsonContainers != nil && reflect.TypeOf(jsonContainers).String() == "string" {
					err := json.Unmarshal([]byte(jsonContainers.(string)), &containers)
					if err == nil {
						for _, c := range containers {
							s.updateContainer(c)
						}
						logrus.WithFields(logrus.Fields{
							"Component":  "ResourceManager",
							"InTopic":    s.serverCfg.GetResponseTopicAntRegistration(),
							"OutTopic":   registration.AntTopic,
							"AntID":      registration.AntID,
							"Containers": len(containers),
						}).Info("added execution containers for ant registration")
					} else {
						logrus.WithFields(logrus.Fields{
							"Component": "ResourceManager",
							"InTopic":   s.serverCfg.GetResponseTopicAntRegistration(),
							"OutTopic":  registration.AntTopic,
							"AntID":     registration.AntID,
							"JSON":      jsonContainers,
							"Error":     err,
						}).Error("failed to add execution containers due to unmarshalling containers")
					}
				}
			} else {
				logrus.WithFields(logrus.Fields{
					"Component": "ResourceManager",
					"InTopic":   s.serverCfg.GetResponseTopicAntRegistration(),
					"OutTopic":  registration.AntTopic,
					"AntID":     registration.AntID,
					"Error":     err,
				}).Error("failed to add execution containers due to unmarshalling task-response")
			}
		} else {
			logrus.WithFields(logrus.Fields{
				"Component": "ResourceManager",
				"InTopic":   s.serverCfg.GetResponseTopicAntRegistration(),
				"OutTopic":  registration.AntTopic,
				"AntID":     registration.AntID,
				"Error":     err,
			}).Error("failed to add execution containers because message could not be sent")
		}
	} else {
		logrus.WithFields(logrus.Fields{
			"Component": "ResourceManager",
			"Topic":     registration.AntTopic,
			"AntID":     registration.AntID,
			"Error":     err,
		}).Error("failed to add execution containers due to marshalling error")
	}
}

// addRegistration for ant
func (s *State) addRegistration(
	ctx context.Context,
	registration *common.AntRegistration) {
	s.lock.Lock()
	defer s.lock.Unlock()
	// update mapping of ant-id => registration
	s.antRegistrations[registration.AntID] = registration

	oldAllocations := s.allocationsByAnt[registration.AntID]
	if oldAllocations == nil {
		oldAllocations = make(map[string]*common.AntAllocation)
		// for new ant registration, we will fetch running containers
		go s.addContainers(ctx, registration) // fetch containers in background
	}

	//ant-id => [request-id: allocation]
	for _, alloc := range registration.Allocations {
		oldAlloc := oldAllocations[alloc.JobRequestID]
		if oldAlloc == nil {
			oldAllocations[alloc.JobRequestID] = alloc
		} else {
			for task, state := range alloc.TaskTypes {
				oldAlloc.TaskTypes[task] = state
			}
			oldAlloc.UpdatedAt = alloc.UpdatedAt
		}
	}

	s.allocationsByAnt[registration.AntID] = oldAllocations

	s.updateAntsByMethods(registration)

	s.updateAntsByTags(registration)
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component":            "ResourceManager",
			"Topic":                registration.AntTopic,
			"AntID":                registration.AntID,
			"Tags":                 registration.Tags,
			"Methods":              registration.Methods,
			"TotalRegistrations":   len(s.antRegistrations),
			"TagsRegistrations":    len(s.antByTag),
			"MethodsRegistrations": len(s.antByMethod),
		}).Debug("registered ant, will add execution container")
	}
}

// removeRegistration for ant
func (s *State) removeRegistration(antID string) (removedTags []string, removedMethods []common.TaskMethod, count int) {
	s.lock.Lock()
	defer s.lock.Unlock()
	beforeCount := len(s.antRegistrations)
	delete(s.antRegistrations, antID)
	afterCount := len(s.antRegistrations)
	delete(s.allocationsByAnt, antID) // ant-id => [request-id: allocation]
	count = beforeCount - afterCount
	removedTags = make([]string, 0)
	removedMethods = make([]common.TaskMethod, 0)
	for tag, ants := range s.antByTag { // tag => [ant-id:true]
		if ants[antID] {
			delete(ants, antID)
			s.antByTag[tag] = ants
			removedTags = append(removedTags, tag)
		}
	}
	for method, ants := range s.antByMethod { // method=> [ant-id:true]
		if ants[antID] {
			delete(ants, antID)
			s.antByMethod[method] = ants
			removedMethods = append(removedMethods, method)
		}
	}
	for requestID, ants := range s.antsByRequest { // method=> [ant-id:true]
		if ants[antID] {
			delete(ants, antID)
			s.antsByRequest[requestID] = ants
		}
	}
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		logrus.WithFields(logrus.Fields{
			"Component":      "ResourceManager",
			"AntID":          antID,
			"Registrations":  s.antRegistrations,
			"Allocations":    s.allocationsByAnt,
			"AntsByTag":      s.antByTag,
			"AntsByMethods":  s.antByMethod,
			"AntsByRequests": s.antsByRequest,
		}).Debugf("removed ant registration and execution container")
	}
	return
}

func (s *State) updateAntsByMethods(registration *common.AntRegistration) {
	// update mapping of method => [ant-id:true]
	for _, method := range registration.Methods {
		antsForMethod := s.antByMethod[method]
		if antsForMethod == nil {
			antsForMethod = make(map[string]bool)
		}
		antsForMethod[registration.AntID] = true
		s.antByMethod[method] = antsForMethod
	}
}

func (s *State) updateAntsByTags(registration *common.AntRegistration) {
	// update mapping of tag => [ant-id:true]
	for _, tag := range registration.Tags {
		antsForTag := s.antByTag[tag]
		if antsForTag == nil {
			antsForTag = make(map[string]bool)
		}
		antsForTag[registration.AntID] = true
		s.antByTag[tag] = antsForTag
	}
}

func (s *State) reapStaleAllocations(timeout time.Duration) (removed []*common.AntReservation) {
	s.lock.Lock()
	defer s.lock.Unlock()
	now := time.Now()
	removed = make([]*common.AntReservation, 0)
	// ant-id => [task-key: allocation]
	for _, allocations := range s.allocationsByAnt {
		for requestID, allocation := range allocations {
			if time.Duration(now.Unix()-allocation.AllocatedAt.Unix())*time.Second > timeout {
				logrus.WithFields(logrus.Fields{
					"Component": "ResourceManager",
					"AntID":     allocation.AntID,
					"RequestID": requestID,
					"TaskTypes": allocation.TaskTypes,
					"Timeout":   timeout,
				}).Info("removing stale allocation of ant")
				for taskType := range allocation.TaskTypes {
					removed = append(removed, common.NewAntReservation(
						allocation.AntID,
						allocation.AntTopic,
						requestID,
						taskType,
						"",
						0,
						0,
					))
				}
			}
		}
	}

	// unlocking here before calling Release as it also uses locks
	return
}

func (s *State) dump(full bool) string {
	s.lock.RLock()
	defer s.lock.RUnlock()
	var buf strings.Builder
	if full {
		for k, v := range s.antRegistrations {
			buf.WriteString("{" + k + "=" + v.String() + "}")
		}
	}

	buf.WriteString("Tags: [")
	for k, v := range s.antByTag {
		buf.WriteString(fmt.Sprintf("%s=%d,", k, len(v)))
	}
	buf.WriteString("] Methods: [")
	for k, v := range s.antByMethod {
		buf.WriteString(fmt.Sprintf("%s=%d,", k, len(v)))
	}
	buf.WriteString("]")
	return buf.String()
}
