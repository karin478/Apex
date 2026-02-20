package metrics

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FormatHuman returns a human-readable table of metrics.
func FormatHuman(metrics []Metric) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%-35s %12s  %s\n", "METRIC", "VALUE", "LABELS"))
	b.WriteString(strings.Repeat("-", 65) + "\n")

	for _, m := range metrics {
		labels := ""
		if len(m.Labels) > 0 {
			var parts []string
			for k, v := range m.Labels {
				parts = append(parts, fmt.Sprintf("%s=%s", k, v))
			}
			labels = strings.Join(parts, ", ")
		}

		valStr := fmt.Sprintf("%.0f", m.Value)
		if m.Value != float64(int64(m.Value)) {
			valStr = fmt.Sprintf("%.2f", m.Value)
		}

		b.WriteString(fmt.Sprintf("%-35s %12s  %s\n", m.Name, valStr, labels))
	}
	return b.String()
}

// FormatJSONL returns one JSON object per line.
func FormatJSONL(metrics []Metric) (string, error) {
	var b strings.Builder
	for _, m := range metrics {
		data, err := json.Marshal(m)
		if err != nil {
			return "", err
		}
		b.Write(data)
		b.WriteByte('\n')
	}
	return b.String(), nil
}
