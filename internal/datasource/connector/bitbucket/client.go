// Package bitbucket is the Bitbucket Cloud repository data source connector.
package bitbucket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jrmatherly/ai-kb/internal/datasource"
	"github.com/jrmatherly/ai-kb/internal/logger"
)

const (
	apiBaseURL = "https://api.bitbucket.org/2.0"

	maxJSONResponseBytes int64 = 16 << 20
	maxRawFileBytes      int64 = 100 << 20

	// maxPaginationHops bounds one paginated walk; a broken server could
	// advertise a fresh next page forever.
	maxPaginationHops    = 10000
	pageLen              = 100
	listingDepth         = 50
	fallbackListingDepth = 10

	// rateLimitWaitMax is the longest rate-limit reset a client sits out, once,
	// before retrying; a longer one ends the run.
	rateLimitWaitMax = 30 * time.Minute
	// statusTimedOut is the status Bitbucket answers when a request ran too long.
	statusTimedOut = 555
)

var (
	errRateLimited = errors.New("bitbucket API rate limit exhausted")
	errTimedOut    = errors.New("bitbucket API timed out")
	errLFSRedirect = errors.New("bitbucket file is served by redirect")
	errTooLarge    = errors.New("response exceeds maximum size")
)

var fullHash = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)

type client struct {
	baseURL, username, token string
	http, rawHTTP            *http.Client
	// The limits are fields so tests can lower them; newClient always sets the
	// production values.
	jsonLimit, rawLimit int64
	waited              bool
}

type apiError struct {
	endpoint string
	status   int
}

func (e *apiError) Error() string {
	return fmt.Sprintf("bitbucket API %s: status %d", e.endpoint, e.status)
}

func (e *apiError) Is(target error) bool {
	switch target {
	case datasource.ErrInvalidCredentials:
		return e.status == http.StatusUnauthorized
	case datasource.ErrPermissionDenied:
		return e.status == http.StatusForbidden
	case datasource.ErrResourceNotFound:
		return e.status == http.StatusNotFound
	case errTimedOut:
		return e.status == statusTimedOut
	}
	return false
}

type rateLimitError struct {
	reset time.Duration
}

func (e *rateLimitError) Error() string {
	if e.reset <= 0 {
		return errRateLimited.Error() + ", resets at an unknown time"
	}
	return fmt.Sprintf("%s, resets in %s", errRateLimited, e.reset)
}

func (e *rateLimitError) Is(target error) bool { return target == errRateLimited }

type repository struct {
	UUID       string `json:"uuid"`
	FullName   string `json:"full_name"`
	MainBranch *struct {
		Name string `json:"name"`
	} `json:"mainbranch"`
	Links struct {
		HTML struct {
			Href string `json:"href"`
		} `json:"html"`
	} `json:"links"`
}

func (r repository) slug() string {
	_, slug, _ := strings.Cut(r.FullName, "/")
	return slug
}

type srcEntry struct {
	Type       string   `json:"type"`
	Path       string   `json:"path"`
	Size       int64    `json:"size"`
	Attributes []string `json:"attributes"`
}

func (e srcEntry) has(attribute string) bool { return slices.Contains(e.Attributes, attribute) }

type commitRef struct {
	Hash string `json:"hash"`
}

type diffEntry struct {
	Status string `json:"status"`
	Old    *struct {
		Path string `json:"path"`
	} `json:"old"`
	New *struct {
		Path string `json:"path"`
	} `json:"new"`
}

func newClient(baseURL, username, token string) (*client, error) {
	if strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("%w: Bitbucket access token is missing", datasource.ErrInvalidConfig)
	}
	raw := datasource.NewConnectorHTTPClient(30 * time.Second)
	// An LFS file redirects to a media host; following it would send the
	// token off the API host, so the caller sees errLFSRedirect instead.
	raw.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &client{
		baseURL:   strings.TrimRight(baseURL, "/"),
		username:  strings.TrimSpace(username),
		token:     token,
		http:      datasource.NewConnectorHTTPClient(30 * time.Second),
		rawHTTP:   raw,
		jsonLimit: maxJSONResponseBytes,
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
		return nil, fmt.Errorf("%w (%d bytes)", errTooLarge, limit)
	}
	return data, nil
}

