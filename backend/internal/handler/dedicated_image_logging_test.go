package handler

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestDedicatedImageLogFieldsAllowlistDropsSensitiveCanaries(t *testing.T) {
	const canary = "dedicated-image-sensitive-canary"
	fields := dedicatedImageLogFields(
		zap.Int64("user_id", 12),
		zap.String("model", "gpt-image-2-1k"),
		zap.String("prompt", canary),
		zap.Strings("reference_images", []string{canary}),
		zap.String("mask", canary),
		zap.String("b64_json", canary),
		zap.String("authorization", "Bearer "+canary),
		zap.String("signed_url_query", canary),
		zap.String("upstream_response", canary),
		zap.String("upstream_task_id", canary),
	)

	core, logs := observer.New(zap.InfoLevel)
	zap.New(core).With(fields...).Info("dedicated_image_test")
	entry := logs.All()
	require.Len(t, entry, 1)
	require.NotContains(t, entry[0].Message, canary)
	for _, field := range entry[0].Context {
		require.NotContains(t, field.Key, canary)
		require.NotContains(t, field.String, canary)
	}
	keys := make([]string, 0, len(entry[0].Context))
	for _, field := range entry[0].Context {
		keys = append(keys, field.Key)
	}
	require.ElementsMatch(t, []string{"user_id", "model"}, keys)
}

func TestDedicatedImageLogFieldsKeepsOperationalFailureMetadata(t *testing.T) {
	fields := dedicatedImageLogFields(
		zap.String("job_id", "imgjob_test"),
		zap.String("status", "failed"),
		zap.String("error_code", "image_upstream_rate_limited"),
		zap.Int("http_status", 429),
		zap.Bool("retryable", true),
	)
	require.Len(t, fields, 5)
}
