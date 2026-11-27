package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	ratelimit "github.com/Wei-Shaw/sub2api/internal/middleware"
	"github.com/Wei-Shaw/sub2api/internal/securityaudit"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/google/wire"
)

// ProvideAdminHandlers creates the AdminHandlers struct
func ProvideAdminHandlers(
	requestCaptureHandler *admin.RequestCaptureHandler,
	dashboardHandler *admin.DashboardHandler,
	userHandler *admin.UserHandler,
	groupHandler *admin.GroupHandler,
	accountHandler *admin.AccountHandler,
	openaiOAuthReauthService *service.OpenAIOAuthReauthService,
	announcementHandler *admin.AnnouncementHandler,
	dataManagementHandler *admin.DataManagementHandler,
	backupHandler *admin.BackupHandler,
	oauthHandler *admin.OAuthHandler,
	openaiOAuthHandler *admin.OpenAIOAuthHandler,
	openaiReauthHandler *admin.OpenAIOAuthReauthHandler,
	geminiOAuthHandler *admin.GeminiOAuthHandler,
	antigravityOAuthHandler *admin.AntigravityOAuthHandler,
	grokOAuthHandler *admin.GrokOAuthHandler,
	cnProviderHandler *admin.CNProviderHandler,
	proxyHandler *admin.ProxyHandler,
	redeemHandler *admin.RedeemHandler,
	promoHandler *admin.PromoHandler,
	settingHandler *admin.SettingHandler,
	opsHandler *admin.OpsHandler,
	accountHealthHandler *admin.AccountHealthHandler,
	systemHandler *admin.SystemHandler,
	subscriptionHandler *admin.SubscriptionHandler,
	usageHandler *admin.UsageHandler,
	userAttributeHandler *admin.UserAttributeHandler,
	errorPassthroughHandler *admin.ErrorPassthroughHandler,
	tlsFingerprintProfileHandler *admin.TLSFingerprintProfileHandler,
	pluginHandler *admin.PluginHandler,
	openaiIPv6Handler *admin.OpenAIIPv6Handler,
	apiKeyHandler *admin.AdminAPIKeyHandler,
	scheduledTestHandler *admin.ScheduledTestHandler,
	channelQualityHandler *admin.ChannelQualityHandler,
	pelicanGroupTestHandler *admin.PelicanGroupTestHandler,
	controlledExperimentHandler *admin.ControlledExperimentHandler,
	accountOpsHandler *admin.AccountOpsHandler,
	openAIReloginHandler *admin.OpenAIReloginHandler,
	accountTokenGuardV2Handler *admin.AccountTokenGuardV2Handler,
	channelHandler *admin.ChannelHandler,
	groupProbeHandler *admin.GroupProbeHandler,
	channelMonitorHandler *admin.ChannelMonitorHandler,
	channelMonitorTemplateHandler *admin.ChannelMonitorRequestTemplateHandler,
	contentModerationHandler *admin.ContentModerationHandler,
	promptAuditHandler *securityaudit.PromptAdminHandler,
	paymentHandler *admin.PaymentHandler,
	affiliateHandler *admin.AffiliateHandler,
	complianceHandler *admin.ComplianceHandler,
	auditLogHandler *admin.AuditLogHandler,
	upstreamBillingProbe *service.UpstreamBillingProbeService,
	ollamaCloudUsage *service.OllamaCloudUsageService,
	opencodeGoUsage *service.OpenCodeGoUsageService,
	settingService *service.SettingService,
	openAIGateway *service.OpenAIGatewayService,
	claudeResetCredits *service.ClaudeResetCreditService,
	codexHarvest *service.CodexHarvestService,
) *AdminHandlers {
	accountHandler.SetCodexHarvestService(codexHarvest)
	accountHandler.SetOpenAIGatewayService(openAIGateway)
	accountHandler.SetUpstreamBillingProbeService(upstreamBillingProbe)
	accountHandler.SetOllamaCloudUsageService(ollamaCloudUsage)
	accountHandler.SetCodexTicketSettings(settingService)
	accountHandler.SetCodexTicketGateway(openAIGateway)
	accountHandler.SetOpenCodeGoUsageService(opencodeGoUsage)
	accountHandler.SetClaudeResetCreditService(claudeResetCredits)
	accountHandler.SetOpenAIOAuthReauthService(openaiOAuthReauthService)
	return &AdminHandlers{
		ChannelQuality:         channelQualityHandler,
		RequestCapture:         requestCaptureHandler,
		Dashboard:              dashboardHandler,
		User:                   userHandler,
		Group:                  groupHandler,
		Account:                accountHandler,
		Announcement:           announcementHandler,
		DataManagement:         dataManagementHandler,
		Backup:                 backupHandler,
		OAuth:                  oauthHandler,
		OpenAIOAuth:            openaiOAuthHandler,
		OpenAIReauth:           openaiReauthHandler,
		GeminiOAuth:            geminiOAuthHandler,
		AntigravityOAuth:       antigravityOAuthHandler,
		GrokOAuth:              grokOAuthHandler,
		CNProvider:             cnProviderHandler,
		Proxy:                  proxyHandler,
		Redeem:                 redeemHandler,
		Promo:                  promoHandler,
		Setting:                settingHandler,
		Ops:                    opsHandler,
		AccountHealth:          accountHealthHandler,
		System:                 systemHandler,
		Subscription:           subscriptionHandler,
		Usage:                  usageHandler,
		UserAttribute:          userAttributeHandler,
		ErrorPassthrough:       errorPassthroughHandler,
		TLSFingerprintProfile:  tlsFingerprintProfileHandler,
		Plugin:                 pluginHandler,
		OpenAIIPv6:             openaiIPv6Handler,
		APIKey:                 apiKeyHandler,
		AccountOps:             accountOpsHandler,
		OpenAIRelogin:          openAIReloginHandler,
		AccountTokenGuardV2:    accountTokenGuardV2Handler,
		ScheduledTest:          scheduledTestHandler,
		PelicanGroupTest:       pelicanGroupTestHandler,
		ControlledExperiment:   controlledExperimentHandler,
		Channel:                channelHandler,
		GroupProbe:             groupProbeHandler,
		ChannelMonitor:         channelMonitorHandler,
		ChannelMonitorTemplate: channelMonitorTemplateHandler,
		ContentModeration:      contentModerationHandler,
		PromptAudit:            promptAuditHandler,
		Payment:                paymentHandler,
		Affiliate:              affiliateHandler,
		Compliance:             complianceHandler,
		AuditLog:               auditLogHandler,
	}
}

