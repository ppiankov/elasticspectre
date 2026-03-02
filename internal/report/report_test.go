package report

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ppiankov/elasticspectre/internal/analyzer"
)

var testFindings = []FindingOutput{
	{
		Type:                "STALE_INDEX",
		Severity:            "high",
		Index:               "logs-2023",
		Message:             "Index 'logs-2023' has zero writes and searches.",
		StorageSavingsBytes: 5 * 1024 * 1024 * 1024,
	},
	{
		Type:     "NO_ILM_POLICY",
		Severity: "medium",
		Index:    "logs-2024",
		Message:  "Index 'logs-2024' has no lifecycle policy.",
	},
	{
		Type:            "SHARD_SPRAWL",
		Severity:        "medium",
		Index:           "metrics",
		Message:         "Index 'metrics' has too many shards.",
		HeapImpactBytes: 200 * 1024 * 1024,
	},
}

func testData() Data {
	return Data{
		Tool:      "elasticspectre",
		Version:   "0.1.0",
		Timestamp: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		Target: Target{
			Type:    "elasticsearch-cluster",
			URIHash: "abc123",
		},
		Findings: testFindings,
		Summary:  BuildSummary(testFindings),
	}
}

// --- SpectreHubReporter ---

func TestSpectreHubReporter_Schema(t *testing.T) {
	var buf bytes.Buffer
	r := &SpectreHubReporter{}
	if err := r.Generate(&buf, testData()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"schema": "spectre/v1"`) {
		t.Error("output missing schema field")
	}
}

func TestSpectreHubReporter_TargetType(t *testing.T) {
	var buf bytes.Buffer
	r := &SpectreHubReporter{}
	if err := r.Generate(&buf, testData()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"type": "elasticsearch-cluster"`) {
		t.Error("output missing target type")
	}
}

func TestSpectreHubReporter_FindingsPreserved(t *testing.T) {
	var buf bytes.Buffer
	r := &SpectreHubReporter{}
	if err := r.Generate(&buf, testData()); err != nil {
		t.Fatal(err)
	}

	var envelope struct {
		Findings []FindingOutput `json:"findings"`
	}
	if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Findings) != 3 {
		t.Fatalf("expected 3 findings, got %d", len(envelope.Findings))
	}
	if envelope.Findings[0].Type != "STALE_INDEX" {
		t.Errorf("first finding type = %s, want STALE_INDEX", envelope.Findings[0].Type)
	}
	if envelope.Findings[0].Index != "logs-2023" {
		t.Errorf("first finding index = %s, want logs-2023", envelope.Findings[0].Index)
	}
}

func TestSpectreHubReporter_StorageSavingsInSummary(t *testing.T) {
	var buf bytes.Buffer
	r := &SpectreHubReporter{}
	if err := r.Generate(&buf, testData()); err != nil {
		t.Fatal(err)
	}

	var envelope struct {
		Summary Summary `json:"summary"`
	}
	if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Summary.TotalStorageSavings != 5*1024*1024*1024 {
		t.Errorf("TotalStorageSavings = %d, want %d", envelope.Summary.TotalStorageSavings, 5*1024*1024*1024)
	}
}

func TestSpectreHubReporter_GoldenFile(t *testing.T) {
	var buf bytes.Buffer
	r := &SpectreHubReporter{}
	if err := r.Generate(&buf, testData()); err != nil {
		t.Fatal(err)
	}

	golden, err := os.ReadFile("testdata/spectrehub.golden.json")
	if err != nil {
		t.Fatal(err)
	}

	// Compare as normalized JSON to ignore whitespace differences
	var gotJSON, wantJSON interface{}
	if err := json.Unmarshal(buf.Bytes(), &gotJSON); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if err := json.Unmarshal(golden, &wantJSON); err != nil {
		t.Fatalf("unmarshal golden: %v", err)
	}

	gotNorm, _ := json.MarshalIndent(gotJSON, "", "  ")
	wantNorm, _ := json.MarshalIndent(wantJSON, "", "  ")

	if string(gotNorm) != string(wantNorm) {
		t.Errorf("output does not match golden file.\nGot:\n%s\nWant:\n%s", gotNorm, wantNorm)
	}
}

// --- TextReporter ---

func TestTextReporter_Header(t *testing.T) {
	var buf bytes.Buffer
	r := &TextReporter{}
	if err := r.Generate(&buf, testData()); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(buf.String(), "elasticspectre audit report") {
		t.Error("output should start with 'elasticspectre audit report'")
	}
}

func TestTextReporter_FindingsTable(t *testing.T) {
	var buf bytes.Buffer
	r := &TextReporter{}
	if err := r.Generate(&buf, testData()); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"STALE_INDEX", "logs-2023", "NO_ILM_POLICY", "SHARD_SPRAWL", "SEVERITY", "TYPE"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestTextReporter_NoFindings(t *testing.T) {
	data := testData()
	data.Findings = nil
	data.Summary = BuildSummary(nil)

	var buf bytes.Buffer
	r := &TextReporter{}
	if err := r.Generate(&buf, data); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "No findings.") {
		t.Error("expected 'No findings.' for empty findings")
	}
}

// --- BuildSummary ---

func TestBuildSummary(t *testing.T) {
	s := BuildSummary(testFindings)
	if s.TotalFindings != 3 {
		t.Errorf("TotalFindings = %d, want 3", s.TotalFindings)
	}
	if s.TotalStorageSavings != 5*1024*1024*1024 {
		t.Errorf("TotalStorageSavings = %d, want %d", s.TotalStorageSavings, 5*1024*1024*1024)
	}
	if s.TotalHeapSavings != 200*1024*1024 {
		t.Errorf("TotalHeapSavings = %d, want %d", s.TotalHeapSavings, 200*1024*1024)
	}
	if s.BySeverity["high"] != 1 {
		t.Errorf("BySeverity[high] = %d, want 1", s.BySeverity["high"])
	}
	if s.BySeverity["medium"] != 2 {
		t.Errorf("BySeverity[medium] = %d, want 2", s.BySeverity["medium"])
	}
}

// --- NewData ---

func TestNewData(t *testing.T) {
	analyzerFindings := []analyzer.Finding{
		{
			Type:                analyzer.StaleIndex,
			Severity:            analyzer.SeverityHigh,
			Index:               "logs-old",
			Message:             "Stale index found.",
			StorageSavingsBytes: 1024,
		},
	}

	target := Target{Type: "elasticsearch-cluster", URIHash: "def456"}
	data := NewData("elasticspectre", "0.2.0", target, analyzerFindings)

	if data.Tool != "elasticspectre" {
		t.Errorf("Tool = %s, want elasticspectre", data.Tool)
	}
	if data.Version != "0.2.0" {
		t.Errorf("Version = %s, want 0.2.0", data.Version)
	}
	if len(data.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(data.Findings))
	}
	if data.Findings[0].Type != "STALE_INDEX" {
		t.Errorf("finding type = %s, want STALE_INDEX", data.Findings[0].Type)
	}
	if data.Findings[0].Severity != "high" {
		t.Errorf("finding severity = %s, want high", data.Findings[0].Severity)
	}
	if data.Findings[0].StorageSavingsBytes != 1024 {
		t.Errorf("StorageSavingsBytes = %d, want 1024", data.Findings[0].StorageSavingsBytes)
	}
	if data.Summary.TotalFindings != 1 {
		t.Errorf("Summary.TotalFindings = %d, want 1", data.Summary.TotalFindings)
	}
}
