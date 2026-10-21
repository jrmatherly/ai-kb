// Package ollama wraps the Ollama client and the deployment switch for its settings section.
package ollama

import "github.com/jrmatherly/ai-kb/internal/envlist"

// EnabledEnv gates the Ollama settings section; off by default.
const EnabledEnv = "AIKB_OLLAMA_ENABLED"

// Enabled reports EnabledEnv; unset or unparsable is false.
func Enabled() bool {
	return envlist.Bool(EnabledEnv)
}
