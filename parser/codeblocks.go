package parser

import (
	"fmt"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/reecepbcups/docci/logger"
	"github.com/reecepbcups/docci/match"
	"github.com/reecepbcups/docci/types"
)

// CodeBlock represents a parsed code block with its metadata
type CodeBlock struct {
	Index           int
	Language        string
	Content         string
	OutputContains  string
	Background      bool
	BackgroundKill  int // 1-based index of background process to kill
	AssertFailure   bool
	OS              string
	WaitForEndpoint string
	WaitTimeoutSecs int
	RetryCount      int
	DelayBeforeSecs float64
	DelayAfterSecs  float64
	DelayPerCmdSecs float64
	IfFileNotExists string
	IfNotInstalled  string
	LineNumber      int
	FileName        string // Added for debugging multiple files
	ReplaceText     string

	// ExpectOutput is the rendered block beneath this one that the reader is
	// meant to compare their terminal against. Empty when the block has no
	// visible checkpoint.
	ExpectOutput string

	// PageReplacements are the page-level replace-text substitutions, applied
	// before any the block declares itself.
	PageReplacements []string

	// Name, Heading and StepTitle locate the block in the page, so that a
	// failure says which step broke rather than which index.
	Name      string
	Heading   string
	StepTitle string

	// File operation fields
	File        string // file: The file name to operate on
	ResetFile   bool   // reset-file: Reset the file to its original content
	LineInsert  int    // line-insert: Insert content at line N (1-based)
	LineReplace string // line-replace: Replace content at line N or N-M
}

// hasOutputCheck reports whether the block asserts anything about its output.
func (c CodeBlock) hasOutputCheck() bool {
	return c.OutputContains != "" || c.ExpectOutput != ""
}

// outputCheckCommand is the shell test the retry loop runs against a captured
// attempt. It mirrors what ValidateOutputs does afterwards: a tripwire is a
// literal containment test on the raw output, and a checkpoint is matched with
// whitespace collapsed so the rendered block does not have to be byte-exact.
func (c CodeBlock) outputCheckCommand() string {
	capture := fmt.Sprintf("\"$docci_capture_%d\"", c.Index)

	if c.ExpectOutput != "" {
		// A real ESC byte in the sed pattern, so colour escapes are stripped the
		// same way the post-run check strips them, on both BSD and GNU sed.
		stripANSI := "sed 's/\x1b\\[[0-9;]*[a-zA-Z]//g'"
		return fmt.Sprintf("%s < %s | tr -s '[:space:]' ' ' | grep -qE -- %s",
			stripANSI, capture, match.ShellSingleQuote(match.ToERE(c.ExpectOutput)))
	}

	return fmt.Sprintf("grep -qF -- %s %s", match.ShellSingleQuote(c.OutputContains), capture)
}

// Describe locates a block for an error message, preferring whatever a reader
// would recognise: the name the author gave it, the step it sits in, or the
// heading above it.
func (c CodeBlock) Describe() string {
	where := c.FileName
	if where == "" {
		where = "block"
	}
	where = fmt.Sprintf("%s:%d", where, c.LineNumber)

	switch {
	case c.Name != "":
		return fmt.Sprintf("%s (%s)", where, c.Name)
	case c.StepTitle != "":
		return fmt.Sprintf("%s (step %q)", where, c.StepTitle)
	case c.Heading != "":
		return fmt.Sprintf("%s (under %q)", where, c.Heading)
	default:
		return where
	}
}

// given a markdown file, parse out all the code blocks within it.
// Codeblocks are provided as ``` and closed with ```.

var ValidLangs = []string{"bash", "shell", "sh"}

// newCodeBlock creates a new CodeBlock with default values
func newCodeBlock(index int, language string) *CodeBlock {
	return &CodeBlock{
		Index:    index,
		Language: language,
	}
}

