// SPDX-License-Identifier: AGPL-3.0-or-later

package trigger

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"plexobject.com/formicary/queen/types"
)

// fixtureDir returns the path to the fixtures directory relative to this file's package.
func fixtureDir(t *testing.T) string {
	t.Helper()
	// Walk up from queen/trigger to the repo root, then into fixtures/.
	dir, err := os.Getwd()
	require.NoError(t, err)
	return filepath.Join(dir, "..", "..", "fixtures")
}

// Test_TriggerParse_WebhookFixture verifies that the webhook fixture YAML is parsed
// correctly: both triggers are present with filter/param/dedup_key template expressions intact.
func Test_TriggerParse_WebhookFixture(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(fixtureDir(t), "webhook_trigger_job.yaml"))
	require.NoError(t, err)

	job, err := types.NewJobDefinitionFromYaml(raw)
	require.NoError(t, err)
	require.Len(t, job.Triggers, 2, "expected 2 triggers")

	push := job.Triggers[0]
	require.Equal(t, "on-push", push.Name)
	require.Equal(t, "webhook", push.Type)
	require.NotEmpty(t, push.Filter, "filter should be preserved")
	require.Contains(t, push.Filter, "refs/heads/main", "filter should reference main branch")
	require.NotEmpty(t, push.Params["branch"], "branch param template should be present")
	require.NotEmpty(t, push.DedupKey, "dedup_key should be preserved")

	dev := job.Triggers[1]
	require.Equal(t, "on-push-dev", dev.Name)
	require.NotEmpty(t, dev.Params["branch"])
}

// Test_TriggerParse_QueueFixture verifies queue fixture trigger parsing.
func Test_TriggerParse_QueueFixture(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(fixtureDir(t), "queue_trigger_job.yaml"))
	require.NoError(t, err)

	job, err := types.NewJobDefinitionFromYaml(raw)
	require.NoError(t, err)
	require.Len(t, job.Triggers, 1)

	trig := job.Triggers[0]
	require.Equal(t, "queue", trig.Type)
	require.Equal(t, "high-value-order", trig.Name)
	require.Equal(t, "orders.completed", trig.Topic)
	require.NotEmpty(t, trig.Filter, "filter should be preserved")
	require.Contains(t, trig.Filter, "atoi", "filter should use atoi")
	require.NotEmpty(t, trig.Params["order_id"])
	require.NotEmpty(t, trig.DedupKey)
}

// Test_TriggerParse_S3Fixture verifies s3 fixture trigger parsing.
func Test_TriggerParse_S3Fixture(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(fixtureDir(t), "s3_trigger_job.yaml"))
	require.NoError(t, err)

	job, err := types.NewJobDefinitionFromYaml(raw)
	require.NoError(t, err)
	require.NotEmpty(t, job.Triggers)

	for _, trig := range job.Triggers {
		require.Equal(t, "s3", trig.Type)
		require.NotEmpty(t, trig.Name)
	}
}

// Test_WebhookEvaluator_FullPath exercises the full webhook trigger path:
// YAML fixture → triggers populated → evaluator runs → params extracted.
func Test_WebhookEvaluator_FullPath(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(fixtureDir(t), "webhook_trigger_job.yaml"))
	require.NoError(t, err)

	job, err := types.NewJobDefinitionFromYaml(raw)
	require.NoError(t, err)
	require.NotEmpty(t, job.Triggers)

	// Find the unauthenticated dev trigger.
	var devTrig *types.TriggerDefinition
	for _, t2 := range job.Triggers {
		if t2.Name == "on-push-dev" {
			devTrig = t2
			break
		}
	}
	require.NotNil(t, devTrig, "on-push-dev trigger not found")

	ev := NewEvaluator(newMemTriggerRepo())
	result, err := ev.Evaluate(context.Background(), &TriggerEvent{
		JobDefinition: job,
		Trigger:       devTrig,
		Data: map[string]interface{}{
			"Body": map[string]interface{}{
				"ref": "refs/heads/main",
				"head_commit": map[string]interface{}{
					"id": "abc123",
				},
			},
		},
	})
	require.NoError(t, err)
	require.True(t, result.Passed)
	require.NotEmpty(t, result.Params["branch"])
	require.NotEmpty(t, result.Params["commit_sha"])
}

