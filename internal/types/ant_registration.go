package types

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"plexobject.com/formicary/internal/utils"
)

// RegistrationTopic topic
const RegistrationTopic = "ant-registration"

// ValidRegistration - for validating
type ValidRegistration func(
	ctx context.Context,
) error

// AntRegistration is used to register remote ants with the resource manager so that tasks can be routed to them based on their capacity.
type AntRegistration struct {
	AntID         string                    `json:"ant_id" mapstructure:"ant_id"`
	AntTopic      string                    `json:"ant_topic" mapstructure:"ant_topic"`
	EncryptionKey string                    `json:"encryption_key" mapstructure:"encryption_key"`
	MaxCapacity   int                       `json:"max_capacity" mapstructure:"max_capacity"`
	Tags          []string                  `json:"tags" mapstructure:"tags"`
	Methods       []TaskMethod              `json:"methods" mapstructure:"methods"`
	CurrentLoad   int                       `json:"current_load" mapstructure:"current_load"`
	TotalExecuted int                       `json:"total_executed" mapstructure:"total_executed"`
	Allocations   map[uint64]*AntAllocation `json:"allocations" mapstructure:"allocations"`
	CreatedAt     time.Time                 `json:"created_at" mapstructure:"created_at"`
	AntStartedAt  time.Time                 `json:"ant_started_at" mapstructure:"ant_started_at"`
	AutoRefresh   bool                      `json:"auto_refresh" mapstructure:"auto_refresh"`
	// Transient property
	ReceivedAt        time.Time         `json:"-" mapstructure:"-"`
	ValidRegistration ValidRegistration `json:"-" mapstructure:"-"`
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
func (r *AntRegistration) Validate() error {
	if r.AntID == "" {
		return fmt.Errorf("antID is not specified for registration")
	}
	if r.Methods == nil || len(r.Methods) == 0 {
		return fmt.Errorf("methods is not specified for registration")
	}
	//if r.AntTopic == "" {
	//	return fmt.Errorf("antTopic is not specified")
	//}
	if r.MaxCapacity <= 0 {
		r.MaxCapacity = 1
	}
	return nil
}

// Key returns unique key
func (r *AntRegistration) Key() string {
	var key strings.Builder
	key.WriteString(r.AntID)
	for _, t := range r.Tags {
		key.WriteString(t + ":")
	}
	for _, m := range r.Methods {
		key.WriteString(string(m) + ":")
	}
	return key.String()
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
func (r *AntRegistration) Marshal() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(r)
}

// String defines description of registration
func (r *AntRegistration) String() string {
	return fmt.Sprintf("ID=%s Tags=%s Max=%d Load=%d Executed=%d\n",
		r.AntID, r.Tags, r.MaxCapacity, r.CurrentLoad, r.TotalExecuted)
}

// UpdatedAtString defines formatted date
func (r *AntRegistration) UpdatedAtString() string {
	return r.ReceivedAt.Format("Jan _2, 15:04:05 MST")
}

// Supports check supported method and tags
func (r *AntRegistration) Supports(
	method TaskMethod,
	tags []string,
	timeout time.Duration) bool {
	if time.Duration(time.Now().Unix()-r.ReceivedAt.Unix())*time.Second > timeout {
		return false
	}
	if r.ValidRegistration != nil {
		if err := r.ValidRegistration(context.Background()); err != nil {
			return false
		}
	}
	matchedMethod := false
	for _, m := range r.Methods {
		if m == method {
			matchedMethod = true
			break
		}
	}
	if !matchedMethod {
		return false
	}
	return utils.MatchTagsArray(r.Tags, tags) == nil
}