// applyTags applies parsed tags to the CodeBlock
func (c *CodeBlock) applyTags(tags MetaTag, lineNumber int, fileName string) {
	c.OutputContains = tags.OutputContains
	c.Background = tags.Background
	c.BackgroundKill = tags.BackgroundKill
	c.AssertFailure = tags.AssertFailure
	c.OS = tags.OS
	c.WaitForEndpoint = tags.WaitForEndpoint
	c.WaitTimeoutSecs = tags.WaitTimeoutSecs
	c.RetryCount = tags.RetryCount
	c.DelayBeforeSecs = tags.DelayBeforeSecs
	c.DelayAfterSecs = tags.DelayAfterSecs
	c.DelayPerCmdSecs = tags.DelayPerCmdSecs
	c.IfFileNotExists = tags.IfFileNotExists
	c.IfNotInstalled = tags.IfNotInstalled
	c.ReplaceText = tags.ReplaceText
	c.File = tags.File
	c.ResetFile = tags.ResetFile
	c.LineInsert = tags.LineInsert
	c.LineReplace = tags.LineReplace
	c.Name = tags.Name
	c.LineNumber = lineNumber
	c.FileName = fileName
}

// GetRetryDelay returns the retry delay in seconds from environment variable or default
func GetRetryDelay() int {
	if delayStr := os.Getenv("DOCCI_RETRY_DELAY"); delayStr != "" {
		if delay, err := strconv.Atoi(delayStr); err == nil && delay >= 0 {
			return delay
		}
	}
	return 2 // Default 2 seconds
}

// headingRe matches an ATX markdown heading outside of a fence.
var headingRe = regexp.MustCompile(`^#{1,6}\s+(.*)$`)

// stepTitleRe matches the title prop of an MDX <Step> component, so that a
// failure can name the step a reader would be on rather than a block index.
var stepTitleRe = regexp.MustCompile(`<Step\s[^>]*title\s*=\s*["']([^"']*)["']`)

// directiveOpenRe matches the start of a docci directive comment, in either the
// MDX form `{/* docci ... */}` or the markdown form `<!-- docci ... -->`.
// An HTML comment is a parse error in MDX and a JSX comment is literal text in
// markdown, so docci reads both and lets the page use whichever it needs.
var directiveOpenRe = regexp.MustCompile(`^\s*(\{/\*|<!--)\s*docci\b`)

// fenceRe matches a fence opening, capturing its indentation, its run of
// backticks, and its info string. The info string belongs to the renderer --
// docci never reads directives out of it.
var fenceRe = regexp.MustCompile("^(\\s*)(`{3,})(.*)$")

// pendingDirectives is a directive comment waiting for the code block it
// describes.
type pendingDirectives struct {
	tags       MetaTag
	lineNumber int
}

// ParseCodeBlocks returns structured code blocks with metadata
func ParseCodeBlocks(markdown string) ([]CodeBlock, error) {
	_, blocks, err := ParseDocument(markdown, "")
	return blocks, err
}

// ParseCodeBlocksWithFileName returns structured code blocks with metadata and filename
func ParseCodeBlocksWithFileName(markdown string, fileName string) ([]CodeBlock, error) {
	_, blocks, err := ParseDocument(markdown, fileName)
	return blocks, err
}

