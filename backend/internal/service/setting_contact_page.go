package service

import (
	"context"
	"fmt"
)

const (
	DefaultContactPageQQGroupNumber   = "1097280864"
	DefaultContactPageQQQRCodeImage   = "/contact/qq-group-qr.jpg"
	DefaultContactPageTelegramName    = "@ai_baipiao"
	DefaultContactPageTelegramURL     = "https://t.me/ai_baipiao"
	DefaultContactPageTelegramQRImage = "/contact/telegram-channel-qr.png"
)

type ContactPageSettings struct {
	QQGroupNumber   string
	QQQRCodeImage   string
	TelegramName    string
	TelegramURL     string
	TelegramQRImage string
}

func contactPageSettingValue(values map[string]string, key, defaultValue string) string {
	if value, ok := values[key]; ok {
		return value
	}
	return defaultValue
}

func (s *SettingService) GetContactPageSettings(ctx context.Context) (*ContactPageSettings, error) {
	values, err := s.settingRepo.GetMultiple(ctx, []string{
		SettingKeyContactPageQQGroupNumber,
		SettingKeyContactPageQQQRCodeImage,
		SettingKeyContactPageTelegramName,
		SettingKeyContactPageTelegramURL,
		SettingKeyContactPageTelegramQRImage,
	})
	if err != nil {
		return nil, fmt.Errorf("get contact page settings: %w", err)
	}

	return &ContactPageSettings{
		QQGroupNumber:   contactPageSettingValue(values, SettingKeyContactPageQQGroupNumber, DefaultContactPageQQGroupNumber),
		QQQRCodeImage:   contactPageSettingValue(values, SettingKeyContactPageQQQRCodeImage, DefaultContactPageQQQRCodeImage),
		TelegramName:    contactPageSettingValue(values, SettingKeyContactPageTelegramName, DefaultContactPageTelegramName),
		TelegramURL:     contactPageSettingValue(values, SettingKeyContactPageTelegramURL, DefaultContactPageTelegramURL),
		TelegramQRImage: contactPageSettingValue(values, SettingKeyContactPageTelegramQRImage, DefaultContactPageTelegramQRImage),
	}, nil
}
