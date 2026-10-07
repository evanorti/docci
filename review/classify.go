package review

import (
	"fmt"
	"regexp"
	"strings"
)

// derivedPattern matches a value the page computes rather than states.
var derivedPattern = regexp.MustCompile(`\$\(|\$\{?[A-Za-z_][A-Za-z0-9_]*`)

// Verdict is what the guard concluded about one assertion.
type Verdict struct {
	Key    string
	Kind   string
	Class  string
	Reason string

	// Lost names the literals the assertion no longer pins.
	Lost []string
}

// Blocking reports whether this verdict must stop the run.
func (v Verdict) Blocking() bool {
	return v.Class == "reduction-load-bearing" || v.Class == "hardcoding"
}

// normalize collapses whitespace. The parser leaves trailing whitespace on
// fenced content, so a raw comparison would call a reformat an edit.
func normalize(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

// Classify decides what an edit did to one assertion. prose is the page's
// sentences only; see Prose for why it must not contain the assertion itself.
func Classify(pair Pair, prose string) Verdict {
	verdict := Verdict{Key: pair.Key, Kind: pair.Kind}

	switch {
	case pair.Before == nil:
		verdict.Class = "added"
		verdict.Reason = "this assertion is new"
		return verdict

	case pair.After == nil:
		verdict.Class = "reduction-load-bearing"
		verdict.Reason = "the assertion was removed entirely, so the page no longer checks this step"
		verdict.Lost = Literals(pair.Before.Text)
		return verdict

	case normalize(pair.Before.Text) == normalize(pair.After.Text):
		verdict.Class = "unchanged"
		return verdict
	}

	if derivedPattern.MatchString(pair.Before.Text) && !derivedPattern.MatchString(pair.After.Text) {
		verdict.Class = "hardcoding"
		verdict.Reason = "a derived value was replaced with a literal, which passes on one machine and breaks for every reader"
		return verdict
	}

	kept := make(map[string]bool)
	for _, literal := range Literals(pair.After.Text) {
		kept[literal] = true
	}

	// A renamed field is a change of shape, not of what is pinned: the value
	// beside it is still compared. Losing a key therefore never counts by
	// itself; a deleted field still loses its value, which does.
	keys := fieldNames(pair.Before.Text)

	var lost, lostLoadBearing []string
	for _, literal := range Literals(pair.Before.Text) {
		if kept[literal] || keys[literal] {
			continue
		}
		lost = append(lost, literal)
		if LoadBearing(literal, prose) {
			lostLoadBearing = append(lostLoadBearing, literal)
		}
	}

	switch {
	case len(lostLoadBearing) > 0:
		verdict.Class = "reduction-load-bearing"
		verdict.Reason = fmt.Sprintf("the assertion no longer pins %s, which the page depends on", strings.Join(lostLoadBearing, ", "))
		verdict.Lost = lostLoadBearing
	case len(lost) > 0:
		verdict.Class = "reduction"
		verdict.Reason = fmt.Sprintf("the assertion dropped %s, none of which the page depends on", strings.Join(lost, ", "))
		verdict.Lost = lost
	default:
		verdict.Class = "re-expression"
		verdict.Reason = "the assertion pins the same facts in a new shape"
		// Said aloud because the diff cannot tell an upstream rename from a
		// check that quietly moved to a different field than the prose names.
		if renames := renamedKeys(pair.Before.Text, pair.After.Text); renames != "" {
			verdict.Reason = "the assertion pins the same values under renamed keys: " + renames
		}
	}

	return verdict
}

// fieldPattern matches a quoted JSON key.
var fieldPattern = regexp.MustCompile(`"([^"]+)"\s*:`)

func fieldNames(text string) map[string]bool {
	names := make(map[string]bool)
	for _, match := range fieldPattern.FindAllStringSubmatch(text, -1) {
		names[match[1]] = true
	}
	return names
}

// renamedKeys lists keys present only before and only after, in order, paired
// by position: "balance -> fee". It is empty when no key changed.
func renamedKeys(before, after string) string {
	oldKeys, newKeys := fieldNames(before), fieldNames(after)
	var gone, added []string
	for _, m := range fieldPattern.FindAllStringSubmatch(before, -1) {
		if !newKeys[m[1]] {
			gone = append(gone, m[1])
		}
	}
	for _, m := range fieldPattern.FindAllStringSubmatch(after, -1) {
		if !oldKeys[m[1]] {
			added = append(added, m[1])
		}
	}
	var parts []string
	for i := 0; i < len(gone) || i < len(added); i++ {
		from, to := "(none)", "(none)"
		if i < len(gone) {
			from = gone[i]
		}
		if i < len(added) {
			to = added[i]
		}
		parts = append(parts, from+" -> "+to)
	}
	return strings.Join(parts, ", ")
}

var (
	fenceLine = regexp.MustCompile("^\\s*(```|~~~)")

	// Directives may span lines, so (?s). Non-greedy so two comments on a page
	// do not swallow the prose between them.
	mdxDirective  = regexp.MustCompile(`(?s)\{/\*\s*docci\b.*?\*/\}`)
	htmlDirective = regexp.MustCompile(`(?s)<!--\s*docci\b.*?-->`)
)

// Prose returns a page's sentences: the document without fenced code and
// without docci directives. LoadBearing asks whether the prose names a value,
// and the assertion under judgement lives in a fence, so leaving fences in
// would make every literal appear named and every reduction look load-bearing.
func Prose(document string) string {
	var kept []string
	inFence := false
	for _, line := range strings.Split(document, "\n") {
		if fenceLine.MatchString(line) {
			inFence = !inFence
			continue
		}
		if !inFence {
			kept = append(kept, line)
		}
	}

	text := strings.Join(kept, "\n")
	text = mdxDirective.ReplaceAllString(text, "")
	return htmlDirective.ReplaceAllString(text, "")
}

// Run compares two versions of a page and returns a verdict per assertion.
// Prose comes from the base version: it is what the reader was promised before
// the edit, and the head's prose may have been rewritten to fit a weaker check.
func Run(beforeDoc, afterDoc, path string) ([]Verdict, error) {
	before, err := Extract(beforeDoc, path)
	if err != nil {
		return nil, err
	}
	after, err := Extract(afterDoc, path)
	if err != nil {
		return nil, err
	}

	pairs, err := PairAssertions(before, after)
	if err != nil {
		return nil, err
	}

	prose := Prose(beforeDoc)
	var verdicts []Verdict
	for _, pair := range pairs {
		verdicts = append(verdicts, Classify(pair, prose))
	}
	return verdicts, nil
}