// do GETs rawURL through hc. A 429 is sat out and retried once per client when
// its reset is short; any other 429 becomes a rateLimitError.
func (c *client) do(ctx context.Context, hc *http.Client, rawURL, accept string) (*http.Response, error) {
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, err
		}
		if c.username != "" {
			req.SetBasicAuth(c.username, c.token)
		} else {
			req.Header.Set("Authorization", "Bearer "+c.token)
		}
		if accept != "" {
			req.Header.Set("Accept", accept)
		}
		resp, err := hc.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusTooManyRequests {
			return resp, nil
		}
		_ = resp.Body.Close()
		limited := &rateLimitError{}
		if sec, err := strconv.ParseInt(resp.Header.Get("x-ratelimit-reset"), 10, 64); err == nil && sec > 0 {
			limited.reset = time.Duration(sec) * time.Second
		}
		if c.waited || limited.reset <= 0 || limited.reset > rateLimitWaitMax {
			return nil, limited
		}
		c.waited = true
		logger.Warnf(ctx, "[Bitbucket] rate limit exhausted; waiting %s before one retry", limited.reset)
		timer := time.NewTimer(limited.reset)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

func (c *client) fetch(ctx context.Context, rawURL string, out any) error {
	endpoint := strings.TrimPrefix(rawURL, c.baseURL)
	resp, err := c.do(ctx, c.http, rawURL, "application/json")
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &apiError{endpoint: endpoint, status: resp.StatusCode}
	}
	body, err := readCapped(resp.Body, c.jsonLimit)
	if err != nil {
		return fmt.Errorf("bitbucket API %s: %w", endpoint, err)
	}
	return json.Unmarshal(body, out)
}

func (c *client) pages(ctx context.Context, first string, each func(values json.RawMessage) error) error {
	seen := make(map[string]struct{})
	for next := first; next != ""; {
		if _, dup := seen[next]; dup {
			return fmt.Errorf("bitbucket pagination repeated page %q", next)
		}
		if len(seen) >= maxPaginationHops {
			logger.Warnf(ctx, "[Bitbucket] pagination exceeded %d pages; aborting", maxPaginationHops)
			return fmt.Errorf("bitbucket pagination exceeded %d pages", maxPaginationHops)
		}
		seen[next] = struct{}{}
		var page struct {
			Values json.RawMessage `json:"values"`
			Next   string          `json:"next"`
		}
		if err := c.fetch(ctx, next, &page); err != nil {
			return err
		}
		if err := each(page.Values); err != nil {
			return err
		}
		next = page.Next
		// The token goes with every request, so a next link may only point
		// back at the API it was issued for.
		if next != "" && !strings.HasPrefix(next, c.baseURL+"/") {
			return fmt.Errorf("bitbucket pagination link %q is outside the API base URL", next)
		}
	}
	return nil
}

func collect[T any](ctx context.Context, c *client, first string) ([]T, error) {
	var all []T
	err := c.pages(ctx, first, func(values json.RawMessage) error {
		var page []T
		if err := json.Unmarshal(values, &page); err != nil {
			return err
		}
		all = append(all, page...)
		return nil
	})
	return all, err
}

func repoPath(key string) string {
	workspace, slug, _ := strings.Cut(key, "/")
	return "/repositories/" + url.PathEscape(workspace) + "/" + url.PathEscape(slug)
}

func escapePath(p string) string {
	segments := strings.Split(p, "/")
	for i, s := range segments {
		segments[i] = url.PathEscape(s)
	}
	return strings.Join(segments, "/")
}

func (c *client) ping(ctx context.Context) error {
	return c.fetch(ctx, c.baseURL+"/user", &struct{}{})
}

