package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/soheilhy/cmux"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"plexobject.com/formicary/internal/acl"
	"plexobject.com/formicary/internal/auth"
	internalGrpc "plexobject.com/formicary/internal/grpc"
	"plexobject.com/formicary/internal/grpc/interceptors"
	"plexobject.com/formicary/internal/health"
	"plexobject.com/formicary/internal/queue"
	"plexobject.com/formicary/internal/tasklet"
	commonTypes "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/internal/web"
	svcpb "plexobject.com/formicary/gen/go/formicary/v1/services"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/controller"
	"plexobject.com/formicary/queen/controller/admin"
	"plexobject.com/formicary/queen/gateway"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/resource"
	"plexobject.com/formicary/queen/security"
	queenService "plexobject.com/formicary/queen/service"
	"plexobject.com/formicary/queen/stats"
	"plexobject.com/formicary/queen/tasklet/wstask"
	"plexobject.com/formicary/queen/trigger"
	"plexobject.com/formicary/queen/webhook"
)

// services holds all gRPC service implementations so they can be reused
// for both gRPC registration and grpc-gateway registration.
type services struct {
	jobDef    *queenService.JobDefinitionService
	jobExec   *queenService.JobExecutionService
	user      *queenService.UserService
	org       *queenService.OrgService
	artifact  *queenService.ArtifactService
	config    *queenService.ConfigService
	resource  *queenService.ResourceService
	audit     *queenService.AuditService
	errorCode *queenService.ErrorCodeService
	jobRes    *queenService.JobResourceService
	health    *queenService.HealthService
	admin     *queenService.AdminService
	triggers  *queenService.TriggerService
}

