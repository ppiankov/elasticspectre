package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const configFileName = ".elasticspectre.yaml"

// Default values applied when config fields are zero.
const (
	DefaultStaleDays = 90
	DefaultFormat    = "text"
)

// Config holds all resolved configuration values.
type Config struct {
	URL           string `yaml:"url"`
	CloudID       string `yaml:"cloud_id"`
	StaleDays     int    `yaml:"stale_days"`
	Format        string `yaml:"format"`
	IncludeSystem bool   `yaml:"include_system"`
}

// Load reads config from YAML file and environment variables.
// File search order: CWD, then home directory. Missing file is not an error.
// Env vars override file values: ELASTICSEARCH_URL, OPENSEARCH_URL, ELASTIC_CLOUD_ID.
func Load() (Config, error) {
	var cfg Config

	if err := loadFile(&cfg); err != nil {
		return cfg, err
	}

	applyEnv(&cfg)

	return cfg, nil
}

// ApplyDefaults sets default values for zero-value fields.
func (c *Config) ApplyDefaults() {
	if c.StaleDays == 0 {
		c.StaleDays = DefaultStaleDays
	}
	if c.Format == "" {
		c.Format = DefaultFormat
	}
}

// loadFile tries CWD then home directory for the config file.
func loadFile(cfg *Config) error {
	// Try CWD first.
	data, err := os.ReadFile(configFileName)
	if err == nil {
		return yaml.Unmarshal(data, cfg)
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	// Try home directory.
	home, err := os.UserHomeDir()
	if err != nil {
		return nil // Can't find home dir — skip file loading.
	}

	data, err = os.ReadFile(filepath.Join(home, configFileName))
	if err == nil {
		return yaml.Unmarshal(data, cfg)
	}
	if errors.Is(err, fs.ErrNotExist) {
		return nil // No config file — not an error.
	}
	return err
}

// applyEnv overrides config with non-empty environment variables.
func applyEnv(cfg *Config) {
	if v := os.Getenv("ELASTICSEARCH_URL"); v != "" {
		cfg.URL = v
	} else if v := os.Getenv("OPENSEARCH_URL"); v != "" {
		cfg.URL = v
	}

	if v := os.Getenv("ELASTIC_CLOUD_ID"); v != "" {
		cfg.CloudID = v
	}
}
