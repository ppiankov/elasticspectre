package commands

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const configFileName = ".elasticspectre.yaml"

const sampleConfig = `# elasticspectre configuration
# See: https://github.com/ppiankov/elasticspectre

# Elasticsearch/OpenSearch cluster URL
# url: "http://localhost:9200"

# Elastic Cloud deployment ID (alternative to url)
# cloud_id: ""

# Days without writes before an index is flagged as stale
# stale_days: 90

# Output format: text, json, sarif, spectrehub
# format: text

# Include system/internal indices (e.g. .kibana, .security)
# include_system: false
`

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Generate a sample .elasticspectre.yaml config",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if _, err := os.Stat(configFileName); err == nil {
				return errors.New(configFileName + " already exists")
			}

			if err := os.WriteFile(configFileName, []byte(sampleConfig), 0o644); err != nil {
				return fmt.Errorf("writing %s: %w", configFileName, err)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "created %s\n", configFileName)
			return nil
		},
	}
}
