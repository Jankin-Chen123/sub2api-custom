package handler

import (
	"errors"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestCodexDedicatedImageErrorDetailsPreservesStablePlanCode(t *testing.T) {
	err := &service.CangyuanAdapterError{
		Code:       "image_plan_invalid",
		HTTPStatus: http.StatusBadRequest,
		Err:        errors.New("visual prompt contains an unexpanded conversation reference"),
	}

	status, errType, code, message, ok := codexDedicatedImageErrorDetails(err)
	require.True(t, ok)
	require.Equal(t, http.StatusBadRequest, status)
	require.Equal(t, "invalid_request_error", errType)
	require.Equal(t, "image_plan_invalid", code)
	require.Contains(t, message, "unexpanded conversation reference")
}

func TestCodexDedicatedImageErrorDetailsFindsWrappedAdapterError(t *testing.T) {
	err := errors.New("bridge wrapper")
	err = errors.Join(err, &service.CangyuanAdapterError{
		Code:       "image_upstream_auth_failed",
		HTTPStatus: http.StatusUnauthorized,
		Err:        errors.New("Cangyuan rejected the image request"),
	})

	status, errType, code, _, ok := codexDedicatedImageErrorDetails(err)
	require.True(t, ok)
	require.Equal(t, http.StatusUnauthorized, status)
	require.Equal(t, "invalid_request_error", errType)
	require.Equal(t, "image_upstream_auth_failed", code)
}

func TestCodexDedicatedImageWebSocketErrorCode(t *testing.T) {
	wrapped := errors.Join(errors.New("ingress turn failed"), &service.CangyuanAdapterError{
		Code:       "image_plan_invalid",
		HTTPStatus: http.StatusBadRequest,
		Err:        errors.New("invalid plan"),
	})

	code, ok := codexDedicatedImageWebSocketErrorCode(wrapped)
	require.True(t, ok)
	require.Equal(t, "image_plan_invalid", code)
}

func TestCodexDedicatedImageErrorDetailsRedactsProviderSecrets(t *testing.T) {
	secret := "sk-provider-secret-123456"
	err := &service.CangyuanAdapterError{
		Code:       "image_upstream_rejected",
		HTTPStatus: http.StatusBadGateway,
		Err:        errors.New("Authorization: Bearer " + secret + " https://provider.example/image.png?signature=private"),
	}

	_, _, _, message, ok := codexDedicatedImageErrorDetails(err)
	require.True(t, ok)
	require.NotContains(t, message, secret)
	require.NotContains(t, message, "signature=private")
}
