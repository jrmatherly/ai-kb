package github

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/jrmatherly/ai-kb/internal/datasource"
	"github.com/jrmatherly/ai-kb/internal/datasource/connector/repofile"
	"github.com/jrmatherly/ai-kb/internal/types"
)

type repositorySelection struct {
	Repository string   `json:"repository"`
	Ref        string   `json:"ref"`
	Paths      []string `json:"paths"`
}

type config struct {
	Repositories []repositorySelection `json:"repositories"`
}

func parseConfig(ds *types.DataSourceConfig) (*config, error) {
	if ds == nil {
		return nil, datasource.ErrInvalidConfig
	}
	raw, ok := ds.Settings["repositories"]
	if !ok {
		return nil, fmt.Errorf("%w: settings.repositories is required", datasource.ErrInvalidConfig)
	}
	items, ok := raw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("%w: settings.repositories must be an array", datasource.ErrInvalidConfig)
	}
	out := &config{Repositories: make([]repositorySelection, 0, len(items))}
	seen := map[string]bool{}
	for _, rawRepository := range items {
		m, ok := rawRepository.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("%w: invalid repository selection", datasource.ErrInvalidConfig)
		}
		value, _ := m["repository"].(string)
		if strings.TrimSpace(value) == "" {
			return nil, fmt.Errorf("%w: repository must be unique and non-empty", datasource.ErrInvalidConfig)
		}
		repository, err := normalizeRepository(value)
		if err != nil {
			return nil, err
		}
		if seen[repository] {
			return nil, fmt.Errorf("%w: repository must be unique and non-empty", datasource.ErrInvalidConfig)
		}
		seen[repository] = true
		ref, _ := m["ref"].(string)
		r := repositorySelection{Repository: repository, Ref: strings.TrimSpace(ref)}
		if rawPaths, exists := m["paths"]; exists {
			values, ok := rawPaths.([]interface{})
			if !ok {
				return nil, fmt.Errorf("%w: paths must be an array", datasource.ErrInvalidConfig)
			}
			for _, value := range values {
				s, ok := value.(string)
				if !ok {
					return nil, fmt.Errorf("%w: path must be a string", datasource.ErrInvalidConfig)
				}
				normalized, err := repofile.NormalizePath(s)
				if err != nil {
					return nil, err
				}
				r.Paths = append(r.Paths, normalized)
			}
		}
		r.Paths = repofile.CollapsePaths(r.Paths)
		out.Repositories = append(out.Repositories, r)
	}
	if len(out.Repositories) == 0 {
		return nil, fmt.Errorf("%w: at least one repository is required", datasource.ErrInvalidConfig)
	}
	return out, nil
}

// repositoryName matches one lowercased owner or repository name.
var repositoryName = regexp.MustCompile(`^[a-z0-9._-]+$`)

// GitHub owner and repository names are case-insensitive, so the lowercased
// form is the identity used for uniqueness.
func normalizeRepository(value string) (string, error) {
	v := strings.TrimSpace(value)
	if strings.Contains(v, "://") {
		return "", fmt.Errorf("%w: repository must be owner/repo, not a URL", datasource.ErrInvalidConfig)
	}
	v = strings.TrimSuffix(strings.Trim(strings.ToLower(v), "/"), ".git")
	owner, repo, ok := strings.Cut(v, "/")
	if !ok || !repositoryName.MatchString(owner) || !repositoryName.MatchString(repo) {
		return "", fmt.Errorf("%w: repository must be owner/repo", datasource.ErrInvalidConfig)
	}
	return v, nil
}