// StartWebServer starts controllers that register REST APIs and admin dashboard,
// plus the gRPC server sharing the same TCP port via cmux.
func StartWebServer(
	ctx context.Context,
	serverCfg *config.ServerConfig,
	repoFactory *repository.Locator,
	userManager *manager.UserManager,
	jobManager *manager.JobManager,
	retentionManager *manager.RetentionManager,
	dashboardStats *manager.DashboardManager,
	resourceManager resource.Manager,
	requestRegistry tasklet.RequestRegistry,
	artifactManager *manager.ArtifactManager,
	statsRegistry *stats.JobStatsRegistry,
	healthMonitor *health.Monitor,
	queueClient queue.Client,
	webServer web.Server,
	httpClient web.HTTPClient,
) error {
	authProviders := make([]auth.Provider, 0)
	if googleAuthProvider, err := security.NewGoogleAuth(&serverCfg.Common); err == nil {
		authProviders = append(authProviders, googleAuthProvider)
	}
	if githubAuthProvider, err := security.NewGithubAuth(&serverCfg.Common, jobManager.BuildGithubPostWebhookHandler()); err == nil {
		authProviders = append(authProviders, githubAuthProvider)
	}

	if err := startWebsocketGateway(serverCfg, queueClient, repoFactory.LogEventRepository, webServer); err != nil {
		return err
	}
	if hp, ok := queueClient.(queue.HTTPHandlerProvider); ok {
		webServer.GET(hp.WebSocketPath(), web.WrapHandler(hp.HTTPHandler()), nil)
	}
	if err := startNewWebsocketProxyRegistry(
		serverCfg, resourceManager, requestRegistry, artifactManager,
		queueClient, serverCfg.Common.GetWebsocketTaskletTopic(), webServer); err != nil {
		return err
	}
	if err := startWebhookProcessor(serverCfg, queueClient, httpClient); err != nil {
		return err
	}

	startControllers(serverCfg, repoFactory, userManager, jobManager,
		resourceManager, artifactManager, statsRegistry, healthMonitor, webServer)
	startAdminControllers(serverCfg, repoFactory, userManager, jobManager,
		retentionManager, dashboardStats, resourceManager, artifactManager, statsRegistry,
		healthMonitor, authProviders, webServer)

	svcs := buildServices(serverCfg, repoFactory, userManager, jobManager,
		dashboardStats, artifactManager)

	grpcSrv := buildGRPCServer(serverCfg, repoFactory, svcs)

	// Start TriggerManager (leader-aware: activates S3/queue triggers only on scheduler leader).
	triggerMgr, err := startTriggerManager(ctx, serverCfg, repoFactory, jobManager, queueClient, webServer)
	if err != nil {
		return fmt.Errorf("trigger manager start failed: %w", err)
	}
	if triggerMgr != nil {
		go func() {
			<-ctx.Done()
			triggerMgr.Stop(context.Background())
		}()
	}

	// Determine auth parameters (same logic as buildGRPCServer).
	jwtSecret := ""
	cookieName := ""
	if serverCfg.Common.Auth != nil && serverCfg.Common.Auth.Enabled {
		jwtSecret = serverCfg.Common.Auth.JWTSecret
		cookieName = serverCfg.Common.Auth.CookieName
	}

	// Register all grpc-gateway REST handlers on /api/v1/* routes.
	gwMux := internalGrpc.NewGatewayMux()
	for _, reg := range []func() error{
		func() error { return svcpb.RegisterJobDefinitionServiceHandlerServer(ctx, gwMux, svcs.jobDef) },
		func() error { return svcpb.RegisterJobExecutionServiceHandlerServer(ctx, gwMux, svcs.jobExec) },
		func() error { return svcpb.RegisterUserServiceHandlerServer(ctx, gwMux, svcs.user) },
		func() error { return svcpb.RegisterOrganizationServiceHandlerServer(ctx, gwMux, svcs.org) },
		func() error { return svcpb.RegisterArtifactServiceHandlerServer(ctx, gwMux, svcs.artifact) },
		func() error { return svcpb.RegisterConfigServiceHandlerServer(ctx, gwMux, svcs.config) },
		func() error { return svcpb.RegisterResourceServiceHandlerServer(ctx, gwMux, svcs.resource) },
		func() error { return svcpb.RegisterAuditServiceHandlerServer(ctx, gwMux, svcs.audit) },
		func() error { return svcpb.RegisterErrorCodeServiceHandlerServer(ctx, gwMux, svcs.errorCode) },
		func() error { return svcpb.RegisterJobResourceServiceHandlerServer(ctx, gwMux, svcs.jobRes) },
		func() error { return svcpb.RegisterHealthServiceHandlerServer(ctx, gwMux, svcs.health) },
		func() error { return svcpb.RegisterAdminServiceHandlerServer(ctx, gwMux, svcs.admin) },
		func() error { return svcpb.RegisterTriggerServiceHandlerServer(ctx, gwMux, svcs.triggers) },
	} {
		if err := reg(); err != nil {
			return fmt.Errorf("grpc-gateway registration failed: %w", err)
		}
	}

	// Wrap gateway mux with HTTP auth middleware.
	// RegisterHandlerServer bypasses gRPC interceptors (in-process call), so we must
	// authenticate at the HTTP layer and inject User/QueryContext into the request context.
	// The gateway propagates the HTTP request context to service handler methods.
	authMiddleware := interceptors.GatewayAuthMiddleware(
		jwtSecret, cookieName,
		&dbUserLoader{repo: repoFactory.UserRepository},
		"/api/v1/health",
		"/api/v1/ping",
	)
	var gwHandler http.Handler = gwMux
	gwHandler = authMiddleware(gwHandler)

	// Mount gateway at root level (bypasses Echo's apiGroup JWT middleware — auth handled above).
	webServer.RegisterRootHandler("/api/v1/", gwHandler)

	return startCmux(ctx, serverCfg, grpcSrv, webServer)
}

// buildServices constructs all gRPC service implementations.
func buildServices(
	serverCfg *config.ServerConfig,
	repoFactory *repository.Locator,
	userManager *manager.UserManager,
	jobManager *manager.JobManager,
	dashboardStats *manager.DashboardManager,
	artifactManager *manager.ArtifactManager,
) *services {
	triggerEvaluator := trigger.NewEvaluator(repoFactory.TriggerStateRepository)
	triggerSubmitter := trigger.NewSubmitter(jobManager, repoFactory.TriggerStateRepository)
	return &services{
		jobDef:    queenService.NewJobDefinitionService(jobManager),
		jobExec:   queenService.NewJobExecutionService(jobManager),
		user:      queenService.NewUserService(userManager, repoFactory.UserRepository, serverCfg),
		org:       queenService.NewOrgService(userManager, repoFactory.ConfigRepository, repoFactory.AuditRecordRepository),
		artifact:  queenService.NewArtifactService(artifactManager),
		config:    queenService.NewConfigService(repoFactory.SystemConfigRepository, repoFactory.JobDefinitionRepository, jobManager),
		resource:  queenService.NewResourceService(dashboardStats, repoFactory.SubscriptionRepository),
		audit:     queenService.NewAuditService(repoFactory.AuditRecordRepository),
		errorCode: queenService.NewErrorCodeService(repoFactory.ErrorCodeRepository),
		jobRes:    queenService.NewJobResourceService(repoFactory.JobResourceRepository),
		health:    queenService.NewHealthService(dashboardStats),
		admin:     queenService.NewAdminService(dashboardStats, userManager),
		triggers:  queenService.NewTriggerService(jobManager, repoFactory.TriggerStateRepository, triggerEvaluator, triggerSubmitter),
	}
}

