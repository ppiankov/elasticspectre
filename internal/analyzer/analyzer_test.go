package analyzer

import (
	"testing"

	"github.com/ppiankov/elasticspectre/internal/elastic"
)

func emptyInput() Input {
	return Input{
		Snapshots: elastic.SnapshotPolicyStatus{HasPolicy: true},
		Security:  elastic.SecurityStatus{AuthEnabled: true},
		StaleDays: 90,
	}
}

func findByType(findings []Finding, ft FindingType) *Finding {
	for i := range findings {
		if findings[i].Type == ft {
			return &findings[i]
		}
	}
	return nil
}

func findByTypeAndIndex(findings []Finding, ft FindingType, index string) *Finding {
	for i := range findings {
		if findings[i].Type == ft && findings[i].Index == index {
			return &findings[i]
		}
	}
	return nil
}

// --- Index findings ---

func TestAnalyze_StaleIndex(t *testing.T) {
	input := emptyInput()
	input.Indices = []elastic.IndexInfo{
		{Name: "logs-2023", Status: "open", DocsCount: 1000, StoreSizeBytes: 5 * 1024 * 1024 * 1024, IndexTotal: 0, SearchTotal: 0, HasILMPolicy: true},
	}

	findings := Analyze(input)
	f := findByType(findings, StaleIndex)
	if f == nil {
		t.Fatal("expected STALE_INDEX finding")
	}
	if f.StorageSavingsBytes != 5*1024*1024*1024 {
		t.Errorf("StorageSavingsBytes = %d, want %d", f.StorageSavingsBytes, 5*1024*1024*1024)
	}
	if f.Severity != SeverityHigh {
		t.Errorf("severity = %s, want high", f.Severity)
	}
}

func TestAnalyze_StaleIndex_Closed(t *testing.T) {
	input := emptyInput()
	input.Indices = []elastic.IndexInfo{
		{Name: "old-logs", Status: "close", IndexTotal: 0, SearchTotal: 0, HasILMPolicy: true},
	}

	findings := Analyze(input)
	if f := findByType(findings, StaleIndex); f != nil {
		t.Error("should not flag closed index as stale")
	}
}

func TestAnalyze_StaleIndex_HasTraffic(t *testing.T) {
	input := emptyInput()
	input.Indices = []elastic.IndexInfo{
		{Name: "active-logs", Status: "open", IndexTotal: 500, SearchTotal: 100, HasILMPolicy: true},
	}

	findings := Analyze(input)
	if f := findByType(findings, StaleIndex); f != nil {
		t.Error("should not flag index with traffic as stale")
	}
}

func TestAnalyze_NoILMPolicy(t *testing.T) {
	input := emptyInput()
	input.Indices = []elastic.IndexInfo{
		{Name: "logs-2024", Status: "open", IndexTotal: 100, SearchTotal: 50, HasILMPolicy: false},
	}

	findings := Analyze(input)
	f := findByType(findings, NoILMPolicy)
	if f == nil {
		t.Fatal("expected NO_ILM_POLICY finding")
	}
	if f.Severity != SeverityMedium {
		t.Errorf("severity = %s, want medium", f.Severity)
	}
}

func TestAnalyze_NoILMPolicy_HasPolicy(t *testing.T) {
	input := emptyInput()
	input.Indices = []elastic.IndexInfo{
		{Name: "logs-2024", Status: "open", HasILMPolicy: true},
	}

	findings := Analyze(input)
	if f := findByType(findings, NoILMPolicy); f != nil {
		t.Error("should not flag index with ILM policy")
	}
}

func TestAnalyze_FrozenCandidate(t *testing.T) {
	input := emptyInput()
	input.Indices = []elastic.IndexInfo{
		{Name: "logs-warm", Status: "open", IndexTotal: 500, SearchTotal: 0, HasILMPolicy: true, TierPreference: "data_warm"},
	}

	findings := Analyze(input)
	f := findByType(findings, FrozenCandidate)
	if f == nil {
		t.Fatal("expected FROZEN_CANDIDATE finding")
	}
	if f.Severity != SeverityLow {
		t.Errorf("severity = %s, want low", f.Severity)
	}
}

func TestAnalyze_FrozenCandidate_HasSearches(t *testing.T) {
	input := emptyInput()
	input.Indices = []elastic.IndexInfo{
		{Name: "logs-warm", Status: "open", SearchTotal: 100, HasILMPolicy: true, TierPreference: "data_warm"},
	}

	findings := Analyze(input)
	if f := findByType(findings, FrozenCandidate); f != nil {
		t.Error("should not flag warm index with searches")
	}
}

// --- Shard findings ---

func TestAnalyze_ShardSprawl(t *testing.T) {
	input := emptyInput()
	input.Shards = []elastic.ShardAudit{
		{Index: "logs-2024", PrimaryCount: 10, ReplicaCount: 0, TotalSizeBytes: 10 * 1024 * 1024 * 1024, AvgShardSize: 1024 * 1024 * 1024, HasSprawl: true},
	}

	findings := Analyze(input)
	f := findByType(findings, ShardSprawl)
	if f == nil {
		t.Fatal("expected SHARD_SPRAWL finding")
	}
	if f.HeapImpactBytes <= 0 {
		t.Errorf("expected positive heap impact, got %d", f.HeapImpactBytes)
	}
	if f.Severity != SeverityMedium {
		t.Errorf("severity = %s, want medium", f.Severity)
	}
}

