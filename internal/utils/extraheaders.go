package utils

import (
	"net/http"
	"strings"
)

// reservedHeaderKeys lists request headers that user-defined headers may not override.
// Each provider's signing, auth or SSE flow depends on them; overriding one can make the call fail outright.
var reservedHeaderKeys = map[string]struct{}{
	"authorization":     {},
	"api-key":           {},
	"x-api-key":         {},
	"x-goog-api-key":    {},
	"content-type":      {},
	"content-length":    {},
	"accept-encoding":   {},
	"host":              {},
	"connection":        {},
	"transfer-encoding": {},
}

// IsReservedHeader reports whether a header key is reserved; reserved headers cannot be overridden by custom headers.
func IsReservedHeader(key string) bool {
	_, ok := reservedHeaderKeys[strings.ToLower(strings.TrimSpace(key))]
	return ok
}

// ApplyCustomHeaders writes the user's custom headers into an http.Request.
// Reserved headers (Authorization, api-key, Content-Type, ...) are skipped so auth and signing stay intact.
// Other headers overwrite entries of the same name, so users can replace defaults (such as Accept).
func ApplyCustomHeaders(req *http.Request, headers map[string]string) {
	if req == nil || len(headers) == 0 {
		return
	}
	for k, v := range headers {
		name := strings.TrimSpace(k)
		if name == "" {
			continue
		}
		if IsReservedHeader(name) {
			continue
		}
		req.Header.Set(name, v)
	}
}
