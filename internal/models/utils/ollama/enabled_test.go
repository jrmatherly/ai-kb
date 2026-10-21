package ollama

import "testing"

func TestEnabledFollowsEnv(t *testing.T) {
	for env, want := range map[string]bool{"": false, "false": false, "maybe": false, "true": true, " 1 ": true} {
		t.Setenv(EnabledEnv, env)
		if got := Enabled(); got != want {
			t.Errorf("AIKB_OLLAMA_ENABLED=%q: Enabled() = %v, want %v", env, got, want)
		}
	}
}
