package elastic

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
)

const (
	shardSprawlThresholdBytes    int64 = 10 * 1024 * 1024 * 1024 // 10 GB
	oversizedShardThresholdBytes int64 = 50 * 1024 * 1024 * 1024 // 50 GB
)

// ShardInfo holds metadata for a single shard.
type ShardInfo struct {
	Index     string
	Shard     int
	Primary   bool
	State     string // STARTED, UNASSIGNED, RELOCATING, INITIALIZING
	SizeBytes int64
	Node      string
}

// ShardAudit holds per-index shard health analysis.
type ShardAudit struct {
	Index           string
	PrimaryCount    int
	ReplicaCount    int
	UnassignedCount int
	TotalSizeBytes  int64
	AvgShardSize    int64 // average size per primary shard
	HasSprawl       bool  // avg primary shard size < 10GB and primary count > 1
	HasOversized    bool  // any primary shard > 50GB
	ReplicaWaste    bool  // has replicas but zero search activity
}

// catShard represents one entry from /_cat/shards?format=json.
type catShard struct {
	Index  string `json:"index"`
	Shard  string `json:"shard"`
	PriRep string `json:"prirep"`
	State  string `json:"state"`
	Store  string `json:"store"`
	Node   string `json:"node"`
}

// listShards fetches raw shard data from /_cat/shards.
func (c *Client) listShards(ctx context.Context) ([]ShardInfo, error) {
	body, err := c.doGet(ctx, "/_cat/shards?format=json&h=index,shard,prirep,state,store,node")
	if err != nil {
		return nil, fmt.Errorf("listing shards: %w", err)
	}

	var raw []catShard
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parsing shard list: %w", err)
	}

	shards := make([]ShardInfo, 0, len(raw))
	for _, cs := range raw {
		shardNum, err := strconv.Atoi(cs.Shard)
		if err != nil {
			shardNum = 0
		}
		shards = append(shards, ShardInfo{
			Index:     cs.Index,
			Shard:     shardNum,
			Primary:   cs.PriRep == "p",
			State:     cs.State,
			SizeBytes: parseSizeToBytes(cs.Store),
			Node:      cs.Node,
		})
	}
	return shards, nil
}

// AuditShards fetches shard allocation data and returns per-index shard audits.
// searchTotals maps index name to lifetime search count (from _stats);
// used for replica waste detection. Pass nil if search data is unavailable.
func (c *Client) AuditShards(ctx context.Context, searchTotals map[string]int64) ([]ShardAudit, error) {
	shards, err := c.listShards(ctx)
	if err != nil {
		return nil, err
	}

	if searchTotals == nil {
		searchTotals = make(map[string]int64)
	}

	// Group shards by index
	type indexShards struct {
		primaries  []ShardInfo
		replicas   []ShardInfo
		unassigned int
	}
	grouped := make(map[string]*indexShards)
	for _, s := range shards {
		is, ok := grouped[s.Index]
		if !ok {
			is = &indexShards{}
			grouped[s.Index] = is
		}
		if s.State == "UNASSIGNED" {
			is.unassigned++
			continue
		}
		if s.Primary {
			is.primaries = append(is.primaries, s)
		} else {
			is.replicas = append(is.replicas, s)
		}
	}

	result := make([]ShardAudit, 0, len(grouped))
	for name, is := range grouped {
		var totalPrimarySize int64
		var hasOversized bool
		for _, p := range is.primaries {
			totalPrimarySize += p.SizeBytes
			if p.SizeBytes > oversizedShardThresholdBytes {
				hasOversized = true
			}
		}

		var avgShardSize int64
		primaryCount := len(is.primaries)
		if primaryCount > 0 {
			avgShardSize = totalPrimarySize / int64(primaryCount)
		}

		var totalSize int64
		for _, p := range is.primaries {
			totalSize += p.SizeBytes
		}
		for _, r := range is.replicas {
			totalSize += r.SizeBytes
		}

		replicaCount := len(is.replicas)
		replicaWaste := replicaCount > 0 && searchTotals[name] == 0

		result = append(result, ShardAudit{
			Index:           name,
			PrimaryCount:    primaryCount,
			ReplicaCount:    replicaCount,
			UnassignedCount: is.unassigned,
			TotalSizeBytes:  totalSize,
			AvgShardSize:    avgShardSize,
			HasSprawl:       primaryCount > 1 && avgShardSize < shardSprawlThresholdBytes,
			HasOversized:    hasOversized,
			ReplicaWaste:    replicaWaste,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Index < result[j].Index
	})

	return result, nil
}
