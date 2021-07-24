package types

import (
	"fmt"
	"time"
)

// ResourceUsageType type of usage
type ResourceUsageType string

const (
	// DiskResource disk resource
	DiskResource ResourceUsageType = "DISK"
	// CPUResource CPU resource
	CPUResource ResourceUsageType = "CPU"
)

// ResourceUsage defines use of a resource
type ResourceUsage struct {
	StartDate      time.Time         `json:"start_date"`
	EndDate        time.Time         `json:"end_date"`
	ResourceType   ResourceUsageType `json:"resource_type"`
	OrganizationID string            `json:"organization_id"`
	Count          int               `json:"count"`
	UserID         string            `json:"user_id"`
	Value          int64             `json:"value"`
	ValueUnit      string            `json:"value_unit"`
	RemainingQuota int64             `json:"remaining_quota"`
}

func (r ResourceUsage) String() string {
	return fmt.Sprintf("%s-%s %d %s",
		r.StartDate.Format("Jan _2"),
		r.EndDate.Format("Jan _2"),
		r.Value,
		r.ValueUnit)
}

// DateString string
func (r ResourceUsage) DateString() string {
	return fmt.Sprintf("%s %s",
		r.StartDate.Format("Jan-_2"),
		r.EndDate.Format("Jan-_2"),
	)
}

// KValue / 1024
func (r ResourceUsage) KValue() int64 {
	if r.ValueUnit == "bytes" {
		return r.Value / 1024
	} else {
		return r.Value
	}
}

// MValue / 1024 * 1024
func (r ResourceUsage) MValue() int64 {
	if r.ValueUnit == "bytes" {
		return r.Value / 1024 / 1024
	} else {
		return r.Value
	}
}

// ValueString string
func (r ResourceUsage) ValueString() string {
	if r.ValueUnit == "bytes" {
		if r.Value > 1024*1024 {
			return fmt.Sprintf("%d MiB",
				r.Value/1024/1024)
		} else if r.Value > 1024 {
			return fmt.Sprintf("%d KiB",
				r.Value/1024)
		} else {
			return fmt.Sprintf("%d B",
				r.Value)
		}
	} else if r.ValueUnit == "seconds" && r.Value > 3600 {
		return fmt.Sprintf("%0.2f Hours",
			float64(r.Value)/3600.0)
	} else if r.ValueUnit == "seconds" && r.Value > 60 {
		return fmt.Sprintf("%0.2f Minutes ",
			float64(r.Value)/60.0)
	} else {
		return fmt.Sprintf("%d-%s",
			r.Value,
			r.ValueUnit)
	}
}
