package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type accountPurposeUpdateRepo struct {
	AdminAccountRepository
	account *Account
}

func (r *accountPurposeUpdateRepo) GetByID(context.Context, int64) (*Account, error) {
	return r.account, nil
}

func (r *accountPurposeUpdateRepo) Update(_ context.Context, account *Account) error {
	r.account = account
	return nil
}

func (r *accountPurposeUpdateRepo) UpdateWithAccountBillingSettings(
	ctx context.Context,
	account *Account,
	_ *bool,
	_ *bool,
	_ *float64,
) error {
	return r.Update(ctx, account)
}

func TestAdminServiceUpdateAccountPreservesImageOnlyPurposeOnSparseExtraEdit(t *testing.T) {
	repo := &accountPurposeUpdateRepo{account: &Account{
		ID:          1,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Credentials: validCangyuanCredentials(),
		Extra:       map[string]any{AccountPurposeExtraKey: AccountPurposeImageOnly},
	}}
	svc := &adminServiceImpl{accountRepo: repo}

	account, err := svc.UpdateAccount(context.Background(), 1, &UpdateAccountInput{
		Extra: map[string]any{"quota_used": float64(1)},
	})

	require.NoError(t, err)
	require.Equal(t, AccountPurposeImageOnly, account.AccountPurpose())
	require.True(t, account.IsImageOnly())
}

func TestAccountPurposeDefaultsToGeneral(t *testing.T) {
	tests := []map[string]any{
		nil,
		{},
		{AccountPurposeExtraKey: nil},
		{AccountPurposeExtraKey: ""},
		{AccountPurposeExtraKey: "legacy-unknown"},
	}
	for _, extra := range tests {
		account := &Account{Extra: extra}
		require.Equal(t, AccountPurposeGeneral, account.AccountPurpose())
		require.False(t, account.IsImageOnly())
	}
}

func TestAccountPurposeImageOnly(t *testing.T) {
	account := &Account{Extra: map[string]any{AccountPurposeExtraKey: AccountPurposeImageOnly}}
	require.Equal(t, AccountPurposeImageOnly, account.AccountPurpose())
	require.True(t, account.IsImageOnly())
}

func TestNormalizeAccountPurposeExtraGeneralKeepsLegacySparse(t *testing.T) {
	normalized, err := NormalizeAccountPurposeExtra(
		PlatformOpenAI,
		AccountTypeAPIKey,
		map[string]any{},
		map[string]any{AccountPurposeExtraKey: AccountPurposeGeneral, "other": true},
	)
	require.NoError(t, err)
	require.Equal(t, map[string]any{"other": true}, normalized)
}

func TestNormalizeAccountPurposeExtraRejectsUnknownPurpose(t *testing.T) {
	_, err := NormalizeAccountPurposeExtra(
		PlatformOpenAI,
		AccountTypeAPIKey,
		map[string]any{},
		map[string]any{AccountPurposeExtraKey: "video_only"},
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "account_purpose")
}

func TestNormalizeAccountPurposeExtraRejectsWrongAccountType(t *testing.T) {
	_, err := NormalizeAccountPurposeExtra(
		PlatformOpenAI,
		AccountTypeOAuth,
		validCangyuanCredentials(),
		map[string]any{AccountPurposeExtraKey: AccountPurposeImageOnly},
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "OpenAI platform and API-key")
}

func TestNormalizeAccountPurposeExtraValidatesDedicatedCredentials(t *testing.T) {
	tests := []struct {
		name        string
		credentials map[string]any
		errorText   string
	}{
		{
			name: "base url",
			credentials: map[string]any{
				"api_key":       "secret",
				"model_mapping": map[string]any{"gpt-image-2-1k": "gpt-image-2-1k"},
			},
			errorText: "HTTPS base_url",
		},
		{
			name: "api key",
			credentials: map[string]any{
				"base_url":      "https://ai.cangyuansuanli.cn",
				"model_mapping": map[string]any{"gpt-image-2-1k": "gpt-image-2-1k"},
			},
			errorText: "API key",
		},
		{
			name: "mapping",
			credentials: map[string]any{
				"base_url": "https://ai.cangyuansuanli.cn",
				"api_key":  "secret",
			},
			errorText: "model_mapping",
		},
		{
			name: "unsupported target",
			credentials: map[string]any{
				"base_url":      "https://ai.cangyuansuanli.cn",
				"api_key":       "secret",
				"model_mapping": map[string]any{"image": "other-image-model"},
			},
			errorText: "gpt-image-2-1k",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NormalizeAccountPurposeExtra(
				PlatformOpenAI,
				AccountTypeAPIKey,
				tt.credentials,
				map[string]any{AccountPurposeExtraKey: AccountPurposeImageOnly},
			)
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.errorText)
		})
	}
}

