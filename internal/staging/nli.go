package staging

import "strings"

// ConflictType represents the relationship between two memories.
type ConflictType string

const (
	Contradiction ConflictType = "CONTRADICTION"
	Entailment    ConflictType = "ENTAILMENT"
	Neutral       ConflictType = "NEUTRAL"
)

// negationKeywords used to detect contradiction in natural language.
var negationKeywords = []string{
	"not", "incorrect", "deprecated", "wrong", "false", "invalid", "obsolete",
}

// ClassifyConflict uses keyword-based heuristics to classify the relationship
// between an existing memory and a new candidate.
func ClassifyConflict(existing, candidate string) ConflictType {
	if existing == "" || candidate == "" {
		return Neutral
	}

	existingLower := strings.ToLower(existing)
	candidateLower := strings.ToLower(candidate)

	existingWords := tokenize(existingLower)
	candidateWords := tokenize(candidateLower)

	overlap := keywordOverlap(existingWords, candidateWords)

	// Check for contradiction: negation keywords + >50% overlap
	if overlap > 0.5 {
		for _, neg := range negationKeywords {
			if strings.Contains(candidateLower, neg) {
				return Contradiction
			}
		}
	}

	// Check for entailment: >80% overlap
	if overlap > 0.8 {
		return Entailment
	}

	return Neutral
}

// tokenize splits text into lowercase word tokens.
func tokenize(text string) map[string]bool {
	words := make(map[string]bool)
	for _, w := range strings.Fields(text) {
		w = strings.Trim(w, ".,;:!?\"'()[]{}") // strip punctuation
		if len(w) > 1 {                          // skip single chars
			words[w] = true
		}
	}
	return words
}

// keywordOverlap returns the fraction of existing words found in candidate.
func keywordOverlap(existing, candidate map[string]bool) float64 {
	if len(existing) == 0 {
		return 0
	}
	matches := 0
	for w := range existing {
		if candidate[w] {
			matches++
		}
	}
	return float64(matches) / float64(len(existing))
}
