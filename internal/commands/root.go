package commands

import (
	"github.com/spf13/cobra"
)

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "elasticspectre",
		Short:         "Elasticsearch/OpenSearch waste auditor",
		Long:          "elasticspectre scans Elasticsearch and OpenSearch clusters to find stale indices, unused templates, oversized shards, and wasted resources.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	flags := rootCmd.PersistentFlags()
	flags.String("url", "", "Elasticsearch/OpenSearch cluster URL")
	flags.String("cloud-id", "", "Elastic Cloud deployment ID")
	flags.Int("stale-days", 90, "days without writes to flag an index as stale")
	flags.String("format", "text", "output format: text, json, sarif, spectrehub")
	flags.Bool("include-system", false, "include system/internal indices in audit")

	rootCmd.AddCommand(newVersionCmd())
	rootCmd.AddCommand(newAuditCmd())
	rootCmd.AddCommand(newInitCmd())

	return rootCmd
}

func Execute() error {
	return NewRootCmd().Execute()
}
