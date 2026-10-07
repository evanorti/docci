package review

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRenamedFieldIsReExpression(t *testing.T) {
	before := Assertion{Key: "k", Kind: "expect", Text: "{\"signer\": \"dev\"}"}
	after := Assertion{Key: "k", Kind: "expect", Text: "{\"signerA\": \"dev\"}"}

	verdict := Classify(Pair{Key: "k", Kind: "expect", Before: &before, After: &after}, "")
	require.Equal(t, "re-expression", verdict.Class)
}

func TestDroppedNoiseIsPlainReduction(t *testing.T) {
	before := Assertion{Key: "k", Kind: "expect", Text: "12:34:56 starting\nready true"}
	after := Assertion{Key: "k", Kind: "expect", Text: "ready true"}

	verdict := Classify(Pair{Key: "k", Kind: "expect", Before: &before, After: &after}, "")
	require.Equal(t, "reduction", verdict.Class)
}

func TestDroppedDemonstratedValueIsLoadBearing(t *testing.T) {
	before := Assertion{Key: "k", Kind: "expect", Text: "{\"balance\": \"10\"}"}
	after := Assertion{Key: "k", Kind: "expect", Text: "{\"balance\": \"<...>\"}"}
	prose := "Confirm the tokens arrived: you should see 10 tokens."

	verdict := Classify(Pair{Key: "k", Kind: "expect", Before: &before, After: &after}, prose)
	require.Equal(t, "reduction-load-bearing", verdict.Class)
	require.Contains(t, verdict.Lost, "10")
}

// Review Focus 2: a deleted assertion is the largest reduction there is.
func TestDeletedAssertionIsLoadBearingReduction(t *testing.T) {
	before := Assertion{Key: "k", Kind: "expect", Text: "{\"ready\": true}"}

	verdict := Classify(Pair{Key: "k", Kind: "expect", Before: &before}, "")
	require.Equal(t, "reduction-load-bearing", verdict.Class)
	require.Contains(t, verdict.Reason, "removed entirely")
}

func TestLiteralReplacingDerivedValueIsHardcoding(t *testing.T) {
	before := Assertion{Key: "k", Kind: "contains", Text: "$(ibc keys show dev -a)"}
	after := Assertion{Key: "k", Kind: "contains", Text: "0x58A57ed9d8d624cBD12e2C467D34787555bB1b25"}

	verdict := Classify(Pair{Key: "k", Kind: "contains", Before: &before, After: &after}, "")
	require.Equal(t, "hardcoding", verdict.Class)
}

// The parser leaves trailing whitespace on fenced content, so a reformat must
// not read as an edit.
func TestWhitespaceOnlyChangeIsUnchanged(t *testing.T) {
	before := Assertion{Key: "k", Kind: "expect", Text: "{\n  \"ready\": true\n}\n"}
	after := Assertion{Key: "k", Kind: "expect", Text: "{  \n\"ready\":   true  \n}"}

	verdict := Classify(Pair{Key: "k", Kind: "expect", Before: &before, After: &after}, "")
	require.Equal(t, "unchanged", verdict.Class)
}

func TestProseDropsFencesAndDirectives(t *testing.T) {
	doc := "Intro sentence.\n\n" +
		"{/* docci name=\"x\"\n  retry=2\n*/}\n\n" +
		"```bash\necho secret-in-code\n```\n\n" +
		"<!-- docci expect-output -->\n\n" +
		"~~~json\n{\"hidden\": 42}\n~~~\n\n" +
		"You should see 10 tokens.\n"

	prose := Prose(doc)
	require.Contains(t, prose, "Intro sentence.")
	require.Contains(t, prose, "You should see 10 tokens.")
	for _, gone := range []string{"secret-in-code", "hidden", "docci", "retry"} {
		require.NotContains(t, prose, gone)
	}
}

const runBase = `Intro.

{/* docci name="counter" */}

` + "```bash\necho hi\n```" + `

{/* docci expect-output name="counter" */}

` + "```json\n{\"height\": \"7\", \"ready\": true}\n```\n"

// Without prose stripping the page itself would name every literal and noise
// would read as load-bearing.
func TestRunDoesNotTreatTheAssertionAsProse(t *testing.T) {
	head := runBase
	head = replaceOnce(head, "\"height\": \"7\"", "\"height\": \"<...>\"")

	verdicts, err := Run(runBase, head, "page.mdx")
	require.NoError(t, err)
	for _, v := range verdicts {
		require.False(t, v.Blocking(), "%+v", v)
	}
}

func TestRunBlocksWhenProseNamesTheValue(t *testing.T) {
	base := runBase + "\nThe height should be 7.\n"
	head := replaceOnce(base, "\"height\": \"7\"", "\"height\": \"<...>\"")

	verdicts, err := Run(base, head, "page.mdx")
	require.NoError(t, err)
	blocking := false
	for _, v := range verdicts {
		blocking = blocking || v.Blocking()
	}
	require.True(t, blocking)
}

func replaceOnce(s, old, repl string) string {
	for i := 0; i+len(old) <= len(s); i++ {
		if s[i:i+len(old)] == old {
			return s[:i] + repl + s[i+len(old):]
		}
	}
	return s
}

func TestRenameReasonNamesTheKeys(t *testing.T) {
	before := Assertion{Key: "k", Kind: "expect", Text: `{"balance": "10"}`}
	after := Assertion{Key: "k", Kind: "expect", Text: `{"fee": "10"}`}

	verdict := Classify(Pair{Key: "k", Kind: "expect", Before: &before, After: &after}, "")
	require.Equal(t, "re-expression", verdict.Class)
	require.Contains(t, verdict.Reason, "balance -> fee")
}

// A rename that also changes the value is a weakening in a rename's clothes.
func TestRenameWithChangedValueFallsThroughToReduction(t *testing.T) {
	before := Assertion{Key: "k", Kind: "expect", Text: `{"balance": "10"}`}
	after := Assertion{Key: "k", Kind: "expect", Text: `{"fee": "<...>"}`}
	prose := "You should see 10 tokens."

	verdict := Classify(Pair{Key: "k", Kind: "expect", Before: &before, After: &after}, prose)
	require.Equal(t, "reduction-load-bearing", verdict.Class)
	require.Contains(t, verdict.Lost, "10")
}
