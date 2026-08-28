package service

import (
	"context"
	cryptorand "crypto/rand"
	"errors"
	"fmt"
	"math"
	"math/big"
	"regexp"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	checkinProbabilityTotal = 100.0
	maxCheckinPrizes        = 50
	checkinStreakTarget     = 7
	defaultStreakBonus      = 5.0
)

var (
	ErrCheckinAlreadyClaimed = errors.New("daily check-in already claimed")
	ErrCheckinNotConfigured  = errors.New("daily check-in prizes are not configured")
	checkinColorPattern      = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
	checkinLocation          = time.FixedZone("Asia/Shanghai", 8*60*60)
)

// CheckinPrize is one segment on the daily lucky wheel.
type CheckinPrize struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Amount      float64 `json:"amount"`
	Probability float64 `json:"probability"`
	Color       string  `json:"color"`
	SortOrder   int     `json:"sort_order"`
}

// CheckinRecord is an immutable snapshot of a completed daily draw.
type CheckinRecord struct {
	ID          int64     `json:"id"`
	PrizeID     *int64    `json:"prize_id,omitempty"`
	PrizeName   string    `json:"prize_name"`
	Amount      float64   `json:"amount"`
	BonusAmount float64   `json:"bonus_amount"`
	TotalAmount float64   `json:"total_amount"`
	Probability float64   `json:"probability"`
	NewBalance  float64   `json:"new_balance"`
	StreakDays  int       `json:"streak_days"`
	CheckedAt   time.Time `json:"checked_at"`
}

// CheckinConfig controls the recurring consecutive-check-in reward.
// StreakTarget is intentionally exposed so the client can render the same
// progress target as the server, while the current product default is seven.
type CheckinConfig struct {
	StreakBonusAmount float64 `json:"streak_bonus_amount"`
	StreakTarget      int     `json:"streak_target"`
}

// CheckinStatus describes today's state and the current wheel configuration.
type CheckinStatus struct {
	Date              string         `json:"date"`
	CheckedToday      bool           `json:"checked_today"`
	CanCheckin        bool           `json:"can_checkin"`
	StreakDays        int            `json:"streak_days"`
	StreakTarget      int            `json:"streak_target"`
	StreakBonusAmount float64        `json:"streak_bonus_amount"`
	DaysUntilBonus    int            `json:"days_until_bonus"`
	Prizes            []CheckinPrize `json:"prizes"`
	TodayResult       *CheckinRecord `json:"today_result,omitempty"`
}

// CheckinRepository owns the transaction that records a draw and credits balance.
type CheckinRepository interface {
	ListCheckinPrizes(ctx context.Context) ([]CheckinPrize, error)
	GetCheckinByDate(ctx context.Context, userID int64, date string) (*CheckinRecord, error)
	GetConsecutiveCheckinDays(ctx context.Context, userID int64, date string) (int, error)
	ListCheckinsByUser(ctx context.Context, userID int64, limit int) ([]CheckinRecord, error)
	DrawCheckin(ctx context.Context, userID int64, date string, randomUnit float64, streakBonusAmount float64) (*CheckinRecord, error)
	ReplaceCheckinPrizes(ctx context.Context, prizes []CheckinPrize) ([]CheckinPrize, error)
}

// CheckinService implements daily lucky-wheel check-in behavior.
type CheckinService struct {
	repo         CheckinRepository
	billingCache *BillingCacheService
	settingRepo  SettingRepository
}

func NewCheckinService(repo CheckinRepository, billingCache *BillingCacheService, settingRepo SettingRepository) *CheckinService {
	return &CheckinService{repo: repo, billingCache: billingCache, settingRepo: settingRepo}
}

func checkinDate(now time.Time) string {
	return now.In(checkinLocation).Format("2006-01-02")
}

func defaultCheckinConfig() CheckinConfig {
	return CheckinConfig{StreakBonusAmount: defaultStreakBonus, StreakTarget: checkinStreakTarget}
}

