package msgraph

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jrmatherly/ai-kb/internal/datasource"
	"github.com/jrmatherly/ai-kb/internal/logger"
	"github.com/jrmatherly/ai-kb/internal/utils"
)

const (
	graphBaseURL = "https://graph.microsoft.com/v1.0"
	loginBaseURL = "https://login.microsoftonline.com"
	graphScope   = "https://graph.microsoft.com/.default"

	apiTimeout      = 30 * time.Second
	downloadTimeout = 120 * time.Second

	maxJSONResponseBytes int64 = 16 << 20
	maxPaginationHops          = 10000
	childrenPageSize           = 200
	tokenSkew                  = 5 * time.Minute
	// rateLimitWaitMax is the longest Retry-After a client sits out, once, before
	// retrying; a longer one, or a second one, ends the run.
	rateLimitWaitMax = 30 * time.Minute
	// missingRetryAfterBackoff is the backoff for a 429 or 503 without Retry-After:
	// 2, 4 and 8 seconds, then the run ends.
	missingRetryAfterBackoff = 2 * time.Second
	missingRetryAfterTries   = 3

	deltaSelect = "id,name,size,file,folder,root,deleted,parentReference,eTag,lastModifiedDateTime,webUrl," +
		"package,remoteItem,malware"
)

var (
	errRateLimited      = errors.New("microsoft graph rate limit exhausted")
	errTooLarge         = errors.New("response exceeds maximum size")
	errEmptyFile        = errors.New("file is empty")
	errSourceChanged    = errors.New("file size changed between listing and download")
	errInsecureDownload = errors.New("download location is not an https URL")
	errNoRedirect       = errors.New("content endpoint did not redirect to a download URL")
)

type client struct {
	graphBaseURL, loginBaseURL string
	cfg                        *config
	// http carries the bearer token; redirectHTTP is the same client that stops at
	// the /content 302; files fetches pre-authenticated URLs without any token.
	http, redirectHTTP, files *http.Client
	jsonLimit                 int64

	tokenMu    sync.Mutex
	token      string
	tokenExpAt time.Time
	waited     bool
}

type apiError struct {
	endpoint string
	status   int
}

func (e *apiError) Error() string {
	return fmt.Sprintf("microsoft graph %s: status %d", e.endpoint, e.status)
}

func (e *apiError) Is(target error) bool {
	switch target {
	case datasource.ErrInvalidCredentials:
		return e.status == http.StatusUnauthorized
	case datasource.ErrPermissionDenied:
		return e.status == http.StatusForbidden
	case datasource.ErrResourceNotFound:
		return e.status == http.StatusNotFound
	}
	return false
}

type rateLimitError struct{ retryAfter time.Duration }

func (e *rateLimitError) Error() string {
	return fmt.Sprintf("%s, retry after %s", errRateLimited, e.retryAfter)
}
func (e *rateLimitError) Is(target error) bool { return target == errRateLimited }

// resyncError is a 410 on a delta link: the enumeration restarts from Location.
// The link carries the delta token, so the message never includes it.
type resyncError struct{ Location string }

func (e *resyncError) Error() string { return "microsoft graph delta token expired; resync required" }

type siteInfo struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	WebURL      string `json:"webUrl"`
}

type driveInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	WebURL string `json:"webUrl"`
}

type driveItem struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Size         int64     `json:"size"`
	ETag         string    `json:"eTag"`
	WebURL       string    `json:"webUrl"`
	LastModified time.Time `json:"lastModifiedDateTime"`
	File         *struct {
		MimeType string `json:"mimeType"`
		Hashes   *struct {
			QuickXorHash string `json:"quickXorHash"`
			Sha1Hash     string `json:"sha1Hash"`
			Sha256Hash   string `json:"sha256Hash"`
		} `json:"hashes"`
	} `json:"file"`
	Folder          *struct{} `json:"folder"`
	Root            *struct{} `json:"root"`
	Deleted         *struct{} `json:"deleted"`
	Package         *struct{} `json:"package"`
	RemoteItem      *struct{} `json:"remoteItem"`
	Malware         *struct{} `json:"malware"`
	ParentReference *struct {
		DriveID string `json:"driveId"`
		ID      string `json:"id"`
	} `json:"parentReference"`
}

func (it driveItem) isFile() bool {
	return it.File != nil && it.Deleted == nil && it.RemoteItem == nil && it.Package == nil
}

