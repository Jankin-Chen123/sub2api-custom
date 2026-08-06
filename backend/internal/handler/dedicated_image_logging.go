package handler

import "go.uber.org/zap"

// dedicatedImageLogFields is the single boundary for structured fields added
// by the dedicated image handlers. Image requests contain prompts, image
// bytes and provider credentials; those values must never be passed through
// as ad-hoc log fields. Keep this list deliberately small and operational.
func dedicatedImageLogFields(fields ...zap.Field) []zap.Field {
	allowed := map[string]struct{}{
		"user_id":         {},
		"api_key_id":      {},
		"group_id":        {},
		"request_id":      {},
		"job_id":          {},
		"operation":       {},
		"model":           {},
		"requested_size":  {},
		"actual_size":     {},
		"status":          {},
		"error_code":      {},
		"duration_ms":     {},
		"source":          {},
		"account_purpose": {},
		"image_tier":      {},
		"retryable":       {},
		"http_status":     {},
		"queue_depth":     {},
	}
	filtered := make([]zap.Field, 0, len(fields))
	for _, field := range fields {
		if _, ok := allowed[field.Key]; ok {
			filtered = append(filtered, field)
		}
	}
	return filtered
}