// ParseDocument reads a markdown or MDX page into its page-level config and its
// executable code blocks.
//
// Directives live in comments above the block they describe, never in the fence
// info string. A directive comment attaches to the next fence, with only blank
// lines and other directive comments allowed in between; a comment that
// attaches to nothing is an error rather than a silent skip, because a test
// tool that quietly stops testing is worse than one that fails.
func ParseDocument(document string, sourcePath string) (PageConfig, []CodeBlock, error) {
	log := logger.GetLogger()

	cfg, body, frontmatterLines, err := SplitFrontmatter(document)
	if err != nil {
		return cfg, nil, fmt.Errorf("%s: %w", sourcePath, err)
	}

	var codeBlocks []CodeBlock
	var pending *pendingDirectives
	var heading, stepTitle string

	// Index of the last executable block that was kept, so that an expect-output
	// block can attach its contents to it. -1 means there is nothing to attach to.
	lastExecutable := -1

	lines := splitIntoLines(body)
	for idx := 0; idx < len(lines); idx++ {
		line := lines[idx]
		lineNumber := idx + 1 + frontmatterLines
		trimmed := strings.TrimSpace(line)

		// A directive comment, which may span several lines.
		if directiveOpenRe.MatchString(line) {
			commentBody, consumed, err := readDirectiveComment(lines, idx)
			if err != nil {
				return cfg, nil, fmt.Errorf("%s:%d: %w", sourcePath, lineNumber, err)
			}

			tags, err := ParseDirectives(commentBody)
			if err != nil {
				return cfg, nil, fmt.Errorf("%s:%d: %w", sourcePath, lineNumber, err)
			}

			if pending == nil {
				pending = &pendingDirectives{tags: tags, lineNumber: lineNumber}
			} else {
				pending.tags = mergeTags(pending.tags, tags)
			}

			idx += consumed
			continue
		}

		// A fence opening.
		if m := fenceRe.FindStringSubmatch(line); m != nil {
			indent, backticks, info := m[1], m[2], strings.TrimSpace(m[3])

			content, closed, consumed := readFenceBody(lines, idx, indent, len(backticks))
			if !closed {
				return cfg, nil, fmt.Errorf("%s:%d: unterminated code fence", sourcePath, lineNumber)
			}

			lang := ""
			if fields := strings.Fields(info); len(fields) > 0 {
				lang = fields[0]
			}

			var tags MetaTag
			if pending != nil {
				tags = pending.tags
			}

			switch {
			case tags.ExpectOutput:
				if lastExecutable < 0 {
					return cfg, nil, fmt.Errorf("%s:%d: expect-output has no code block above it to check", sourcePath, lineNumber)
				}
				if codeBlocks[lastExecutable].ExpectOutput != "" {
					return cfg, nil, fmt.Errorf("%s:%d: %s already has an expected output block", sourcePath, lineNumber, codeBlocks[lastExecutable].Describe())
				}
				codeBlocks[lastExecutable].ExpectOutput = content
				log.Debug("Attached expected output", "block", codeBlocks[lastExecutable].Index)

			case tags.Ignore:
				log.Debug("Ignoring code block due to ignore directive", "line", lineNumber)

			case contains(ValidLangs, lang) || tags.File != "":
				if err := tags.Validate(lineNumber); err != nil {
					return cfg, nil, fmt.Errorf("%s: %w", sourcePath, err)
				}

				if !ShouldRunOnCurrentOS(tags.OS) || !ShouldRunBasedOnCommandInstallation(tags.IfNotInstalled) {
					log.Debug("Skipping code block", "required_os", tags.OS, "current_os", GetCurrentOS(), "line", lineNumber)
					// Nothing to attach expected output to.
					lastExecutable = -1
					break
				}

				block := newCodeBlock(len(codeBlocks)+1, lang)
				block.applyTags(tags, lineNumber, sourcePath)
				block.Heading = heading
				block.StepTitle = stepTitle
				block.PageReplacements = cfg.ReplaceText
				block.Content = content
				codeBlocks = append(codeBlocks, *block)
				lastExecutable = len(codeBlocks) - 1

			default:
				// A fence docci has no business running: a config sample, a
				// rendered JSON response, a snippet in another language.
				if pending != nil {
					return cfg, nil, fmt.Errorf("%s:%d: directives attach to a %q block, which docci does not execute. Use expect-output, or remove the directives",
						sourcePath, pending.lineNumber, lang)
				}
			}

			pending = nil
			idx += consumed
			continue
		}

		// Track where we are in the page, for error messages.
		if m := headingRe.FindStringSubmatch(line); m != nil {
			heading = strings.TrimSpace(m[1])
			stepTitle = ""
		}
		if m := stepTitleRe.FindStringSubmatch(line); m != nil {
			stepTitle = m[1]
		}

		// Anything else between a directive comment and its block breaks the
		// attachment, and is far more likely to be a mistake than an intent.
		if pending != nil && trimmed != "" {
			return cfg, nil, fmt.Errorf("%s:%d: directives are followed by prose, not a code block. A docci comment attaches to the code block directly below it",
				sourcePath, pending.lineNumber)
		}
	}

	if pending != nil {
		return cfg, nil, fmt.Errorf("%s:%d: directives at the end of the page attach to no code block", sourcePath, pending.lineNumber)
	}

	if err := validateBackgroundKills(codeBlocks); err != nil {
		return cfg, nil, err
	}

	return cfg, codeBlocks, nil
}

