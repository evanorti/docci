package match

import (
	"fmt"
	"regexp"
	"strings"
)

// Wildcard stands for a span of expected output that changes between runs --
// a block height, a hash, a timestamp, a duration. It reads as a placeholder to
// a human, which matters because this text renders on the page, and matches any
// span for docci.
const Wildcard = "<...>"

// wildcardEscape is substituted for a literal `<...>` that the author escaped as
// `\<...>`, for the rare output that really does contain the placeholder.
const wildcardEscape = `\<...>`

// Matching is "contains", not equality: real output is surrounded by log lines,
// prompts and progress that no page should have to reproduce. Whitespace is
// collapsed on both sides, so a rendered block that was re-indented to fit the
// page still matches the terminal it was copied from.
// Matches reports whether actual output satisfies an expected checkpoint.
func Matches(actual, expected string) bool {
	pattern, err := toPattern(expected)
	if err != nil {
		return false
	}
	return pattern.MatchString(collapseWhitespace(actual))
}

// toPattern compiles an expected block into a regexp: everything is
// literal except the wildcard.
func toPattern(expected string) (*regexp.Regexp, error) {
	const escapeSentinel = "\x00docci-literal-wildcard\x00"

	expected = strings.ReplaceAll(expected, wildcardEscape, escapeSentinel)
	expected = collapseWhitespace(expected)

	var sb strings.Builder
	for i, segment := range strings.Split(expected, Wildcard) {
		if i > 0 {
			sb.WriteString(`.*?`)
		}
		sb.WriteString(regexp.QuoteMeta(strings.ReplaceAll(segment, escapeSentinel, Wildcard)))
	}

	return regexp.Compile(sb.String())
}

// ansiRe matches the escape sequences a tool emits when it colours its output.
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

var whitespaceRe = regexp.MustCompile(`\s+`)

// collapseWhitespace reduces every run of whitespace to a single space and
// strips colour escapes, so that indentation, line breaks and a tool's choice
// to colour its output do not decide whether a checkpoint passes. Nobody should
// have to paste escape sequences into a page to make a check match.
func collapseWhitespace(s string) string {
	s = ansiRe.ReplaceAllString(s, "")
	return strings.TrimSpace(whitespaceRe.ReplaceAllString(s, " "))
}

// DescribeMismatch says which line of the expected output first failed to
// appear, so the reader of a CI log does not have to diff two blobs by eye.
func DescribeMismatch(actual, expected string) string {
	for _, line := range strings.Split(expected, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !Matches(actual, line) {
			return fmt.Sprintf("first line that did not appear: %s", strings.TrimSpace(line))
		}
	}
	return "the lines appear, but not in this order"
}

// ToERE compiles an expected checkpoint into a POSIX extended regular
// expression, for the generated script to test with grep while a block is
// still being retried. The semantics match Matches: everything is literal
// except the wildcard, and whitespace is collapsed first.
//
// ERE has no lazy quantifier, so the wildcard becomes `.*`. For a containment
// test that makes no difference.
func ToERE(expected string) string {
	const escapeSentinel = "\x00docci-literal-wildcard\x00"

	expected = strings.ReplaceAll(expected, wildcardEscape, escapeSentinel)
	expected = collapseWhitespace(expected)

	var sb strings.Builder
	for i, segment := range strings.Split(expected, Wildcard) {
		if i > 0 {
			sb.WriteString(`.*`)
		}
		sb.WriteString(regexp.QuoteMeta(strings.ReplaceAll(segment, escapeSentinel, Wildcard)))
	}

	return sb.String()
}

// ShellSingleQuote wraps a string for safe use inside single quotes in bash.
func ShellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
