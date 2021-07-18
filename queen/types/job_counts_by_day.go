package types

// JobCountsByDay defines counts on job types by day
type JobCountsByDay struct {
	// SucceededCounts defines total number of records succeeded
	SucceededCounts int64 `json:"succeeded_counts"`
	// FailedCounts defines total number of records succeeded
	FailedCounts int64 `json:"failed_counts"`
	// Date for unixepoch
	Day string `json:"day"`
}