func TestAnalyze_OversizedShard(t *testing.T) {
	input := emptyInput()
	input.Shards = []elastic.ShardAudit{
		{Index: "big-index", PrimaryCount: 1, HasOversized: true},
	}

	findings := Analyze(input)
	f := findByType(findings, OversizedShard)
	if f == nil {
		t.Fatal("expected OVERSIZED_SHARD finding")
	}
	if f.Severity != SeverityMedium {
		t.Errorf("severity = %s, want medium", f.Severity)
	}
}

func TestAnalyze_UnassignedShard(t *testing.T) {
	input := emptyInput()
	input.Shards = []elastic.ShardAudit{
		{Index: "logs-2024", UnassignedCount: 3},
	}

	findings := Analyze(input)
	f := findByType(findings, UnassignedShard)
	if f == nil {
		t.Fatal("expected UNASSIGNED_SHARD finding")
	}
	if f.Severity != SeverityHigh {
		t.Errorf("severity = %s, want high", f.Severity)
	}
}

func TestAnalyze_ReplicaWaste(t *testing.T) {
	input := emptyInput()
	input.Shards = []elastic.ShardAudit{
		{Index: "logs-2024", PrimaryCount: 1, ReplicaCount: 1, TotalSizeBytes: 20 * 1024 * 1024 * 1024, ReplicaWaste: true},
	}

	findings := Analyze(input)
	f := findByType(findings, ReplicaWaste)
	if f == nil {
		t.Fatal("expected REPLICA_WASTE finding")
	}
	if f.StorageSavingsBytes != 10*1024*1024*1024 {
		t.Errorf("StorageSavingsBytes = %d, want %d", f.StorageSavingsBytes, 10*1024*1024*1024)
	}
	if f.Severity != SeverityLow {
		t.Errorf("severity = %s, want low", f.Severity)
	}
}

// --- Cluster findings ---

func TestAnalyze_NoSnapshotPolicy(t *testing.T) {
	input := emptyInput()
	input.Snapshots = elastic.SnapshotPolicyStatus{HasPolicy: false}

	findings := Analyze(input)
	f := findByType(findings, NoSnapshotPolicy)
	if f == nil {
		t.Fatal("expected NO_SNAPSHOT_POLICY finding")
	}
	if f.Severity != SeverityHigh {
		t.Errorf("severity = %s, want high", f.Severity)
	}
	if f.Index != "" {
		t.Errorf("cluster finding should have empty index, got %q", f.Index)
	}
}

func TestAnalyze_NoAuth(t *testing.T) {
	input := emptyInput()
	input.Security = elastic.SecurityStatus{AuthEnabled: false}

	findings := Analyze(input)
	f := findByType(findings, NoAuth)
	if f == nil {
		t.Fatal("expected NO_AUTH finding")
	}
	if f.Severity != SeverityHigh {
		t.Errorf("severity = %s, want high", f.Severity)
	}
}

// --- Edge cases ---

func TestAnalyze_EmptyInput(t *testing.T) {
	input := emptyInput()
	findings := Analyze(input)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for empty input, got %d", len(findings))
	}
}

func TestAnalyze_DeterministicOrder(t *testing.T) {
	input := emptyInput()
	input.Snapshots = elastic.SnapshotPolicyStatus{HasPolicy: false}
	input.Indices = []elastic.IndexInfo{
		{Name: "zebra", Status: "open", IndexTotal: 100, HasILMPolicy: false},
		{Name: "alpha", Status: "open", IndexTotal: 100, HasILMPolicy: false},
	}

	findings := Analyze(input)
	if len(findings) < 3 {
		t.Fatalf("expected at least 3 findings, got %d", len(findings))
	}
	// High severity (NO_SNAPSHOT_POLICY) should come before medium (NO_ILM_POLICY)
	if findings[0].Severity != SeverityHigh {
		t.Errorf("first finding should be high severity, got %s", findings[0].Severity)
	}
	// Within same severity, alpha before zebra
	alphaILM := findByTypeAndIndex(findings, NoILMPolicy, "alpha")
	zebraILM := findByTypeAndIndex(findings, NoILMPolicy, "zebra")
	if alphaILM == nil || zebraILM == nil {
		t.Fatal("expected NO_ILM_POLICY findings for both indices")
	}
	// Find positions
	var alphaPos, zebraPos int
	for i, f := range findings {
		if f.Type == NoILMPolicy && f.Index == "alpha" {
			alphaPos = i
		}
		if f.Type == NoILMPolicy && f.Index == "zebra" {
			zebraPos = i
		}
	}
	if alphaPos >= zebraPos {
		t.Errorf("alpha (pos %d) should come before zebra (pos %d)", alphaPos, zebraPos)
	}
}

// --- formatBytes ---

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1024 * 1024, "1.0 MB"},
		{int64(1.5 * 1024 * 1024), "1.5 MB"},
		{1024 * 1024 * 1024, "1.0 GB"},
		{int64(2.5 * 1024 * 1024 * 1024), "2.5 GB"},
		{1024 * 1024 * 1024 * 1024, "1.0 TB"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatBytes(tt.input)
			if got != tt.want {
				t.Errorf("formatBytes(%d) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
