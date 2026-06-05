// SPDX-License-Identifier: AGPL-3.0-or-later

package trigger

import (
	"context"
	"fmt"
	"strings"
	"text/template"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"plexobject.com/formicary/internal/tracing"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/types"
	"plexobject.com/formicary/queen/utils"
)

// EvalResult holds the output of evaluating a trigger against an event.
type EvalResult struct {
	// Passed is true when the filter matched (or no filter was set) and rate limit was not exceeded.
	Passed   bool
	// Params is the map of extracted job parameters.
	Params   map[string]string
	// DedupKey is the evaluated dedup_key template result (used as JobRequest.UserKey).
	DedupKey string
}

// TriggerEvent carries the event data passed to the evaluator.
type TriggerEvent struct {
	JobDefinition *types.JobDefinition
	Trigger       *types.TriggerDefinition
	// Data is the template context: {"Body": ..., "Object": ..., "Message": ..., "Headers": ..., etc.}
	Data map[string]interface{}
}

// Evaluator runs the filter/dedup/rate-limit pipeline shared by all trigger sources.
type Evaluator struct {
	triggerStateRepo repository.TriggerStateRepository
}

// NewEvaluator creates an Evaluator backed by the given TriggerStateRepository.
func NewEvaluator(repo repository.TriggerStateRepository) *Evaluator {
	return &Evaluator{triggerStateRepo: repo}
}

// Evaluate runs the trigger pipeline and returns an EvalResult.
func (e *Evaluator) Evaluate(ctx context.Context, event *TriggerEvent) (*EvalResult, error) {
	ctx, span := tracing.Tracer("formicary.trigger").Start(ctx, "trigger.evaluate",
		trace.WithAttributes(
			attribute.String("trigger.type", event.Trigger.Type),
			attribute.String("trigger.name", event.Trigger.Name),
			attribute.String("job.type", event.JobDefinition.JobType),
		),
	)
	defer func() { span.End() }()

	// Step 1: Filter
	if event.Trigger.Filter != "" {
		result, err := evalTemplate(event.Trigger.Filter, event.Data)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, fmt.Errorf("trigger %q filter evaluation failed: %w", event.Trigger.Name, err)
		}
		if strings.TrimSpace(result) != "true" {
			return &EvalResult{Passed: false}, nil
		}
	}

	// Step 2: Extract params from templates.
	params := make(map[string]string, len(event.Trigger.Params))
	for paramName, tmplExpr := range event.Trigger.Params {
		val, err := evalTemplate(tmplExpr, event.Data)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, fmt.Errorf("trigger %q param %q evaluation failed: %w", event.Trigger.Name, paramName, err)
		}
		params[paramName] = val
	}

	// Step 3: Evaluate dedup key.
	var dedupKey string
	if event.Trigger.DedupKey != "" {
		var err error
		dedupKey, err = evalTemplate(event.Trigger.DedupKey, event.Data)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, fmt.Errorf("trigger %q dedup_key evaluation failed: %w", event.Trigger.Name, err)
		}
		dedupKey = strings.TrimSpace(dedupKey)
	}

	// Step 4: Rate limit check.
	if event.Trigger.RateLimit != nil && event.Trigger.RateLimit.Max > 0 && event.Trigger.RateLimit.Window > 0 {
		allowed, err := e.checkAndIncrementRateLimit(ctx, event.JobDefinition.ID, event.Trigger)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, err
		}
		if !allowed {
			span.SetAttributes(attribute.Bool("trigger.rate_limited", true))
			return &EvalResult{Passed: false}, nil
		}
	}

	return &EvalResult{
		Passed:   true,
		Params:   params,
		DedupKey: dedupKey,
	}, nil
}

// checkAndIncrementRateLimit atomically increments the rate-limit window counter via
// IncrementWindowCount, which uses a DB-level conditional UPDATE to prevent TOCTOU races
// under concurrent webhook hits. Returns false if the limit has been reached.
func (e *Evaluator) checkAndIncrementRateLimit(ctx context.Context, jobDefID string, t *types.TriggerDefinition) (bool, error) {
	newCount, err := e.triggerStateRepo.IncrementWindowCount(
		jobDefID, t.Name, t.RateLimit.Window,
	)
	if err != nil {
		return false, err
	}
	return int(newCount) <= t.RateLimit.Max, nil
}

// evalTemplate evaluates a Go template expression against the given data map.
// It supports the same funcmap as queen/utils/template_helper.go plus atoi/atof.
func evalTemplate(expr string, data map[string]interface{}) (string, error) {
	if !strings.Contains(expr, "{{") {
		return expr, nil
	}
	funcs := utils.TemplateFuncs()
	// Add atoi and atof for numeric comparisons in filter expressions.
	funcs["atoi"] = func(s string) int {
		s = strings.TrimSpace(s)
		var n int
		_, _ = fmt.Sscanf(s, "%d", &n)
		return n
	}
	funcs["atof"] = func(s string) float64 {
		s = strings.TrimSpace(s)
		var f float64
		_, _ = fmt.Sscanf(s, "%f", &f)
		return f
	}
	tmpl, err := template.New("").Funcs(template.FuncMap(funcs)).Parse(expr)
	if err != nil {
		return "", fmt.Errorf("template parse error: %w", err)
	}
	var sb strings.Builder
	if err = tmpl.Execute(&sb, data); err != nil {
		return "", fmt.Errorf("template execute error: %w", err)
	}
	return sb.String(), nil
}
