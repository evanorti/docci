package review

import (
	"regexp"
	"strings"
)

// noisePatterns match values that change between runs and carry no claim about
// whether a step worked. They are never load-bearing, even when the prose
// happens to mention them.
var noisePatterns = []*regexp.Regexp{
	regexp.MustCompile(`^\d{1,2}:\d{2}:\d{2}$`),   // 12:34:56
	regexp.MustCompile(`^\d{4}-\d{2}-\d{2}`),      // 2026-10-07...
	regexp.MustCompile(`^\d+(\.\d+)?(ms|s|m|h)$`), // 1m4s, 250ms
	regexp.MustCompile(`^\d+[hms]\d+[ms]$`),       // 1m4s split form
	regexp.MustCompile(`^(/|~/)[^\s]*$`),          // absolute paths
	regexp.MustCompile(`^0x[0-9a-fA-F]{40,}$`),    // addresses and hashes
}

// terminalStates are values whose whole purpose is to say a step succeeded or
// failed. Losing one means losing the page's ability to tell the difference.
var terminalStates = map[string]bool{
	"true": true, "false": true, "null": true,
	"SUCCEEDED": true, "FAILED": true, "PENDING": true,
	"valid": true, "invalid": true, "ready": true, "healthy": true,
}

// wildcard is docci's placeholder for output that varies; it asserts nothing.
const wildcard = "<...>"

// structureReplacer blanks JSON structure and prose quoting, so that what is
// left is whitespace-separated values. Colons are not here on purpose: they
// live inside timestamps, and only a trailing one marks a key.
var structureReplacer = strings.NewReplacer(
	"{", " ", "}", " ", "[", " ", "]", " ", ",", " ", "\"", " ", "`", " ", wildcard, " ",
)

// tokenize splits text into whole values as they appear, never fragments, so
// that a timestamp or a path reaches the noise patterns intact. Literals and
// LoadBearing both use it. Two tokenizers would drift apart, and a literal
// extracted one way would then fail to match the prose tokenized another.
func tokenize(text string) []string {
	var tokens []string
	for _, field := range strings.Fields(structureReplacer.Replace(text)) {
		// Trailing punctuation is a sentence or a key, not part of the value.
		field = strings.Trim(field, ".;:!?()")
		if field != "" {
			tokens = append(tokens, field)
		}
	}
	return tokens
}

// Literals returns the comparable values in an assertion's text, in order of
// first appearance and without duplicates.
func Literals(text string) []string {
	seen := make(map[string]bool)
	var literals []string
	for _, token := range tokenize(text) {
		if seen[token] {
			continue
		}
		seen[token] = true
		literals = append(literals, token)
	}
	return literals
}

// LoadBearing reports whether losing this literal costs the page something it
// could previously catch.
//
// Two ways to qualify: the value says whether the step succeeded, or the prose
// around the block names it, which makes it load-bearing by construction --
// text promising "you should see 10 tokens" is wrong the moment the assertion
// stops checking the 10.
func LoadBearing(literal string, prose string) bool {
	for _, noise := range noisePatterns {
		if noise.MatchString(literal) {
			return false
		}
	}

	if terminalStates[literal] {
		return true
	}

	// Whole-token, case-insensitive membership: "1" must not match inside
	// "100" or "2026", and "Height" in prose names "height" in output. A
	// literal that still holds a wildcard is load-bearing when any fragment
	// of it is named, because missing a match lets a reduction through
	// unchecked, and over-matching only makes the agent report.
	named := make(map[string]bool)
	for _, token := range tokenize(prose) {
		named[strings.ToLower(token)] = true
		for _, piece := range pieces(token) {
			named[strings.ToLower(piece)] = true
		}
	}
	for _, fragment := range tokenize(literal) {
		if named[strings.ToLower(fragment)] {
			return true
		}
	}
	return false
}

// piecePattern splits a prose token at boundaries between digits, letters and
// symbols, so "10%" and "10x" still name the value 10. It applies only to the
// prose side: splitting extracted literals would turn a timestamp back into
// fragments, which is the bug tokenize exists to prevent.
var piecePattern = regexp.MustCompile(`[0-9]+|[A-Za-z]+|[^\sA-Za-z0-9]+`)

func pieces(token string) []string {
	return piecePattern.FindAllString(token, -1)
}