func (s *CheckinService) GetConfig(ctx context.Context) (*CheckinConfig, error) {
	config := defaultCheckinConfig()
	if s.settingRepo == nil {
		return &config, nil
	}
	raw, err := s.settingRepo.GetValue(ctx, "daily_checkin_streak_bonus_amount")
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return &config, nil
		}
		return nil, fmt.Errorf("get check-in configuration: %w", err)
	}
	if raw != "" {
		var amount float64
		if _, scanErr := fmt.Sscan(strings.TrimSpace(raw), &amount); scanErr != nil || math.IsNaN(amount) || math.IsInf(amount, 0) || amount < 0 || amount > 1000000 {
			return nil, fmt.Errorf("invalid daily check-in streak bonus amount")
		}
		config.StreakBonusAmount = amount
	}
	return &config, nil
}

func (s *CheckinService) UpdateConfig(ctx context.Context, streakBonusAmount float64) (*CheckinConfig, error) {
	if math.IsNaN(streakBonusAmount) || math.IsInf(streakBonusAmount, 0) || streakBonusAmount < 0 || streakBonusAmount > 1000000 {
		return nil, infraerrors.BadRequest("INVALID_CHECKIN_CONFIG", "streak bonus amount must be between 0 and 1000000")
	}
	if s.settingRepo == nil {
		config := defaultCheckinConfig()
		config.StreakBonusAmount = streakBonusAmount
		return &config, nil
	}
	if err := s.settingRepo.Set(ctx, "daily_checkin_streak_bonus_amount", fmt.Sprintf("%.8f", streakBonusAmount)); err != nil {
		return nil, fmt.Errorf("save check-in configuration: %w", err)
	}
	return s.GetConfig(ctx)
}

func (s *CheckinService) GetStatus(ctx context.Context, userID int64) (*CheckinStatus, error) {
	prizes, err := s.repo.ListCheckinPrizes(ctx)
	if err != nil {
		return nil, fmt.Errorf("list check-in prizes: %w", err)
	}
	date := checkinDate(time.Now())
	record, err := s.repo.GetCheckinByDate(ctx, userID, date)
	if err != nil {
		return nil, fmt.Errorf("get today's check-in: %w", err)
	}
	config, err := s.GetConfig(ctx)
	if err != nil {
		return nil, err
	}
	streakDays, err := s.repo.GetConsecutiveCheckinDays(ctx, userID, date)
	if err != nil {
		return nil, fmt.Errorf("get check-in streak: %w", err)
	}
	configured := ValidateCheckinPrizes(prizes) == nil
	daysUntilBonus := config.StreakTarget - (streakDays % config.StreakTarget)
	if daysUntilBonus == 0 {
		daysUntilBonus = config.StreakTarget
	}
	return &CheckinStatus{
		Date:              date,
		CheckedToday:      record != nil,
		CanCheckin:        record == nil && configured,
		StreakDays:        streakDays,
		StreakTarget:      config.StreakTarget,
		StreakBonusAmount: config.StreakBonusAmount,
		DaysUntilBonus:    daysUntilBonus,
		Prizes:            prizes,
		TodayResult:       record,
	}, nil
}

func (s *CheckinService) Draw(ctx context.Context, userID int64) (*CheckinRecord, error) {
	unit, err := secureRandomUnit()
	if err != nil {
		return nil, fmt.Errorf("generate check-in randomness: %w", err)
	}
	config, err := s.GetConfig(ctx)
	if err != nil {
		return nil, err
	}
	record, err := s.repo.DrawCheckin(ctx, userID, checkinDate(time.Now()), unit, config.StreakBonusAmount)
	if err != nil {
		switch {
		case errors.Is(err, ErrCheckinAlreadyClaimed):
			return nil, infraerrors.Conflict("CHECKIN_ALREADY_CLAIMED", "You have already checked in today")
		case errors.Is(err, ErrCheckinNotConfigured):
			return nil, infraerrors.ServiceUnavailable("CHECKIN_NOT_CONFIGURED", "The lucky wheel is not configured correctly")
		default:
			return nil, fmt.Errorf("draw daily check-in: %w", err)
		}
	}
	if s.billingCache != nil {
		_ = s.billingCache.InvalidateUserBalance(ctx, userID)
	}
	return record, nil
}