func ProvideGatewayHandler(
	gatewayService *service.GatewayService,
	openAIGatewayService *service.OpenAIGatewayService,
	geminiCompatService *service.GeminiMessagesCompatService,
	antigravityGatewayService *service.AntigravityGatewayService,
	userService *service.UserService,
	concurrencyService *service.ConcurrencyService,
	billingCacheService *service.BillingCacheService,
	usageService *service.UsageService,
	apiKeyService *service.APIKeyService,
	usageRecordWorkerPool *service.UsageRecordWorkerPool,
	errorPassthroughService *service.ErrorPassthroughService,
	contentModerationService *service.ContentModerationService,
	userMsgQueueService *service.UserMessageQueueService,
	cfg *config.Config,
	settingService *service.SettingService,
	coordinator *securityaudit.Coordinator,
	geminiCachedContentService *service.GeminiCachedContentService,
) *GatewayHandler {
	h := NewGatewayHandler(gatewayService, openAIGatewayService, geminiCompatService, antigravityGatewayService,
		userService, concurrencyService, billingCacheService, usageService, apiKeyService, usageRecordWorkerPool,
		errorPassthroughService, contentModerationService, userMsgQueueService, cfg, settingService)
	h.securityAuditCoordinator = coordinator
	h.geminiCachedContentService = geminiCachedContentService
	return h
}

func ProvideOpenAIGatewayHandler(
	gatewayService *service.OpenAIGatewayService,
	pluginManager *service.PluginManager,
	concurrencyService *service.ConcurrencyService,
	billingCacheService *service.BillingCacheService,
	apiKeyService *service.APIKeyService,
	subscriptionService *service.SubscriptionService,
	usageRecordWorkerPool *service.UsageRecordWorkerPool,
	errorPassthroughService *service.ErrorPassthroughService,
	contentModerationService *service.ContentModerationService,
	opsService *service.OpsService,
	grokQuotaService *service.GrokQuotaService,
	cfg *config.Config,
	coordinator *securityaudit.Coordinator,
	compositeResolver *service.CompositeRouteResolver,
) *OpenAIGatewayHandler {
	gatewayService.SetPluginManager(pluginManager)
	h := NewOpenAIGatewayHandler(gatewayService, concurrencyService, billingCacheService, apiKeyService,
		usageRecordWorkerPool, errorPassthroughService, contentModerationService, opsService, cfg)
	h.compositeResolver = compositeResolver
	h.wsSubscriptionLookup = subscriptionService
	h.securityAuditCoordinator = coordinator
	h.grokMediaEligibilityProber = grokQuotaService
	return h
}

func ProvideBatchImageHandler(
	batchService *service.BatchImagePublicService,
	download *service.BatchImageDownloadService,
	cleanup *service.BatchImageCleanupService,
	openAI *OpenAIGatewayHandler,
) *BatchImageHandler {
	h := NewBatchImageHandler(batchService, download, cleanup)
	h.openAI = openAI
	return h
}

