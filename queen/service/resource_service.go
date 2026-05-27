// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"plexobject.com/formicary/internal/grpc/interceptors"
	commonTypes "plexobject.com/formicary/internal/types"
	svcpb "plexobject.com/formicary/gen/go/formicary/v1/services"
	protoResource "plexobject.com/formicary/gen/go/formicary/v1/resource"
	protoUser "plexobject.com/formicary/gen/go/formicary/v1/user"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/repository"
)

// ResourceService implements svcpb.ResourceServiceServer.
type ResourceService struct {
	svcpb.UnimplementedResourceServiceServer
	dashboardManager       *manager.DashboardManager
	subscriptionRepository repository.SubscriptionRepository
}

// NewResourceService creates a ResourceService.
func NewResourceService(
	dashboardManager *manager.DashboardManager,
	subscriptionRepository repository.SubscriptionRepository,
) *ResourceService {
	return &ResourceService{
		dashboardManager:       dashboardManager,
		subscriptionRepository: subscriptionRepository,
	}
}

func (s *ResourceService) QueryAntRegistrations(ctx context.Context, req *svcpb.QueryAntRegistrationsRequest) (*svcpb.QueryAntRegistrationsResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	all := s.dashboardManager.AntRegistrations()

	// Filter by method and tags in-memory (no DB query for registrations).
	var filtered []*commonTypes.AntRegistration
	for _, r := range all {
		if req.Method != "" {
			found := false
			for _, m := range r.Methods {
				if strings.EqualFold(string(m), req.Method) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		filtered = append(filtered, r)
	}

	total := int64(len(filtered))
	ps := pageSize(req.PageSize)
	p := int(req.Page)
	start := p * ps
	end := start + ps
	if start > len(filtered) {
		start = len(filtered)
	}
	if end > len(filtered) {
		end = len(filtered)
	}
	page := filtered[start:end]

	return &svcpb.QueryAntRegistrationsResponse{
		Records:      toProtoAntRegistrations(page),
		TotalRecords: total,
		Page:         req.Page,
		PageSize:     effectivePageSize(req.PageSize),
		TotalPages:   totalPages(total, effectivePageSize(req.PageSize)),
	}, nil
}

func (s *ResourceService) GetAntRegistration(ctx context.Context, req *svcpb.GetAntRegistrationRequest) (*svcpb.GetAntRegistrationResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	for _, r := range s.dashboardManager.AntRegistrations() {
		if r.AntID == req.AntId {
			return &svcpb.GetAntRegistrationResponse{AntRegistration: toProtoAntRegistration(r)}, nil
		}
	}
	return nil, status.Errorf(codes.NotFound, "ant registration %s not found", req.AntId)
}

func (s *ResourceService) QuerySubscriptions(ctx context.Context, req *svcpb.QuerySubscriptionsRequest) (*svcpb.QuerySubscriptionsResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	params := map[string]interface{}{}
	if req.UserId != "" {
		params["user_id"] = req.UserId
	}
	if req.OrganizationId != "" {
		params["organization_id"] = req.OrganizationId
	}
	if req.Kind != "" {
		params["kind"] = req.Kind
	}
	recs, total, err := s.subscriptionRepository.Query(qc, params, int(req.Page), pageSize(req.PageSize), req.Order)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &svcpb.QuerySubscriptionsResponse{
		Records:      toProtoSubscriptions(recs),
		TotalRecords: total,
		Page:         req.Page,
		PageSize:     effectivePageSize(req.PageSize),
		TotalPages:   totalPages(total, effectivePageSize(req.PageSize)),
	}, nil
}

func (s *ResourceService) GetSubscription(ctx context.Context, req *svcpb.GetSubscriptionRequest) (*svcpb.GetSubscriptionResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	sub, err := s.subscriptionRepository.Get(qc, req.Id)
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &svcpb.GetSubscriptionResponse{Subscription: toProtoSubscription(sub)}, nil
}

func (s *ResourceService) SaveSubscription(ctx context.Context, req *svcpb.SaveSubscriptionRequest) (*svcpb.SaveSubscriptionResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	sub := fromProtoSubscription(req.Subscription)
	if sub == nil {
		return nil, status.Error(codes.InvalidArgument, "subscription is required")
	}
	var saved *commonTypes.Subscription
	var err error
	if req.Id != "" {
		sub.ID = req.Id
		saved, err = s.subscriptionRepository.Update(qc, sub)
	} else {
		saved, err = s.subscriptionRepository.Create(qc, sub)
	}
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &svcpb.SaveSubscriptionResponse{Subscription: toProtoSubscription(saved)}, nil
}

// ---- Ant registration conversion helpers -----------------------------------

func toProtoAntRegistration(r *commonTypes.AntRegistration) *protoResource.AntRegistration {
	if r == nil {
		return nil
	}
	methods := make([]string, 0, len(r.Methods))
	for _, m := range r.Methods {
		methods = append(methods, string(m))
	}
	return &protoResource.AntRegistration{
		AntId:         r.AntID,
		AntTopic:      r.AntTopic,
		EncryptionKey: r.EncryptionKey,
		MaxCapacity:   int32(r.MaxCapacity),
		Tags:          r.Tags,
		Methods:       methods,
		CurrentLoad:   int32(r.CurrentLoad),
		TotalExecuted: int32(r.TotalExecuted),
		AutoRefresh:   r.AutoRefresh,
		CreatedAt:     timestamppb.New(r.CreatedAt),
		AntStartedAt:  timestamppb.New(r.AntStartedAt),
	}
}

func toProtoAntRegistrations(regs []*commonTypes.AntRegistration) []*protoResource.AntRegistration {
	out := make([]*protoResource.AntRegistration, 0, len(regs))
	for _, r := range regs {
		out = append(out, toProtoAntRegistration(r))
	}
	return out
}

// ---- Subscription conversion helpers ---------------------------------------

func toProtoSubscription(s *commonTypes.Subscription) *protoUser.Subscription {
	if s == nil {
		return nil
	}
	return &protoUser.Subscription{
		Id:             s.ID,
		UserId:         s.UserID,
		OrganizationId: s.OrganizationID,
		Policy:         string(s.Policy),
		Kind:           string(s.Kind),
		Period:         string(s.Period),
		Price:          s.Price,
		CpuQuota:       s.CPUQuota,
		DiskQuota:      s.DiskQuota,
		StartedAt:      timestamppb.New(s.StartedAt),
		EndedAt:        timestamppb.New(s.EndedAt),
	}
}

func toProtoSubscriptions(subs []*commonTypes.Subscription) []*protoUser.Subscription {
	out := make([]*protoUser.Subscription, 0, len(subs))
	for _, s := range subs {
		out = append(out, toProtoSubscription(s))
	}
	return out
}

func fromProtoSubscription(p *protoUser.Subscription) *commonTypes.Subscription {
	if p == nil {
		return nil
	}
	sub := &commonTypes.Subscription{
		ID:             p.Id,
		UserID:         p.UserId,
		OrganizationID: p.OrganizationId,
		Policy:         commonTypes.Policy(p.Policy),
		Kind:           commonTypes.Kind(p.Kind),
		Period:         commonTypes.Period(p.Period),
		Price:          p.Price,
		CPUQuota:       p.CpuQuota,
		DiskQuota:      p.DiskQuota,
	}
	if p.StartedAt != nil {
		sub.StartedAt = p.StartedAt.AsTime()
	}
	if p.EndedAt != nil {
		sub.EndedAt = p.EndedAt.AsTime()
	}
	return sub
}
