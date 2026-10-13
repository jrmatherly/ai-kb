package interfaces

import (
	"context"

	"github.com/jrmatherly/ai-kb/internal/types"
)

// WeKnoraCloudService manages WeKnoraCloud credentials
type WeKnoraCloudService interface {
	// SaveCredentials only saves the APPID/APPSECRET credentials to the workspace config; it creates no models
	SaveCredentials(ctx context.Context, appID, appSecret string) error
	// CheckStatus checks whether the current workspace's WeKnoraCloud credentials decrypt
	// needsReinit=true means the encrypted state is broken (a changed salt, for example) and the
	// user must re-enter the credentials
	CheckStatus(ctx context.Context) (*types.WeKnoraCloudStatusResult, error)
}
