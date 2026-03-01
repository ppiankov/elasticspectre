package elastic

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newMockServer creates a test server that routes GET requests by path.
func newMockServer(t *testing.T, routes map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		for pattern, resp := range routes {
			if path == pattern {
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(resp); err != nil {
					t.Errorf("encoding response for %s: %v", path, err)
				}
				return
			}
		}
		http.NotFound(w, r)
	}))
}

func newTestClient(t *testing.T, server *httptest.Server) *Client {
	t.Helper()
	c, err := New(Options{URL: server.URL})
	if err != nil {
		t.Fatalf("creating test client: %v", err)
	}
	return c
}

func esRootResponse() map[string]any {
	return map[string]any{
		"name":         "test-node",
		"cluster_name": "test-cluster",
		"version": map[string]any{
			"number":       "8.12.0",
			"build_flavor": "default",
		},
		"tagline": "You Know, for Search",
	}
}

func osRootResponse() map[string]any {
	return map[string]any{
		"name":         "test-node",
		"cluster_name": "test-cluster",
		"version": map[string]any{
			"number":       "2.12.0",
			"distribution": "opensearch",
		},
	}
}

func defaultStatsResponse() map[string]any {
	return map[string]any{
		"indices": map[string]any{
			"logs-2024": map[string]any{
				"total": map[string]any{
					"indexing": map[string]any{"index_total": 5000},
					"search":   map[string]any{"query_total": 200},
				},
			},
			"metrics-2024": map[string]any{
				"total": map[string]any{
					"indexing": map[string]any{"index_total": 0},
					"search":   map[string]any{"query_total": 0},
				},
			},
		},
	}
}

func defaultCatIndices() []map[string]string {
	return []map[string]string{
		{"index": "logs-2024", "status": "open", "docs.count": "1000", "store.size": "5mb"},
		{"index": "metrics-2024", "status": "open", "docs.count": "500", "store.size": "2mb"},
	}
}

func emptyILMResponse() map[string]any {
	return map[string]any{"indices": map[string]any{}}
}

func emptySettingsResponse() map[string]any {
	return map[string]any{}
}

// fullESRoutes returns a standard set of routes for an Elasticsearch cluster.
func fullESRoutes(catIndices any, stats any) map[string]any {
	return map[string]any{
		"/":                       esRootResponse(),
		"/_cat/indices":           catIndices,
		"/_stats/indexing,search": stats,
		"/*/_ilm/explain":         emptyILMResponse(),
		"/_all/_settings/index.routing.allocation.include._tier_preference": emptySettingsResponse(),
	}
}

// --- Constructor tests ---

