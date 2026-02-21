package template

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lyndonlyu/apex/internal/dag"
)

// FormatTemplateList formats a slice of templates as a table with columns
// NAME, DESC, VARS (count), and NODES (count).
func FormatTemplateList(templates []Template) string {
	if len(templates) == 0 {
		return "No templates.\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%-20s %-30s %-6s %-6s\n", "NAME", "DESC", "VARS", "NODES")
	for _, t := range templates {
		fmt.Fprintf(&b, "%-20s %-30s %-6d %-6d\n", t.Name, t.Desc, len(t.Vars), len(t.Nodes))
	}
	return b.String()
}

// FormatTemplate formats a single template with its variables and nodes
// in a human-readable detail view.
func FormatTemplate(t Template) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Name: %s\nDescription: %s\n", t.Name, t.Desc)

	b.WriteString("\nVariables:\n")
	fmt.Fprintf(&b, "  %-15s %-15s %s\n", "NAME", "DEFAULT", "DESC")
	for _, v := range t.Vars {
		fmt.Fprintf(&b, "  %-15s %-15s %s\n", v.Name, v.Default, v.Desc)
	}

	b.WriteString("\nNodes:\n")
	fmt.Fprintf(&b, "  %-15s %-40s %s\n", "ID", "TASK", "DEPENDS")
	for _, n := range t.Nodes {
		depends := strings.Join(n.Depends, ", ")
		fmt.Fprintf(&b, "  %-15s %-40s %s\n", n.ID, n.Task, depends)
	}

	return b.String()
}

// FormatNodeSpecs formats a slice of dag.NodeSpec as a table with columns
// ID, TASK, and DEPENDS.
func FormatNodeSpecs(specs []dag.NodeSpec) string {
	if len(specs) == 0 {
		return "No node specs.\n"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%-15s %-40s %-20s\n", "ID", "TASK", "DEPENDS")
	for _, s := range specs {
		depends := strings.Join(s.Depends, ", ")
		fmt.Fprintf(&b, "%-15s %-40s %-20s\n", s.ID, s.Task, depends)
	}
	return b.String()
}

// FormatTemplateListJSON marshals a slice of templates as indented JSON.
func FormatTemplateListJSON(templates []Template) (string, error) {
	data, err := json.MarshalIndent(templates, "", "  ")
	if err != nil {
		return "", fmt.Errorf("template: json marshal: %w", err)
	}
	return string(data), nil
}
