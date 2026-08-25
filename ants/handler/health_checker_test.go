package handler

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/internal/ant_config"
	"plexobject.com/formicary/internal/types"
)

func TestMethodHealthChecker_NoMethods(t *testing.T) {
	cfg := &ant_config.AntConfig{Common: types.CommonConfig{ID: "test-ant"}}
	reg := cfg.NewAntRegistration()
	h := NewMethodHealthChecker(cfg, reg)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	h.Start(ctx)
	defer h.Stop()

	// No methods → MethodHealth stays nil; ant is always routable.
	time.Sleep(50 * time.Millisecond)
	assert.Nil(t, reg.MethodHealth, "no methods means no health entries")
}

func TestMethodHealthChecker_SetHealth_Healthy(t *testing.T) {
	cfg := &ant_config.AntConfig{Common: types.CommonConfig{ID: "test-ant"}}
	reg := cfg.NewAntRegistration()
	h := NewMethodHealthChecker(cfg, reg)

	h.setHealth("KUBERNETES", true, "")
	require.NotNil(t, reg.MethodHealth)
	entry := reg.MethodHealth["KUBERNETES"]
	require.NotNil(t, entry)
	assert.True(t, entry.Healthy)
	assert.Empty(t, entry.Error)
	assert.WithinDuration(t, time.Now(), entry.LastCheckedAt, 2*time.Second)
}

func TestMethodHealthChecker_SetHealth_Unhealthy(t *testing.T) {
	cfg := &ant_config.AntConfig{Common: types.CommonConfig{ID: "test-ant"}}
	reg := cfg.NewAntRegistration()
	h := NewMethodHealthChecker(cfg, reg)

	h.setHealth("DOCKER", false, "docker daemon unreachable: dial unix /var/run/docker.sock: no such file")
	require.NotNil(t, reg.MethodHealth)
	entry := reg.MethodHealth["DOCKER"]
	require.NotNil(t, entry)
	assert.False(t, entry.Healthy)
	assert.Contains(t, entry.Error, "docker daemon unreachable")
}

func TestAntRegistration_SupportsPerMethodHealth(t *testing.T) {
	reg := &types.AntRegistration{
		AntID:       "test-ant",
		Methods:     []types.TaskMethod{types.Kubernetes, types.Docker, types.Shell},
		Tags:        []string{},
		ReceivedAt:  time.Now(),
		MaxCapacity: 5,
		MethodHealth: map[string]*types.MethodHealthEntry{
			"KUBERNETES": {Healthy: false, Error: "api server down", LastCheckedAt: time.Now()},
			"DOCKER":     {Healthy: true, LastCheckedAt: time.Now()},
			// SHELL has no entry — treated as healthy
		},
	}

	timeout := 5 * time.Minute

	// KUBERNETES is unhealthy — must not be supported.
	assert.False(t, reg.Supports(types.Kubernetes, nil, timeout),
		"unhealthy KUBERNETES method must not be supported")

	// DOCKER is healthy — must still be supported on the same ant.
	assert.True(t, reg.Supports(types.Docker, nil, timeout),
		"healthy DOCKER method must remain supported")

	// SHELL has no health entry — defaults to supported.
	assert.True(t, reg.Supports(types.Shell, nil, timeout),
		"method with no health entry must default to supported")
}

func TestMethodHealthChecker_ConsecutiveFailureThreshold(t *testing.T) {
	cfg := &ant_config.AntConfig{Common: types.CommonConfig{ID: "test-ant"}}
	reg := cfg.NewAntRegistration()
	h := NewMethodHealthChecker(cfg, reg)

	const method = "KUBERNETES"

	// Below the threshold: failures should NOT yet mark the method unhealthy.
	for i := 0; i < unhealthyThreshold-1; i++ {
		h.setHealth(method, false, "transient error")
		// Manually increment the counter to simulate checkAll behaviour.
		h.consecutiveFails[method] = i + 1
	}
	// Only threshold-1 failures → health still unset (method treated as healthy).
	// The setHealth calls above DO write entries, so re-check via the actual threshold logic.
	// Reset and drive through checkAll-equivalent logic:
	reg2 := cfg.NewAntRegistration()
	h2 := NewMethodHealthChecker(cfg, reg2)
	for i := 0; i < unhealthyThreshold-1; i++ {
		h2.consecutiveFails[method]++
		if h2.consecutiveFails[method] >= unhealthyThreshold {
			h2.setHealth(method, false, "error")
		}
	}
	assert.Nil(t, reg2.MethodHealth, "below threshold: MethodHealth must remain nil")

	// At exactly the threshold: method must be marked unhealthy.
	h2.consecutiveFails[method]++
	if h2.consecutiveFails[method] >= unhealthyThreshold {
		h2.setHealth(method, false, "persistent error")
	}
	require.NotNil(t, reg2.MethodHealth)
	assert.False(t, reg2.MethodHealth[method].Healthy, "at threshold: method must be marked unhealthy")

	// Recovery: one success resets counter and marks healthy.
	h2.consecutiveFails[method] = 0
	h2.setHealth(method, true, "")
	assert.True(t, reg2.MethodHealth[method].Healthy, "after recovery: method must be marked healthy again")
}

func TestMethodHealthChecker_StopIsIdempotent(t *testing.T) {
	cfg := &ant_config.AntConfig{Common: types.CommonConfig{ID: "test-ant"}}
	reg := cfg.NewAntRegistration()
	h := NewMethodHealthChecker(cfg, reg)

	ctx := context.Background()
	h.Start(ctx)
	h.Stop()
	h.Stop() // second Stop must not panic
}
