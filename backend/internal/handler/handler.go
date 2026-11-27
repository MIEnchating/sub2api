package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/securityaudit"
)

// AdminHandlers contains all admin-related HTTP handlers
type AdminHandlers struct {
	ChannelQuality         *admin.ChannelQualityHandler
	RequestCapture         *admin.RequestCaptureHandler
	AccountOps             *admin.AccountOpsHandler
	OpenAIRelogin          *admin.OpenAIReloginHandler
	AccountTokenGuardV2    *admin.AccountTokenGuardV2Handler
	Dashboard              *admin.DashboardHandler
	User                   *admin.UserHandler
	Group                  *admin.GroupHandler
	Account                *admin.AccountHandler
	Announcement           *admin.AnnouncementHandler
	DataManagement         *admin.DataManagementHandler
	Backup                 *admin.BackupHandler
	OAuth                  *admin.OAuthHandler
	OpenAIOAuth            *admin.OpenAIOAuthHandler
	OpenAIReauth           *admin.OpenAIOAuthReauthHandler
	GeminiOAuth            *admin.GeminiOAuthHandler
	AntigravityOAuth       *admin.AntigravityOAuthHandler
	GrokOAuth              *admin.GrokOAuthHandler
	CNProvider             *admin.CNProviderHandler
	Proxy                  *admin.ProxyHandler
	Redeem                 *admin.RedeemHandler
	Promo                  *admin.PromoHandler
	Setting                *admin.SettingHandler
	Ops                    *admin.OpsHandler
	AccountHealth          *admin.AccountHealthHandler
	System                 *admin.SystemHandler
	Subscription           *admin.SubscriptionHandler
	Usage                  *admin.UsageHandler
	UserAttribute          *admin.UserAttributeHandler
	ErrorPassthrough       *admin.ErrorPassthroughHandler
	TLSFingerprintProfile  *admin.TLSFingerprintProfileHandler
	Plugin                 *admin.PluginHandler
	OpenAIIPv6             *admin.OpenAIIPv6Handler
	APIKey                 *admin.AdminAPIKeyHandler
	ScheduledTest          *admin.ScheduledTestHandler
	PelicanGroupTest       *admin.PelicanGroupTestHandler
	ControlledExperiment   *admin.ControlledExperimentHandler
	Channel                *admin.ChannelHandler
	ChannelMonitor         *admin.ChannelMonitorHandler
	ChannelMonitorTemplate *admin.ChannelMonitorRequestTemplateHandler
	ContentModeration      *admin.ContentModerationHandler
	PromptAudit            *securityaudit.PromptAdminHandler
	Payment                *admin.PaymentHandler
	Affiliate              *admin.AffiliateHandler
	Compliance             *admin.ComplianceHandler
	AuditLog               *admin.AuditLogHandler
}

// Handlers contains all HTTP handlers
type Handlers struct {
	KeyPouchImport   *KeyPouchImportHandler
	GPTDirect        *GPTDirectHandler
	QQBot            *QQBotHandler
	Lottery          *LotteryHandler
	Auth             *AuthHandler
	User             *UserHandler
	MCPOAuth         *MCPOAuthHandler
	APIKey           *APIKeyHandler
	Usage            *UsageHandler
	UsageRecords     *UsageRecordsHandler
	Redeem           *RedeemHandler
	Subscription     *SubscriptionHandler
	Announcement     *AnnouncementHandler
	ChannelMonitor   *ChannelMonitorUserHandler
	ChannelMonitorV2 *ChannelMonitorV2Handler
	ChannelMonitorV3 *ChannelMonitorV3Handler
	Admin            *AdminHandlers
	Gateway          *GatewayHandler
	OpenAIGateway    *OpenAIGatewayHandler
	Setting          *SettingHandler
	Totp             *TotpHandler
	Passkey          *PasskeyHandler
	Payment          *PaymentHandler
	PaymentWebhook   *PaymentWebhookHandler
	AvailableChannel *AvailableChannelHandler
	PelicanShowcase  *PelicanShowcaseHandler
	SupportTicket    *SupportTicketHandler
	ModelPlaza       *ModelPlazaHandler
	AsyncImage       *AsyncImageHandler
	BatchImage       *BatchImageHandler
}

// BuildInfo contains build-time information
type BuildInfo struct {
	Version   string
	BuildType string // "source" for manual builds, "release" for CI builds
}
