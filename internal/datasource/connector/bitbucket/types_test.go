package bitbucket

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
		"Atlassian/AUI": "atlassian/aui",
		"/ws/repo/":     "ws/repo",
		"ws/repo.git":   "ws/repo",
	} {
		cfg, err := parseConfig(settings(in))
		require.NoError(t, err, in)
		require.Equal(t, want, cfg.Repositories[0].Repository)
	}
	for _, in := range []string{"https://bitbucket.org/ws/repo", "ws", "ws/repo/extra", "{uuid}/repo", ""} {
		_, err := parseConfig(settings(in))
		require.ErrorIs(t, err, datasource.ErrInvalidConfig, in)
	}
	_, err := parseConfig(settings("https://bitbucket.org/ws/repo"))
	require.EqualError(t, err, datasource.ErrInvalidConfig.Error()+": repository must be workspace/slug, not a URL")
	_, err = parseConfig(settings(""))
	require.EqualError(t, err, datasource.ErrInvalidConfig.Error()+": repository is required")
	_, err = parseConfig(settings("ws/repo/extra"))
	require.EqualError(t, err, datasource.ErrInvalidConfig.Error()+": repository must be workspace/slug")
}

func TestParseConfigCollapsesDirectories(t *testing.T) {
	cfg, err := parseConfig(&types.DataSourceConfig{Settings: map[string]interface{}{
		"repositories": []interface{}{map[string]interface{}{
			"repository": "ws/repo", "ref": "main",
			"paths": []interface{}{"/docs/", "docs/api", "src"},
		}},
	}})
	require.NoError(t, err)
	want := []repositorySelection{{Repository: "ws/repo", Ref: "main", Paths: []string{"docs", "src"}}}
	require.Equal(t, want, cfg.Repositories)
}

func TestParseConfigRejectsDuplicateRepositoryIgnoringCase(t *testing.T) {
	_, err := parseConfig(&types.DataSourceConfig{Settings: map[string]interface{}{
		"repositories": []interface{}{
			map[string]interface{}{"repository": "ws/repo"},
			map[string]interface{}{"repository": "WS/Repo"},
		},
	}})
	require.ErrorIs(t, err, datasource.ErrInvalidConfig)
	require.EqualError(t, err, datasource.ErrInvalidConfig.Error()+": repository must be unique")
}

func TestParseConfigRequiresRepositories(t *testing.T) {
	_, err := parseConfig(&types.DataSourceConfig{Settings: map[string]interface{}{}})
	require.ErrorIs(t, err, datasource.ErrInvalidConfig)
	require.EqualError(t, err, datasource.ErrInvalidConfig.Error()+": settings.repositories is required")

	_, err = parseConfig(&types.DataSourceConfig{Settings: map[string]interface{}{"repositories": []interface{}{}}})
	require.ErrorIs(t, err, datasource.ErrInvalidConfig)
	require.EqualError(t, err, datasource.ErrInvalidConfig.Error()+": at least one repository is required")
}
