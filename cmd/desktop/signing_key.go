package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jrmatherly/ai-kb/internal/utils"
)

// ensureDesktopSigningKey persists the desktop signing key and, when no
// explicit secret is configured, a JWT secret and an AES key. It never
// replaces a configured AES key, so upgrading cannot make stored credentials
// unreadable.
// The JWT secret is never that AES key: a leaked session secret must not
// decrypt stored credentials.
func ensureDesktopSigningKey() error {
	dir, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	dir = filepath.Join(dir, "AI KB Lite")
	if err := ensureDesktopSigningKeyInDir(dir); err != nil {
		return err
	}
	return ensureDesktopAESKey(dir)
}

func ensureDesktopSigningKeyInDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	// An existing HMAC (an explicit signing key, or the AES key of an older
	// install) stays the signer exactly as configured: replacing it would
	// invalidate presigned URLs, and copying it into signing.key would leave
	// the AES key in a second file. Sessions then get their own secret.
	if len(utils.SystemHMACKey()) != 0 {
		return ensureDesktopJWTSecret(dir, nil)
	}
	path := filepath.Join(dir, "signing.key")
	key, err := readOrCreateKeyFile(path, 32, base64.RawURLEncoding.EncodeToString)
	if err != nil {
		return err
	}
	if len(key) < 32 {
		return fmt.Errorf("invalid desktop signing key in %s", path)
	}
	if err := os.Setenv("SYSTEM_SIGNING_KEY", string(key)); err != nil {
		return err
	}
	return ensureDesktopJWTSecret(dir, key)
}

// ensureDesktopJWTSecret fills JWT_SECRET when it is unset or a shipped
// default. A signing key this function generated can also sign sessions.
// A nil signingKey means the signer came from the environment (possibly the
// AES key), so sessions get their own persisted secret.
func ensureDesktopJWTSecret(dir string, signingKey []byte) error {
	jwtSecret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if jwtSecret != "" && !utils.IsDefaultJWTSecret(jwtSecret) {
		return nil
	}
	secret := string(signingKey)
	if signingKey == nil {
		path := filepath.Join(dir, "jwt.key")
		key, err := readOrCreateKeyFile(path, 32, base64.RawURLEncoding.EncodeToString)
		if err != nil {
			return err
		}
		if len(key) < 32 {
			return fmt.Errorf("invalid desktop jwt secret in %s", path)
		}
		secret = string(key)
	}
	return os.Setenv("JWT_SECRET", secret)
}

// ensureDesktopAESKey gives a desktop install that has no SYSTEM_AES_KEY, or
// only the shipped default its bundled .env carries (or an older example key),
// its own persisted key.
// Values stored in plaintext before it existed stay readable: decryption passes
// them through. It runs after the signing key is settled, so a new AES key
// never becomes the HMAC fallback signer.
func ensureDesktopAESKey(dir string) error {
	if key := os.Getenv("SYSTEM_AES_KEY"); key != "" && !utils.IsDefaultAESKey(key) {
		return nil
	}
	path := filepath.Join(dir, "aes.key")
	key, err := readOrCreateKeyFile(path, 16, hex.EncodeToString)
	if err != nil {
		return err
	}
	if len(key) != 32 {
		return fmt.Errorf("invalid desktop AES key in %s", path)
	}
	return os.Setenv("SYSTEM_AES_KEY", string(key))
}

// readOrCreateKeyFile returns the key stored at path. When the file does not
// exist it writes size random bytes there, encoded with encode. O_EXCL makes a
// concurrent first start read the winner's key instead of overwriting it.
func readOrCreateKeyFile(path string, size int, encode func([]byte) string) ([]byte, error) {
	key, err := os.ReadFile(path)
	if !os.IsNotExist(err) {
		return key, err
	}
	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	newKey := encode(raw)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if os.IsExist(err) {
		return os.ReadFile(path)
	}
	if err != nil {
		return nil, err
	}
	_, err = f.WriteString(newKey)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	return []byte(newKey), err
}
