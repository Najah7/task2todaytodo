package usecase

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/Najah7/task2todaytodo/internal/application/project/dao"
)

var (
	ErrProjectInvitationInvalidInput = errors.New("project invitation input is invalid")
	ErrProjectInvitationInvalidEmail = errors.New("project invitation email is invalid")
	ErrProjectInvitationNotFound     = errors.New("project invitation not found")
	ErrProjectInvitationRejected     = errors.New("project invitation was rejected")
	ErrProjectInvitationConflict     = errors.New("project invitation already exists or recipient is a member")
	ErrProjectInvitationRateLimited  = errors.New("project invitation email rate limit exceeded")
	ErrProjectInvitationTokenInvalid = errors.New("project invitation token is invalid or expired")
)

const (
	projectInvitationSendCooldown = time.Minute
	projectInvitationSendWindow   = time.Hour
	projectInvitationSendLimit    = 5
)

func normalizeInvitationEmail(value string) (string, error) {
	parsed, err := mail.ParseAddress(strings.TrimSpace(value))
	if err != nil || parsed.Address == "" || strings.Contains(parsed.Address, " ") {
		return "", ErrProjectInvitationInvalidEmail
	}
	if parsed.Name != "" || !strings.EqualFold(parsed.Address, strings.TrimSpace(value)) {
		return "", ErrProjectInvitationInvalidEmail
	}
	return strings.ToLower(parsed.Address), nil
}

func newProjectInvitationToken() (raw string, hash []byte, err error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", nil, err
	}
	digest := sha256.Sum256(bytes)
	return base64.RawURLEncoding.EncodeToString(bytes), digest[:], nil
}

func parseProjectInvitationToken(raw string) ([]byte, error) {
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(raw)
	if err != nil || len(decoded) != 32 {
		return nil, ErrProjectInvitationTokenInvalid
	}
	digest := sha256.Sum256(decoded)
	return digest[:], nil
}

func invitationEmailHash(email string) []byte {
	digest := sha256.Sum256([]byte(strings.ToLower(email)))
	return digest[:]
}

func invitationRoleName(role string) string {
	switch role {
	case "admin":
		return "Administrator"
	case "editor":
		return "Editor"
	case "viewer":
		return "Viewer"
	default:
		return role
	}
}

func invitationStatus(invitation dao.ProjectInvitation, now time.Time) string {
	switch {
	case !invitation.UsedAt.IsZero():
		return "accepted"
	case !invitation.RevokedAt.IsZero():
		return "revoked"
	case !invitation.ProjectDeletedAt.IsZero():
		return "project_deleted"
	case !invitation.ExpiresAt.After(now):
		return "expired"
	default:
		return "pending"
	}
}
