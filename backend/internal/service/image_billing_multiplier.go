package service

import "errors"

type DedicatedImageCostEstimate struct {
	BaseCost       float64
	RateMultiplier float64
	ActualCost     float64
}

func resolveImageRateMultiplier(apiKey *APIKey, effectiveGroupMultiplier float64) float64 {
	if apiKey != nil && apiKey.Group != nil && apiKey.Group.ImageRateIndependent {
		if apiKey.Group.ImageRateMultiplier < 0 {
			return 0
		}
		return apiKey.Group.ImageRateMultiplier
	}
	return effectiveGroupMultiplier
}

func resolveVideoRateMultiplier(apiKey *APIKey, effectiveGroupMultiplier float64) float64 {
	if apiKey != nil && apiKey.Group != nil && apiKey.Group.VideoRateIndependent {
		if apiKey.Group.VideoRateMultiplier < 0 {
			return 0
		}
		return apiKey.Group.VideoRateMultiplier
	}
	return effectiveGroupMultiplier
}

func EstimateDedicatedImageCost(billing *BillingService, apiKey *APIKey, model, sizeTier string) (float64, error) {
	estimate, err := EstimateDedicatedImageCostSnapshot(billing, apiKey, model, sizeTier)
	if err != nil {
		return 0, err
	}
	return estimate.ActualCost, nil
}

func EstimateDedicatedImageCostSnapshot(billing *BillingService, apiKey *APIKey, model, sizeTier string) (*DedicatedImageCostEstimate, error) {
	if billing == nil || apiKey == nil || apiKey.Group == nil {
		return nil, errors.New("image billing context is unavailable")
	}
	multiplier := resolveImageRateMultiplier(apiKey, apiKey.Group.RateMultiplier)
	cost := billing.CalculateImageCost(model, sizeTier, 1, imagePriceConfigFromAPIKey(apiKey), multiplier)
	if cost == nil || cost.ActualCost < 0 {
		return nil, errors.New("image cost could not be estimated")
	}
	return &DedicatedImageCostEstimate{BaseCost: cost.TotalCost, RateMultiplier: multiplier, ActualCost: cost.ActualCost}, nil
}
