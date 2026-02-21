package precheck

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDirCheckPass(t *testing.T) {
	dir := t.TempDir()
	c := DirCheck{Dir: dir}
	result := c.Run()

	assert.True(t, result.Passed)
	assert.Equal(t, "OK", result.Message)
	assert.Equal(t, "dir:"+dir, result.Name)
}

func TestDirCheckFail(t *testing.T) {
	c := DirCheck{Dir: "/nonexistent/path/xyz"}
	result := c.Run()

	assert.False(t, result.Passed)
	assert.Contains(t, result.Message, "directory not found")
	assert.Equal(t, "dir:/nonexistent/path/xyz", result.Name)
}

func TestFileCheckPass(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "testfile.txt")
	err := os.WriteFile(path, []byte("hello"), 0o644)
	require.NoError(t, err)

	c := FileCheck{Path: path, Desc: "testfile"}
	result := c.Run()

	assert.True(t, result.Passed)
	assert.Equal(t, "OK", result.Message)
	assert.Equal(t, "file:testfile", result.Name)
}

func TestFileCheckFail(t *testing.T) {
	c := FileCheck{Path: "/nonexistent/file.txt", Desc: "missing"}
	result := c.Run()

	assert.False(t, result.Passed)
	assert.Contains(t, result.Message, "file not found")
}

func TestBinaryCheck(t *testing.T) {
	// "go" binary should be available in the test environment.
	t.Run("found", func(t *testing.T) {
		c := BinaryCheck{Binary: "go"}
		result := c.Run()

		assert.True(t, result.Passed)
		assert.Contains(t, result.Message, "/go")
		assert.Equal(t, "binary:go", result.Name)
	})

	t.Run("not_found", func(t *testing.T) {
		c := BinaryCheck{Binary: "nonexistent_binary_xyz_abc_123"}
		result := c.Run()

		assert.False(t, result.Passed)
		assert.Contains(t, result.Message, "not found in PATH")
	})
}

func TestRunnerAllPass(t *testing.T) {
	r := NewRunner()
	r.Add(CustomCheck{
		CheckName: "pass1",
		Fn: func() CheckResult {
			return CheckResult{Name: "pass1", Passed: true, Message: "OK"}
		},
	})
	r.Add(CustomCheck{
		CheckName: "pass2",
		Fn: func() CheckResult {
			return CheckResult{Name: "pass2", Passed: true, Message: "OK"}
		},
	})

	result := r.Run()

	assert.True(t, result.AllPassed)
	assert.Len(t, result.Results, 2)
	assert.NotEmpty(t, result.Duration)

	// Verify Checks() returns correct names.
	names := r.Checks()
	assert.Equal(t, []string{"pass1", "pass2"}, names)
}

func TestRunnerOneFail(t *testing.T) {
	r := NewRunner()
	r.Add(CustomCheck{
		CheckName: "pass",
		Fn: func() CheckResult {
			return CheckResult{Name: "pass", Passed: true, Message: "OK"}
		},
	})
	r.Add(CustomCheck{
		CheckName: "fail",
		Fn: func() CheckResult {
			return CheckResult{Name: "fail", Passed: false, Message: "something broke"}
		},
	})

	result := r.Run()

	assert.False(t, result.AllPassed)
	assert.Len(t, result.Results, 2)

	// First should pass, second should fail.
	assert.True(t, result.Results[0].Passed)
	assert.False(t, result.Results[1].Passed)
}
