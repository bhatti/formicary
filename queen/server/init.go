package server

import (
	"context"
	"plexobject.com/formicary/internal/auth"
	"plexobject.com/formicary/internal/health"
	"plexobject.com/formicary/internal/queue"
	"plexobject.com/formicary/internal/tasklet"
	"plexobject.com/formicary/internal/web"
	"plexobject.com/formicary/queen/config"
	"plexobject.com/formicary/queen/controller"
	"plexobject.com/formicary/queen/controller/admin"
	"plexobject.com/formicary/queen/gateway"
	"plexobject.com/formicary/queen/manager"
	"plexobject.com/formicary/queen/repository"
	"plexobject.com/formicary/queen/resource"
	"plexobject.com/formicary/queen/security"
	"plexobject.com/formicary/queen/stats"
	"plexobject.com/formicary/queen/tasklet/wstask"
	"plexobject.com/formicary/queen/webhook"
	"strconv"
)

// StartWebServer starts controllers that register REST APIs and admin dashboard
func StartWebServer(
	_ context.Context,
	serverCfg *config.ServerConfig,
	repoFactory *repository.Locator,
	userManager *manager.UserManager,
	jobManager *manager.JobManager,
	dashboardStats *manager.DashboardManager,
	resourceManager resource.Manager,
	requestRegistry tasklet.RequestRegistry,
	artifactManager *manager.ArtifactManager,
	statsRegistry *stats.JobStatsRegistry,
	heathMonitor *health.Monitor,
	queueClient queue.Client,
	webServer web.Server,
	http web.HTTPClient,
) error {
	authProviders := make([]auth.Provider, 0)
	if googleAuthProvider, err := security.NewGoogleAuth(&serverCfg.CommonConfig); err == nil {
		authProviders = append(authProviders, googleAuthProvider)
	}

	if githubAuthProvider, err := security.NewGithubAuth(&serverCfg.CommonConfig, jobManager.BuildGithubPostWebhookHandler()); err == nil {
		authProviders = append(authProviders, githubAuthProvider)
	}

	if err := startWebsocketGateway(serverCfg, queueClient, repoFactory.LogEventRepository, webServer); err != nil {
		return err
	}

	if err := startNewWebsocketProxyRegistry(
		serverCfg,
		resourceManager,
		requestRegistry,
		artifactManager,
		queueClient,
		serverCfg.GetWebsocketTaskletTopic(),
		webServer); err != nil {
		return err
	}

	if err := startWebhookProcessor(serverCfg, queueClient, http); err != nil {
		return err
	}

	startControllers(
		serverCfg,
		repoFactory,
		userManager,
		jobManager,
		resourceManager,
		artifactManager,
		statsRegistry,
		heathMonitor,
		webServer)

	startAdminControllers(
		serverCfg,
		repoFactory,
		userManager,
		jobManager,
		dashboardStats,
		resourceManager,
		artifactManager,
		statsRegistry,
		heathMonitor,
		authProviders,
		webServer)

	webServer.Start(":" + strconv.Itoa(serverCfg.CommonConfig.HTTPPort))
	return nil
}

