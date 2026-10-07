package review

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractPullsBothAssertionKinds(t *testing.T) {
	page := strings.Join([]string{
		"## Check the counter",
		"",
		"<!-- docci name=\"counter reports a height\" output-contains=\"ready\" -->",
		"",
		"```bash",
		"cat counter.txt",
		"```",
		"",
		"<!-- docci expect-output -->",
		"",
		"```json",
		"{\"height\": \"3\"}",
		"```",
	}, "\n")

	assertions, err := Extract(page, "page.md")
	require.NoError(t, err)
	require.Len(t, assertions, 2)

	require.Equal(t, "contains", assertions[0].Kind)
	require.Equal(t, "ready", assertions[0].Text)
	require.Equal(t, "counter reports a height", assertions[0].Key)

	require.Equal(t, "expect", assertions[1].Kind)
	require.Contains(t, assertions[1].Text, "\"height\": \"3\"")
	require.Equal(t, "counter reports a height", assertions[1].Key)
}

func TestUnnamedKeySurvivesEditAbove(t *testing.T) {
	unnamed := []string{
		"```bash",
		"cat counter.txt",
		"```",
		"",
		"<!-- docci expect-output -->",
		"",
		"```json",
		"{\"height\": \"3\"}",
		"```",
	}
	base := strings.Join(append([]string{"Intro.", ""}, unnamed...), "\n")
	edited := strings.Join(append([]string{"Intro.", "", "A new paragraph inserted above the block.", ""}, unnamed...), "\n")

	before, err := Extract(base, "page.md")
	require.NoError(t, err)
	after, err := Extract(edited, "page.md")
	require.NoError(t, err)
	require.Len(t, before, 1)
	require.Len(t, after, 1)

	require.Equal(t, before[0].Key, after[0].Key)
	require.NotEqual(t, before[0].Line, after[0].Line, "the edit should have moved the block")
}
