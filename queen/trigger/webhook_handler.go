// SPDX-License-Identifier: AGPL-3.0-or-later

package trigger

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"plexobject.com/formicary/internal/tracing"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/utils"
	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/types"
)

const defaultWebhookBodyMaxBytes = 1024 * 1024 // 1MB

// WebhookHandler registers and handles inbound HTTP webhook triggers.
type WebhookHandler struct {
	jobManager         *manager.JobManager
	evaluator          *Evaluator
	submitter          *Submitter
	webhookBodyMaxBytes int64
}

// NewWebhookHandler creates a WebhookHandler and registers the inbound route on webServer.
func NewWebhookHandler(
	jobManager *manager.JobManager,
	evaluator *Evaluator,
	submitter *Submitter,
	webhookBodyMaxBytes int64,
	webServer web.Server,
) *WebhookHandler {
	if webhookBodyMaxBytes <= 0 {
		webhookBodyMaxBytes = defaultWebhookBodyMaxBytes
	}
	h := &WebhookHandler{
		jobManager:          jobManager,
		evaluator:           evaluator,
		submitter:           submitter,
		webhookBodyMaxBytes: webhookBodyMaxBytes,
	}
	// Single parameterized route — no auth middleware, triggers carry their own auth.
	webServer.POST("/api/triggers/:job_type/:trigger_name", h.handleWebhook, nil)
	return h
}

func (h *WebhookHandler) handleWebhook(c web.APIContext) error {
	jobType := c.Param("job_type")
	triggerName := c.Param("trigger_name")

	ctx := c.Request().Context()
	ctx, span := tracing.Tracer("formicary.trigger").Start(ctx, "trigger.webhook",
		trace.WithAttributes(
			attribute.String("trigger.name", triggerName),
			attribute.String("job.type", jobType),
			attribute.String("http.method", c.Request().Method),
		),
	)
	defer func() { span.End() }()

	// Load the job definition. Triggers are keyed by job-type and are public-facing,
	// so we use an admin context for the lookup.
	qcAdmin := common.NewQueryContextFromIDs("", "").WithAdmin()
	jobDef, err := h.jobManager.GetJobDefinitionByType(qcAdmin, jobType, "")
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": fmt.Sprintf("job type %q not found", jobType)})
	}

	triggerDef := findTrigger(jobDef.Triggers, triggerName)
	if triggerDef == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": fmt.Sprintf("trigger %q not found on job %q", triggerName, jobType)})
	}
	if triggerDef.Type != "webhook" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "trigger is not of type webhook"})
	}

	// Read body with size limit. LimitReader is capped at maxBytes so we can detect overflow
	// by checking if we read exactly maxBytes (meaning there may be more data).
	body, err := io.ReadAll(io.LimitReader(c.Request().Body, h.webhookBodyMaxBytes))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to read request body"})
	}
	// Peek one more byte to detect if the body exceeds the limit.
	peek := make([]byte, 1)
	if n, _ := c.Request().Body.Read(peek); n > 0 {
		return c.JSON(http.StatusRequestEntityTooLarge, map[string]string{"error": "request body exceeds maximum allowed size"})
	}

	// Verify authentication.
	if triggerDef.Auth != nil {
		secret := jobDef.GetConfigString(triggerDef.Auth.SecretConfig)
		if secret == "" {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "webhook auth secret not configured"})
		}
		if err = verifyTriggerAuth(triggerDef.Auth, secret, c.Request().Header, body); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "auth failed")
			logrus.WithFields(logrus.Fields{
				"Component":   "WebhookHandler",
				"JobType":     jobType,
				"TriggerName": triggerName,
			}).Warnf("webhook auth failed: %v", err)
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication failed"})
		}
	}

	// Parse JSON body.
	var bodyData interface{}
	if len(body) > 0 {
		if err = json.Unmarshal(body, &bodyData); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		}
	}

	// Build template context.
	headers := make(map[string]string)
	for k := range c.Request().Header {
		headers[k] = c.Request().Header.Get(k)
	}
	query := make(map[string]string)
	for k, vs := range c.QueryParams() {
		if len(vs) > 0 {
			query[k] = vs[0]
		}
	}
	data := map[string]interface{}{
		"Body":    bodyData,
		"Headers": headers,
		"Query":   query,
	}

	result, err := h.evaluator.Evaluate(ctx, &TriggerEvent{
		JobDefinition: jobDef,
		Trigger:       triggerDef,
		Data:          data,
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	if !result.Passed {
		// 202 with filtered=true: caller's event didn't match the filter or hit rate limit.
		// Not an error — the webhook was received and processed correctly.
		return c.JSON(http.StatusAccepted, map[string]interface{}{"filtered": true, "request_id": ""})
	}

	saved, err := h.submitter.Submit(ctx, jobDef, triggerName, result)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	requestID := ""
	if saved != nil {
		requestID = saved.ID
	}
	return c.JSON(http.StatusAccepted, map[string]string{"request_id": requestID})
}

// verifyTriggerAuth checks the auth method on the incoming request.
func verifyTriggerAuth(auth *types.TriggerAuth, secret string, header http.Header, body []byte) error {
	switch auth.Method {
	case "hmac_sha256":
		sigHeader := header.Get(auth.Header)
		if sigHeader == "" {
			return fmt.Errorf("missing signature header %q", auth.Header)
		}
		// VerifySignature strips any "sha256=" prefix itself.
		return utils.VerifySignature(secret, sigHeader, body)
	case "bearer_token":
		authHeader := header.Get("Authorization")
		expected := "Bearer " + secret
		if subtle.ConstantTimeCompare([]byte(authHeader), []byte(expected)) != 1 {
			return fmt.Errorf("invalid bearer token")
		}
	case "api_key_header":
		val := header.Get(auth.Header)
		if subtle.ConstantTimeCompare([]byte(val), []byte(secret)) != 1 {
			return fmt.Errorf("invalid API key in header %q", auth.Header)
		}
	default:
		return fmt.Errorf("unsupported auth method %q", auth.Method)
	}
	return nil
}

// findTrigger returns the TriggerDefinition with the given name, or nil.
func findTrigger(triggers []*types.TriggerDefinition, name string) *types.TriggerDefinition {
	for _, t := range triggers {
		if t.Name == name {
			return t
		}
	}
	return nil
}