func (s *CheckinService) ListPrizes(ctx context.Context) ([]CheckinPrize, error) {
	return s.repo.ListCheckinPrizes(ctx)
}

// ListHistory returns the user's most recent completed check-in rewards.
func (s *CheckinService) ListHistory(ctx context.Context, userID int64, limit int) ([]CheckinRecord, error) {
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}
	return s.repo.ListCheckinsByUser(ctx, userID, limit)
}

func (s *CheckinService) ReplacePrizes(ctx context.Context, prizes []CheckinPrize) ([]CheckinPrize, error) {
	for i := range prizes {
		prizes[i].ID = 0
		prizes[i].Name = strings.TrimSpace(prizes[i].Name)
		prizes[i].Color = strings.ToUpper(strings.TrimSpace(prizes[i].Color))
		prizes[i].SortOrder = i
	}
	if err := ValidateCheckinPrizes(prizes); err != nil {
		return nil, infraerrors.BadRequest("INVALID_CHECKIN_PRIZES", err.Error())
	}
	updated, err := s.repo.ReplaceCheckinPrizes(ctx, prizes)
	if err != nil {
		return nil, fmt.Errorf("replace check-in prizes: %w", err)
	}
	return updated, nil
}

// ValidateCheckinPrizes enforces an honest wheel: displayed probabilities add up to 100%.
func ValidateCheckinPrizes(prizes []CheckinPrize) error {
	if len(prizes) < 2 {
		return errors.New("at least two prizes are required")
	}
	if len(prizes) > maxCheckinPrizes {
		return fmt.Errorf("no more than %d prizes are allowed", maxCheckinPrizes)
	}
	total := 0.0
	for i := range prizes {
		prize := prizes[i]
		name := strings.TrimSpace(prize.Name)
		if name == "" || len([]rune(name)) > 80 {
			return fmt.Errorf("prize %d must have a name of at most 80 characters", i+1)
		}
		if math.IsNaN(prize.Amount) || math.IsInf(prize.Amount, 0) || prize.Amount < 0 || prize.Amount > 1000000 {
			return fmt.Errorf("prize %d has an invalid amount", i+1)
		}
		if math.IsNaN(prize.Probability) || math.IsInf(prize.Probability, 0) || prize.Probability <= 0 || prize.Probability > 100 {
			return fmt.Errorf("prize %d has an invalid probability", i+1)
		}
		if !checkinColorPattern.MatchString(strings.TrimSpace(prize.Color)) {
			return fmt.Errorf("prize %d has an invalid color", i+1)
		}
		total += prize.Probability
	}
	if math.Abs(total-checkinProbabilityTotal) > 0.000001 {
		return fmt.Errorf("prize probabilities must add up to 100%% (current total: %.6g%%)", total)
	}
	return nil
}

// SelectCheckinPrize maps a uniformly distributed [0,1) value to a configured prize.
func SelectCheckinPrize(prizes []CheckinPrize, randomUnit float64) (*CheckinPrize, error) {
	if err := ValidateCheckinPrizes(prizes); err != nil {
		return nil, ErrCheckinNotConfigured
	}
	if randomUnit < 0 || randomUnit >= 1 || math.IsNaN(randomUnit) {
		return nil, errors.New("random value must be in [0,1)")
	}
	target := randomUnit * checkinProbabilityTotal
	cumulative := 0.0
	for i := range prizes {
		cumulative += prizes[i].Probability
		if target < cumulative || i == len(prizes)-1 {
			selected := prizes[i]
			return &selected, nil
		}
	}
	return nil, ErrCheckinNotConfigured
}

func secureRandomUnit() (float64, error) {
	const precision = int64(1) << 53
	n, err := cryptorand.Int(cryptorand.Reader, big.NewInt(precision))
	if err != nil {
		return 0, err
	}
	return float64(n.Int64()) / float64(precision), nil
}