// buildGRPCServer wires all service implementations onto a gRPC server.
func buildGRPCServer(
	serverCfg *config.ServerConfig,
	repoFactory *repository.Locator,
	svcs *services,
) *grpc.Server {
	// Pass empty secret when auth is disabled — interceptor treats this as
	// anonymous admin mode (dev/test only).
	jwtSecret := ""
	cookieName := ""
	if serverCfg.Common.Auth != nil && serverCfg.Common.Auth.Enabled {
		jwtSecret = serverCfg.Common.Auth.JWTSecret
		cookieName = serverCfg.Common.Auth.CookieName
	}

	grpcSrv := internalGrpc.NewServer(internalGrpc.ServerConfig{
		JWTSecret:          jwtSecret,
		CookieName:         cookieName,
		RateLimitPerSecond: serverCfg.Common.RateLimitPerSecond,
		RequestTimeout:     30 * time.Second,
		MethodPermissions:  buildMethodPermissions(),
		UserLoader:         &dbUserLoader{repo: repoFactory.UserRepository},
		SkipAuthMethods: []string{
			svcpb.HealthService_Ping_FullMethodName,
			svcpb.HealthService_GetHealth_FullMethodName,
		},
	})

	svcpb.RegisterJobDefinitionServiceServer(grpcSrv, svcs.jobDef)
	svcpb.RegisterJobExecutionServiceServer(grpcSrv, svcs.jobExec)
	svcpb.RegisterUserServiceServer(grpcSrv, svcs.user)
	svcpb.RegisterOrganizationServiceServer(grpcSrv, svcs.org)
	svcpb.RegisterArtifactServiceServer(grpcSrv, svcs.artifact)
	svcpb.RegisterConfigServiceServer(grpcSrv, svcs.config)
	svcpb.RegisterResourceServiceServer(grpcSrv, svcs.resource)
	svcpb.RegisterAuditServiceServer(grpcSrv, svcs.audit)
	svcpb.RegisterErrorCodeServiceServer(grpcSrv, svcs.errorCode)
	svcpb.RegisterJobResourceServiceServer(grpcSrv, svcs.jobRes)
	svcpb.RegisterHealthServiceServer(grpcSrv, svcs.health)
	svcpb.RegisterAdminServiceServer(grpcSrv, svcs.admin)
	svcpb.RegisterTriggerServiceServer(grpcSrv, svcs.triggers)
	if serverCfg.Common.Debug {
		reflection.Register(grpcSrv)
	}
	return grpcSrv
}

// startCmux binds a single TCP listener on HTTPPort and dispatches gRPC vs HTTP.
func startCmux(
	ctx context.Context,
	serverCfg *config.ServerConfig,
	grpcSrv *grpc.Server,
	webServer web.Server,
) error {
	addr := ":" + strconv.Itoa(serverCfg.Common.HTTPPort)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	mux := cmux.New(lis)
	grpcLis := mux.MatchWithWriters(
		cmux.HTTP2MatchHeaderFieldSendSettings("content-type", "application/grpc"),
	)
	httpLis := mux.Match(cmux.Any())

	go func() {
		logrus.WithField("addr", addr).Info("gRPC server starting")
		if err := grpcSrv.Serve(grpcLis); err != nil {
			logrus.WithError(err).Error("gRPC server stopped unexpectedly")
		}
	}()

	go func() {
		logrus.WithField("addr", addr).Info("HTTP server starting")
		webServer.StartWithListener(httpLis)
	}()

	go func() {
		<-ctx.Done()
		logrus.Info("shutting down gRPC server")
		grpcSrv.GracefulStop()
	}()

	// mux.Serve() blocks until the listener is closed. This keeps the calling
	// goroutine (and therefore the process) alive for the server's lifetime.
	logrus.WithField("addr", addr).Infof("server listening (gRPC + HTTP on same port via cmux)")
	if err := mux.Serve(); err != nil {
		logrus.WithError(err).Info("cmux stopped")
	}
	return nil
}

