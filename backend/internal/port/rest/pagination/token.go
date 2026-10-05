package pagination

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"time"
)

const (
	TokenLifetime = 24 * time.Hour
	tokenVersion  = 1
	maxTokenBytes = 32 * 1024
	maxTokenText  = 48 * 1024
)

var ErrInvalidToken = errors.New("invalid page token")

// Scope binds a page token to one caller, list, parent, sort order, and
// canonical field mask. Page size intentionally does not belong here.
type Scope struct {
	UserID string `json:"user_id"`
	List   string `json:"list"`
	Parent string `json:"parent"`
	Order  string `json:"order"`
	Fields string `json:"fields"`
}

// Codec encrypts and authenticates opaque page tokens using AES-GCM.
type Codec struct {
	aead cipher.AEAD
	now  func() time.Time
}

type tokenPayload struct {
	Version   int             `json:"v"`
	IssuedAt  int64           `json:"iat"`
	ExpiresAt int64           `json:"exp"`
	Scope     Scope           `json:"scope"`
	Anchor    json.RawMessage `json:"anchor"`
}

// NewCodec creates a token codec from a 32-byte AES-256 key.
func NewCodec(key []byte) (*Codec, error) {
	if len(key) != 32 {
		return nil, errors.New("page token key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Codec{aead: aead, now: time.Now}, nil
}

// Encode encrypts a typed resource-specific cursor anchor into a URL-safe
// opaque token. The token expires 24 hours after issuance.
func Encode[T any](codec *Codec, scope Scope, anchor T) (string, error) {
	if codec == nil || codec.aead == nil {
		return "", ErrInvalidToken
	}
	anchorJSON, err := json.Marshal(anchor)
	if err != nil {
		return "", err
	}
	now := codec.now().UTC()
	payload, err := json.Marshal(tokenPayload{
		Version:   tokenVersion,
		IssuedAt:  now.UnixNano(),
		ExpiresAt: now.Add(TokenLifetime).UnixNano(),
		Scope:     scope,
		Anchor:    anchorJSON,
	})
	if err != nil {
		return "", err
	}
	if len(payload) > maxTokenBytes {
		return "", ErrInvalidToken
	}
	nonce := make([]byte, codec.aead.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := codec.aead.Seal(nonce, nonce, payload, nil)
	token := base64.RawURLEncoding.EncodeToString(sealed)
	if len(token) > maxTokenText {
		return "", ErrInvalidToken
	}
	return token, nil
}

// Decode decrypts token and restores its typed anchor only when token is
// valid, unexpired, and bound to expectedScope.
func Decode[T any](codec *Codec, token string, expectedScope Scope) (T, error) {
	var zero T
	if codec == nil || codec.aead == nil || token == "" || len(token) > maxTokenText {
		return zero, ErrInvalidToken
	}
	sealed, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(sealed) <= codec.aead.NonceSize() || len(sealed) > maxTokenBytes+codec.aead.NonceSize()+codec.aead.Overhead() {
		return zero, ErrInvalidToken
	}
	nonceSize := codec.aead.NonceSize()
	plain, err := codec.aead.Open(nil, sealed[:nonceSize], sealed[nonceSize:], nil)
	if err != nil || len(plain) > maxTokenBytes {
		return zero, ErrInvalidToken
	}
	var payload tokenPayload
	if err := json.Unmarshal(plain, &payload); err != nil {
		return zero, ErrInvalidToken
	}
	now := codec.now().UTC().UnixNano()
	if payload.Version != tokenVersion || payload.IssuedAt <= 0 || payload.ExpiresAt != payload.IssuedAt+int64(TokenLifetime) ||
		payload.IssuedAt > now || now >= payload.ExpiresAt || payload.Scope != expectedScope || len(payload.Anchor) == 0 {
		return zero, ErrInvalidToken
	}
	var anchor T
	if err := json.Unmarshal(payload.Anchor, &anchor); err != nil {
		return zero, ErrInvalidToken
	}
	return anchor, nil
}
