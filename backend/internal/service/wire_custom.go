package service

import (
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/google/wire"
)

// CustomImageGenerationProviderSet groups custom dedicated image generation
// providers so upstream wire.go edits remain minimal during syncs.
var CustomImageGenerationProviderSet = wire.NewSet(
	ProvideImageGenerationResultStore,
	wire.Bind(new(ImageGenerationResultStore), new(*ResolvingImageGenerationResultStore)),
	wire.Bind(new(ImageGenerationResultReader), new(*ResolvingImageGenerationResultStore)),
	wire.Bind(new(ImageStorageDeleter), new(*ResolvingImageGenerationResultStore)),
	ProvideImageGenerationBilling,
	ProvideImageGenerationAccountSelector,
	ProvideImageGenerationProviderFactory,
	ProvideImageGenerationQueueController,
	ProvideImageGenerationOrchestrator,
	ProvideImageGenerationWorker,
	ProvideImageGenerationWorkerRuntime,
	ProvideImageGenerationCleanupService,
	ProvideCodexDedicatedImageBridge,
	ProvideDedicatedImageRuntimeController,
)

func ProvideRedeemService(
	redeemRepo RedeemCodeRepository,
	userRepo UserRepository,
	subscriptionService *SubscriptionService,
	cache RedeemCache,
	billingCacheService *BillingCacheService,
	entClient *dbent.Client,
	authCacheInvalidator APIKeyAuthCacheInvalidator,
	affiliateService *AffiliateService,
	newcomerCampaign *NewcomerCampaignService,
) *RedeemService {
	svc := NewRedeemService(redeemRepo, userRepo, subscriptionService, cache, billingCacheService, entClient, authCacheInvalidator, affiliateService)
	svc.SetNewcomerCampaignService(newcomerCampaign)
	return svc
}

func ProvideGatewayService(
	accountRepo AccountRepository,
	groupRepo GroupRepository,
	usageLogRepo UsageLogRepository,
	usageBillingRepo UsageBillingRepository,
	userRepo UserRepository,
	userSubRepo UserSubscriptionRepository,
	userGroupRateRepo UserGroupRateRepository,
	cache GatewayCache,
	cfg *config.Config,
	schedulerSnapshot *SchedulerSnapshotService,
	concurrencyService *ConcurrencyService,
	billingService *BillingService,
	rateLimitService *RateLimitService,
	billingCacheService *BillingCacheService,
	identityService *IdentityService,
	httpUpstream HTTPUpstream,
	deferredService *DeferredService,
	claudeTokenProvider *ClaudeTokenProvider,
	sessionLimitCache SessionLimitCache,
	rpmCache RPMCache,
	digestStore *DigestSessionStore,
	settingService *SettingService,
	tlsFPProfileService *TLSFingerprintProfileService,
	channelService *ChannelService,
	resolver *ModelPricingResolver,
	compositeResolver *CompositeRouteResolver,
	balanceNotifyService *BalanceNotifyService,
	userPlatformQuotaRepo UserPlatformQuotaRepository,
	newcomerCampaign *NewcomerCampaignService,
) *GatewayService {
	svc := NewGatewayService(accountRepo, groupRepo, usageLogRepo, usageBillingRepo, userRepo, userSubRepo,
		userGroupRateRepo, cache, cfg, schedulerSnapshot, concurrencyService, billingService, rateLimitService,
		billingCacheService, identityService, httpUpstream, deferredService, claudeTokenProvider,
		sessionLimitCache, rpmCache, digestStore, settingService, tlsFPProfileService, channelService,
		resolver, compositeResolver, balanceNotifyService, userPlatformQuotaRepo)
	svc.SetNewcomerCampaignService(newcomerCampaign)
	return svc
}

