package executor

import "testing"

// A command whose output has no trailing newline leaves the end marker on the
// same line as that output. The marker must still be recognised, or the block
// records nothing and the page fails with "produced no output to check".
func TestParseBlockOutputsWithoutTrailingNewline(t *testing.T) {
	out := "### DOCCI_BLOCK_START_1 ###\n" +
		`{"status": "ok"}` + "### DOCCI_BLOCK_END_1 ###\n"

	got := ParseBlockOutputs(out)
	if want := `{"status": "ok"}`; got[1] != want {
		t.Fatalf("block 1 = %q, want %q", got[1], want)
	}
}

func TestParseBlockOutputsWholeLineMarkers(t *testing.T) {
	out := "### DOCCI_BLOCK_START_2 ###\nhello\nworld\n### DOCCI_BLOCK_END_2 ###\n"

	got := ParseBlockOutputs(out)
	if want := "hello\nworld"; got[2] != want {
		t.Fatalf("block 2 = %q, want %q", got[2], want)
	}
}

// The block that follows an unterminated one must still be captured.
func TestParseBlockOutputsSequenceAfterUnterminated(t *testing.T) {
	out := "### DOCCI_BLOCK_START_1 ###\nbare### DOCCI_BLOCK_END_1 ###\n" +
		"### DOCCI_BLOCK_START_2 ###\nsecond\n### DOCCI_BLOCK_END_2 ###\n"

	got := ParseBlockOutputs(out)
	if got[1] != "bare" || got[2] != "second" {
		t.Fatalf("got %q / %q, want \"bare\" / \"second\"", got[1], got[2])
	}
}