// Test_QueueEvaluator_FullPath exercises the queue trigger filter with atoi.
func Test_QueueEvaluator_FullPath(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(fixtureDir(t), "queue_trigger_job.yaml"))
	require.NoError(t, err)

	job, err := types.NewJobDefinitionFromYaml(raw)
	require.NoError(t, err)
	require.NotEmpty(t, job.Triggers)
	trig := job.Triggers[0]

	ev := NewEvaluator(newMemTriggerRepo())
	ctx := context.Background()

	// Below threshold — should be filtered out.
	lowOrder := map[string]interface{}{
		"Message": map[string]interface{}{
			"order_id":      "ord-1",
			"customer_name": "Alice",
			"total":         500,
			"currency":      "USD",
		},
	}
	r1, err := ev.Evaluate(ctx, &TriggerEvent{JobDefinition: job, Trigger: trig, Data: lowOrder})
	require.NoError(t, err)
	require.False(t, r1.Passed, "order below $1000 should be filtered")

	// Above threshold — should pass with correct params.
	highOrder := map[string]interface{}{
		"Message": map[string]interface{}{
			"order_id":      "ord-2",
			"customer_name": "Bob",
			"total":         2500,
			"currency":      "EUR",
		},
	}
	r2, err := ev.Evaluate(ctx, &TriggerEvent{JobDefinition: job, Trigger: trig, Data: highOrder})
	require.NoError(t, err)
	require.True(t, r2.Passed, "order above $1000 should pass")
	require.Equal(t, "ord-2", r2.Params["order_id"])
	require.Equal(t, "EUR", r2.Params["currency"])
	require.Equal(t, "order-ord-2", r2.DedupKey)
}

// Test_RateLimit_Resets_AfterWindow verifies rate-limit window reset without sleep.
func Test_RateLimit_Resets_AfterWindow(t *testing.T) {
	repo := newMemTriggerRepo()
	ev := NewEvaluator(repo)
	window := 5 * time.Minute
	trig := &types.TriggerDefinition{
		Type: "webhook",
		Name: "rl-reset",
		RateLimit: &types.TriggerRateLimit{
			Max:    1,
			Window: window,
		},
	}
	def := &types.JobDefinition{ID: "def-rl", JobType: "rl-job"}
	ctx := context.Background()
	data := map[string]interface{}{}

	r1, err := ev.Evaluate(ctx, &TriggerEvent{JobDefinition: def, Trigger: trig, Data: data})
	require.NoError(t, err)
	require.True(t, r1.Passed)

	r2, err := ev.Evaluate(ctx, &TriggerEvent{JobDefinition: def, Trigger: trig, Data: data})
	require.NoError(t, err)
	require.False(t, r2.Passed, "should be rate-limited")

	// Expire the window by back-dating WindowStart.
	k := repo.key(def.ID, trig.Name)
	repo.states[k].WindowStart = time.Now().Add(-(window + time.Second))

	r3, err := ev.Evaluate(ctx, &TriggerEvent{JobDefinition: def, Trigger: trig, Data: data})
	require.NoError(t, err)
	require.True(t, r3.Passed, "should pass after window reset")
}

// Test_RecordFired_UpdatesLastSeenTime verifies that RecordFired writes LastSeenTime
// so the dashboard "Last Fired" column reflects each trigger invocation.
func Test_RecordFired_UpdatesLastSeenTime(t *testing.T) {
	repo := newMemTriggerRepo()
	before := time.Now()

	err := repo.RecordFired("def-1", "on-push")
	require.NoError(t, err)

	state, err := repo.FindByJobAndTrigger("def-1", "on-push")
	require.NoError(t, err)
	require.NotNil(t, state)
	require.False(t, state.LastSeenTime.IsZero(), "LastSeenTime should be set")
	require.True(t, !state.LastSeenTime.Before(before), "LastSeenTime should be >= before")

	// Second call updates the time.
	time.Sleep(time.Millisecond)
	first := state.LastSeenTime
	err = repo.RecordFired("def-1", "on-push")
	require.NoError(t, err)
	state2, _ := repo.FindByJobAndTrigger("def-1", "on-push")
	require.False(t, state2.LastSeenTime.Before(first), "second RecordFired should update time")
}

// Test_IsDuplicateKeyError verifies the helper recognises all DB duplicate-key error formats.
func Test_IsDuplicateKeyError(t *testing.T) {
	require.True(t, isDuplicateKeyError(fmt.Errorf("UNIQUE constraint failed: formicary_job_requests.user_key")))
	require.True(t, isDuplicateKeyError(fmt.Errorf("Duplicate entry 'key-1' for key 'user_key'")))
	require.True(t, isDuplicateKeyError(fmt.Errorf("duplicate key value violates unique constraint")))
	require.False(t, isDuplicateKeyError(fmt.Errorf("some other db error")))
	require.False(t, isDuplicateKeyError(nil))
}
