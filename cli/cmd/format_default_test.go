package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrmatherly/ai-kb/cli/internal/cmdutil"
	"github.com/jrmatherly/ai-kb/cli/internal/iostreams"
)

// Precedence: --format > AIKB_FORMAT > config.yaml defaults.format > json.
func TestDefaultsFormatFromConfig(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  string
		args []string
		want string
	}{{
		name: "config default applies",
		args: []string{"version"},
		want: "aikb dev (commit none, built unknown)\n",
	}, {
		name: "env beats config",
		env:  "json",
		args: []string{"version", "--jq", ".data.version"},
		want: "dev\n",
	}, {
		name: "flag beats config",
		args: []string{"version", "--format", "json", "--jq", ".data.version"},
		want: "dev\n",
	}, {
		name: "config view reports it",
		args: []string{"config", "view", "--format", "json", "--jq", ".data.format_default"},
		want: "text\n",
	}, {
		name: "config view reports env first",
		env:  "json",
		args: []string{"config", "view", "--jq", ".data.format_default"},
		want: "json\n",
	}} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, os.MkdirAll(filepath.Join(dir, "aikb"), 0o700))
			require.NoError(t, os.WriteFile(filepath.Join(dir, "aikb", "config.yaml"),
				[]byte("defaults:\n  format: text\n"), 0o600))
			t.Setenv("XDG_CONFIG_HOME", dir)
			t.Setenv("AIKB_FORMAT", tc.env)

			ioOut, _ := iostreams.SetForTest(t)
			var out bytes.Buffer
			root := NewRootCmd(cmdutil.New())
			root.SetArgs(tc.args)
			root.SetOut(&out)
			require.NoError(t, root.Execute())
			assert.Equal(t, tc.want, out.String()+ioOut.String())
		})
	}
}
