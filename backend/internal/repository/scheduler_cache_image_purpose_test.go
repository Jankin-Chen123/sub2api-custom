package repository

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestBuildSchedulerMetadataAccountKeepsImageOnlyPurpose(t *testing.T) {
	account := service.Account{
		ID:       901,
		Platform: service.PlatformOpenAI,
		Type:     service.AccountTypeAPIKey,
		Extra:    map[string]any{service.AccountPurposeExtraKey: service.AccountPurposeImageOnly},
	}

	metadata := buildSchedulerMetadataAccount(account)

	require.Equal(t, service.AccountPurposeImageOnly, metadata.AccountPurpose())
	require.Equal(t, service.AccountPurposeImageOnly, metadata.Extra[service.AccountPurposeExtraKey])
}
