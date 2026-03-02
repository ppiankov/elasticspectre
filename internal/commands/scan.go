package commands

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ppiankov/elasticspectre/internal/analyzer"
	"github.com/ppiankov/elasticspectre/internal/config"
	"github.com/ppiankov/elasticspectre/internal/elastic"
	"github.com/ppiankov/elasticspectre/internal/report"
)

func newAuditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "audit",
		Short: "Audit Elasticsearch/OpenSearch cluster for waste",
		Long:  "Scan an Elasticsearch or OpenSearch cluster to find stale indices, unused templates, oversized shards, and wasted resources.",
		RunE:  runAudit,
	}
}

func runAudit(cmd *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	mergeFlags(cmd, &cfg)
	cfg.ApplyDefaults()

	if cfg.URL == "" && cfg.CloudID == "" {
		return errors.New("either --url or --cloud-id is required")
	}
	if cfg.URL != "" && cfg.CloudID != "" {
		return errors.New("--url and --cloud-id are mutually exclusive")
	}

	reporter, err := selectReporter(cfg.Format)
	if err != nil {
		return err
	}

	client, err := elastic.New(elastic.Options{
		URL:     cfg.URL,
		CloudID: cfg.CloudID,
	})
	if err != nil {
		return fmt.Errorf("creating client: %w", err)
	}

	ctx := context.Background()

	info, err := client.Info(ctx)
	if err != nil {
		return fmt.Errorf("connecting to cluster: %w", err)
	}

	indices, err := client.AuditIndices(ctx, cfg.IncludeSystem)
	if err != nil {
		return fmt.Errorf("auditing indices: %w", err)
	}

	searchTotals := make(map[string]int64, len(indices))
	for _, idx := range indices {
		searchTotals[idx.Name] = idx.SearchTotal
	}

	shards, err := client.AuditShards(ctx, searchTotals)
	if err != nil {
		return fmt.Errorf("auditing shards: %w", err)
	}

	health, err := client.Health(ctx)
	if err != nil {
		return fmt.Errorf("checking cluster health: %w", err)
	}

	snapshots, err := client.CheckSnapshotPolicies(ctx, info.Flavor)
	if err != nil {
		return fmt.Errorf("checking snapshot policies: %w", err)
	}

	security, err := client.CheckSecurity(ctx, info.Flavor)
	if err != nil {
		return fmt.Errorf("checking security: %w", err)
	}

	findings := analyzer.Analyze(analyzer.Input{
		Indices:   indices,
		Shards:    shards,
		Health:    health,
		Snapshots: snapshots,
		Security:  security,
		StaleDays: cfg.StaleDays,
	})

	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(client.BaseURL())))
	target := report.Target{
		Type:    "elasticsearch-cluster",
		URIHash: hash,
	}

	data := report.NewData("elasticspectre", Version, target, findings)
	return reporter.Generate(cmd.OutOrStdout(), data)
}

// mergeFlags overrides config with explicitly set CLI flags.
func mergeFlags(cmd *cobra.Command, cfg *config.Config) {
	if cmd.Flags().Changed("url") {
		cfg.URL, _ = cmd.Flags().GetString("url")
	}
	if cmd.Flags().Changed("cloud-id") {
		cfg.CloudID, _ = cmd.Flags().GetString("cloud-id")
	}
	if cmd.Flags().Changed("stale-days") {
		cfg.StaleDays, _ = cmd.Flags().GetInt("stale-days")
	}
	if cmd.Flags().Changed("format") {
		cfg.Format, _ = cmd.Flags().GetString("format")
	}
	if cmd.Flags().Changed("include-system") {
		cfg.IncludeSystem, _ = cmd.Flags().GetBool("include-system")
	}
}

// selectReporter returns the appropriate reporter for the given format.
func selectReporter(format string) (report.Reporter, error) {
	switch format {
	case "text":
		return &report.TextReporter{}, nil
	case "json", "spectrehub":
		return &report.SpectreHubReporter{}, nil
	default:
		return nil, fmt.Errorf("unsupported format: %q (use text, json, or spectrehub)", format)
	}
}
