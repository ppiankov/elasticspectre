package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_FileNotFound(t *testing.T) {
	// Run in empty temp dir — no config file exists.
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	// Clear env vars that could interfere.
	t.Setenv("ELASTICSEARCH_URL", "")
	t.Setenv("OPENSEARCH_URL", "")
	t.Setenv("ELASTIC_CLOUD_ID", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.URL != "" {
		t.Errorf("URL = %q, want empty", cfg.URL)
	}
	if cfg.StaleDays != 0 {
		t.Errorf("StaleDays = %d, want 0 (before defaults)", cfg.StaleDays)
	}
}

func TestLoad_ParsesYAML(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	yaml := `url: "http://localhost:9200"
cloud_id: "my-deploy:abc123"
stale_days: 60
format: json
include_system: true
`
	if err := os.WriteFile(filepath.Join(dir, configFileName), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("ELASTICSEARCH_URL", "")
	t.Setenv("OPENSEARCH_URL", "")
	t.Setenv("ELASTIC_CLOUD_ID", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.URL != "http://localhost:9200" {
		t.Errorf("URL = %q, want http://localhost:9200", cfg.URL)
	}
	if cfg.CloudID != "my-deploy:abc123" {
		t.Errorf("CloudID = %q, want my-deploy:abc123", cfg.CloudID)
	}
	if cfg.StaleDays != 60 {
		t.Errorf("StaleDays = %d, want 60", cfg.StaleDays)
	}
	if cfg.Format != "json" {
		t.Errorf("Format = %q, want json", cfg.Format)
	}
	if !cfg.IncludeSystem {
		t.Error("IncludeSystem = false, want true")
	}
}

func TestLoad_PartialYAML(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	yaml := `url: "http://es:9200"
`
	if err := os.WriteFile(filepath.Join(dir, configFileName), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("ELASTICSEARCH_URL", "")
	t.Setenv("OPENSEARCH_URL", "")
	t.Setenv("ELASTIC_CLOUD_ID", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.URL != "http://es:9200" {
		t.Errorf("URL = %q, want http://es:9200", cfg.URL)
	}
	if cfg.StaleDays != 0 {
		t.Errorf("StaleDays = %d, want 0", cfg.StaleDays)
	}
	if cfg.Format != "" {
		t.Errorf("Format = %q, want empty", cfg.Format)
	}
}

func TestLoad_EnvElasticsearchURL(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	t.Setenv("ELASTICSEARCH_URL", "http://env-es:9200")
	t.Setenv("OPENSEARCH_URL", "")
	t.Setenv("ELASTIC_CLOUD_ID", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.URL != "http://env-es:9200" {
		t.Errorf("URL = %q, want http://env-es:9200", cfg.URL)
	}
}

func TestLoad_EnvOpensearchURL(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	t.Setenv("ELASTICSEARCH_URL", "")
	t.Setenv("OPENSEARCH_URL", "http://env-os:9200")
	t.Setenv("ELASTIC_CLOUD_ID", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.URL != "http://env-os:9200" {
		t.Errorf("URL = %q, want http://env-os:9200", cfg.URL)
	}
}

func TestLoad_EnvElasticCloudID(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	t.Setenv("ELASTICSEARCH_URL", "")
	t.Setenv("OPENSEARCH_URL", "")
	t.Setenv("ELASTIC_CLOUD_ID", "my-cloud:encoded")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.CloudID != "my-cloud:encoded" {
		t.Errorf("CloudID = %q, want my-cloud:encoded", cfg.CloudID)
	}
}

func TestLoad_EnvPriority(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	// ELASTICSEARCH_URL takes priority over OPENSEARCH_URL.
	t.Setenv("ELASTICSEARCH_URL", "http://es-wins:9200")
	t.Setenv("OPENSEARCH_URL", "http://os-loses:9200")
	t.Setenv("ELASTIC_CLOUD_ID", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.URL != "http://es-wins:9200" {
		t.Errorf("URL = %q, want http://es-wins:9200", cfg.URL)
	}
}

func TestLoad_EnvOverridesFile(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	yaml := `url: "http://file-url:9200"
cloud_id: "file-cloud"
`
	if err := os.WriteFile(filepath.Join(dir, configFileName), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("ELASTICSEARCH_URL", "http://env-url:9200")
	t.Setenv("OPENSEARCH_URL", "")
	t.Setenv("ELASTIC_CLOUD_ID", "env-cloud")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.URL != "http://env-url:9200" {
		t.Errorf("URL = %q, want http://env-url:9200 (env override)", cfg.URL)
	}
	if cfg.CloudID != "env-cloud" {
		t.Errorf("CloudID = %q, want env-cloud (env override)", cfg.CloudID)
	}
}

func TestApplyDefaults(t *testing.T) {
	cfg := Config{}
	cfg.ApplyDefaults()

	if cfg.StaleDays != DefaultStaleDays {
		t.Errorf("StaleDays = %d, want %d", cfg.StaleDays, DefaultStaleDays)
	}
	if cfg.Format != DefaultFormat {
		t.Errorf("Format = %q, want %q", cfg.Format, DefaultFormat)
	}
}

func TestApplyDefaults_NoOverwrite(t *testing.T) {
	cfg := Config{
		StaleDays: 30,
		Format:    "json",
	}
	cfg.ApplyDefaults()

	if cfg.StaleDays != 30 {
		t.Errorf("StaleDays = %d, want 30 (preserved)", cfg.StaleDays)
	}
	if cfg.Format != "json" {
		t.Errorf("Format = %q, want json (preserved)", cfg.Format)
	}
}