type deltaPage struct {
	Items []driveItem `json:"value"`
	Next  string      `json:"@odata.nextLink"`
	Delta string      `json:"@odata.deltaLink"`
}

func newClient(graphBase, loginBase string, cfg *config) *client {
	redirect := datasource.NewConnectorHTTPClient(apiTimeout)
	// The /content answer is a 302 to a pre-authenticated host; the caller
	// resolves it so the bearer token never leaves the Graph host.
	redirect.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &client{
		graphBaseURL: strings.TrimRight(graphBase, "/"),
		loginBaseURL: strings.TrimRight(loginBase, "/"),
		cfg:          cfg,
		http:         datasource.NewConnectorHTTPClient(apiTimeout),
		redirectHTTP: redirect,
		files:        datasource.NewConnectorHTTPClient(downloadTimeout),
		jsonLimit:    maxJSONResponseBytes,
	}
}

// readCapped reads a body, refusing anything larger than limit instead of
// buffering it; a truncated body would be indexed as if it were the whole document.
func readCapped(body io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%w (%d bytes)", errTooLarge, limit)
	}
	return data, nil
}

// accessToken returns the cached app-only token or mints one with the client
// credentials grant. The token endpoint's 400 and 401 mean the credentials
// are wrong; anything else is a transport or service failure.
func (c *client) accessToken(ctx context.Context) (string, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()
	if c.token != "" && time.Now().Before(c.tokenExpAt) {
		return c.token, nil
	}
	form := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {c.cfg.ClientID},
		"client_secret": {c.cfg.ClientSecret},
		"scope":         {graphScope},
	}
	endpoint := c.loginBaseURL + "/" + url.PathEscape(c.cfg.TenantID) + "/oauth2/v2.0/token"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("microsoft identity token request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := readCapped(resp.Body, c.jsonLimit)
	if err != nil {
		return "", err
	}
	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnauthorized {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(body, &e)
		return "", fmt.Errorf("%w: microsoft identity rejected the app credentials (%s)",
			datasource.ErrInvalidCredentials, e.Error)
	}
	if resp.StatusCode != http.StatusOK {
		return "", &apiError{endpoint: "token", status: resp.StatusCode}
	}
	var tok struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tok); err != nil || tok.AccessToken == "" {
		return "", fmt.Errorf("microsoft identity token response: %w", errors.Join(err, errors.New("no access_token")))
	}
	c.token = tok.AccessToken
	ttl := time.Duration(tok.ExpiresIn) * time.Second
	if ttl > tokenSkew {
		ttl -= tokenSkew
	} else if ttl <= 0 {
		ttl = tokenSkew // a missing expires_in is not a reason to mint a token per call
	}
	c.tokenExpAt = time.Now().Add(ttl)
	return c.token, nil
}

func (c *client) invalidateToken(rejected string) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()
	if c.token == rejected {
		c.token = ""
	}
}

// do GETs rawURL with the bearer token through hc. A 401 re-mints the token
// once; a 429 or 503 is sat out within the limits above, or ends the call.
func (c *client) do(ctx context.Context, hc *http.Client, rawURL string) (*http.Response, error) {
	reminted := false
	backoffs := 0
	for {
		token, err := c.accessToken(ctx)
		if err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")
		resp, err := hc.Do(req)
		if err != nil {
			// *url.Error prints the full URL, and its query carries delta and paging tokens.
			cause := errors.Unwrap(err)
			if cause == nil {
				cause = err
			}
			return nil, fmt.Errorf("microsoft graph %s: %w", endpointLabel(rawURL), cause)
		}
		switch resp.StatusCode {
		case http.StatusUnauthorized:
			if reminted {
				return resp, nil
			}
			_ = resp.Body.Close()
			c.invalidateToken(token)
			reminted = true
			continue
		case http.StatusTooManyRequests, http.StatusServiceUnavailable:
			_ = resp.Body.Close()
			limited := &rateLimitError{}
			if sec, err := strconv.ParseInt(resp.Header.Get("Retry-After"), 10, 64); err == nil && sec > 0 {
				limited.retryAfter = time.Duration(sec) * time.Second
			}
			var wait time.Duration
			switch {
			case limited.retryAfter <= 0 && backoffs < missingRetryAfterTries:
				wait = missingRetryAfterBackoff << backoffs
				backoffs++
			case limited.retryAfter > 0 && !c.waited && limited.retryAfter <= rateLimitWaitMax:
				wait = limited.retryAfter
				c.waited = true
			default:
				return nil, limited
			}
			logger.Warnf(ctx, "[MSGraph] throttled; waiting %s before retrying", wait)
			if err := sleepCtx(ctx, wait); err != nil {
				return nil, err
			}
			continue
		}
		return resp, nil
	}
}

