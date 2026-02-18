package context

import (
	"fmt"
	"strings"
)

// CompressionPolicy defines how aggressively content should be compressed.
type CompressionPolicy int

const (
	PolicyExact        CompressionPolicy = iota // No compression
	PolicyStructural                            // Keep signatures and structure
	PolicySummarizable                          // Keep headings and first paragraph
	PolicyReference                             // Just path + first line + size
)

// String returns a human-readable name for the compression policy.
func (p CompressionPolicy) String() string {
	switch p {
	case PolicyExact:
		return "exact"
	case PolicyStructural:
		return "structural"
	case PolicySummarizable:
		return "summarizable"
	case PolicyReference:
		return "reference"
	default:
		return "unknown"
	}
}

// Degrade returns the next more aggressive compression policy.
// PolicySummarizable degrades to PolicyReference.
// PolicyStructural degrades to PolicySummarizable.
// PolicyExact stays PolicyExact (cannot degrade further in a useful way).
// PolicyReference stays PolicyReference (already most aggressive).
func Degrade(p CompressionPolicy) CompressionPolicy {
	switch p {
	case PolicyStructural:
		return PolicySummarizable
	case PolicySummarizable:
		return PolicyReference
	default:
		return p
	}
}

// Compress dispatches to the appropriate compression function based on the policy.
func Compress(policy CompressionPolicy, path, text string) string {
	switch policy {
	case PolicyExact:
		return CompressExact(text)
	case PolicyStructural:
		return CompressStructural(text)
	case PolicySummarizable:
		return CompressSummarizable(text)
	case PolicyReference:
		return CompressReference(path, text)
	default:
		return text
	}
}

// CompressExact returns text unchanged.
func CompressExact(text string) string {
	return text
}

// CompressStructural keeps function/type signatures, package declarations, and
// import statements while truncating function bodies. For non-code text it
// falls back to CompressSummarizable.
func CompressStructural(text string) string {
	if !looksLikeCode(text) {
		return CompressSummarizable(text)
	}

	lines := strings.Split(text, "\n")
	var out []string
	inBody := false
	braceDepth := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Always keep package and import lines.
		if strings.HasPrefix(trimmed, "package ") ||
			strings.HasPrefix(trimmed, "import ") ||
			trimmed == "import (" {
			out = append(out, line)
			continue
		}

		// Track brace depth to know when we are inside a function body.
		if inBody {
			braceDepth += strings.Count(line, "{") - strings.Count(line, "}")
			if braceDepth <= 0 {
				out = append(out, "}")
				inBody = false
				braceDepth = 0
			}
			continue
		}

		// Detect function/type/class declarations.
		if isSigLine(trimmed) {
			out = append(out, line)
			// If the signature opens a brace block, enter body-skipping mode.
			opens := strings.Count(line, "{")
			closes := strings.Count(line, "}")
			if opens > closes {
				braceDepth = opens - closes
				inBody = true
			}
			continue
		}

		// Keep comments directly preceding a signature (doc comments).
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*") {
			out = append(out, line)
			continue
		}

		// Keep blank lines for readability (collapse consecutive blanks).
		if trimmed == "" {
			if len(out) == 0 || strings.TrimSpace(out[len(out)-1]) != "" {
				out = append(out, "")
			}
			continue
		}

		// Keep import block closing paren.
		if trimmed == ")" {
			out = append(out, line)
			continue
		}

		// Inside an import block, keep the imports.
		// Simple heuristic: if we see a quoted string, keep it.
		if strings.Contains(trimmed, "\"") {
			out = append(out, line)
			continue
		}
	}

	// Trim trailing blank lines.
	for len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
		out = out[:len(out)-1]
	}

	return strings.Join(out, "\n") + "\n"
}

// CompressSummarizable keeps markdown headings and the first non-empty
// paragraph (the first contiguous block of non-heading, non-blank lines)
// after the title heading.
func CompressSummarizable(text string) string {
	lines := strings.Split(text, "\n")
	var out []string
	seenTitle := false
	capturedFirstPara := false
	inFirstPara := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Keep all headings.
		if strings.HasPrefix(trimmed, "#") {
			// If we were in the first paragraph, close it.
			if inFirstPara {
				inFirstPara = false
				capturedFirstPara = true
			}
			out = append(out, line)
			if !seenTitle {
				seenTitle = true
			}
			continue
		}

		// Capture the first paragraph after the title.
		if seenTitle && !capturedFirstPara {
			if trimmed == "" {
				if inFirstPara {
					// End of first paragraph.
					inFirstPara = false
					capturedFirstPara = true
				}
				// Keep blank lines between title and first paragraph for formatting.
				out = append(out, line)
				continue
			}
			// Non-blank, non-heading line after title: part of first paragraph.
			inFirstPara = true
			out = append(out, line)
			continue
		}

		// After first paragraph is captured, only keep headings (handled above)
		// and blank lines around headings for readability.
		if trimmed == "" {
			if len(out) > 0 && strings.TrimSpace(out[len(out)-1]) != "" {
				out = append(out, "")
			}
		}
	}

	// Trim trailing blank lines.
	for len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
		out = out[:len(out)-1]
	}

	return strings.Join(out, "\n") + "\n"
}

// CompressReference returns a compact reference line with the file path,
// byte count, and the first line of the content.
func CompressReference(path string, text string) string {
	firstLine := text
	if idx := strings.IndexByte(text, '\n'); idx >= 0 {
		firstLine = text[:idx]
	}
	return fmt.Sprintf("[ref: %s (%d bytes)] %s", path, len(text), firstLine)
}

// looksLikeCode returns true if the text appears to be source code based on
// the presence of common code-level keywords at the start of lines.
func looksLikeCode(text string) bool {
	prefixes := []string{"package ", "func ", "type ", "import ", "def ", "class "}
	for _, prefix := range prefixes {
		if strings.Contains(text, "\n"+prefix) || strings.HasPrefix(text, prefix) {
			return true
		}
	}
	return false
}

// isSigLine returns true if a trimmed line looks like a function, type, or
// class declaration.
func isSigLine(trimmed string) bool {
	sigPrefixes := []string{"func ", "type ", "def ", "class "}
	for _, p := range sigPrefixes {
		if strings.HasPrefix(trimmed, p) {
			return true
		}
	}
	return false
}
