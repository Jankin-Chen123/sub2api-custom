package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

type ImageGenerationProviderFactory interface {
	ForAccount(account *Account, requireImageOnly bool) (CangyuanImageClient, error)
}

type DefaultImageGenerationProviderFactory struct {
	HTTPClient *http.Client
}

func (f *DefaultImageGenerationProviderFactory) ForAccount(account *Account, requireImageOnly bool) (CangyuanImageClient, error) {
	if account == nil {
		return nil, errors.New("image provider account is required")
	}
	if requireImageOnly {
		return NewCangyuanImageAdapterFromAccount(account, f.HTTPClient)
	}
	// Already-submitted jobs stay pinned even if an administrator later changes
	// the account purpose or disables scheduling. Polling still uses the exact
	// stored account credentials and never resubmits elsewhere.
	apiKey := account.GetCredential("api_key")
	if apiKey == "" && account.IsOpenAIOAuth() {
		apiKey = account.GetOpenAIAccessToken()
	}
	return NewCangyuanImageAdapter(account.GetOpenAIBaseURL(), apiKey, f.HTTPClient)
}

type ImageGenerationAccountLease struct {
	Account   *Account
	ImageOnly bool
	Release   func()
}

type ImageGenerationAccountSelector interface {
	Select(ctx context.Context, job *ImageGenerationJob) (*ImageGenerationAccountLease, error)
	GetBoundAccount(ctx context.Context, accountID int64) (*Account, error)
}

type imageGenerationExecutionLeaser interface {
	AcquireExecution(ctx context.Context, job *ImageGenerationJob, accountID int64) (func(), bool, error)
}

type DedicatedImageAccountSelector struct {
	Gateway              *OpenAIGatewayService
	Accounts             AccountRepository
	ImageAdmission       *ImageGenerationAdmission
	AllowGeneralFallback bool
	Config               *config.Config
}

func (s *DedicatedImageAccountSelector) Select(ctx context.Context, job *ImageGenerationJob) (*ImageGenerationAccountLease, error) {
	if s == nil || s.Gateway == nil || job == nil {
		return nil, errors.New("dedicated image account selector is not configured")
	}
	allowGeneralFallback := s.AllowGeneralFallback
	if s.Config != nil {
		allowGeneralFallback = s.Config.DedicatedImageRuntime().FallbackToGeneral
	}
	selection, _, err := s.Gateway.SelectAccountWithSchedulerForDedicatedImages(
		ctx,
		job.GroupID,
		job.JobID,
		job.PublicModel,
		nil,
		OpenAIImagesCapabilityNative,
		allowGeneralFallback,
	)
	if err != nil {
		return nil, err
	}
	if selection == nil || selection.Account == nil || (!selection.Account.IsImageOnly() && !allowGeneralFallback) {
		if selection != nil && selection.ReleaseFunc != nil {
			selection.ReleaseFunc()
		}
		return nil, errors.New("no dedicated image account is available")
	}
	selectedAccount := selection.Account
	// Scheduler snapshots intentionally omit provider endpoint secrets and may
	// also carry only metadata. Hydrate the selected account before submission so
	// the worker never tries to initialize Cangyuan with a redacted base URL or
	// stale credentials. The persisted job still binds the account by ID.
	if s.Accounts != nil {
		fresh, resolveErr := s.Accounts.GetByID(ctx, selectedAccount.ID)
		if resolveErr != nil || fresh == nil {
			if selection.ReleaseFunc != nil {
				selection.ReleaseFunc()
			}
			if resolveErr != nil {
				return nil, fmt.Errorf("resolve selected image account: %w", resolveErr)
			}
			return nil, errors.New("selected image account is unavailable")
		}
		selectedAccount = fresh
	}
	recordImageGenerationAccountSelection(selectedAccount.IsImageOnly())
	imageRelease, acquired, err := s.AcquireExecution(ctx, job, accountIDOrZero(selectedAccount))
	if err != nil || !acquired {
		if selection.ReleaseFunc != nil {
			selection.ReleaseFunc()
		}
		if err != nil {
			return nil, err
		}
		return nil, errors.New("image concurrency limit exceeded")
	}
	var once sync.Once
	return &ImageGenerationAccountLease{
		Account:   selectedAccount,
		ImageOnly: selectedAccount.IsImageOnly(),
		Release: func() {
			once.Do(func() {
				if imageRelease != nil {
					imageRelease()
				}
				if selection.ReleaseFunc != nil {
					selection.ReleaseFunc()
				}
			})
		},
	}, nil
}

func accountIDOrZero(account *Account) int64 {
	if account == nil {
		return 0
	}
	return account.ID
}

func (s *DedicatedImageAccountSelector) AcquireExecution(ctx context.Context, job *ImageGenerationJob, accountID int64) (func(), bool, error) {
	if s == nil || s.ImageAdmission == nil {
		return nil, true, nil
	}
	if job == nil || accountID <= 0 {
		return nil, false, errors.New("image execution identity is incomplete")
	}
	var userID, apiKeyID, groupID int64
	if job.UserID != nil {
		userID = *job.UserID
	}
	if job.APIKeyID != nil {
		apiKeyID = *job.APIKeyID
	}
	if job.GroupID != nil {
		groupID = *job.GroupID
	}
	tier := ""
	if strings.Contains(strings.ToLower(strings.TrimSpace(job.PublicModel)), "4k") {
		tier = ImageConcurrencyTier4K
	}
	return s.ImageAdmission.Acquire(ctx, ImageGenerationAdmissionRequest{
		UserID: userID, APIKeyID: apiKeyID, GroupID: groupID, AccountID: accountID, Tier: tier,
	})
}

func (s *DedicatedImageAccountSelector) GetBoundAccount(ctx context.Context, accountID int64) (*Account, error) {
	if s == nil || s.Accounts == nil || accountID <= 0 {
		return nil, errors.New("bound image account resolver is not configured")
	}
	return s.Accounts.GetByID(ctx, accountID)
}
