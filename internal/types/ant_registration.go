package types

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
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
	Allocations   map[string]*AntAllocation `json:"allocations" mapstructure:"allocations"`
	CreatedAt     time.Time                 `json:"created_at" mapstructure:"created_at"`
	AntStartedAt  time.Time                 `json:"ant_started_at" mapstructure:"ant_started_at"`
	AutoRefresh   bool                      `json:"auto_refresh" mapstructure:"auto_refresh"`
	ConfigInfo    map[string]any            `json:"config_info" mapstructure:"config_info"`
	// OrgID is set server-side from the ant's JWT org_id claim when auth is enabled.
	// Empty when auth is disabled; org filtering is skipped in that case (no-op).
	OrgID string `json:"org_id,omitempty" mapstructure:"org_id"`
	// MethodHealth maps each supported method name (e.g. "KUBERNETES", "DOCKER") to its
	// most-recent health probe result. Updated by the ant's background health checker and
	// included in every heartbeat so the queen can route per-method rather than blocking
	// the whole ant. Absent entries mean the method has not been probed yet (treated as healthy).
	// Access must be guarded by healthMu — use SetMethodHealth/methodHealthSnapshot.
	MethodHealth map[string]*MethodHealthEntry `json:"method_health,omitempty" mapstructure:"method_health"`
	// healthMu guards MethodHealth: the health-checker goroutine writes it while the
	// tasklet heartbeat goroutine reads it via Marshal(). Not serialized (json:"-").
	healthMu sync.RWMutex `json:"-" mapstructure:"-"`
	// Transient property
	ReceivedAt        time.Time         `json:"-" mapstructure:"-"`
	ValidRegistration ValidRegistration `json:"-" mapstructure:"-"`
}

// MethodHealthEntry records the outcome of a single method-level health probe.
type MethodHealthEntry struct {
	// Healthy is true when the probe succeeded.
	Healthy bool `json:"healthy" mapstructure:"healthy"`
	// Error is empty when healthy; human-readable reason when unhealthy.
	Error string `json:"error,omitempty" mapstructure:"error"`
	// LastCheckedAt is the wall-clock time of the most-recent probe.
	LastCheckedAt time.Time `json:"last_checked_at" mapstructure:"last_checked_at"`
}

