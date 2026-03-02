package elastic

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func clusterHealthResp(status string, unassigned int) map[string]any {
	return map[string]any{
		"status":              status,
		"number_of_nodes":     3,
		"active_shards":       50,
		"unassigned_shards":   unassigned,
		"relocating_shards":   0,
		"initializing_shards": 0,
	}
}

// --- Health tests ---

func TestHealth_Green(t *testing.T) {
	server := newMockServer(t, map[string]any{
		"/_cluster/health": clusterHealthResp("green", 0),
	})
	defer server.Close()

	h, err := newTestClient(t, server).Health(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h.Status != "green" {
		t.Errorf("status = %q, want green", h.Status)
	}
	if h.UnassignedShards != 0 {
		t.Errorf("unassigned = %d, want 0", h.UnassignedShards)
	}
	if h.NodeCount != 3 {
		t.Errorf("nodes = %d, want 3", h.NodeCount)
	}
}

func TestHealth_Yellow(t *testing.T) {
	server := newMockServer(t, map[string]any{
		"/_cluster/health": clusterHealthResp("yellow", 5),
	})
	defer server.Close()

	h, err := newTestClient(t, server).Health(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h.Status != "yellow" {
		t.Errorf("status = %q, want yellow", h.Status)
	}
	if h.UnassignedShards != 5 {
		t.Errorf("unassigned = %d, want 5", h.UnassignedShards)
	}
}

func TestHealth_Red(t *testing.T) {
	server := newMockServer(t, map[string]any{
		"/_cluster/health": clusterHealthResp("red", 20),
	})
	defer server.Close()

	h, err := newTestClient(t, server).Health(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h.Status != "red" {
		t.Errorf("status = %q, want red", h.Status)
	}
	if h.UnassignedShards != 20 {
		t.Errorf("unassigned = %d, want 20", h.UnassignedShards)
	}
}

// --- Snapshot policy tests ---

func TestCheckSnapshotPolicies_ES_HasPolicy(t *testing.T) {
	server := newMockServer(t, map[string]any{
		"/_slm/policy": map[string]any{
			"daily-snap":  map[string]any{"schedule": "0 0 * * *"},
			"weekly-snap": map[string]any{"schedule": "0 0 * * 0"},
		},
	})
	defer server.Close()

	sp, err := newTestClient(t, server).CheckSnapshotPolicies(context.Background(), Elasticsearch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sp.PolicyCount != 2 {
		t.Errorf("policy count = %d, want 2", sp.PolicyCount)
	}
	if !sp.HasPolicy {
		t.Error("expected HasPolicy=true")
	}
}

func TestCheckSnapshotPolicies_ES_NoPolicy(t *testing.T) {
	server := newMockServer(t, map[string]any{
		"/_slm/policy": map[string]any{},
	})
	defer server.Close()

	sp, err := newTestClient(t, server).CheckSnapshotPolicies(context.Background(), Elasticsearch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sp.PolicyCount != 0 {
		t.Errorf("policy count = %d, want 0", sp.PolicyCount)
	}
	if sp.HasPolicy {
		t.Error("expected HasPolicy=false")
	}
}

func TestCheckSnapshotPolicies_ES_EndpointMissing(t *testing.T) {
	// No /_slm/policy route → 404 → soft failure
	server := newMockServer(t, map[string]any{})
	defer server.Close()

	sp, err := newTestClient(t, server).CheckSnapshotPolicies(context.Background(), Elasticsearch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sp.PolicyCount != 0 {
		t.Errorf("policy count = %d, want 0", sp.PolicyCount)
	}
	if sp.HasPolicy {
		t.Error("expected HasPolicy=false on soft failure")
	}
}

func TestCheckSnapshotPolicies_OS_HasPolicy(t *testing.T) {
	server := newMockServer(t, map[string]any{
		"/_plugins/_sm/policies": map[string]any{
			"policies": []map[string]any{
				{"name": "daily-snap"},
			},
		},
	})
	defer server.Close()

	sp, err := newTestClient(t, server).CheckSnapshotPolicies(context.Background(), OpenSearch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sp.PolicyCount != 1 {
		t.Errorf("policy count = %d, want 1", sp.PolicyCount)
	}
	if !sp.HasPolicy {
		t.Error("expected HasPolicy=true")
	}
}

func TestCheckSnapshotPolicies_OS_NoPolicy(t *testing.T) {
	server := newMockServer(t, map[string]any{
		"/_plugins/_sm/policies": map[string]any{
			"policies": []map[string]any{},
		},
	})
	defer server.Close()

	sp, err := newTestClient(t, server).CheckSnapshotPolicies(context.Background(), OpenSearch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sp.PolicyCount != 0 {
		t.Errorf("policy count = %d, want 0", sp.PolicyCount)
	}
	if sp.HasPolicy {
		t.Error("expected HasPolicy=false")
	}
}

// --- Security tests ---

func TestCheckSecurity_ES_Authenticated(t *testing.T) {
	server := newMockServer(t, map[string]any{
		"/_security/_authenticate": map[string]any{"username": "elastic"},
	})
	defer server.Close()

	ss, err := newTestClient(t, server).CheckSecurity(context.Background(), Elasticsearch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ss.AuthEnabled {
		t.Error("expected AuthEnabled=true on 200")
	}
}

func TestCheckSecurity_ES_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"security_exception"}`))
	}))
	defer server.Close()

	c := newTestClient(t, server)
	ss, err := c.CheckSecurity(context.Background(), Elasticsearch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ss.AuthEnabled {
		t.Error("expected AuthEnabled=true on 401")
	}
}

func TestCheckSecurity_ES_Forbidden(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"security_exception"}`))
	}))
	defer server.Close()

	c := newTestClient(t, server)
	ss, err := c.CheckSecurity(context.Background(), Elasticsearch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ss.AuthEnabled {
		t.Error("expected AuthEnabled=true on 403")
	}
}

func TestCheckSecurity_ES_NoPlugin(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"no handler found"}`))
	}))
	defer server.Close()

	c := newTestClient(t, server)
	ss, err := c.CheckSecurity(context.Background(), Elasticsearch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ss.AuthEnabled {
		t.Error("expected AuthEnabled=false on 404")
	}
}

func TestCheckSecurity_OS_Authenticated(t *testing.T) {
	server := newMockServer(t, map[string]any{
		"/_plugins/_security/authinfo": map[string]any{"user": "admin"},
	})
	defer server.Close()

	ss, err := newTestClient(t, server).CheckSecurity(context.Background(), OpenSearch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ss.AuthEnabled {
		t.Error("expected AuthEnabled=true on 200")
	}
}

func TestCheckSecurity_OS_NoPlugin(t *testing.T) {
	// No /_plugins/_security/authinfo route → 404 → auth disabled
	server := newMockServer(t, map[string]any{})
	defer server.Close()

	ss, err := newTestClient(t, server).CheckSecurity(context.Background(), OpenSearch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ss.AuthEnabled {
		t.Error("expected AuthEnabled=false on 404")
	}
}
