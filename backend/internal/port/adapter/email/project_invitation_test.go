package email

import (
	"strings"
	"testing"
	"time"
)

func TestProjectInvitationMessageHasBilingualFragmentLinksAndReadableExpiry(t *testing.T) {
	message := ProjectInvitationMessage("member@example.com", "https://app.example.test", "raw-token", "計画", "Administrator", 7*24*time.Hour)
	if message.Subject == "" || message.To != "member@example.com" {
		t.Fatalf("invalid invitation message: %+v", message)
	}
	for _, value := range []string{
		"管理者", "7日間", "https://app.example.test/invitations?lang=ja#token=raw-token",
		"Administrator", "7 days", "https://app.example.test/invitations?lang=en#token=raw-token",
	} {
		if !strings.Contains(message.TextBody, value) && !strings.Contains(message.HTMLBody, value) {
			t.Errorf("invitation message does not contain %q", value)
		}
	}
	if strings.Contains(message.TextBody, "168h0m0s") || strings.Contains(message.HTMLBody, "168h0m0s") {
		t.Fatal("expiry should use a reader-friendly duration")
	}
}
