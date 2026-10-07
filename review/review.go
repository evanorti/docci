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
	// title or heading.
	Key string

	// Kind is "contains" for a hidden tripwire or "expect" for a rendered
	// checkpoint.
	Kind string

	// Text is the asserted content.
	Text string

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
		key := assertionKey(block)

		if block.OutputContains != "" {
			assertions = append(assertions, Assertion{
				Key: key, Kind: "contains", Text: block.OutputContains, Line: block.LineNumber,
			})
		}
		if block.ExpectOutput != "" {
			assertions = append(assertions, Assertion{
				Key: key, Kind: "expect", Text: block.ExpectOutput, Line: block.LineNumber,
			})
		}
	}

	return assertions, nil
}

// assertionKey names an assertion the way a human would refer to it.
func assertionKey(block parser.CodeBlock) string {
	switch {
	case block.Name != "":
		return block.Name
	case block.StepTitle != "":
		return block.StepTitle
	case block.Heading != "":
		return block.Heading
	default:
		return fmt.Sprintf("line %d", block.LineNumber)
	}
}
