package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type imageAdmissionCacheStub struct {
	mu         sync.Mutex
	acquire    []bool
	acquireErr error
	dimensions []ImageConcurrencyDimension
	released   int
}

func (s *imageAdmissionCacheStub) AcquireAccountSlot(context.Context, int64, int, string) (bool, error) {
	return true, nil
}
func (s *imageAdmissionCacheStub) ReleaseAccountSlot(context.Context, int64, string) error {
	return nil
}
func (s *imageAdmissionCacheStub) GetAccountConcurrency(context.Context, int64) (int, error) {
	return 0, nil
}
func (s *imageAdmissionCacheStub) GetAccountConcurrencyBatch(context.Context, []int64) (map[int64]int, error) {
	return map[int64]int{}, nil
}
func (s *imageAdmissionCacheStub) IncrementAccountWaitCount(context.Context, int64, int) (bool, error) {
	return true, nil
}
func (s *imageAdmissionCacheStub) DecrementAccountWaitCount(context.Context, int64) error { return nil }
func (s *imageAdmissionCacheStub) GetAccountWaitingCount(context.Context, int64) (int, error) {
	return 0, nil
}
func (s *imageAdmissionCacheStub) AcquireUserSlot(context.Context, int64, int, string) (bool, error) {
	return true, nil
}
func (s *imageAdmissionCacheStub) ReleaseUserSlot(context.Context, int64, string) error { return nil }
func (s *imageAdmissionCacheStub) GetUserConcurrency(context.Context, int64) (int, error) {
	return 0, nil
}
func (s *imageAdmissionCacheStub) IncrementWaitCount(context.Context, int64, int) (bool, error) {
	return true, nil
}
func (s *imageAdmissionCacheStub) DecrementWaitCount(context.Context, int64) error { return nil }
func (s *imageAdmissionCacheStub) GetAccountsLoadBatch(context.Context, []AccountWithConcurrency) (map[int64]*AccountLoadInfo, error) {
	return map[int64]*AccountLoadInfo{}, nil
}
func (s *imageAdmissionCacheStub) GetUsersLoadBatch(context.Context, []UserWithConcurrency) (map[int64]*UserLoadInfo, error) {
	return map[int64]*UserLoadInfo{}, nil
}
func (s *imageAdmissionCacheStub) CleanupExpiredAccountSlots(context.Context, int64) error {
	return nil
}
func (s *imageAdmissionCacheStub) CleanupExpiredAccountSlotKeys(context.Context) error    { return nil }
func (s *imageAdmissionCacheStub) CleanupStaleProcessSlots(context.Context, string) error { return nil }

func (s *imageAdmissionCacheStub) AcquireImageSlots(_ context.Context, dimensions []ImageConcurrencyDimension, _ string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dimensions = append([]ImageConcurrencyDimension(nil), dimensions...)
	if s.acquireErr != nil {
		return false, s.acquireErr
	}
	if len(s.acquire) == 0 {
		return true, nil
	}
	result := s.acquire[0]
	s.acquire = s.acquire[1:]
	return result, nil
}

func (s *imageAdmissionCacheStub) ReleaseImageSlots(context.Context, []ImageConcurrencyDimension, string) error {
	s.mu.Lock()
	s.released++
	s.mu.Unlock()
	return nil
}

func TestImageGenerationAdmission_DisabledDoesNotNeedRedis(t *testing.T) {
	admission := NewImageGenerationAdmission(nil, ImageGenerationAdmissionConfig{Enabled: false})
	release, acquired, err := admission.Acquire(context.Background(), ImageGenerationAdmissionRequest{UserID: 1})
	require.NoError(t, err)
	require.True(t, acquired)
	require.Nil(t, release)
}