// buildMethodPermissions maps each gRPC full method name to the ACL permission required.
func buildMethodPermissions() map[string]*acl.Permission {
	p := make(map[string]*acl.Permission)

	// Job definitions
	for _, m := range []string{
		svcpb.JobDefinitionService_QueryJobDefinitions_FullMethodName,
		svcpb.JobDefinitionService_QueryPlugins_FullMethodName,
		svcpb.JobDefinitionService_GetJobDefinition_FullMethodName,
		svcpb.JobDefinitionService_GetJobDefinitionYAML_FullMethodName,
		svcpb.JobDefinitionService_GetJobDefinitionMermaid_FullMethodName,
		svcpb.JobDefinitionService_GetJobDefinitionStats_FullMethodName,
	} {
		p[m] = acl.NewPermission(acl.JobDefinition, acl.View)
	}
	for _, m := range []string{
		svcpb.JobDefinitionService_CreateJobDefinition_FullMethodName,
		svcpb.JobDefinitionService_UpdateJobDefinition_FullMethodName,
		svcpb.JobDefinitionService_EnableJobDefinition_FullMethodName,
		svcpb.JobDefinitionService_DisableJobDefinition_FullMethodName,
		svcpb.JobDefinitionService_UpdateConcurrency_FullMethodName,
	} {
		p[m] = acl.NewPermission(acl.JobDefinition, acl.Write)
	}
	p[svcpb.JobDefinitionService_DeleteJobDefinition_FullMethodName] = acl.NewPermission(acl.JobDefinition, acl.Delete)

	// Job executions / requests
	for _, m := range []string{
		svcpb.JobExecutionService_QueryJobRequests_FullMethodName,
		svcpb.JobExecutionService_GetJobRequest_FullMethodName,
		svcpb.JobExecutionService_GetJobExecution_FullMethodName,
		svcpb.JobExecutionService_GetJobRequestMermaid_FullMethodName,
		svcpb.JobExecutionService_GetJobStats_FullMethodName,
		svcpb.JobExecutionService_GetJobWaitTime_FullMethodName,
	} {
		p[m] = acl.NewPermission(acl.JobRequest, acl.View)
	}
	for _, m := range []string{
		svcpb.JobExecutionService_SubmitJob_FullMethodName,
		svcpb.JobExecutionService_TriggerJob_FullMethodName,
		svcpb.JobExecutionService_RestartJob_FullMethodName,
		svcpb.JobExecutionService_PauseJob_FullMethodName,
	} {
		p[m] = acl.NewPermission(acl.JobRequest, acl.Execute)
	}
	p[svcpb.JobExecutionService_VoteOnApproval_FullMethodName] = acl.NewPermission(acl.JobRequest, acl.Approve)
	for _, m := range []string{
		svcpb.JobExecutionService_GetApprovalStatus_FullMethodName,
		svcpb.JobExecutionService_ListPendingApprovals_FullMethodName,
	} {
		p[m] = acl.NewPermission(acl.JobRequest, acl.View)
	}
	p[svcpb.JobExecutionService_CancelJob_FullMethodName] = acl.NewPermission(acl.JobRequest, acl.Delete)

	// Job resources
	for _, m := range []string{
		svcpb.JobResourceService_QueryJobResources_FullMethodName,
		svcpb.JobResourceService_GetJobResource_FullMethodName,
	} {
		p[m] = acl.NewPermission(acl.JobResource, acl.View)
	}
	p[svcpb.JobResourceService_SaveJobResource_FullMethodName] = acl.NewPermission(acl.JobResource, acl.Write)
	p[svcpb.JobResourceService_DeleteJobResource_FullMethodName] = acl.NewPermission(acl.JobResource, acl.Delete)

	// Users
	for _, m := range []string{
		svcpb.UserService_QueryUsers_FullMethodName,
		svcpb.UserService_GetUser_FullMethodName,
		svcpb.UserService_GetProfile_FullMethodName,
	} {
		p[m] = acl.NewPermission(acl.User, acl.View)
	}
	p[svcpb.UserService_CreateUser_FullMethodName] = acl.NewPermission(acl.User, acl.Write)
	p[svcpb.UserService_UpdateUser_FullMethodName] = acl.NewPermission(acl.User, acl.Write)
	p[svcpb.UserService_DeleteUser_FullMethodName] = acl.NewPermission(acl.User, acl.Delete)

	// Organizations (org CRUD — admin only)
	for _, m := range []string{
		svcpb.OrganizationService_QueryOrgs_FullMethodName,
		svcpb.OrganizationService_GetOrg_FullMethodName,
	} {
		p[m] = acl.NewPermission(acl.Organization, acl.View)
	}
	for _, m := range []string{
		svcpb.OrganizationService_CreateOrg_FullMethodName,
		svcpb.OrganizationService_UpdateOrg_FullMethodName,
	} {
		p[m] = acl.NewPermission(acl.Organization, acl.Write)
	}
	p[svcpb.OrganizationService_DeleteOrg_FullMethodName] = acl.NewPermission(acl.Organization, acl.Delete)

	// Org configs — gated on OrgConfig resource (OrgAdmin or Admin)
	p[svcpb.OrganizationService_QueryOrgConfigs_FullMethodName] = acl.NewPermission(acl.OrgConfig, acl.View)
	p[svcpb.OrganizationService_GetOrgConfig_FullMethodName] = acl.NewPermission(acl.OrgConfig, acl.View)
	p[svcpb.OrganizationService_RevealOrgConfig_FullMethodName] = acl.NewPermission(acl.OrgConfig, acl.Update)
	p[svcpb.OrganizationService_SaveOrgConfig_FullMethodName] = acl.NewPermission(acl.OrgConfig, acl.Write)
	p[svcpb.OrganizationService_DeleteOrgConfig_FullMethodName] = acl.NewPermission(acl.OrgConfig, acl.Delete)

	// User configs — any authenticated user owns their own configs
	p[svcpb.OrganizationService_QueryUserConfigs_FullMethodName] = acl.NewPermission(acl.UserConfig, acl.Query)
	p[svcpb.OrganizationService_GetUserConfig_FullMethodName] = acl.NewPermission(acl.UserConfig, acl.View)
	p[svcpb.OrganizationService_RevealUserConfig_FullMethodName] = acl.NewPermission(acl.UserConfig, acl.Update)
	p[svcpb.OrganizationService_SaveUserConfig_FullMethodName] = acl.NewPermission(acl.UserConfig, acl.Write)
	p[svcpb.OrganizationService_DeleteUserConfig_FullMethodName] = acl.NewPermission(acl.UserConfig, acl.Delete)

	// System / job configs
	for _, m := range []string{
		svcpb.ConfigService_QuerySystemConfigs_FullMethodName,
		svcpb.ConfigService_GetSystemConfig_FullMethodName,
		svcpb.ConfigService_QueryJobConfigs_FullMethodName,
	} {
		p[m] = acl.NewPermission(acl.SystemConfig, acl.View)
	}
	for _, m := range []string{
		svcpb.ConfigService_SaveSystemConfig_FullMethodName,
		svcpb.ConfigService_SaveJobConfig_FullMethodName,
	} {
		p[m] = acl.NewPermission(acl.SystemConfig, acl.Write)
	}
	for _, m := range []string{
		svcpb.ConfigService_DeleteSystemConfig_FullMethodName,
		svcpb.ConfigService_DeleteJobConfig_FullMethodName,
	} {
		p[m] = acl.NewPermission(acl.SystemConfig, acl.Delete)
	}

	// Error codes
	for _, m := range []string{
		svcpb.ErrorCodeService_QueryErrorCodes_FullMethodName,
		svcpb.ErrorCodeService_GetErrorCode_FullMethodName,
	} {
		p[m] = acl.NewPermission(acl.ErrorCode, acl.View)
	}
	p[svcpb.ErrorCodeService_SaveErrorCode_FullMethodName] = acl.NewPermission(acl.ErrorCode, acl.Write)
	p[svcpb.ErrorCodeService_DeleteErrorCode_FullMethodName] = acl.NewPermission(acl.ErrorCode, acl.Delete)

	// Artifacts
	for _, m := range []string{
		svcpb.ArtifactService_QueryArtifacts_FullMethodName,
		svcpb.ArtifactService_GetArtifact_FullMethodName,
	} {
		p[m] = acl.NewPermission(acl.Artifact, acl.View)
	}
	p[svcpb.ArtifactService_DeleteArtifact_FullMethodName] = acl.NewPermission(acl.Artifact, acl.Delete)

	// Resources / ant registrations
	for _, m := range []string{
		svcpb.ResourceService_QueryAntRegistrations_FullMethodName,
		svcpb.ResourceService_GetAntRegistration_FullMethodName,
		svcpb.ResourceService_QuerySubscriptions_FullMethodName,
		svcpb.ResourceService_GetSubscription_FullMethodName,
	} {
		p[m] = acl.NewPermission(acl.AntExecutor, acl.View)
	}
	p[svcpb.ResourceService_SaveSubscription_FullMethodName] = acl.NewPermission(acl.AntExecutor, acl.Write)

	// Audit
	p[svcpb.AuditService_QueryAuditRecords_FullMethodName] = acl.NewPermission(acl.Audit, acl.View)

	// Dashboard stats (admin-only)
	p[svcpb.AdminService_GetDashboardStats_FullMethodName] = acl.NewPermission(acl.Dashboard, acl.View)

	// Trigger management
	p[svcpb.TriggerService_ListTriggerStates_FullMethodName] = acl.NewPermission(acl.JobDefinition, acl.View)
	p[svcpb.TriggerService_ResetTriggerState_FullMethodName] = acl.NewPermission(acl.JobDefinition, acl.Write)
	p[svcpb.TriggerService_FireWebhookTrigger_FullMethodName] = acl.NewPermission(acl.JobRequest, acl.Execute)

	return p
}

