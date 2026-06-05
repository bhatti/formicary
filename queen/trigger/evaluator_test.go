// SPDX-License-Identifier: AGPL-3.0-or-later

package trigger

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"plexobject.com/formicary/queen/types"
)

// memTriggerRepo is an in-memory TriggerStateRepository for evaluator unit tests.
type memTriggerRepo struct {
	states map[string]*types.TriggerState
}

func newMemTriggerRepo() *memTriggerRepo {
	return &memTriggerRepo{states: make(map[string]*types.TriggerState)}
}

func (m *memTriggerRepo) key(jobDefID, triggerName string) string {
	return jobDefID + "/" + triggerName
}

func (m *memTriggerRepo) FindByJobDefinitionID(jobDefinitionID string) ([]*types.TriggerState, error) {
	var out []*types.TriggerState
	for _, s := range m.states {
		if s.JobDefinitionID == jobDefinitionID {
			cp := *s
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (m *memTriggerRepo) FindByJobAndTrigger(jobDefinitionID, triggerName string) (*types.TriggerState, error) {
	if s, ok := m.states[m.key(jobDefinitionID, triggerName)]; ok {
		cp := *s
		return &cp, nil
	}
	return nil, nil
}

func (m *memTriggerRepo) Upsert(state *types.TriggerState) (*types.TriggerState, error) {
	cp := *state
	m.states[m.key(state.JobDefinitionID, state.TriggerName)] = &cp
	return &cp, nil
}

func (m *memTriggerRepo) RecordFired(jobDefinitionID, triggerName string) error {
	k := m.key(jobDefinitionID, triggerName)
	if s, ok := m.states[k]; ok {
		s.LastSeenTime = time.Now()
	} else {
		m.states[k] = &types.TriggerState{
			JobDefinitionID: jobDefinitionID,
			TriggerName:     triggerName,
			LastSeenTime:    time.Now(),
		}
	}
	return nil
}

func (m *memTriggerRepo) IncrementWindowCount(jobDefinitionID, triggerName string, windowDuration time.Duration) (int32, error) {
	k := m.key(jobDefinitionID, triggerName)
	now := time.Now()
	s, ok := m.states[k]
	if !ok || now.Sub(s.WindowStart) >= windowDuration {
		s = &types.TriggerState{
			JobDefinitionID: jobDefinitionID,
			TriggerName:     triggerName,
			WindowStart:     now,
			WindowCount:     1,
		}
		m.states[k] = s
		return 1, nil
	}
	s.WindowCount++
	return s.WindowCount, nil
}

func (m *memTriggerRepo) Reset(jobDefinitionID, triggerName string) error {
	if s, ok := m.states[m.key(jobDefinitionID, triggerName)]; ok {
		s.LastSeenKey = ""
		s.LastSeenTime = time.Time{}
		s.WindowStart = time.Time{}
		s.WindowCount = 0
	}
	return nil
}

func (m *memTriggerRepo) DeleteByJobDefinitionID(jobDefinitionID string) error {
	for k, s := range m.states {
		if s.JobDefinitionID == jobDefinitionID {
			delete(m.states, k)
		}
	}
	return nil
}

func sampleJobDef() *types.JobDefinition {
	return &types.JobDefinition{
		ID:      "def-1",
		JobType: "test-job",
	}
}

func Test_Evaluator_NoFilter_Passes(t *testing.T) {
	ev := NewEvaluator(newMemTriggerRepo())
	trig := &types.TriggerDefinition{
		Type: "webhook",
		Name: "no-filter",
		Params: map[string]string{
			"key": "{{ .Body.key }}",
		},
	}
	result, err := ev.Evaluate(context.Background(), &TriggerEvent{
		JobDefinition: sampleJobDef(),
		Trigger:       trig,
		Data: map[string]interface{}{
			"Body": map[string]interface{}{"key": "hello"},
		},
	})
	require.NoError(t, err)
	require.True(t, result.Passed)
	require.Equal(t, "hello", result.Params["key"])
}

func Test_Evaluator_FilterFalse_Blocked(t *testing.T) {
	ev := NewEvaluator(newMemTriggerRepo())
	trig := &types.TriggerDefinition{
		Type:   "webhook",
		Name:   "filtered",
		Filter: `{{ if eq .Body.action "opened" }}true{{ end }}`,
	}
	result, err := ev.Evaluate(context.Background(), &TriggerEvent{
		JobDefinition: sampleJobDef(),
		Trigger:       trig,
		Data: map[string]interface{}{
			"Body": map[string]interface{}{"action": "closed"},
		},
	})
	require.NoError(t, err)
	require.False(t, result.Passed)
}

func Test_Evaluator_FilterTrue_Passes(t *testing.T) {
	ev := NewEvaluator(newMemTriggerRepo())
	trig := &types.TriggerDefinition{
		Type:   "webhook",
		Name:   "filtered",
		Filter: `{{ if eq .Body.action "opened" }}true{{ end }}`,
	}
	result, err := ev.Evaluate(context.Background(), &TriggerEvent{
		JobDefinition: sampleJobDef(),
		Trigger:       trig,
		Data: map[string]interface{}{
			"Body": map[string]interface{}{"action": "opened"},
		},
	})
	require.NoError(t, err)
	require.True(t, result.Passed)
}

func Test_Evaluator_DedupKey_Evaluated(t *testing.T) {
	ev := NewEvaluator(newMemTriggerRepo())
	trig := &types.TriggerDefinition{
		Type:     "webhook",
		Name:     "dedup",
		DedupKey: "webhook-{{ .Body.id }}",
	}
	result, err := ev.Evaluate(context.Background(), &TriggerEvent{
		JobDefinition: sampleJobDef(),
		Trigger:       trig,
		Data: map[string]interface{}{
			"Body": map[string]interface{}{"id": "abc123"},
		},
	})
	require.NoError(t, err)
	require.True(t, result.Passed)
	require.Equal(t, "webhook-abc123", result.DedupKey)
}

func Test_Evaluator_RateLimit_Enforced(t *testing.T) {
	repo := newMemTriggerRepo()
	ev := NewEvaluator(repo)
	trig := &types.TriggerDefinition{
		Type: "webhook",
		Name: "rate-limited",
		RateLimit: &types.TriggerRateLimit{
			Max:    2,
			Window: time.Minute,
		},
	}
	def := sampleJobDef()
	ctx := context.Background()
	data := map[string]interface{}{}

	r1, err := ev.Evaluate(ctx, &TriggerEvent{JobDefinition: def, Trigger: trig, Data: data})
	require.NoError(t, err)
	require.True(t, r1.Passed, "first call should pass")

	r2, err := ev.Evaluate(ctx, &TriggerEvent{JobDefinition: def, Trigger: trig, Data: data})
	require.NoError(t, err)
	require.True(t, r2.Passed, "second call should pass")

	r3, err := ev.Evaluate(ctx, &TriggerEvent{JobDefinition: def, Trigger: trig, Data: data})
	require.NoError(t, err)
	require.False(t, r3.Passed, "third call should be rate-limited")
}

func Test_Evaluator_LiteralParam_NoTemplate(t *testing.T) {
	ev := NewEvaluator(newMemTriggerRepo())
	trig := &types.TriggerDefinition{
		Type: "webhook",
		Name: "literal",
		Params: map[string]string{
			"env": "production",
		},
	}
	result, err := ev.Evaluate(context.Background(), &TriggerEvent{
		JobDefinition: sampleJobDef(),
		Trigger:       trig,
		Data:          map[string]interface{}{},
	})
	require.NoError(t, err)
	require.True(t, result.Passed)
	require.Equal(t, "production", result.Params["env"])
}

func Test_Evaluator_RateLimit_WindowReset(t *testing.T) {
	repo := newMemTriggerRepo()
	ev := NewEvaluator(repo)
	window := 5 * time.Minute
	trig := &types.TriggerDefinition{
		Type: "webhook",
		Name: "window-reset",
		RateLimit: &types.TriggerRateLimit{
			Max:    1,
			Window: window,
		},
	}
	def := sampleJobDef()
	ctx := context.Background()
	data := map[string]interface{}{}

	r1, err := ev.Evaluate(ctx, &TriggerEvent{JobDefinition: def, Trigger: trig, Data: data})
	require.NoError(t, err)
	require.True(t, r1.Passed)

	r2, err := ev.Evaluate(ctx, &TriggerEvent{JobDefinition: def, Trigger: trig, Data: data})
	require.NoError(t, err)
	require.False(t, r2.Passed, "should be blocked within window")

	// Back-date the window start so the evaluator sees it as expired — no sleep needed.
	k := repo.key(def.ID, trig.Name)
	if s, ok := repo.states[k]; ok {
		s.WindowStart = time.Now().Add(-(window + time.Second))
	}

	r3, err := ev.Evaluate(ctx, &TriggerEvent{JobDefinition: def, Trigger: trig, Data: data})
	require.NoError(t, err)
	require.True(t, r3.Passed, "should pass after window reset")
}

func Test_Evaluator_BadTemplate_ReturnsError(t *testing.T) {
	ev := NewEvaluator(newMemTriggerRepo())
	trig := &types.TriggerDefinition{
		Type:   "webhook",
		Name:   "bad-filter",
		Filter: "{{ .Body.unclosed",
	}
	_, err := ev.Evaluate(context.Background(), &TriggerEvent{
		JobDefinition: sampleJobDef(),
		Trigger:       trig,
		Data:          map[string]interface{}{"Body": map[string]interface{}{}},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "filter evaluation failed")
}

func Test_Evaluator_AtoiFilter(t *testing.T) {
	ev := NewEvaluator(newMemTriggerRepo())
	trig := &types.TriggerDefinition{
		Type:   "queue",
		Name:   "numeric-filter",
		Filter: `{{ if gt (atoi (printf "%v" .Message.total)) 1000 }}true{{ end }}`,
	}
	ctx := context.Background()
	def := sampleJobDef()

	// Below threshold — should be blocked.
	r1, err := ev.Evaluate(ctx, &TriggerEvent{
		JobDefinition: def,
		Trigger:       trig,
		Data:          map[string]interface{}{"Message": map[string]interface{}{"total": 500}},
	})
	require.NoError(t, err)
	require.False(t, r1.Passed)

	// Above threshold — should pass.
	r2, err := ev.Evaluate(ctx, &TriggerEvent{
		JobDefinition: def,
		Trigger:       trig,
		Data:          map[string]interface{}{"Message": map[string]interface{}{"total": 1500}},
	})
	require.NoError(t, err)
	require.True(t, r2.Passed)
}
