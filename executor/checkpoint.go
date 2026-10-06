package executor

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

// matchExpected reports whether actual output satisfies an expected checkpoint.
//
// Matching is "contains", not equality: real output is surrounded by log lines,
// prompts and progress that no page should have to reproduce. Whitespace is
// collapsed on both sides, so a rendered block that was re-indented to fit the
// page still matches the terminal it was copied from.
func matchExpected(actual, expected string) bool {
	pattern, err := expectedToPattern(expected)
	if err != nil {
		return false
	}
	return pattern.MatchString(collapseWhitespace(actual))
}

// expectedToPattern compiles an expected block into a regexp: everything is
// literal except the wildcard.
func expectedToPattern(expected string) (*regexp.Regexp, error) {
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

// collapseWhitespace reduces every run of whitespace to a single space, so that
// indentation and line breaks do not decide whether a checkpoint passes.
func collapseWhitespace(s string) string {
	return strings.TrimSpace(regexp.MustCompile(`\s+`).ReplaceAllString(s, " "))
}

// describeMismatch says which line of the expected output first failed to
// appear, so the reader of a CI log does not have to diff two blobs by eye.
func describeMismatch(actual, expected string) string {
	for _, line := range strings.Split(expected, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !matchExpected(actual, line) {
			return fmt.Sprintf("first line that did not appear: %s", strings.TrimSpace(line))
		}
	}
	return "the lines appear, but not in this order"
}
