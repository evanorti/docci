package executor

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/reecepbcups/docci/logger"
	"github.com/reecepbcups/docci/match"
	"github.com/reecepbcups/docci/types"
)

type ExecResponse struct {
	ExitCode uint
	Error    error // only if ExitCode != 0
	Stdout   string
	Stderr   string
}

func NewExecResponse(exitCode uint, stdout, stderr string, err error) ExecResponse {
	return ExecResponse{
		ExitCode: exitCode,
		Error:    err,
		Stdout:   stdout,
		Stderr:   stderr,
	}
}

// Exec runs a specific codeblock in a bash shell.
// returns exit (status code, error message)

func Exec(commands string) (ExecResponse, error) {
	log := logger.GetLogger()
	log.Debug("Executing commands in bash shell")

	cmd := exec.Command("bash", "-c", commands)
	cmd.Env = append(os.Environ(), "IS_DOCCI_RUN=true")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return ExecResponse{}, fmt.Errorf("create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return ExecResponse{}, fmt.Errorf("create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return ExecResponse{}, fmt.Errorf("start command: %w", err)
	}

	var stdoutBuf, stderrBuf strings.Builder // captures output for further validation
	var mu sync.Mutex                        // For thread-safe string builder access

	// Create goroutines to read both stdout and stderr concurrently
	done := make(chan bool, 2)

	// Handle stdout
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			if line != "" {
				// Don't print DOCCI markers and cleanup messages to stdout
				shouldPrint := true

				if strings.Contains(line, "DOCCI_BLOCK_START_") || strings.Contains(line, "DOCCI_BLOCK_END_") {
					shouldPrint = false
				}
				if strings.Contains(line, "Cleaning up background processes") {
					shouldPrint = false
				}
				// Don't show "=== Code Block" headers
				if strings.Contains(line, "=== Code Block") {
					shouldPrint = false
				}

				if shouldPrint {
					io.WriteString(os.Stdout, line+"\n")
				}
				// Always capture in buffer for validation
				mu.Lock()
				stdoutBuf.WriteString(line + "\n")
				mu.Unlock()
			}
		}
		done <- true
	}()

	// Handle stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			if line != "" {

				// TODO: DevEx:
				// if error like `bash: -c: line 3: unexpected EOF while looking for matching `"'`
				// show the actual line number in the file / code block section to help debug.
				// This case above is when you forget to add a closing quote to an echo line.

				io.WriteString(os.Stderr, line+"\n")
				mu.Lock()
				stderrBuf.WriteString(line + "\n")
				mu.Unlock()
			}
		}
		done <- true
	}()

	// Wait for both goroutines to finish
	<-done
	<-done

	if err := cmd.Wait(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode := exitError.ExitCode()
			exitErr := exitError.Error()
			log.Debug("Command exited with code", "exitCode", exitCode, "error", exitErr)
			return NewExecResponse(uint(exitCode), stdoutBuf.String(), stderrBuf.String(), fmt.Errorf(exitError.Error())), nil
		} else {
			return ExecResponse{}, fmt.Errorf("wait command: %w", err)
		}
	}

	log.Debug("Command executed successfully")
	return NewExecResponse(0, stdoutBuf.String(), stderrBuf.String(), nil), nil
}

// blockMarker matches a block boundary wherever it appears. A command whose
// output has no trailing newline leaves the marker on the same line as that
// output, so the marker cannot be matched as a whole line: it would be missed,
// the block would record nothing, and the page would fail with "produced no
// output to check". curl against an endpoint that returns bare JSON does this
// every time.
var blockMarker = regexp.MustCompile(`### DOCCI_BLOCK_(START|END)_(\d+) ###`)

// ParseBlockOutputs extracts output for each code block based on markers
func ParseBlockOutputs(output string) map[int]string {
	log := logger.GetLogger()
	log.Debug("Parsing block outputs from execution result")
	blockOutputs := make(map[int]string)

	var currentBlock int
	var currentOutput strings.Builder
	inBlock := false
	cursor := 0

	flush := func(upTo int) {
		if !inBlock {
			return
		}
		currentOutput.WriteString(output[cursor:upTo])
	}

	for _, m := range blockMarker.FindAllStringSubmatchIndex(output, -1) {
		flush(m[0])
		kind := output[m[2]:m[3]]
		var index int
		fmt.Sscanf(output[m[4]:m[5]], "%d", &index)

		switch kind {
		case "START":
			currentBlock = index
			currentOutput.Reset()
			inBlock = true
			log.Debug("Found start marker for block", "block", currentBlock)
		case "END":
			if inBlock {
				blockOutputs[currentBlock] = cleanBlockOutput(currentOutput.String())
				log.Debug("Found end marker for block", "block", currentBlock,
					"capturedOutputLength", len(blockOutputs[currentBlock]))
			}
			inBlock = false
		}
		cursor = m[1]
	}

	log.Debug("Parsed block outputs", "count", len(blockOutputs))
	return blockOutputs
}

// cleanBlockOutput drops the headers docci prints between blocks, which are
// not output the page produced.
func cleanBlockOutput(raw string) string {
	kept := make([]string, 0, 8)
	for _, line := range strings.Split(raw, "\n") {
		if strings.HasPrefix(line, "### === Code Block") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.TrimSpace(strings.Join(kept, "\n"))
}

// ValidateOutputs checks if block outputs contain expected strings
func ValidateOutputs(blockOutputs map[int]string, validationMap map[int]types.Validation) []error {
	log := logger.GetLogger()
	log.Debug("Validating block outputs")
	var errors []error

	// Sort so that failures are reported in page order rather than map order.
	indexes := make([]int, 0, len(validationMap))
	for blockIndex := range validationMap {
		indexes = append(indexes, blockIndex)
	}
	sort.Ints(indexes)

	for _, blockIndex := range indexes {
		validation := validationMap[blockIndex]

		output, exists := blockOutputs[blockIndex]
		if !exists {
			log.Error("No output found for block", "block", validation.Label)
			errors = append(errors, fmt.Errorf("%s: produced no output to check", validation.Label))
			continue
		}

		if validation.Contains != "" && !strings.Contains(output, validation.Contains) {
			log.Error("Block output does not contain expected string", "block", validation.Label, "expected", validation.Contains)
			errors = append(errors, fmt.Errorf("%s: output does not contain %q\nActual output:\n%s",
				validation.Label, validation.Contains, output))
			continue
		}

		if validation.Expect != "" && !match.Matches(output, validation.Expect) {
			log.Error("Block output does not match its checkpoint", "block", validation.Label)
			errors = append(errors, fmt.Errorf("%s: output does not match the expected output shown on the page.\n%s\n\nExpected:\n%s\n\nActual output:\n%s",
				validation.Label, match.DescribeMismatch(output, validation.Expect), validation.Expect, output))
			continue
		}

		log.Debug("Block validation passed", "block", validation.Label)
	}

	return errors
}
