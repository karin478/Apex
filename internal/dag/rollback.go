package dag

// RollbackQuality grades the completeness of a snapshot rollback.
type RollbackQuality string

const (
	QualityFull       RollbackQuality = "FULL"       // all changes rolled back
	QualityPartial    RollbackQuality = "PARTIAL"    // some changes rolled back
	QualityStructural RollbackQuality = "STRUCTURAL" // structure only, content needs manual check
	QualityNone       RollbackQuality = "NONE"       // no snapshot available
)

// RollbackResult captures the outcome of a rollback attempt.
type RollbackResult struct {
	Quality  RollbackQuality `json:"quality"`
	RunID    string          `json:"run_id"`
	Restored int             `json:"restored"` // number of files restored
	Detail   string          `json:"detail"`
}
