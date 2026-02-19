package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"regexp"

	"github.com/google/uuid"
	"github.com/lyndonlyu/apex/internal/redact"
)

// dateFileRe matches audit log files named YYYY-MM-DD.jsonl
var dateFileRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}\.jsonl$`)

// auditFiles returns only date-named .jsonl files from the audit directory,
// excluding non-audit files like anchors.jsonl.
func auditFiles(dir string) ([]string, error) {
	all, err := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	if err != nil {
		return nil, err
	}
	var filtered []string
	for _, f := range all {
		if dateFileRe.MatchString(filepath.Base(f)) {
			filtered = append(filtered, f)
		}
	}
	return filtered, nil
}

type Entry struct {
	Task         string
	RiskLevel    string
	Outcome      string
	Duration     time.Duration
	Model        string
	Error        string
	SandboxLevel string
}

type Record struct {
	Timestamp    string `json:"timestamp"`
	ActionID     string `json:"action_id"`
	Task         string `json:"task"`
	RiskLevel    string `json:"risk_level"`
	Outcome      string `json:"outcome"`
	DurationMs   int64  `json:"duration_ms"`
	Model        string `json:"model"`
	Error        string `json:"error,omitempty"`
	SandboxLevel string `json:"sandbox_level,omitempty"`
	PrevHash     string `json:"prev_hash,omitempty"`
	Hash         string `json:"hash,omitempty"`
}

type Logger struct {
	dir      string
	lastHash string
	redactor *redact.Redactor
}

func NewLogger(dir string) (*Logger, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	l := &Logger{dir: dir}
	l.initLastHash()
	return l, nil
}

func (l *Logger) initLastHash() {
	files, err := auditFiles(l.dir)
	if err != nil || len(files) == 0 {
		return
	}
	sort.Strings(files) // ascending date order
	// Read from the newest file
	path := files[len(files)-1]
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	content := strings.TrimSpace(string(data))
	if content == "" {
		return
	}
	lines := strings.Split(content, "\n")
	lastLine := lines[len(lines)-1]
	var r Record
	if err := json.Unmarshal([]byte(lastLine), &r); err != nil {
		return
	}
	l.lastHash = r.Hash
}

func (l *Logger) SetRedactor(r *redact.Redactor) {
	l.redactor = r
}

func computeHash(r Record) string {
	saved := r.Hash
	r.Hash = ""
	data, _ := json.Marshal(r)
	r.Hash = saved
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func (l *Logger) Log(entry Entry) error {
	record := Record{
		Timestamp:    time.Now().UTC().Format(time.RFC3339),
		ActionID:     uuid.New().String(),
		Task:         entry.Task,
		RiskLevel:    entry.RiskLevel,
		Outcome:      entry.Outcome,
		DurationMs:   entry.Duration.Milliseconds(),
		Model:        entry.Model,
		Error:        entry.Error,
		SandboxLevel: entry.SandboxLevel,
		PrevHash:     l.lastHash,
	}
	// Redact sensitive data before hashing
	if l.redactor != nil {
		record.Task = l.redactor.Redact(record.Task)
		record.Error = l.redactor.Redact(record.Error)
	}
	record.Hash = computeHash(record)

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

	if _, err = f.Write(data); err != nil {
		return err
	}
	l.lastHash = record.Hash
	return nil
}

func (l *Logger) Recent(n int) ([]Record, error) {
	files, err := auditFiles(l.dir)
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

func (l *Logger) Verify() (bool, int, error) {
	files, err := auditFiles(l.dir)
	if err != nil {
		return false, -1, err
	}
	sort.Strings(files) // ascending date order

	var expectedPrevHash string
	index := 0

	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return false, -1, err
		}
		content := strings.TrimSpace(string(data))
		if content == "" {
			continue
		}
		lines := strings.Split(content, "\n")
		for _, line := range lines {
			var r Record
			if err := json.Unmarshal([]byte(line), &r); err != nil {
				return false, -1, err
			}
			// Old records without hash: skip verification, treat as chain head
			if r.Hash == "" {
				expectedPrevHash = ""
				index++
				continue
			}
			// Verify hash integrity
			if computeHash(r) != r.Hash {
				return false, index, nil
			}
			// Verify chain linkage
			if r.PrevHash != expectedPrevHash {
				return false, index, nil
			}
			expectedPrevHash = r.Hash
			index++
		}
	}

	return true, -1, nil
}

func (l *Logger) RecordsForDate(date string) ([]Record, error) {
	path := filepath.Join(l.dir, date+".jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	content := strings.TrimSpace(string(data))
	if content == "" {
		return nil, nil
	}
	lines := strings.Split(content, "\n")
	var records []Record
	for _, line := range lines {
		var r Record
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			return nil, fmt.Errorf("parse audit record: %w", err)
		}
		records = append(records, r)
	}
	return records, nil
}

func (l *Logger) LastHashForDate(date string) (string, int, error) {
	records, err := l.RecordsForDate(date)
	if err != nil {
		return "", 0, err
	}
	if len(records) == 0 {
		return "", 0, nil
	}
	return records[len(records)-1].Hash, len(records), nil
}

func (l *Logger) Dir() string {
	return l.dir
}
