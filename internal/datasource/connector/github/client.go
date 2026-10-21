// Package github is the GitHub repository data source connector.
package github

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
	"time"

	"github.com/jrmatherly/ai-kb/internal/datasource"
	"github.com/jrmatherly/ai-kb/internal/logger"
)

const (
	defaultBaseURL = "https://api.github.com"
	apiVersion     = "2022-11-28"
	jsonMediaType  = "application/vnd.github+json"
	rawMediaType   = "application/vnd.github.raw+json"

	// maxJSONResponseBytes bounds user, repository and commit JSON.
	maxJSONResponseBytes int64 = 16 << 20
	// maxTreeResponseBytes bounds a recursive tree, which lists every entry of
	// a repository (up to the API's own 100,000-entry truncation point).
	maxTreeResponseBytes int64 = 64 << 20
	// maxRawFileBytes is the API's own limit for a blob.
	maxRawFileBytes int64 = 100 << 20
)

// maxPaginationHops bounds a single paginated walk. The next page comes from
// the Link header, so a broken server or proxy could advertise pages forever.
const maxPaginationHops = 10000

var errRateLimited = errors.New("github API rate limit exhausted")

type client struct {
	baseURL, token string
	http           *http.Client
	// The limits are fields so tests can lower them; newClient always sets the
	// production values.
	jsonLimit, treeLimit, rawLimit int64
}

type apiError struct {
	endpoint string
	status   int
}

func (e *apiError) Error() string {
	return fmt.Sprintf("github API %s: status %d", e.endpoint, e.status)
}

// Is maps 401 and 403 to the datasource sentinels. A rate-limited 403 never
// reaches here: it becomes errRateLimited instead.
func (e *apiError) Is(target error) bool {
	switch target {
	case datasource.ErrInvalidCredentials:
		return e.status == http.StatusUnauthorized
	case datasource.ErrPermissionDenied:
		return e.status == http.StatusForbidden
	}
	return false
}

type repository struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	HTMLURL       string `json:"html_url"`
	DefaultBranch string `json:"default_branch"`
}

type treeEntry struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
	Type string `json:"type"`
	SHA  string `json:"sha"`
	Size int64  `json:"size"`
}

type tree struct {
	SHA       string      `json:"sha"`
	Entries   []treeEntry `json:"tree"`
	Truncated bool        `json:"truncated"`
}

// normalizeBaseURL maps a user-supplied host or URL to the REST API root:
// github.com and its aliases to api.github.com, a *.ghe.com data-residency
// host to api.<host>, and anything else to an Enterprise Server /api/v3 root.
func normalizeBaseURL(value string) string {
	value = strings.TrimRight(strings.TrimSpace(value), "/")
	if value == "" {
		return defaultBaseURL
	}
	scheme := "https"
	if i := strings.Index(value, "://"); i >= 0 {
		scheme, value = value[:i], value[i+3:]
	}
	host, path, _ := strings.Cut(value, "/")
	lower := strings.ToLower(host)
	switch {
	case lower == "github.com" || lower == "www.github.com" || lower == "api.github.com":
		return defaultBaseURL
	case strings.HasSuffix(lower, ".ghe.com"):
		return "https://api." + strings.TrimPrefix(lower, "api.")
	}
	path = strings.TrimRight(strings.TrimSuffix("/"+path, "/api/v3"), "/")
	return scheme + "://" + host + path + "/api/v3"
}

func newClient(baseURL, token string) (*client, error) {
	if strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("GitHub access token is missing")
	}
	if err := datasource.ValidateConnectorBaseURL(baseURL); err != nil {
		return nil, err
	}
	return &client{
		baseURL:   normalizeBaseURL(baseURL),
		token:     token,
		http:      datasource.NewConnectorHTTPClient(30 * time.Second),
		jsonLimit: maxJSONResponseBytes,
		treeLimit: maxTreeResponseBytes,
		rawLimit:  maxRawFileBytes,
	}, nil
}

// readCapped reads a response body, refusing anything larger than limit instead
// of buffering it. A truncated body would be indexed as if it were the whole
// document.
func readCapped(body io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("response exceeds maximum size (%d bytes)", limit)
	}
	return data, nil
}

