package types

import "time"

// DateRange date range
type DateRange struct {
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
}

// StartAndEndString string
func (r DateRange) StartAndEndString() string {
	if r.StartDate.Month() == r.EndDate.Month() {
		return r.StartDate.Format("Jan _2") + " - " + r.EndDate.Format("_2")
	}
	return r.StartDate.Format("Jan _2") + " - " + r.EndDate.Format("Jan _2")
}

// StartString string
func (r DateRange) StartString() string {
	return r.StartDate.Format("Jan _2")
}
