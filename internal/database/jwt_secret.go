package database

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/jrmatherly/ai-kb/internal/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// systemSecret is a row of system_secrets: server-generated secrets that no API reads or lists.
type systemSecret struct {
	Name  string `gorm:"primaryKey"`
	Value string
}

func (systemSecret) TableName() string { return "system_secrets" }

const jwtSecretName = "jwt_secret"

// ResolveDefaultJWTSecret replaces a shipped default JWT_SECRET with a random per-install secret kept in
// system_secrets, so sessions survive restarts and nobody can sign tokens with the public default. Every replica
// inserts its candidate only when no row exists and then reads the row back, so they all converge on one value.
func ResolveDefaultJWTSecret(db *gorm.DB) error {
	if !utils.IsDefaultJWTSecret(os.Getenv("JWT_SECRET")) {
		return nil
	}
	candidate := make([]byte, 32)
	if _, err := rand.Read(candidate); err != nil {
		return err
	}
	if err := db.Clauses(clause.OnConflict{DoNothing: true}).
		Create(&systemSecret{Name: jwtSecretName, Value: hex.EncodeToString(candidate)}).Error; err != nil {
		return fmt.Errorf("store the per-install JWT secret (the system_secrets table comes from the migrations; "+
			"run them, or set your own JWT_SECRET): %w", err)
	}
	var stored systemSecret
	if err := db.Where("name = ?", jwtSecretName).First(&stored).Error; err != nil {
		return fmt.Errorf("read the per-install JWT secret: %w", err)
	}
	return os.Setenv("JWT_SECRET", stored.Value)
}
