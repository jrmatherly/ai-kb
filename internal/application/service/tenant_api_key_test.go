package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	apprepo "github.com/jrmatherly/ai-kb/internal/application/repository"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
)

// fakeTenantAPIKeyRepo locks like a database would: the service writes
// last_used_at from a detached goroutine.
type fakeTenantAPIKeyRepo struct {
	mu                  sync.Mutex
	byHash              map[string]*types.TenantAPIKey
	nextID              uint64
	lastUsedUpdateCount int
}

func TestTenantAPIKeyServiceCreateAPIKeyUsesSKPrefix(t *testing.T) {
	ctx := context.Background()
	repo := newFakeTenantAPIKeyRepo()
	svc := NewTenantAPIKeyService(repo)

	result, err := svc.CreateAPIKey(ctx, interfaces.TenantAPIKeyCreateRequest{
		TenantID: 42,
		Name:     "integration",
	})
	if err != nil {
		t.Fatalf("CreateAPIKey returned error: %v", err)
	}
	if !strings.HasPrefix(result.Token, "sk-") {
		t.Fatalf("created token = %q, want sk- prefix", result.Token)
	}
	if want := result.Token[:7] + "..." + result.Token[len(result.Token)-4:]; result.APIKey.KeyHint != want {
		t.Fatalf("created key_hint = %q, want %q", result.APIKey.KeyHint, want)
	}
}

func newFakeTenantAPIKeyRepo() *fakeTenantAPIKeyRepo {
	return &fakeTenantAPIKeyRepo{byHash: map[string]*types.TenantAPIKey{}, nextID: 1}
}

func (r *fakeTenantAPIKeyRepo) CreateAPIKey(_ context.Context, key *types.TenantAPIKey) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byHash[key.KeyHash]; ok {
		return errors.New("duplicate key hash")
	}
	cp := *key
	cp.ID = r.nextID
	r.nextID++
	r.byHash[cp.KeyHash] = &cp
	key.ID = cp.ID
	return nil
}

func (r *fakeTenantAPIKeyRepo) GetAPIKeyByHash(_ context.Context, hash string) (*types.TenantAPIKey, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key, ok := r.byHash[hash]
	if !ok {
		return nil, apprepo.ErrTenantAPIKeyNotFound
	}
	cp := *key
	return &cp, nil
}

