package docparser

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func engineNames(t *testing.T) []string {
	t.Helper()
	names := []string{}
	for _, e := range ListAllEngines(false, nil, nil) {
		names = append(names, e.Name)
	}
	return names
}

func TestListAllEnginesOmitsWeKnoraCloudWhenDisabled(t *testing.T) {
	t.Setenv(WeKnoraCloudEnabledEnv, "")
	require.NotContains(t, engineNames(t), WeKnoraCloudEngineName)

	t.Setenv(WeKnoraCloudEnabledEnv, "true")
	require.Contains(t, engineNames(t), WeKnoraCloudEngineName)
}
