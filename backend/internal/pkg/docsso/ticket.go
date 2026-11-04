// Package docsso issues short-lived credentials for the independent docs admin.
package docsso

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"
)

const Audience = "aibaipiao-docs-admin/v1"
const Lifetime = time.Minute

// Key is a domain-separated key; it cannot be used to sign Sub2API JWTs.
func Key(jwtSecret string) []byte {
	mac := hmac.New(sha256.New, []byte(jwtSecret))
	mac.Write([]byte(Audience))
	return mac.Sum(nil)
}

func Issue(jwtSecret string, userID int64, now time.Time) (string, error) {
	if len(jwtSecret) < 32 || userID <= 0 {
		return "", errors.New("invalid docs SSO signing configuration or subject")
	}
	payload, err := json.Marshal(struct {
		Audience string `json:"aud"`
		Subject  int64  `json:"sub"`
		IssuedAt int64  `json:"iat"`
		Expires  int64  `json:"exp"`
	}{Audience, userID, now.Unix(), now.Add(Lifetime).Unix()})
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, Key(jwtSecret))
	mac.Write([]byte(encoded))
	return encoded + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}
