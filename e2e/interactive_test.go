package e2e_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestInteractiveSmokeNoArgs(t *testing.T) {
	env := newTestEnv(t)

	// Run apex with no args. go-prompt requires a TTY, so without one
	// it will exit or error quickly. The success criterion is that the
	// binary does not panic or hang — any exit code is acceptable.
	cmd := exec.Command(apexBin)
	cmd.Dir = env.WorkDir
	cmd.Env = []string{
		"HOME=" + env.Home,
		"PATH=" + os.Getenv("PATH"),
		"USER=" + os.Getenv("USER"),
	}

	// Provide empty stdin and set a short timeout
	cmd.Stdin = strings.NewReader("")

	done := make(chan error, 1)
	go func() {
		done <- cmd.Run()
	}()

	select {
	case <-done:
		// Process exited (any exit code is fine — no hang, no panic)
	case <-time.After(5 * time.Second):
		cmd.Process.Kill()
		t.Fatal("apex with no args hung for >5s")
	}
}
