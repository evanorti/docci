package parser

import "strings"

import "testing"

// A block ends with `trap - DEBUG`, which returns 0. Without restoring the
// block's own status first, a command that failed without aborting -- the left
// side of an `&&`, which bash exempts from `set -e` -- left the block looking
// successful, and the page passed while the command had failed.
func TestCodeExecutionTemplateRestoresBlockStatus(t *testing.T) {
	if !strings.Contains(codeExecutionTemplate, "__docci_rc=$?") {
		t.Fatal("block template does not capture the block's exit status")
	}
	trap := strings.Index(codeExecutionTemplate, "trap - DEBUG")
	capture := strings.Index(codeExecutionTemplate, "__docci_rc=$?")
	restore := strings.Index(codeExecutionTemplate, "( exit $__docci_rc )")
	if capture == -1 || trap == -1 || restore == -1 {
		t.Fatal("block template is missing capture, trap reset or restore")
	}
	if !(capture < trap && trap < restore) {
		t.Fatalf("expected capture before trap reset before restore, got %d/%d/%d", capture, trap, restore)
	}
}
