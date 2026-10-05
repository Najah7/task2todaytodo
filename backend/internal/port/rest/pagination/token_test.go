package pagination

import (
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

type testAnchor struct {
	CreatedAt string `json:"created_at"`
	ID        int64  `json:"id"`
}

func testCodec(t *testing.T) *Codec {
	t.Helper()
	codec, err := NewCodec([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	codec.now = func() time.Time { return time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC) }
	return codec
}

func testScope() Scope {
	return Scope{UserID: "user-1", List: "projects", Parent: "", Order: "created_at_desc_id_desc", Fields: "items(id,title)"}
}

func TestTokenRoundTripAndPageSizeChange(t *testing.T) {
	codec := testCodec(t)
	anchor := testAnchor{CreatedAt: "2026-10-03T12:34:56.123456Z", ID: 912}
	token, err := Encode(codec, testScope(), anchor)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(token, "+") || strings.Contains(token, "/") || strings.Contains(token, "=") {
		t.Fatalf("token is not raw URL-safe base64: %q", token)
	}
	got, err := Decode[testAnchor](codec, token, testScope())
	if err != nil || got != anchor {
		t.Fatalf("Decode() = %#v, %v; want %#v, nil", got, err, anchor)
	}
	// Page size is intentionally absent from Scope and can change across pages.
	if _, err := ParsePageSize("75", true); err != nil {
		t.Fatal(err)
	}
}

func TestTokenUsesRandomNonce(t *testing.T) {
	codec := testCodec(t)
	anchor := testAnchor{CreatedAt: "2026-10-03T12:00:00Z", ID: 1}
	first, err := Encode(codec, testScope(), anchor)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Encode(codec, testScope(), anchor)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("identical payloads produced identical tokens")
	}
}

func TestDecodeRejectsTamperingAndScopeMismatch(t *testing.T) {
	codec := testCodec(t)
	token, err := Encode(codec, testScope(), testAnchor{ID: 1})
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		t.Fatal(err)
	}
	decoded[len(decoded)-1] ^= 0x80
	tampered := base64.RawURLEncoding.EncodeToString(decoded)
	if _, err := Decode[testAnchor](codec, tampered, testScope()); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("tampered token error = %v, want ErrInvalidToken", err)
	}

	for name, change := range map[string]func(*Scope){
		"user":   func(s *Scope) { s.UserID = "user-2" },
		"list":   func(s *Scope) { s.List = "tasks" },
		"parent": func(s *Scope) { s.Parent = "project-1" },
		"order":  func(s *Scope) { s.Order = "name_asc_id_asc" },
		"fields": func(s *Scope) { s.Fields = "items(id)" },
	} {
		t.Run(name, func(t *testing.T) {
			scope := testScope()
			change(&scope)
			if _, err := Decode[testAnchor](codec, token, scope); !errors.Is(err, ErrInvalidToken) {
				t.Fatalf("scope mismatch error = %v, want ErrInvalidToken", err)
			}
		})
	}
}

func TestDecodeRejectsExpiredTokenAt24HourBoundary(t *testing.T) {
	codec := testCodec(t)
	token, err := Encode(codec, testScope(), testAnchor{ID: 1})
	if err != nil {
		t.Fatal(err)
	}
	codec.now = func() time.Time { return time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC).Add(-time.Nanosecond) }
	if _, err := Decode[testAnchor](codec, token, testScope()); err != nil {
		t.Fatalf("token before expiry rejected: %v", err)
	}
	codec.now = func() time.Time { return time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC) }
	if _, err := Decode[testAnchor](codec, token, testScope()); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expired token error = %v, want ErrInvalidToken", err)
	}
}

func TestDecodeRejectsMalformedUnknownVersionAndExpiredClaims(t *testing.T) {
	codec := testCodec(t)
	for name, payload := range map[string]tokenPayload{
		"unknown version": {
			Version: 2, IssuedAt: codec.now().UnixNano(), ExpiresAt: codec.now().Add(TokenLifetime).UnixNano(), Scope: testScope(), Anchor: json.RawMessage(`{"id":1}`),
		},
		"expired claim": {
			Version: tokenVersion, IssuedAt: codec.now().Add(-TokenLifetime).UnixNano(), ExpiresAt: codec.now().UnixNano(), Scope: testScope(), Anchor: json.RawMessage(`{"id":1}`),
		},
	} {
		t.Run(name, func(t *testing.T) {
			token := sealPayload(t, codec.aead, payload)
			if _, err := Decode[testAnchor](codec, token, testScope()); !errors.Is(err, ErrInvalidToken) {
				t.Fatalf("Decode() error = %v, want ErrInvalidToken", err)
			}
		})
	}
	for _, token := range []string{"", "%%", strings.Repeat("a", maxTokenText+1)} {
		if _, err := Decode[testAnchor](codec, token, testScope()); !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("malformed token error = %v, want ErrInvalidToken", err)
		}
	}
}

func TestNewCodecRequiresAES256Key(t *testing.T) {
	for _, key := range [][]byte{nil, make([]byte, 16), make([]byte, 24), make([]byte, 33)} {
		if _, err := NewCodec(key); err == nil {
			t.Fatalf("NewCodec(%d-byte key) succeeded", len(key))
		}
	}
}

func sealPayload(t *testing.T, aead cipher.AEAD, payload tokenPayload) string {
	t.Helper()
	plain, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		t.Fatal(err)
	}
	sealed := aead.Seal(nonce, nonce, plain, nil)
	return base64.RawURLEncoding.EncodeToString(sealed)
}