// ProvideSystemHandler creates admin.SystemHandler with UpdateService
func ProvideSystemHandler(updateService *service.UpdateService, lockService *service.SystemOperationLockService, adminService service.AdminService) *admin.SystemHandler {
	h := admin.NewSystemHandler(updateService, lockService)
	if checker, ok := adminService.(service.MihomoNodeAdminService); ok {
		h.SetMihomoNodeChecker(checker)
	}
	return h
}

// ProvideSettingHandler creates SettingHandler with version from BuildInfo
func ProvideSettingHandler(settingService *service.SettingService, buildInfo BuildInfo, notificationEmailService *service.NotificationEmailService) *SettingHandler {
	h := NewSettingHandler(settingService, buildInfo.Version)
	h.SetNotificationEmailService(notificationEmailService)
	return h
}

// ProvideAdminSettingHandler creates admin.SettingHandler with notification template APIs.
func ProvideAIOpsHandler(ops *service.OpsService, tests *service.AccountTestService) *admin.OpsHandler {
	ops.SetAIOpsProbeRunner(tests)
	return admin.NewOpsHandler(ops)
}

func ProvideAdminSettingHandler(settingService *service.SettingService, emailService *service.EmailService, turnstileService *service.TurnstileService, aliyunCaptchaService *service.AliyunCaptchaService, opsService *service.OpsService, paymentConfigService *service.PaymentConfigService, paymentService *service.PaymentService, userAttributeService *service.UserAttributeService, notificationEmailService *service.NotificationEmailService, totpService *service.TotpService, userService *service.UserService) *admin.SettingHandler {
	h := admin.NewSettingHandler(settingService, emailService, turnstileService, opsService, paymentConfigService, paymentService, userAttributeService)
	h.SetNotificationEmailService(notificationEmailService)
	h.SetAliyunCaptchaService(aliyunCaptchaService)
	h.SetStepUpDeps(totpService, userService)
	h.SetOpenAICodexTicketProxyTester(service.NewOpenAICodexTicketProxyTester(settingService))
	return h
}

// ProvideHandlers creates the Handlers struct
func ProvideHandlers(
	gptDirectHandler *GPTDirectHandler,
	qqBotHandler *QQBotHandler,
	lotteryHandler *LotteryHandler,
	authHandler *AuthHandler,
	userHandler *UserHandler,
	mcpOAuthHandler *MCPOAuthHandler,
	apiKeyHandler *APIKeyHandler,
	usageHandler *UsageHandler,
	usageRecordsHandler *UsageRecordsHandler,
	redeemHandler *RedeemHandler,
	subscriptionHandler *SubscriptionHandler,
	announcementHandler *AnnouncementHandler,
	channelMonitorUserHandler *ChannelMonitorUserHandler,
	channelMonitorV2Handler *ChannelMonitorV2Handler,
	channelMonitorV3Handler *ChannelMonitorV3Handler,
	adminHandlers *AdminHandlers,
	gatewayHandler *GatewayHandler,
	openaiGatewayHandler *OpenAIGatewayHandler,
	settingHandler *SettingHandler,
	totpHandler *TotpHandler,
	passkeyHandler *PasskeyHandler,
	paymentHandler *PaymentHandler,
	paymentWebhookHandler *PaymentWebhookHandler,
	availableChannelHandler *AvailableChannelHandler,
	pelicanShowcaseHandler *PelicanShowcaseHandler,
	supportTicketHandler *SupportTicketHandler,
	modelPlazaHandler *ModelPlazaHandler,
	asyncImageHandler *AsyncImageHandler,
	batchImageHandler *BatchImageHandler,
	keyPouchImportHandler *KeyPouchImportHandler,

	_ *service.IdempotencyCoordinator,
	_ *service.IdempotencyCleanupService,
	_ *service.OpenAIQuotaAutoResetService,
) *Handlers {
	return &Handlers{
		KeyPouchImport:   keyPouchImportHandler,
		GPTDirect:        gptDirectHandler,
		QQBot:            qqBotHandler,
		Lottery:          lotteryHandler,
		Auth:             authHandler,
		User:             userHandler,
		MCPOAuth:         mcpOAuthHandler,
		APIKey:           apiKeyHandler,
		Usage:            usageHandler,
		UsageRecords:     usageRecordsHandler,
		Redeem:           redeemHandler,
		Subscription:     subscriptionHandler,
		Announcement:     announcementHandler,
		ChannelMonitor:   channelMonitorUserHandler,
		ChannelMonitorV2: channelMonitorV2Handler,
		ChannelMonitorV3: channelMonitorV3Handler,
		Admin:            adminHandlers,
		Gateway:          gatewayHandler,
		OpenAIGateway:    openaiGatewayHandler,
		Setting:          settingHandler,
		Totp:             totpHandler,
		Passkey:          passkeyHandler,
		Payment:          paymentHandler,
		PaymentWebhook:   paymentWebhookHandler,
		AvailableChannel: availableChannelHandler,
		PelicanShowcase:  pelicanShowcaseHandler,
		SupportTicket:    supportTicketHandler,
		ModelPlaza:       modelPlazaHandler,
		BatchImage:       batchImageHandler,
		AsyncImage:       asyncImageHandler,
	}
}

