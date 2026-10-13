package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jrmatherly/ai-kb/internal/handler"
	"github.com/jrmatherly/ai-kb/internal/storageallowlist"
	"github.com/stretchr/testify/require"
)

func TestStorageEngineStatusReportsEnvConfiguredBackend(t *testing.T) {
	cases := map[string]map[string]string{
		"ks3": {
			"KS3_ENDPOINT": "ks3-cn-beijing.ksyuncs.com", "KS3_REGION": "BEIJING",
			"KS3_ACCESS_KEY": "ak", "KS3_SECRET_KEY": "sk", "KS3_BUCKET_NAME": "b",
		},
		"oss": {
			"OSS_ENDPOINT": "oss-cn-hangzhou.aliyuncs.com", "OSS_REGION": "cn-hangzhou",
			"OSS_ACCESS_KEY": "ak", "OSS_SECRET_KEY": "sk", "OSS_BUCKET_NAME": "b",
		},
		"cos": {"COS_REGION": "ap-guangzhou", "COS_SECRET_ID": "ak", "COS_SECRET_KEY": "sk", "COS_BUCKET_NAME": "b"},
		"s3":  {"S3_REGION": "us-east-1", "S3_ACCESS_KEY": "ak", "S3_SECRET_KEY": "sk", "S3_BUCKET_NAME": "b"},
		"tos": {
			"TOS_ENDPOINT": "tos-cn-beijing.volces.com", "TOS_REGION": "cn-beijing",
			"TOS_ACCESS_KEY": "ak", "TOS_SECRET_KEY": "sk", "TOS_BUCKET_NAME": "b",
		},
		// OBS_REGION is optional: the file service defaults it.
		"obs": {
			"OBS_ENDPOINT":   "obs.cn-north-4.myhuaweicloud.com",
			"OBS_ACCESS_KEY": "ak", "OBS_SECRET_KEY": "sk", "OBS_BUCKET_NAME": "b",
		},
		// Credentials for providers other than STORAGE_TYPE configure nothing.
		"local": {
			"TOS_ENDPOINT": "tos-cn-beijing.volces.com", "TOS_REGION": "cn-beijing",
			"TOS_ACCESS_KEY": "ak", "TOS_SECRET_KEY": "sk", "TOS_BUCKET_NAME": "b",
			"OBS_ENDPOINT": "obs.cn-north-4.myhuaweicloud.com", "OBS_REGION": "cn-north-4",
			"OBS_ACCESS_KEY": "ak", "OBS_SECRET_KEY": "sk", "OBS_BUCKET_NAME": "b",
		},
	}
	// Blank every storage variable first so a developer's exported env cannot
	// change the result.
	storageEnv := []string{"MINIO_ENDPOINT", "MINIO_ACCESS_KEY_ID", "MINIO_SECRET_ACCESS_KEY", "MINIO_BUCKET_NAME"}
	for _, env := range cases {
		for k := range env {
			storageEnv = append(storageEnv, k)
		}
	}
	for provider, env := range cases {
		t.Run(provider, func(t *testing.T) {
			for _, k := range storageEnv {
				t.Setenv(k, "")
			}
			t.Setenv(storageallowlist.AllowListEnv, "")
			t.Setenv("STORAGE_TYPE", provider)
			for k, v := range env {
				t.Setenv(k, v)
			}
			engine := newKBRouteTestEngine(t, 1, &stubWikiKBLookup{}, nil, func(r *gin.RouterGroup, g *rbacGuards) {
				RegisterSystemRoutes(r, &handler.SystemHandler{}, g)
			})
			w := httptest.NewRecorder()
			engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/system/storage-engine-status", nil))
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())

			var resp struct {
				Data struct {
					Engines []handler.StorageEngineStatusItem `json:"engines"`
				} `json:"data"`
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			available := map[string]bool{}
			for _, e := range resp.Data.Engines {
				available[e.Name] = e.Available
			}
			require.Equal(t, map[string]bool{
				"local": true, "minio": false, "cos": provider == "cos", "tos": provider == "tos",
				"s3": provider == "s3", "oss": provider == "oss", "ks3": provider == "ks3", "obs": provider == "obs",
			}, available)
		})
	}
}
