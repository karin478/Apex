package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/lyndonlyu/apex/internal/aggregator"
	"github.com/spf13/cobra"
)

var (
	aggStrategy  string
	aggFile      string
	aggKeyField  string
	aggSortField string
	aggFormat    string
)

var aggregateCmd = &cobra.Command{
	Use:   "aggregate",
	Short: "Run an aggregation pipeline on input data",
	RunE:  runAggregate,
}

func init() {
	aggregateCmd.Flags().StringVar(&aggStrategy, "strategy", "", "Aggregation strategy (summarize|merge|reduce)")
	aggregateCmd.Flags().StringVar(&aggFile, "file", "", "Input JSON file containing array of Input objects")
	aggregateCmd.Flags().StringVar(&aggKeyField, "key-field", "", "Key field for merge deduplication")
	aggregateCmd.Flags().StringVar(&aggSortField, "sort-field", "", "Sort field for merge ordering")
	aggregateCmd.Flags().StringVar(&aggFormat, "format", "", "Output format (json for JSON output)")

	_ = aggregateCmd.MarkFlagRequired("strategy")
	_ = aggregateCmd.MarkFlagRequired("file")
}

func runAggregate(cmd *cobra.Command, args []string) error {
	strategy := aggregator.Strategy(aggStrategy)
	switch strategy {
	case aggregator.StrategySummarize, aggregator.StrategyMerge, aggregator.StrategyReduce:
		// valid
	default:
		return fmt.Errorf("aggregate: unknown strategy: %s", aggStrategy)
	}

	data, err := os.ReadFile(aggFile)
	if err != nil {
		return fmt.Errorf("aggregate: read file: %w", err)
	}

	var inputs []aggregator.Input
	if err := json.Unmarshal(data, &inputs); err != nil {
		return fmt.Errorf("aggregate: parse input JSON: %w", err)
	}

	p := aggregator.NewPipeline(strategy)

	if aggKeyField != "" || aggSortField != "" {
		p.SetMergeOptions(aggregator.MergeOptions{
			KeyField:  aggKeyField,
			SortField: aggSortField,
		})
	}

	for _, in := range inputs {
		p.Add(in)
	}

	result, err := p.Execute()
	if err != nil {
		return fmt.Errorf("aggregate: execute: %w", err)
	}

	if aggFormat == "json" {
		fmt.Println(aggregator.FormatResultJSON(result))
	} else {
		fmt.Print(aggregator.FormatResult(result))
	}

	return nil
}
