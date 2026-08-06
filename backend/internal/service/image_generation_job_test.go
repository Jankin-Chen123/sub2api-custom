package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRedactImageGenerationErrorMessage(t *testing.T) {
	secret := "sk-abcdefghijklmnopqrstuvwxyz123456"
	message := "upstream Authorization: Bearer " + secret + " url=https://example.com/image.png?signature=private\nretry"
	redacted := RedactImageGenerationErrorMessage(message, 1024)
	require.NotContains(t, redacted, secret)
	require.NotContains(t, redacted, "signature=private")
	require.Contains(t, redacted, "[redacted]")
	require.NotContains(t, redacted, "\n")

	require.Len(t, RedactImageGenerationErrorMessage(strings.Repeat("x", 200), 128), 128)
}

func TestNewImageGenerationJobID(t *testing.T) {
	first, err := NewImageGenerationJobID()
	require.NoError(t, err)
	second, err := NewImageGenerationJobID()
	require.NoError(t, err)
	require.Regexp(t, `^imgjob_[a-f0-9]{32}$`, first)
	require.NotEqual(t, first, second)
}
