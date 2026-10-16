package service

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	apperrors "github.com/jrmatherly/ai-kb/internal/errors"
	"github.com/jrmatherly/ai-kb/internal/logger"
	"github.com/jrmatherly/ai-kb/internal/models/providers"
	modelsutils "github.com/jrmatherly/ai-kb/internal/models/utils"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/jrmatherly/ai-kb/internal/types/interfaces"
	"github.com/jrmatherly/ai-kb/internal/utils"
)

type weKnoraCloudService struct {
	tenantRepo interfaces.TenantRepository
}

// NewWeKnoraCloudService builds a WeKnoraCloudService.
func NewWeKnoraCloudService(
	repo interfaces.ModelRepository,
	tenantRepo interfaces.TenantRepository,
) interfaces.WeKnoraCloudService {
	return &weKnoraCloudService{
		tenantRepo: tenantRepo,
	}
}

func IsWeKnoraCloudDocReaderAddr(addr string) bool {
	readerURL := strings.TrimRight(providers.WeKnoraCloudBaseURL, "/") + "/api/v1/doc/reader"
	return strings.TrimSuffix(strings.TrimSpace(addr), "/") == readerURL
}

// SaveCredentials stores only the APPID/APPSECRET credentials; it does not create models.
func (s *weKnoraCloudService) SaveCredentials(ctx context.Context, appID, appSecret string) error {
	if appID == "" {
		return apperrors.NewBadRequestError("app_id is required")
	}
	if appSecret == "" {
		return apperrors.NewBadRequestError("app_secret is required")
	}

	if err := s.verifyCredentials(ctx, appID, appSecret); err != nil {
		return err
	}

	tenantID := types.MustTenantIDFromContext(ctx)
	return s.updateTenantCredentials(ctx, tenantID, appID, appSecret)
}

// verifyCredentials sends a GET with signature headers to WeKnoraCloud /api/v1/health.
//
// health is usually a liveness probe, and the remote often does not check the APPID/SECRET or the
// signature, so HTTP 200 usually only means the gateway or service is reachable and does not prove
// the credentials are valid. A strict check would have to call an endpoint that requires auth.
func (s *weKnoraCloudService) verifyCredentials(ctx context.Context, appID, appSecret string) error {
	baseURL := strings.TrimRight(providers.WeKnoraCloudBaseURL, "/")
	healthURL := baseURL + "/api/v1/health"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		return fmt.Errorf("create verification request failed: %w", err)
	}

	requestID := fmt.Sprintf("verify-%d", time.Now().UnixNano())
	signHeaders := modelsutils.Sign(appID, appSecret, requestID, "{}")
	for k, v := range signHeaders {
		req.Header.Set(k, v)
	}

	logger.Infof(ctx, "credential verification request: method=GET url=%s app_id=%s request_id=%s ",
		healthURL, appID, requestID)

	clientCfg := utils.DefaultSSRFSafeHTTPClientConfig()
	clientCfg.Timeout = 10 * time.Second
	client := utils.NewSSRFSafeHTTPClient(clientCfg)
	resp, err := client.Do(req)
	if err != nil {
		logger.Warnf(ctx, "credential verification HTTP failed: url=%s err=%v", healthURL, err)
		return apperrors.NewBadGatewayError("credential verification failed: service unreachable: " + err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return apperrors.NewBadRequestError(fmt.Sprintf(
			"credential verification failed: invalid APPID or APPSECRET (HTTP %d)", resp.StatusCode))
	}
	if resp.StatusCode != http.StatusOK {
		return apperrors.NewBadGatewayError(fmt.Sprintf(
			"credential verification failed: invalid response status code: %d", resp.StatusCode))
	}
	return nil
}

// CheckStatus reports whether the WeKnoraCloud credentials can be decrypted.
func (s *weKnoraCloudService) CheckStatus(ctx context.Context) (*types.WeKnoraCloudStatusResult, error) {
	tenantID := types.MustTenantIDFromContext(ctx)

	tenant, err := s.tenantRepo.GetTenantByID(ctx, tenantID)
	if err != nil || tenant == nil {
		return &types.WeKnoraCloudStatusResult{HasModels: false, NeedsReinit: false}, nil
	}

	creds := tenant.Credentials.GetWeKnoraCloud()
	if creds == nil {
		return &types.WeKnoraCloudStatusResult{HasModels: false, NeedsReinit: false}, nil
	}

	// CredentialsConfig.Scan already attempts decryption.
	// If the AES key has rotated, Scan silently keeps the enc:v1:... blob.
	if strings.HasPrefix(creds.AppSecret, utils.EncPrefix) {
		return &types.WeKnoraCloudStatusResult{
			HasModels:   true,
			NeedsReinit: true,
			Reason: "Failed to decrypt the WeKnoraCloud credentials (the encryption key changed after a " +
				"restart). Enter the APPID and APPSECRET again.",
		}, nil
	}

	return &types.WeKnoraCloudStatusResult{HasModels: true, NeedsReinit: false}, nil
}

// updateTenantCredentials updates the workspace's WeKnoraCloud credentials.
func (s *weKnoraCloudService) updateTenantCredentials(ctx context.Context, tenantID uint64, appID, appSecret string) error {
	if s.tenantRepo == nil {
		return fmt.Errorf("tenant repository is required")
	}

	tenant, err := s.tenantRepo.GetTenantByID(ctx, tenantID)
	if err != nil {
		return err
	}
	if tenant.Credentials == nil {
		tenant.Credentials = &types.CredentialsConfig{}
	}
	tenant.Credentials.WeKnoraCloud = &types.WeKnoraCloudCredentials{
		AppID:     appID,
		AppSecret: appSecret,
	}
	return s.tenantRepo.UpdateTenant(ctx, tenant)
}
