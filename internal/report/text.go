package report

import (
	"fmt"
	"io"
	"text/tabwriter"
)

// TextReporter outputs a human-readable text report.
type TextReporter struct{}

// Generate writes a tabular text report to w.
func (r *TextReporter) Generate(w io.Writer, data Data) error {
	_, _ = fmt.Fprintf(w, "elasticspectre audit report\n")
	_, _ = fmt.Fprintf(w, "Target: %s (sha256:%s)\n\n", data.Target.Type, data.Target.URIHash)

	if len(data.Findings) == 0 {
		_, _ = fmt.Fprintf(w, "No findings.\n")
		return nil
	}

	_, _ = fmt.Fprintf(w, "FINDINGS (%d total", data.Summary.TotalFindings)
	for _, sev := range []string{"high", "medium", "low", "info"} {
		if count, ok := data.Summary.BySeverity[sev]; ok && count > 0 {
			_, _ = fmt.Fprintf(w, ", %d %s", count, sev)
		}
	}
	_, _ = fmt.Fprintf(w, ")\n\n")

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "SEVERITY\tTYPE\tINDEX\tMESSAGE\tSAVINGS")
	for _, f := range data.Findings {
		savings := "-"
		if f.StorageSavingsBytes > 0 {
			savings = formatBytes(f.StorageSavingsBytes)
		} else if f.HeapImpactBytes > 0 {
			savings = formatBytes(f.HeapImpactBytes) + " heap"
		}

		index := f.Index
		if index == "" {
			index = "-"
		}

		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", f.Severity, f.Type, index, f.Message, savings)
	}
	if err := tw.Flush(); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(w, "\nSUMMARY\n")
	_, _ = fmt.Fprintf(w, "  Total findings:      %d\n", data.Summary.TotalFindings)
	_, _ = fmt.Fprintf(w, "  Storage savings:     %s\n", formatBytes(data.Summary.TotalStorageSavings))
	_, _ = fmt.Fprintf(w, "  Heap savings:        %s\n", formatBytes(data.Summary.TotalHeapSavings))

	return nil
}

// formatBytes converts bytes to a human-readable string.
func formatBytes(b int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
		tb = 1024 * gb
	)

	switch {
	case b >= tb:
		return fmt.Sprintf("%.1f TB", float64(b)/float64(tb))
	case b >= gb:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(gb))
	case b >= mb:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(kb))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
