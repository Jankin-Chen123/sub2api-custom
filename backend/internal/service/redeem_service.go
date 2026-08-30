package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

var (
	ErrRedeemCodeNotFound  = infraerrors.NotFound("REDEEM_CODE_NOT_FOUND", "redeem code not found")
	ErrRedeemCodeUsed      = infraerrors.Conflict("REDEEM_CODE_USED", "redeem code already used")
	ErrRedeemCodeExpired   = infraerrors.Conflict("REDEEM_CODE_EXPIRED", "redeem code expired")
	ErrInsufficientBalance = infraerrors.BadRequest("INSUFFICIENT_BALANCE", "insufficient balance")
	ErrRedeemRateLimited   = infraerrors.TooManyRequests("REDEEM_RATE_LIMITED", "too many failed attempts, please try again later")
	ErrRedeemCodeLocked    = infraerrors.Conflict("REDEEM_CODE_LOCKED", "redeem code is being processed, please try again")
)

const (
	redeemMaxErrorsPerHour  = 20
	redeemRateLimitDuration = time.Hour
	redeemLockDuration      = 10 * time.Second // 锁超时时间，防止死锁
)

type ctxKeySkipRedeemAffiliate struct{}

// ContextSkipRedeemAffiliate returns a context that suppresses the redeem-level
// affiliate rebate. Used by payment fulfillment which handles rebate separately
// via applyAffiliateRebateForOrder (with audit-log deduplication).
func ContextSkipRedeemAffiliate(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxKeySkipRedeemAffiliate{}, true)
}

// RedeemCache defines cache operations for redeem service
type RedeemCache interface {
	GetRedeemAttemptCount(ctx context.Context, userID int64) (int, error)
	IncrementRedeemAttemptCount(ctx context.Context, userID int64) error

	AcquireRedeemLock(ctx context.Context, code string, ttl time.Duration) (bool, error)
	ReleaseRedeemLock(ctx context.Context, code string) error
}

type RedeemCodeRepository interface {
	Create(ctx context.Context, code *RedeemCode) error
	CreateBatch(ctx context.Context, codes []RedeemCode) error
	GetByID(ctx context.Context, id int64) (*RedeemCode, error)
	GetByCode(ctx context.Context, code string) (*RedeemCode, error)
	Update(ctx context.Context, code *RedeemCode) error
	BatchUpdate(ctx context.Context, ids []int64, fields RedeemCodeBatchUpdateFields) (int64, error)
	Delete(ctx context.Context, id int64) error
	Use(ctx context.Context, id, userID int64) error

	List(ctx context.Context, params pagination.PaginationParams) ([]RedeemCode, *pagination.PaginationResult, error)
	ListWithFilters(ctx context.Context, params pagination.PaginationParams, codeType, status, search string) ([]RedeemCode, *pagination.PaginationResult, error)
	ListByUser(ctx context.Context, userID int64, limit int) ([]RedeemCode, error)
	// ListByUserPaginated returns paginated balance/concurrency history for a specific user.
	// codeType filter is optional - pass empty string to return all types.
	ListByUserPaginated(ctx context.Context, userID int64, params pagination.PaginationParams, codeType string) ([]RedeemCode, *pagination.PaginationResult, error)
	// SumPositiveBalanceByUser returns the total recharged amount (sum of positive balance values) for a user.
	SumPositiveBalanceByUser(ctx context.Context, userID int64) (float64, error)
}

// RedeemAffiliateReviewRepository is implemented by the SQL repository for
// the administrator-only affiliate review workflow.
type RedeemAffiliateReviewRepository interface {
	UpdateAffiliateReview(ctx context.Context, id int64, status string, amount *float64, reviewedAt time.Time) error
	GetByIDForUpdate(ctx context.Context, id int64) (*RedeemCode, error)
}

// GenerateCodesRequest 生成兑换码请求
type GenerateCodesRequest struct {
	Count int     `json:"count"`
	Value float64 `json:"value"`
	Type  string  `json:"type"`
}

