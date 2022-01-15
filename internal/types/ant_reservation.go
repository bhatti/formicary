package types

import (
	"fmt"
	"runtime/debug"
	"time"
)

// AntReservation is used for keeping track of reservation
type AntReservation struct {
	JobRequestID  uint64    `json:"job_request_id" mapstructure:"job_request_id"`
	TaskType      string    `json:"task_type" mapstructure:"task_type"`
	AntID         string    `json:"ant_id" mapstructure:"ant_id"`
	AntTopic      string    `json:"ant_topic" mapstructure:"ant_topic"`
	EncryptionKey string    `json:"encryption_key" mapstructure:"encryption_key"`
	AllocatedAt   time.Time `json:"allocated_at" mapstructure:"allocated_at"`
	CurrentLoad   int       `json:"current_load" mapstructure:"current_load"`
	// TotalReservations is set by resource-manager when reserving resources to show total reservations
	TotalReservations int `json:"total_reservations" mapstructure:"total_reservations"`
}

// NewAntReservation constructor
func NewAntReservation(
	antID string,
	antTopic string,
	requestID uint64,
	taskType string,
	encryptionKey string,
	currentLoad int) *AntReservation {
	return &AntReservation{
		JobRequestID:  requestID,
		TaskType:      taskType,
		AntID:         antID,
		AntTopic:      antTopic,
		EncryptionKey: encryptionKey,
		AllocatedAt:   time.Now(),
		CurrentLoad:   currentLoad,
	}
}

// Validate validates
func (r *AntReservation) Validate() error {
	if r.AntID == "" {
		debug.PrintStack()
		return fmt.Errorf("antID is not specified for ant reservation")
	}
	if r.AntTopic == "" {
		return fmt.Errorf("antTopic is not specified for ant reservation")
	}
	return nil
}

// String defines description of reservation
func (r *AntReservation) String() string {
	return fmt.Sprintf("AntID=%s Topic=%s RequestID=%d TaskType=%v Load=%d",
		r.AntID, r.AntTopic, r.JobRequestID, r.TaskType, r.CurrentLoad)
}

// AllocatedAtString formatted
func (r *AntReservation) AllocatedAtString() string {
	return r.AllocatedAt.Format("Jan _2, 15:04:05 MST")
}
