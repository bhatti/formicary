// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"encoding/json"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"plexobject.com/formicary/internal/grpc/interceptors"
	svcpb "plexobject.com/formicary/gen/go/formicary/v1/services"
	protoDomain "plexobject.com/formicary/gen/go/formicary/v1/domain"
	"plexobject.com/formicary/queen/config"
	queenTypes "plexobject.com/formicary/queen/types"
	"plexobject.com/formicary/queen/repository"
)

// ensure SlackService implements the generated server interface at compile time.
var _ svcpb.SlackServiceServer = (*SlackService)(nil)

// SlackService implements svcpb.SlackServiceServer.
// Routes are stored as a JSON-encoded []config.SlackRouteConfig in a SystemConfig
// record with kind="JSON" and name="SlackRoutes". This makes them editable via the
// Sys Configs admin page as well as through this API.
type SlackService struct {
	svcpb.UnimplementedSlackServiceServer
	systemConfigRepo repository.SystemConfigRepository
}

// NewSlackService creates a SlackService.
func NewSlackService(systemConfigRepo repository.SystemConfigRepository) *SlackService {
	return &SlackService{systemConfigRepo: systemConfigRepo}
}

func (s *SlackService) GetSlackRoutes(ctx context.Context, _ *svcpb.GetSlackRoutesRequest) (*svcpb.GetSlackRoutesResponse, error) {
	if interceptors.QueryContextFromContext(ctx) == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	cfg, err := s.systemConfigRepo.GetByKindName("JSON", "SlackRoutes")
	if err != nil || cfg == nil {
		return &svcpb.GetSlackRoutesResponse{}, nil
	}
	var routes []config.SlackRouteConfig
	if err := json.Unmarshal([]byte(cfg.Value), &routes); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to parse slack routes: %v", err)
	}
	return &svcpb.GetSlackRoutesResponse{
		Routes:   toProtoSlackRoutes(routes),
		ConfigId: cfg.ID,
	}, nil
}

func (s *SlackService) SaveSlackRoutes(ctx context.Context, req *svcpb.SaveSlackRoutesRequest) (*svcpb.SaveSlackRoutesResponse, error) {
	qc := interceptors.QueryContextFromContext(ctx)
	if qc == nil {
		return nil, status.Error(codes.Unauthenticated, "no query context")
	}
	if !qc.IsAdmin() {
		return nil, status.Error(codes.PermissionDenied, "admin role required to save slack routes")
	}
	routes := fromProtoSlackRoutes(req.Routes)
	b, err := json.Marshal(routes)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to marshal slack routes: %v", err)
	}
	existing, _ := s.systemConfigRepo.GetByKindName("JSON", "SlackRoutes")
	if existing != nil {
		existing.Value = string(b)
		if _, err := s.systemConfigRepo.Save(existing); err != nil {
			return nil, interceptors.MapDomainError(err)
		}
		return &svcpb.SaveSlackRoutesResponse{
			ConfigId: existing.ID,
			Routes:   toProtoSlackRoutes(routes),
		}, nil
	}
	saved, err := s.systemConfigRepo.Save(queenTypes.NewSystemConfig("", "JSON", "SlackRoutes", string(b)))
	if err != nil {
		return nil, interceptors.MapDomainError(err)
	}
	return &svcpb.SaveSlackRoutesResponse{
		ConfigId: saved.ID,
		Routes:   toProtoSlackRoutes(routes),
	}, nil
}

// ---- conversion helpers -----------------------------------------------------

func toProtoSlackRoute(r config.SlackRouteConfig) *protoDomain.SlackRoute {
	return &protoDomain.SlackRoute{
		Triggers:    r.Triggers,
		JobType:     r.JobType,
		Description: r.Description,
		IdVar:       r.IdVar,
		Params:      r.Params,
	}
}

func toProtoSlackRoutes(routes []config.SlackRouteConfig) []*protoDomain.SlackRoute {
	out := make([]*protoDomain.SlackRoute, 0, len(routes))
	for _, r := range routes {
		out = append(out, toProtoSlackRoute(r))
	}
	return out
}

func fromProtoSlackRoute(p *protoDomain.SlackRoute) config.SlackRouteConfig {
	return config.SlackRouteConfig{
		Triggers:    p.Triggers,
		JobType:     p.JobType,
		Description: p.Description,
		IdVar:       p.IdVar,
		Params:      p.Params,
	}
}

func fromProtoSlackRoutes(routes []*protoDomain.SlackRoute) []config.SlackRouteConfig {
	out := make([]config.SlackRouteConfig, 0, len(routes))
	for _, r := range routes {
		if r != nil {
			out = append(out, fromProtoSlackRoute(r))
		}
	}
	return out
}
