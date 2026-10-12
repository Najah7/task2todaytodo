package email

import (
	"context"
	"fmt"
	"html"
	"net/url"
	"strings"
	"time"
)

const authButtonStyle = "display:inline-block;padding:12px 20px;background:#17181c;color:#fff;text-decoration:none;border-radius:8px"

// Message contains the recipient, subject, and plain-text and HTML alternatives.
type Message struct {
	To       string
	Subject  string
	TextBody string
	HTMLBody string
}

// Sender sends one email message.
type Sender interface {
	Send(ctx context.Context, message Message) error
}

// AuthLinkSender renders and sends verification or password-reset links.
type AuthLinkSender struct {
	Provider Sender
}

// SendAuthLink sends one of the supported authentication links.
func (s AuthLinkSender) SendAuthLink(ctx context.Context, to, purpose, frontendBaseURL, rawToken string, lifetime time.Duration) error {
	if s.Provider == nil {
		return fmt.Errorf("email sender is required")
	}
	if lifetime <= 0 {
		return fmt.Errorf("authentication email lifetime must be positive")
	}
	message := AuthMessage(to, purpose, frontendBaseURL, rawToken, lifetime)
	if message.Subject == "" {
		return fmt.Errorf("unsupported authentication email purpose")
	}
	return s.Provider.Send(ctx, message)
}

// AuthMessage builds the bilingual email used for verification and password reset.
func AuthMessage(to, purpose, frontendBaseURL, rawToken string, lifetime time.Duration) Message {
	if lifetime <= 0 {
		return Message{}
	}
	baseURL := strings.TrimRight(frontendBaseURL, "/")
	token := url.QueryEscape(rawToken)

	var path, jaSubject, enSubject, jaIntro, enIntro, jaAction, enAction, jaExpiry, enExpiry string
	switch purpose {
	case "email_verification":
		path = "/verify-email"
		jaSubject = "メールアドレスを確認してください"
		enSubject = "Verify your email address"
		jaIntro = "Task2TodayToDo への登録ありがとうございます。以下のボタンからメールアドレスを確認してください。"
		enIntro = "Thank you for signing up for Task2TodayToDo. Verify your email address using one of the buttons below."
		jaAction = "メールアドレスを確認"
		enAction = "Verify email"
	case "password_reset":
		path = "/reset-password"
		jaSubject = "パスワードを再設定してください"
		enSubject = "Reset your password"
		jaIntro = "パスワード再設定のリクエストを受け付けました。以下のボタンから新しいパスワードを設定してください。"
		enIntro = "We received a request to reset your password. Use one of the buttons below to choose a new password."
		jaAction = "パスワードを再設定"
		enAction = "Reset password"
	default:
		return Message{}
	}
	jaLifetime, enLifetime := formatLifetime(lifetime)
	jaExpiry = fmt.Sprintf("このリンクは%s有効で、一度だけ使用できます。心当たりがない場合、このメールを無視してください。", jaLifetime)
	enExpiry = fmt.Sprintf("This link expires in %s and can be used once. If you did not request this, you can ignore this email.", enLifetime)

	jaURL := baseURL + path + "?lang=ja#token=" + token
	enURL := baseURL + path + "?lang=en#token=" + token
	message := Message{
		To:      to,
		Subject: jaSubject + " / " + enSubject,
		TextBody: fmt.Sprintf(
			"日本語\n\n%s\n%s\n%s\n\nEnglish\n\n%s\n%s\n%s\n",
			jaIntro, jaAction+": "+jaURL, jaExpiry, enIntro, enAction+": "+enURL, enExpiry,
		),
	}
	message.HTMLBody = fmt.Sprintf(`<!doctype html>
<html lang="ja"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"></head>
<body style="margin:0;background:#f5f5f5;color:#202124;font-family:Arial,sans-serif;line-height:1.6">
<main style="max-width:560px;margin:24px auto;padding:28px;background:#fff;border-radius:12px">
<section lang="ja">
<h1 style="font-size:22px">%s</h1>
<p>%s</p>
<p><a href="%s" style="%s">日本語で進む</a></p>
<p style="font-size:14px">%s</p>
</section>
<hr style="margin:28px 0;border:0;border-top:1px solid #e5e7eb">
<section lang="en">
<h1 style="font-size:22px">%s</h1>
<p>%s</p>
<p><a href="%s" style="%s">Continue in English</a></p>
<p style="font-size:14px">%s</p>
</section>
</main></body></html>`,
		html.EscapeString(jaSubject), html.EscapeString(jaIntro), html.EscapeString(jaURL),
		authButtonStyle, html.EscapeString(jaExpiry),
		html.EscapeString(enSubject), html.EscapeString(enIntro), html.EscapeString(enURL),
		authButtonStyle, html.EscapeString(enExpiry),
	)
	return message
}

func formatLifetime(lifetime time.Duration) (japanese, english string) {
	switch {
	case lifetime%time.Hour == 0:
		value := int64(lifetime / time.Hour)
		unit := "hour"
		if value != 1 {
			unit += "s"
		}
		return fmt.Sprintf("%d時間", value), fmt.Sprintf("%d %s", value, unit)
	case lifetime%time.Minute == 0:
		value := int64(lifetime / time.Minute)
		unit := "minute"
		if value != 1 {
			unit += "s"
		}
		return fmt.Sprintf("%d分", value), fmt.Sprintf("%d %s", value, unit)
	case lifetime%time.Second == 0:
		value := int64(lifetime / time.Second)
		unit := "second"
		if value != 1 {
			unit += "s"
		}
		return fmt.Sprintf("%d秒", value), fmt.Sprintf("%d %s", value, unit)
	default:
		return lifetime.String(), lifetime.String()
	}
}
