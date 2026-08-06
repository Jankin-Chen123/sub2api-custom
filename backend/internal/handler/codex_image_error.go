package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// codexDedicatedImageErrorDetails converts an error raised by the dedicated
// image bridge into a client-safe OpenAI error response. The bridge may wrap
// the adapter error while crossing the HTTP or WebSocket ingress layers, so
// errors.As is intentionally used here.
func codexDedicatedImageErrorDetails(err error) (status int, errType, code, message string, ok bool) {
	var adapterErr *service.CangyuanAdapterError
	if !errors.As(err, &adapterErr) || adapterErr == nil {
		return 0, "", "", "", false
	}

	code = strings.TrimSpace(adapterErr.Code)
	if code == "" {
		return 0, "", "", "", false
	}
	status = adapterErr.HTTPStatus
	if status < http.StatusBadRequest || status > 599 {
		status = http.StatusBadGateway
	}
	if status >= http.StatusBadRequest && status < http.StatusInternalServerError {
		errType = "invalid_request_error"
	} else {
		errType = "upstream_error"
	}

	message = strings.TrimSpace(service.RedactImageGenerationErrorMessage(adapterErr.Error(), 1024))
	if message == "" {
		message = "dedicated image request failed"
	}
	return status, errType, code, message, true
}

func codexDedicatedImageWebSocketErrorCode(err error) (string, bool) {
	_, _, code, _, ok := codexDedicatedImageErrorDetails(err)
	return code, ok
}
