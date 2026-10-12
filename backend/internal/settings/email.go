package settings

import (
	"fmt"
	"net"
	"net/mail"
	"net/url"
	"strconv"
	"strings"
)

type EmailSettings struct {
	Provider        string
	SMTPHost        string
	SMTPPort        int
	FromAddress     string
	FromName        string
	Region          string
	FrontendBaseURL string
}

func loadEmailSettings(value func(string, string) string, environment string) (EmailSettings, error) {
	settings := EmailSettings{
		Provider:        strings.ToLower(value("EMAIL_PROVIDER", "mailpit")),
		SMTPHost:        value("SMTP_HOST", "localhost"),
		FromAddress:     value("EMAIL_FROM_ADDRESS", "no-reply@task2todaytodo.local"),
		FromName:        value("EMAIL_FROM_NAME", "Task2TodayToDo"),
		Region:          value("EMAIL_REGION", "ap-northeast-1"),
		FrontendBaseURL: value("FRONTEND_BASE_URL", "http://localhost:5173"),
	}
	port, err := strconv.Atoi(value("SMTP_PORT", "1025"))
	if err != nil || port < 1 || port > 65535 {
		return EmailSettings{}, fmt.Errorf("SMTP_PORT must be an integer from 1 through 65535")
	}
	settings.SMTPPort = port
	if settings.Provider != "mailpit" && settings.Provider != "ses" {
		return EmailSettings{}, fmt.Errorf("EMAIL_PROVIDER must be mailpit or ses")
	}
	if environment == "production" && settings.Provider != "ses" {
		return EmailSettings{}, fmt.Errorf("EMAIL_PROVIDER must be ses in production")
	}
	if settings.SMTPHost == "" || settings.FromName == "" || settings.Region == "" {
		return EmailSettings{}, fmt.Errorf("email provider host, from name, and region are required")
	}
	from, err := mail.ParseAddress(settings.FromAddress)
	if err != nil || from.Address != settings.FromAddress {
		return EmailSettings{}, fmt.Errorf("EMAIL_FROM_ADDRESS must be a valid email address")
	}
	frontend, err := url.Parse(settings.FrontendBaseURL)
	if err != nil || frontend.Hostname() == "" || frontend.User != nil || frontend.RawQuery != "" || frontend.Fragment != "" || (frontend.Path != "" && frontend.Path != "/") {
		return EmailSettings{}, fmt.Errorf("FRONTEND_BASE_URL must be an origin URL without path, query, or fragment")
	}
	local := isLocalHost(frontend.Hostname())
	if environment == "production" && frontend.Scheme != "https" {
		return EmailSettings{}, fmt.Errorf("FRONTEND_BASE_URL must use HTTPS in production")
	}
	if frontend.Scheme != "https" && !(frontend.Scheme == "http" && local) {
		return EmailSettings{}, fmt.Errorf("FRONTEND_BASE_URL must use HTTPS unless it points to localhost")
	}
	return settings, nil
}

func isLocalHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	parsed := net.ParseIP(host)
	return parsed != nil && parsed.IsLoopback()
}
