package events

import (
	"encoding/json"
	"fmt"
	"github.com/twinj/uuid"
	"time"
)

// RecentlyCompletedJobsEvent for notifying job ids of completed jobs
type RecentlyCompletedJobsEvent struct {
	BaseEvent
	JobIDs []uint64 `json:"job_ids"`
}

// NewRecentlyCompletedJobsEvent constructor
func NewRecentlyCompletedJobsEvent(
	source string,
	jobIDs []uint64,
) *RecentlyCompletedJobsEvent {
	return &RecentlyCompletedJobsEvent{
		BaseEvent: BaseEvent{
			ID:        uuid.NewV4().String(),
			Source:    source,
			EventType: "RecentlyCompletedJobsEvent",
			CreatedAt: time.Now(),
		},
		JobIDs: jobIDs,
	}
}

// String format
func (l *RecentlyCompletedJobsEvent) String() string {
	return fmt.Sprintf("%v", l.JobIDs)
}

// UnmarshalRecentlyCompletedJobsEvent unmarshal
func UnmarshalRecentlyCompletedJobsEvent(b []byte) (*RecentlyCompletedJobsEvent, error) {
	var event RecentlyCompletedJobsEvent
	if err := json.Unmarshal(b, &event); err != nil {
		return nil, err
	}
	return &event, nil
}

// Marshal serializes event
func (l *RecentlyCompletedJobsEvent) Marshal() ([]byte, error) {
	return json.Marshal(l)
}
