package docsso

import (
	"encoding/hex"
	"testing"
	"time"
)

func TestDocsSSOProtocolFixture(t *testing.T) {
	const secret = "docs-sso-fixture-secret-at-least-32-characters"
	if hex.EncodeToString(Key(secret)) != "b371460e792275e11388e7f7d9ae068b36fb4c0b1c0910ed26e742ab14f31b3d" {
		t.Fatal("docs key derivation changed")
	}
	ticket, err := Issue(secret, 42, time.Unix(1700000000, 0))
	const want = "eyJhdWQiOiJhaWJhaXBpYW8tZG9jcy1hZG1pbi92MSIsInN1YiI6NDIsImlhdCI6MTcwMDAwMDAwMCwiZXhwIjoxNzAwMDAwMDYwfQ.MAQzcRT7VJ5tLKINpEIvOQI-XEabIJnGOCdDD5tEZvI"
	if err != nil || ticket != want {
		t.Fatal("ticket protocol differs from the independent docs verifier")
	}
	for _, input := range []struct {
		secret string
		userID int64
	}{{"short", 42}, {secret, 0}, {secret, -1}} {
		if _, err := Issue(input.secret, input.userID, time.Now()); err == nil {
			t.Fatal("accepted invalid signing input")
		}
	}
}
