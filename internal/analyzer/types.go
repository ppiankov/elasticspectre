package analyzer

import "github.com/ppiankov/elasticspectre/internal/elastic"

// Severity levels for findings, ordered from most to least critical.
type Severity string

const (
	SeverityHigh   Severity = "high"
	SeverityMedium Severity = "medium"
	SeverityLow    Severity = "low"
	SeverityInfo   Severity = "info"
)

// FindingType identifies the kind of waste or risk detected.
type FindingType string

const (
	StaleIndex         FindingType = "STALE_INDEX"
	NoILMPolicy        FindingType = "NO_ILM_POLICY"
	OpenIndexNoTraffic FindingType = "OPEN_INDEX_NO_TRAFFIC"
	FrozenCandidate    FindingType = "FROZEN_CANDIDATE"
	ShardSprawl        FindingType = "SHARD_SPRAWL"
	OversizedShard     FindingType = "OVERSIZED_SHARD"
	UnassignedShard    FindingType = "UNASSIGNED_SHARD"
	ReplicaWaste       FindingType = "REPLICA_WASTE"
	NoSnapshotPolicy   FindingType = "NO_SNAPSHOT_POLICY"
	NoAuth             FindingType = "NO_AUTH"
)

// Finding represents a single waste or risk observation.
type Finding struct {
	Type                FindingType
	Severity            Severity
	Index               string // empty for cluster-level findings
	Message             string
	StorageSavingsBytes int64 // estimated savings from remediation
	HeapImpactBytes     int64 // estimated heap savings for shard findings
}

// Input aggregates all data the analyzer needs.
type Input struct {
	Indices   []elastic.IndexInfo
	Shards    []elastic.ShardAudit
	Health    elastic.ClusterHealth
	Snapshots elastic.SnapshotPolicyStatus
	Security  elastic.SecurityStatus
	StaleDays int // threshold from --stale-days flag
}