func TestNormalizeAccountPurposeExtraAcceptsCangyuanTiers(t *testing.T) {
	normalized, err := NormalizeAccountPurposeExtra(
		PlatformOpenAI,
		AccountTypeAPIKey,
		validCangyuanCredentials(),
		map[string]any{AccountPurposeExtraKey: AccountPurposeImageOnly},
	)
	require.NoError(t, err)
	require.Equal(t, AccountPurposeImageOnly, normalized[AccountPurposeExtraKey])
}

func TestNormalizeAccountPurposeExtraRejectsUnsafeCangyuanBaseURL(t *testing.T) {
	for _, baseURL := range []string{
		"https://ai.cangyuansuanli.cn?key=leak",
		"https://ai.cangyuansuanli.cn/v1#images",
		"https://user:password@ai.cangyuansuanli.cn/v1",
	} {
		t.Run(baseURL, func(t *testing.T) {
			credentials := validCangyuanCredentials()
			credentials["base_url"] = baseURL
			_, err := NormalizeAccountPurposeExtra(
				PlatformOpenAI,
				AccountTypeAPIKey,
				credentials,
				map[string]any{AccountPurposeExtraKey: AccountPurposeImageOnly},
			)
			require.Error(t, err)
			require.Contains(t, err.Error(), "without credentials")
		})
	}
}

func TestAccountSupportsCangyuanImageFallbackRequiresExplicitCangyuanConfig(t *testing.T) {
	valid := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"base_url":      "https://images.example.test/v1",
			"api_key":       "test-key",
			"model_mapping": map[string]any{"gpt-image-2-1k": CangyuanImageModel1K},
		},
	}
	require.True(t, valid.SupportsCangyuanImageFallback())

	noMapping := *valid
	noMapping.Credentials = map[string]any{
		"base_url": "https://images.example.test/v1",
		"api_key":  "test-key",
	}
	require.False(t, noMapping.SupportsCangyuanImageFallback())

	wrongTarget := *valid
	wrongTarget.Credentials = map[string]any{
		"base_url":      "https://images.example.test/v1",
		"api_key":       "test-key",
		"model_mapping": map[string]any{"gpt-image-2-1k": "gpt-image-1"},
	}
	require.False(t, wrongTarget.SupportsCangyuanImageFallback())

	imageOnly := *valid
	imageOnly.Extra = map[string]any{AccountPurposeExtraKey: AccountPurposeImageOnly}
	require.False(t, imageOnly.SupportsCangyuanImageFallback(), "image_only accounts must be selected through the dedicated execution stage")

	unsafeURL := *valid
	unsafeURL.Credentials = map[string]any{
		"base_url":      "https://images.example.test/v1?token=secret",
		"api_key":       "test-key",
		"model_mapping": map[string]any{"gpt-image-2-1k": CangyuanImageModel1K},
	}
	require.False(t, unsafeURL.SupportsCangyuanImageFallback())
}

func validCangyuanCredentials() map[string]any {
	return map[string]any{
		"base_url": "https://ai.cangyuansuanli.cn/v1",
		"api_key":  "secret",
		"model_mapping": map[string]any{
			"gpt-image-2-1k": "gpt-image-2-1k",
			"gpt-image-2-2k": "gpt-image-2-2k",
			"gpt-image-2-4k": "gpt-image-2-4k",
		},
	}
}
