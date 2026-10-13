package stripe

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"
)

var ErrInvalidWebhookSignature = errors.New("invalid Stripe webhook signature")

func VerifyWebhookSignature(payload []byte, header, secret string, now time.Time) error {
	var timestamp string
	var signatures []string
	for _, item := range strings.Split(header, ",") {
		key, value, found := strings.Cut(strings.TrimSpace(item), "=")
		if !found {
			continue
		}
		switch key {
		case "t":
			timestamp = value
		case "v1":
			signatures = append(signatures, value)
		}
	}
	seconds, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || len(signatures) == 0 {
		return ErrInvalidWebhookSignature
	}
	signedAt := time.Unix(seconds, 0)
	if now.Sub(signedAt) > 5*time.Minute || signedAt.Sub(now) > 5*time.Minute {
		return ErrInvalidWebhookSignature
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(timestamp + "."))
	_, _ = mac.Write(payload)
	expected := mac.Sum(nil)
	for _, signature := range signatures {
		provided, decodeErr := hex.DecodeString(signature)
		if decodeErr == nil && hmac.Equal(provided, expected) {
			return nil
		}
	}
	return ErrInvalidWebhookSignature
}