func (c *client) workspaces(ctx context.Context) ([]string, error) {
	first := fmt.Sprintf("%s/user/workspaces?pagelen=%d&fields=values.slug,next", c.baseURL, pageLen)
	all, err := collect[struct {
		Slug string `json:"slug"`
	}](ctx, c, first)
	if err != nil {
		return nil, err
	}
	slugs := make([]string, len(all))
	for i, w := range all {
		slugs[i] = w.Slug
	}
	return slugs, nil
}

func (c *client) repositories(ctx context.Context, workspace string) ([]repository, error) {
	first := fmt.Sprintf("%s/repositories/%s?role=member&pagelen=%d&fields=values.full_name,values.uuid,"+
		"values.mainbranch.name,values.links.html.href,next", c.baseURL, url.PathEscape(workspace), pageLen)
	return collect[repository](ctx, c, first)
}

func (c *client) repository(ctx context.Context, key string) (*repository, error) {
	var r repository
	if err := c.fetch(ctx, c.baseURL+repoPath(key), &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// headCommit resolves ref as a branch, then a tag, then a full commit hash.
func (c *client) headCommit(ctx context.Context, key, ref string) (string, error) {
	var named struct {
		Target commitRef `json:"target"`
	}
	escaped := url.PathEscape(ref)
	endpoint := repoPath(key) + "/refs/branches/" + escaped
	err := c.fetch(ctx, c.baseURL+endpoint, &named)
	if errors.Is(err, datasource.ErrResourceNotFound) {
		endpoint = repoPath(key) + "/refs/tags/" + escaped
		err = c.fetch(ctx, c.baseURL+endpoint, &named)
	}
	hash := named.Target.Hash
	if err != nil {
		if !errors.Is(err, datasource.ErrResourceNotFound) || !fullHash.MatchString(ref) {
			return "", err
		}
		var commit commitRef
		endpoint = repoPath(key) + "/commit/" + escaped
		if err := c.fetch(ctx, c.baseURL+endpoint, &commit); err != nil {
			return "", err
		}
		hash = commit.Hash
	}
	if hash == "" {
		return "", fmt.Errorf("bitbucket API %s: no commit hash in the response", endpoint)
	}
	return hash, nil
}

func (c *client) commitExists(ctx context.Context, key, hash string) (bool, error) {
	var commit commitRef
	err := c.fetch(ctx, c.baseURL+repoPath(key)+"/commit/"+url.PathEscape(hash), &commit)
	if errors.Is(err, datasource.ErrResourceNotFound) {
		return false, nil
	}
	return err == nil, err
}

func (c *client) listing(ctx context.Context, key, commit, dir string, depth int) ([]srcEntry, error) {
	if dir != "" {
		dir = escapePath(dir) + "/"
	}
	first := fmt.Sprintf("%s%s/src/%s/%s?max_depth=%d&pagelen=%d"+
		"&fields=values.path,values.type,values.size,values.attributes,next",
		c.baseURL, repoPath(key), url.PathEscape(commit), dir, depth, pageLen)
	return collect[srcEntry](ctx, c, first)
}

func (c *client) diffstat(ctx context.Context, key, newHead, oldHead string) ([]diffEntry, error) {
	first := fmt.Sprintf("%s%s/diffstat/%s..%s?topic=false&renames=false&pagelen=%d"+
		"&fields=values.status,values.old.path,values.new.path,next",
		c.baseURL, repoPath(key), url.PathEscape(newHead), url.PathEscape(oldHead), pageLen)
	return collect[diffEntry](ctx, c, first)
}

func (c *client) raw(ctx context.Context, key, commit, path string) ([]byte, error) {
	endpoint := repoPath(key) + "/src/" + url.PathEscape(commit) + "/" + escapePath(path)
	resp, err := c.do(ctx, c.rawHTTP, c.baseURL+endpoint, "")
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return nil, fmt.Errorf("%w: %s", errLFSRedirect, endpoint)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &apiError{endpoint: endpoint, status: resp.StatusCode}
	}
	body, err := readCapped(resp.Body, c.rawLimit)
	if err != nil {
		return nil, fmt.Errorf("bitbucket API %s: %w", endpoint, err)
	}
	return body, nil
}
