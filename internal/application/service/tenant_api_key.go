package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"

	apprepo "github.com/jrmatherly/ai-kb/internal/application/repository"
	"github.com/jrmatherly/ai-kb/internal/logger"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
)

// apiKeyLastUsedMinInterval bounds how often we persist last_used_at per key.
// The UI only needs minute-level freshness; throttling avoids a DB write on
// every authenticated request under high QPS.
const apiKeyLastUsedMinInterval = time.Minute

type tenantAPIKeyService struct {
	repo          interfaces.TenantAPIKeyRepository
	lastUsedTouch sync.Map // key ID (uint64) -> time.Time of last persisted touch
}

func NewTenantAPIKeyService(repo interfaces.TenantAPIKeyRepository) interfaces.TenantAPIKeyService {
	return &tenantAPIKeyService{repo: repo}
}

func (s *tenantAPIKeyService) CreateAPIKey(
	ctx context.Context, req interfaces.TenantAPIKeyCreateRequest,
) (*interfaces.TenantAPIKeyCreateResult, error) {
	scopeType := types.NormalizeAPIKeyScopeType(req.ScopeType)
	if scopeType == types.APIKeyScopeTenant && req.TenantID == 0 {
		return nil, errors.New("tenant_id is required")
	}
	if scopeType == types.APIKeyScopePlatform && req.FullAccess {
		return nil, errors.New("platform API keys require explicit capabilities")
	}
	capabilities := types.NormalizeAPIKeyCapabilities(types.StringArray(req.Capabilities))
	if scopeType == types.APIKeyScopePlatform && len(capabilities) == 0 {
		return nil, errors.New("platform API keys require at least one capability")
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errors.New("name is required")
	}
	token, err := generateTenantAPIKeyToken()
	if err != nil {
		return nil, err
	}
	expiresAt := req.ExpiresAt
	if expiresAt != nil {
		utc := expiresAt.UTC()
		expiresAt = &utc
	}
	var tenantID *uint64
	if scopeType == types.APIKeyScopeTenant {
		tenantID = &req.TenantID
	}
	key := &types.TenantAPIKey{
		TenantID:         tenantID,
		ScopeType:        scopeType,
		Name:             name,
		KeyHash:          hashTenantAPIKey(token),
		KeyHint:          types.TenantAPIKeyHint(token),
		FullAccess:       req.FullAccess,
		KnowledgeBaseIDs: normalizeAPIKeyIDs(req.KnowledgeBaseIDs),
		Capabilities:     capabilities,
		ExpiresAt:        expiresAt,
	}
	if key.FullAccess {
		key.KnowledgeBaseIDs = nil
		key.Capabilities = nil
	}
	if err := s.repo.CreateAPIKey(ctx, key); err != nil {
		return nil, err
	}
	return &interfaces.TenantAPIKeyCreateResult{APIKey: key, Token: token}, nil
}

func (s *tenantAPIKeyService) AuthenticateAPIKey(ctx context.Context, token string) (*types.TenantAPIKey, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, apprepo.ErrTenantAPIKeyNotFound
	}
	key, err := s.repo.GetAPIKeyByHash(ctx, hashTenantAPIKey(token))
	if err != nil {
		return nil, err
	}
	if key.RevokedAt != nil {
		return nil, apprepo.ErrTenantAPIKeyNotFound
	}
	if key.ExpiresAt != nil && time.Now().UTC().After(key.ExpiresAt.UTC()) {
		return nil, apprepo.ErrTenantAPIKeyNotFound
	}
	s.touchAPIKeyLastUsedAsync(key.ID)
	return key, nil
}

// touchAPIKeyLastUsedAsync persists last_used_at at most once per key per
// apiKeyLastUsedMinInterval. The write runs in a detached goroutine so auth
// latency is not tied to an UPDATE on the hot path.
func (s *tenantAPIKeyService) touchAPIKeyLastUsedAsync(keyID uint64) {
	now := time.Now().UTC()
	if v, ok := s.lastUsedTouch.Load(keyID); ok {
		if now.Sub(v.(time.Time)) < apiKeyLastUsedMinInterval {
			return
		}
	}
	s.lastUsedTouch.Store(keyID, now)
	go func(id uint64, at time.Time) {
		if err := s.repo.UpdateAPIKeyLastUsed(context.Background(), id, at); err != nil {
			logger.Warnf(context.Background(),
				"failed to update tenant api key last_used_at (id=%d): %v", id, err)
			s.lastUsedTouch.Delete(id)
		}
	}(keyID, now)
}

func (s *tenantAPIKeyService) ListAPIKeys(ctx context.Context, tenantID uint64) ([]*types.TenantAPIKey, error) {
	return s.repo.ListAPIKeys(ctx, tenantID)
}

func (s *tenantAPIKeyService) ListPlatformAPIKeys(ctx context.Context) ([]*types.TenantAPIKey, error) {
	return s.repo.ListPlatformAPIKeys(ctx)
}

// UpdateAPIKey updates a workspace API key with the same semantics as creation.
// A scoped key needs at least one capability; a full-access key clears the fine-grained capabilities and knowledge base
// scope.
func (s *tenantAPIKeyService) UpdateAPIKey(
	ctx context.Context, req interfaces.TenantAPIKeyUpdateRequest,
) (*types.TenantAPIKey, error) {
	if req.TenantID == 0 {
		return nil, errors.New("tenant_id is required")
	}
	if req.APIKeyID == 0 {
		return nil, errors.New("api_key_id is required")
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errors.New("name is required")
	}
	capabilities := types.NormalizeAPIKeyCapabilities(types.StringArray(req.Capabilities))
	if !req.FullAccess && len(capabilities) == 0 {
		return nil, errors.New("capabilities are required for scoped API keys")
	}
	expiresAt := req.ExpiresAt
	if expiresAt != nil {
		utc := expiresAt.UTC()
		expiresAt = &utc
	}
	key := &types.TenantAPIKey{
		Name:             name,
		FullAccess:       req.FullAccess,
		KnowledgeBaseIDs: normalizeAPIKeyIDs(req.KnowledgeBaseIDs),
		Capabilities:     capabilities,
		ExpiresAt:        expiresAt,
	}
	if key.FullAccess {
		key.KnowledgeBaseIDs = nil
		key.Capabilities = nil
	}
	return s.repo.UpdateAPIKey(ctx, req.TenantID, req.APIKeyID, key)
}

func (s *tenantAPIKeyService) RevokeAPIKey(ctx context.Context, tenantID uint64, id uint64) error {
	return s.repo.RevokeAPIKey(ctx, tenantID, id)
}

func (s *tenantAPIKeyService) RevokePlatformAPIKey(ctx context.Context, id uint64) error {
	return s.repo.RevokePlatformAPIKey(ctx, id)
}

func generateTenantAPIKeyToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "sk-" + base64.RawURLEncoding.EncodeToString(b[:]), nil
}

func hashTenantAPIKey(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func normalizeAPIKeyIDs(in []string) types.StringArray {
	out := types.StringArray{}
	seen := map[string]struct{}{}
	for _, id := range in {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
