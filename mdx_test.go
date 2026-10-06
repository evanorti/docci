package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/reecepbcups/docci/parser"
	"github.com/reecepbcups/docci/types"
)

// withExampleDir runs fn with the MDX examples as the working directory, since
// those pages write and clean up a file beside themselves.
func withExampleDir(t *testing.T, fn func()) {
	t.Helper()

	original, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir("examples/mdx"); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() {
		os.Remove("counter.txt")
		if err := os.Chdir(original); err != nil {
			t.Fatalf("restore wd: %v", err)
		}
	}()

	fn()
}

// TestMDXIndentedFencesRun is the regression that matters most for MDX: a fence
// nested inside <Steps> used to be skipped silently, so a page could report
// success while testing nothing.
func TestMDXIndentedFencesRun(t *testing.T) {
	document, err := os.ReadFile("examples/mdx/01-start.mdx")
	if err != nil {
		t.Fatalf("read page: %v", err)
	}

	blocks, err := parser.ParseCodeBlocksWithFileName(string(document), "01-start.mdx")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	var found bool
	for _, block := range blocks {
		if block.StepTitle == "Write the first value" {
			found = true
			if !strings.HasPrefix(block.Content, "echo") {
				t.Errorf("indented fence kept its indentation: %q", block.Content)
			}
		}
	}
	if !found {
		t.Fatalf("no block was parsed from inside <Steps>; got %d blocks", len(blocks))
	}
}

// TestMDXBlocksCarryTheirLocation checks that a failure can name the step a
// reader would be on rather than a block index.
func TestMDXBlocksCarryTheirLocation(t *testing.T) {
	document, _ := os.ReadFile("examples/mdx/01-start.mdx")
	blocks, err := parser.ParseCodeBlocksWithFileName(string(document), "01-start.mdx")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	var checkpoint *parser.CodeBlock
	for i := range blocks {
		if blocks[i].Name == "counter reports a height" {
			checkpoint = &blocks[i]
		}
	}
	if checkpoint == nil {
		t.Fatal("named block was not parsed")
	}
	if checkpoint.ExpectOutput == "" {
		t.Error("expect-output block was not attached to the block above it")
	}
	if !strings.Contains(checkpoint.Describe(), "counter reports a height") {
		t.Errorf("failure would not name the step: %s", checkpoint.Describe())
	}
}

// TestMDXPageRunsWithPrerequisites runs a page that declares `needs`, which
// should run the page it builds on first, in the same shell.
func TestMDXPageRunsWithPrerequisites(t *testing.T) {
	order, configs, err := parser.ResolveRunOrder([]string{"examples/mdx/02-continues.mdx"})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(order) != 2 {
		t.Fatalf("expected the prerequisite to be added, got %v", order)
	}
	if filepath.Base(order[0]) != "01-start.mdx" {
		t.Errorf("prerequisite did not run first: %v", order)
	}

	abs, _ := filepath.Abs(order[0])
	if len(configs[abs].Cleanup) == 0 {
		t.Error("page cleanup was not read from frontmatter")
	}

	withExampleDir(t, func() {
		result := RunDocciFilesWithOptions([]string{"01-start.mdx", "02-continues.mdx"}, types.DocciOpts{HideBackgroundLogs: true})
		if !result.Success {
			t.Errorf("merged run failed: %s", result.Stderr)
		}
	})
}

// TestMDXCheckpointFailsLoudly breaks the rendered output and checks the page
// fails with a message naming the step.
func TestMDXCheckpointFailsLoudly(t *testing.T) {
	document, _ := os.ReadFile("examples/mdx/01-start.mdx")
	broken := strings.Replace(string(document), `"ready": true`+"\n}", `"ready": false`+"\n}", 1)
	if broken == string(document) {
		t.Fatal("test fixture did not change; the expected-output block moved")
	}

	withExampleDir(t, func() {
		if err := os.WriteFile("broken.mdx", []byte(broken), 0644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
		defer os.Remove("broken.mdx")

		result := RunDocciFileWithOptions("broken.mdx", types.DocciOpts{HideBackgroundLogs: true})
		if result.Success {
			t.Fatal("a page whose output no longer matches reported success")
		}
		if !strings.Contains(result.Stderr, "counter reports a height") {
			t.Errorf("failure did not name the step: %s", result.Stderr)
		}
	})
}

// TestMDXRunnableFalseIsSkipped checks the opt-out for illustrative pages.
func TestMDXRunnableFalseIsSkipped(t *testing.T) {
	document, _ := os.ReadFile("examples/mdx/03-not-runnable.mdx")
	cfg, _, _, err := parser.SplitFrontmatter(string(document))
	if err != nil {
		t.Fatalf("frontmatter: %v", err)
	}
	if cfg.IsRunnable() {
		t.Error("runnable: false was not read from frontmatter")
	}
}

// TestMDXRetryWaitsForOutput covers the case a polling tutorial step creates: a
// command that exits cleanly while reporting a pending state. Retrying only on
// a non-zero exit never waits for anything, so retry has to re-check the
// block's own expected output.
func TestMDXRetryWaitsForOutput(t *testing.T) {
	t.Setenv("DOCCI_RETRY_DELAY", "0")

	withExampleDir(t, func() {
		defer os.Remove("attempts.txt")

		result := RunDocciFileWithOptions("04-poll.mdx", types.DocciOpts{HideBackgroundLogs: true})
		if !result.Success {
			t.Fatalf("a block that polls to success was reported as failed: %s", result.Stderr)
		}
		if !strings.Contains(result.Stdout, "PACKET_STATE_PENDING") {
			t.Error("expected the pending attempts to be shown, not swallowed")
		}
		if !strings.Contains(result.Stdout, "PACKET_STATE_SUCCEEDED") {
			t.Error("expected the successful attempt to be shown")
		}
	})
}

// TestMDXRetryGivesUpLoudly checks that output which never arrives fails the
// page and names the step, rather than hanging or passing.
func TestMDXRetryGivesUpLoudly(t *testing.T) {
	t.Setenv("DOCCI_RETRY_DELAY", "0")

	document, _ := os.ReadFile("examples/mdx/04-poll.mdx")
	never := strings.Replace(string(document), `-ge 3 `, `-ge 99 `, 1)
	if never == string(document) {
		t.Fatal("test fixture did not change")
	}

	withExampleDir(t, func() {
		if err := os.WriteFile("never.mdx", []byte(never), 0644); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
		defer os.Remove("never.mdx")
		defer os.Remove("attempts.txt")

		result := RunDocciFileWithOptions("never.mdx", types.DocciOpts{HideBackgroundLogs: true})
		if result.Success {
			t.Fatal("output that never appeared was reported as success")
		}
		if !strings.Contains(result.Stdout, "transfer reaches a terminal state") {
			t.Errorf("giving up did not name the step: %s", result.Stdout)
		}
	})
}