// dbUserLoader implements interceptors.UserLoader using the user repository.
type dbUserLoader struct {
	repo repository.UserRepository
}

func (l *dbUserLoader) GetUserByUsername(_ context.Context, username string) (*commonTypes.User, error) {
	qc := commonTypes.NewQueryContext(nil, "")
	return l.repo.GetByUsername(qc, username)
}

func startTriggerManager(
	ctx context.Context,
	serverCfg *config.ServerConfig,
	repoFactory *repository.Locator,
	jobManager *manager.JobManager,
	queueClient queue.Client,
	webServer web.Server,
) (*trigger.Manager, error) {
	if serverCfg.Jobs.DisableTriggers {
		return nil, nil
	}
	evaluator := trigger.NewEvaluator(repoFactory.TriggerStateRepository)
	submitter := trigger.NewSubmitter(jobManager, repoFactory.TriggerStateRepository)
	webhookHandler := trigger.NewWebhookHandler(
		jobManager, evaluator, submitter,
		serverCfg.Jobs.TriggerWebhookBodyMaxBytes,
		webServer,
	)
	mgr := trigger.New(
		serverCfg,
		queueClient,
		jobManager,
		repoFactory.TriggerStateRepository,
		evaluator,
		submitter,
		webhookHandler,
	)
	return mgr, mgr.Start(ctx)
}

