package elastic

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// ClusterHealth holds cluster-level health information.
type ClusterHealth struct {
	Status             string
	NodeCount          int
	ActiveShards       int
	UnassignedShards   int
	RelocatingShards   int
	InitializingShards int
}

// SnapshotPolicyStatus reports whether automated snapshot policies exist.
type SnapshotPolicyStatus struct {
	PolicyCount int
	HasPolicy   bool
}

// SecurityStatus reports whether authentication is enabled.
type SecurityStatus struct {
	AuthEnabled bool
}

// clusterHealthResponse mirrors GET /_cluster/health.
type clusterHealthResponse struct {
	Status             string `json:"status"`
	NumberOfNodes      int    `json:"number_of_nodes"`
	ActiveShards       int    `json:"active_shards"`
	UnassignedShards   int    `json:"unassigned_shards"`
	RelocatingShards   int    `json:"relocating_shards"`
	InitializingShards int    `json:"initializing_shards"`
}

// osmPoliciesResponse mirrors OpenSearch GET /_plugins/_sm/policies.
type osmPoliciesResponse struct {
	Policies []json.RawMessage `json:"policies"`
}

// Health fetches cluster health from /_cluster/health.
func (c *Client) Health(ctx context.Context) (ClusterHealth, error) {
	body, err := c.doGet(ctx, "/_cluster/health")
	if err != nil {
		return ClusterHealth{}, fmt.Errorf("cluster health: %w", err)
	}

	var raw clusterHealthResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return ClusterHealth{}, fmt.Errorf("parsing cluster health: %w", err)
	}

	return ClusterHealth{
		Status:             raw.Status,
		NodeCount:          raw.NumberOfNodes,
		ActiveShards:       raw.ActiveShards,
		UnassignedShards:   raw.UnassignedShards,
		RelocatingShards:   raw.RelocatingShards,
		InitializingShards: raw.InitializingShards,
	}, nil
}

// CheckSnapshotPolicies checks for automated snapshot policies.
// Uses SLM (Elasticsearch) or Snapshot Management (OpenSearch) based on flavor.
// Returns defaults if the endpoint is unavailable (soft failure).
func (c *Client) CheckSnapshotPolicies(ctx context.Context, flavor Flavor) (SnapshotPolicyStatus, error) {
	if flavor == OpenSearch {
		body, err := c.doGet(ctx, "/_plugins/_sm/policies")
		if err != nil {
			return SnapshotPolicyStatus{}, nil
		}
		var resp osmPoliciesResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return SnapshotPolicyStatus{}, nil
		}
		count := len(resp.Policies)
		return SnapshotPolicyStatus{PolicyCount: count, HasPolicy: count > 0}, nil
	}

	// Elasticsearch: GET /_slm/policy returns map[string]policyObject
	body, err := c.doGet(ctx, "/_slm/policy")
	if err != nil {
		return SnapshotPolicyStatus{}, nil
	}
	var policies map[string]json.RawMessage
	if err := json.Unmarshal(body, &policies); err != nil {
		return SnapshotPolicyStatus{}, nil
	}
	count := len(policies)
	return SnapshotPolicyStatus{PolicyCount: count, HasPolicy: count > 0}, nil
}

// CheckSecurity checks whether authentication is enabled.
// Uses _security/_authenticate (ES) or _plugins/_security/authinfo (OpenSearch).
// 200/401/403 = auth enabled; 404/other = security plugin not installed.
func (c *Client) CheckSecurity(ctx context.Context, flavor Flavor) (SecurityStatus, error) {
	path := "/_security/_authenticate"
	if flavor == OpenSearch {
		path = "/_plugins/_security/authinfo"
	}

	_, err := c.doGet(ctx, path)
	if err == nil {
		return SecurityStatus{AuthEnabled: true}, nil
	}

	errMsg := err.Error()
	if strings.Contains(errMsg, "HTTP 401") || strings.Contains(errMsg, "HTTP 403") {
		return SecurityStatus{AuthEnabled: true}, nil
	}

	return SecurityStatus{AuthEnabled: false}, nil
}
