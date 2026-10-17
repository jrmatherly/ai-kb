package utils

import (
	"encoding/base64"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSignAndVerifyOIDCState(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-oidc-state-secret")
	oidcStateSecretOnce = sync.Once{}
	oidcStateSecret = ""

	state, err := SignOIDCState(&OIDCStatePayload{
		Nonce:       "nonce-abc",
		RedirectURI: "http://localhost:5173/login",
		IssuedAt:    time.Now().Unix(),
	})
	if err != nil {
		t.Fatalf("SignOIDCState: %v", err)
	}
	got, err := VerifyOIDCState(state)
	if err != nil {
		t.Fatalf("VerifyOIDCState: %v", err)
	}
	if got.Nonce != "nonce-abc" || got.RedirectURI != "http://localhost:5173/login" {
		t.Fatalf("unexpected payload: %+v", got)
	}
}

func TestVerifyOIDCStateRejectsTamperedPayload(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-oidc-state-secret")
	oidcStateSecretOnce = sync.Once{}
	oidcStateSecret = ""

	state, err := SignOIDCState(&OIDCStatePayload{
		Nonce:       "nonce-abc",
		RedirectURI: "http://localhost:5173/login",
		IssuedAt:    time.Now().Unix(),
	})
	if err != nil {
		t.Fatalf("SignOIDCState: %v", err)
	}
	parts := strings.Split(state, ".")
	tampered := parts[0] + ".AAAA"
	if _, err := VerifyOIDCState(tampered); err == nil {
		t.Fatal("expected tampered state to be rejected")
	}
}

func TestSignOIDCState_CarriesReturnTo(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-oidc-state-secret")
	oidcStateSecretOnce = sync.Once{}
	oidcStateSecret = ""

	withReturn, err := SignOIDCState(&OIDCStatePayload{
		Nonce:       "nonce-abc",
		RedirectURI: "http://localhost:8080/api/v1/auth/oidc/callback",
		ReturnTo:    "https://abc.chromiumapp.org/oidc",
	})
	if err != nil {
		t.Fatalf("SignOIDCState: %v", err)
	}
	got, err := VerifyOIDCState(withReturn)
	if err != nil {
		t.Fatalf("VerifyOIDCState: %v", err)
	}
	if got.ReturnTo != "https://abc.chromiumapp.org/oidc" {
		t.Fatalf("ReturnTo = %q, want the signed value", got.ReturnTo)
	}
	if !strings.Contains(string(mustDecodePayload(t, withReturn)), `"return_to"`) {
		t.Fatalf("payload should carry return_to: %s", mustDecodePayload(t, withReturn))
	}

	without, err := SignOIDCState(&OIDCStatePayload{Nonce: "nonce-abc", RedirectURI: "http://localhost:8080/cb"})
	if err != nil {
		t.Fatalf("SignOIDCState: %v", err)
	}
	plain, err := VerifyOIDCState(without)
	if err != nil {
		t.Fatalf("VerifyOIDCState: %v", err)
	}
	if plain.ReturnTo != "" {
		t.Fatalf("ReturnTo = %q, want empty", plain.ReturnTo)
	}
	if strings.Contains(string(mustDecodePayload(t, without)), "return_to") {
		t.Fatalf("payload should omit return_to when empty: %s", mustDecodePayload(t, without))
	}
}

func mustDecodePayload(t *testing.T, state string) []byte {
	t.Helper()
	raw, err := base64.RawURLEncoding.DecodeString(strings.Split(state, ".")[0])
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	return raw
}

func TestDecodeOIDCStateUnverified_ReadsThePayloadOfATamperedToken(t *testing.T) {
	state, err := SignOIDCState(&OIDCStatePayload{
		Nonce: "n", RedirectURI: "http://x/cb", ReturnTo: "https://abc.chromiumapp.org/oidc",
	})
	if err != nil {
		t.Fatalf("SignOIDCState: %v", err)
	}
	tampered := state[:len(state)-2] + "zz"
	if _, err := VerifyOIDCState(tampered); err == nil {
		t.Fatalf("tampered state verified")
	}
	payload, err := DecodeOIDCStateUnverified(tampered)
	if err != nil {
		t.Fatalf("DecodeOIDCStateUnverified: %v", err)
	}
	if payload.ReturnTo != "https://abc.chromiumapp.org/oidc" {
		t.Fatalf("ReturnTo = %q", payload.ReturnTo)
	}
	if _, err := DecodeOIDCStateUnverified("not-a-state"); err == nil {
		t.Fatalf("malformed state decoded")
	}
}