// ///////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////

func startWebhookProcessor(
	serverCfg *config.ServerConfig,
	queueClient queue.Client,
	httpClient web.HTTPClient,
) error {
	return webhook.New(serverCfg, queueClient, httpClient).Start(context.Background())
}

func startNewWebsocketProxyRegistry(
	serverCfg *config.ServerConfig,
	resourceManager resource.Manager,
	requestRegistry tasklet.RequestRegistry,
	artifactManager *manager.ArtifactManager,
	queueClient queue.Client,
	requestTopic string,
	webServer web.Server) error {
	return wstask.NewWebsocketProxyRegistry(
		serverCfg,
		resourceManager,
		requestRegistry,
		artifactManager,
		queueClient,
		requestTopic,
		webServer).Start(context.Background())
}

func startWebsocketGateway(
	serverCfg *config.ServerConfig,
	queueClient queue.Client,
	logsArchiver repository.LogEventRepository,
	webServer web.Server) error {
	return gateway.New(serverCfg, queueClient, logsArchiver, webServer).Start(context.Background())
}

func startControllers(
	cfg *config.ServerConfig,
	repoFactory *repository.Locator,
	userManager *manager.UserManager,
	jobManager *manager.JobManager,
	resourceManager resource.Manager,
	artifactManager *manager.ArtifactManager,
	statsRegistry *stats.JobStatsRegistry,
	healthMonitor *health.Monitor,
	webServer web.Server) {
	if cfg.Common.Debug {
		controller.NewProfileStatsController(&cfg.Common, webServer)
	}
	controller.NewIndexController(webServer)
	controller.NewAuditController(repoFactory.AuditRecordRepository, webServer)
	controller.NewUserController(userManager, webServer)
	controller.NewOrganizationController(userManager, webServer)
	controller.NewOrganizationConfigController(repoFactory.AuditRecordRepository, repoFactory.ConfigRepository, webServer)
	controller.NewUserConfigController(repoFactory.AuditRecordRepository, repoFactory.ConfigRepository, webServer)
	controller.NewJobDefinitionController(jobManager, statsRegistry, webServer)
	controller.NewJobConfigController(repoFactory.AuditRecordRepository, repoFactory.JobDefinitionRepository, webServer)
	controller.NewJobResourceController(repoFactory.AuditRecordRepository, repoFactory.JobResourceRepository, webServer)
	controller.NewSystemConfigController(repoFactory.SystemConfigRepository, webServer)
	controller.NewErrorCodeController(repoFactory.ErrorCodeRepository, webServer)
	controller.NewJobRequestController(jobManager, webServer)
	controller.NewAntRegistrationController(resourceManager, webServer)
	controller.NewArtifactController(artifactManager, webServer)
	controller.NewContainerExecutionController(resourceManager, webServer)
	controller.NewHealthController(healthMonitor, webServer)
	controller.NewSubscriptionController(
		repoFactory.SubscriptionRepository,
		repoFactory.UserRepository,
		repoFactory.OrgRepository,
		repoFactory.AuditRecordRepository,
		webServer)
	controller.NewEmailVerificationController(userManager, webServer)
}

