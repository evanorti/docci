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
