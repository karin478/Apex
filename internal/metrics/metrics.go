package metrics

import (
	"path/filepath"
	"time"

	"github.com/lyndonlyu/apex/internal/audit"
	"github.com/lyndonlyu/apex/internal/health"
	"github.com/lyndonlyu/apex/internal/manifest"
)

// Metric represents a single metric data point.
type Metric struct {
	Name      string            `json:"name"`
	Value     float64           `json:"value"`
	Labels    map[string]string `json:"labels,omitempty"`
	Timestamp string            `json:"timestamp"`
}

// Collector gathers metrics from apex subsystems.
type Collector struct {
	baseDir string
}

// NewCollector creates a Collector rooted at baseDir (~/.apex).
func NewCollector(baseDir string) *Collector {
	return &Collector{baseDir: baseDir}
}

// Collect gathers all metrics from runs, health, and audit.
func (c *Collector) Collect() ([]Metric, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	var metrics []Metric

	// Run metrics from manifest store
	runsDir := filepath.Join(c.baseDir, "runs")
	store := manifest.NewStore(runsDir)
	manifests, err := store.Recent(10000) // all
	if err == nil {
		metrics = append(metrics, c.runMetrics(manifests, now)...)
	}

	// Health metric
	report := health.Evaluate(c.baseDir)
	metrics = append(metrics, Metric{
		Name:      "apex_health_level",
		Value:     float64(report.Level),
		Labels:    map[string]string{"level": report.Level.String()},
		Timestamp: now,
	})

	// Audit metrics
	auditDir := filepath.Join(c.baseDir, "audit")
	logger, logErr := audit.NewLogger(auditDir)
	if logErr == nil {
		metrics = append(metrics, c.auditMetrics(logger, now)...)
	}

	return metrics, nil
}

func (c *Collector) runMetrics(manifests []*manifest.Manifest, now string) []Metric {
	var metrics []Metric

	metrics = append(metrics, Metric{
		Name: "apex_runs_total", Value: float64(len(manifests)), Timestamp: now,
	})

	// By outcome
	outcomes := map[string]int{}
	totalNodes := 0
	failedNodes := 0
	var totalDuration int64

	for _, m := range manifests {
		outcomes[m.Outcome]++
		totalNodes += m.NodeCount
		totalDuration += m.DurationMs
		for _, n := range m.Nodes {
			if n.Status == "failed" {
				failedNodes++
			}
		}
	}

	for outcome, count := range outcomes {
		metrics = append(metrics, Metric{
			Name: "apex_runs_by_outcome", Value: float64(count),
			Labels: map[string]string{"outcome": outcome}, Timestamp: now,
		})
	}

	if len(manifests) > 0 {
		metrics = append(metrics, Metric{
			Name:      "apex_run_duration_ms_avg",
			Value:     float64(totalDuration) / float64(len(manifests)),
			Timestamp: now,
		})
	}

	metrics = append(metrics, Metric{
		Name: "apex_dag_nodes_total", Value: float64(totalNodes), Timestamp: now,
	})
	metrics = append(metrics, Metric{
		Name: "apex_dag_nodes_failed", Value: float64(failedNodes), Timestamp: now,
	})

	return metrics
}

func (c *Collector) auditMetrics(logger *audit.Logger, now string) []Metric {
	var metrics []Metric

	records, err := logger.Recent(100000) // all
	if err == nil {
		metrics = append(metrics, Metric{
			Name: "apex_audit_entries_total", Value: float64(len(records)), Timestamp: now,
		})
	}

	valid, _, verifyErr := logger.Verify()
	chainVal := float64(0)
	if verifyErr == nil && valid {
		chainVal = 1
	}
	metrics = append(metrics, Metric{
		Name: "apex_audit_chain_valid", Value: chainVal, Timestamp: now,
	})

	return metrics
}
