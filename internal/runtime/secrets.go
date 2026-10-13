package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/jrmatherly/ai-kb/internal/logger"
	"github.com/jrmatherly/ai-kb/internal/utils"
)

// ValidateSecrets refuses a start without a usable SYSTEM_AES_KEY and JWT_SECRET: without them credentials would be
// stored in plaintext and every restart would sign everyone out. The shipped defaults pass, with a warning.
func ValidateSecrets(ctx context.Context) error {
	var problems []string
	if strings.TrimSpace(os.Getenv("JWT_SECRET")) == "" {
		problems = append(problems, "JWT_SECRET is not set; generate one with `openssl rand -hex 32`")
	}
	switch aesKey := os.Getenv("SYSTEM_AES_KEY"); {
	case aesKey == "":
		problems = append(problems,
			"SYSTEM_AES_KEY is not set; generate one with `openssl rand -hex 16` (32 characters)")
	case len(aesKey) != 32:
		problems = append(problems, fmt.Sprintf(
			"SYSTEM_AES_KEY is %d bytes long; it must be exactly 32 (`openssl rand -hex 16`)", len(aesKey)))
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}

	if utils.IsDefaultAESKey(os.Getenv("SYSTEM_AES_KEY")) {
		logger.Warn(ctx, "[startup-env] SYSTEM_AES_KEY is the shipped default: stored credentials are encrypted "+
			"with a public key. Set your own (`openssl rand -hex 16`) before storing real credentials; values "+
			"saved under the default become unreadable when the key changes and must be entered again")
	}
	if utils.IsDefaultJWTSecret(os.Getenv("JWT_SECRET")) {
		logger.Warn(ctx, "[startup-env] JWT_SECRET is the shipped default: it is never used to sign sessions; "+
			"a per-install secret stored in the database is used instead. Set your own (`openssl rand -hex 32`)")
	}
	return nil
}