/////////////////////////////////////////// PRIVATE METHODS ////////////////////////////////////////////
func startWebhookProcessor(
	serverCfg *config.ServerConfig,
	queueClient queue.Client,
	http web.HTTPClient,
) error {
	return webhook.New(serverCfg, queueClient, http).Start(context.Background())
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
	_ *config.ServerConfig,
	repoFactory *repository.Locator,
	userManager *manager.UserManager,
	jobManager *manager.JobManager,
	resourceManager resource.Manager,
	artifactManager *manager.ArtifactManager,
	statsRegistry *stats.JobStatsRegistry,
	heathMonitor *health.Monitor,
	webServer web.Server) {
	controller.NewIndexController(webServer)
	controller.NewAuditController(
		repoFactory.AuditRecordRepository,
		webServer)
	controller.NewUserController(
		userManager,
		webServer)
	controller.NewOrganizationController(
		userManager,
		webServer)
	controller.NewOrganizationConfigController(
		repoFactory.AuditRecordRepository,
		repoFactory.OrgConfigRepository,
		webServer)
	controller.NewJobDefinitionController(jobManager,
		statsRegistry,
		webServer)
	controller.NewJobConfigController(
		repoFactory.AuditRecordRepository,
		repoFactory.JobDefinitionRepository,
		webServer)
	controller.NewJobResourceController(
		repoFactory.AuditRecordRepository,
		repoFactory.JobResourceRepository,
		webServer)
	controller.NewSystemConfigController(
		repoFactory.SystemConfigRepository,
		webServer)
	controller.NewErrorCodeController(
		repoFactory.ErrorCodeRepository,
		webServer)
	controller.NewJobRequestController(
		jobManager,
		webServer)
	controller.NewAntRegistrationController(
		resourceManager,
		webServer)
	controller.NewArtifactController(
		artifactManager,
		webServer)
	controller.NewContainerExecutionController(
		resourceManager,
		webServer)
	controller.NewHealthController(
		heathMonitor,
		webServer)
	controller.NewSubscriptionController(
		repoFactory.SubscriptionRepository,
		repoFactory.UserRepository,
		repoFactory.OrgRepository,
		repoFactory.AuditRecordRepository,
		webServer)
	controller.NewEmailVerificationController(
		userManager,
		webServer)
}

func startAdminControllers(
	serverCfg *config.ServerConfig,
	repoFactory *repository.Locator,
	userManager *manager.UserManager,
	jobManager *manager.JobManager,
	dashboardStats *manager.DashboardManager,
	resourceManager resource.Manager,
	artifactManager *manager.ArtifactManager,
	statsRegistry *stats.JobStatsRegistry,
	heathMonitor *health.Monitor,
	authProviders []auth.Provider,
	webServer web.Server) {
	admin.NewAuditAdminController(repoFactory.AuditRecordRepository, webServer)
	admin.NewAuthController(
		&serverCfg.CommonConfig,
		authProviders,
		repoFactory.AuditRecordRepository,
		repoFactory.UserRepository,
		repoFactory.OrgRepository,
		webServer)
	admin.NewUserAdminController(
		&serverCfg.CommonConfig,
		userManager,
		repoFactory.JobExecutionRepository,
		repoFactory.ArtifactRepository,
		webServer)
	admin.NewOrganizationConfigAdminController(
		repoFactory.AuditRecordRepository,
		repoFactory.OrgConfigRepository,
		webServer)
	admin.NewOrganizationAdminController(
		userManager,
		webServer)
	admin.NewInvitationAdminController(
		userManager,
		webServer)
	admin.NewJobDefinitionAdminController(
		jobManager,
		resourceManager,
		statsRegistry,
		webServer)
	admin.NewJobConfigAdminController(
		repoFactory.AuditRecordRepository,
		repoFactory.JobDefinitionRepository,
		webServer)
	admin.NewJobResourceAdminController(
		repoFactory.AuditRecordRepository,
		repoFactory.JobResourceRepository, webServer)
	admin.NewSystemConfigAdminController(
		repoFactory.SystemConfigRepository, webServer)
	admin.NewErrorCodeAdminController(
		repoFactory.ErrorCodeRepository, webServer)
	admin.NewJobRequestAdminController(jobManager, webServer)
	admin.NewAntAdminController(resourceManager, webServer)
	admin.NewArtifactAdminController(artifactManager, webServer)
	admin.NewDashboardAdminController(dashboardStats, webServer)
	admin.NewExecutionContainerAdminController(resourceManager, webServer)
	admin.NewHealthAdminController(heathMonitor, webServer)
	admin.NewEmailVerificationAdminController(userManager, webServer)
}
