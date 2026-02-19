package e2e_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// apexBin holds the absolute path to the compiled apex binary.
// It is built once in TestMain and reused by every test in this package.
var apexBin string

// findProjectRoot walks up from cwd until it finds go.mod.
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find go.mod in any parent of %s", dir)
		}
		dir = parent
	}
}

func TestMain(m *testing.M) {
	root, err := findProjectRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e setup: %v\n", err)
		os.Exit(1)
	}

	// Build the apex binary once into a temp directory
	tmpDir, err := os.MkdirTemp("", "apex-e2e-bin-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e setup: failed to create temp dir: %v\n", err)
		os.Exit(1)
	}

	apexBin = filepath.Join(tmpDir, "apex")
	cmd := exec.Command("go", "build", "-o", apexBin, "./cmd/apex")
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "e2e setup: failed to build apex: %v\n", err)
		os.RemoveAll(tmpDir)
		os.Exit(1)
	}

	code := m.Run()

	os.RemoveAll(tmpDir)
	os.Exit(code)
}
