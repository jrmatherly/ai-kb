package session

import (
	"context"
	stderrors "errors"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jrmatherly/ai-kb/internal/browserskill"
	"github.com/jrmatherly/ai-kb/internal/errors"
	"github.com/jrmatherly/ai-kb/internal/logger"
	"github.com/jrmatherly/ai-kb/internal/types"
)

// pairingOrigin chooses the WebSocket host the extension will dial.
// A loopback API listener always wins, so a Lite page cannot point the
// extension at another host. A public page origin is kept only when its
// hostname is the request host; the port still comes from that page so a
// reverse proxy can preserve the external port.
func pairingOrigin(r *http.Request, page string) string {
	host := ""
	if r != nil {
		host = r.Host
	}
	if isLoopbackHostname(requestHostname(host)) {
		if origin, ok := loopbackPairingOrigin(host); ok {
			return origin
		}
		return ""
	}
	if origin, ok := pageOriginMatchingHost(page, host); ok {
		return origin
	}
	if isHTTPOrigin(page) {
		if origin := requestHTTPOrigin(r); origin != "" {
			return origin
		}
	}
	return page
}

func requestHostname(host string) string {
	if parsed, _, err := net.SplitHostPort(host); err == nil {
		return parsed
	}
	return host
}

func isLoopbackHostname(hostname string) bool {
	switch strings.ToLower(hostname) {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}

func loopbackPairingOrigin(host string) (string, bool) {
	host = strings.TrimSpace(host)
	if host == "" || strings.ContainsAny(host, " \t\r\n/@\\") {
		return "", false
	}
	hostname, port := host, ""
	if parsed, parsedPort, err := net.SplitHostPort(host); err == nil {
		hostname, port = parsed, parsedPort
	}
	if port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 || strconv.Itoa(n) != port {
			return "", false
		}
	}
	switch strings.ToLower(hostname) {
	case "localhost", "127.0.0.1":
		if port == "" {
			return "http://" + hostname, true
		}
		return "http://" + hostname + ":" + port, true
	case "::1":
		if port == "" {
			return "http://[::1]", true
		}
		return "http://[::1]:" + port, true
	default:
		return "", false
	}
}

func pageOriginMatchingHost(page, requestHost string) (string, bool) {
	u, err := url.Parse(page)
	if err != nil || u.User != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", false
	}
	reqName := requestHostname(requestHost)
	if reqName == "" || !strings.EqualFold(u.Hostname(), reqName) {
		return "", false
	}
	return u.Scheme + "://" + u.Host, true
}

func isHTTPOrigin(page string) bool {
	u, err := url.Parse(page)
	return err == nil && u.Host != "" && (u.Scheme == "http" || u.Scheme == "https")
}

func requestHTTPOrigin(r *http.Request) string {
	if r == nil {
		return ""
	}
	host := strings.TrimSpace(r.Host)
	if host == "" || strings.ContainsAny(host, " \t\r\n/@\\") {
		return ""
	}
	if _, err := url.ParseRequestURI("http://" + host + "/"); err != nil {
		return ""
	}
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return scheme + "://" + host
}

func browserSkillScope(ctx context.Context) browserskill.Scope {
	tenant, _ := types.TenantIDFromContext(ctx)
	user, _ := types.UserIDFromContext(ctx)
	return browserskill.Scope{Tenant: tenant, User: user}
}

