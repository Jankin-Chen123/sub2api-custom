package service

import (
	"context"
	"errors"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type checkinRepoStub struct {
	prizes       []CheckinPrize
	record       *CheckinRecord
	history      []CheckinRecord
	drawRecord   *CheckinRecord
	drawErr      error
	replaced     []CheckinPrize
	drawUserID   int64
	drawDate     string
	drawUnitSeen float64
	streakDays   int
}

func (s *checkinRepoStub) ListCheckinPrizes(context.Context) ([]CheckinPrize, error) {
	return append([]CheckinPrize(nil), s.prizes...), nil
}

func (s *checkinRepoStub) GetCheckinByDate(context.Context, int64, string) (*CheckinRecord, error) {
	return s.record, nil
}

func (s *checkinRepoStub) GetConsecutiveCheckinDays(context.Context, int64, string) (int, error) {
	return s.streakDays, nil
}

func (s *checkinRepoStub) ListCheckinsByUser(context.Context, int64, int) ([]CheckinRecord, error) {
	return append([]CheckinRecord(nil), s.history...), nil
}

func (s *checkinRepoStub) DrawCheckin(_ context.Context, userID int64, date string, unit float64, _ float64) (*CheckinRecord, error) {
	s.drawUserID = userID
	s.drawDate = date
	s.drawUnitSeen = unit
	return s.drawRecord, s.drawErr
}

func (s *checkinRepoStub) ReplaceCheckinPrizes(_ context.Context, prizes []CheckinPrize) ([]CheckinPrize, error) {
	s.replaced = append([]CheckinPrize(nil), prizes...)
	return prizes, nil
}

func validCheckinPrizes() []CheckinPrize {
	return []CheckinPrize{
		{ID: 1, Name: "Small", Amount: 0.01, Probability: 70, Color: "#60A5FA"},
		{ID: 2, Name: "Large", Amount: 1, Probability: 30, Color: "#F97316"},
	}
}

func TestSelectCheckinPrizeUsesConfiguredProbabilityBoundaries(t *testing.T) {
	prizes := validCheckinPrizes()

	got, err := SelectCheckinPrize(prizes, 0)
	require.NoError(t, err)
	require.Equal(t, int64(1), got.ID)

	got, err = SelectCheckinPrize(prizes, 0.699999)
	require.NoError(t, err)
	require.Equal(t, int64(1), got.ID)

	got, err = SelectCheckinPrize(prizes, 0.7)
	require.NoError(t, err)
	require.Equal(t, int64(2), got.ID)
}

func TestValidateCheckinPrizesRequiresExactHundredPercent(t *testing.T) {
	prizes := validCheckinPrizes()
	prizes[1].Probability = 29

	err := ValidateCheckinPrizes(prizes)
	require.EqualError(t, err, "prize probabilities must add up to 100% (current total: 99%)")
}

func TestCheckinStatusReflectsExistingDailyRecord(t *testing.T) {
	record := &CheckinRecord{ID: 9, PrizeName: "Small", Amount: 0.01}
	repo := &checkinRepoStub{prizes: validCheckinPrizes(), record: record}
	svc := NewCheckinService(repo, nil, nil)

	status, err := svc.GetStatus(context.Background(), 42)
	require.NoError(t, err)
	require.True(t, status.CheckedToday)
	require.False(t, status.CanCheckin)
	require.Same(t, record, status.TodayResult)
}

func TestListCheckinHistoryReturnsRecentRewards(t *testing.T) {
	repo := &checkinRepoStub{history: []CheckinRecord{{ID: 3, Amount: 0.5}}}
	svc := NewCheckinService(repo, nil, nil)

	history, err := svc.ListHistory(context.Background(), 42, 25)
	require.NoError(t, err)
	require.Len(t, history, 1)
	require.Equal(t, int64(3), history[0].ID)
}

func TestReplaceCheckinPrizesNormalizesAdminInput(t *testing.T) {
	repo := &checkinRepoStub{}
	svc := NewCheckinService(repo, nil, nil)

	updated, err := svc.ReplacePrizes(context.Background(), []CheckinPrize{
		{ID: 99, Name: "  Small  ", Amount: 0.01, Probability: 70, Color: "#60a5fa"},
		{ID: 100, Name: "Large", Amount: 1, Probability: 30, Color: "#f97316"},
	})
	require.NoError(t, err)
	require.Equal(t, "Small", updated[0].Name)
	require.Equal(t, "#60A5FA", updated[0].Color)
	require.Zero(t, updated[0].ID)
	require.Equal(t, 1, updated[1].SortOrder)
}

func TestDrawMapsDuplicateToConflict(t *testing.T) {
	repo := &checkinRepoStub{drawErr: ErrCheckinAlreadyClaimed}
	svc := NewCheckinService(repo, nil, nil)

	_, err := svc.Draw(context.Background(), 42)
	require.Error(t, err)
	require.True(t, errors.Is(err, infraerrors.Conflict("CHECKIN_ALREADY_CLAIMED", "")))
	require.Equal(t, "CHECKIN_ALREADY_CLAIMED", infraerrors.Reason(err))
	require.Equal(t, int64(42), repo.drawUserID)
	require.NotEmpty(t, repo.drawDate)
	require.GreaterOrEqual(t, repo.drawUnitSeen, 0.0)
	require.Less(t, repo.drawUnitSeen, 1.0)
}
