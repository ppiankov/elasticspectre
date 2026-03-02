package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func executeCommand(args ...string) (string, error) {
	cmd := NewRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func TestVersionOutput(t *testing.T) {
	Version = "1.2.3"
	Commit = "abc1234"
	Date = "2025-01-01T00:00:00Z"

	out, err := executeCommand("version")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"1.2.3", "abc1234", "2025-01-01T00:00:00Z"} {
		if !strings.Contains(out, want) {
			t.Errorf("output %q missing %q", out, want)
		}
	}
}

func TestAuditHelpShowsFlags(t *testing.T) {
	out, err := executeCommand("audit", "--help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, flag := range []string{"--url", "--cloud-id", "--stale-days", "--format", "--include-system"} {
		if !strings.Contains(out, flag) {
			t.Errorf("audit --help missing flag %q", flag)
		}
	}
}

func TestAuditRequiresConnection(t *testing.T) {
	t.Setenv("ELASTICSEARCH_URL", "")
	t.Setenv("OPENSEARCH_URL", "")
	t.Setenv("ELASTIC_CLOUD_ID", "")

	_, err := executeCommand("audit")
	if err == nil {
		t.Fatal("expected error when neither --url nor --cloud-id provided")
	}
	if !strings.Contains(err.Error(), "--url or --cloud-id") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAuditRejectsBothConnections(t *testing.T) {
	_, err := executeCommand("audit", "--url", "http://localhost:9200", "--cloud-id", "my-deploy")
	if err == nil {
		t.Fatal("expected error when both --url and --cloud-id provided")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAuditInvalidFormat(t *testing.T) {
	_, err := executeCommand("audit", "--url", "http://localhost:9200", "--format", "invalid")
	if err == nil {
		t.Fatal("expected error for unsupported format")
	}
	if !strings.Contains(err.Error(), "unsupported format") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAuditConnectError(t *testing.T) {
	// Use a port that nothing is listening on.
	_, err := executeCommand("audit", "--url", "http://127.0.0.1:1")
	if err == nil {
		t.Fatal("expected connection error")
	}
	if !strings.Contains(err.Error(), "connecting to cluster") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestInitCreatesConfig(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	out, err := executeCommand("init")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "created") {
		t.Errorf("expected 'created' message, got %q", out)
	}

	data, err := os.ReadFile(filepath.Join(dir, configFileName))
	if err != nil {
		t.Fatalf("config file not found: %v", err)
	}
	content := string(data)
	for _, want := range []string{"url:", "cloud_id:", "stale_days:", "format:", "include_system:"} {
		if !strings.Contains(content, want) {
			t.Errorf("config missing %q", want)
		}
	}
}

func TestInitFailsIfExists(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	if err := os.WriteFile(configFileName, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := executeCommand("init")
	if err == nil {
		t.Fatal("expected error when config already exists")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestResolveTarget(t *testing.T) {
	if got := resolveTarget("http://localhost:9200", ""); got != "http://localhost:9200" {
		t.Errorf("expected URL, got %q", got)
	}
	if got := resolveTarget("", "cloud123"); got != "cloud123" {
		t.Errorf("expected cloud ID, got %q", got)
	}
}
