package utils

import "strings"

// The values .env.example ships, so a first start works out of the box. Encryption under DefaultAESKey is real,
// but the key is public. A public JWT secret would let anyone mint login tokens, so the server swaps
// DefaultJWTSecret for a per-install secret (database.ResolveDefaultJWTSecret). Startup warns about both.
const (
	DefaultAESKey    = "aikb-default-aes-key-change-me!!"
	DefaultJWTSecret = "aikb-default-jwt-secret-change-me"
)

// IsDefaultAESKey reports whether key is the shipped default or an example key that older example files used.
func IsDefaultAESKey(key string) bool {
	switch key {
	case DefaultAESKey, "ai-kb-system-aes-key-32bytes!!", "your-32-byte-long-encryption-key!":
		return true
	}
	return false
}

// IsDefaultJWTSecret reports whether secret is the shipped default or a placeholder that older example files used.
func IsDefaultJWTSecret(secret string) bool {
	switch strings.TrimSpace(secret) {
	case DefaultJWTSecret, "ai-kb-jwt-secret", "CHANGE-ME-jwt-secret":
		return true
	}
	return false
}
