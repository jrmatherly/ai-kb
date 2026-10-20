package skillparity

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/jrmatherly/ai-kb/cli/cmd"
	"github.com/jrmatherly/ai-kb/cli/internal/cmdutil"
)

// TestFlagUsageBackticksNameOneValue fails when a flag's usage string
// backticks a phrase: pflag renders the first backticked span as the flag's
// value name, so "see `aikb model list`" printed as "--chat-model aikb model
// list". A single word (`duration`, `key=value`) is the intended use.
func TestFlagUsageBackticksNameOneValue(t *testing.T) {
	root := cmd.NewRootCmd(&cmdutil.Factory{})
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		c.LocalFlags().VisitAll(func(f *pflag.Flag) {
			if name, _ := pflag.UnquoteUsage(f); strings.ContainsAny(name, " /") {
				t.Errorf("%s --%s: usage %q renders %q as the value name", c.CommandPath(), f.Name, f.Usage, name)
			}
		})
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(root)
}