// fetch GETs an absolute URL and returns its capped body and headers.
func (c *client) fetch(ctx context.Context, rawURL, accept string, limit int64) ([]byte, http.Header, error) {
	endpoint := strings.TrimPrefix(rawURL, c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", accept)
	req.Header.Set("X-GitHub-Api-Version", apiVersion)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if err := statusError(endpoint, resp); err != nil {
		return nil, nil, err
	}
	body, err := readCapped(resp.Body, limit)
	if err != nil {
		return nil, nil, fmt.Errorf("github API %s: %w", endpoint, err)
	}
	return body, resp.Header, nil
}

func statusError(endpoint string, resp *http.Response) error {
	status := resp.StatusCode
	if status >= 200 && status < 300 {
		return nil
	}
	if status == http.StatusTooManyRequests || (status == http.StatusForbidden &&
		(resp.Header.Get("Retry-After") != "" || resp.Header.Get("X-RateLimit-Remaining") == "0")) {
		reset := "unknown"
		if sec, err := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64); err == nil {
			reset = time.Unix(sec, 0).UTC().Format(time.RFC3339)
		}
		return fmt.Errorf("%w until %s", errRateLimited, reset)
	}
	return &apiError{endpoint: endpoint, status: status}
}

func (c *client) getCapped(ctx context.Context, endpoint string, out any, limit int64) error {
	body, _, err := c.fetch(ctx, c.baseURL+endpoint, jsonMediaType, limit)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, out)
}

// repoPath escapes each segment of an "owner/repo" key.
func repoPath(key string) string {
	owner, repo, _ := strings.Cut(key, "/")
	return "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo)
}

func (c *client) ping(ctx context.Context) error {
	var user struct {
		ID int64 `json:"id"`
	}
	return c.getCapped(ctx, "/user", &user, c.jsonLimit)
}

func (c *client) repositories(ctx context.Context) ([]repository, error) {
	next := c.baseURL + "/user/repos?per_page=100&sort=full_name&affiliation=owner,collaborator,organization_member"
	var all []repository
	seen := make(map[string]struct{})
	for next != "" {
		if _, dup := seen[next]; dup {
			return nil, fmt.Errorf("github repository pagination repeated page %q", next)
		}
		if len(seen) >= maxPaginationHops {
			logger.Warnf(ctx, "[GitHub] repository pagination exceeded %d pages; aborting", maxPaginationHops)
			return nil, fmt.Errorf("github repository pagination exceeded %d pages", maxPaginationHops)
		}
		seen[next] = struct{}{}
		body, header, err := c.fetch(ctx, next, jsonMediaType, c.jsonLimit)
		if err != nil {
			return nil, err
		}
		var page []repository
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, err
		}
		all = append(all, page...)
		next = nextLink(header.Get("Link"))
		// The token goes with every request, so a next link may only point
		// back at the API it was issued for.
		if next != "" && !strings.HasPrefix(next, c.baseURL+"/") {
			return nil, fmt.Errorf("github repository pagination link %q is outside the API base URL", next)
		}
	}
	return all, nil
}

// nextLink returns the URL of the rel="next" segment of a Link header. It
// scans <...> targets rather than splitting on commas, which a URL may contain.
func nextLink(header string) string {
	for {
		start := strings.IndexByte(header, '<')
		end := strings.IndexByte(header, '>')
		if start < 0 || end < start {
			return ""
		}
		target, rest := header[start+1:end], header[end+1:]
		params := rest
		if i := strings.IndexByte(rest, '<'); i >= 0 {
			params = rest[:i]
		}
		if strings.Contains(params, `rel="next"`) {
			return target
		}
		header = rest
	}
}

func (c *client) repository(ctx context.Context, key string) (*repository, error) {
	var r repository
	if err := c.getCapped(ctx, repoPath(key), &r, c.jsonLimit); err != nil {
		return nil, err
	}
	return &r, nil
}

func (c *client) commitSHA(ctx context.Context, key, ref string) (string, error) {
	var v struct {
		SHA string `json:"sha"`
	}
	err := c.getCapped(ctx, repoPath(key)+"/commits/"+url.PathEscape(ref), &v, c.jsonLimit)
	return v.SHA, err
}

func (c *client) tree(ctx context.Context, key, sha string, recursive bool) (*tree, error) {
	endpoint := repoPath(key) + "/git/trees/" + url.PathEscape(sha)
	if recursive {
		endpoint += "?recursive=1"
	}
	var t tree
	if err := c.getCapped(ctx, endpoint, &t, c.treeLimit); err != nil {
		return nil, err
	}
	return &t, nil
}

func (c *client) blob(ctx context.Context, key, sha string) ([]byte, error) {
	body, _, err := c.fetch(ctx, c.baseURL+repoPath(key)+"/git/blobs/"+url.PathEscape(sha), rawMediaType, c.rawLimit)
	return body, err
}
