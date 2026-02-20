package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lyndonlyu/apex/internal/credinjector"
	"github.com/spf13/cobra"
)

var credentialFormat string

var credentialCmd = &cobra.Command{
	Use:   "credential",
	Short: "Manage credential references",
}

var credentialListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured credential references",
	RunE:  runCredentialList,
}

var credentialValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate all credential references are resolvable",
	RunE:  runCredentialValidate,
}

var credentialScrubCmd = &cobra.Command{
	Use:   "scrub",
	Short: "Scrub credential values from stdin text",
	RunE:  runCredentialScrub,
}

func init() {
	credentialListCmd.Flags().StringVar(&credentialFormat, "format", "", "Output format (json)")
	credentialCmd.AddCommand(credentialListCmd, credentialValidateCmd, credentialScrubCmd)
}

func credentialDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("credinjector: home dir: %w", err)
	}
	return filepath.Join(home, ".claude", "credentials"), nil
}

func loadVault() (*credinjector.Vault, error) {
	dir, err := credentialDir()
	if err != nil {
		return nil, err
	}
	return credinjector.LoadVaultDir(dir)
}

func runCredentialList(cmd *cobra.Command, args []string) error {
	vault, err := loadVault()
	if err != nil {
		return err
	}

	if credentialFormat == "json" {
		out, err := credinjector.FormatCredentialListJSON(vault)
		if err != nil {
			return err
		}
		fmt.Println(out)
	} else {
		fmt.Print(credinjector.FormatCredentialList(vault))
	}
	return nil
}

func runCredentialValidate(cmd *cobra.Command, args []string) error {
	vault, err := loadVault()
	if err != nil {
		return err
	}

	if len(vault.Credentials) == 0 {
		fmt.Println("No credentials configured.")
		return nil
	}

	fmt.Print(credinjector.FormatValidationResult(vault))

	errs := credinjector.ValidateVault(vault)
	if len(errs) > 0 {
		return fmt.Errorf("%d credential(s) failed validation", len(errs))
	}
	return nil
}

func runCredentialScrub(cmd *cobra.Command, args []string) error {
	vault, err := loadVault()
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		fmt.Println(credinjector.Scrub(scanner.Text(), vault))
	}
	return scanner.Err()
}
