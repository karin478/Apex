package main

import (
	"fmt"
	"os"
)

// homeDir returns the user's home directory or an error.
func homeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return home, nil
}
