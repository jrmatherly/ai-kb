package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsImportable(t *testing.T) {
	all := ImportCapabilities{Images: true, Audio: true}
	images := ImportCapabilities{Images: true}
	none := ImportCapabilities{}
	for _, tc := range []struct {
		ext  string
		caps ImportCapabilities
		want bool
	}{
		{"md", none, true},
		{"mdx", none, true},
		{"png", none, false},
		{"png", images, true},
		{"png", all, true},
		{"mp3", none, false},
		{"mp3", images, false},
		{"mp3", ImportCapabilities{Audio: true}, true},
		{"webp", all, false}, // image type outside the shared set stays unsupported
		{"go", all, false},
		{"", all, false},
	} {
		require.Equal(t, tc.want, IsImportable(tc.ext, tc.caps), "%s %+v", tc.ext, tc.caps)
	}
}

func TestIsImportableIgnoresCase(t *testing.T) {
	require.False(t, IsImportable(".PNG", ImportCapabilities{}))
	require.True(t, IsImportable(".PNG", ImportCapabilities{Images: true}))
}

func TestMissingModel(t *testing.T) {
	require.Equal(t, "vision", MissingModel("png", ImportCapabilities{}))
	require.Equal(t, "speech", MissingModel(".WAV", ImportCapabilities{Images: true}))
	require.Equal(t, "", MissingModel("png", ImportCapabilities{Images: true}))
	require.Equal(t, "", MissingModel("md", ImportCapabilities{}))
	require.Equal(t, "", MissingModel("", ImportCapabilities{}))
}

func TestGained(t *testing.T) {
	require.True(t, ImportCapabilities{Images: true}.Gained(ImportCapabilities{}))
	require.False(t, ImportCapabilities{}.Gained(ImportCapabilities{Images: true}))
	require.False(t, ImportCapabilities{Images: true}.Gained(ImportCapabilities{Images: true}))
	require.True(t, ImportCapabilities{Audio: true}.Gained(ImportCapabilities{}))
	require.False(t, ImportCapabilities{Audio: true}.Gained(ImportCapabilities{Audio: true}))
}
