// Package review compares the assertions on two versions of a page, so that an
// automated fix can be checked for whether it reduced what the page can catch.
package review

import (
	"fmt"
	"strconv"

	"github.com/reecepbcups/docci/parser"
)

// Assertion is one claim a page makes about output. Build one with Extract,
// or with NamedAssertion in tests. A hand-built Assertion has no identity, and
// PairAssertions refuses it rather than guess.
type Assertion struct {
	// Key is the name a human would use for this assertion, for display and
	// error messages. Pairing uses identity, not Key, because the same string
	// can come from different sources; see identity.
	Key string

	// Kind is "contains" for a hidden tripwire or "expect" for a rendered
	// checkpoint.
	Kind string

	// Text is the asserted content.
	Text string

	// Line is where the assertion sits in this version of the page. It is for
	// error messages only and is never used to match assertions across versions.
	Line int

	identity identity
}

// Named reports whether the assertion's identity came from the author, rather
// than from its ordinal. Only named assertions are tracked by name across
// versions; see PairAssertions.
func (a Assertion) Named() bool {
	return a.identity.named()
}

// identitySource says where an assertion's key came from. The four sources
// share one string space, so a key is only meaningful together with its source.
// A block named "assertion #1" and a block under a heading "assertion #1" are
// different assertions, and must never share an identity.
type identitySource string

const (
	sourceName    identitySource = "name"    // the block's name= directive
	sourceStep    identitySource = "step"    // the step title above the block
	sourceHeading identitySource = "heading" // the heading above the block
	sourceOrdinal identitySource = "ordinal" // the count of unnamed assertions
)

// identity is what pairing compares. It is unexported and its zero value is
// invalid, so an Assertion built without Extract or NamedAssertion cannot be
// mistaken for an unnamed one.
type identity struct {
	source  identitySource
	value   string // the key, for every source except ordinal
	ordinal int    // the position among unnamed assertions, for the ordinal source
}

func (i identity) valid() bool {
	switch i.source {
	case sourceName, sourceStep, sourceHeading:
		return i.value != ""
	case sourceOrdinal:
		return i.ordinal > 0
	default:
		return false
	}
}

func (i identity) named() bool {
	return i.source != sourceOrdinal && i.valid()
}

// token is the identity's stable form. It includes the source, so equal
// strings from different sources never compare equal. Pairing relies on this
// on its own, not only on the sequence check.
func (i identity) token() string {
	return string(i.source) + "\x00" + i.value + "\x00" + strconv.Itoa(i.ordinal)
}

// NamedAssertion builds an assertion whose identity is the author's name. It
// exists for callers and tests that have a name and no page to extract from.
func NamedAssertion(name, kind, text string) Assertion {
	return Assertion{Key: name, Kind: kind, Text: text, identity: identity{source: sourceName, value: name}}
}

// Extract returns every assertion a page makes, in page order.
func Extract(document, path string) ([]Assertion, error) {
	_, blocks, err := parser.ParseDocument(document, path)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	var assertions []Assertion
	unnamed := 0
	for _, block := range blocks {
		if block.OutputContains == "" && block.ExpectOutput == "" {
			continue
		}

		key, id := assertionIdentity(block, unnamed+1)
		if id.source == sourceOrdinal {
			unnamed++
		}

		if block.OutputContains != "" {
			assertions = append(assertions, Assertion{
				Key: key, Kind: "contains", Text: block.OutputContains, Line: block.LineNumber, identity: id,
			})
		}
		if block.ExpectOutput != "" {
			assertions = append(assertions, Assertion{
				Key: key, Kind: "expect", Text: block.ExpectOutput, Line: block.LineNumber, identity: id,
			})
		}
	}

	return assertions, nil
}

// assertionIdentity names an assertion the way a human would refer to it, and
// returns the identity pairing uses.
//
// Invariant: an unnamed assertion's identity is its position among the page's
// unnamed assertions only. Named assertions do not count toward it, so adding
// or removing a named block never renumbers an unnamed one, and unrelated
// content moving above it does not change it either. PairAssertions relies on
// this: it refuses unless the unnamed assertions match in order, so equal
// ordinals can only mean the same assertion. If this counted named assertions,
// inserting one named block would renumber every unnamed one below it and
// pair the wrong assertions with no error. Keep it this way.
//
// The ordinal argument is one more than the number of unnamed assertions
// before this block. The caller advances its count only for unnamed blocks.
func assertionIdentity(block parser.CodeBlock, ordinal int) (string, identity) {
	switch {
	case block.Name != "":
		return block.Name, identity{source: sourceName, value: block.Name}
	case block.StepTitle != "":
		return block.StepTitle, identity{source: sourceStep, value: block.StepTitle}
	case block.Heading != "":
		return block.Heading, identity{source: sourceHeading, value: block.Heading}
	default:
		return fmt.Sprintf("assertion #%d", ordinal), identity{source: sourceOrdinal, ordinal: ordinal}
	}
}