func (r *fakeTenantAPIKeyRepo) ListAPIKeys(_ context.Context, tenantID uint64) ([]*types.TenantAPIKey, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []*types.TenantAPIKey{}
	for _, key := range r.byHash {
		if key.TenantIDValue() == tenantID && key.RevokedAt == nil {
			cp := *key
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *fakeTenantAPIKeyRepo) ListPlatformAPIKeys(_ context.Context) ([]*types.TenantAPIKey, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []*types.TenantAPIKey{}
	for _, key := range r.byHash {
		if key.IsPlatform() && key.RevokedAt == nil {
			cp := *key
			out = append(out, &cp)
		}
	}
	return out, nil
}

func (r *fakeTenantAPIKeyRepo) RevokeAPIKey(_ context.Context, tenantID uint64, id uint64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	for _, key := range r.byHash {
		if key.ID == id && key.TenantIDValue() == tenantID && key.RevokedAt == nil {
			key.RevokedAt = &now
			return nil
		}
	}
	return apprepo.ErrTenantAPIKeyNotFound
}

// UpdateAPIKey mimics the repository's tenant boundary and overwrites the configurable API key fields.
// It takes a tenant ID, a key ID and the new configuration and returns the updated key; a key in another tenant or a
// revoked key is not found.
func (r *fakeTenantAPIKeyRepo) UpdateAPIKey(
	_ context.Context, tenantID uint64, id uint64, update *types.TenantAPIKey,
) (*types.TenantAPIKey, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, key := range r.byHash {
		if key.ID == id && key.TenantIDValue() == tenantID && key.RevokedAt == nil {
			key.Name = update.Name
			key.FullAccess = update.FullAccess
			key.KnowledgeBaseIDs = append(types.StringArray(nil), update.KnowledgeBaseIDs...)
			key.Capabilities = append(types.StringArray(nil), update.Capabilities...)
			key.ExpiresAt = update.ExpiresAt
			cp := *key
			return &cp, nil
		}
	}
	return nil, apprepo.ErrTenantAPIKeyNotFound
}

// TestTenantAPIKeyServiceUpdateNormalizesConfiguration checks input normalization on a general update.
// The input has duplicate IDs and capabilities and a UTC+8 expiry; the output is deduplicated, the name cleaned and the
// time in UTC.
func TestTenantAPIKeyServiceUpdateNormalizesConfiguration(t *testing.T) {
	ctx := context.Background()
	repo := newFakeTenantAPIKeyRepo()
	svc := NewTenantAPIKeyService(repo)
	created, err := svc.CreateAPIKey(ctx, interfaces.TenantAPIKeyCreateRequest{
		TenantID: 42, Name: "scoped", Capabilities: []string{"retrieve"},
	})
	if err != nil {
		t.Fatalf("CreateAPIKey returned error: %v", err)
	}

	expiresAt := time.Date(2026, 9, 1, 12, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	updated, err := svc.UpdateAPIKey(ctx, interfaces.TenantAPIKeyUpdateRequest{
		TenantID: 42, APIKeyID: created.APIKey.ID,
		Name: " updated ", Capabilities: []string{"retrieve", "chat", "retrieve"},
		KnowledgeBaseIDs: []string{" kb-1 ", "", "kb-2", "kb-1"},
		ExpiresAt:        &expiresAt,
	})
	if err != nil {
		t.Fatalf("UpdateAPIKey returned error: %v", err)
	}
	if updated.Name != "updated" {
		t.Fatalf("name = %q, want updated", updated.Name)
	}
	if got, want := []string(updated.KnowledgeBaseIDs), []string{"kb-1", "kb-2"}; !apiKeyEqualStrings(got, want) {
		t.Fatalf("knowledge_base_ids = %#v, want %#v", got, want)
	}
	if got, want := []string(updated.Capabilities), []string{"retrieve", "chat"}; !apiKeyEqualStrings(got, want) {
		t.Fatalf("capabilities = %#v, want %#v", got, want)
	}
	if updated.ExpiresAt == nil || updated.ExpiresAt.Location() != time.UTC {
		t.Fatalf("expires_at = %v, want UTC", updated.ExpiresAt)
	}

	full, err := svc.UpdateAPIKey(ctx, interfaces.TenantAPIKeyUpdateRequest{
		TenantID: 42, APIKeyID: created.APIKey.ID, Name: "full", FullAccess: true,
		KnowledgeBaseIDs: []string{"kb-ignored"}, Capabilities: []string{"retrieve"},
	})
	if err != nil {
		t.Fatalf("updating to full access returned error: %v", err)
	}
	if !full.FullAccess || len(full.KnowledgeBaseIDs) != 0 || len(full.Capabilities) != 0 {
		t.Fatalf("full access scope = full:%v kbs:%v caps:%v, want true/empty/empty",
			full.FullAccess, full.KnowledgeBaseIDs, full.Capabilities)
	}
}

func apiKeyEqualStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (r *fakeTenantAPIKeyRepo) RevokePlatformAPIKey(_ context.Context, id uint64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	for _, key := range r.byHash {
		if key.ID == id && key.IsPlatform() && key.RevokedAt == nil {
			key.RevokedAt = &now
			return nil
		}
	}
	return apprepo.ErrTenantAPIKeyNotFound
}

func (r *fakeTenantAPIKeyRepo) updateCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastUsedUpdateCount
}

func (r *fakeTenantAPIKeyRepo) UpdateAPIKeyLastUsed(_ context.Context, id uint64, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastUsedUpdateCount++
	for _, key := range r.byHash {
		if key.ID == id && key.RevokedAt == nil {
			key.LastUsedAt = &at
		}
	}
	return nil
}

func uint64Pointer(value uint64) *uint64 { return &value }

func TestTenantAPIKeyServiceCreatesPlatformKeyWithoutTenant(t *testing.T) {
	repo := newFakeTenantAPIKeyRepo()
	svc := NewTenantAPIKeyService(repo)
	created, err := svc.CreateAPIKey(context.Background(), interfaces.TenantAPIKeyCreateRequest{
		ScopeType:    types.APIKeyScopePlatform,
		Name:         "automation",
		Capabilities: []string{string(types.APIKeyCapabilityRetrieve)},
	})
	if err != nil {
		t.Fatalf("CreateAPIKey returned error: %v", err)
	}
	if !created.APIKey.IsPlatform() || created.APIKey.TenantID != nil {
		t.Fatalf("created key scope = %q tenant=%v, want platform with nil tenant", created.APIKey.ScopeType, created.APIKey.TenantID)
	}
	if created.APIKey.FullAccess {
		t.Fatal("platform API key must not be full-access")
	}
}

func TestTenantAPIKeyServiceRejectsFullAccessPlatformKey(t *testing.T) {
	svc := NewTenantAPIKeyService(newFakeTenantAPIKeyRepo())
	_, err := svc.CreateAPIKey(context.Background(), interfaces.TenantAPIKeyCreateRequest{
		ScopeType: types.APIKeyScopePlatform,
		Name:      "unsafe", FullAccess: true,
	})
	if err == nil {
		t.Fatal("full-access platform key should be rejected")
	}
}

func TestTenantAPIKeyServiceRevokeAPIKey(t *testing.T) {
	ctx := context.Background()
	repo := newFakeTenantAPIKeyRepo()
	svc := NewTenantAPIKeyService(repo)

	created, err := svc.CreateAPIKey(ctx, interfaces.TenantAPIKeyCreateRequest{
		TenantID: 42,
		Name:     "integration",
	})
	if err != nil {
		t.Fatalf("CreateAPIKey returned error: %v", err)
	}
	if err := svc.RevokeAPIKey(ctx, 42, created.APIKey.ID); err != nil {
		t.Fatalf("RevokeAPIKey returned error: %v", err)
	}
	if _, err := svc.AuthenticateAPIKey(ctx, created.Token); err == nil {
		t.Fatal("revoked key should not authenticate")
	}
}

func TestTenantAPIKeyServiceAuthenticateThrottlesLastUsedUpdates(t *testing.T) {
	ctx := context.Background()
	repo := newFakeTenantAPIKeyRepo()
	svc := NewTenantAPIKeyService(repo)

	created, err := svc.CreateAPIKey(ctx, interfaces.TenantAPIKeyCreateRequest{
		TenantID: 42,
		Name:     "integration",
	})
	if err != nil {
		t.Fatalf("CreateAPIKey returned error: %v", err)
	}

	for i := 0; i < 5; i++ {
		if _, err := svc.AuthenticateAPIKey(ctx, created.Token); err != nil {
			t.Fatalf("AuthenticateAPIKey #%d returned error: %v", i+1, err)
		}
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for repo.updateCount() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if n := repo.updateCount(); n != 1 {
		t.Fatalf("last_used update count = %d, want 1 (throttled async write)", n)
	}
}

func TestTenantAPIKeyServiceAuthenticateRejectsExpiredKey(t *testing.T) {
	ctx := context.Background()
	repo := newFakeTenantAPIKeyRepo()
	svc := NewTenantAPIKeyService(repo)

	expired := time.Now().UTC().Add(-time.Minute)
	created, err := svc.CreateAPIKey(ctx, interfaces.TenantAPIKeyCreateRequest{
		TenantID:  42,
		Name:      "short-lived",
		ExpiresAt: &expired,
	})
	if err != nil {
		t.Fatalf("CreateAPIKey returned error: %v", err)
	}
	if _, err := svc.AuthenticateAPIKey(ctx, created.Token); err == nil {
		t.Fatal("expired key should not authenticate")
	}
}
