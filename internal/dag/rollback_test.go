package dag

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRollbackQualityString(t *testing.T) {
	assert.Equal(t, "FULL", string(QualityFull))
	assert.Equal(t, "PARTIAL", string(QualityPartial))
	assert.Equal(t, "STRUCTURAL", string(QualityStructural))
	assert.Equal(t, "NONE", string(QualityNone))
}

func TestRollbackResultNoSnapshot(t *testing.T) {
	result := RollbackResult{
		Quality: QualityNone,
		RunID:   "run-001",
		Detail:  "no snapshot available",
	}
	assert.Equal(t, QualityNone, result.Quality)
	assert.Equal(t, 0, result.Restored)
}

func TestRollbackResultFull(t *testing.T) {
	result := RollbackResult{
		Quality:  QualityFull,
		RunID:    "run-002",
		Restored: 5,
		Detail:   "all changes rolled back",
	}
	assert.Equal(t, QualityFull, result.Quality)
	assert.Equal(t, 5, result.Restored)
}
