package types

import (
	"testing"

	"github.com/jrmatherly/ai-kb/internal/storageallowlist"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorageBackendPathRoundTrip(t *testing.T) {
	path := BuildStorageBackendPath("backend-a", "cos://bucket/region/7/file.pdf")
	id, inner, ok := ParseStorageBackendPath(path)
	require.True(t, ok)
	assert.Equal(t, "backend-a", id)
	assert.Equal(t, "cos://bucket/region/7/file.pdf", inner)
	assert.Equal(t, "cos", ParseProviderScheme(path))
}

func TestSharesStorageBackendWithUsesConcreteInstance(t *testing.T) {
	aID, bID := "cos-a", "cos-b"
	a := &KnowledgeBase{StorageBackendID: &aID, StorageProviderConfig: &StorageProviderConfig{Provider: "cos"}}
	b := &KnowledgeBase{StorageBackendID: &bID, StorageProviderConfig: &StorageProviderConfig{Provider: "cos"}}
	assert.False(t, a.SharesStorageBackendWith(b, "", "cos"))

	b.StorageBackendID = &aID
	assert.True(t, a.SharesStorageBackendWith(b, "", "cos"))
}

func TestNewStorageBackendResponseMasksCredentials(t *testing.T) {
	backend := &StorageBackend{Config: StorageBackendConfig{AccessKeyID: "id", SecretAccessKey: "secret"}}
	response := NewStorageBackendResponse(backend)
	assert.Equal(t, RedactedSecretPlaceholder, response.Config.AccessKeyID)
	assert.Equal(t, RedactedSecretPlaceholder, response.Config.SecretAccessKey)
	assert.Equal(t, "id", backend.Config.AccessKeyID)
}

func TestStorageBackendFromEnvironment(t *testing.T) {
	t.Setenv("STORAGE_TYPE", "s3")
	t.Setenv("S3_ENDPOINT", "https://s3.example.com")
	t.Setenv("S3_REGION", "ap-test-1")
	t.Setenv("S3_ACCESS_KEY", "access")
	t.Setenv("S3_SECRET_KEY", "secret")
	t.Setenv("S3_BUCKET_NAME", "bucket")

	backend := StorageBackendFromEnvironment(42)
	require.NotNil(t, backend)
	assert.Equal(t, uint64(42), backend.TenantID)
	assert.Equal(t, "s3", backend.Provider)
	assert.Equal(t, StorageBackendSourceEnv, backend.Source)
	assert.True(t, backend.LegacyAlias)
	assert.Equal(t, "bucket", backend.Config.BucketName)
}

func TestStorageBackendFromEnvironmentKS3(t *testing.T) {
	t.Setenv("STORAGE_TYPE", "ks3")
	t.Setenv("KS3_ENDPOINT", "https://ks3-cn-beijing.ksyuncs.com")
	t.Setenv("KS3_REGION", "cn-beijing")
	t.Setenv("KS3_ACCESS_KEY", "access")
	t.Setenv("KS3_SECRET_KEY", "secret")
	t.Setenv("KS3_BUCKET_NAME", "bucket")
	t.Setenv("KS3_PATH_PREFIX", "aikb/")

	backend := StorageBackendFromEnvironment(42)
	require.NotNil(t, backend)
	assert.Equal(t, "ks3", backend.Provider)
	assert.Equal(t, &KS3EngineConfig{
		Endpoint: "https://ks3-cn-beijing.ksyuncs.com", Region: "cn-beijing", AccessKey: "access",
		SecretKey: "secret", BucketName: "bucket", PathPrefix: "aikb/",
	}, backend.ToStorageEngineConfig().KS3)
}

func TestStorageBackendRejectsTraversingPathPrefix(t *testing.T) {
	backend := &StorageBackend{
		TenantID: 1, Name: "unsafe", Provider: "local",
		Config: StorageBackendConfig{PathPrefix: "../outside"},
	}
	require.Error(t, backend.Validate())
}

// S3 needs only a region and a bucket: an empty endpoint means AWS itself, and
// no keys means the AWS default credential chain (IAM roles, IRSA, env).
func TestStorageBackendS3Validation(t *testing.T) {
	cases := []struct {
		name string
		cfg  StorageBackendConfig
		want string
	}{
		{"default chain on AWS", StorageBackendConfig{Region: "us-east-1", BucketName: "b"}, ""},
		{"static keys", StorageBackendConfig{
			Region: "us-east-1", BucketName: "b", AccessKeyID: "ak", SecretAccessKey: "sk",
		}, ""},
		{"half a key pair", StorageBackendConfig{
			Region: "us-east-1", BucketName: "b", AccessKeyID: "ak",
		}, "access_key_id and secret_access_key must be set together"},
		{"no region", StorageBackendConfig{BucketName: "b"}, "region is required"},
		{"no bucket", StorageBackendConfig{Region: "us-east-1"}, "bucket_name is required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(storageallowlist.AllowListEnv, "")
			err := (&StorageBackend{TenantID: 1, Name: "s3", Provider: "s3", Config: tc.cfg}).Validate()
			if tc.want == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

// OBS falls back to cn-north-4 at runtime, so saving must not demand a region.
func TestStorageBackendOBSValidationDefaultsRegion(t *testing.T) {
	t.Setenv(storageallowlist.AllowListEnv, "")
	cfg := StorageBackendConfig{Endpoint: "obs.example.com", AccessKeyID: "ak", SecretAccessKey: "sk", BucketName: "b"}
	require.NoError(t, (&StorageBackend{TenantID: 1, Name: "obs", Provider: "obs", Config: cfg}).Validate())

	cfg.Endpoint = ""
	err := (&StorageBackend{TenantID: 1, Name: "obs", Provider: "obs", Config: cfg}).Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "endpoint is required")
}
