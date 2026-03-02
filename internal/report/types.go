package report

import (
	"io"
	"time"

	"github.com/ppiankov/elasticspectre/internal/analyzer"
)

// Reporter generates output from report data.
type Reporter interface {
	Generate(w io.Writer, data Data) error
}

// Data holds all information needed to generate a report.
type Data struct {
	Tool      string          `json:"tool"`
	Version   string          `json:"version"`
	Timestamp time.Time       `json:"timestamp"`
	Target    Target          `json:"target"`
	Findings  []FindingOutput `json:"findings"`
	Summary   Summary         `json:"summary"`
}

// Target identifies the cluster being audited.
type Target struct {
	Type    string `json:"type"`     // "elasticsearch-cluster"
	URIHash string `json:"uri_hash"` // SHA256 of cluster URL
}

// FindingOutput is the JSON-serializable representation of a finding.
type FindingOutput struct {
	Type                string `json:"type"`
	Severity            string `json:"severity"`
	Index               string `json:"index,omitempty"`
	Message             string `json:"message"`
	StorageSavingsBytes int64  `json:"storage_savings_bytes,omitempty"`
	HeapImpactBytes     int64  `json:"heap_impact_bytes,omitempty"`
}

// Summary aggregates findings by severity and totals.
type Summary struct {
	TotalFindings       int            `json:"total_findings"`
	TotalStorageSavings int64          `json:"total_storage_savings_bytes"`
	TotalHeapSavings    int64          `json:"total_heap_savings_bytes"`
	BySeverity          map[string]int `json:"by_severity"`
}

// NewData converts analyzer findings into report data.
func NewData(tool, version string, target Target, findings []analyzer.Finding) Data {
	outputs := make([]FindingOutput, len(findings))
	for i, f := range findings {
		outputs[i] = FindingOutput{
			Type:                string(f.Type),
			Severity:            string(f.Severity),
			Index:               f.Index,
			Message:             f.Message,
			StorageSavingsBytes: f.StorageSavingsBytes,
			HeapImpactBytes:     f.HeapImpactBytes,
		}
	}

	return Data{
		Tool:      tool,
		Version:   version,
		Timestamp: time.Now().UTC(),
		Target:    target,
		Findings:  outputs,
		Summary:   BuildSummary(outputs),
	}
}

// BuildSummary aggregates totals from a slice of findings.
func BuildSummary(findings []FindingOutput) Summary {
	s := Summary{
		TotalFindings: len(findings),
		BySeverity:    make(map[string]int),
	}
	for _, f := range findings {
		s.TotalStorageSavings += f.StorageSavingsBytes
		s.TotalHeapSavings += f.HeapImpactBytes
		s.BySeverity[f.Severity]++
	}
	return s
}
