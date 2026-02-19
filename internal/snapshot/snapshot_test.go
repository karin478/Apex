package snapshot

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com")
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v failed: %s", args, out)
	}
	run("init")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("original"), 0644))
	run("add", ".")
	run("commit", "-m", "initial")
	return dir
}

func TestCreateSnapshot(t *testing.T) {
	dir := initGitRepo(t)
	m := New(dir)
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("modified"), 0644)
	snap, err := m.Create("test-run-001")
	require.NoError(t, err)
	assert.Contains(t, snap.Message, "apex-test-run-001")
	assert.Equal(t, "test-run-001", snap.RunID)
	data, _ := os.ReadFile(filepath.Join(dir, "file.txt"))
	assert.Equal(t, "original", string(data))
}

func TestCreateWithNoChanges(t *testing.T) {
	dir := initGitRepo(t)
	m := New(dir)
	snap, err := m.Create("test-run-002")
	assert.NoError(t, err)
	assert.Nil(t, snap)
}

func TestRestoreSnapshot(t *testing.T) {
	dir := initGitRepo(t)
	m := New(dir)
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("before-run"), 0644)
	_, err := m.Create("test-run-003")
	require.NoError(t, err)
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("after-run-failed"), 0644)
	err = m.Restore("test-run-003")
	require.NoError(t, err)
	data, _ := os.ReadFile(filepath.Join(dir, "file.txt"))
	assert.Equal(t, "before-run", string(data))
}

func TestListSnapshots(t *testing.T) {
	dir := initGitRepo(t)
	m := New(dir)
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("v1"), 0644)
	m.Create("run-a")
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("v2"), 0644)
	m.Create("run-b")
	snaps, err := m.List()
	require.NoError(t, err)
	assert.Len(t, snaps, 2)
}

func TestDropSnapshot(t *testing.T) {
	dir := initGitRepo(t)
	m := New(dir)
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("v1"), 0644)
	m.Create("run-drop")
	err := m.Drop("run-drop")
	require.NoError(t, err)
	snaps, _ := m.List()
	assert.Empty(t, snaps)
}

func TestApplyRestoresWorkingTree(t *testing.T) {
	dir := initGitRepo(t)
	m := New(dir)

	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("user-edits"), 0644)

	snap, err := m.Create("test-apply")
	require.NoError(t, err)
	require.NotNil(t, snap)

	// After Create, working tree is clean (stashed away)
	data, _ := os.ReadFile(filepath.Join(dir, "file.txt"))
	assert.Equal(t, "original", string(data))

	// Apply restores the working tree without removing the stash
	err = m.Apply("test-apply")
	require.NoError(t, err)

	data, _ = os.ReadFile(filepath.Join(dir, "file.txt"))
	assert.Equal(t, "user-edits", string(data))

	// Stash still exists (Apply doesn't remove it)
	snaps, _ := m.List()
	assert.Len(t, snaps, 1)
}

func TestCreateInNonGitDir(t *testing.T) {
	dir := t.TempDir()
	m := New(dir)
	_, err := m.Create("test-run")
	assert.Error(t, err)
}
