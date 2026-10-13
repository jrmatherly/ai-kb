package runtime

import (
	"context"
	"testing"

	"github.com/jrmatherly/ai-kb/internal/utils"
	"github.com/stretchr/testify/require"
)

func TestValidateSecrets(t *testing.T) {
	cases := []struct {
		name, jwt, aes, wantErr string
	}{
		{"shipped defaults start", utils.DefaultJWTSecret, utils.DefaultAESKey, ""},
		{"own values start", "operator-secret", "0123456789abcdef0123456789abcdef", ""},
		{"missing JWT", "", utils.DefaultAESKey, "JWT_SECRET is not set"},
		{"missing AES", "operator-secret", "", "SYSTEM_AES_KEY is not set"},
		{"short AES", "operator-secret", "too-short", "SYSTEM_AES_KEY is 9 bytes long"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("JWT_SECRET", tc.jwt)
			t.Setenv("SYSTEM_AES_KEY", tc.aes)
			err := ValidateSecrets(context.Background())
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}
