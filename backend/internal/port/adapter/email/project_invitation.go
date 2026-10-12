package email

import (
	"context"
	"fmt"
	"html"
	"net/url"
	"strings"
	"time"
)

type ProjectInvitationSender struct{ Provider Sender }

func (s ProjectInvitationSender) SendProjectInvitation(ctx context.Context, to, frontendBaseURL, rawToken, projectTitle, roleName string, lifetime time.Duration) error {
	if s.Provider == nil {
		return fmt.Errorf("email sender is required")
	}
	message := ProjectInvitationMessage(to, frontendBaseURL, rawToken, projectTitle, roleName, lifetime)
	if message.Subject == "" {
		return fmt.Errorf("project invitation fields are required")
	}
	return s.Provider.Send(ctx, message)
}

func ProjectInvitationMessage(to, frontendBaseURL, rawToken, projectTitle, roleName string, lifetime time.Duration) Message {
	if strings.TrimSpace(to) == "" || strings.TrimSpace(rawToken) == "" || strings.TrimSpace(projectTitle) == "" || strings.TrimSpace(roleName) == "" || lifetime <= 0 {
		return Message{}
	}
	baseURL := strings.TrimRight(frontendBaseURL, "/") + "/invitations"
	token := url.QueryEscape(rawToken)
	jaURL, enURL := baseURL+"?lang=ja#token="+token, baseURL+"?lang=en#token="+token
	jaTitle, enTitle := projectTitle, projectTitle
	jaRole, enRole := invitationRoleJapanese(roleName), roleName
	jaIntro := fmt.Sprintf("「%s」プロジェクトに招待されました。招待された役割は「%s」です。", jaTitle, jaRole)
	enIntro := fmt.Sprintf("You were invited to the project %q with the %s role.", enTitle, enRole)
	jaExpiry := fmt.Sprintf("このリンクは%s有効で、一度だけ使用できます。", invitationLifetimeJapanese(lifetime))
	enExpiry := fmt.Sprintf("This link expires in %s and can be used once.", invitationLifetimeEnglish(lifetime))
	message := Message{
		To:      to,
		Subject: "プロジェクトへの招待 / Project invitation",
		TextBody: fmt.Sprintf("日本語\n\n%s\n招待を確認: %s\n%s\n\nEnglish\n\n%s\nReview invitation: %s\n%s\n",
			jaIntro, jaURL, jaExpiry, enIntro, enURL, enExpiry),
	}
	message.HTMLBody = fmt.Sprintf(`<!doctype html>
<html lang="ja"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"></head>
<body style="margin:0;background:#f5f5f5;color:#202124;font-family:Arial,sans-serif;line-height:1.6"><main style="max-width:560px;margin:24px auto;padding:28px;background:#fff;border-radius:12px">
<section lang="ja"><h1>プロジェクトへの招待</h1><p>%s</p><p><a href="%s" style="%s">日本語で招待を確認</a></p><p>%s</p></section>
<hr style="margin:28px 0;border:0;border-top:1px solid #e5e7eb">
<section lang="en"><h1>Project invitation</h1><p>%s</p><p><a href="%s" style="%s">Review invitation in English</a></p><p>%s</p></section>
</main></body></html>`, html.EscapeString(jaIntro), html.EscapeString(jaURL), authButtonStyle, html.EscapeString(jaExpiry), html.EscapeString(enIntro), html.EscapeString(enURL), authButtonStyle, html.EscapeString(enExpiry))
	return message
}

func invitationRoleJapanese(role string) string {
	switch role {
	case "Administrator":
		return "管理者"
	case "Editor":
		return "編集者"
	case "Viewer":
		return "閲覧者"
	default:
		return role
	}
}

func invitationLifetimeJapanese(lifetime time.Duration) string {
	if lifetime%(24*time.Hour) == 0 {
		return fmt.Sprintf("%d日間", int(lifetime/(24*time.Hour)))
	}
	if lifetime%time.Hour == 0 {
		return fmt.Sprintf("%d時間", int(lifetime/time.Hour))
	}
	if lifetime%time.Minute == 0 {
		return fmt.Sprintf("%d分", int(lifetime/time.Minute))
	}
	return lifetime.Round(time.Second).String()
}

func invitationLifetimeEnglish(lifetime time.Duration) string {
	if lifetime%(24*time.Hour) == 0 {
		days := int(lifetime / (24 * time.Hour))
		unit := "days"
		if days == 1 {
			unit = "day"
		}
		return fmt.Sprintf("%d %s", days, unit)
	}
	if lifetime%time.Hour == 0 {
		hours := int(lifetime / time.Hour)
		unit := "hours"
		if hours == 1 {
			unit = "hour"
		}
		return fmt.Sprintf("%d %s", hours, unit)
	}
	return lifetime.Round(time.Second).String()
}
