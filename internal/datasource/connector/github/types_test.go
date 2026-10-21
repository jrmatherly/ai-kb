package github

import (
	"testing"

	"github.com/jrmatherly/ai-kb/internal/datasource"
	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/stretchr/testify/require"
)

func settings(repository string) *types.DataSourceConfig {
	return &types.DataSourceConfig{Settings: map[string]interface{}{
		"repositories": []interface{}{map[string]interface{}{"repository": repository}},
	}}
}

func TestParseConfigNormalisesRepository(t *testing.T) {
	for in, want := range map[string]string{
		"owner/repo":     "owner/repo",
		"/OWNER/Repo/":   "owner/repo",
		"owner/repo.git": "owner/repo",
	} {
		cfg, err := parseConfig(settings(in))
		require.NoError(t, err, in)
		require.Equal(t, want, cfg.Repositories[0].Repository)
	}
	for _, in := range []string{"https://github.com/owner/repo", "owner", "owner/repo/extra", ""} {
		_, err := parseConfig(settings(in))
		require.ErrorIs(t, err, datasource.ErrInvalidConfig, in)
	}
	_, err := parseConfig(settings("https://github.com/owner/repo"))
	require.EqualError(t, err, datasource.ErrInvalidConfig.Error()+": repository must be owner/repo, not a URL")
}

func TestParseConfigCollapsesDirectories(t *testing.T) {
	cfg, err := parseConfig(&types.DataSourceConfig{Settings: map[string]interface{}{
		"repositories": []interface{}{map[string]interface{}{
			"repository": "owner/repo", "ref": "main",
			"paths": []interface{}{"/docs/", "docs/api", "src"},
		}},
	}})
	require.NoError(t, err)
	want := []repositorySelection{{Repository: "owner/repo", Ref: "main", Paths: []string{"docs", "src"}}}
	require.Equal(t, want, cfg.Repositories)
}

func TestParseConfigRejectsDuplicateRepositoryIgnoringCase(t *testing.T) {
	_, err := parseConfig(&types.DataSourceConfig{Settings: map[string]interface{}{
		"repositories": []interface{}{
			map[string]interface{}{"repository": "owner/repo"},
			map[string]interface{}{"repository": "Owner/Repo"},
		},
	}})
	require.ErrorIs(t, err, datasource.ErrInvalidConfig)
	require.EqualError(t, err, datasource.ErrInvalidConfig.Error()+": repository must be unique and non-empty")
}

func TestParseConfigRequiresRepositories(t *testing.T) {
	_, err := parseConfig(&types.DataSourceConfig{Settings: map[string]interface{}{}})
	require.ErrorIs(t, err, datasource.ErrInvalidConfig)
	require.EqualError(t, err, datasource.ErrInvalidConfig.Error()+": settings.repositories is required")

	_, err = parseConfig(&types.DataSourceConfig{Settings: map[string]interface{}{"repositories": []interface{}{}}})
	require.ErrorIs(t, err, datasource.ErrInvalidConfig)
	require.EqualError(t, err, datasource.ErrInvalidConfig.Error()+": at least one repository is required")
}
