package context

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEstimateTokensEnglish(t *testing.T) {
	text := "Hello world, this is a test."
	tokens := EstimateTokens(text)
	assert.Greater(t, tokens, 0)
	assert.InDelta(t, len([]rune(text))/3, tokens, 5)
}

func TestEstimateTokensChinese(t *testing.T) {
	text := "你好世界这是一个测试"
	tokens := EstimateTokens(text)
	assert.Greater(t, tokens, 0)
}

func TestEstimateTokensEmpty(t *testing.T) {
	assert.Equal(t, 0, EstimateTokens(""))
}

func TestEstimateTokensLong(t *testing.T) {
	text := strings.Repeat("word ", 3000)
	tokens := EstimateTokens(text)
	assert.Greater(t, tokens, 3000)
	assert.Less(t, tokens, 8000)
}
