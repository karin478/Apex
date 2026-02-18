package context

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompressExact(t *testing.T) {
	text := "keep this exactly as is"
	result := CompressExact(text)
	assert.Equal(t, text, result)
}

func TestCompressStructuralGo(t *testing.T) {
	code := "package main\n\nimport \"fmt\"\n\n// Greet says hello.\nfunc Greet(name string) string {\n\tmsg := fmt.Sprintf(\"Hello, %s!\", name)\n\treturn msg\n}\n\nfunc helper() {\n\t// long implementation\n\tx := 1\n\ty := 2\n\tz := x + y\n\t_ = z\n}\n"
	result := CompressStructural(code)
	assert.Contains(t, result, "func Greet(name string) string")
	assert.Contains(t, result, "func helper()")
	assert.Contains(t, result, "package main")
	assert.Less(t, len(result), len(code))
}

func TestCompressSummarizable(t *testing.T) {
	doc := "# Redis Migration Guide\n\nThis document describes the migration from Redis 6 to Redis 7.\n\n## Prerequisites\n\nYou need to have Redis 7.2 installed on all servers.\nMake sure to backup your data before proceeding.\n\n## Step 1: Update Configuration\n\nEdit redis.conf and change the ACL settings.\nThere are many detailed sub-steps here.\nLine after line of detailed instructions.\nMore details that can be summarized.\n\n## Step 2: Run Migration\n\nExecute the migration script provided in the tools directory.\nAdditional details about running the migration.\n"
	result := CompressSummarizable(doc)
	assert.Contains(t, result, "# Redis Migration Guide")
	assert.Contains(t, result, "## Prerequisites")
	assert.Contains(t, result, "migration from Redis 6 to Redis 7")
	assert.Less(t, len(result), len(doc))
}

func TestCompressReference(t *testing.T) {
	text := "first line of the file\nsecond line\nthird line\nmany more lines"
	result := CompressReference("path/to/file.go", text)
	assert.Contains(t, result, "path/to/file.go")
	assert.Contains(t, result, "first line of the file")
	assert.Less(t, len(result), len(text))
}

func TestCompressStructuralNonCode(t *testing.T) {
	text := "Some plain text content\nwith multiple lines\nand more content here"
	result := CompressStructural(text)
	assert.NotEmpty(t, result)
}
