package stripe

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
	"time"
)

func TestVerifyWebhookSignature(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	payload := []byte(`{"id":"evt_test","type":"customer.subscription.updated"}`)
	secret := "whsec_test"
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(fmt.Sprintf("%d.", now.Unix())))
	_, _ = mac.Write(payload)
	header := fmt.Sprintf("t=%d,v1=%s", now.Unix(), hex.EncodeToString(mac.Sum(nil)))
	if err := VerifyWebhookSignature(payload, header, secret, now); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}
	if err := VerifyWebhookSignature(payload, header, secret, now.Add(6*time.Minute)); err == nil {
		t.Fatal("stale signature accepted")
	}
	if err := VerifyWebhookSignature(append(payload, ' '), header, secret, now); err == nil {
		t.Fatal("signature accepted for modified raw payload")
	}
}
