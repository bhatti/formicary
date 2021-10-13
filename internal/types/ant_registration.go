package types

import (
	"encoding/json"
	"fmt"
	"time"

	"plexobject.com/formicary/internal/utils"
)

// RegistrationTopic topic
const RegistrationTopic = "ant-registration"

// AntRegistration is used to register remote ants with the resource manager so that tasks can be routed to them based on their capacity.
type AntRegistration struct {
	AntID         string                    `json:"ant_id" mapstructure:"ant_id"`
	AntTopic      string                    `json:"ant_topic" mapstructure:"ant_topic"`
	EncryptionKey string                    `json:"encryption_key" mapstructure:"encryption_key"`
	MaxCapacity   int                       `json:"max_capacity" mapstructure:"max_capacity"`
	Tags          []string                  `json:"tags" mapstructure:"tags"`
	Methods       []TaskMethod              `json:"methods" mapstructure:"methods"`
	CurrentLoad   int                       `json:"current_load" mapstructure:"current_load"`
	Allocations   map[uint64]*AntAllocation `json:"allocations" mapstructure:"allocations"`
	CreatedAt     time.Time                 `json:"created_at" mapstructure:"created_at"`
	AntStartedAt  time.Time                 `json:"ant_started_at" mapstructure:"ant_started_at"`
	// Transient property
	ReceivedAt time.Time `json:"-" mapstructure:"-"`
}

// AntAllocation is used for keeping track of allocation capacity of the ant worker so that resource manager can throttle
// tasks that are sent to the ant follower.
type AntAllocation struct {
	JobRequestID uint64                  `json:"job_request_id" mapstructure:"job_request_id"`
	TaskTypes    map[string]RequestState // [task-type:state]
	AntID        string                  `json:"ant_id" mapstructure:"ant_id"`
	AntTopic     string                  `json:"ant_topic" mapstructure:"ant_topic"`
	AllocatedAt  time.Time               `json:"allocated_at" mapstructure:"allocated_at"`
	UpdatedAt    time.Time               `json:"updated_at" mapstructure:"updated_at"`
}

// NewAntAllocation constructor
func NewAntAllocation(
	antID string,
	antTopic string,
	requestID uint64,
	taskType string) *AntAllocation {
	return &AntAllocation{
		JobRequestID: requestID,
		TaskTypes:    map[string]RequestState{taskType: EXECUTING},
		AntID:        antID,
		AntTopic:     antTopic,
		AllocatedAt:  time.Now(),
		UpdatedAt:    time.Now(),
	}
}

// String defines description of allocation
func (wa *AntAllocation) String() string {
	return fmt.Sprintf("AntID=%s Topic=%s RequestID=%d TaskType=%v",
		wa.AntID, wa.AntTopic, wa.JobRequestID, wa.TaskTypes)
}

// Load returns tasks using this allocation
func (wa *AntAllocation) Load() int {
	return len(wa.TaskTypes)
}

// AllocatedAtString formatted
func (wa *AntAllocation) AllocatedAtString() string {
	return wa.AllocatedAt.Format("Jan _2, 15:04:05 MST")
}

// Validate validates
func (wr *AntRegistration) Validate() error {
	if wr.AntID == "" {
		return fmt.Errorf("antID is not specified")
	}
	if wr.Methods == nil || len(wr.Methods) == 0 {
		return fmt.Errorf("methods is not specified")
	}
	if wr.AntTopic == "" {
		return fmt.Errorf("antTopic is not specified")
	}
	if wr.MaxCapacity <= 0 {
		return fmt.Errorf("maxCapacity is not specified")
	}
	return nil
}

// UnmarshalAntRegistration unmarshal
func UnmarshalAntRegistration(b []byte) (*AntRegistration, error) {
	var registration AntRegistration
	if err := json.Unmarshal(b, &registration); err != nil {
		return nil, err
	}
	if err := registration.Validate(); err != nil {
		return nil, err
	}
	return &registration, nil
}

// Marshal marshals
func (wr *AntRegistration) Marshal() ([]byte, error) {
	if err := wr.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(wr)
}

// String defines description of registration
func (wr *AntRegistration) String() string {
	return fmt.Sprintf("ID=%s Tags=%s Max=%d Load=%d\n",
		wr.AntID, wr.Tags, wr.MaxCapacity, wr.CurrentLoad)
}

// UpdatedAtString defines formatted date
func (wr *AntRegistration) UpdatedAtString() string {
	return wr.ReceivedAt.Format("Jan _2, 15:04:05 MST")
}

// Supports check supported method and tags
func (wr *AntRegistration) Supports(
	method TaskMethod,
	tags []string) bool {
	matchedMethod := false
	for _, m := range wr.Methods {
		if m == method {
			matchedMethod = true
			break
		}
	}
	if !matchedMethod {
		return false
	}
	return utils.MatchTagsArray(wr.Tags, tags) == nil
}
