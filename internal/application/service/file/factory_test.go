package file

import (
	"testing"

	"github.com/jrmatherly/ai-kb/internal/types"
)

// OBS_* env configures the process-wide backend only when STORAGE_TYPE=obs
// (types.StorageBackendFromEnvironment); a workspace's own OBS config never
// borrows it, the same as every other provider.
func TestNewFileServiceFromStorageConfigOBSIgnoresEnv(t *testing.T) {
	t.Setenv("OBS_ENDPOINT", "obs.cn-north-4.myhuaweicloud.com")
	t.Setenv("OBS_ACCESS_KEY", "ak")
	t.Setenv("OBS_SECRET_KEY", "sk")
	t.Setenv("OBS_BUCKET_NAME", "b")

	_, provider, err := NewFileServiceFromStorageConfig("obs", &types.StorageEngineConfig{}, "")
	if provider != "obs" || err == nil || err.Error() != "incomplete obs config" {
		t.Fatalf("got provider=%q err=%v, want obs and incomplete obs config", provider, err)
	}
}
