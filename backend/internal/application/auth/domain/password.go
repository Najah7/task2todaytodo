package domain

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

const PasswordMinLength = 8

const (
	argonVersion       = 19
	argonMemoryKiB     = 19 * 1024
	argonIterations    = 2
	argonParallelism   = 1
	argonSaltLength    = 16
	argonKeyLength     = 32
	maxArgonMemoryKiB  = 128 * 1024
	maxArgonIterations = 10
	maxArgonThreads    = 8
)

var (
	ErrPasswordEmpty            = errors.New("password cannot be empty")
	ErrPasswordTooShort         = errors.New("password must be at least 8 characters long")
	ErrPasswordMissingLowercase = errors.New("password must contain at least one lowercase letter")
	ErrPasswordMissingUppercase = errors.New("password must contain at least one uppercase letter")
	ErrPasswordMissingDigit     = errors.New("password must contain at least one digit")
	ErrPasswordMissingSpecial   = errors.New("password must contain at least one special character")
	ErrPasswordMustBeHashed     = errors.New("password hash is invalid")
	ErrPasswordMustNotBeHashed  = errors.New("password input must not be a password hash")
)

// Password contains only an encoded password hash. Plaintext exists only as a
// constructor argument and is never retained by a domain model.
type Password struct{ encoded string }

var (
	lowercaseRE = regexp.MustCompile(`[a-z]`)
	uppercaseRE = regexp.MustCompile(`[A-Z]`)
	digitRE     = regexp.MustCompile(`[0-9]`)
	specialRE   = regexp.MustCompile(`[!@#\$%\^&\*\(\)_\+\-=\[\]\{\};':"\\|,.<>/?]`)
)

// NewPassword validates user input and hashes it using the current Argon2id
// policy. The returned value never contains the plaintext password.
func NewPassword(value string) (Password, error) {
	if value == "" {
		return Password{}, ErrPasswordEmpty
	}
	if err := validatePassword(value); err != nil {
		return Password{}, err
	}
	return hashPassword(value)
}

// NewHashedPassword validates and restores a persisted Argon2id encoding.
func NewHashedPassword(value string) (Password, error) {
	if value == "" {
		return Password{}, ErrPasswordEmpty
	}
	if _, _, _, _, _, err := parsePasswordHash(value); err != nil {
		return Password{}, ErrPasswordMustBeHashed
	}
	return Password{encoded: value}, nil
}

func validatePassword(value string) error {
	if len(value) < PasswordMinLength {
		return ErrPasswordTooShort
	}
	if !lowercaseRE.MatchString(value) {
		return ErrPasswordMissingLowercase
	}
	if !uppercaseRE.MatchString(value) {
		return ErrPasswordMissingUppercase
	}
	if !digitRE.MatchString(value) {
		return ErrPasswordMissingDigit
	}
	if !specialRE.MatchString(value) {
		return ErrPasswordMissingSpecial
	}
	return nil
}

func hashPassword(value string) (Password, error) {
	salt := make([]byte, argonSaltLength)
	if _, err := rand.Read(salt); err != nil {
		return Password{}, fmt.Errorf("generate password salt: %w", err)
	}
	derived := argon2.IDKey([]byte(value), salt, argonIterations, argonMemoryKiB, argonParallelism, argonKeyLength)
	encoded := fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argonVersion, argonMemoryKiB, argonIterations, argonParallelism,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(derived))
	return Password{encoded: encoded}, nil
}

func (p Password) Verify(input string) bool {
	memory, iterations, parallelism, salt, expected, err := parsePasswordHash(p.encoded)
	if err != nil {
		return false
	}
	actual := argon2.IDKey([]byte(input), salt, iterations, memory, parallelism, uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

func parsePasswordHash(value string) (uint32, uint32, uint8, []byte, []byte, error) {
	parts := strings.Split(value, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" || parts[2] != "v=19" {
		return 0, 0, 0, nil, nil, ErrPasswordMustBeHashed
	}
	params := strings.Split(parts[3], ",")
	if len(params) != 3 {
		return 0, 0, 0, nil, nil, ErrPasswordMustBeHashed
	}
	param := make(map[string]uint64, 3)
	for _, item := range params {
		key, raw, found := strings.Cut(item, "=")
		if !found || (key != "m" && key != "t" && key != "p") {
			return 0, 0, 0, nil, nil, ErrPasswordMustBeHashed
		}
		n, err := strconv.ParseUint(raw, 10, 32)
		if err != nil {
			return 0, 0, 0, nil, nil, ErrPasswordMustBeHashed
		}
		if _, duplicate := param[key]; duplicate {
			return 0, 0, 0, nil, nil, ErrPasswordMustBeHashed
		}
		param[key] = n
	}
	memory, iterations, parallelism := param["m"], param["t"], param["p"]
	if memory < argonMemoryKiB || memory > maxArgonMemoryKiB || iterations < argonIterations || iterations > maxArgonIterations || parallelism < 1 || parallelism > maxArgonThreads {
		return 0, 0, 0, nil, nil, ErrPasswordMustBeHashed
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) < argonSaltLength || len(salt) > 64 {
		return 0, 0, 0, nil, nil, ErrPasswordMustBeHashed
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(key) != argonKeyLength {
		return 0, 0, 0, nil, nil, ErrPasswordMustBeHashed
	}
	return uint32(memory), uint32(iterations), uint8(parallelism), salt, key, nil
}

func (p Password) String() string { return p.encoded }
