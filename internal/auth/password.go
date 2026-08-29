// Package auth implements single-tenant password authentication for the web UI:
// argon2id password hashing, server-side sessions (cookie-backed), and login
// rate limiting. It owns the auth_credentials / auth_sessions schema.
//
// Design: docs/design/04-auth-security.md.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2id parameters follow OWASP's recommended configuration for
// interactive login (64 MiB memory, 3 iterations, 2 lanes).
const (
	argonTime    = 3
	argonMemory  = 64 * 1024 // KiB
	argonThreads = 2
	argonKeyLen  = 32
	saltLen      = 16
)

// MinPasswordLen / MaxPasswordLen bound acceptable passwords. MinPasswordLen
// follows docs/design/06-security-hardening.md §2.2 (10 → 12 for public-exposed
// deployments); the policy additionally requires an ASCII letter and digit.
const (
	MinPasswordLen = 12
	MaxPasswordLen = 256
)

// ErrMalformedPHC marks a stored hash that is not valid argon2id PHC.
var ErrMalformedPHC = errors.New("malformed argon2id PHC string")

// HashPassword returns a PHC-string encoded argon2id hash of password.
func HashPassword(password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// VerifyPassword reports whether password matches the PHC-encoded argon2id hash.
// A malformed stored hash returns an error; a wrong password returns (false, nil).
func VerifyPassword(password, phc string) (bool, error) {
	salt, key, timeFactor, memoryKiB, threads, err := decodePHC(phc)
	if err != nil {
		return false, err
	}
	candidate := argon2.IDKey([]byte(password), salt, timeFactor, memoryKiB, threads, uint32(len(key)))
	if subtle.ConstantTimeCompare(key, candidate) != 1 {
		return false, nil
	}
	return true, nil
}

// decodePHC parses "$argon2id$v=19$m=65536,t=3,p=2$<salt>$<key>".
func decodePHC(phc string) (salt, key []byte, timeFactor, memoryKiB uint32, threads uint8, err error) {
	parts := strings.Split(phc, "$")
	// ["", "argon2id", "v=19", "m=...,t=...,p=...", salt, key]
	if len(parts) != 6 || parts[1] != "argon2id" {
		return nil, nil, 0, 0, 0, ErrMalformedPHC
	}
	var version int
	if _, err = fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return nil, nil, 0, 0, 0, ErrMalformedPHC
	}
	var m, t, p uint64
	params := strings.Split(parts[3], ",")
	if len(params) != 3 {
		return nil, nil, 0, 0, 0, ErrMalformedPHC
	}
	for _, kv := range params {
		kvParts := strings.SplitN(kv, "=", 2)
		if len(kvParts) != 2 {
			return nil, nil, 0, 0, 0, ErrMalformedPHC
		}
		value, convErr := strconv.ParseUint(kvParts[1], 10, 32)
		if convErr != nil {
			return nil, nil, 0, 0, 0, ErrMalformedPHC
		}
		switch kvParts[0] {
		case "m":
			m = value
		case "t":
			t = value
		case "p":
			p = value
		default:
			return nil, nil, 0, 0, 0, ErrMalformedPHC
		}
	}
	if m == 0 || t == 0 || p == 0 || p > 255 {
		return nil, nil, 0, 0, 0, ErrMalformedPHC
	}
	salt, err = base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) == 0 {
		return nil, nil, 0, 0, 0, ErrMalformedPHC
	}
	key, err = base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(key) == 0 {
		return nil, nil, 0, 0, 0, ErrMalformedPHC
	}
	return salt, key, uint32(t), uint32(m), uint8(p), nil
}
