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
func PairAssertions(before, after []Assertion) ([]Pair, error) {
	if len(before) == 0 {
		return nil, fmt.Errorf("cannot compare: the base version of this page asserts nothing")
	}

	index := func(assertions []Assertion, side string) (map[string]Assertion, error) {
		seen := make(map[string]Assertion, len(assertions))
		for _, assertion := range assertions {
			id := assertion.Key + "\x00" + assertion.Kind
			if _, duplicate := seen[id]; duplicate {
				return nil, fmt.Errorf("cannot compare: %q appears more than once in the %s version; give each block a distinct name", assertion.Key, side)
			}
			seen[id] = assertion
		}
		return seen, nil
	}

	beforeIndex, err := index(before, "base")
	if err != nil {
		return nil, err
	}
	afterIndex, err := index(after, "head")
	if err != nil {
		return nil, err
	}

	var pairs []Pair
	for _, assertion := range before {
		id := assertion.Key + "\x00" + assertion.Kind
		pair := Pair{Key: assertion.Key, Kind: assertion.Kind, Before: &assertion}
		if match, ok := afterIndex[id]; ok {
			pair.After = &match
		}
		pairs = append(pairs, pair)
	}

	for _, assertion := range after {
		id := assertion.Key + "\x00" + assertion.Kind
		if _, existed := beforeIndex[id]; !existed {
			pairs = append(pairs, Pair{Key: assertion.Key, Kind: assertion.Kind, After: &assertion})
		}
	}

	return pairs, nil
}
