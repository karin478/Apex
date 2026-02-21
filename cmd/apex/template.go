package main

import (
	"fmt"
	"strings"

	tmpl "github.com/lyndonlyu/apex/internal/template"
	"github.com/spf13/cobra"
)

var templateFormat string
var templateVars []string

var templateCmd = &cobra.Command{
	Use:   "template",
	Short: "Task template management",
}

var templateListCmd = &cobra.Command{
	Use:   "list",
	Short: "List templates",
	RunE:  runTemplateList,
}

var templateShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show template details",
	Args:  cobra.ExactArgs(1),
	RunE:  runTemplateShow,
}

var templateExpandCmd = &cobra.Command{
	Use:   "expand",
	Short: "Expand template with variables",
	Args:  cobra.ExactArgs(1),
	RunE:  runTemplateExpand,
}

func init() {
	templateListCmd.Flags().StringVar(&templateFormat, "format", "", "Output format (json)")
	templateExpandCmd.Flags().StringSliceVar(&templateVars, "var", nil, "Variables (key=value)")
	templateCmd.AddCommand(templateListCmd, templateShowCmd, templateExpandCmd)
}

func defaultTemplateRegistry() *tmpl.Registry {
	reg := tmpl.NewRegistry()
	reg.Register(tmpl.Template{
		Name: "build-test-deploy",
		Desc: "Standard build, test, and deploy pipeline",
		Vars: []tmpl.TemplateVar{
			{Name: "project", Default: "myapp", Desc: "Project name"},
			{Name: "env", Default: "staging", Desc: "Target environment"},
		},
		Nodes: []tmpl.TemplateNode{
			{ID: "build", Task: "Build {{.project}} for {{.env}}"},
			{ID: "test", Task: "Test {{.project}}", Depends: []string{"build"}},
			{ID: "deploy", Task: "Deploy {{.project}} to {{.env}}", Depends: []string{"test"}},
		},
	})
	return reg
}

func parseVarFlags(vars []string) map[string]string {
	m := make(map[string]string)
	for _, v := range vars {
		parts := strings.SplitN(v, "=", 2)
		if len(parts) == 2 {
			m[parts[0]] = parts[1]
		}
	}
	return m
}

func runTemplateList(cmd *cobra.Command, args []string) error {
	reg := defaultTemplateRegistry()
	templates := reg.List()

	if templateFormat == "json" {
		out, err := tmpl.FormatTemplateListJSON(templates)
		if err != nil {
			return err
		}
		fmt.Println(out)
	} else {
		fmt.Print(tmpl.FormatTemplateList(templates))
	}
	return nil
}

func runTemplateShow(cmd *cobra.Command, args []string) error {
	reg := defaultTemplateRegistry()
	t, err := reg.Get(args[0])
	if err != nil {
		return err
	}
	fmt.Print(tmpl.FormatTemplate(t))
	return nil
}

func runTemplateExpand(cmd *cobra.Command, args []string) error {
	reg := defaultTemplateRegistry()
	t, err := reg.Get(args[0])
	if err != nil {
		return err
	}
	vars := parseVarFlags(templateVars)
	specs, err := t.Expand(vars)
	if err != nil {
		return err
	}
	fmt.Print(tmpl.FormatNodeSpecs(specs))
	return nil
}
