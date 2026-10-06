package parser

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDirectiveCommentAttachesToNextBlock(t *testing.T) {
	page := strings.Join([]string{
		"{/* docci retry=3 name=\"check health\" */}",
		"",
		"```bash",
		"echo hi",
		"```",
	}, "\n")

	blocks, err := ParseCodeBlocks(page)
	require.NoError(t, err)
	require.Len(t, blocks, 1)
	require.Equal(t, 3, blocks[0].RetryCount)
	require.Equal(t, "check health", blocks[0].Name)
}

func TestMultiLineDirectiveComment(t *testing.T) {
	page := strings.Join([]string{
		"{/* docci",
		"  retry=2",
		"  output-contains=\"ok\"",
		"*/}",
		"",
		"```bash",
		"echo ok",
		"```",
	}, "\n")

	blocks, err := ParseCodeBlocks(page)
	require.NoError(t, err)
	require.Len(t, blocks, 1)
	require.Equal(t, 2, blocks[0].RetryCount)
	require.Equal(t, "ok", blocks[0].OutputContains)
}

func TestHTMLCommentFormIsReadToo(t *testing.T) {
	page := "<!-- docci retry=4 -->\n\n```bash\necho hi\n```"

	blocks, err := ParseCodeBlocks(page)
	require.NoError(t, err)
	require.Len(t, blocks, 1)
	require.Equal(t, 4, blocks[0].RetryCount)
}

// A directive comment separated from its block by prose is far more likely to
// be a mistake than an intent, and silently ignoring it means the page stops
// being tested without anyone noticing.
func TestDirectivesFollowedByProseIsAnError(t *testing.T) {
	page := "{/* docci retry=3 */}\n\nSome prose got in the way.\n\n```bash\necho hi\n```"

	_, err := ParseCodeBlocks(page)
	require.Error(t, err)
	require.Contains(t, err.Error(), "attaches to the code block directly below it")
}

func TestDirectivesAtEndOfPageIsAnError(t *testing.T) {
	page := "```bash\necho hi\n```\n\n{/* docci retry=3 */}\n"

	_, err := ParseCodeBlocks(page)
	require.Error(t, err)
	require.Contains(t, err.Error(), "attach to no code block")
}

// The fence info string belongs to the renderer. Anything in it is left alone.
func TestInfoStringIsNotReadForDirectives(t *testing.T) {
	page := "```bash title=\"Terminal\" highlight={1-2}\necho hi\n```"

	blocks, err := ParseCodeBlocks(page)
	require.NoError(t, err)
	require.Len(t, blocks, 1)
	require.Equal(t, "bash", blocks[0].Language)
	require.Zero(t, blocks[0].RetryCount)
}

func TestIndentedFenceInsideMDXComponent(t *testing.T) {
	page := strings.Join([]string{
		"<Steps>",
		"  <Step title=\"Do the thing\">",
		"    ```bash",
		"    if true; then",
		"      echo nested",
		"    fi",
		"    ```",
		"  </Step>",
		"</Steps>",
	}, "\n")

	blocks, err := ParseCodeBlocks(page)
	require.NoError(t, err)
	require.Len(t, blocks, 1)
	require.Equal(t, "Do the thing", blocks[0].StepTitle)
	// The fence's own indentation is stripped; the code's is not.
	require.Equal(t, "if true; then\n  echo nested\nfi\n", blocks[0].Content)
}

func TestExpectOutputAttachesToTheBlockAbove(t *testing.T) {
	page := strings.Join([]string{
		"```bash",
		"echo hi",
		"```",
		"",
		"{/* docci expect-output */}",
		"",
		"```json",
		"{\"ok\": true}",
		"```",
	}, "\n")

	blocks, err := ParseCodeBlocks(page)
	require.NoError(t, err)
	require.Len(t, blocks, 1, "the rendered output block is not executed")
	require.Contains(t, blocks[0].ExpectOutput, `{"ok": true}`)
}

func TestExpectOutputWithNothingAboveIsAnError(t *testing.T) {
	page := "{/* docci expect-output */}\n\n```json\n{}\n```"

	_, err := ParseCodeBlocks(page)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no code block above it")
}