// endpointLabel names a Graph call for an error without its query string,
// which carries delta and paging tokens.
func endpointLabel(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "request"
	}
	return u.Path
}

func (c *client) getJSON(ctx context.Context, rawURL string, out any) error {
	endpoint := endpointLabel(rawURL)
	resp, err := c.do(ctx, c.http, rawURL)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusGone {
		loc := resp.Header.Get("Location")
		if loc == "" || !c.onGraphHost(loc) {
			return &apiError{endpoint: endpoint, status: resp.StatusCode}
		}
		return &resyncError{Location: loc}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &apiError{endpoint: endpoint, status: resp.StatusCode}
	}
	body, err := readCapped(resp.Body, c.jsonLimit)
	if err != nil {
		return fmt.Errorf("microsoft graph %s: %w", endpoint, err)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("microsoft graph %s: %w: %w", endpoint, datasource.ErrFetchFailed, err)
	}
	return nil
}

func (c *client) onGraphHost(link string) bool { return strings.HasPrefix(link, c.graphBaseURL+"/") }

// pages walks a Graph collection by its nextLink, refusing a repeated link and
// any link off the Graph host, since the bearer token goes with every request.
func (c *client) pages(ctx context.Context, first string, each func(values json.RawMessage) error) error {
	seen := map[string]struct{}{}
	for next := first; next != ""; {
		if _, dup := seen[next]; dup {
			return fmt.Errorf("microsoft graph pagination repeated a page link")
		}
		if len(seen) >= maxPaginationHops {
			return fmt.Errorf("microsoft graph pagination exceeded %d pages", maxPaginationHops)
		}
		seen[next] = struct{}{}
		var page struct {
			Value json.RawMessage `json:"value"`
			Next  string          `json:"@odata.nextLink"`
		}
		if err := c.getJSON(ctx, next, &page); err != nil {
			return err
		}
		if err := each(page.Value); err != nil {
			return err
		}
		next = page.Next
		if next != "" && !c.onGraphHost(next) {
			return fmt.Errorf("microsoft graph pagination link is outside the Graph host")
		}
	}
	return nil
}

func collect[T any](ctx context.Context, c *client, first string) ([]T, error) {
	var all []T
	err := c.pages(ctx, first, func(values json.RawMessage) error {
		var page []T
		if err := json.Unmarshal(values, &page); err != nil {
			return fmt.Errorf("%w: %w", datasource.ErrFetchFailed, err)
		}
		all = append(all, page...)
		return nil
	})
	return all, err
}

func escapeSegments(p string) string {
	parts := strings.Split(p, "/")
	for i, s := range parts {
		parts[i] = url.PathEscape(s)
	}
	return strings.Join(parts, "/")
}

