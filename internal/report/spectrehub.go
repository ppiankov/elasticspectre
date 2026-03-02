package report

import (
	"encoding/json"
	"io"
)

// spectreHubEnvelope wraps report data with a schema identifier.
type spectreHubEnvelope struct {
	Schema string `json:"schema"`
	Data
}

// SpectreHubReporter outputs spectre/v1 JSON.
type SpectreHubReporter struct{}

// Generate writes the spectre/v1 JSON envelope to w.
func (r *SpectreHubReporter) Generate(w io.Writer, data Data) error {
	envelope := spectreHubEnvelope{
		Schema: "spectre/v1",
		Data:   data,
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(envelope)
}
