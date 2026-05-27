// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	commonTypes "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/grpc/interceptors"
	svcpb "plexobject.com/formicary/gen/go/formicary/v1/services"
	queenTypes "plexobject.com/formicary/queen/types"
	"plexobject.com/formicary/queen/manager"
)

// JobExecutionService implements svcpb.JobExecutionServiceServer.
type JobExecutionService struct {
	svcpb.UnimplementedJobExecutionServiceServer
	jobManager *manager.JobManager
}

// NewJobExecutionService creates a JobExecutionService backed by jobManager.
func NewJobExecutionService(jobManager *manager.JobManager) *JobExecutionService {
	return &JobExecutionService{jobManager: jobManager}
}

func (s *JobExecutionService) SubmitJob(ctx context.Context, req *svcpb.SubmitJobRequest) (*svcpb.SubmitJobResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	jr := queenTypes.NewRequest()
	jr.JobType = req.JobType
	jr.JobGroup = req.JobGroup
	jr.JobPriority = int(req.JobPriority)
	jr.UserKey = req.UserKey
	jr.Description = req.Description
	if req.ScheduledAt != "" {
		if t, err := time.Parse(time.RFC3339, req.ScheduledAt); err == nil {
			jr.ScheduledAt = t
		}
	}
	for k, v := range req.Params {
		jr.NameValueParams[k] = v
	}
	saved, err := s.jobManager.SaveJobRequest(qc, jr)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &svcpb.SubmitJobResponse{JobRequest: toProtoJobRequest(saved)}, nil
}

func (s *JobExecutionService) QueryJobRequests(ctx context.Context, req *svcpb.QueryJobRequestsRequest) (*svcpb.QueryJobRequestsResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	params := map[string]interface{}{}
	if req.JobType != "" {
		params["job_type"] = req.JobType
	}
	if req.JobState != "" {
		params["job_state"] = req.JobState
	}
	if req.JobGroup != "" {
		params["job_group"] = req.JobGroup
	}
	if req.OrganizationId != "" {
		params["organization_id"] = req.OrganizationId
	}
	if req.UserId != "" {
		params["user_id"] = req.UserId
	}
	recs, total, err := s.jobManager.QueryJobRequests(qc, params, int(req.Page), pageSize(req.PageSize), req.Order)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &svcpb.QueryJobRequestsResponse{
		Records:      toProtoJobRequestSlice(recs),
		TotalRecords: total,
		Page:         req.Page,
		PageSize:     effectivePageSize(req.PageSize),
		TotalPages:   totalPages(total, effectivePageSize(req.PageSize)),
	}, nil
}

func (s *JobExecutionService) GetJobRequest(ctx context.Context, req *svcpb.GetJobRequestRequest) (*svcpb.GetJobRequestResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	jr, err := s.jobManager.GetJobRequest(qc, req.Id)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &svcpb.GetJobRequestResponse{JobRequest: toProtoJobRequest(jr)}, nil
}

func (s *JobExecutionService) GetJobExecution(ctx context.Context, req *svcpb.GetJobExecutionRequest) (*svcpb.GetJobExecutionResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	exec, err := s.jobManager.GetJobExecution(req.Id)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &svcpb.GetJobExecutionResponse{JobExecution: toProtoJobExecution(exec)}, nil
}

func (s *JobExecutionService) CancelJob(ctx context.Context, req *svcpb.CancelJobRequest) (*emptypb.Empty, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	if err := s.jobManager.CancelJobRequest(qc, req.Id); err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *JobExecutionService) PauseJob(ctx context.Context, req *svcpb.PauseJobRequest) (*emptypb.Empty, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	if err := s.jobManager.PauseJobRequest(qc, req.Id); err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *JobExecutionService) RestartJob(ctx context.Context, req *svcpb.RestartJobRequest) (*emptypb.Empty, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	if err := s.jobManager.RestartJobRequest(qc, req.Id); err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *JobExecutionService) TriggerJob(ctx context.Context, req *svcpb.TriggerJobRequest) (*emptypb.Empty, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	if err := s.jobManager.TriggerJobRequest(qc, req.Id); err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *JobExecutionService) ReviewJob(ctx context.Context, req *svcpb.ReviewJobRequest) (*emptypb.Empty, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	review := req.Review
	if review == nil {
		return nil, status.Error(codes.InvalidArgument, "review is required")
	}
	reviewReq := &queenTypes.ReviewTaskRequest{
		RequestID:   review.RequestId,
		ExecutionID: review.ExecutionId,
		TaskType:    review.TaskType,
		ReviewedBy:  review.ReviewedBy,
		Comments:    review.Comments,
		Status:      commonTypes.RequestState(review.Status),
	}
	if err := s.jobManager.ReviewTaskRequestForManualApproval(ctx, qc, reviewReq); err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *JobExecutionService) GetJobWaitTime(ctx context.Context, req *svcpb.GetJobRequestRequest) (*svcpb.JobWaitTimeResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	estimate, err := s.jobManager.GetWaitEstimate(qc, req.Id)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &svcpb.JobWaitTimeResponse{
		EstimatedWaitSecs: int64(estimate.EstimatedWait / time.Second),
	}, nil
}

func (s *JobExecutionService) GetJobRequestMermaid(ctx context.Context, req *svcpb.GetJobRequestRequest) (*svcpb.GetJobExecutionResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	mermaid, err := s.jobManager.GetMermaidConfigForJobRequest(qc, req.Id)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	// Return mermaid as the mermaid field on a wrapped execution response
	return &svcpb.GetJobExecutionResponse{Mermaid: mermaid}, nil
}

func (s *JobExecutionService) GetJobStats(ctx context.Context, req *svcpb.QueryJobRequestsRequest) (*svcpb.JobRequestStatsResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	params := map[string]interface{}{}
	if req.JobType != "" {
		params["job_type"] = req.JobType
	}
	_, total, err := s.jobManager.QueryJobRequests(qc, params, 0, 1, nil)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &svcpb.JobRequestStatsResponse{
		Stats: []*svcpb.JobRequestStat{
			{JobType: req.JobType, Total: total},
		},
	}, nil
}
