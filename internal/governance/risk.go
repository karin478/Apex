package governance

import (
	"strings"
)

type RiskLevel int

const (
	LOW RiskLevel = iota
	MEDIUM
	HIGH
	CRITICAL
)

func (r RiskLevel) String() string {
	switch r {
	case LOW:
		return "LOW"
	case MEDIUM:
		return "MEDIUM"
	case HIGH:
		return "HIGH"
	case CRITICAL:
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}

// ParseRiskLevel converts a string to a RiskLevel.
func ParseRiskLevel(s string) RiskLevel {
	switch strings.ToUpper(s) {
	case "LOW":
		return LOW
	case "MEDIUM":
		return MEDIUM
	case "HIGH":
		return HIGH
	case "CRITICAL":
		return CRITICAL
	default:
		return LOW
	}
}

// Policy defines configurable risk governance behavior.
type Policy struct {
	AutoApprove []string // risk levels to auto-approve
	Confirm     []string // risk levels requiring confirmation
	Reject      []string // risk levels to reject
}

// DefaultPolicy returns the hardcoded default governance policy.
func DefaultPolicy() Policy {
	return Policy{
		AutoApprove: []string{"LOW"},
		Confirm:     []string{"MEDIUM"},
		Reject:      []string{"CRITICAL"},
	}
}

var activePolicy = DefaultPolicy()

// SetPolicy overrides the active governance policy (call from config loading).
func SetPolicy(p Policy) {
	activePolicy = p
}

// GetPolicy returns the active governance policy.
func GetPolicy() Policy {
	return activePolicy
}

func (r RiskLevel) policyContains(levels []string) bool {
	s := r.String()
	for _, l := range levels {
		if strings.EqualFold(l, s) {
			return true
		}
	}
	return false
}

func (r RiskLevel) ShouldAutoApprove() bool {
	return r.policyContains(activePolicy.AutoApprove)
}

func (r RiskLevel) ShouldConfirm() bool {
	return r.policyContains(activePolicy.Confirm)
}

func (r RiskLevel) ShouldReject() bool {
	return r.policyContains(activePolicy.Reject)
}

func (r RiskLevel) ShouldRequireApproval() bool {
	// Approval = not auto-approved, not confirmed, not rejected, but HIGH enough
	// Falls through to: anything requiring explicit per-node approval
	return !r.ShouldAutoApprove() && !r.ShouldConfirm() && !r.ShouldReject()
}

// --- Risk Classification ---

// criticalPhrases match as substrings — only true operational dangers.
var criticalPhrases = []string{
	"production", "encryption key", "密钥",
}

// highPhrases match destructive operations.
var highPhrases = []string{
	"rm -rf",
	"drop table", "drop database", "drop index",
	"delete from", "truncate table",
	"force push", "force-push",
	"删除数据库", "部署到生产", "迁移数据库",
}

// highWords match only as whole words (not substrings).
var highWords = []string{
	"deploy", "migrate",
}

// mediumPhrases match mutation-intent phrases (reduces false positives).
var mediumPhrases = []string{
	"write to", "write file", "write data",
	"modify file", "modify config", "modify database",
	"install package", "install dependency",
	"update config", "update database", "update schema",
	"create file", "create directory", "create database",
	"edit file", "edit config",
	"change config", "change permission",
	"remove file", "remove directory", "remove package",
	"修改配置", "修改文件", "安装依赖", "创建文件", "编辑配置",
}

// mediumWords match only as whole words — avoids "write tests" triggering MEDIUM.
var mediumWords = []string{
	"overwrite", "chmod", "chown",
}

func Classify(task string) RiskLevel {
	lower := strings.ToLower(task)

	for _, p := range criticalPhrases {
		if strings.Contains(lower, p) {
			return CRITICAL
		}
	}

	for _, p := range highPhrases {
		if strings.Contains(lower, p) {
			return HIGH
		}
	}
	for _, w := range highWords {
		if containsWord(lower, w) {
			return HIGH
		}
	}

	for _, p := range mediumPhrases {
		if strings.Contains(lower, p) {
			return MEDIUM
		}
	}
	for _, w := range mediumWords {
		if containsWord(lower, w) {
			return MEDIUM
		}
	}

	return LOW
}

// containsWord checks if word appears as a whole word in text.
func containsWord(text, word string) bool {
	idx := 0
	for {
		pos := strings.Index(text[idx:], word)
		if pos == -1 {
			return false
		}
		start := idx + pos
		end := start + len(word)

		leftOK := start == 0 || !isWordChar(text[start-1])
		rightOK := end == len(text) || !isWordChar(text[end])

		if leftOK && rightOK {
			return true
		}
		idx = start + 1
	}
}

func isWordChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}
