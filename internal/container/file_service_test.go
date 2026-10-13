package container

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setKS3Env(t *testing.T, endpoint string) {
	t.Setenv("STORAGE_TYPE", "ks3")
	t.Setenv("KS3_ENDPOINT", endpoint)
	t.Setenv("KS3_REGION", "cn-beijing")
	t.Setenv("KS3_ACCESS_KEY", "ak")
	t.Setenv("KS3_SECRET_KEY", "sk")
	t.Setenv("KS3_BUCKET_NAME", "aikb")
}

// KS3 addresses buckets virtual-hosted style, so the bucket probe goes to
// aikb.<endpoint host>. A test server cannot answer that name, but the error
// shows the env reached the KS3 constructor and its SSRF-safe client.
func TestInitRawFileServiceBuildsKS3FromEnv(t *testing.T) {
	setKS3Env(t, "http://127.0.0.1:9")

	svc, err := initRawFileService(nil)
	assert.Nil(t, svc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `failed to check KS3 bucket "aikb"`)
	assert.Contains(t, err.Error(), `Head "http://aikb.127.0.0.1:9/": outbound request blocked by SSRF policy`)
}

func TestInitRawFileServiceRejectsUnsafeKS3Endpoint(t *testing.T) {
	setKS3Env(t, "http://169.254.169.254")

	svc, err := initRawFileService(nil)
	assert.Nil(t, svc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsafe KS3 endpoint")
}

func TestInitRawFileServiceRejectsIncompleteKS3Env(t *testing.T) {
	setKS3Env(t, "https://ks3-cn-beijing.ksyuncs.com")
	t.Setenv("KS3_BUCKET_NAME", "")

	svc, err := initRawFileService(nil)
	assert.Nil(t, svc)
	require.EqualError(t, err, "missing KS3 configuration")
}
