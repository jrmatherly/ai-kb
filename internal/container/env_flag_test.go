package container

import "testing"

func TestEnvFlag(t *testing.T) {
	for v, want := range map[string]bool{
		"": false, "false": false, "0": false, "yes": false,
		"true": true, "TRUE": true, "1": true, "t": true,
	} {
		t.Setenv("AIKB_MIGRATE_FAIL_FAST", v)
		if got := envFlag("AIKB_MIGRATE_FAIL_FAST"); got != want {
			t.Errorf("AIKB_MIGRATE_FAIL_FAST=%q: got %v, want %v", v, got, want)
		}
	}
}
