package elastic

import (
	"context"
	"fmt"
	"testing"
)

func shardEntry(index string, shard int, prirep, state, store, node string) map[string]any {
	return map[string]any{
		"index":  index,
		"shard":  fmt.Sprintf("%d", shard),
		"prirep": prirep,
		"state":  state,
		"store":  store,
		"node":   node,
	}
}

// --- listShards tests ---

func TestListShards_BasicParsing(t *testing.T) {
	catShards := []map[string]any{
		shardEntry("logs-2024", 0, "p", "STARTED", "25gb", "node-1"),
		shardEntry("logs-2024", 0, "r", "STARTED", "25gb", "node-2"),
		shardEntry("logs-2024", 1, "p", "STARTED", "20gb", "node-1"),
	}
	server := newMockServer(t, map[string]any{"/_cat/shards": catShards})
	defer server.Close()

	shards, err := newTestClient(t, server).listShards(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(shards) != 3 {
		t.Fatalf("expected 3 shards, got %d", len(shards))
	}
	if !shards[0].Primary {
		t.Error("shard 0 should be primary")
	}
	if shards[1].Primary {
		t.Error("shard 1 should be replica")
	}
	if shards[0].SizeBytes != 25*1024*1024*1024 {
		t.Errorf("shard 0 size = %d, want %d", shards[0].SizeBytes, 25*1024*1024*1024)
	}
}

func TestListShards_UnassignedShard(t *testing.T) {
	catShards := []map[string]any{
		shardEntry("logs-2024", 0, "p", "STARTED", "10gb", "node-1"),
		shardEntry("logs-2024", 0, "r", "UNASSIGNED", "", ""),
	}
	server := newMockServer(t, map[string]any{"/_cat/shards": catShards})
	defer server.Close()

	shards, err := newTestClient(t, server).listShards(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(shards) != 2 {
		t.Fatalf("expected 2 shards, got %d", len(shards))
	}
	unassigned := shards[1]
	if unassigned.State != "UNASSIGNED" {
		t.Errorf("state = %q, want UNASSIGNED", unassigned.State)
	}
	if unassigned.SizeBytes != 0 {
		t.Errorf("unassigned shard size = %d, want 0", unassigned.SizeBytes)
	}
	if unassigned.Node != "" {
		t.Errorf("unassigned shard node = %q, want empty", unassigned.Node)
	}
}

func TestListShards_EmptyCluster(t *testing.T) {
	server := newMockServer(t, map[string]any{"/_cat/shards": []map[string]any{}})
	defer server.Close()

	shards, err := newTestClient(t, server).listShards(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(shards) != 0 {
		t.Errorf("expected 0 shards, got %d", len(shards))
	}
}

// --- AuditShards tests ---

func TestAuditShards_ShardSprawl(t *testing.T) {
	// 5 primaries at 1GB each → avg 1GB < 10GB threshold → sprawl
	var catShards []map[string]any
	for i := 0; i < 5; i++ {
		catShards = append(catShards, shardEntry("logs-2024", i, "p", "STARTED", "1gb", "node-1"))
	}
	server := newMockServer(t, map[string]any{"/_cat/shards": catShards})
	defer server.Close()

	audits, err := newTestClient(t, server).AuditShards(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(audits) != 1 {
		t.Fatalf("expected 1 audit, got %d", len(audits))
	}
	if !audits[0].HasSprawl {
		t.Error("expected HasSprawl=true for 5 shards at 1GB each")
	}
}

func TestAuditShards_NoSprawlSingleShard(t *testing.T) {
	// Single primary at 500MB → single-shard index is exempt from sprawl
	catShards := []map[string]any{
		shardEntry("small-index", 0, "p", "STARTED", "500mb", "node-1"),
	}
	server := newMockServer(t, map[string]any{"/_cat/shards": catShards})
	defer server.Close()

	audits, err := newTestClient(t, server).AuditShards(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if audits[0].HasSprawl {
		t.Error("expected HasSprawl=false for single-shard index")
	}
}

func TestAuditShards_Oversized(t *testing.T) {
	// 1 primary at 55GB → oversized
	catShards := []map[string]any{
		shardEntry("big-index", 0, "p", "STARTED", "55gb", "node-1"),
	}
	server := newMockServer(t, map[string]any{"/_cat/shards": catShards})
	defer server.Close()

	audits, err := newTestClient(t, server).AuditShards(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !audits[0].HasOversized {
		t.Error("expected HasOversized=true for 55GB shard")
	}
}

func TestAuditShards_UnassignedCount(t *testing.T) {
	catShards := []map[string]any{
		shardEntry("logs-2024", 0, "p", "STARTED", "10gb", "node-1"),
		shardEntry("logs-2024", 0, "r", "UNASSIGNED", "", ""),
		shardEntry("logs-2024", 1, "p", "UNASSIGNED", "", ""),
	}
	server := newMockServer(t, map[string]any{"/_cat/shards": catShards})
	defer server.Close()

	audits, err := newTestClient(t, server).AuditShards(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if audits[0].UnassignedCount != 2 {
		t.Errorf("unassigned = %d, want 2", audits[0].UnassignedCount)
	}
}

func TestAuditShards_ReplicaWaste(t *testing.T) {
	catShards := []map[string]any{
		shardEntry("logs-2024", 0, "p", "STARTED", "10gb", "node-1"),
		shardEntry("logs-2024", 0, "r", "STARTED", "10gb", "node-2"),
	}
	server := newMockServer(t, map[string]any{"/_cat/shards": catShards})
	defer server.Close()

	// Zero searches → replica waste
	audits, err := newTestClient(t, server).AuditShards(context.Background(), map[string]int64{"logs-2024": 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !audits[0].ReplicaWaste {
		t.Error("expected ReplicaWaste=true when searchTotal=0")
	}
}

func TestAuditShards_ReplicaNoWaste(t *testing.T) {
	catShards := []map[string]any{
		shardEntry("logs-2024", 0, "p", "STARTED", "10gb", "node-1"),
		shardEntry("logs-2024", 0, "r", "STARTED", "10gb", "node-2"),
	}
	server := newMockServer(t, map[string]any{"/_cat/shards": catShards})
	defer server.Close()

	// Has searches → no replica waste
	audits, err := newTestClient(t, server).AuditShards(context.Background(), map[string]int64{"logs-2024": 500})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if audits[0].ReplicaWaste {
		t.Error("expected ReplicaWaste=false when searchTotal=500")
	}
}

func TestAuditShards_DeterministicOrder(t *testing.T) {
	catShards := []map[string]any{
		shardEntry("zebra-index", 0, "p", "STARTED", "10gb", "node-1"),
		shardEntry("alpha-index", 0, "p", "STARTED", "10gb", "node-1"),
		shardEntry("middle-index", 0, "p", "STARTED", "10gb", "node-1"),
	}
	server := newMockServer(t, map[string]any{"/_cat/shards": catShards})
	defer server.Close()

	audits, err := newTestClient(t, server).AuditShards(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(audits) != 3 {
		t.Fatalf("expected 3 audits, got %d", len(audits))
	}
	if audits[0].Index != "alpha-index" || audits[1].Index != "middle-index" || audits[2].Index != "zebra-index" {
		t.Errorf("expected alphabetical order, got %s, %s, %s", audits[0].Index, audits[1].Index, audits[2].Index)
	}
}