func TestNew_URLOnly(t *testing.T) {
	c, err := New(Options{URL: "http://localhost:9200"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.baseURL != "http://localhost:9200" {
		t.Errorf("baseURL = %q, want http://localhost:9200", c.baseURL)
	}
}

func TestNew_URLTrailingSlash(t *testing.T) {
	c, err := New(Options{URL: "http://localhost:9200/"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.baseURL != "http://localhost:9200" {
		t.Errorf("baseURL = %q, want http://localhost:9200", c.baseURL)
	}
}

func TestNew_CloudIDOnly(t *testing.T) {
	// base64("us-east-1.aws.found.io$es1$kib1") = "dXMtZWFzdC0xLmF3cy5mb3VuZC5pbyRlczEka2liMQ=="
	c, err := New(Options{CloudID: "my-deploy:dXMtZWFzdC0xLmF3cy5mb3VuZC5pbyRlczEka2liMQ=="})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "https://es1.us-east-1.aws.found.io:443"
	if c.baseURL != want {
		t.Errorf("baseURL = %q, want %q", c.baseURL, want)
	}
}

func TestNew_NeitherURLNorCloudID(t *testing.T) {
	_, err := New(Options{})
	if err == nil {
		t.Fatal("expected error when neither URL nor CloudID provided")
	}
}

func TestNew_BothURLAndCloudID(t *testing.T) {
	_, err := New(Options{URL: "http://localhost:9200", CloudID: "x:abc"})
	if err == nil {
		t.Fatal("expected error when both URL and CloudID provided")
	}
}

// --- Cloud ID tests ---

func TestResolveCloudID_Valid(t *testing.T) {
	url, err := resolveCloudID("my-deploy:dXMtZWFzdC0xLmF3cy5mb3VuZC5pbyRlczEka2liMQ==")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "https://es1.us-east-1.aws.found.io:443"
	if url != want {
		t.Errorf("got %q, want %q", url, want)
	}
}

func TestResolveCloudID_MissingColon(t *testing.T) {
	_, err := resolveCloudID("nocolon")
	if err == nil {
		t.Fatal("expected error for missing colon")
	}
}

func TestResolveCloudID_InvalidBase64(t *testing.T) {
	_, err := resolveCloudID("name:!!!invalid!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestResolveCloudID_MissingSegments(t *testing.T) {
	// base64("hostonly") = "aG9zdG9ubHk="
	_, err := resolveCloudID("name:aG9zdG9ubHk=")
	if err == nil {
		t.Fatal("expected error for missing $ segments")
	}
}

// --- Flavor detection tests ---

func TestInfo_Elasticsearch(t *testing.T) {
	server := newMockServer(t, map[string]any{"/": esRootResponse()})
	defer server.Close()

	info, err := newTestClient(t, server).Info(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Flavor != Elasticsearch {
		t.Errorf("flavor = %s, want elasticsearch", info.Flavor)
	}
	if info.Version != "8.12.0" {
		t.Errorf("version = %s, want 8.12.0", info.Version)
	}
	if info.Name != "test-node" {
		t.Errorf("name = %s, want test-node", info.Name)
	}
}

func TestInfo_OpenSearch(t *testing.T) {
	server := newMockServer(t, map[string]any{"/": osRootResponse()})
	defer server.Close()

	info, err := newTestClient(t, server).Info(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Flavor != OpenSearch {
		t.Errorf("flavor = %s, want opensearch", info.Flavor)
	}
	if info.Version != "2.12.0" {
		t.Errorf("version = %s, want 2.12.0", info.Version)
	}
}

// --- Auth tests ---

func TestDoGet_BasicAuth(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	c, err := New(Options{URL: server.URL, Username: "admin", Password: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.doGet(context.Background(), "/")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(gotAuth, "Basic ") {
		t.Errorf("expected Basic auth header, got %q", gotAuth)
	}
}

func TestDoGet_APIKeyAuth(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	c, err := New(Options{URL: server.URL, APIKey: "my-api-key"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.doGet(context.Background(), "/")
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth != "ApiKey my-api-key" {
		t.Errorf("expected 'ApiKey my-api-key', got %q", gotAuth)
	}
}

func TestDoGet_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"forbidden"}`))
	}))
	defer server.Close()

	c := newTestClient(t, server)
	_, err := c.doGet(context.Background(), "/")
	if err == nil {
		t.Fatal("expected error for 403 response")
	}
	if !strings.Contains(err.Error(), "HTTP 403") {
		t.Errorf("expected HTTP 403 in error, got: %v", err)
	}
}

// --- AuditIndices tests ---

func TestAuditIndices_BasicFlow(t *testing.T) {
	routes := fullESRoutes(defaultCatIndices(), defaultStatsResponse())
	server := newMockServer(t, routes)
	defer server.Close()

	indices, err := newTestClient(t, server).AuditIndices(context.Background(), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(indices) != 2 {
		t.Fatalf("expected 2 indices, got %d", len(indices))
	}

	for _, idx := range indices {
		if idx.Name == "logs-2024" {
			if idx.IndexTotal != 5000 {
				t.Errorf("logs-2024 IndexTotal = %d, want 5000", idx.IndexTotal)
			}
			if idx.SearchTotal != 200 {
				t.Errorf("logs-2024 SearchTotal = %d, want 200", idx.SearchTotal)
			}
			if idx.DocsCount != 1000 {
				t.Errorf("logs-2024 DocsCount = %d, want 1000", idx.DocsCount)
			}
			if idx.StoreSizeBytes != 5*1024*1024 {
				t.Errorf("logs-2024 StoreSizeBytes = %d, want %d", idx.StoreSizeBytes, 5*1024*1024)
			}
		}
		if idx.Name == "metrics-2024" {
			if idx.IndexTotal != 0 {
				t.Errorf("metrics-2024 IndexTotal = %d, want 0", idx.IndexTotal)
			}
			if idx.SearchTotal != 0 {
				t.Errorf("metrics-2024 SearchTotal = %d, want 0", idx.SearchTotal)
			}
		}
	}
}

func TestAuditIndices_SystemIndexFiltering(t *testing.T) {
	catIndices := []map[string]string{
		{"index": "logs-2024", "status": "open", "docs.count": "100", "store.size": "1mb"},
		{"index": ".kibana", "status": "open", "docs.count": "50", "store.size": "500kb"},
		{"index": ".security", "status": "open", "docs.count": "10", "store.size": "100kb"},
	}
	routes := fullESRoutes(catIndices, map[string]any{"indices": map[string]any{}})
	server := newMockServer(t, routes)
	defer server.Close()

	indices, err := newTestClient(t, server).AuditIndices(context.Background(), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(indices) != 1 {
		t.Fatalf("expected 1 index (system excluded), got %d", len(indices))
	}
	if indices[0].Name != "logs-2024" {
		t.Errorf("expected logs-2024, got %s", indices[0].Name)
	}
}

func TestAuditIndices_IncludeSystem(t *testing.T) {
	catIndices := []map[string]string{
		{"index": "logs-2024", "status": "open", "docs.count": "100", "store.size": "1mb"},
		{"index": ".kibana", "status": "open", "docs.count": "50", "store.size": "500kb"},
	}
	routes := fullESRoutes(catIndices, map[string]any{"indices": map[string]any{}})
	server := newMockServer(t, routes)
	defer server.Close()

	indices, err := newTestClient(t, server).AuditIndices(context.Background(), true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(indices) != 2 {
		t.Fatalf("expected 2 indices (system included), got %d", len(indices))
	}
}

func TestAuditIndices_WithILMManaged(t *testing.T) {
	catIndices := []map[string]string{
		{"index": "logs-2024", "status": "open", "docs.count": "100", "store.size": "1mb"},
		{"index": "metrics-2024", "status": "open", "docs.count": "50", "store.size": "500kb"},
	}
	ilmResp := map[string]any{
		"indices": map[string]any{
			"logs-2024": map[string]any{"managed": true},
		},
	}
	routes := map[string]any{
		"/":                       esRootResponse(),
		"/_cat/indices":           catIndices,
		"/_stats/indexing,search": map[string]any{"indices": map[string]any{}},
		"/*/_ilm/explain":         ilmResp,
		"/_all/_settings/index.routing.allocation.include._tier_preference": emptySettingsResponse(),
	}
	server := newMockServer(t, routes)
	defer server.Close()

	indices, err := newTestClient(t, server).AuditIndices(context.Background(), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, idx := range indices {
		if idx.Name == "logs-2024" && !idx.HasILMPolicy {
			t.Error("logs-2024 should have HasILMPolicy=true")
		}
		if idx.Name == "metrics-2024" && idx.HasILMPolicy {
			t.Error("metrics-2024 should have HasILMPolicy=false")
		}
	}
}

func TestAuditIndices_OpenSearchISM(t *testing.T) {
	catIndices := []map[string]string{
		{"index": "logs-2024", "status": "open", "docs.count": "100", "store.size": "1mb"},
	}
	ismResp := map[string]any{
		"total_managed_indices": 1,
		"logs-2024": map[string]any{
			"index.plugins.index_state_management.policy_id": "hot-warm-delete",
		},
	}
	routes := map[string]any{
		"/":                       osRootResponse(),
		"/_cat/indices":           catIndices,
		"/_stats/indexing,search": map[string]any{"indices": map[string]any{}},
		"/_plugins/_ism/explain":  ismResp,
		"/_all/_settings/index.routing.allocation.include._tier_preference": emptySettingsResponse(),
	}
	server := newMockServer(t, routes)
	defer server.Close()

	indices, err := newTestClient(t, server).AuditIndices(context.Background(), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(indices) != 1 {
		t.Fatalf("expected 1 index, got %d", len(indices))
	}
	if !indices[0].HasILMPolicy {
		t.Error("logs-2024 should have HasILMPolicy=true via ISM")
	}
}

func TestAuditIndices_TierPreference(t *testing.T) {
	catIndices := []map[string]string{
		{"index": "logs-2024", "status": "open", "docs.count": "100", "store.size": "1mb"},
	}
	tierResp := map[string]any{
		"logs-2024": map[string]any{
			"settings": map[string]any{
				"index.routing.allocation.include._tier_preference": "data_warm",
			},
		},
	}
	routes := map[string]any{
		"/":                       esRootResponse(),
		"/_cat/indices":           catIndices,
		"/_stats/indexing,search": map[string]any{"indices": map[string]any{}},
		"/*/_ilm/explain":         emptyILMResponse(),
		"/_all/_settings/index.routing.allocation.include._tier_preference": tierResp,
	}
	server := newMockServer(t, routes)
	defer server.Close()

	indices, err := newTestClient(t, server).AuditIndices(context.Background(), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(indices) != 1 {
		t.Fatalf("expected 1 index, got %d", len(indices))
	}
	if indices[0].TierPreference != "data_warm" {
		t.Errorf("TierPreference = %q, want data_warm", indices[0].TierPreference)
	}
}

func TestAuditIndices_ClosedIndex(t *testing.T) {
	catIndices := []map[string]string{
		{"index": "old-logs", "status": "close", "docs.count": "0", "store.size": "10mb"},
	}
	routes := fullESRoutes(catIndices, map[string]any{"indices": map[string]any{}})
	server := newMockServer(t, routes)
	defer server.Close()

	indices, err := newTestClient(t, server).AuditIndices(context.Background(), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(indices) != 1 {
		t.Fatalf("expected 1 index, got %d", len(indices))
	}
	if indices[0].Status != "close" {
		t.Errorf("status = %q, want close", indices[0].Status)
	}
	if indices[0].IndexTotal != 0 {
		t.Errorf("closed index should have IndexTotal=0, got %d", indices[0].IndexTotal)
	}
}

// --- parseSizeToBytes tests ---

func TestParseSizeToBytes(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{"", 0},
		{"0", 0},
		{"100b", 100},
		{"1kb", 1024},
		{"2mb", 2 * 1024 * 1024},
		{"2.5mb", int64(2.5 * 1024 * 1024)},
		{"1gb", 1024 * 1024 * 1024},
		{"1tb", 1024 * 1024 * 1024 * 1024},
		{"500", 500},
		{"invalid", 0},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseSizeToBytes(tt.input)
			if got != tt.want {
				t.Errorf("parseSizeToBytes(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}