// AntAllocation is used for keeping track of allocation capacity of the ant worker so that resource manager can throttle
// tasks that are sent to the ant follower.
type AntAllocation struct {
	JobRequestID string                  `json:"job_request_id" mapstructure:"job_request_id"`
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
	requestID string,
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
	return fmt.Sprintf("AntID=%s Topic=%s RequestID=%s TaskType=%v",
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

// SetMethodHealth safely updates the health entry for one method from the health-checker
// goroutine. It is the only correct way to mutate MethodHealth after construction.
func (r *AntRegistration) SetMethodHealth(method string, entry *MethodHealthEntry) {
	r.healthMu.Lock()
	defer r.healthMu.Unlock()
	if r.MethodHealth == nil {
		r.MethodHealth = make(map[string]*MethodHealthEntry)
	}
	r.MethodHealth[method] = entry
}

// MethodHealthSnapshot returns a shallow copy of MethodHealth taken under a read lock.
// Use this whenever iterating health entries outside the ant registration's own goroutine.
func (r *AntRegistration) MethodHealthSnapshot() map[string]*MethodHealthEntry {
	return r.methodHealthSnapshot()
}

// methodHealthSnapshot returns a shallow copy of MethodHealth taken under a read lock.
// Callers that need to iterate health entries safely (e.g. Marshal, String) must use this.
func (r *AntRegistration) methodHealthSnapshot() map[string]*MethodHealthEntry {
	r.healthMu.RLock()
	defer r.healthMu.RUnlock()
	if len(r.MethodHealth) == 0 {
		return nil
	}
	cp := make(map[string]*MethodHealthEntry, len(r.MethodHealth))
	for k, v := range r.MethodHealth {
		cp[k] = v
	}
	return cp
}

// antRegistrationWire is a shadow type used by Marshal to avoid encoding the unexported
// healthMu field and to allow snapshotting MethodHealth under a read lock before encoding.
type antRegistrationWire struct {
	AntID         string                    `json:"ant_id"`
	AntTopic      string                    `json:"ant_topic"`
	EncryptionKey string                    `json:"encryption_key"`
	MaxCapacity   int                       `json:"max_capacity"`
	Tags          []string                  `json:"tags"`
	Methods       []TaskMethod              `json:"methods"`
	CurrentLoad   int                       `json:"current_load"`
	TotalExecuted int                       `json:"total_executed"`
	Allocations   map[string]*AntAllocation `json:"allocations"`
	CreatedAt     time.Time                 `json:"created_at"`
	AntStartedAt  time.Time                 `json:"ant_started_at"`
	AutoRefresh   bool                      `json:"auto_refresh"`
	ConfigInfo    map[string]any            `json:"config_info"`
	OrgID         string                    `json:"org_id,omitempty"`
	MethodHealth  map[string]*MethodHealthEntry `json:"method_health,omitempty"`
}

// Marshal marshals the registration. MethodHealth is snapshotted under a read lock so
// the encoding is safe to run concurrently with the health-checker goroutine.
func (r *AntRegistration) Marshal() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	wire := &antRegistrationWire{
		AntID:         r.AntID,
		AntTopic:      r.AntTopic,
		EncryptionKey: r.EncryptionKey,
		MaxCapacity:   r.MaxCapacity,
		Tags:          r.Tags,
		Methods:       r.Methods,
		CurrentLoad:   r.CurrentLoad,
		TotalExecuted: r.TotalExecuted,
		Allocations:   r.Allocations,
		CreatedAt:     r.CreatedAt,
		AntStartedAt:  r.AntStartedAt,
		AutoRefresh:   r.AutoRefresh,
		ConfigInfo:    r.ConfigInfo,
		OrgID:         r.OrgID,
		MethodHealth:  r.methodHealthSnapshot(),
	}
	return json.Marshal(wire)
}

// String defines description of registration
func (r *AntRegistration) String() string {
	var health strings.Builder
	for m, h := range r.methodHealthSnapshot() {
		if health.Len() > 0 {
			health.WriteByte(' ')
		}
		if h.Healthy {
			fmt.Fprintf(&health, "%s=OK", m)
		} else {
			fmt.Fprintf(&health, "%s=UNHEALTHY(%s)", m, h.Error)
		}
	}
	return fmt.Sprintf("ID=%s OrgID=%s Tags=%s Methods=%v Max=%d Load=%d Executed=%d Health=[%s]\n",
		r.AntID, r.OrgID, r.Tags, r.Methods, r.MaxCapacity, r.CurrentLoad, r.TotalExecuted, health.String())
}

// UpdatedAtString defines formatted date
func (r *AntRegistration) UpdatedAtString() string {
	return r.ReceivedAt.Format("Jan _2, 15:04:05 MST")
}

// LoadPercent returns the current load as an integer percentage of max capacity (0–100).
// Returns 0 when capacity is zero to avoid division by zero.
func (r *AntRegistration) LoadPercent() int {
	if r.MaxCapacity <= 0 {
		return 0
	}
	pct := r.CurrentLoad * 100 / r.MaxCapacity
	if pct > 100 {
		return 100
	}
	return pct
}

// AvailableCapacity returns how many more jobs this ant can accept right now.
func (r *AntRegistration) AvailableCapacity() int {
	avail := r.MaxCapacity - r.CurrentLoad
	if avail < 0 {
		return 0
	}
	return avail
}

// HasExternalMethods returns true when this ant handles at least one non-internal method.
// Internal ants (ExpireArtifacts, ForkJob, AwaitForkedJob, FanOutJob, Messaging, Manual)
// are built into the queen; only external ants need a Logs view or count toward the
// "no external ant" dashboard warning.
func (r *AntRegistration) HasExternalMethods() bool {
	for _, m := range r.Methods {
		if !m.IsInternal() {
			return true
		}
	}
	return false
}

// IsAlive returns true if the registration heartbeat is within the alive timeout window.
func (r *AntRegistration) IsAlive(timeout time.Duration) bool {
	if time.Duration(time.Now().Unix()-r.ReceivedAt.Unix())*time.Second > timeout {
		return false
	}
	if r.ValidRegistration != nil {
		if err := r.ValidRegistration(context.Background()); err != nil {
			return false
		}
	}
	return true
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
	// Per-method health gate: skip this ant ONLY for the requested method when its
	// backend is unhealthy. Other methods on the same ant remain available.
	r.healthMu.RLock()
	entry, exists := r.MethodHealth[string(method)]
	r.healthMu.RUnlock()
	if exists && !entry.Healthy {
		return false
	}
	return utils.MatchTagsArray(r.Tags, tags) == nil
}
