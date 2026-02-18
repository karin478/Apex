package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Entry struct {
	Task      string
	RiskLevel string
	Outcome   string
	Duration  time.Duration
	Model     string
	Error     string
}

type Record struct {
	Timestamp  string `json:"timestamp"`
	ActionID   string `json:"action_id"`
	Task       string `json:"task"`
	RiskLevel  string `json:"risk_level"`
	Outcome    string `json:"outcome"`
	DurationMs int64  `json:"duration_ms"`
	Model      string `json:"model"`
	Error      string `json:"error,omitempty"`
}

type Logger struct {
	dir string
}

func NewLogger(dir string) (*Logger, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	return &Logger{dir: dir}, nil
}

func (l *Logger) Log(entry Entry) error {
	record := Record{
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		ActionID:   uuid.New().String(),
		Task:       entry.Task,
		RiskLevel:  entry.RiskLevel,
		Outcome:    entry.Outcome,
		DurationMs: entry.Duration.Milliseconds(),
		Model:      entry.Model,
		Error:      entry.Error,
	}

	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	filename := time.Now().Format("2006-01-02") + ".jsonl"
	path := filepath.Join(l.dir, filename)

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(data)
	return err
}

func (l *Logger) Recent(n int) ([]Record, error) {
	files, err := filepath.Glob(filepath.Join(l.dir, "*.jsonl"))
	if err != nil {
		return nil, err
	}
	sort.Sort(sort.Reverse(sort.StringSlice(files)))

	var records []Record
	for _, f := range files {
		if len(records) >= n {
			break
		}
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		for i := len(lines) - 1; i >= 0; i-- {
			if len(records) >= n {
				break
			}
			var r Record
			if err := json.Unmarshal([]byte(lines[i]), &r); err != nil {
				continue
			}
			records = append(records, r)
		}
	}
	return records, nil
}
