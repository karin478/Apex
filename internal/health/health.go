package health

// Level represents the system health level.
type Level int

const (
	GREEN    Level = iota // All components healthy
	YELLOW                // 1 important component degraded
	RED                   // 1 critical or 2+ important degraded
	CRITICAL              // 2+ critical components degraded
)

// String returns the human-readable name of the Level.
func (l Level) String() string {
	switch l {
	case GREEN:
		return "GREEN"
	case YELLOW:
		return "YELLOW"
	case RED:
		return "RED"
	case CRITICAL:
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}

// ComponentStatus represents the health status of a single component.
type ComponentStatus struct {
	Name     string // e.g. "audit_chain", "sandbox_available"
	Category string // "critical", "important", "optional"
	Healthy  bool
	Detail   string // Human-readable description
}

// Report is the result of a health evaluation.
type Report struct {
	Level      Level
	Components []ComponentStatus
}

// Determine is a pure logic function (no I/O) that counts failed critical and
// important components and returns the appropriate health Level.
//
//	if criticalFailed >= 2: CRITICAL
//	else if criticalFailed == 1: RED
//	else if importantFailed >= 2: RED
//	else if importantFailed == 1: YELLOW
//	else: GREEN
func Determine(components []ComponentStatus) Level {
	var criticalFailed, importantFailed int

	for _, c := range components {
		if c.Healthy {
			continue
		}
		switch c.Category {
		case "critical":
			criticalFailed++
		case "important":
			importantFailed++
		}
	}

	switch {
	case criticalFailed >= 2:
		return CRITICAL
	case criticalFailed == 1:
		return RED
	case importantFailed >= 2:
		return RED
	case importantFailed == 1:
		return YELLOW
	default:
		return GREEN
	}
}

// NewReport creates a Report with the Level set by calling Determine.
func NewReport(components []ComponentStatus) *Report {
	return &Report{
		Level:      Determine(components),
		Components: components,
	}
}
