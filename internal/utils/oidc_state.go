package utils

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

const oidcStateMaxAge = 10 * time.Minute

// OIDCStatePayload is the signed OIDC authorization state carried in the
// redirect URL and validated on callback.
type OIDCStatePayload struct {
	Nonce       string `json:"nonce"`
	RedirectURI string `json:"redirect_uri,omitempty"`
	// ReturnTo is where the callback sends the browser instead of "/", set only
	// for an allow-listed browser extension return URL.
	ReturnTo string `json:"return_to,omitempty"`
	IssuedAt int64  `json:"iat"`
}

var (
	oidcStateSecretOnce sync.Once
	oidcStateSecret     string
)

func oidcStateSigningKey() string {
	oidcStateSecretOnce.Do(func() {
		if envSecret := strings.TrimSpace(os.Getenv("JWT_SECRET")); envSecret != "" {
			oidcStateSecret = envSecret
			return
		}
		randomBytes := make([]byte, 32)
		if _, err := rand.Read(randomBytes); err != nil {
			panic(fmt.Sprintf("failed to generate OIDC state signing key: %v", err))
		}
		oidcStateSecret = base64.StdEncoding.EncodeToString(randomBytes)
	})
	return oidcStateSecret
}

// SignOIDCState returns a tamper-evident state token: base64url(payload).base64url(hmac).
func SignOIDCState(payload *OIDCStatePayload) (string, error) {
	if payload == nil {
		return "", errors.New("oidc state payload is required")
	}
	if strings.TrimSpace(payload.Nonce) == "" {
		return "", errors.New("oidc state nonce is required")
	}
	if strings.TrimSpace(payload.RedirectURI) == "" {
		return "", errors.New("oidc state redirect_uri is required")
	}
	if payload.IssuedAt == 0 {
		payload.IssuedAt = time.Now().Unix()
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal oidc state: %w", err)
	}
	mac := hmac.New(sha256.New, []byte(oidcStateSigningKey()))
	mac.Write(raw)
	sig := mac.Sum(nil)
	return base64.RawURLEncoding.EncodeToString(raw) + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

// parseOIDCState splits and decodes a state token without checking it.
func parseOIDCState(raw string) (payloadBytes, sigBytes []byte, payload *OIDCStatePayload, err error) {
	parts := strings.Split(strings.TrimSpace(raw), ".")
	if len(parts) != 2 {
		return nil, nil, nil, errors.New("invalid oidc state format")
	}
	payloadBytes, err = base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, nil, nil, fmt.Errorf("decode oidc state payload: %w", err)
	}
	sigBytes, err = base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, nil, nil, fmt.Errorf("decode oidc state signature: %w", err)
	}
	payload = &OIDCStatePayload{}
	if err := json.Unmarshal(payloadBytes, payload); err != nil {
		return nil, nil, nil, fmt.Errorf("unmarshal oidc state: %w", err)
	}
	return payloadBytes, sigBytes, payload, nil
}

// DecodeOIDCStateUnverified returns the payload without checking the signature
// or freshness. Only for values that are checked against an allow-list anyway,
// such as choosing where an expired login lands.
func DecodeOIDCStateUnverified(raw string) (*OIDCStatePayload, error) {
	_, _, payload, err := parseOIDCState(raw)
	return payload, err
}

// VerifyOIDCState validates the HMAC and freshness of a state token.
func VerifyOIDCState(raw string) (*OIDCStatePayload, error) {
	payloadBytes, sigBytes, payload, err := parseOIDCState(raw)
	if err != nil {
		return nil, err
	}
	mac := hmac.New(sha256.New, []byte(oidcStateSigningKey()))
	mac.Write(payloadBytes)
	if !hmac.Equal(mac.Sum(nil), sigBytes) {
		return nil, errors.New("oidc state signature mismatch")
	}
	if strings.TrimSpace(payload.RedirectURI) == "" {
		return nil, errors.New("state.redirect_uri is required")
	}
	if payload.IssuedAt == 0 {
		return nil, errors.New("state.iat is required")
	}
	issuedAt := time.Unix(payload.IssuedAt, 0)
	if time.Since(issuedAt) > oidcStateMaxAge || time.Until(issuedAt) > time.Minute {
		return nil, errors.New("oidc state expired or invalid timestamp")
	}
	return payload, nil
}
