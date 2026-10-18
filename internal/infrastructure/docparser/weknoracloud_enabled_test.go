package docparser

import (
	"context"
	"testing"

	"github.com/jrmatherly/ai-kb/internal/types"
	"github.com/stretchr/testify/require"
)

func weKnoraCloudEngineInfo(t *testing.T, overrides map[string]string) types.ParserEngineInfo {
	t.Helper()
	for _, e := range ListAllEngines(true, overrides, nil) {
		if e.Name == WeKnoraCloudEngineName {
			return e
		}
	}
	t.Fatalf("%s is not registered", WeKnoraCloudEngineName)
	return types.ParserEngineInfo{}
}

func TestWeKnoraCloudEngineUnavailableWhenDisabled(t *testing.T) {
	overrides := map[string]string{"weknoracloud_app_id": "app"}

	t.Setenv(WeKnoraCloudEnabledEnv, "")
	available, reason := (&weKnoraCloudEngine{}).CheckAvailable(true, overrides)
	require.False(t, available)
	require.Contains(t, reason, WeKnoraCloudEnabledEnv)

	t.Setenv(WeKnoraCloudEnabledEnv, "true")
	on := weKnoraCloudEngineInfo(t, overrides)
	require.True(t, on.Available, on.UnavailableReason)
}

func TestWeKnoraCloudReaderRefusesWhenDisabled(t *testing.T) {
	t.Setenv(WeKnoraCloudEnabledEnv, "")
	reader, err := NewReader(context.Background(), WeKnoraCloudEngineName, "pdf", false, ReaderDeps{})
	require.Nil(t, reader)
	require.ErrorContains(t, err, WeKnoraCloudEnabledEnv)
}