// ProvideUserHandler keeps the user-facing scheduled test results wired when
// regenerating the application's dependency graph.
func ProvideUserHandler(
	userService *service.UserService,
	authService *service.AuthService,
	emailService *service.EmailService,
	emailCache service.EmailCache,
	affiliateService *service.AffiliateService,
	userPlatformQuotaRepo service.UserPlatformQuotaRepository,
	scheduledTestService *service.ScheduledTestService,
) *UserHandler {
	h := NewUserHandler(userService, authService, emailService, emailCache, affiliateService, userPlatformQuotaRepo)
	h.SetScheduledTestService(scheduledTestService)
	return h
}

// ProviderSet is the Wire provider set for all handlers
var ProviderSet = wire.NewSet(
	NewKeyPouchImportHandler,
	NewQQBotHandler,
	// Top-level handlers
	NewLotteryHandler,
	NewAuthHandler,
	ProvideUserHandler,
	NewMCPOAuthHandler,
	NewAPIKeyHandler,
	NewUsageHandler,
	NewGPTDirectHandler,
	ratelimit.NewRateLimiter,
	NewUsageRecordsHandler,
	NewRedeemHandler,
	NewSubscriptionHandler,
	NewAnnouncementHandler,
	NewChannelMonitorUserHandler,
	NewChannelMonitorV2Handler,
	NewChannelMonitorV3Handler,
	ProvideGatewayHandler,
	ProvideOpenAIGatewayHandler,
	NewTotpHandler,
	NewPasskeyHandler,
	ProvideSettingHandler,
	NewPaymentHandler,
	NewPaymentWebhookHandler,
	NewAvailableChannelHandler,
	NewPelicanShowcaseHandler,
	NewSupportTicketHandler,
	NewModelPlazaHandler,
	ProvideBatchImageHandler,
	NewAsyncImageHandler,

	// Admin handlers
	admin.NewDashboardHandler,
	admin.NewUserHandler,
	admin.ProvideGroupHandlerWithProtection,
	admin.ProvideAccountHandler,
	admin.NewAnnouncementHandler,
	admin.NewDataManagementHandler,
	admin.NewBackupHandler,
	admin.NewOAuthHandler,
	admin.NewOpenAIOAuthHandler,
	admin.NewOpenAIOAuthReauthHandler,
	admin.NewGeminiOAuthHandler,
	admin.NewAntigravityOAuthHandler,
	admin.NewGrokOAuthHandler,
	admin.NewCNProviderHandler,
	admin.NewProxyHandler,
	admin.NewRedeemHandler,
	admin.NewPromoHandler,
	ProvideAdminSettingHandler,
	ProvideAIOpsHandler,
	admin.NewAccountHealthHandler,
	ProvideSystemHandler,
	admin.NewSubscriptionHandler,
	admin.NewUsageHandler,
	admin.NewUserAttributeHandler,
	admin.NewErrorPassthroughHandler,
	admin.NewTLSFingerprintProfileHandler,
	admin.NewPluginHandler,
	admin.NewOpenAIIPv6Handler,
	admin.NewAdminAPIKeyHandler,
	admin.NewScheduledTestHandler,
	admin.NewChannelQualityHandler,
	admin.NewPelicanGroupTestHandler,
	admin.NewControlledExperimentHandler,
	admin.NewAccountOpsHandler,
	admin.NewOpenAIReloginHandler,
	admin.NewAccountTokenGuardV2Handler,
	admin.ProvideMatureChannelHandler,
	admin.NewGroupProbeHandler,
	admin.NewChannelMonitorHandler,
	admin.NewChannelMonitorRequestTemplateHandler,
	admin.NewContentModerationHandlerWithSafetyPrompt,
	admin.NewPaymentHandler,
	admin.NewAffiliateHandler,
	admin.NewComplianceHandler,
	admin.NewAuditLogHandler,
	admin.NewRequestCaptureHandler,

	// AdminHandlers and Handlers constructors
	ProvideAdminHandlers,
	ProvideHandlers,
)
