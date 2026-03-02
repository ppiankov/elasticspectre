package analyzer

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ppiankov/elasticspectre/internal/elastic"
)

const (
	heapPerShardBytes   int64 = 25 * 1024 * 1024        // 25 MB per shard
	idealShardSizeBytes int64 = 30 * 1024 * 1024 * 1024 // 30 GB target per shard
)

// severityRank maps severity to sort order (lower = more critical).
var severityRank = map[Severity]int{
	SeverityHigh:   0,
	SeverityMedium: 1,
	SeverityLow:    2,
	SeverityInfo:   3,
}

// Analyze classifies input data into actionable findings.
func Analyze(input Input) []Finding {
	var findings []Finding

	findings = append(findings, analyzeIndices(input.Indices)...)
	findings = append(findings, analyzeShards(input.Shards)...)
	findings = append(findings, analyzeCluster(input.Health, input.Snapshots, input.Security)...)

	sort.Slice(findings, func(i, j int) bool {
		ri, rj := severityRank[findings[i].Severity], severityRank[findings[j].Severity]
		if ri != rj {
			return ri < rj
		}
		return findings[i].Index < findings[j].Index
	})

	return findings
}

func analyzeIndices(indices []elastic.IndexInfo) []Finding {
	var findings []Finding
	for _, idx := range indices {
		if idx.Status != "open" {
			continue
		}

		isStale := idx.IndexTotal == 0 && idx.SearchTotal == 0

		if isStale {
			findings = append(findings, Finding{
				Type:                StaleIndex,
				Severity:            SeverityHigh,
				Index:               idx.Name,
				Message:             fmt.Sprintf("Index '%s' has zero writes and searches. Delete to reclaim %s.", idx.Name, formatBytes(idx.StoreSizeBytes)),
				StorageSavingsBytes: idx.StoreSizeBytes,
			})
			// Skip OPEN_INDEX_NO_TRAFFIC — STALE_INDEX already covers it
		}

		if !idx.HasILMPolicy {
			findings = append(findings, Finding{
				Type:     NoILMPolicy,
				Severity: SeverityMedium,
				Index:    idx.Name,
				Message:  fmt.Sprintf("Index '%s' has no lifecycle policy. Configure ILM/ISM to automate retention.", idx.Name),
			})
		}

		if !isStale && idx.SearchTotal == 0 && strings.Contains(idx.TierPreference, "warm") {
			findings = append(findings, Finding{
				Type:     FrozenCandidate,
				Severity: SeverityLow,
				Index:    idx.Name,
				Message:  fmt.Sprintf("Index '%s' is on warm tier with zero searches. Consider moving to frozen tier.", idx.Name),
			})
		}
	}
	return findings
}

func analyzeShards(shards []elastic.ShardAudit) []Finding {
	var findings []Finding
	for _, sa := range shards {
		if sa.UnassignedCount > 0 {
			findings = append(findings, Finding{
				Type:     UnassignedShard,
				Severity: SeverityHigh,
				Index:    sa.Index,
				Message:  fmt.Sprintf("Index '%s' has %d unassigned shard(s). Check cluster allocation and disk space.", sa.Index, sa.UnassignedCount),
			})
		}

		if sa.HasSprawl {
			idealCount := sa.TotalSizeBytes / idealShardSizeBytes
			if idealCount < 1 {
				idealCount = 1
			}
			excessShards := int64(sa.PrimaryCount) - idealCount
			if excessShards < 0 {
				excessShards = 0
			}
			heapSavings := heapPerShardBytes * excessShards

			findings = append(findings, Finding{
				Type:            ShardSprawl,
				Severity:        SeverityMedium,
				Index:           sa.Index,
				Message:         fmt.Sprintf("Index '%s' has %d primary shards averaging %s each. Merge to reduce heap by %s.", sa.Index, sa.PrimaryCount, formatBytes(sa.AvgShardSize), formatBytes(heapSavings)),
				HeapImpactBytes: heapSavings,
			})
		}

		if sa.HasOversized {
			findings = append(findings, Finding{
				Type:     OversizedShard,
				Severity: SeverityMedium,
				Index:    sa.Index,
				Message:  fmt.Sprintf("Index '%s' has shards exceeding 50 GB. Split the index to improve performance.", sa.Index),
			})
		}

		if sa.ReplicaWaste {
			var replicaBytes int64
			totalShards := sa.PrimaryCount + sa.ReplicaCount
			if totalShards > 0 {
				replicaBytes = sa.TotalSizeBytes * int64(sa.ReplicaCount) / int64(totalShards)
			}
			findings = append(findings, Finding{
				Type:                ReplicaWaste,
				Severity:            SeverityLow,
				Index:               sa.Index,
				Message:             fmt.Sprintf("Index '%s' has %d replica(s) but zero search traffic. Remove replicas to reclaim %s.", sa.Index, sa.ReplicaCount, formatBytes(replicaBytes)),
				StorageSavingsBytes: replicaBytes,
			})
		}
	}
	return findings
}

func analyzeCluster(health elastic.ClusterHealth, snapshots elastic.SnapshotPolicyStatus, security elastic.SecurityStatus) []Finding {
	var findings []Finding

	if !snapshots.HasPolicy {
		findings = append(findings, Finding{
			Type:     NoSnapshotPolicy,
			Severity: SeverityHigh,
			Message:  "Cluster has no snapshot lifecycle policies. Configure SLM to protect against data loss.",
		})
	}

	if !security.AuthEnabled {
		findings = append(findings, Finding{
			Type:     NoAuth,
			Severity: SeverityHigh,
			Message:  "Cluster has no authentication enabled. Enable security to prevent unauthorized access.",
		})
	}

	return findings
}

// formatBytes converts bytes to a human-readable string.
func formatBytes(b int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
		tb = 1024 * gb
	)

	switch {
	case b >= tb:
		return fmt.Sprintf("%.1f TB", float64(b)/float64(tb))
	case b >= gb:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(gb))
	case b >= mb:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(kb))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
