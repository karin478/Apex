package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/lyndonlyu/apex/internal/paging"
	"github.com/spf13/cobra"
)

var pagingLines string
var pagingFormat string

var pagingCmd = &cobra.Command{
	Use:     "paging",
	Aliases: []string{"page"},
	Short:   "Context paging for on-demand artifact retrieval",
}

var pagingPageCmd = &cobra.Command{
	Use:   "fetch <artifact-id>",
	Short: "Fetch content from an artifact",
	Args:  cobra.ExactArgs(1),
	RunE:  runPagingFetch,
}

var pagingBudgetCmd = &cobra.Command{
	Use:   "budget",
	Short: "Show default paging budget",
	RunE:  runPagingBudget,
}

func init() {
	pagingPageCmd.Flags().StringVar(&pagingLines, "lines", "", "Line range (e.g. 10-50)")
	pagingBudgetCmd.Flags().StringVar(&pagingFormat, "format", "", "Output format (json)")
	pagingCmd.AddCommand(pagingPageCmd, pagingBudgetCmd)
}

// parseLineRange parses "START-END" into two ints.
func parseLineRange(s string) (start, end int, err error) {
	if s == "" {
		return 0, 0, nil
	}
	parts := strings.SplitN(s, "-", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid line range %q (expected START-END)", s)
	}
	start, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid start line %q: %w", parts[0], err)
	}
	end, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid end line %q: %w", parts[1], err)
	}
	return start, end, nil
}

func runPagingFetch(cmd *cobra.Command, args []string) error {
	artifactID := args[0]

	startLine, endLine, err := parseLineRange(pagingLines)
	if err != nil {
		return err
	}

	// Use an empty in-memory store — real integration would use artifact store.
	store := &memoryStore{data: map[string]string{}}
	budget := paging.DefaultBudget()
	pager := paging.NewPager(store, budget)

	result, err := pager.Page(paging.PageRequest{
		ArtifactID: artifactID,
		StartLine:  startLine,
		EndLine:    endLine,
	})
	if err != nil {
		return err
	}

	fmt.Print(paging.FormatPageResult(result))
	return nil
}

func runPagingBudget(cmd *cobra.Command, args []string) error {
	budget := paging.DefaultBudget()

	if pagingFormat == "json" {
		out, err := paging.FormatBudgetJSON(budget)
		if err != nil {
			return err
		}
		fmt.Println(out)
	} else {
		fmt.Print(paging.FormatBudget(budget))
	}
	return nil
}

// memoryStore is a simple in-memory ContentStore for CLI usage.
type memoryStore struct {
	data map[string]string
}

func (m *memoryStore) GetContent(artifactID string) (string, error) {
	content, ok := m.data[artifactID]
	if !ok {
		return "", fmt.Errorf("artifact %q not found", artifactID)
	}
	return content, nil
}
