//go:build unit

package admin

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateContactPageUpdate(t *testing.T) {
	t.Run("accepts supported contact settings", func(t *testing.T) {
		err := validateContactPageUpdate(UpdateSettingsRequest{
			ContactPageQQGroupNumber:   "1097280864",
			ContactPageQQQRCodeImage:   "data:image/png;base64,cXE=",
			ContactPageTelegramName:    "@ai_baipiao",
			ContactPageTelegramURL:     "https://t.me/ai_baipiao",
			ContactPageTelegramQRImage: "/contact/telegram-channel-qr.png",
		})
		require.NoError(t, err)
	})

	t.Run("rejects malformed QQ group numbers", func(t *testing.T) {
		err := validateContactPageUpdate(UpdateSettingsRequest{
			ContactPageQQGroupNumber: "group-123",
		})
		require.ErrorContains(t, err, "QQ group number")
	})

	t.Run("rejects non-Telegram outbound URLs", func(t *testing.T) {
		err := validateContactPageUpdate(UpdateSettingsRequest{
			ContactPageTelegramURL: "https://example.com/channel",
		})
		require.ErrorContains(t, err, "t.me")
	})

	t.Run("rejects non-image data URLs", func(t *testing.T) {
		err := validateContactPageUpdate(UpdateSettingsRequest{
			ContactPageQQQRCodeImage: "data:text/html;base64,PGgxPmJhZDwvaDE+",
		})
		require.ErrorContains(t, err, "QQ QR image")
	})
}
