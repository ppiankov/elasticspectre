package elastic

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// IndexInfo holds aggregated metadata for a single index.
type IndexInfo struct {
	Name           string
	Status         string // "open" or "close"
	DocsCount      int64
	StoreSizeBytes int64
	IndexTotal     int64  // lifetime write operations (from _stats)
	SearchTotal    int64  // lifetime search operations (from _stats)
	HasILMPolicy   bool   // ILM (ES) or ISM (OpenSearch) policy attached
	TierPreference string // e.g. "data_hot", "data_warm"
}

// AuditIndices fetches all index data and returns a slice of IndexInfo.
// System indices (names starting with ".") are excluded unless includeSystem is true.
func (c *Client) AuditIndices(ctx context.Context, includeSystem bool) ([]IndexInfo, error) {
	info, err := c.Info(ctx)
	if err != nil {
		return nil, err
	}

	catIndices, err := c.listIndices(ctx)
	if err != nil {
		return nil, err
	}

	stats, err := c.fetchStats(ctx)
	if err != nil {
		return nil, err
	}

	ilmStatus, err := c.fetchILMStatus(ctx, info.Flavor)
	if err != nil {
		return nil, err
	}

	tierPrefs, err := c.fetchTierPreferences(ctx)
	if err != nil {
		return nil, err
	}

	var result []IndexInfo
	for _, ci := range catIndices {
		name := ci.Index
		if !includeSystem && len(name) > 0 && name[0] == '.' {
			continue
		}

		docsCount, _ := strconv.ParseInt(ci.DocsCount, 10, 64)
		storeSizeBytes := parseSizeToBytes(ci.StoreSize)

		idx := IndexInfo{
			Name:           name,
			Status:         ci.Status,
			DocsCount:      docsCount,
			StoreSizeBytes: storeSizeBytes,
			HasILMPolicy:   ilmStatus[name],
			TierPreference: tierPrefs[name],
		}

		if s, ok := stats.Indices[name]; ok {
			idx.IndexTotal = s.Total.Indexing.IndexTotal
			idx.SearchTotal = s.Total.Search.QueryTotal
		}

		result = append(result, idx)
	}
	return result, nil
}

// catIndex represents one entry from /_cat/indices?format=json.
type catIndex struct {
	Index     string `json:"index"`
	Status    string `json:"status"`
	DocsCount string `json:"docs.count"`
	StoreSize string `json:"store.size"`
}

// listIndices fetches the basic index catalog.
func (c *Client) listIndices(ctx context.Context) ([]catIndex, error) {
	body, err := c.doGet(ctx, "/_cat/indices?format=json&h=index,status,docs.count,store.size")
	if err != nil {
		return nil, fmt.Errorf("listing indices: %w", err)
	}

	var indices []catIndex
	if err := json.Unmarshal(body, &indices); err != nil {
		return nil, fmt.Errorf("parsing index list: %w", err)
	}
	return indices, nil
}

// statsResponse mirrors the relevant parts of GET /_stats/indexing,search.
type statsResponse struct {
	Indices map[string]struct {
		Total struct {
			Indexing struct {
				IndexTotal int64 `json:"index_total"`
			} `json:"indexing"`
			Search struct {
				QueryTotal int64 `json:"query_total"`
			} `json:"search"`
		} `json:"total"`
	} `json:"indices"`
}

// fetchStats calls GET /_stats and returns per-index indexing/search totals.
func (c *Client) fetchStats(ctx context.Context) (statsResponse, error) {
	body, err := c.doGet(ctx, "/_stats/indexing,search")
	if err != nil {
		return statsResponse{}, fmt.Errorf("fetching stats: %w", err)
	}

	var stats statsResponse
	if err := json.Unmarshal(body, &stats); err != nil {
		return statsResponse{}, fmt.Errorf("parsing stats: %w", err)
	}
	return stats, nil
}

// ilmExplainResponse mirrors GET /*/_ilm/explain.
type ilmExplainResponse struct {
	Indices map[string]struct {
		Managed bool `json:"managed"`
	} `json:"indices"`
}

// fetchILMStatus returns a map of index name -> has lifecycle policy.
// ILM/ISM endpoint failures are soft — missing plugin is not an error.
func (c *Client) fetchILMStatus(ctx context.Context, flavor Flavor) (map[string]bool, error) {
	managed := make(map[string]bool)

	if flavor == OpenSearch {
		body, err := c.doGet(ctx, "/_plugins/_ism/explain")
		if err != nil {
			return managed, nil
		}
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(body, &raw); err != nil {
			return managed, nil
		}
		for name, entry := range raw {
			if name == "total_managed_indices" {
				continue
			}
			var info struct {
				PolicyID string `json:"index.plugins.index_state_management.policy_id"`
			}
			if err := json.Unmarshal(entry, &info); err == nil && info.PolicyID != "" {
				managed[name] = true
			}
		}
		return managed, nil
	}

	// Elasticsearch: use ILM explain
	body, err := c.doGet(ctx, "/*/_ilm/explain?only_managed=true")
	if err != nil {
		return managed, nil
	}
	var resp ilmExplainResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return managed, nil
	}
	for name, info := range resp.Indices {
		if info.Managed {
			managed[name] = true
		}
	}
	return managed, nil
}

// fetchTierPreferences returns a map of index name -> tier preference string.
func (c *Client) fetchTierPreferences(ctx context.Context) (map[string]string, error) {
	tiers := make(map[string]string)

	body, err := c.doGet(ctx, "/_all/_settings/index.routing.allocation.include._tier_preference?flat_settings=true")
	if err != nil {
		return tiers, nil
	}

	var raw map[string]struct {
		Settings map[string]string `json:"settings"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return tiers, nil
	}

	for name, idx := range raw {
		if pref, ok := idx.Settings["index.routing.allocation.include._tier_preference"]; ok {
			tiers[name] = pref
		}
	}
	return tiers, nil
}

// parseSizeToBytes converts human-readable sizes (e.g. "2.5mb", "1gb") to bytes.
func parseSizeToBytes(s string) int64 {
	if s == "" {
		return 0
	}
	s = strings.TrimSpace(strings.ToLower(s))

	multipliers := []struct {
		suffix string
		mult   int64
	}{
		{"tb", 1024 * 1024 * 1024 * 1024},
		{"gb", 1024 * 1024 * 1024},
		{"mb", 1024 * 1024},
		{"kb", 1024},
		{"b", 1},
	}

	for _, m := range multipliers {
		if strings.HasSuffix(s, m.suffix) {
			numStr := strings.TrimSuffix(s, m.suffix)
			val, err := strconv.ParseFloat(numStr, 64)
			if err != nil {
				return 0
			}
			return int64(val * float64(m.mult))
		}
	}

	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return val
}
