package api

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	secutils "github.com/jrmatherly/ai-kb/internal/utils"
)

// LLM call timeouts. They are only a fallback for when the caller set no deadline, so a hung request
// cannot block a worker forever. A deadline already on the caller's ctx (shorter or longer than the default)
// is respected as is, with no default timeout stacked on top. Override with environment variables:
//   - AIKB_LLM_CHAT_TIMEOUT_SECONDS    fallback timeout for non-streaming calls (default 600s)
//   - AIKB_LLM_STREAM_TIMEOUT_SECONDS  fallback timeout for streaming calls (default 1800s)
var (
	DefaultChatTimeout   = envDurationSeconds("AIKB_LLM_CHAT_TIMEOUT_SECONDS", 300*time.Second)
	DefaultStreamTimeout = envDurationSeconds("AIKB_LLM_STREAM_TIMEOUT_SECONDS", 600*time.Second)
)

// envDurationSeconds reads an environment variable in seconds, falling back to fallback
// when it fails to parse or is not positive.
func envDurationSeconds(key string, fallback time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return fallback
	}
	return time.Duration(n) * time.Second
}

// WithLLMTimeout adds a fallback timeout only when the caller's ctx has no deadline;
// when the caller set a deadline explicitly (shorter or longer) the ctx is returned as is,
// so the caller has the final say over its own timeout policy.
func WithLLMTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, d)
}

// HTTPClient is a shared HTTP client for raw HTTP LLM calls with connection-level timeouts.
// Per-request timeout is enforced via context deadline (see DefaultChatTimeout / DefaultStreamTimeout)
// rather than http.Client.Timeout, so streaming calls are not prematurely terminated.
// Uses SSRFSafeDialContext to prevent DNS rebinding attacks at the connection layer.
var httpTransport = &http.Transport{
	Proxy:               http.ProxyFromEnvironment,
	DialContext:         secutils.SSRFSafeDialContext,
	TLSHandshakeTimeout: 10 * time.Second,
	IdleConnTimeout:     90 * time.Second,
	MaxIdleConnsPerHost: 5,
}

// HTTPClient is the shared SSRF-safe client every protocol uses.
var HTTPClient = secutils.NewSSRFSafeHTTPClientWithTransport(
	secutils.SSRFSafeHTTPClientConfig{Timeout: 0, MaxRedirects: 10},
	httpTransport,
)
