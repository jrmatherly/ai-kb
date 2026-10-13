package web_search

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jrmatherly/ai-kb/internal/types"
)

func TestMetasoProviderSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer mk-test" {
			t.Fatalf("Authorization = %q", got)
		}
		var request metasoSearchRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request.Query != "AI KB" || request.Scope != "scholar" || request.Size != 2 {
			t.Fatalf("unexpected request: %+v", request)
		}
		if !request.IncludeSummary || request.IncludeRawContent || !request.ConciseSnippet {
			t.Fatalf("unexpected content options: %+v", request)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"credits":3,"searchParameters":{"q":"AI KB","scope":"scholar"},
			"total":3,"scholars":[
			{"title":"First","link":"https://example.com/1","score":"high","position":1,
			 "summary":"Summary","snippet":"Snippet","date":"2026-08-09","authors":["Author"]},
			{"title":"Second","link":"https://example.com/2","snippet":"Fallback snippet","date":"invalid"},
			{"title":"Third","link":"https://example.com/3","snippet":"must be capped"}
		]}`))
	}))
	defer server.Close()

	metaso := &MetasoProvider{client: server.Client(), baseURL: server.URL, apiKey: "mk-test", scope: "scholar"}
	results, err := metaso.Search(context.Background(), " AI KB ", 2, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	if results[0].Snippet != "Summary" || results[1].Snippet != "Fallback snippet" {
		t.Fatalf("unexpected snippets: %q, %q", results[0].Snippet, results[1].Snippet)
	}
	if results[0].Source != "metaso" || results[0].PublishedAt == nil {
		t.Fatalf("unexpected first result: %+v", results[0])
	}
	if want := time.Date(2026, 8, 9, 0, 0, 0, 0, time.UTC); !results[0].PublishedAt.Equal(want) {
		t.Fatalf("date = %v, want %v", results[0].PublishedAt, want)
	}
	if results[1].PublishedAt != nil {
		t.Fatalf("invalid date should be ignored: %v", results[1].PublishedAt)
	}
}

func TestMetasoProviderScopeArrays(t *testing.T) {
	fixtures := map[string]struct {
		payload string
		want    int
	}{
		"webpage": {
			payload: "{\"total\":1,\"webpages\":[{\"title\":\"W\",\"link\":\"https://example.com/w\",\n\t\t\t\t\"score\":\"high\",\"position\":1,\"summary\":\"\u7f51\u9875\u6458\u8981\",\"date\":\"2026-08-09\"}]}",
			want:    1,
		},
		"document": {
			payload: "{\"credits\":3,\"searchParameters\":{\"q\":\"1\",\"scope\":\"document\"},\"total\":15,\n\t\t\t\t\"documents\":[\n\t\t\t\t{\"title\":\"\u6587\u6863\u6807\u9898\u4e00\",\"link\":\"https://example.com/doc/1\",\"score\":\"high\",\n\t\t\t\t \"snippet\":\"\u6587\u6863\u7247\u6bb5\u4e00\",\"position\":1,\"authors\":[\"\u4f5c\u8005\u7532\"]},\n\t\t\t\t{\"title\":\"\u6587\u6863\u6807\u9898\u4e8c\",\"link\":\"https://example.com/doc/2 with space\",\"score\":\"medium\",\n\t\t\t\t \"snippet\":\"\u6587\u6863\u7247\u6bb5\u4e8c\",\"position\":2,\"authors\":[\"\u4f5c\u8005\u4e59\"]}\n\t\t\t]}",
			want:    2,
		},
		"scholar": {
			payload: "{\"credits\":3,\"searchParameters\":{\"q\":\"1\",\"scope\":\"scholar\"},\"total\":43,\n\t\t\t\t\"scholars\":[\n\t\t\t\t{\"title\":\"\u5b66\u672f\u6807\u9898\u4e00\",\"link\":\"https://example.com/paper?q=1\",\"score\":\"high\",\n\t\t\t\t \"snippet\":\"\u5b66\u672f\u7247\u6bb5\u4e00\",\"position\":1,\"authors\":[\"\u4f5c\u8005\u7532\"],\"date\":\"2012-10-15\"},\n\t\t\t\t{\"title\":\"\u5b66\u672f\u6807\u9898\u4e8c\",\"link\":\"https://example.com/paper?q=2\",\"score\":\"medium\",\n\t\t\t\t \"snippet\":\"\",\"position\":2,\"date\":\"2011-10-05\"}\n\t\t\t]}",
			want:    2,
		},
		"podcast": {
			payload: "{\"total\":1,\"podcasts\":[{\"title\":\"P\",\"link\":\"https://example.com/p\",\n\t\t\t\t\"score\":\"medium\",\"position\":1,\"snippet\":\"\u64ad\u5ba2\u7247\u6bb5\",\"authors\":[\"\u4e3b\u64ad\"],\n\t\t\t\t\"date\":\"2022-10-13\",\"duration\":\"299\"}]}",
			want:    1,
		},
		"video": {
			payload: "{\"total\":1,\"videos\":[{\"title\":\"V\",\"link\":\"https://example.com/v\",\n\t\t\t\t\"score\":\"medium\",\"position\":1,\"snippet\":\"\u89c6\u9891\u7247\u6bb5\",\"authors\":[\"UP\"],\n\t\t\t\t\"date\":\"2024-06-17\",\"duration\":\"497\",\"coverImage\":\"https://example.com/v.jpg\"}]}",
			want:    1,
		},
	}
	for scope, fixture := range fixtures {
		t.Run(scope, func(t *testing.T) {
			body := fixture.payload
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(body))
			}))
			defer server.Close()

			metaso := &MetasoProvider{
				client: server.Client(), baseURL: server.URL,
				apiKey: "mk-test", scope: scope,
			}
			results, err := metaso.Search(context.Background(), "q", 5, true)
			if err != nil {
				t.Fatal(err)
			}
			if len(results) != fixture.want {
				t.Fatalf("scope=%s len(results) = %d, want %d", scope, len(results), fixture.want)
			}
			if results[0].Title == "" {
				t.Fatalf("scope=%s title is empty", scope)
			}
			if results[0].URL == "" {
				t.Fatalf("scope=%s URL is empty", scope)
			}
			if results[0].Snippet == "" {
				t.Fatalf("scope=%s snippet is empty", scope)
			}
		})
	}
}

func TestMetasoProviderImageScopeHasNoLink(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{\"credits\":3,\"searchParameters\":{\"q\":\"1\",\"scope\":\"image\"},\"total\":31,\n\t\t\t\"images\":[{\"title\":\"\u56fe\u7247\u4e00\",\"score\":\"low\",\"position\":1,\n\t\t\t \"imageUrl\":\"https://example.com/i.jpg\",\"imageWidth\":800,\"imageHeight\":600}]}"))
	}))
	defer server.Close()

	metaso := &MetasoProvider{
		client: server.Client(), baseURL: server.URL,
		apiKey: "mk-test", scope: "image",
	}
	results, err := metaso.Search(context.Background(), "q", 5, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].Title == "" {
		t.Fatalf("title is empty: %+v", results[0])
	}
	if results[0].URL != "" {
		t.Fatalf("URL = %q, want empty: image entries have no link", results[0].URL)
	}
}

func TestMetasoProviderToleratesNonStringURLFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{\"total\":1,\"webpages\":[{\"title\":\"W\",\"link\":\"https://example.com/w\",\n\t\t\t\"snippet\":\"\u7247\u6bb5\",\"url\":{\"href\":\"https://example.com/obj\"},\n\t\t\t\"imageUrl\":{\"href\":\"https://example.com/img\"}}]}"))
	}))
	defer server.Close()

	metaso := &MetasoProvider{
		client: server.Client(), baseURL: server.URL,
		apiKey: "mk-test", scope: "webpage",
	}
	results, err := metaso.Search(context.Background(), "q", 5, false)
	if err != nil {
		t.Fatalf("non-string url fields must not fail the search: %v", err)
	}
	if len(results) != 1 || results[0].URL != "https://example.com/w" {
		t.Fatalf("results = %+v, want one result for https://example.com/w", results)
	}
}

func TestMetasoProviderToleratesUnmodelledFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{\"total\":3,\"pageSize\":10,\"webpages\":[\n\t\t\t{\"title\":\"W\",\"link\":\"https://example.com/w\",\"snippet\":\"\u7247\u6bb5\",\n\t\t\t \"score\":123,\"position\":\"1\",\"authors\":null,\"unknownNested\":{\"a\":[1,2]}}]}"))
	}))
	defer server.Close()

	metaso := &MetasoProvider{
		client: server.Client(), baseURL: server.URL,
		apiKey: "mk-test", scope: "webpage",
	}
	results, err := metaso.Search(context.Background(), "q", 5, false)
	if err != nil {
		t.Fatalf("unmodelled fields must not fail the search: %v", err)
	}
	if len(results) != 1 || results[0].URL != "https://example.com/w" {
		t.Fatalf("results = %+v, want one result for https://example.com/w", results)
	}
}

func TestValidateMetasoParameters(t *testing.T) {
	if err := ValidateMetasoParameters(types.WebSearchProviderParameters{}); err == nil {
		t.Fatal("expected missing API key error")
	}
	if err := ValidateMetasoParameters(types.WebSearchProviderParameters{APIKey: "mk-test", ExtraConfig: map[string]string{"scope": "unknown"}}); err == nil {
		t.Fatal("expected invalid scope error")
	}
	if err := ValidateMetasoParameters(types.WebSearchProviderParameters{APIKey: "mk-test"}); err != nil {
		t.Fatalf("default parameters: %v", err)
	}
}

func TestMetasoProviderHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"invalid API key"}`))
	}))
	defer server.Close()
	metaso := &MetasoProvider{client: server.Client(), baseURL: server.URL, apiKey: "bad", scope: defaultMetasoScope}
	_, err := metaso.Search(context.Background(), "test", 1, false)
	if err == nil || !strings.Contains(err.Error(), "invalid API key") {
		t.Fatalf("error = %v", err)
	}
}
