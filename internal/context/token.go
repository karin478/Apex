package context

// EstimateTokens returns an approximate token count for the given text.
// Uses rune count / 3 as a rough approximation suitable for mixed CJK/Latin text.
func EstimateTokens(text string) int {
	runes := []rune(text)
	if len(runes) == 0 {
		return 0
	}
	est := len(runes) / 3
	if est == 0 {
		est = 1
	}
	return est
}
