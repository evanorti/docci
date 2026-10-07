package review

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"sort"
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

	// Any derived expression that became a literal is hardcoding, even when
	// others survive: "$(a) $(b)" -> "$(a) 5" passes on one machine only.
	if len(derivedPattern.FindAllString(pair.Before.Text, -1)) > len(derivedPattern.FindAllString(pair.After.Text, -1)) {
		verdict.Class = "hardcoding"
		verdict.Reason = "a derived value was replaced with a literal, which passes on one machine and breaks for every reader"
		return verdict
	}

	lost, renames := lostLiterals(pair.Before.Text, pair.After.Text)

	var lostLoadBearing []string
	for _, literal := range lost {
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
		if len(renames) > 0 {
			verdict.Reason = "the assertion pins the same values under renamed keys: " + strings.Join(renames, ", ")
		}
	}

	return verdict
}

// fieldValuePattern is the fallback for key/value text that is not valid JSON.
var fieldValuePattern = regexp.MustCompile(`"([^"]+)"\s*:\s*("(?:[^"\\]|\\.)*"|[^\s,}\]]+)`)

// fields reads key/value content into path -> value, or reports false when the
// text has none. Comparing keys one by one, not as a bag of literals, is what
// stops a value surviving under one key from excusing its deletion under
// another.
func fields(text string) (map[string]string, bool) {
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	var parsed any
	if err := decoder.Decode(&parsed); err == nil {
		switch parsed.(type) {
		case map[string]any, []any:
			out := make(map[string]string)
			flatten("", parsed, out)
			return out, true
		}
	}

	matches := fieldValuePattern.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil, false
	}
	out := make(map[string]string)
	for _, m := range matches {
		key := m[1]
		for n := 2; ; n++ {
			if _, taken := out[key]; !taken {
				break
			}
			key = fmt.Sprintf("%s#%d", m[1], n)
		}
		out[key] = strings.Trim(m[2], `"`)
	}
	return out, true
}

func flatten(path string, value any, out map[string]string) {
	switch v := value.(type) {
	case map[string]any:
		for key, child := range v {
			flatten(path+"."+key, child, out)
		}
	case []any:
		for i, child := range v {
			flatten(fmt.Sprintf("%s[%d]", path, i), child, out)
		}
	case nil:
		out[path] = "null"
	case json.Number:
		out[path] = v.String()
	default:
		out[path] = fmt.Sprint(v)
	}
}

// lostLiterals returns what the edit stopped pinning, and any key renames that
// kept their value. A key present before and absent after is lost whatever the
// other keys hold, unless a new key took over its exact value.
func lostLiterals(before, after string) (lost []string, renames []string) {
	oldFields, oldOK := fields(before)
	newFields, newOK := fields(after)
	if !oldOK || !newOK {
		return lostTokens(before, after), nil
	}

	var removed, added, changed []string
	for key, value := range oldFields {
		if newValue, ok := newFields[key]; !ok {
			removed = append(removed, key)
		} else if normalize(newValue) != normalize(value) {
			changed = append(changed, key)
		}
	}
	for key := range newFields {
		if _, ok := oldFields[key]; !ok {
			added = append(added, key)
		}
	}
	// Maps iterate randomly; sorted keeps verdicts and reasons stable.
	sort.Strings(removed)
	sort.Strings(added)
	sort.Strings(changed)

	claimed := make(map[string]bool)
	for _, key := range removed {
		renamed := false
		for _, candidate := range added {
			if !claimed[candidate] && normalize(newFields[candidate]) == normalize(oldFields[key]) {
				claimed[candidate] = true
				renames = append(renames, lastSegment(key)+" -> "+lastSegment(candidate))
				renamed = true
				break
			}
		}
		if !renamed {
			lost = appendUnique(lost, lastSegment(key))
			lost = appendUnique(lost, Literals(oldFields[key])...)
		}
	}

	for _, key := range changed {
		kept := make(map[string]bool)
		for _, literal := range Literals(newFields[key]) {
			kept[literal] = true
		}
		for _, literal := range Literals(oldFields[key]) {
			if !kept[literal] {
				lost = appendUnique(lost, literal)
			}
		}
	}
	return lost, renames
}

// lostTokens compares line-oriented output as a multiset, so a repeated line
// dropped to one copy is a loss even though the word is still present.
func lostTokens(before, after string) []string {
	remaining := make(map[string]int)
	for _, token := range tokenize(after) {
		remaining[token]++
	}
	var lost []string
	for _, token := range tokenize(before) {
		if remaining[token] > 0 {
			remaining[token]--
			continue
		}
		lost = appendUnique(lost, token)
	}
	return lost
}

func lastSegment(path string) string {
	if i := strings.LastIndex(path, "."); i >= 0 {
		path = path[i+1:]
	}
	if i := strings.Index(path, "#"); i >= 0 {
		path = path[:i]
	}
	return path
}

func appendUnique(list []string, items ...string) []string {
	for _, item := range items {
		if item != "" && !slices.Contains(list, item) {
			list = append(list, item)
		}
	}
	return list
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
