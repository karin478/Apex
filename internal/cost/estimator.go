package cost

import (
	"fmt"
	"strings"
)

type Estimate struct {
	InputTokens  int
	OutputTokens int
	TotalCost    float64
	Model        string
	NodeCount    int
}

// modelPricing stores per-1M-token prices: [input, output]
var modelPricing = map[string][2]float64{
	"sonnet": {3.0, 15.0},
	"opus":   {15.0, 75.0},
	"haiku":  {0.25, 1.25},
}

func EstimateRun(enrichedTasks map[string]string, model string) *Estimate {
	if len(enrichedTasks) == 0 {
		return &Estimate{Model: model}
	}

	totalInput := 0
	for _, text := range enrichedTasks {
		totalInput += EstimateTokens(text)
	}

	// Estimate output as 2x input (typical for code generation)
	totalOutput := totalInput * 2

	inputPrice, outputPrice := lookupPricing(model)
	totalCost := float64(totalInput)/1_000_000*inputPrice + float64(totalOutput)/1_000_000*outputPrice

	return &Estimate{
		InputTokens:  totalInput,
		OutputTokens: totalOutput,
		TotalCost:    totalCost,
		Model:        model,
		NodeCount:    len(enrichedTasks),
	}
}

func FormatCost(cost float64) string {
	if cost < 0.005 {
		return "<$0.01"
	}
	return fmt.Sprintf("~$%.2f", cost)
}

func lookupPricing(model string) (float64, float64) {
	lower := strings.ToLower(model)
	for key, prices := range modelPricing {
		if strings.Contains(lower, key) {
			return prices[0], prices[1]
		}
	}
	// Default to sonnet pricing
	return 3.0, 15.0
}

// EstimateTokens approximates the token count for a text string (~chars/3).
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
