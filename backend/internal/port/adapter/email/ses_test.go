package email

import (
	"context"
	"net/mail"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sesv2"
)

type sesClientFunc func(context.Context, *sesv2.SendEmailInput, ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error)

func (f sesClientFunc) SendEmail(ctx context.Context, input *sesv2.SendEmailInput, options ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error) {
	return f(ctx, input, options...)
}

func TestSESSendUsesBilingualAlternatives(t *testing.T) {
	var got *sesv2.SendEmailInput
	var hasDeadline bool
	sender := sesSender{
		client: sesClientFunc(func(ctx context.Context, input *sesv2.SendEmailInput, _ ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error) {
			got = input
			_, hasDeadline = ctx.Deadline()
			return &sesv2.SendEmailOutput{}, nil
		}),
		fromAddress: &mail.Address{Address: "noreply@example.test"},
		fromName:    "Task2TodayToDo",
	}
	message := AuthMessage("user@example.test", "email_verification", "https://app.example.test", "token", time.Hour)
	if err := sender.Send(context.Background(), message); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if !hasDeadline {
		t.Fatal("SendEmail context has no timeout deadline")
	}
	if got == nil || got.Content == nil || got.Content.Simple == nil {
		t.Fatal("SendEmail() did not receive simple message content")
	}
	if len(got.Destination.ToAddresses) != 1 || got.Destination.ToAddresses[0] != "user@example.test" {
		t.Fatalf("ToAddresses = %v, want recipient", got.Destination.ToAddresses)
	}
	if got.Content.Simple.Body.Text == nil || got.Content.Simple.Body.Html == nil {
		t.Fatal("SES message missing text or HTML alternative")
	}
	for _, want := range []string{"?lang=ja#token=token", "?lang=en#token=token"} {
		if !strings.Contains(*got.Content.Simple.Body.Text.Data, want) || !strings.Contains(*got.Content.Simple.Body.Html.Data, want) {
			t.Errorf("SES alternatives do not contain %q", want)
		}
	}
}