// RedeemCodeResponse 兑换码响应
type RedeemCodeResponse struct {
	Code      string    `json:"code"`
	Value     float64   `json:"value"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type NullableTimeUpdate struct {
	Set   bool
	Value *time.Time
}

type NullableInt64Update struct {
	Set   bool
	Value *int64
}

type RedeemCodeBatchUpdateFields struct {
	Status    *string
	ExpiresAt NullableTimeUpdate
	Notes     *string
	GroupID   NullableInt64Update

	// Core fields are intentionally modeled only so service validation can
	// reject payloads that try to mutate redemption value semantics in bulk.
	Type  *string
	Value *float64
}

func (f RedeemCodeBatchUpdateFields) HasChanges() bool {
	return f.Status != nil ||
		f.ExpiresAt.Set ||
		f.Notes != nil ||
		f.GroupID.Set ||
		f.Type != nil ||
		f.Value != nil
}

func (f RedeemCodeBatchUpdateFields) HasCoreFieldChanges() bool {
	return f.Type != nil || f.Value != nil
}

func (f RedeemCodeBatchUpdateFields) TouchesUsedSensitiveFields() bool {
	return f.Status != nil || f.ExpiresAt.Set || f.GroupID.Set
}

type RedeemCodeBatchUpdateInput struct {
	IDs    []int64
	Fields RedeemCodeBatchUpdateFields
}

type RedeemCodeBatchUpdateResult struct {
	Updated int64 `json:"updated"`
}

type RedeemAffiliateBatchReviewResult struct {
	Processed   int     `json:"processed"`
	Skipped     int     `json:"skipped"`
	TotalRebate float64 `json:"total_rebate"`
}

type newcomerCampaignReconciler interface {
	OnRedeemCompleted(context.Context, int64, *RedeemCode) error
	ReconcileUser(context.Context, int64) error
}

// RedeemService 兑换码服务
type RedeemService struct {
	redeemRepo           RedeemCodeRepository
	userRepo             UserRepository
	redeemUserRepo       RedeemUserAdjustmentRepository
	subscriptionService  *SubscriptionService
	cache                RedeemCache
	billingCacheService  *BillingCacheService
	entClient            *dbent.Client
	authCacheInvalidator APIKeyAuthCacheInvalidator
	affiliateService     *AffiliateService
	newcomerCampaign     newcomerCampaignReconciler
}

// NewRedeemService 创建兑换码服务实例
func NewRedeemService(
	redeemRepo RedeemCodeRepository,
	userRepo UserRepository,
	subscriptionService *SubscriptionService,
	cache RedeemCache,
	billingCacheService *BillingCacheService,
	entClient *dbent.Client,
	authCacheInvalidator APIKeyAuthCacheInvalidator,
	affiliateService *AffiliateService,
) *RedeemService {
	redeemUserRepo, _ := userRepo.(RedeemUserAdjustmentRepository)
	return &RedeemService{
		redeemRepo:           redeemRepo,
		userRepo:             userRepo,
		redeemUserRepo:       redeemUserRepo,
		subscriptionService:  subscriptionService,
		cache:                cache,
		billingCacheService:  billingCacheService,
		entClient:            entClient,
		authCacheInvalidator: authCacheInvalidator,
		affiliateService:     affiliateService,
	}
}

func (s *RedeemService) SetNewcomerCampaignService(campaign *NewcomerCampaignService) {
	if s != nil {
		s.newcomerCampaign = campaign
	}
}

// GenerateRandomCode 生成随机兑换码
func (s *RedeemService) GenerateRandomCode() (string, error) {
	// 生成16字节随机数据
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate random bytes: %w", err)
	}

	// 转换为十六进制字符串
	code := hex.EncodeToString(bytes)

	// 格式化为 XXXX-XXXX-XXXX-XXXX 格式
	parts := []string{
		strings.ToUpper(code[0:8]),
		strings.ToUpper(code[8:16]),
		strings.ToUpper(code[16:24]),
		strings.ToUpper(code[24:32]),
	}

	return strings.Join(parts, "-"), nil
}

// GenerateCodes 批量生成兑换码
func (s *RedeemService) GenerateCodes(ctx context.Context, req GenerateCodesRequest) ([]RedeemCode, error) {
	if req.Count <= 0 {
		return nil, errors.New("count must be greater than 0")
	}

	// 邀请码类型不需要数值，其他类型需要非零值（支持负数用于退款）
	if req.Type != RedeemTypeInvitation && req.Value == 0 {
		return nil, errors.New("value must not be zero")
	}

	if req.Count > 1000 {
		return nil, errors.New("cannot generate more than 1000 codes at once")
	}

	codeType := req.Type
	if codeType == "" {
		codeType = RedeemTypeBalance
	}

	// 邀请码类型的 value 设为 0
	value := req.Value
	if codeType == RedeemTypeInvitation {
		value = 0
	}

	codes := make([]RedeemCode, 0, req.Count)
	for i := 0; i < req.Count; i++ {
		code, err := s.GenerateRandomCode()
		if err != nil {
			return nil, fmt.Errorf("generate code: %w", err)
		}

		codes = append(codes, RedeemCode{
			Code:                  code,
			Type:                  codeType,
			Value:                 value,
			Status:                StatusUnused,
			AffiliateRebateStatus: AffiliateRebateStatusNotApplicable,
		})
	}

	// 批量插入
	if err := s.redeemRepo.CreateBatch(ctx, codes); err != nil {
		return nil, fmt.Errorf("create batch codes: %w", err)
	}

	return codes, nil
}

// CreateCode creates a redeem code with caller-provided code value.
// It is primarily used by admin integrations that require an external order ID
// to be mapped to a deterministic redeem code.
func (s *RedeemService) CreateCode(ctx context.Context, code *RedeemCode) error {
	if code == nil {
		return errors.New("redeem code is required")
	}
	code.Code = strings.TrimSpace(code.Code)
	if code.Code == "" {
		return errors.New("code is required")
	}
	if code.Type == "" {
		code.Type = RedeemTypeBalance
	}
	if code.Type != RedeemTypeInvitation && code.Value == 0 {
		return errors.New("value must not be zero")
	}
	if code.Status == "" {
		code.Status = StatusUnused
	}
	if code.AffiliateRebateStatus == "" {
		code.AffiliateRebateStatus = AffiliateRebateStatusNotApplicable
	}
	if code.IsExpired() {
		return ErrRedeemCodeExpired
	}

	if err := s.redeemRepo.Create(ctx, code); err != nil {
		return fmt.Errorf("create redeem code: %w", err)
	}
	return nil
}

func (s *RedeemService) BatchUpdate(ctx context.Context, input *RedeemCodeBatchUpdateInput) (*RedeemCodeBatchUpdateResult, error) {
	if input == nil {
		return nil, infraerrors.BadRequest("REDEEM_CODE_BATCH_UPDATE_INVALID", "batch update input is required")
	}
	if len(input.IDs) == 0 {
		return nil, infraerrors.BadRequest("REDEEM_CODE_BATCH_UPDATE_IDS_REQUIRED", "ids are required")
	}
	if !input.Fields.HasChanges() {
		return nil, infraerrors.BadRequest("REDEEM_CODE_BATCH_UPDATE_EMPTY", "at least one field must be selected")
	}
	if input.Fields.HasCoreFieldChanges() {
		return nil, infraerrors.BadRequest("REDEEM_CODE_CORE_FIELDS_IMMUTABLE", "type and value cannot be batch updated")
	}

	ids := make([]int64, 0, len(input.IDs))
	seen := make(map[int64]struct{}, len(input.IDs))
	for _, id := range input.IDs {
		if id <= 0 {
			return nil, infraerrors.BadRequest("REDEEM_CODE_BATCH_UPDATE_INVALID_ID", "ids must be positive")
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, infraerrors.BadRequest("REDEEM_CODE_BATCH_UPDATE_IDS_REQUIRED", "ids are required")
	}

	if input.Fields.Status != nil {
		switch *input.Fields.Status {
		case StatusUnused, StatusDisabled:
		default:
			return nil, infraerrors.BadRequest("REDEEM_CODE_STATUS_INVALID", "status must be unused or disabled")
		}
	}
	if input.Fields.ExpiresAt.Set && input.Fields.ExpiresAt.Value != nil {
		expiresAt := input.Fields.ExpiresAt.Value.UTC()
		if !expiresAt.After(time.Now().UTC()) {
			return nil, infraerrors.BadRequest("REDEEM_CODE_EXPIRES_AT_INVALID", "expires_at must be in the future")
		}
		input.Fields.ExpiresAt.Value = &expiresAt
	}
	if input.Fields.GroupID.Set && input.Fields.GroupID.Value != nil && *input.Fields.GroupID.Value <= 0 {
		return nil, infraerrors.BadRequest("REDEEM_CODE_GROUP_ID_INVALID", "group_id must be positive")
	}

	updated, err := s.redeemRepo.BatchUpdate(ctx, ids, input.Fields)
	if err != nil {
		return nil, err
	}
	return &RedeemCodeBatchUpdateResult{Updated: updated}, nil
}

// checkRedeemRateLimit 检查用户兑换错误次数是否超限
func (s *RedeemService) checkRedeemRateLimit(ctx context.Context, userID int64) error {
	if s.cache == nil {
		return nil
	}

	count, err := s.cache.GetRedeemAttemptCount(ctx, userID)
	if err != nil {
		// Redis 出错时不阻止用户操作
		return nil
	}

	if count >= redeemMaxErrorsPerHour {
		return ErrRedeemRateLimited
	}

	return nil
}

// incrementRedeemErrorCount 增加用户兑换错误计数
func (s *RedeemService) incrementRedeemErrorCount(ctx context.Context, userID int64) {
	if s.cache == nil {
		return
	}

	_ = s.cache.IncrementRedeemAttemptCount(ctx, userID)
}

// acquireRedeemLock 尝试获取兑换码的分布式锁
// 返回 true 表示获取成功，false 表示锁已被占用
func (s *RedeemService) acquireRedeemLock(ctx context.Context, code string) bool {
	if s.cache == nil {
		return true // 无 Redis 时降级为不加锁
	}

	ok, err := s.cache.AcquireRedeemLock(ctx, code, redeemLockDuration)
	if err != nil {
		// Redis 出错时不阻止操作，依赖数据库层面的状态检查
		return true
	}
	return ok
}

// releaseRedeemLock 释放兑换码的分布式锁
func (s *RedeemService) releaseRedeemLock(ctx context.Context, code string) {
	if s.cache == nil {
		return
	}

	_ = s.cache.ReleaseRedeemLock(ctx, code)
}

func unsupportedRedeemTypeError(codeType string) error {
	if codeType == RedeemTypeInvitation {
		return infraerrors.BadRequest("REDEEM_CODE_UNSUPPORTED_TYPE", "invitation codes can only be used during registration")
	}
	return infraerrors.BadRequest("REDEEM_CODE_UNSUPPORTED_TYPE", fmt.Sprintf("unsupported redeem type: %s", codeType))
}

// Redeem 使用兑换码
func (s *RedeemService) Redeem(ctx context.Context, userID int64, code string) (*RedeemCode, error) {
	// 检查限流
	if err := s.checkRedeemRateLimit(ctx, userID); err != nil {
		return nil, err
	}

	// 获取分布式锁，防止同一兑换码并发使用
	if !s.acquireRedeemLock(ctx, code) {
		return nil, ErrRedeemCodeLocked
	}
	defer s.releaseRedeemLock(ctx, code)

	// 查找兑换码
	redeemCode, err := s.redeemRepo.GetByCode(ctx, code)
	if err != nil {
		if errors.Is(err, ErrRedeemCodeNotFound) {
			s.incrementRedeemErrorCount(ctx, userID)
			return nil, ErrRedeemCodeNotFound
		}
		return nil, fmt.Errorf("get redeem code: %w", err)
	}

	// 检查兑换码状态和码本身的过期时间
	if redeemCode.IsExpired() {
		s.incrementRedeemErrorCount(ctx, userID)
		return nil, ErrRedeemCodeExpired
	}
	if !redeemCode.CanUse() {
		s.incrementRedeemErrorCount(ctx, userID)
		return nil, ErrRedeemCodeUsed
	}

	// 验证兑换码类型的前置条件。邀请码属于注册流程，不能通过普通兑换接口使用。
	switch redeemCode.Type {
	case RedeemTypeBalance, RedeemTypeConcurrency:
	case RedeemTypeSubscription:
		if redeemCode.GroupID == nil {
			return nil, infraerrors.BadRequest("REDEEM_CODE_INVALID", "invalid subscription redeem code: missing group_id")
		}
	default:
		return nil, unsupportedRedeemTypeError(redeemCode.Type)
	}

	// 获取用户信息
	_, err = s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}

	// 使用数据库事务保证兑换码标记与权益发放的原子性
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// 将事务放入 context，使 repository 方法能够使用同一事务
	txCtx := dbent.NewTxContext(ctx, tx)

	// 【关键】先标记兑换码为已使用，确保并发安全
	// 利用数据库乐观锁（WHERE status = 'unused'）保证原子性
	if err := s.redeemRepo.Use(txCtx, redeemCode.ID, userID); err != nil {
		if errors.Is(err, ErrRedeemCodeNotFound) || errors.Is(err, ErrRedeemCodeUsed) {
			return nil, ErrRedeemCodeUsed
		}
		return nil, fmt.Errorf("mark code as used: %w", err)
	}

	// 执行兑换逻辑（兑换码已被锁定，此时可安全操作）
	switch redeemCode.Type {
	case RedeemTypeBalance:
		amount := redeemCode.Value
		if amount < 0 {
			if s.redeemUserRepo == nil {
				return nil, errors.New("user repository does not support atomic redeem balance adjustments")
			}
			if err := s.redeemUserRepo.ApplyRedeemBalanceAdjustment(txCtx, userID, amount); err != nil {
				return nil, fmt.Errorf("update user balance: %w", err)
			}
		} else if err := s.userRepo.UpdateBalance(txCtx, userID, amount); err != nil {
			return nil, fmt.Errorf("update user balance: %w", err)
		}

	case RedeemTypeConcurrency:
		delta := int(redeemCode.Value)
		if delta < 0 {
			if s.redeemUserRepo == nil {
				return nil, errors.New("user repository does not support atomic redeem concurrency adjustments")
			}
			if err := s.redeemUserRepo.ApplyRedeemConcurrencyAdjustment(txCtx, userID, delta); err != nil {
				return nil, fmt.Errorf("update user concurrency: %w", err)
			}
		} else if err := s.userRepo.UpdateConcurrency(txCtx, userID, delta); err != nil {
			return nil, fmt.Errorf("update user concurrency: %w", err)
		}

	case RedeemTypeSubscription:
		validityDays := redeemCode.ValidityDays
		if validityDays < 0 {
			// 负数天数：缩短订阅，减到 0 则取消订阅
			if err := s.reduceOrCancelSubscription(txCtx, userID, *redeemCode.GroupID, -validityDays, redeemCode.Code); err != nil {
				return nil, fmt.Errorf("reduce or cancel subscription: %w", err)
			}
		} else {
			if validityDays == 0 {
				validityDays = 30
			}
			_, _, err := s.subscriptionService.AssignOrExtendSubscription(txCtx, &AssignSubscriptionInput{
				UserID:       userID,
				GroupID:      *redeemCode.GroupID,
				ValidityDays: validityDays,
				AssignedBy:   0, // 系统分配
				Notes:        fmt.Sprintf("通过兑换码 %s 兑换", redeemCode.Code),
			})
			if err != nil {
				return nil, fmt.Errorf("assign or extend subscription: %w", err)
			}
		}

	default:
		return nil, unsupportedRedeemTypeError(redeemCode.Type)
	}

	// Automatic affiliate review runs inside the redemption transaction so a
	// valid paid code cannot credit the user without its matching rebate.
	if redeemCode.Type == RedeemTypeBalance && redeemCode.Value > 0 && s.affiliateService != nil {
		decision := s.affiliateService.GetRedeemAutoReviewDecision(txCtx, redeemCode.Value)
		if decision != "" {
			reviewRepo, ok := s.redeemRepo.(RedeemAffiliateReviewRepository)
			if !ok {
				return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "redeem affiliate review unavailable")
			}
			switch decision {
			case "valid":
				if !s.affiliateService.IsEnabled(txCtx) {
					break
				}
				rebate, err := s.affiliateService.AccrueInviteRebateForRedeemCode(txCtx, userID, redeemCode.ID, redeemCode.Value)
				if err != nil {
					return nil, fmt.Errorf("accrue automatic redeem affiliate rebate: %w", err)
				}
				if err := reviewRepo.UpdateAffiliateReview(txCtx, redeemCode.ID, AffiliateRebateStatusApproved, &rebate, time.Now().UTC()); err != nil {
					return nil, fmt.Errorf("mark automatic redeem affiliate review approved: %w", err)
				}
			case "excluded":
				if err := reviewRepo.UpdateAffiliateReview(txCtx, redeemCode.ID, AffiliateRebateStatusExcluded, nil, time.Now().UTC()); err != nil {
					return nil, fmt.Errorf("mark automatic redeem affiliate review excluded: %w", err)
				}
			}
		}
	}

	// 提交事务
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	// 事务提交成功后失效缓存
	s.invalidateRedeemCaches(ctx, userID, redeemCode)

	// 重新获取更新后的兑换码
	redeemCode, err = s.redeemRepo.GetByID(ctx, redeemCode.ID)
	if err != nil {
		return nil, fmt.Errorf("get updated redeem code: %w", err)
	}
	if s.newcomerCampaign != nil {
		if err := s.newcomerCampaign.OnRedeemCompleted(ctx, userID, redeemCode); err != nil {
			// Redemption has already committed. Keep the user-visible redemption
			// successful and leave the repeatable campaign reconciliation to the
			// repair/status entry point.
			slog.Warn("newcomer campaign redemption reconciliation failed", "user_id", userID, "redeem_code_id", redeemCode.ID, "error", err)
		}
	}

	return redeemCode, nil
}

// invalidateRedeemCaches 失效兑换相关的缓存
func (s *RedeemService) invalidateRedeemCaches(ctx context.Context, userID int64, redeemCode *RedeemCode) {
	switch redeemCode.Type {
	case RedeemTypeBalance:
		if s.authCacheInvalidator != nil {
			s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
		}
		if s.billingCacheService == nil {
			return
		}
		go func() {
			cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = s.billingCacheService.InvalidateUserBalance(cacheCtx, userID)
		}()
	case RedeemTypeConcurrency:
		if s.authCacheInvalidator != nil {
			s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
		}
		if s.billingCacheService == nil {
			return
		}
	case RedeemTypeSubscription:
		if s.authCacheInvalidator != nil {
			s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
		}
		if s.billingCacheService == nil {
			return
		}
		if redeemCode.GroupID != nil {
			groupID := *redeemCode.GroupID
			go func() {
				cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = s.billingCacheService.InvalidateSubscription(cacheCtx, userID, groupID)
			}()
		}
	}
}

// ReviewAffiliateRedeem applies or excludes a used positive balance redeem
// code. Approval and quota accrual share one database transaction; the ledger
// source ID makes retries idempotent.
func (s *RedeemService) ReviewAffiliateRedeem(ctx context.Context, id int64, decision string) (*RedeemCode, error) {
	reviewRepo, ok := s.redeemRepo.(RedeemAffiliateReviewRepository)
	if !ok || s.entClient == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "redeem affiliate review unavailable")
	}

	decision = strings.ToLower(strings.TrimSpace(decision))
	if decision != "valid" && decision != "free" {
		return nil, infraerrors.BadRequest("REDEEM_AFFILIATE_DECISION_INVALID", "decision must be valid or free")
	}

	code, err := s.redeemRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if code.Status != StatusUsed || code.UsedBy == nil || code.Type != RedeemTypeBalance || code.Value <= 0 {
		return nil, infraerrors.BadRequest("REDEEM_AFFILIATE_NOT_ELIGIBLE", "only used positive balance redeem codes can be reviewed")
	}
	if code.AffiliateRebateStatus == "" {
		code.AffiliateRebateStatus = AffiliateRebateStatusPending
	}
	if decision == "free" {
		if code.AffiliateRebateStatus == AffiliateRebateStatusApproved {
			return nil, infraerrors.Conflict("REDEEM_AFFILIATE_ALREADY_APPROVED", "approved redeem code rebate cannot be excluded")
		}
		tx, err := s.entClient.Tx(ctx)
		if err != nil {
			return nil, fmt.Errorf("begin affiliate review transaction: %w", err)
		}
		defer func() { _ = tx.Rollback() }()
		txCtx := dbent.NewTxContext(ctx, tx)
		lockedCode, err := reviewRepo.GetByIDForUpdate(txCtx, id)
		if err != nil {
			return nil, err
		}
		if lockedCode.Status != StatusUsed || lockedCode.UsedBy == nil || lockedCode.Type != RedeemTypeBalance || lockedCode.Value <= 0 {
			return nil, infraerrors.BadRequest("REDEEM_AFFILIATE_NOT_ELIGIBLE", "only used positive balance redeem codes can be reviewed")
		}
		if lockedCode.AffiliateRebateStatus == AffiliateRebateStatusApproved {
			return nil, infraerrors.Conflict("REDEEM_AFFILIATE_ALREADY_APPROVED", "approved redeem code rebate cannot be excluded")
		}
		if err := reviewRepo.UpdateAffiliateReview(txCtx, id, AffiliateRebateStatusExcluded, nil, time.Now().UTC()); err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("commit affiliate review transaction: %w", err)
		}
		updated, err := s.redeemRepo.GetByID(ctx, id)
		if err != nil {
			return nil, err
		}
		s.reconcileCampaignAfterRedeemReview(ctx, updated)
		return updated, nil
	}

	if code.AffiliateRebateStatus == AffiliateRebateStatusApproved {
		// Keep an already-approved code idempotent, but still run the
		// campaign repair hook. This covers an earlier failed callback and
		// makes the review endpoint self-healing.
		s.reconcileCampaignAfterRedeemReview(ctx, code)
		return code, nil
	}
	if s.affiliateService == nil || !s.affiliateService.IsEnabled(ctx) {
		return nil, infraerrors.Conflict("AFFILIATE_DISABLED", "affiliate rebate is disabled")
	}

	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin affiliate review transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)
	lockedCode, err := reviewRepo.GetByIDForUpdate(txCtx, id)
	if err != nil {
		return nil, err
	}
	if lockedCode.Status != StatusUsed || lockedCode.UsedBy == nil || lockedCode.Type != RedeemTypeBalance || lockedCode.Value <= 0 {
		return nil, infraerrors.BadRequest("REDEEM_AFFILIATE_NOT_ELIGIBLE", "only used positive balance redeem codes can be reviewed")
	}
	if lockedCode.AffiliateRebateStatus == AffiliateRebateStatusApproved {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		updated, err := s.redeemRepo.GetByID(ctx, id)
		if err != nil {
			return nil, err
		}
		s.reconcileCampaignAfterRedeemReview(ctx, updated)
		return updated, nil
	}

	rebate, err := s.affiliateService.AccrueInviteRebateForRedeemCode(txCtx, *lockedCode.UsedBy, lockedCode.ID, lockedCode.Value)
	if err != nil {
		return nil, fmt.Errorf("accrue redeem affiliate rebate: %w", err)
	}
	if err := reviewRepo.UpdateAffiliateReview(txCtx, id, AffiliateRebateStatusApproved, &rebate, time.Now().UTC()); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit affiliate review transaction: %w", err)
	}
	updated, err := s.redeemRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	// The review result is already committed. A campaign repair failure is
	// logged by the helper and must never turn a successful review into an API
	// failure.
	s.reconcileCampaignAfterRedeemReview(ctx, updated)
	return updated, nil
}

// reconcileCampaignAfterRedeemReview keeps the activity qualification in sync
// with an administrator changing a used balance code to free/excluded. The
// affiliate rebate switch is intentionally not consulted here: this is an
// activity-state repair, not a cash-rebate operation.
func (s *RedeemService) reconcileCampaignAfterRedeemReview(ctx context.Context, code *RedeemCode) {
	if s == nil || s.newcomerCampaign == nil || code == nil || code.UsedBy == nil {
		return
	}
	if err := s.newcomerCampaign.OnRedeemCompleted(ctx, *code.UsedBy, code); err != nil {
		slog.Warn("newcomer campaign reconciliation after redeem review failed", "user_id", *code.UsedBy, "redeem_code_id", code.ID, "affiliate_rebate_status", code.AffiliateRebateStatus, "error", err)
	}
}

// ReviewAffiliateRedeems applies one affiliate-review decision to a batch of
// selected codes. Non-eligible records are skipped so a mixed selection from
// the admin table does not prevent eligible records from being processed.
func (s *RedeemService) ReviewAffiliateRedeems(ctx context.Context, ids []int64, decision string) (*RedeemAffiliateBatchReviewResult, error) {
	reviewRepo, ok := s.redeemRepo.(RedeemAffiliateReviewRepository)
	if !ok || s.entClient == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "redeem affiliate review unavailable")
	}

	decision = strings.ToLower(strings.TrimSpace(decision))
	if decision != "valid" && decision != "free" {
		return nil, infraerrors.BadRequest("REDEEM_AFFILIATE_DECISION_INVALID", "decision must be valid or free")
	}
	if len(ids) == 0 {
		return nil, infraerrors.BadRequest("REDEEM_AFFILIATE_BATCH_EMPTY", "ids are required")
	}
	if len(ids) > 1000 {
		return nil, infraerrors.BadRequest("REDEEM_AFFILIATE_BATCH_TOO_LARGE", "cannot review more than 1000 redeem codes at once")
	}

	uniqueIDs := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, infraerrors.BadRequest("REDEEM_AFFILIATE_INVALID_ID", "ids must be positive")
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		uniqueIDs = append(uniqueIDs, id)
	}
	sort.Slice(uniqueIDs, func(i, j int) bool { return uniqueIDs[i] < uniqueIDs[j] })

	if decision == "valid" && (s.affiliateService == nil || !s.affiliateService.IsEnabled(ctx)) {
		return nil, infraerrors.Conflict("AFFILIATE_DISABLED", "affiliate rebate is disabled")
	}

	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin affiliate batch review transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)
	result := &RedeemAffiliateBatchReviewResult{}
	campaignUserIDs := make(map[int64]struct{})

	for _, id := range uniqueIDs {
		code, err := reviewRepo.GetByIDForUpdate(txCtx, id)
		if err != nil {
			if errors.Is(err, ErrRedeemCodeNotFound) {
				result.Skipped++
				continue
			}
			return nil, err
		}
		if code.Status != StatusUsed || code.UsedBy == nil || code.Type != RedeemTypeBalance || code.Value <= 0 {
			result.Skipped++
			continue
		}

		if decision == "free" {
			if code.AffiliateRebateStatus == AffiliateRebateStatusApproved {
				result.Skipped++
				continue
			}
			if err := reviewRepo.UpdateAffiliateReview(txCtx, id, AffiliateRebateStatusExcluded, nil, time.Now().UTC()); err != nil {
				return nil, err
			}
			campaignUserIDs[*code.UsedBy] = struct{}{}
			result.Processed++
			continue
		}

		if code.AffiliateRebateStatus == AffiliateRebateStatusApproved {
			campaignUserIDs[*code.UsedBy] = struct{}{}
			result.Skipped++
			continue
		}
		rebate, err := s.affiliateService.AccrueInviteRebateForRedeemCode(txCtx, *code.UsedBy, code.ID, code.Value)
		if err != nil {
			return nil, fmt.Errorf("accrue redeem affiliate rebate: %w", err)
		}
		if err := reviewRepo.UpdateAffiliateReview(txCtx, id, AffiliateRebateStatusApproved, &rebate, time.Now().UTC()); err != nil {
			return nil, err
		}
		campaignUserIDs[*code.UsedBy] = struct{}{}
		result.Processed++
		result.TotalRebate += rebate
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit affiliate batch review transaction: %w", err)
	}
	if s.newcomerCampaign != nil {
		if s.newcomerCampaign != nil {
			for userID := range campaignUserIDs {
				if err := s.newcomerCampaign.ReconcileUser(ctx, userID); err != nil {
					slog.Warn("newcomer campaign batch reconciliation after redeem review failed", "user_id", userID, "error", err)
				}
			}
		}
	}
	return result, nil
}

// GetByID 根据ID获取兑换码
func (s *RedeemService) GetByID(ctx context.Context, id int64) (*RedeemCode, error) {
	code, err := s.redeemRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get redeem code: %w", err)
	}
	return code, nil
}

// GetByCode 根据Code获取兑换码
func (s *RedeemService) GetByCode(ctx context.Context, code string) (*RedeemCode, error) {
	redeemCode, err := s.redeemRepo.GetByCode(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("get redeem code: %w", err)
	}
	return redeemCode, nil
}

// List 获取兑换码列表（管理员功能）
func (s *RedeemService) List(ctx context.Context, params pagination.PaginationParams) ([]RedeemCode, *pagination.PaginationResult, error) {
	codes, pagination, err := s.redeemRepo.List(ctx, params)
	if err != nil {
		return nil, nil, fmt.Errorf("list redeem codes: %w", err)
	}
	return codes, pagination, nil
}

// Delete 删除兑换码（管理员功能）
func (s *RedeemService) Delete(ctx context.Context, id int64) error {
	// 检查兑换码是否存在
	code, err := s.redeemRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get redeem code: %w", err)
	}

	// 不允许删除已使用的兑换码
	if code.IsUsed() {
		return infraerrors.Conflict("REDEEM_CODE_DELETE_USED", "cannot delete used redeem code")
	}

	if err := s.redeemRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete redeem code: %w", err)
	}

	return nil
}

// GetStats 获取兑换码统计信息
func (s *RedeemService) GetStats(ctx context.Context) (map[string]any, error) {
	// TODO: 实现统计逻辑
	// 统计未使用、已使用的兑换码数量
	// 统计总面值等

	stats := map[string]any{
		"total_codes":  0,
		"unused_codes": 0,
		"used_codes":   0,
		"total_value":  0.0,
	}

	return stats, nil
}

// GetUserHistory 获取用户的兑换历史
func (s *RedeemService) GetUserHistory(ctx context.Context, userID int64, limit int) ([]RedeemCode, error) {
	codes, err := s.redeemRepo.ListByUser(ctx, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("get user redeem history: %w", err)
	}
	return codes, nil
}

// reduceOrCancelSubscription 缩短订阅天数，剩余天数 <= 0 时取消订阅
func (s *RedeemService) reduceOrCancelSubscription(ctx context.Context, userID, groupID int64, reduceDays int, code string) error {
	sub, err := s.subscriptionService.userSubRepo.GetByUserIDAndGroupID(ctx, userID, groupID)
	if err != nil {
		return ErrSubscriptionNotFound
	}

	now := time.Now()
	remaining := int(sub.ExpiresAt.Sub(now).Hours() / 24)
	if remaining < 0 {
		remaining = 0
	}

	notes := fmt.Sprintf("通过兑换码 %s 退款扣减 %d 天", code, reduceDays)

	if remaining <= reduceDays {
		// 剩余天数不足，直接取消订阅
		if err := s.subscriptionService.userSubRepo.UpdateStatus(ctx, sub.ID, SubscriptionStatusExpired); err != nil {
			return fmt.Errorf("cancel subscription: %w", err)
		}
		// 设置过期时间为当前时间
		if err := s.subscriptionService.userSubRepo.ExtendExpiry(ctx, sub.ID, now); err != nil {
			return fmt.Errorf("set subscription expiry: %w", err)
		}
	} else {
		// 缩短天数
		newExpiresAt := sub.ExpiresAt.AddDate(0, 0, -reduceDays)
		if err := s.subscriptionService.userSubRepo.ExtendExpiry(ctx, sub.ID, newExpiresAt); err != nil {
			return fmt.Errorf("reduce subscription: %w", err)
		}
	}

	// 追加备注
	newNotes := sub.Notes
	if newNotes != "" {
		newNotes += "\n"
	}
	newNotes += notes
	if err := s.subscriptionService.userSubRepo.UpdateNotes(ctx, sub.ID, newNotes); err != nil {
		return fmt.Errorf("update subscription notes: %w", err)
	}

	// 失效缓存
	s.subscriptionService.InvalidateSubCache(userID, groupID)

	return nil
}
