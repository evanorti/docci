// Package review compares the assertions on two versions of a page, so that an
// automated fix can be checked for whether it reduced what the page can catch.
package review

import (
	"fmt"

	"github.com/reecepbcups/docci/parser"
)

// Assertion is one claim a page makes about output.
type Assertion struct {
	// Key identifies the same assertion across two versions of a page. It is
	// the block's name where the author gave one, falling back to its step
	// title or heading, and finally to its ordinal among the page's
	// assertions. It cannot be the block's position in the file, because an
	// edit anywhere above a block shifts its position and would make an
	// unchanged assertion look removed.
	Key string

	// Named is true when Key came from the author: the block's name, its step
	// title or its heading. It is false when Key fell back to an ordinal. Only
	// named assertions can be tracked across versions; see PairAssertions.
	Named bool

	// Kind is "contains" for a hidden tripwire or "expect" for a rendered
	// checkpoint.
	Kind string

	// Text is the asserted content.
	Text string

	// Line is where the assertion sits in this version of the page. It is for
	// error messages only and is never used to match assertions across versions.
	Line int
}

// Extract returns every assertion a page makes, in page order.
func Extract(document, path string) ([]Assertion, error) {
	_, blocks, err := parser.ParseDocument(document, path)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	var assertions []Assertion
	for _, block := range blocks {
		key, named := assertionKey(block, len(assertions)+1)

		if block.OutputContains != "" {
			assertions = append(assertions, Assertion{
				Key: key, Named: named, Kind: "contains", Text: block.OutputContains, Line: block.LineNumber,
			})
		}
		if block.ExpectOutput != "" {
			assertions = append(assertions, Assertion{
				Key: key, Named: named, Kind: "expect", Text: block.ExpectOutput, Line: block.LineNumber,
			})
		}
	}

	return assertions, nil
}

// assertionKey names an assertion the way a human would refer to it, and
// reports whether the author supplied that name. The ordinal is the number of
// assertions up to and including this one, which keeps an unnamed assertion's
// key stable when unrelated content moves above it.
func assertionKey(block parser.CodeBlock, ordinal int) (string, bool) {
	switch {
	case block.Name != "":
		return block.Name, true
	case block.StepTitle != "":
		return block.StepTitle, true
	case block.Heading != "":
		return block.Heading, true
	default:
		return fmt.Sprintf("assertion #%d", ordinal), false
	}
}