func ProvideOpenAIGatewayService(
	accountRepo AccountRepository,
	usageLogRepo UsageLogRepository,
	usageBillingRepo UsageBillingRepository,
	userRepo UserRepository,
	userSubRepo UserSubscriptionRepository,
	userGroupRateRepo UserGroupRateRepository,
	cache GatewayCache,
	cfg *config.Config,
	schedulerSnapshot *SchedulerSnapshotService,
	concurrencyService *ConcurrencyService,
	billingService *BillingService,
	rateLimitService *RateLimitService,
	billingCacheService *BillingCacheService,
	httpUpstream HTTPUpstream,
	deferredService *DeferredService,
	openAITokenProvider *OpenAITokenProvider,
	grokTokenProvider *GrokTokenProvider,
	resolver *ModelPricingResolver,
	channelService *ChannelService,
	balanceNotifyService *BalanceNotifyService,
	settingService *SettingService,
	userPlatformQuotaRepo UserPlatformQuotaRepository,
	newcomerCampaign *NewcomerCampaignService,
) *OpenAIGatewayService {
	svc := NewOpenAIGatewayService(accountRepo, usageLogRepo, usageBillingRepo, userRepo, userSubRepo,
		userGroupRateRepo, cache, cfg, schedulerSnapshot, concurrencyService, billingService, rateLimitService,
		billingCacheService, httpUpstream, deferredService, openAITokenProvider, grokTokenProvider,
		resolver, channelService, balanceNotifyService, settingService, userPlatformQuotaRepo)
	svc.SetNewcomerCampaignService(newcomerCampaign)
	return svc
}

func ProvideImageGenerationResultStore(settings *ImageStorageSettingService, cfg *config.Config) *ResolvingImageGenerationResultStore {
	return NewResolvingImageGenerationResultStore(settings, cfg.DedicatedImage.ResultPrefix, cfg.DedicatedImage.MaxOutputBytes)
}

func ProvideImageGenerationBilling(
	repo UsageBillingRepository,
	apiKeys *APIKeyService,
	accounts AccountRepository,
	usageLogs UsageLogRepository,
	cache *BillingCacheService,
) ImageGenerationBilling {
	return &UsageImageGenerationBilling{Repo: repo, APIKeys: apiKeys, Accounts: accounts, UsageLogs: usageLogs, Cache: cache}
}

func ProvideImageGenerationAccountSelector(gateway *OpenAIGatewayService, accounts AccountRepository, cfg *config.Config) ImageGenerationAccountSelector {
	var admission *ImageGenerationAdmission
	if gateway != nil && cfg != nil {
		imageConcurrency := cfg.Gateway.ImageConcurrency
		admission = NewImageGenerationAdmission(gateway.concurrencyService, ImageGenerationAdmissionConfig{
			Enabled:            imageConcurrency.Enabled,
			MaxPerUser:         imageConcurrency.MaxPerUser,
			MaxPerAPIKey:       imageConcurrency.MaxPerAPIKey,
			MaxPerGroup:        imageConcurrency.MaxPerGroup,
			MaxPerAccount:      imageConcurrency.MaxPerAccount,
			Max4KConcurrent:    imageConcurrency.Max4KConcurrent,
			OverflowMode:       imageConcurrency.OverflowMode,
			WaitTimeout:        time.Duration(imageConcurrency.WaitTimeoutSeconds) * time.Second,
			MaxWaitingRequests: imageConcurrency.MaxWaitingRequests,
		})
	}
	allowGeneralFallback := false
	if cfg != nil {
		allowGeneralFallback = cfg.DedicatedImage.FallbackToGeneral
	}
	return &DedicatedImageAccountSelector{
		Gateway: gateway, Accounts: accounts, ImageAdmission: admission,
		AllowGeneralFallback: allowGeneralFallback, Config: cfg,
	}
}

func ProvideImageGenerationProviderFactory() ImageGenerationProviderFactory {
	return &DefaultImageGenerationProviderFactory{}
}

func ProvideImageGenerationQueueController(
	repo ImageGenerationJobRepository,
	settings *SettingService,
	concurrency *ConcurrencyService,
) *ImageGenerationQueueController {
	return NewImageGenerationQueueController(repo, settings, concurrency)
}

func ProvideImageGenerationOrchestrator(
	repo ImageGenerationJobRepository,
	payloads ImageGenerationPayloadStore,
	wakeup ImageGenerationWakeup,
	queue *ImageGenerationQueueController,
	cfg *config.Config,
) *ImageGenerationOrchestrator {
	orchestrator := NewImageGenerationOrchestrator(repo, payloads, time.Duration(cfg.DedicatedImage.PayloadTTLHours)*time.Hour, wakeup)
	orchestrator.SetQueueController(queue)
	return orchestrator
}

