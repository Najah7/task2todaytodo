package email

import (
	"context"
	"strings"
	"testing"
	"time"
)

type senderFunc func(context.Context, Message) error

func (f senderFunc) Send(ctx context.Context, message Message) error {
	return f(ctx, message)
}

func TestAuthMessageBuildsBilingualVerificationLinks(t *testing.T) {
	message := AuthMessage("user@example.test", "email_verification", "https://app.example.test/", "raw+token", time.Hour)

	if message.To != "user@example.test" {
		t.Fatalf("To = %q, want recipient", message.To)
	}
	if !strings.Contains(message.Subject, "メールアドレスを確認してください") || !strings.Contains(message.Subject, "Verify your email address") {
		t.Fatalf("Subject = %q, want Japanese and English", message.Subject)
	}
	for _, link := range []string{
		"https://app.example.test/verify-email?lang=ja#token=raw%2Btoken",
		"https://app.example.test/verify-email?lang=en#token=raw%2Btoken",
	} {
		if !strings.Contains(message.TextBody, link) || !strings.Contains(message.HTMLBody, link) {
			t.Errorf("message bodies do not contain %q", link)
		}
	}
	for _, content := range []string{"日本語で進む", "Continue in English", "1時間有効", "expires in 1 hour", "background:#17181c"} {
		if !strings.Contains(message.HTMLBody, content) {
			t.Errorf("HTML body missing %q", content)
		}
	}
	for _, content := range []string{"1時間有効", "expires in 1 hour"} {
		if !strings.Contains(message.TextBody, content) {
			t.Errorf("text body missing expiry %q", content)
		}
	}
}

func TestAuthMessageBuildsPasswordResetLinks(t *testing.T) {
	message := AuthMessage("user@example.test", "password_reset", "https://app.example.test", "token", 30*time.Minute)
	for _, link := range []string{
		"https://app.example.test/reset-password?lang=ja#token=token",
		"https://app.example.test/reset-password?lang=en#token=token",
	} {
		if !strings.Contains(message.TextBody, link) || !strings.Contains(message.HTMLBody, link) {
			t.Errorf("message bodies do not contain %q", link)
		}
	}
	for _, content := range []string{"日本語で進む", "Continue in English", "30分有効", "expires in 30 minutes", "background:#17181c"} {
		if !strings.Contains(message.HTMLBody, content) {
			t.Errorf("HTML body missing %q", content)
		}
	}
	for _, content := range []string{"30分有効", "expires in 30 minutes"} {
		if !strings.Contains(message.TextBody, content) {
			t.Errorf("text body missing expiry %q", content)
		}
	}
}

func TestAuthMessageRejectsUnknownPurpose(t *testing.T) {
	if got := AuthMessage("user@example.test", "unknown", "https://app.example.test", "token", time.Hour); got != (Message{}) {
		t.Fatalf("AuthMessage() = %+v, want empty message", got)
	}
}

func TestAuthLinkSenderRendersAndSendsMessage(t *testing.T) {
	var got Message
	sender := AuthLinkSender{Provider: senderFunc(func(_ context.Context, message Message) error {
		got = message
		return nil
	})}
	if err := sender.SendAuthLink(context.Background(), "user@example.test", "password_reset", "https://app.example.test", "secret-token", 30*time.Minute); err != nil {
		t.Fatalf("SendAuthLink() error = %v", err)
	}
	if got.To != "user@example.test" || !strings.Contains(got.TextBody, "secret-token") {
		t.Fatalf("sent message = %+v, want rendered recipient and link", got)
	}
}

func TestAuthLinkSenderRejectsUnknownPurpose(t *testing.T) {
	sender := AuthLinkSender{Provider: senderFunc(func(context.Context, Message) error {
		t.Fatal("Send() called for unsupported purpose")
		return nil
	})}
	if err := sender.SendAuthLink(context.Background(), "user@example.test", "unknown", "https://app.example.test", "token", time.Hour); err == nil {
		t.Fatal("SendAuthLink() error = nil, want unsupported purpose error")
	}
}
