package credinjector

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FormatCredentialList formats vault credentials as a human-readable table.
func FormatCredentialList(vault *Vault) string {
	if vault == nil || len(vault.Credentials) == 0 {
		return "No credentials configured."
	}

	phW := len("PLACEHOLDER")
	srcW := len("SOURCE")
	keyW := len("KEY")

	for _, c := range vault.Credentials {
		if l := len(c.Placeholder); l > phW {
			phW = l
		}
		if l := len(c.Source); l > srcW {
			srcW = l
		}
		if l := len(c.Key); l > keyW {
			keyW = l
		}
	}

	var b strings.Builder
	rowFmt := fmt.Sprintf("%%-%ds  %%-%ds  %%s\n", phW, srcW)
	fmt.Fprintf(&b, rowFmt, "PLACEHOLDER", "SOURCE", "KEY")
	for _, c := range vault.Credentials {
		fmt.Fprintf(&b, rowFmt, c.Placeholder, c.Source, c.Key)
	}
	return b.String()
}

// FormatValidationResult formats validation results as OK/FAIL per credential.
func FormatValidationResult(vault *Vault) string {
	var b strings.Builder
	for _, ref := range vault.Credentials {
		_, err := Resolve(ref)
		if err != nil {
			fmt.Fprintf(&b, "FAIL  %s: %v\n", ref.Placeholder, err)
		} else {
			fmt.Fprintf(&b, "OK    %s\n", ref.Placeholder)
		}
	}
	return b.String()
}

// FormatCredentialListJSON formats vault credentials as indented JSON.
func FormatCredentialListJSON(vault *Vault) (string, error) {
	if vault == nil || len(vault.Credentials) == 0 {
		return "[]", nil
	}
	data, err := json.MarshalIndent(vault.Credentials, "", "  ")
	if err != nil {
		return "", fmt.Errorf("credinjector: json marshal: %w", err)
	}
	return string(data), nil
}
