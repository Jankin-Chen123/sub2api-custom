package service

import "testing"

func TestAccountTLSFingerprintRequiresExplicitOptIn(t *testing.T) {
	tests := []struct {
		name    string
		account *Account
		want    bool
	}{
		{name: "nil account", account: nil, want: false},
		{name: "default OpenAI API key", account: &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey}, want: false},
		{name: "enabled OpenAI API key", account: &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Extra: map[string]any{"enable_tls_fingerprint": true}}, want: true},
		{name: "enabled Anthropic OAuth", account: &Account{Platform: PlatformAnthropic, Type: AccountTypeOAuth, Extra: map[string]any{"enable_tls_fingerprint": true}}, want: true},
		{name: "wrong value", account: &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Extra: map[string]any{"enable_tls_fingerprint": "true"}}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.account.IsTLSFingerprintEnabled(); got != tt.want {
				t.Fatalf("IsTLSFingerprintEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}
