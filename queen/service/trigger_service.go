// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"encoding/json"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	svcpb "plexobject.com/formicary/gen/go/formicary/v1/services"
	queenpb "plexobject.com/formicary/gen/go/formicary/v1/queen"
	"plexobject.com/formicary/internal/grpc/interceptors"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/trigger"
	"plexobject.com/formicary/queen/types"
)

// TriggerService implements svcpb.TriggerServiceServer.
type TriggerService struct {
	svcpb.UnimplementedTriggerServiceServer
	jobManager  *manager.JobManager
	triggerRepo repository.TriggerStateRepository
	evaluator   *trigger.Evaluator
	submitter   *trigger.Submitter
}

// NewTriggerService constructs a TriggerService.
func NewTriggerService(
	jobManager *manager.JobManager,
	triggerRepo repository.TriggerStateRepository,
	evaluator *trigger.Evaluator,
	submitter *trigger.Submitter,
) *TriggerService {
	return &TriggerService{
		jobManager:  jobManager,
		triggerRepo: triggerRepo,
		evaluator:   evaluator,
		submitter:   submitter,
	}
}

// ListTriggerStates returns runtime state for all triggers on a job definition.
func (s *TriggerService) ListTriggerStates(ctx context.Context, req *svcpb.ListTriggerStatesRequest) (*svcpb.ListTriggerStatesResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	def, err := s.jobManager.GetJobDefinitionByType(qc, req.GetJobType(), "")
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	states, err := s.triggerRepo.FindByJobDefinitionID(def.ID)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &svcpb.ListTriggerStatesResponse{
		States: toProtoTriggerStates(states),
	}, nil
}

// ResetTriggerState clears poll markers and rate-limit counters for one trigger.
func (s *TriggerService) ResetTriggerState(ctx context.Context, req *svcpb.ResetTriggerStateRequest) (*emptypb.Empty, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	def, err := s.jobManager.GetJobDefinitionByType(qc, req.GetJobType(), "")
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	if err := s.triggerRepo.Reset(def.ID, req.GetTriggerName()); err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &emptypb.Empty{}, nil
}

// FireWebhookTrigger programmatically fires a webhook trigger.
func (s *TriggerService) FireWebhookTrigger(ctx context.Context, req *svcpb.FireWebhookTriggerRequest) (*svcpb.FireWebhookTriggerResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	def, err := s.jobManager.GetJobDefinitionByType(qc, req.GetJobType(), "")
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}

	// Find the trigger definition by name.
	var triggerDef *types.TriggerDefinition
	for _, t := range def.Triggers {
		if t.Name == req.GetTriggerName() {
			triggerDef = t
			break
		}
	}
	if triggerDef == nil {
		return nil, status.Errorf(codes.NotFound, "trigger %q not found on job type %q", req.GetTriggerName(), req.GetJobType())
	}
	if triggerDef.Type != "webhook" {
		return nil, status.Errorf(codes.InvalidArgument, "trigger %q is type %q, not webhook", req.GetTriggerName(), triggerDef.Type)
	}

	// Parse payload as JSON.
	var body interface{}
	if len(req.GetPayload()) > 0 {
		if err := json.Unmarshal(req.GetPayload(), &body); err != nil {
			body = string(req.GetPayload())
		}
	}

	headers := make(map[string]interface{})
	for k, v := range req.GetHeaders() {
		headers[k] = v
	}

	// Query params are not available via gRPC; empty map keeps template context consistent
	// with the HTTP webhook handler so filter/param templates work identically.
	data := map[string]interface{}{
		"Body":    body,
		"Headers": headers,
		"Query":   map[string]string{},
	}

	result, err := s.evaluator.Evaluate(ctx, &trigger.TriggerEvent{
		JobDefinition: def,
		Trigger:       triggerDef,
		Data:          data,
	})
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	if !result.Passed {
		return &svcpb.FireWebhookTriggerResponse{RequestId: ""}, nil
	}

	savedReq, err := s.submitter.Submit(ctx, def, triggerDef.Name, result)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	reqID := ""
	if savedReq != nil {
		reqID = savedReq.ID
	}
	return &svcpb.FireWebhookTriggerResponse{RequestId: reqID}, nil
}

// toProtoTriggerStates maps internal GORM TriggerState rows to proto TriggerState messages.
func toProtoTriggerStates(states []*types.TriggerState) []*queenpb.TriggerState {
	out := make([]*queenpb.TriggerState, 0, len(states))
	for _, s := range states {
		out = append(out, toProtoTriggerState(s))
	}
	return out
}

func toProtoTriggerState(s *types.TriggerState) *queenpb.TriggerState {
	ps := &queenpb.TriggerState{
		Id:              s.ID,
		JobDefinitionId: s.JobDefinitionID,
		TriggerName:     s.TriggerName,
		LastSeenKey:     s.LastSeenKey,
		WindowCount:     s.WindowCount,
	}
	if !s.LastSeenTime.IsZero() {
		ps.LastSeenTime = timestamppb.New(s.LastSeenTime)
	}
	if !s.WindowStart.IsZero() {
		ps.WindowStart = timestamppb.New(s.WindowStart)
	}
	if !s.CreatedAt.IsZero() {
		ps.CreatedAt = timestamppb.New(s.CreatedAt)
	}
	if !s.UpdatedAt.IsZero() {
		ps.UpdatedAt = timestamppb.New(s.UpdatedAt)
	}
	return ps
}
