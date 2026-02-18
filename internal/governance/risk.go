package governance

import "strings"

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

func (r RiskLevel) ShouldAutoApprove() bool {
	return r == LOW
}

func (r RiskLevel) ShouldConfirm() bool {
	return r == MEDIUM
}

func (r RiskLevel) ShouldReject() bool {
	return r >= HIGH
}

var criticalKeywords = []string{
	"production", "encryption key", "密钥",
}

var highKeywords = []string{
	"delete", "drop", "deploy", "migrate", "rm -rf",
	"删除", "部署", "生产", "迁移",
}

var mediumKeywords = []string{
	"write", "modify", "install", "update", "config",
	"create", "edit", "change", "add", "remove",
	"修改", "安装", "配置", "创建", "编辑",
}

func Classify(task string) RiskLevel {
	lower := strings.ToLower(task)

	for _, kw := range criticalKeywords {
		if strings.Contains(lower, kw) {
			return CRITICAL
		}
	}

	for _, kw := range highKeywords {
		if strings.Contains(lower, kw) {
			return HIGH
		}
	}

	for _, kw := range mediumKeywords {
		if strings.Contains(lower, kw) {
			return MEDIUM
		}
	}

	return LOW
}
