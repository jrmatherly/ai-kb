package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// The Lite wrapper runs the server from ~/.config/aikb, where the aikb CLI
// keeps its own config.yaml. Viper's search order would load that file as
// the server config; the explicit path must win.
func TestLoadConfigHonorsAIKBConfigFile(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	dir := t.TempDir()
	cliProfile := "current_profile: dev\nprofiles:\n  dev:\n    host: http://localhost:8080\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(cliProfile), 0o600); err != nil {
		t.Fatal(err)
	}
	serverCfg, err := os.ReadFile(filepath.Join("..", "..", "config", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	explicit := filepath.Join(dir, "config", "config.yaml")
	if err := os.WriteFile(explicit, serverCfg, 0o600); err != nil {
		t.Fatal(err)
	}

	t.Chdir(dir)
	t.Setenv(ConfigFileEnv, explicit)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig with %s set: %v", ConfigFileEnv, err)
	}
	if cfg.KnowledgeBase == nil || cfg.KnowledgeBase.ChunkSize <= 0 {
		t.Fatalf("server config was not loaded: %+v", cfg.KnowledgeBase)
	}
	if used := viper.ConfigFileUsed(); !strings.HasSuffix(used, filepath.Join("config", "config.yaml")) {
		t.Fatalf("viper used %q, want the explicit file", used)
	}
}
