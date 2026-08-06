package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type cangyuanImageTestClient struct {
	submitRequest CangyuanImageRequest
	submitResult  *CangyuanImageResult
	pollResults   []*CangyuanImageResult
}

func (c *cangyuanImageTestClient) SubmitGeneration(_ context.Context, request CangyuanImageRequest) (*CangyuanImageResult, error) {
	c.submitRequest = request
	return c.submitResult, nil
}

func (c *cangyuanImageTestClient) SubmitEdit(context.Context, CangyuanImageRequest) (*CangyuanImageResult, error) {
	return nil, nil
}

func (c *cangyuanImageTestClient) PollGeneration(context.Context, string) (*CangyuanImageResult, error) {
	if len(c.pollResults) == 0 {
		return nil, nil
	}
	result := c.pollResults[0]
	c.pollResults = c.pollResults[1:]
	return result, nil
}

func (c *cangyuanImageTestClient) PollEdit(context.Context, string) (*CangyuanImageResult, error) {
	return nil, nil
}

func imageOnlyTestAccount() *Account {
	return &Account{
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Extra:       map[string]any{AccountPurposeExtraKey: AccountPurposeImageOnly},
		Credentials: map[string]any{"base_url": "https://ai.cangyuansuanli.cn/v1", "api_key": "test-key", "model_mapping": map[string]any{"gpt-image-2-1k": CangyuanImageModel1K}},
	}
}

func TestTestCangyuanImageAccountUsesExplicitOneKDefaults(t *testing.T) {
	client := &cangyuanImageTestClient{
		submitResult: &CangyuanImageResult{Status: "completed", Completed: true, Data: []CangyuanImageData{{URL: "https://example.test/image.png"}}},
	}
	result, err := testCangyuanImageAccountWithClient(context.Background(), imageOnlyTestAccount(), "", "", client, time.Millisecond)

	require.NoError(t, err)
	require.Equal(t, CangyuanImageModel1K, result.Model)
	require.Equal(t, "completed", result.Status)
	require.True(t, result.Completed)
	require.Equal(t, CangyuanImageModel1K, client.submitRequest.Model)
	require.Equal(t, "1K", client.submitRequest.ImageSize)
	require.Equal(t, "1K", client.submitRequest.OutputResolution)
	require.Equal(t, 1, client.submitRequest.N)
	require.Equal(t, "url", client.submitRequest.ResponseFormat)
	require.False(t, client.submitRequest.Async)
}

func TestTestCangyuanImageAccountPollsWithoutExposingProviderBinding(t *testing.T) {
	client := &cangyuanImageTestClient{
		submitResult: &CangyuanImageResult{Status: "processing", UpstreamTaskID: "provider-secret-task"},
		pollResults:  []*CangyuanImageResult{{Status: "completed", Completed: true, Data: []CangyuanImageData{{B64JSON: "image"}}}},
	}
	result, err := testCangyuanImageAccountWithClient(context.Background(), imageOnlyTestAccount(), CangyuanImageModel2K, "dog", client, time.Millisecond)

	require.NoError(t, err)
	require.Equal(t, CangyuanImageModel2K, result.Model)
	require.Equal(t, "completed", result.Status)
	require.GreaterOrEqual(t, result.Duration, time.Duration(0))
}

func TestTestCangyuanImageAccountRejectsNonImageAccount(t *testing.T) {
	client := &cangyuanImageTestClient{}
	account := imageOnlyTestAccount()
	account.Extra = nil
	_, err := testCangyuanImageAccountWithClient(context.Background(), account, CangyuanImageModel1K, "dog", client, time.Millisecond)

	require.Error(t, err)
	require.Empty(t, client.submitRequest.Model)
}