// readDirectiveComment returns the body of a directive comment -- everything
// after the `docci` keyword and before the closing delimiter -- along with how
// many extra lines it spanned.
func readDirectiveComment(lines []string, start int) (string, int, error) {
	open := lines[start]
	closer := "*/}"
	if strings.Contains(open, "<!--") {
		closer = "-->"
	}

	var body strings.Builder
	for i := start; i < len(lines); i++ {
		line := lines[i]
		if i == start {
			line = directiveOpenRe.ReplaceAllString(line, "")
		}
		if cut := strings.Index(line, closer); cut >= 0 {
			body.WriteString(line[:cut])
			return body.String(), i - start, nil
		}
		body.WriteString(line)
		body.WriteString(" ")
	}

	return "", 0, fmt.Errorf("unterminated docci comment: expected %q", closer)
}

// readFenceBody returns the contents of a fence, with the fence's own
// indentation stripped so that a block nested inside <Steps> or <CodeGroup>
// executes the same as one at the top level.
func readFenceBody(lines []string, start int, indent string, backtickCount int) (string, bool, int) {
	var content strings.Builder

	for i := start + 1; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if isFenceClose(trimmed, backtickCount) {
			return content.String(), true, i - start
		}

		content.WriteString(stripIndent(line, indent))
		content.WriteString("\n")
	}

	return content.String(), false, len(lines) - start - 1
}

// isFenceClose reports whether a line is a closing fence of at least the
// opening fence's length.
func isFenceClose(trimmed string, backtickCount int) bool {
	if len(trimmed) < backtickCount {
		return false
	}
	for _, r := range trimmed {
		if r != '`' {
			return false
		}
	}
	return true
}

// stripIndent removes up to the fence's indentation from a content line,
// leaving any deeper indentation the code itself relies on.
func stripIndent(line string, indent string) string {
	for i := 0; i < len(indent); i++ {
		if len(line) == 0 || (line[0] != ' ' && line[0] != '\t') {
			break
		}
		line = line[1:]
	}
	return line
}

// mergeTags folds a second directive comment into a first, so that consecutive
// comments above one block behave as though they were written as one. A value
// set by the later comment wins.
func mergeTags(into MetaTag, from MetaTag) MetaTag {
	dst := reflect.ValueOf(&into).Elem()
	src := reflect.ValueOf(from)

	for i := 0; i < src.NumField(); i++ {
		field := src.Field(i)
		if field.IsZero() {
			continue
		}
		dst.Field(i).Set(field)
	}

	return into
}

// validateBackgroundKills checks that every background-kill names a background
// process that actually exists.
func validateBackgroundKills(codeBlocks []CodeBlock) error {
	backgroundIndexes := make(map[int]bool)
	for _, block := range codeBlocks {
		if block.Background {
			backgroundIndexes[block.Index] = true
		}
	}

	for _, block := range codeBlocks {
		if block.BackgroundKill == 0 {
			continue
		}
		if backgroundIndexes[block.BackgroundKill] {
			continue
		}

		var availableIndexes []int
		for idx := range backgroundIndexes {
			availableIndexes = append(availableIndexes, idx)
		}
		sort.Ints(availableIndexes)

		if len(availableIndexes) == 0 {
			return fmt.Errorf("%s: background-kill=%d references a non-existent background process. No background processes are defined in this file",
				block.Describe(), block.BackgroundKill)
		}
		return fmt.Errorf("%s: background-kill=%d references a non-existent background process. Available background process indexes: %v",
			block.Describe(), block.BackgroundKill, availableIndexes)
	}

	return nil
}

