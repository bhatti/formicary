package types

import (
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func Test_ShouldVerifyResourceUsageString(t *testing.T) {
	resUsage := ResourceUsage{
		StartDate:      time.Now(),
		EndDate:        time.Now().Add(time.Hour),
		ResourceType:   DiskResource,
		UserID:         "123",
		OrganizationID: "456",
		Count:          1,
		Value:          100,
		ValueUnit:      "bytes",
	}
	require.Contains(t, resUsage.String(), "100 bytes")
}

func Test_ShouldVerifyResourceUsageDateString(t *testing.T) {
	resUsage := ResourceUsage{
		StartDate:      time.Now(),
		EndDate:        time.Now().Add(time.Hour),
		ResourceType:   DiskResource,
		UserID:         "123",
		OrganizationID: "456",
		Count:          1,
		Value:          100,
		ValueUnit:      "bytes",
	}
	require.NotEqual(t, "", resUsage.DateString())
}

func Test_ShouldVerifyResourceUsageKValue(t *testing.T) {
	resUsage := ResourceUsage{
		StartDate:      time.Now(),
		EndDate:        time.Now().Add(time.Hour),
		ResourceType:   DiskResource,
		UserID:         "123",
		OrganizationID: "456",
		Count:          1,
		Value:          1000000,
		ValueUnit:      "bytes",
	}
	require.NotEqual(t, "", resUsage.KValue())
	require.NotEqual(t, "", resUsage.MValue())
}

func Test_ShouldVerifyResourceValueKString(t *testing.T) {
	resUsage := ResourceUsage{
		StartDate:      time.Now(),
		EndDate:        time.Now().Add(time.Hour),
		ResourceType:   DiskResource,
		UserID:         "123",
		OrganizationID: "456",
		Count:          1,
		Value:          1000000,
		ValueUnit:      "bytes",
	}
	require.Equal(t, "976 KiB", resUsage.ValueString())
	resUsage.Value *= resUsage.Value
	require.Equal(t, "953674 MiB", resUsage.ValueString())
	resUsage.Value = 100
	require.Equal(t, "100 B", resUsage.ValueString())
}

func Test_ShouldVerifyResourceValueSecsString(t *testing.T) {
	resUsage := ResourceUsage{
		StartDate:      time.Now(),
		EndDate:        time.Now().Add(time.Hour),
		ResourceType:   CPUResource,
		UserID:         "123",
		OrganizationID: "456",
		Count:          1,
		Value:          1000000,
		ValueUnit:      "seconds",
	}
	require.Equal(t, "277.78 Hours", resUsage.ValueString())
	resUsage.Value *= resUsage.Value
	require.Equal(t, "277777777.78 Hours", resUsage.ValueString())
	resUsage.Value = 100
	require.Equal(t, "1.67 Minutes", resUsage.ValueString())
	resUsage.ValueUnit = "blah"
	require.Equal(t, "100 blah", resUsage.ValueString())
}