func (c *client) site(ctx context.Context, host, sitePath string) (*siteInfo, error) {
	endpoint := c.graphBaseURL + "/sites/" + url.PathEscape(host)
	if sitePath != "" {
		endpoint += ":/" + escapeSegments(sitePath)
	}
	var s siteInfo
	if err := c.getJSON(ctx, endpoint+"?$select=id,displayName,webUrl", &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func (c *client) siteDrives(ctx context.Context, siteID string) ([]driveInfo, error) {
	return collect[driveInfo](ctx, c, c.graphBaseURL+"/sites/"+url.PathEscape(siteID)+"/drives?$select=id,name,webUrl")
}

func (c *client) userDrive(ctx context.Context, user string) (*driveInfo, error) {
	var d driveInfo
	if err := c.getJSON(
		ctx,
		c.graphBaseURL+"/users/"+url.PathEscape(user)+"/drive?$select=id,name,webUrl",
		&d,
	); err != nil {
		return nil, err
	}
	return &d, nil
}

func (c *client) drive(ctx context.Context, driveID string) (*driveInfo, error) {
	var d driveInfo
	if err := c.getJSON(
		ctx,
		c.graphBaseURL+"/drives/"+url.PathEscape(driveID)+"?$select=id,name,webUrl",
		&d,
	); err != nil {
		return nil, err
	}
	return &d, nil
}

func (c *client) itemPath(driveID, itemID string) string {
	base := c.graphBaseURL + "/drives/" + url.PathEscape(driveID)
	if itemID == "" {
		return base + "/root"
	}
	return base + "/items/" + url.PathEscape(itemID)
}

func (c *client) item(ctx context.Context, driveID, itemID string) (*driveItem, error) {
	var it driveItem
	if err := c.getJSON(ctx, c.itemPath(driveID, itemID)+"?$select="+deltaSelect, &it); err != nil {
		return nil, err
	}
	return &it, nil
}

// childFolders lists the direct child folders of a drive root or folder.
func (c *client) childFolders(ctx context.Context, driveID, itemID string) ([]driveItem, error) {
	first := fmt.Sprintf(
		"%s/children?$select=id,name,folder,deleted,remoteItem,lastModifiedDateTime,webUrl&$top=%d",
		c.itemPath(driveID, itemID), childrenPageSize,
	)
	all, err := collect[driveItem](ctx, c, first)
	if err != nil {
		return nil, err
	}
	folders := all[:0]
	for _, it := range all {
		if it.Folder != nil && it.Deleted == nil && it.RemoteItem == nil {
			folders = append(folders, it)
		}
	}
	return folders, nil
}

func (c *client) deltaStart(driveID, itemID string) string {
	return c.itemPath(driveID, itemID) + "/delta?$select=" + deltaSelect
}

// deltaPage fetches one page of a delta enumeration. A 410 is a *resyncError
// carrying the fresh start link.
func (c *client) deltaPage(ctx context.Context, link string) (*deltaPage, error) {
	if !c.onGraphHost(link) {
		return nil, fmt.Errorf("microsoft graph delta link is outside the Graph host")
	}
	var page deltaPage
	if err := c.getJSON(ctx, link, &page); err != nil {
		return nil, err
	}
	if page.Next != "" && !c.onGraphHost(page.Next) || page.Delta != "" && !c.onGraphHost(page.Delta) {
		return nil, fmt.Errorf("microsoft graph delta answered a link outside the Graph host")
	}
	return &page, nil
}

// download resolves the /content redirect itself, validates the pre-authenticated
// URL and fetches it without the bearer token through a capped reader. The body
// must be exactly expectedSize bytes and at most limit bytes.
func (c *client) download(ctx context.Context, driveID, itemID string, expectedSize, limit int64) ([]byte, error) {
	endpoint := c.itemPath(driveID, itemID) + "/content"
	resp, err := c.do(ctx, c.redirectHTTP, endpoint)
	if err != nil {
		return nil, err
	}
	_ = resp.Body.Close()
	if resp.StatusCode < 300 || resp.StatusCode >= 400 {
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil, errNoRedirect
		}
		return nil, &apiError{endpoint: endpointLabel(endpoint), status: resp.StatusCode}
	}
	location := resp.Header.Get("Location")
	u, err := url.Parse(location)
	// Production download hosts are https; the only http host accepted is the
	// test fake serving as the Graph host itself.
	fakeHost := strings.TrimPrefix(c.graphBaseURL, "http://")
	plainFake := u != nil && u.Scheme == "http" && strings.HasPrefix(c.graphBaseURL, "http://") && u.Host == fakeHost
	if err != nil || u.Hostname() == "" || u.User != nil || (u.Scheme != "https" && !plainFake) {
		return nil, errInsecureDownload
	}
	if err := utils.ValidateURLForSSRF(location); err != nil {
		return nil, fmt.Errorf("%w: %w", datasource.ErrFetchFailed, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, location, nil)
	if err != nil {
		return nil, err
	}
	file, err := c.files.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: download failed", datasource.ErrFetchFailed)
	}
	defer func() { _ = file.Body.Close() }()
	if file.StatusCode < 200 || file.StatusCode >= 300 {
		return nil, &apiError{endpoint: "download", status: file.StatusCode}
	}
	body, err := readCapped(file.Body, limit)
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, errEmptyFile
	}
	if int64(len(body)) != expectedSize {
		return nil, errSourceChanged
	}
	return body, nil
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