// WaitForEndpoint polls an HTTP endpoint until it's ready or timeout is reached
func WaitForEndpoint(url string, timeoutSecs int) error {
	log := logger.GetLogger()
	log.Info("Waiting for endpoint to be ready", "url", url, "timeout_secs", timeoutSecs)

	timeout := time.Duration(timeoutSecs) * time.Second
	client := &http.Client{
		Timeout: 5 * time.Second, // 5 second timeout per request
	}

	start := time.Now()
	for {
		if time.Since(start) >= timeout {
			return fmt.Errorf("timeout waiting for endpoint %s after %d seconds", url, timeoutSecs)
		}

		resp, err := client.Get(url)
		if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
			resp.Body.Close()
			log.Info("Endpoint is ready", "url", url, "status", resp.StatusCode)
			return nil
		}

		if resp != nil {
			resp.Body.Close()
		}

		log.Debug("Endpoint not ready yet, retrying in 1 second", "url", url)
		time.Sleep(1 * time.Second)
	}
}

// BuildExecutableScript creates a single script with validation markers
func BuildExecutableScript(blocks []CodeBlock) (string, map[int]types.Validation, map[int]bool) {
	return BuildExecutableScriptWithOptions(blocks, types.DocciOpts{
		HideBackgroundLogs: false,
		KeepRunning:        false,
	})
}

