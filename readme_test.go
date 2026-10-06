package main

import (
	"net"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/reecepbcups/docci/types"
)

func TestReadme(t *testing.T) {
	// The README starts a demo server on :3000 and curls it. If something else
	// on the machine already holds that port, this tests the other process
	// rather than docci, and fails for a reason that has nothing to do with the
	// change under test. Say so instead of reporting a false failure.
	// Dial rather than bind: a listener on another interface, or one holding the
	// port with SO_REUSEADDR, still breaks the README's demo server even though
	// binding 127.0.0.1 appears to succeed.
	if conn, err := net.DialTimeout("tcp", "127.0.0.1:3000", 2*time.Second); err == nil {
		conn.Close()
		t.Skip("something is already listening on port 3000, so the README's demo server cannot start")
	}

	// Read the README.md file
	readmeContent, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatalf("Failed to read README.md: %v", err)
	}

	// Process the content to remove ````bash sections
	processedContent := processReadmeContent(string(readmeContent))

	// Write the processed content to a temporary file
	tempFile := "/tmp/processed_readme.md"
	err = os.WriteFile(tempFile, []byte(processedContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write processed README to temp file: %v", err)
	}

	// Clean up temp file after test
	defer os.Remove(tempFile)

	// Run docci on the processed README
	result := RunDocciFileWithOptions(tempFile, types.DocciOpts{
		HideBackgroundLogs: true,
		KeepRunning:        false,
	}) // hide background logs for cleaner test output

	// Check if the execution was successful
	if !result.Success {
		t.Errorf("README execution failed with exit code: %d", result.ExitCode)
		if len(result.ValidationErrors) > 0 {
			t.Errorf("Validation errors: %v", result.ValidationErrors)
		}
	}
}

// processReadmeContent removes ````bash sections and their closing ```` markers
// while preserving the nested ```bash blocks inside them
func processReadmeContent(content string) string {
	// Pattern to match ````bash blocks with their content and closing ````
	// This regex captures:
	// - ````bash (opening)
	// - Any content in between (non-greedy)
	// - ```` (closing)
	pattern := "(?s)````bash\\s*\\n(.*?)\\n````"

	re := regexp.MustCompile(pattern)

	// Replace each ````bash block with just its inner content
	result := re.ReplaceAllStringFunc(content, func(match string) string {
		// Extract the content between ````bash and ````
		lines := strings.Split(match, "\n")
		if len(lines) < 3 {
			return ""
		}

		// Remove the first line (````bash) and last line (````)
		innerContent := strings.Join(lines[1:len(lines)-1], "\n")

		// Return just the inner content
		return innerContent
	})

	return result
}
