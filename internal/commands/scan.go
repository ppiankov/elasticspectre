package commands

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
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
	url, _ := cmd.Flags().GetString("url")
	cloudID, _ := cmd.Flags().GetString("cloud-id")

	if url == "" && cloudID == "" {
		return errors.New("either --url or --cloud-id is required")
	}
	if url != "" && cloudID != "" {
		return errors.New("--url and --cloud-id are mutually exclusive")
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "audit not yet implemented (target: %s)\n", resolveTarget(url, cloudID))
	return nil
}
