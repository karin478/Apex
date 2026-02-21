package staging

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNLIContradiction(t *testing.T) {
	result := ClassifyConflict(
		"the API timeout is 30 seconds",
		"the API timeout is not 30 seconds, it is 60 seconds",
	)
	assert.Equal(t, Contradiction, result)
}

func TestNLIEntailment(t *testing.T) {
	result := ClassifyConflict(
		"the database uses WAL mode for journaling",
		"the database uses WAL mode for journaling and caching",
	)
	assert.Equal(t, Entailment, result)
}

func TestNLINeutral(t *testing.T) {
	result := ClassifyConflict(
		"the API timeout is 30 seconds",
		"the server runs on port 8080",
	)
	assert.Equal(t, Neutral, result)
}

func TestNLIEmptyInputs(t *testing.T) {
	assert.Equal(t, Neutral, ClassifyConflict("", "anything"))
	assert.Equal(t, Neutral, ClassifyConflict("anything", ""))
}
