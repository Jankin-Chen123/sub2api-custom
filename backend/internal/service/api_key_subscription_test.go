package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type subscriptionAPIKeyRepoStub struct {
	APIKeyRepository
	keys        []APIKey
	createCalls int
}

func (r *subscriptionAPIKeyRepoStub) ListByUserID(_ context.Context, userID int64, _ pagination.PaginationParams, filters APIKeyListFilters) ([]APIKey, *pagination.PaginationResult, error) {
	out := make([]APIKey, 0)
	for _, key := range r.keys {
		if key.UserID != userID || filters.Status != key.Status {
			continue
		}
		if filters.GroupID != nil && (key.GroupID == nil || *key.GroupID != *filters.GroupID) {
			continue
		}
		out = append(out, key)
	}
	return out, &pagination.PaginationResult{Total: int64(len(out))}, nil
}

func (r *subscriptionAPIKeyRepoStub) Create(_ context.Context, key *APIKey) error {
	r.createCalls++
	key.ID = int64(100 + r.createCalls)
	r.keys = append(r.keys, *key)
	return nil
}

type subscriptionAPIKeyUserRepoStub struct{ UserRepository }

func (subscriptionAPIKeyUserRepoStub) GetByID(_ context.Context, id int64) (*User, error) {
	return &User{ID: id}, nil
}

type subscriptionAPIKeyGroupRepoStub struct{ GroupRepository }

func (subscriptionAPIKeyGroupRepoStub) GetByID(_ context.Context, id int64) (*Group, error) {
	return &Group{ID: id, Name: "Pro", SubscriptionType: SubscriptionTypeSubscription}, nil
}

type subscriptionAPIKeySubscriptionRepoStub struct{ UserSubscriptionRepository }

func (subscriptionAPIKeySubscriptionRepoStub) GetActiveByUserIDAndGroupID(_ context.Context, userID, groupID int64) (*UserSubscription, error) {
	return &UserSubscription{UserID: userID, GroupID: groupID, Status: SubscriptionStatusActive}, nil
}

func TestEnsureSubscriptionAPIKeyReusesExistingGroupKey(t *testing.T) {
	groupID := int64(7)
	repo := &subscriptionAPIKeyRepoStub{keys: []APIKey{{
		ID: 1, UserID: 42, GroupID: &groupID, Key: "sk-existing", Status: StatusAPIKeyActive,
	}}}
	svc := &APIKeyService{apiKeyRepo: repo}

	key, err := svc.EnsureSubscriptionAPIKey(context.Background(), 42, groupID, "Pro")

	require.NoError(t, err)
	require.Equal(t, "sk-existing", key.Key)
	require.Zero(t, repo.createCalls)
}

func TestEnsureSubscriptionAPIKeyCreatesThroughExistingCreateFlow(t *testing.T) {
	groupID := int64(7)
	repo := &subscriptionAPIKeyRepoStub{}
	svc := &APIKeyService{
		apiKeyRepo:  repo,
		userRepo:    subscriptionAPIKeyUserRepoStub{},
		groupRepo:   subscriptionAPIKeyGroupRepoStub{},
		userSubRepo: subscriptionAPIKeySubscriptionRepoStub{},
		cfg:         &config.Config{},
	}

	key, err := svc.EnsureSubscriptionAPIKey(context.Background(), 42, groupID, "Pro")

	require.NoError(t, err)
	require.Equal(t, 1, repo.createCalls)
	require.Equal(t, "Subscription - Pro", key.Name)
	require.Equal(t, groupID, *key.GroupID)
	require.NotEmpty(t, key.Key)
	require.Equal(t, StatusActive, key.Status)
}
