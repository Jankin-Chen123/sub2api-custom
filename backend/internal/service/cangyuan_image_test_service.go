package service

import (
	"context"
	"errors"
	"strings"
	"time"
)

const (
	defaultCangyuanImageTestPrompt = "Generate a simple cute orange puppy sticker on a clean pastel background."
	cangyuanImageTestTimeout       = 3 * time.Minute
	cangyuanImageTestPollInterval  = 2 * time.Second
)

// CangyuanImageTestResult deliberately contains only public test metadata. It
// never carries the provider key, signed output URL, or provider task ID.
type CangyuanImageTestResult struct {
	Model     string
	Status    string
	Completed bool
	Duration  time.Duration
}

// RunCangyuanImageAccountTest performs one explicitly requested, billable
// image generation against an image_only account. It is intentionally separate
// from the durable user job path: an administrator is testing the credential,
// not creating a user-owned task or charging a user balance.
func RunCangyuanImageAccountTest(ctx context.Context, account *Account, model, prompt string) (*CangyuanImageTestResult, error) {
	return testCangyuanImageAccountWithClient(ctx, account, model, prompt, nil, cangyuanImageTestPollInterval)
}

func testCangyuanImageAccountWithClient(
	ctx context.Context,
	account *Account,
	model string,
	prompt string,
	client CangyuanImageClient,
	pollInterval time.Duration,
) (*CangyuanImageTestResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if account == nil || !account.IsImageOnly() {
		return nil, errors.New("image_only account is required")
	}

	model = strings.TrimSpace(model)
	if model == "" {
		model = CangyuanImageModel1K
	}
	if mapped := strings.TrimSpace(account.GetMappedModel(model)); mapped != "" {
		model = mapped
	}
	tier, _, ok := cangyuanImageTier(model)
	if !ok {
		return nil, errors.New("unsupported Cangyuan image test model")
	}
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		prompt = defaultCangyuanImageTestPrompt
	}

	if client == nil {
		var err error
		client, err = NewCangyuanImageAdapterFromAccount(account, nil)
		if err != nil {
			return nil, err
		}
	}
	if pollInterval <= 0 {
		pollInterval = cangyuanImageTestPollInterval
	}

	started := time.Now()
	operationCtx, cancel := context.WithTimeout(ctx, cangyuanImageTestTimeout)
	defer cancel()
	request := CangyuanImageRequest{
		Model:            model,
		Prompt:           prompt,
		N:                1,
		ResponseFormat:   "url",
		Async:            false,
		ImageSize:        tier,
		OutputResolution: tier,
	}
	result, err := client.SubmitGeneration(operationCtx, request)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, errors.New("cangyuan returned an empty result")
	}
	if result.Failed {
		return nil, errors.New("cangyuan image test failed")
	}
	if result.Completed {
		return &CangyuanImageTestResult{Model: model, Status: "completed", Completed: true, Duration: time.Since(started)}, nil
	}
	if strings.TrimSpace(result.UpstreamTaskID) == "" {
		return nil, errors.New("Cangyuan returned an incomplete result")
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-operationCtx.Done():
			return nil, errors.New("Cangyuan image test timed out")
		case <-ticker.C:
			result, err = client.PollGeneration(operationCtx, result.UpstreamTaskID)
			if err != nil {
				return nil, err
			}
			if result == nil {
				return nil, errors.New("Cangyuan returned an empty polling result")
			}
			if result.Failed {
				return nil, errors.New("Cangyuan image test failed")
			}
			if result.Completed {
				return &CangyuanImageTestResult{Model: model, Status: "completed", Completed: true, Duration: time.Since(started)}, nil
			}
			if strings.TrimSpace(result.UpstreamTaskID) == "" {
				return nil, errors.New("Cangyuan polling result lost its task binding")
			}
		}
	}
}
