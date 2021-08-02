package events

import (
	"context"
	"encoding/json"
	"fmt"
	"plexobject.com/formicary/ants/executor"
	"strconv"
	"time"

	"github.com/twinj/uuid"

	"plexobject.com/formicary/internal/types"
)

// ContainerLifecycleEvent is a lifecycle event used to update state of the containers.
type ContainerLifecycleEvent struct {
	BaseEvent
	UserID string `json:"user_id"`
	AntID  string `json:"ant_id"`
	// Method of container
	Method types.TaskMethod `json:"method"`
	// ContainerName
	ContainerName string `json:"container_name"`
	// ContainerID
	ContainerID string `json:"container_id"`
	// ContainerState
	ContainerState types.RequestState `json:"container_state"`
	// Labels
	Labels map[string]string `json:"labels"`
	// StartedAt
	StartedAt time.Time `json:"started_at" mapstructure:"started_at"`
	// EndedAt
	EndedAt *time.Time `json:"ended_at" mapstructure:"ended_at"`
}

// GetID - container id for implementing executor.Info
func (cle *ContainerLifecycleEvent) GetID() string {
	return cle.ContainerID
}

// GetName - container name for implementing executor.Info
func (cle *ContainerLifecycleEvent) GetName() string {
	return cle.ContainerName
}

// GetState - state for implementing executor.Info
func (cle *ContainerLifecycleEvent) GetState() executor.State {
	if cle.ContainerState.Done() {
		return executor.Removing
	}
	return executor.Running
}

// GetStartedAt - time for implementing executor.Info
func (cle *ContainerLifecycleEvent) GetStartedAt() time.Time {
	return cle.StartedAt
}

// GetEndedAt - time for implementing executor.Info
func (cle *ContainerLifecycleEvent) GetEndedAt() *time.Time {
	return cle.EndedAt
}

// GetLabels - container labels for implementing executor.Info
func (cle *ContainerLifecycleEvent) GetLabels() map[string]string {
	return cle.Labels
}

// GetRuntimeInfo for implementing executor.Info
func (cle *ContainerLifecycleEvent) GetRuntimeInfo(_ context.Context) string {
	return ""
}

// NewContainerLifecycleEvent constructor
func NewContainerLifecycleEvent(
	source string,
	userID string,
	antID string,
	method types.TaskMethod,
	containerName string,
	containerID string,
	containerState types.RequestState,
	labels map[string]string,
	startedAt time.Time,
	endedAt *time.Time) *ContainerLifecycleEvent {
	return &ContainerLifecycleEvent{
		BaseEvent: BaseEvent{
			ID:        uuid.NewV4().String(),
			Source:    source,
			EventType: "ContainerLifecycleEvent",
			CreatedAt: time.Now(),
		},
		UserID:         userID,
		AntID:          antID,
		Method:         method,
		ContainerName:  containerName,
		ContainerID:    containerID,
		ContainerState: containerState,
		Labels:         labels,
		StartedAt:      startedAt,
		EndedAt:        endedAt,
	}
}

// String format
func (cle *ContainerLifecycleEvent) String() string {
	return fmt.Sprintf("AntID=%s ContainerName=%s ContainerState=%s Method=%s",
		cle.AntID, cle.ContainerName, cle.ContainerState, cle.Method)
}

// Key key for the event
func (cle *ContainerLifecycleEvent) Key() string {
	return ContainerLifecycleEventKey(cle.Method, cle.ContainerName)
}

// ContainerLifecycleEventKey key for the event
func ContainerLifecycleEventKey(method types.TaskMethod, containerName string) string {
	return fmt.Sprintf("%s:%s", method, containerName)
}

// UnmarshalContainerLifecycleEvent unmarshal
func UnmarshalContainerLifecycleEvent(b []byte) (*ContainerLifecycleEvent, error) {
	var event ContainerLifecycleEvent
	if err := json.Unmarshal(b, &event); err != nil {
		return nil, err
	}
	if err := event.Validate(); err != nil {
		return nil, err
	}
	return &event, nil
}

// Validate validates event for lifecycle job
func (cle *ContainerLifecycleEvent) Validate() error {
	if cle.AntID == "" {
		return fmt.Errorf("ant id is not specified")
	}
	if cle.Method == "" {
		return fmt.Errorf("method is not specified")
	}
	if cle.ContainerName == "" {
		return fmt.Errorf("container name is not specified")
	}
	if cle.ContainerID == "" {
		return fmt.Errorf("container id is not specified")
	}
	return nil
}

// Marshal serializes event
func (cle *ContainerLifecycleEvent) Marshal() ([]byte, error) {
	if err := cle.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(cle)
}

// ElapsedDuration time duration of container
func (cle *ContainerLifecycleEvent) ElapsedDuration() string {
	if cle.EndedAt == nil || cle.ContainerState == types.EXECUTING {
		return time.Now().Sub(cle.StartedAt).String()
	}
	return cle.EndedAt.Sub(cle.StartedAt).String()
}

// Elapsed unix time elapsed of container
func (cle *ContainerLifecycleEvent) Elapsed() int64 {
	if cle.EndedAt == nil || cle.ContainerState == types.EXECUTING {
		return time.Now().Sub(cle.StartedAt).Milliseconds()
	}
	return cle.EndedAt.Sub(cle.StartedAt).Milliseconds()
}

// ElapsedSecs returns time since executor started in secs
func (cle *ContainerLifecycleEvent) ElapsedSecs() time.Duration {
	if cle.EndedAt == nil || cle.ContainerState == types.EXECUTING {
		return time.Duration(time.Now().Sub(cle.StartedAt).Seconds()) * time.Second
	}
	return time.Duration(cle.EndedAt.Sub(cle.StartedAt).Seconds()) * time.Second
}

// StartedAtString formatted date
func (cle *ContainerLifecycleEvent) StartedAtString() string {
	return cle.StartedAt.Format("Jan _2, 15:04:05 MST")
}

// RequestID from labels
func (cle *ContainerLifecycleEvent) RequestID() uint64 {
	strID := cle.Labels["RequestID"]
	if strID == "" {
		return 0
	}
	id, _ := strconv.ParseUint(strID, 10, 64)
	return id
}