func ProvideImageGenerationWorker(
	repo ImageGenerationJobRepository,
	payloads ImageGenerationPayloadStore,
	results ImageGenerationResultStore,
	billing ImageGenerationBilling,
	accounts ImageGenerationAccountSelector,
	providers ImageGenerationProviderFactory,
	queue *ImageGenerationQueueController,
	cfg *config.Config,
) *ImageGenerationWorker {
	options := ImageGenerationWorkerOptions{
		LeaseDuration:     time.Duration(cfg.DedicatedImage.LeaseDurationSeconds) * time.Second,
		PollInterval:      time.Duration(cfg.DedicatedImage.PollIntervalSeconds) * time.Second,
		RetryDelay:        time.Duration(cfg.DedicatedImage.RetryDelaySeconds) * time.Second,
		IdleDelay:         time.Duration(cfg.DedicatedImage.IdleDelayMilliseconds) * time.Millisecond,
		RecoveryInterval:  time.Duration(cfg.DedicatedImage.RecoveryIntervalSeconds) * time.Second,
		PayloadTTL:        time.Duration(cfg.DedicatedImage.PayloadTTLHours) * time.Hour,
		MaxOutputBytes:    cfg.DedicatedImage.MaxOutputBytes,
		MaxSubmitAttempts: cfg.DedicatedImage.MaxSubmitAttempts,
		RecoveryLimit:     cfg.DedicatedImage.RecoveryLimit,
	}
	worker := NewImageGenerationWorker(repo, payloads, results, billing, accounts, providers, options)
	worker.SetQueueController(queue)
	return worker
}

func ProvideImageGenerationWorkerRuntime(worker *ImageGenerationWorker, wakeup ImageGenerationWakeup, cfg *config.Config) *ImageGenerationWorkerRuntime {
	runtime := NewImageGenerationWorkerRuntime(worker, wakeup)
	if cfg != nil {
		runtimeSettings := cfg.DedicatedImageRuntime()
		if runtimeSettings.Enabled && runtimeSettings.WorkerEnabled {
			runtime.Start()
		}
	}
	return runtime
}

func ProvideImageGenerationCleanupService(
	repo ImageGenerationJobRepository,
	payloads ImageGenerationPayloadStore,
	results ImageGenerationResultReader,
	deleter ImageStorageDeleter,
	cfg *config.Config,
) *ImageGenerationCleanupService {
	if cfg == nil {
		return NewImageGenerationCleanupService(repo, payloads, results, deleter, 0, 0, 0)
	}
	svc := NewImageGenerationCleanupService(
		repo, payloads, results, deleter,
		time.Duration(cfg.DedicatedImage.TerminalRetentionHours)*time.Hour,
		time.Duration(cfg.DedicatedImage.CleanupIntervalMinutes)*time.Minute,
		cfg.DedicatedImage.CleanupBatchSize,
	)
	runtimeSettings := cfg.DedicatedImageRuntime()
	if runtimeSettings.Enabled && runtimeSettings.WorkerEnabled {
		svc.Start()
	}
	return svc
}

func ProvideCodexDedicatedImageBridge(
	gateway *OpenAIGatewayService,
	orchestrator *ImageGenerationOrchestrator,
	repo ImageGenerationJobRepository,
	results ImageGenerationResultReader,
	payloads ImageGenerationPayloadStore,
	queue *ImageGenerationQueueController,
	billing *BillingService,
	worker *ImageGenerationWorkerRuntime,
	cfg *config.Config,
) *CodexDedicatedImageBridge {
	return NewCodexDedicatedImageBridge(gateway, orchestrator, repo, results, payloads, queue, billing, worker, cfg)
}

// ProvideChannelMonitorAccountProbe wires the local-only V1 account probe.
// The endpoint/key/group checks in the implementation fail closed, so an
// external endpoint or a key that cannot be hydrated locally falls back to
// the original group-level checker.
func ProvideChannelMonitorAccountProbe(
	apiKeyRepo APIKeyRepository,
	accountRepo AccountRepository,
	settingRepo SettingRepository,
	gateway *GatewayService,
	openAIGateway *OpenAIGatewayService,
	geminiGateway *GeminiMessagesCompatService,
) ChannelMonitorAccountProbe {
	return newChannelMonitorAccountProbe(apiKeyRepo, accountRepo, settingRepo, gateway, openAIGateway, geminiGateway)
}

func ProvideNewcomerCampaignService(
	repo NewcomerCampaignRepository,
	affiliateService *AffiliateService,
	billingCacheService *BillingCacheService,
	authCacheInvalidator APIKeyAuthCacheInvalidator,
) *NewcomerCampaignService {
	svc := NewNewcomerCampaignServiceWithRepository(repo, affiliateService)
	svc.persistentConfig = true
	svc.SetCacheInvalidators(billingCacheService, authCacheInvalidator)
	return svc
}
