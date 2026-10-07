package review

import "fmt"

// Pair is one assertion seen in both versions of a page, or in only one of
// them. A nil After means the assertion was deleted.
type Pair struct {
	Key    string
	Kind   string
	Before *Assertion
	After  *Assertion
}

// PairAssertions matches assertions across two versions of a page by the name
// a human would use for them.
//
// It refuses rather than guesses. A page that cannot be compared must say so:
// reporting "no reductions found" for a page nobody could compare reads as
// approval, which is the one answer that must never be wrong.
//
// Only named assertions are judged. An unnamed assertion's identity is its
// ordinal, and an ordinal cannot tell an edited assertion from a different one
// that took its place. So if the unnamed assertions differ at all between the
// versions, the page is refused and the author is asked to name them.
func PairAssertions(before, after []Assertion) ([]Pair, error) {
	if len(before) == 0 {
		return nil, fmt.Errorf("cannot compare: the base version of this page asserts nothing")
	}

	if err := checkIdentified(before, "base"); err != nil {
		return nil, err
	}
	if err := checkIdentified(after, "head"); err != nil {
		return nil, err
	}

	if err := checkUnnamedUnchanged(before, after); err != nil {
		return nil, err
	}

	beforeIndex, err := indexAssertions(before, "base")
	if err != nil {
		return nil, err
	}
	afterIndex, err := indexAssertions(after, "head")
	if err != nil {
		return nil, err
	}

	var pairs []Pair
	for _, assertion := range before {
		pair := Pair{Key: assertion.Key, Kind: assertion.Kind, Before: &assertion}
		if match, ok := afterIndex[pairID(assertion)]; ok {
			pair.After = &match
		}
		pairs = append(pairs, pair)
	}

	for _, assertion := range after {
		if _, existed := beforeIndex[pairID(assertion)]; !existed {
			pairs = append(pairs, Pair{Key: assertion.Key, Kind: assertion.Kind, After: &assertion})
		}
	}

	return pairs, nil
}

// checkIdentified refuses an assertion that did not come from Extract or
// NamedAssertion. Such a value has a zero identity, which would otherwise be
// read as something to pair by accident.
func checkIdentified(assertions []Assertion, side string) error {
	for _, assertion := range assertions {
		if !assertion.identity.valid() {
			return fmt.Errorf("cannot compare: assertion %q in the %s version has no identity; build it with Extract or NamedAssertion", assertion.Key, side)
		}
	}
	return nil
}

// pairID identifies an assertion across versions. Kind is part of the
// identity because one block can assert both ways and both assertions share a
// name. The identity's token is part of it because an unnamed assertion's
// identity is an ordinal, and an ordinal can equal a name a human chose: a
// block named "assertion #1" must never be paired with an unnamed assertion
// keyed "assertion #1". The identity has to be sound on its own. The sequence
// check in checkUnnamedUnchanged is a second line of defence, not the first.
// The NUL separator cannot appear in a block name, so the parts cannot run
// together.
func pairID(assertion Assertion) string {
	return assertion.identity.token() + "\x00" + assertion.Kind
}

// indexAssertions maps each assertion's pairID to the assertion. A repeated
// pairID means two blocks share a name, and pairing either one would be a
// guess, so it is refused.
func indexAssertions(assertions []Assertion, side string) (map[string]Assertion, error) {
	seen := make(map[string]Assertion, len(assertions))
	for _, assertion := range assertions {
		id := pairID(assertion)
		if _, duplicate := seen[id]; duplicate {
			return nil, fmt.Errorf("cannot compare: %q appears more than once in the %s version; give each block a distinct name", assertion.Key, side)
		}
		seen[id] = assertion
	}
	return seen, nil
}

// checkUnnamedUnchanged refuses unless the unnamed assertions match in order,
// by kind and text, across the two versions. Comparing counts is not enough.
// Replacing one unnamed assertion and appending another leaves the count
// unchanged, and the ordinal identities then pair the old assertion with the
// wrong one without any error.
func checkUnnamedUnchanged(before, after []Assertion) error {
	beforeUnnamed := unnamedInOrder(before)
	afterUnnamed := unnamedInOrder(after)

	if len(beforeUnnamed) != len(afterUnnamed) {
		return unnamedChangedError(len(beforeUnnamed), len(afterUnnamed))
	}
	for i := range beforeUnnamed {
		if beforeUnnamed[i].Kind != afterUnnamed[i].Kind || beforeUnnamed[i].Text != afterUnnamed[i].Text {
			return unnamedChangedError(len(beforeUnnamed), len(afterUnnamed))
		}
	}
	return nil
}

func unnamedInOrder(assertions []Assertion) []Assertion {
	var unnamed []Assertion
	for _, assertion := range assertions {
		if !assertion.Named() {
			unnamed = append(unnamed, assertion)
		}
	}
	return unnamed
}

func unnamedChangedError(before, after int) error {
	return fmt.Errorf("cannot compare: the unnamed assertions changed (%d in the base version, %d in the head version); give each block a name so it can be tracked across versions", before, after)
}
