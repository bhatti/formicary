// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"plexobject.com/formicary/internal/grpc/interceptors"
	svcpb "plexobject.com/formicary/gen/go/formicary/v1/services"
	protoQueen "plexobject.com/formicary/gen/go/formicary/v1/queen"
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
	if err := s.jobManager.RestartJobRequest(qc, req.Id, req.Hard, req.Version); err != nil {
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

func (s *JobExecutionService) VoteOnApproval(ctx context.Context, req *svcpb.VoteOnApprovalRequest) (*svcpb.VoteOnApprovalResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	if req.Vote == nil {
		return nil, status.Error(codes.InvalidArgument, "vote is required")
	}
	voteReq := &queenTypes.ApprovalVoteRequest{
		RequestID: req.RequestId,
		TaskType:  req.TaskType,
		VoterID:   qc.GetUserID(),
		VoterName: req.Vote.VoterName,
		Decision:  queenTypes.ApprovalDecision(req.Vote.Decision),
		Comments:  req.Vote.Comments,
	}
	approvalStatus, err := s.jobManager.CastApprovalVote(ctx, qc, voteReq)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &svcpb.VoteOnApprovalResponse{
		Status: toProtoApprovalStatus(approvalStatus),
	}, nil
}

func (s *JobExecutionService) GetApprovalStatus(ctx context.Context, req *svcpb.GetApprovalStatusRequest) (*svcpb.GetApprovalStatusResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	approvalStatus, err := s.jobManager.GetApprovalStatus(ctx, qc, req.RequestId, req.TaskType)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &svcpb.GetApprovalStatusResponse{
		Status: toProtoApprovalStatus(approvalStatus),
	}, nil
}

func (s *JobExecutionService) ListPendingApprovals(ctx context.Context, req *svcpb.ListPendingApprovalsRequest) (*svcpb.ListPendingApprovalsResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	page := int(req.Page)
	pageSize := int(req.PageSize)
	if pageSize <= 0 {
		pageSize = 20
	}
	pendingApprovals, total, err := s.jobManager.ListPendingApprovals(ctx, qc, page, pageSize)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	resp := &svcpb.ListPendingApprovalsResponse{
		TotalRecords: total,
		Page:         int32(page),
		PageSize:     int32(pageSize),
	}
	for _, pa := range pendingApprovals {
		resp.Approvals = append(resp.Approvals, toProtoPendingApproval(pa))
	}
	return resp, nil
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

func toProtoApprovalStatus(s *queenTypes.ApprovalStatus) *protoQueen.ApprovalStatus {
	if s == nil {
		return nil
	}
	p := &protoQueen.ApprovalStatus{
		TaskExecutionId:   s.TaskExecutionID,
		JobRequestId:      s.JobRequestID,
		ApprovalsReceived: int32(s.ApprovalsReceived),
		RejectionsReceived: int32(s.RejectionsReceived),
		MinApprovalsRequired: int32(s.MinApprovalsRequired),
		QuorumReached:     s.QuorumReached,
		Rejected:          s.Rejected,
		SlaBreached:       s.SLABreached,
	}
	if s.Deadline != nil {
		p.Deadline = timestamppb.New(*s.Deadline)
	}
	for _, v := range s.Votes {
		p.Votes = append(p.Votes, toProtoApprovalVote(v))
	}
	if s.Policy != nil {
		p.Policy = toProtoApprovalPolicy(s.Policy)
	}
	return p
}

func toProtoApprovalVote(v *queenTypes.ApprovalVote) *protoQueen.ApprovalVote {
	if v == nil {
		return nil
	}
	return &protoQueen.ApprovalVote{
		Id:              v.ID,
		TaskExecutionId: v.TaskExecutionID,
		JobRequestId:    v.JobRequestID,
		VoterId:         v.VoterID,
		VoterName:       v.VoterName,
		Decision:        string(v.Decision),
		Comments:        v.Comments,
		VotedAt:         timestamppb.New(v.VotedAt),
	}
}

func toProtoApprovalPolicy(p *queenTypes.ApprovalPolicy) *protoQueen.ApprovalPolicy {
	if p == nil {
		return nil
	}
	return &protoQueen.ApprovalPolicy{
		Id:                   p.ID,
		TaskDefinitionId:     p.TaskDefinitionID,
		MinApprovals:         int32(p.MinApprovals),
		AllowedRoles:         p.AllowedRoles,
		AllowedUsers:         p.AllowedUsers,
		RequireUnanimous:     p.RequireUnanimous,
		SlaDeadlineNs:        int64(p.SLADeadline),
		TimeoutAction:        string(p.TimeoutAction),
		EscalationRecipients: p.EscalationRecipients,
		EscalationMessage:    p.EscalationMessage,
		CreatedAt:            timestamppb.New(p.CreatedAt),
		UpdatedAt:            timestamppb.New(p.UpdatedAt),
	}
}

func toProtoPendingApproval(pa *queenTypes.PendingApproval) *protoQueen.PendingApproval {
	if pa == nil {
		return nil
	}
	return &protoQueen.PendingApproval{
		JobRequestId:    pa.JobRequestID,
		TaskExecutionId: pa.TaskExecutionID,
		JobType:         pa.JobType,
		TaskType:        pa.TaskType,
		Status:          toProtoApprovalStatus(pa.Status),
		RequestedAt:     timestamppb.New(pa.RequestedAt),
	}
}