// BuildExecutableScriptWithOptions creates a single script with validation markers and options
func BuildExecutableScriptWithOptions(blocks []CodeBlock, opts types.DocciOpts) (string, map[int]types.Validation, map[int]bool) {
	log := logger.GetLogger()
	var script strings.Builder
	validationMap := make(map[int]types.Validation) // maps block index to what its output must satisfy
	assertFailureMap := make(map[int]bool)          // maps block index to assert-failure flag
	var backgroundPIDs []string
	debugEnabled := logger.IsDebugEnabled()

	// Always generate markers for parsing, visibility controlled in executor

	// Add trap at the beginning to clean up background processes
	// Only set the trap if keepRunning is false
	if !opts.KeepRunning {
		script.WriteString(replaceTemplateVars(scriptCleanupTemplate, map[string]string{
			"DEBUG_CLEANUP": formatDebugCleanup(debugEnabled),
		}))
	}

	var backgroundIndexes []int

	for _, block := range blocks {
		// Handle background kill first if specified
		if block.BackgroundKill > 0 {
			script.WriteString(replaceTemplateVars(backgroundKillTemplate, map[string]string{
				"KILL_INDEX": strconv.Itoa(block.BackgroundKill),
				"FILE_INFO":  formatFileInfo(block.FileName),
			}))
		}

		if block.Background {
			// For background blocks, wrap in { } & and redirect output
			script.WriteString(replaceTemplateVars(backgroundBlockTemplate, map[string]string{
				"INDEX":     strconv.Itoa(block.Index),
				"FILE_INFO": formatFileInfo(block.FileName),
				"CONTENT":   block.Content,
			}))
			backgroundPIDs = append(backgroundPIDs, fmt.Sprintf("$DOCCI_BG_PID_%d", block.Index))
			backgroundIndexes = append(backgroundIndexes, block.Index)
		} else {
			// Regular blocks with markers (always generated for parsing)
			script.WriteString(replaceTemplateVars(blockStartMarkerTemplate, map[string]string{
				"INDEX": strconv.Itoa(block.Index),
			}))

			// Add the block header comment only in debug mode
			if debugEnabled {
				script.WriteString(replaceTemplateVars(blockHeaderTemplate, map[string]string{
					"INDEX":     strconv.Itoa(block.Index),
					"LANGUAGE":  block.Language,
					"FILE_INFO": formatFileInfo(block.FileName),
				}))
			}

			// Add delay before block if specified
			if block.DelayBeforeSecs > 0 {
				script.WriteString(replaceTemplateVars(delayBeforeTemplate, map[string]string{
					"INDEX": strconv.Itoa(block.Index),
					"DELAY": strconv.FormatFloat(block.DelayBeforeSecs, 'g', -1, 64),
				}))
			}

			// Add wait-for-endpoint logic if needed
			if block.WaitForEndpoint != "" {
				script.WriteString(replaceTemplateVars(waitForEndpointTemplate, map[string]string{
					"ENDPOINT": block.WaitForEndpoint,
					"TIMEOUT":  strconv.Itoa(block.WaitTimeoutSecs),
				}))
			}

			// Add file existence check as guard clause if needed
			if block.IfFileNotExists != "" {
				script.WriteString(replaceTemplateVars(fileExistenceGuardStartTemplate, map[string]string{
					"FILE":  block.IfFileNotExists,
					"INDEX": strconv.Itoa(block.Index),
				}))
			}

			// Apply text replacements, page-level ones first so a block can
			// still override what the page did.
			blockContent := block.Content
			for _, replacement := range append(append([]string{}, block.PageReplacements...), block.ReplaceText) {
				if replacement == "" {
					continue
				}
				parts := strings.SplitN(replacement, ";", 2)
				if len(parts) != 2 {
					continue
				}
				blockContent = strings.ReplaceAll(blockContent, parts[0], parts[1])
				log.Debug("Applied text replacement", "block", block.Index, "old", parts[0], "new", parts[1])
			}

			// Check if this is a file operation block
			if block.File != "" {
				// Handle file operations
				if block.ResetFile || block.LineInsert == 0 && block.LineReplace == "" {
					// Create or reset file
					operation := "create"
					if block.ResetFile {
						operation = "reset"
					}
					script.WriteString(replaceTemplateVars(fileCreateOrResetTemplate, map[string]string{
						"OPERATION": operation,
						"FILE":      block.File,
						"FILE_INFO": formatFileInfo(block.FileName),
						"CONTENT":   blockContent,
					}))
				} else if block.LineInsert > 0 {
					// Insert at line
					script.WriteString(replaceTemplateVars(fileLineInsertTemplate, map[string]string{
						"FILE":      block.File,
						"LINE":      strconv.Itoa(block.LineInsert),
						"FILE_INFO": formatFileInfo(block.FileName),
						"CONTENT":   blockContent,
					}))
				} else if block.LineReplace != "" {
					// Replace line(s)
					startLine := ""
					endLine := ""

					if strings.Contains(block.LineReplace, "-") {
						parts := strings.Split(block.LineReplace, "-")
						startLine = strings.TrimSpace(parts[0])
						endLine = strings.TrimSpace(parts[1])
					} else {
						startLine = block.LineReplace
						endLine = block.LineReplace
					}

					script.WriteString(replaceTemplateVars(fileLineReplaceTemplate, map[string]string{
						"FILE":       block.File,
						"LINES":      block.LineReplace,
						"START_LINE": startLine,
						"END_LINE":   endLine,
						"FILE_INFO":  formatFileInfo(block.FileName),
						"CONTENT":    blockContent,
					}))
				}
			} else {
				// Regular code execution (not a file operation)
				// Prepare the code content with per-command delay and command display
				delaySeconds := block.DelayPerCmdSecs
				codeContent := replaceTemplateVars(codeExecutionTemplate, map[string]string{
					"DELAY":      strconv.FormatFloat(delaySeconds, 'g', -1, 64),
					"BASH_FLAGS": formatBashFlags(block.AssertFailure),
					"CONTENT":    blockContent,
				})

				// Add the actual code with retry logic if needed
				if block.RetryCount > 0 && block.hasOutputCheck() {
					retryDelay := GetRetryDelay()
					script.WriteString(replaceTemplateVars(retryUntilOutputStartTemplate, map[string]string{
						"INDEX":       strconv.Itoa(block.Index),
						"MAX_RETRIES": strconv.Itoa(block.RetryCount),
						"RETRY_DELAY": strconv.Itoa(retryDelay),
						"LABEL":       block.Describe(),
					}))
					script.WriteString(codeContent)
					script.WriteString(replaceTemplateVars(retryUntilOutputEndTemplate, map[string]string{
						"INDEX": strconv.Itoa(block.Index),
						"CHECK": block.outputCheckCommand(),
						"LABEL": block.Describe(),
					}))
				} else if block.RetryCount > 0 {
					retryDelay := GetRetryDelay()
					script.WriteString(replaceTemplateVars(retryWrapperStartTemplate, map[string]string{
						"INDEX":       strconv.Itoa(block.Index),
						"MAX_RETRIES": strconv.Itoa(block.RetryCount),
						"RETRY_DELAY": strconv.Itoa(retryDelay),
					}))
					script.WriteString(codeContent)
					script.WriteString(replaceTemplateVars(retryWrapperEndTemplate, map[string]string{
						"INDEX": strconv.Itoa(block.Index),
					}))
				} else {
					script.WriteString(codeContent)
				}
			}

			// Close the guard clause if needed
			if block.IfFileNotExists != "" {
				script.WriteString("fi\n")
			}

			// Add delay after block if specified
			if block.DelayAfterSecs > 0 {
				script.WriteString(replaceTemplateVars(delayAfterTemplate, map[string]string{
					"INDEX": strconv.Itoa(block.Index),
					"DELAY": strconv.FormatFloat(block.DelayAfterSecs, 'g', -1, 64),
				}))
			}

			// Add a marker after the block
			script.WriteString(replaceTemplateVars(blockEndMarkerTemplate, map[string]string{
				"INDEX": strconv.Itoa(block.Index),
			}))

			// Store validation requirement if present
			if block.OutputContains != "" || block.ExpectOutput != "" {
				validationMap[block.Index] = types.Validation{
					Label:    block.Describe(),
					Contains: block.OutputContains,
					Expect:   block.ExpectOutput,
				}
			}
			// Store assert-failure requirement if present
			if block.AssertFailure {
				assertFailureMap[block.Index] = true
			}
		}
	}

	// Add section to display background logs at the end (unless hidden)
	if len(backgroundIndexes) > 0 && !opts.HideBackgroundLogs {
		var logEntries strings.Builder
		for _, bgIndex := range backgroundIndexes {
			logEntries.WriteString(replaceTemplateVars(backgroundLogEntryTemplate, map[string]string{
				"INDEX": strconv.Itoa(bgIndex),
			}))
		}
		script.WriteString(replaceTemplateVars(backgroundLogsDisplayTemplate, map[string]string{
			"LOG_ENTRIES": logEntries.String(),
		}))
	} else if len(backgroundIndexes) > 0 && opts.HideBackgroundLogs {
		// Still clean up the background output files even if we're not displaying them
		var cleanupCommands strings.Builder
		for _, bgIndex := range backgroundIndexes {
			cleanupCommands.WriteString(fmt.Sprintf("rm -f /tmp/docci_bg_%d.out\n", bgIndex))
		}
		script.WriteString(replaceTemplateVars(backgroundLogsCleanupTemplate, map[string]string{
			"CLEANUP_COMMANDS": cleanupCommands.String(),
		}))
	}

	// Add infinite sleep if keepRunning is true (as a final block)
	if opts.KeepRunning {
		script.WriteString(replaceTemplateVars(keepRunningTemplate, map[string]string{
			"DEBUG_CLEANUP": formatDebugCleanup(debugEnabled),
		}))
	}

	return script.String(), validationMap, assertFailureMap
}
