package datasource

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jrmatherly/ai-kb/internal/utils"
)

// ValidateConnectorBaseURL checks a connector API base URL against the SSRF policy.
// Empty rawURL is allowed; callers apply their own default before issuing requests.
func ValidateConnectorBaseURL(rawURL string) error {
	url := strings.TrimSpace(rawURL)
	if url == "" {
		return nil
	}
	if !strings.Contains(url, "://") {
		url = "https://" + url
	}
	if err := utils.ValidateURLForSSRF(url); err != nil {
		return fmt.Errorf("base_url SSRF validation failed: %w", err)
	}
	return nil
}

// NewConnectorHTTPClient returns an HTTP client with redirect and dial-time SSRF guards.
func NewConnectorHTTPClient(timeout time.Duration) *http.Client {
	cfg := utils.DefaultSSRFSafeHTTPClientConfig()
	cfg.Timeout = timeout
	return utils.NewSSRFSafeHTTPClient(cfg)
}

const (
	// MaxJSONResponseBytes caps a JSON API response: a truncated body would be
	// indexed as if it were the whole document, so a larger one is refused.
	MaxJSONResponseBytes int64 = 16 << 20
	// MaxPaginationHops bounds any next-link walk so a looping or hostile server
	// cannot pin a sync.
	MaxPaginationHops = 10000
	// CheckpointEvery is how many emitted items a streaming sync lets pass
	// between cursor checkpoints.
	CheckpointEvery = 50
)

// ReadCapped reads a body, refusing anything larger than limit instead of
// buffering it; a limit of zero or less means MaxJSONResponseBytes.
func ReadCapped(body io.Reader, limit int64) ([]byte, error) {
	if limit <= 0 {
		limit = MaxJSONResponseBytes
	}
	data, err := io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%w (%d bytes)", ErrResponseTooLarge, limit)
	}
	return data, nil
}

// SleepContext waits for d or until ctx ends, whichever comes first.
func SleepContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