func TestImageGenerationAdmission_AcquiresAllConfiguredDimensionsAndReleases(t *testing.T) {
	cache := &imageAdmissionCacheStub{}
	service := NewConcurrencyService(cache)
	admission := NewImageGenerationAdmission(service, ImageGenerationAdmissionConfig{
		Enabled:            true,
		MaxPerUser:         2,
		MaxPerAPIKey:       3,
		MaxPerGroup:        4,
		MaxPerAccount:      5,
		Max4KConcurrent:    1,
		OverflowMode:       "reject",
		MaxWaitingRequests: 10,
	})

	release, acquired, err := admission.Acquire(context.Background(), ImageGenerationAdmissionRequest{
		UserID: 10, APIKeyID: 20, GroupID: 30, AccountID: 40, Tier: ImageConcurrencyTier4K,
	})
	require.NoError(t, err)
	require.True(t, acquired)
	require.NotNil(t, release)

	require.Equal(t, []ImageConcurrencyDimension{
		{Name: ImageConcurrencyTier4K, ID: 1, Max: 1},
		{Name: ImageConcurrencyDimensionAccount, ID: 40, Max: 5},
		{Name: ImageConcurrencyDimensionAPIKey, ID: 20, Max: 3},
		{Name: ImageConcurrencyDimensionGroup, ID: 30, Max: 4},
		{Name: ImageConcurrencyDimensionUser, ID: 10, Max: 2},
	}, cache.dimensions)
	release()
	require.Equal(t, 1, cache.released)

	// Release is idempotent at the caller boundary; Redis ZREM itself is also
	// harmless when a process crashed after the lease TTL expired.
	release()
	require.Equal(t, 1, cache.released)
}

func TestImageGenerationAdmission_RejectsAndWaitsWithContext(t *testing.T) {
	cache := &imageAdmissionCacheStub{acquire: []bool{false}}
	service := NewConcurrencyService(cache)
	admission := NewImageGenerationAdmission(service, ImageGenerationAdmissionConfig{
		Enabled: true, MaxPerUser: 1, OverflowMode: "reject",
	})
	release, acquired, err := admission.Acquire(context.Background(), ImageGenerationAdmissionRequest{UserID: 1})
	require.NoError(t, err)
	require.False(t, acquired)
	require.Nil(t, release)

	cache.acquire = []bool{false}
	waiting := NewImageGenerationAdmission(service, ImageGenerationAdmissionConfig{
		Enabled: true, MaxPerUser: 1, OverflowMode: "wait", WaitTimeout: 10 * time.Millisecond,
	})
	release, acquired, err = waiting.Acquire(context.Background(), ImageGenerationAdmissionRequest{UserID: 1})
	require.NoError(t, err)
	require.False(t, acquired)
	require.Nil(t, release)
}

func TestImageGenerationAdmission_ReportsCacheFailureWithoutBypassingLimit(t *testing.T) {
	cache := &imageAdmissionCacheStub{acquireErr: errors.New("redis down")}
	service := NewConcurrencyService(cache)
	admission := NewImageGenerationAdmission(service, ImageGenerationAdmissionConfig{Enabled: true, MaxPerGroup: 1})
	_, acquired, err := admission.Acquire(context.Background(), ImageGenerationAdmissionRequest{GroupID: 7})
	require.ErrorIs(t, err, ErrImageGenerationAdmissionUnavailable)
	require.False(t, acquired)
}

func TestDedicatedImageAccountSelector_AcquiresExecutionLeaseForPolling(t *testing.T) {
	cache := &imageAdmissionCacheStub{}
	admission := NewImageGenerationAdmission(NewConcurrencyService(cache), ImageGenerationAdmissionConfig{
		Enabled: true, MaxPerUser: 2, MaxPerAPIKey: 2, MaxPerGroup: 2, MaxPerAccount: 1, Max4KConcurrent: 1,
	})
	selector := &DedicatedImageAccountSelector{ImageAdmission: admission}
	userID, apiKeyID, groupID := int64(1), int64(2), int64(3)
	job := &ImageGenerationJob{UserID: &userID, APIKeyID: &apiKeyID, GroupID: &groupID, PublicModel: CangyuanImageModel4K}

	release, acquired, err := selector.AcquireExecution(context.Background(), job, 44)
	require.NoError(t, err)
	require.True(t, acquired)
	require.NotNil(t, release)
	require.Len(t, cache.dimensions, 5)
	release()
	require.Equal(t, 1, cache.released)
}
