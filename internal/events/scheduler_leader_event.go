package events

// JobSchedulerLeaderEvent is used to fail over to another server if leader dies
type JobSchedulerLeaderEvent struct {
	BaseEvent
}
