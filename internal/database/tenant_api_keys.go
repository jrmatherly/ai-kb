package database

import (
	"context"
	"fmt"
	"strings"

	"github.com/jrmatherly/ai-kb/internal/logger"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/utils"
	"gorm.io/gorm"
)

// legacyKeyHashPrefix is the placeholder key_hash migration 000065 gives the keys it copies from tenants.api_key.
const legacyKeyHashPrefix = "migrated-tenant-"

type legacyTenantAPIKey struct {
	ID      uint64
	KeyHash string
	APIKey  string
}

// BackfillTenantAPIKeyHints converts every tenant_api_keys row that still stores its token in api_key (keys created
// before the hash-only scheme, and keys migration 000065 copied with a placeholder hash) into the hash-only form: the
// real key_hash, the key_hint, and an empty api_key. Rows already converted are skipped, so it is safe on every start.
// A row whose token cannot be decrypted (SYSTEM_AES_KEY changed) is left for a later start and reported.
func BackfillTenantAPIKeyHints(ctx context.Context, db *gorm.DB) error {
	if !db.Migrator().HasColumn("tenant_api_keys", "api_key") {
		return nil
	}
	var rows []legacyTenantAPIKey
	if err := db.WithContext(ctx).Table("tenant_api_keys").
		Select("id, key_hash, api_key").
		Where("api_key <> ''").
		Find(&rows).Error; err != nil {
		return fmt.Errorf("list tenant API keys to backfill: %w", err)
	}
	converted := 0
	for _, row := range rows {
		token, err := utils.DecryptStoredSecret(row.APIKey)
		if err != nil || token == "" {
			logger.Warnf(ctx, "[tenant-api-keys] key %d keeps its stored token: decrypt failed: %v", row.ID, err)
			continue
		}
		updates := map[string]any{"key_hint": types.TenantAPIKeyHint(token), "api_key": ""}
		if strings.HasPrefix(row.KeyHash, legacyKeyHashPrefix) {
			updates["key_hash"] = types.HashTenantAPIKey(token)
		}
		err = db.WithContext(ctx).Table("tenant_api_keys").Where("id = ?", row.ID).Updates(updates).Error
		if err != nil {
			return fmt.Errorf("backfill tenant API key %d: %w", row.ID, err)
		}
		converted++
	}
	if converted > 0 {
		logger.Infof(ctx, "[tenant-api-keys] converted %d stored token(s) to hash-only form", converted)
	}
	return nil
}
