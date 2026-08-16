package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type affiliateRedeemAutoSettingsRepo struct {
	SettingRepository
	values map[string]string
}

func (r *affiliateRedeemAutoSettingsRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	result := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := r.values[key]; ok {
			result[key] = value
		}
	}
	return result, nil
}

func TestNormalizeAffiliateRedeemAutoAmounts(t *testing.T) {
	got, err := NormalizeAffiliateRedeemAutoAmounts([]float64{20, 10, 20, 5.5})

	require.NoError(t, err)
	require.Equal(t, []float64{5.5, 10, 20}, got)

	_, err = NormalizeAffiliateRedeemAutoAmounts([]float64{10.001})
	require.Error(t, err)

	_, err = NormalizeAffiliateRedeemAutoAmounts([]float64{0})
	require.Error(t, err)
}

func TestValidateAffiliateRedeemAutoAmountsRejectsConflict(t *testing.T) {
	err := ValidateAffiliateRedeemAutoAmounts([]float64{10, 20}, []float64{5, 10})

	require.Error(t, err)
}

func TestGetAffiliateRedeemAutoReviewDecision(t *testing.T) {
	repo := &affiliateRedeemAutoSettingsRepo{values: map[string]string{
		SettingKeyAffiliateRedeemAutoValidAmounts:    `[20, 10]`,
		SettingKeyAffiliateRedeemAutoExcludedAmounts: `[5]`,
	}}
	settings := &SettingService{settingRepo: repo}

	require.Equal(t, "valid", settings.GetAffiliateRedeemAutoReviewDecision(context.Background(), 10))
	require.Equal(t, "excluded", settings.GetAffiliateRedeemAutoReviewDecision(context.Background(), 5))
	require.Empty(t, settings.GetAffiliateRedeemAutoReviewDecision(context.Background(), 15))
}