// BrowserSkillConnection exposes task status and user controls for an owned conversation.
func (h *Handler) BrowserSkillConnection(c *gin.Context) {
	id := c.Param("session_id")
	if id == "" {
		id = c.Param("id")
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 40*time.Second)
	defer cancel()
	if _, err := h.sessionService.GetOwnedSession(ctx, id); err != nil {
		_ = c.Error(errors.NewNotFoundError("session not found"))
		return
	}
	c.Header("Cache-Control", "no-store")
	scope := browserSkillScope(ctx)
	if c.Request.Method == http.MethodGet {
		status, err := h.browserSkill.GetStatus(ctx, scope, id)
		if err != nil {
			_ = c.Error(browserSkillError(ctx, err, errors.NewServiceUnavailableError))
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "data": status})
		return
	}
	if !h.browserSkill.Enabled() {
		_ = c.Error(errors.NewServiceUnavailableError("local browser is unavailable"))
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
	var input struct {
		Action string `json:"action"`
	}
	if c.ShouldBindJSON(&input) != nil {
		_ = c.Error(errors.NewBadRequestError("invalid browser action"))
		return
	}
	var err error
	switch input.Action {
	case "preview":
		var frame []byte
		frame, err = h.browserSkill.Preview(ctx, scope, id)
		if err == nil {
			c.Data(http.StatusOK, "application/json", append(append([]byte(`{"success":true,"data":`), frame...), '}'))
			return
		}
	case "focus":
		err = h.browserSkill.Focus(ctx, scope, id)
	case "select", "start", "resume", "pause", "stop":
		err = h.browserSkill.Control(ctx, scope, id, input.Action)
	default:
		_ = c.Error(errors.NewBadRequestError("invalid browser action"))
		return
	}
	if err != nil {
		_ = c.Error(browserSkillError(ctx, err, errors.NewConflictError))
		return
	}
	status, err := h.browserSkill.GetStatus(ctx, scope, id)
	if err != nil {
		_ = c.Error(browserSkillError(ctx, err, errors.NewServiceUnavailableError))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": status})
}

// browserSkillError maps the manager's sentinels to statuses: a store failure is a
// fixed 503 (cause logged), an invalid grant 403, a daemon or owner-replica failure
// 502 with its text; every other message keeps the call site's status.
func browserSkillError(ctx context.Context, err error, newErr func(string) *errors.AppError) *errors.AppError {
	var rpcErr *browserskill.RPCError
	switch {
	case stderrors.Is(err, browserskill.ErrStore):
		logger.Errorf(ctx, "browser skill store failed: %v", err)
		return errors.NewServiceUnavailableError("browser connection status unavailable")
	case stderrors.Is(err, browserskill.ErrAuthorization):
		return errors.NewForbiddenError(err.Error())
	case stderrors.Is(err, browserskill.ErrUpstream), stderrors.As(err, &rpcErr):
		return errors.NewBadGatewayError(err.Error())
	}
	return newErr(err.Error())
}

// BrowserSkillExtension upgrades a device-authenticated extension connection.
func (h *Handler) BrowserSkillExtension(c *gin.Context) {
	h.browserSkill.ServeHTTP(c.Writer, c.Request)
}

// BrowserSkillAccount manages this member's shared browser connection independently
// of any conversation. Normal authentication and tenant membership apply upstream.
func (h *Handler) BrowserSkillAccount(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	scope := browserSkillScope(ctx)
	if scope.User == "" || scope.Tenant == 0 {
		_ = c.Error(errors.NewUnauthorizedError("login required"))
		return
	}
	c.Header("Cache-Control", "no-store")
	if c.Request.Method == http.MethodGet {
		status, err := h.browserSkill.Account(ctx, scope)
		if err != nil {
			_ = c.Error(errors.NewServiceUnavailableError("browser connection status unavailable"))
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "data": status})
		return
	}
	if !h.browserSkill.Enabled() {
		_ = c.Error(errors.NewServiceUnavailableError("local browser is unavailable"))
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
	var input struct {
		Action string `json:"action"`
		Origin string `json:"origin"`
	}
	if c.ShouldBindJSON(&input) != nil {
		_ = c.Error(errors.NewBadRequestError("invalid browser action"))
		return
	}
	switch input.Action {
	case "pair":
		link, err := h.browserSkill.Pair(ctx, scope, pairingOrigin(c.Request, input.Origin))
		if err != nil {
			_ = c.Error(browserSkillError(ctx, err, errors.NewConflictError))
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"pairing_link": link}})
	case "revoke":
		if err := h.browserSkill.Revoke(ctx, scope); err != nil {
			_ = c.Error(errors.NewServiceUnavailableError("could not revoke browser authorization"))
			return
		}
		status, err := h.browserSkill.Account(ctx, scope)
		if err != nil {
			_ = c.Error(errors.NewServiceUnavailableError("browser connection status unavailable"))
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "data": status})
	default:
		_ = c.Error(errors.NewBadRequestError("invalid account browser action"))
	}
}

// BrowserSkillAuthorize exchanges one-use pairing and renewable device credentials.
func (h *Handler) BrowserSkillAuthorize(c *gin.Context) {
	h.browserSkill.AuthorizeHTTP(c.Writer, c.Request)
}

// BrowserSkillInternal forwards signed requests to the owning node's browser gateway.
func (h *Handler) BrowserSkillInternal(c *gin.Context) {
	h.browserSkill.InternalHTTP(c.Writer, c.Request)
}

// BrowserSkillDownload serves the extension archive for installation.
func (h *Handler) BrowserSkillDownload(c *gin.Context) {
	h.browserSkill.DownloadExtension(c.Writer, c.Request)
}