func TestHeadingGivesABlockItsLocation(t *testing.T) {
	page := "## Start the chain\n\n```bash\necho hi\n```"

	blocks, err := ParseCodeBlocks(page)
	require.NoError(t, err)
	require.Equal(t, "Start the chain", blocks[0].Heading)
	require.Contains(t, blocks[0].Describe(), "Start the chain")
}

func TestFrontmatterIsReadAndLineNumbersSurviveIt(t *testing.T) {
	page := strings.Join([]string{
		"---",
		"title: \"A page\"",
		"docci:",
		"  runnable: false",
		"  needs:",
		"    - ./other",
		"  cleanup:",
		"    - rm -f thing",
		"---",
		"",
		"```bash",
		"echo hi",
		"```",
	}, "\n")

	cfg, blocks, err := ParseDocument(page, "page.mdx")
	require.NoError(t, err)
	require.False(t, cfg.IsRunnable())
	require.Equal(t, []string{"./other"}, cfg.Needs)
	require.Equal(t, []string{"rm -f thing"}, cfg.Cleanup)
	require.Len(t, blocks, 1)
	require.Equal(t, 11, blocks[0].LineNumber, "line numbers should point at the real file line")
}

func TestPageWithoutFrontmatterIsUnchanged(t *testing.T) {
	cfg, blocks, err := ParseDocument("```bash\necho hi\n```", "page.md")
	require.NoError(t, err)
	require.True(t, cfg.IsRunnable())
	require.Len(t, blocks, 1)
}

// A background block used to skip text replacement entirely, so a page that
// substitutes an isolated home directory or a test endpoint would run the
// unsubstituted command. That failure is silent and can be destructive.
func TestBackgroundBlocksGetTextReplacements(t *testing.T) {
	page := strings.Join([]string{
		"---",
		"docci:",
		"  replace-text:",
		"    - \"serve;serve --home /sandbox\"",
		"---",
		"",
		"<!-- docci background -->",
		"",
		"```bash",
		"myapp serve",
		"```",
	}, "\n")

	_, blocks, err := ParseDocument(page, "page.md")
	require.NoError(t, err)
	require.Len(t, blocks, 1)

	script, _, _ := BuildExecutableScript(blocks)
	require.Contains(t, script, "myapp serve --home /sandbox")
	require.NotContains(t, script, "myapp serve\n")
}

func TestPageReplacementsApplyBeforeBlockOnes(t *testing.T) {
	page := strings.Join([]string{
		"---",
		"docci:",
		"  replace-text:",
		"    - \"HOST;example.test\"",
		"---",
		"",
		"<!-- docci replace-text=\"PORT;8080\" -->",
		"",
		"```bash",
		"curl http://HOST:PORT/",
		"```",
	}, "\n")

	_, blocks, err := ParseDocument(page, "page.md")
	require.NoError(t, err)

	script, _, _ := BuildExecutableScript(blocks)
	require.Contains(t, script, "curl http://example.test:8080/")
}

// A reader watching their terminal sees stdout and stderr interleaved, so a
// page describes both. A tool that logs to stderr -- which most CLIs do --
// would otherwise make a page's claim true for a human and false for CI.
func TestBlockOutputIncludesStderr(t *testing.T) {
	page := "<!-- docci output-contains=\"from stderr\" -->\n\n```bash\necho 'from stderr' >&2\n```"

	blocks, err := ParseCodeBlocks(page)
	require.NoError(t, err)

	script, _, _ := BuildExecutableScript(blocks)
	require.Contains(t, script, "exec 2>&1", "command stderr has to reach the block's captured output")
	require.Contains(t, script, "exec 9>&2", "docci's own narration needs a channel of its own")
}

// docci's per-command narration must not land in a block's output, or an
// assertion could match the text of its own command rather than its result.
func TestNarrationStaysOutOfBlockOutput(t *testing.T) {
	blocks, err := ParseCodeBlocks("```bash\necho hi\n```")
	require.NoError(t, err)

	script, _, _ := BuildExecutableScript(blocks)
	require.Contains(t, script, "Executing CMD: $BASH_COMMAND\" >&9")
	require.NotContains(t, script, "Executing CMD: $BASH_COMMAND\" >&2")
}
