package datasource

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The default allow-list is restated in docs and config examples, so a connector added to
// DefaultConnectorTypes must be added to every copy in the same order.
func TestDefaultConnectorTypesParity(t *testing.T) {
	want := strings.Join(DefaultConnectorTypes, ",")
	copies := []struct {
		path    string
		pattern *regexp.Regexp
	}{
		{".env.example", regexp.MustCompile(`(?m)^# AIKB_DATASOURCE_TYPES=(.*)$`)},
		{"docker-compose.yml", regexp.MustCompile(
			`# Data source connector types offered \(allow-list; empty means (.*?), all means`)},
		{"website-docs/01-getting-started/04-configuration.md", regexp.MustCompile(
			"(?m)^\\| `AIKB_DATASOURCE_TYPES` \\| empty \\(`([^`]*)`\\) \\|")},
		{".claude/skills/upstream-review/SKILL.md", regexp.MustCompile(
			"`AIKB_DATASOURCE_TYPES` allow-list \\(default `([^`]*)`, ")},
	}
	for _, c := range copies {
		t.Run(c.path, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("..", "..", c.path))
			if err != nil {
				t.Fatal(err)
			}
			m := c.pattern.FindSubmatch(content)
			if m == nil {
				t.Fatalf("no line matches %s", c.pattern)
			}
			if got := string(m[1]); got != want {
				t.Errorf("lists %q, DefaultConnectorTypes is %q", got, want)
			}
		})
	}
}