func startAdminControllers(
	serverCfg *config.ServerConfig,
	repoFactory *repository.Locator,
	userManager *manager.UserManager,
	jobManager *manager.JobManager,
	retentionManager *manager.RetentionManager,
	dashboardStats *manager.DashboardManager,
	resourceManager resource.Manager,
	artifactManager *manager.ArtifactManager,
	statsRegistry *stats.JobStatsRegistry,
	healthMonitor *health.Monitor,
	authProviders []auth.Provider,
	webServer web.Server) {
	admin.NewAuditAdminController(repoFactory.AuditRecordRepository, repoFactory.JobRequestRepository, webServer)
	admin.NewAuthController(
		&serverCfg.Common,
		authProviders,
		repoFactory.AuditRecordRepository,
		repoFactory.UserRepository,
		repoFactory.OrgRepository,
		userManager,
		webServer)
	admin.NewUserAdminController(
		&serverCfg.Common, userManager,
		repoFactory.JobExecutionRepository, repoFactory.ArtifactRepository, webServer)
	admin.NewOrganizationConfigAdminController(repoFactory.AuditRecordRepository, repoFactory.ConfigRepository, webServer)
	admin.NewUserConfigAdminController(repoFactory.AuditRecordRepository, repoFactory.ConfigRepository, webServer)
	admin.NewOrganizationAdminController(userManager, webServer)
	admin.NewInvitationAdminController(userManager, webServer)
	admin.NewJobDefinitionAdminController(jobManager, resourceManager, statsRegistry, repoFactory.TriggerStateRepository, webServer)
	admin.NewJobConfigAdminController(repoFactory.AuditRecordRepository, repoFactory.JobDefinitionRepository, webServer)
	admin.NewJobResourceAdminController(repoFactory.AuditRecordRepository, repoFactory.JobResourceRepository, webServer)
	admin.NewSystemConfigAdminController(repoFactory.SystemConfigRepository, webServer)
	admin.NewErrorCodeAdminController(repoFactory.ErrorCodeRepository, webServer)
	admin.NewJobRequestAdminController(jobManager, webServer)
	admin.NewAntAdminController(resourceManager, webServer)
	admin.NewArtifactAdminController(artifactManager, webServer)
	admin.NewDashboardAdminController(dashboardStats, webServer)
	admin.NewExecutionContainerAdminController(resourceManager, webServer)
	admin.NewHealthAdminController(healthMonitor, webServer)
	admin.NewEmailVerificationAdminController(userManager, webServer)
	admin.NewRetentionAdminController(retentionManager, webServer)
}
