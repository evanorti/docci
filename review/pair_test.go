package review

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPairMatchesByKeyAndKind(t *testing.T) {
	before := []Assertion{NamedAssertion("mint", "contains", "executed")}
	after := []Assertion{NamedAssertion("mint", "contains", "txHash")}

	pairs, err := PairAssertions(before, after)
	require.NoError(t, err)
	require.Len(t, pairs, 1)
	require.Equal(t, "executed", pairs[0].Before.Text)
	require.Equal(t, "txHash", pairs[0].After.Text)
}

// A deleted assertion is the largest possible reduction, not an absent pair.
func TestPairKeepsDeletedAssertions(t *testing.T) {
	before := []Assertion{NamedAssertion("balance", "expect", "{\"balance\": \"10\"}")}

	pairs, err := PairAssertions(before, nil)
	require.NoError(t, err)
	require.Len(t, pairs, 1)
	require.NotNil(t, pairs[0].Before)
	require.Nil(t, pairs[0].After, "a removed assertion must survive pairing so it can be judged")
}

// Ambiguous keys must refuse rather than pair arbitrarily.
func TestPairRefusesDuplicateKeys(t *testing.T) {
	before := []Assertion{
		NamedAssertion("check", "contains", "a"),
		NamedAssertion("check", "contains", "b"),
	}

	_, err := PairAssertions(before, before)
	require.Error(t, err)
	require.Contains(t, err.Error(), "appears more than once")
}

// A page with no assertions on one side cannot be compared.
func TestPairRefusesWhenBeforeIsEmpty(t *testing.T) {
	after := []Assertion{NamedAssertion("check", "contains", "a")}

	_, err := PairAssertions(nil, after)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot compare")
}

// One block that asserts both ways yields two assertions with the same Key.
// Pairing on Key alone would silently drop one of them, so this test builds
// both versions from real pages and checks that each kind pairs on its own.
func TestPairKeepsBothKindsOfOneBlock(t *testing.T) {
	block := []string{
		"## Mint a token",
		"",
		"<!-- docci name=\"mint\" output-contains=\"placeholder\" -->",
		"",
		"```bash",
		"mint-token",
		"```",
		"",
		"<!-- docci expect-output -->",
		"",
		"```json",
		"{\"minted\": true}",
		"```",
	}
	pageWith := func(contains string) string {
		lines := append([]string(nil), block...)
		lines[2] = "<!-- docci name=\"mint\" output-contains=\"" + contains + "\" -->"
		return strings.Join(lines, "\n")
	}

	before, err := Extract(pageWith("executed"), "page.md")
	require.NoError(t, err)
	after, err := Extract(pageWith("txHash"), "page.md")
	require.NoError(t, err)
	require.Len(t, before, 2, "the block should yield one contains and one expect assertion")
	require.Len(t, after, 2)

	pairs, err := PairAssertions(before, after)
	require.NoError(t, err)
	require.Len(t, pairs, 2, "dropping one kind of a dual-field block would hide a reduction")

	byKind := map[string]Pair{}
	for _, pair := range pairs {
		require.Equal(t, "mint", pair.Key)
		require.NotNil(t, pair.Before)
		require.NotNil(t, pair.After)
		byKind[pair.Kind] = pair
	}
	require.Len(t, byKind, 2, "the two pairs must have distinct kinds")

	require.Equal(t, "executed", byKind["contains"].Before.Text)
	require.Equal(t, "txHash", byKind["contains"].After.Text)
	require.Contains(t, byKind["expect"].Before.Text, `"minted": true`)
	require.Contains(t, byKind["expect"].After.Text, `"minted": true`)
}

// unnamedExpectBlock is a bash block with an unnamed rendered checkpoint, the
// shape a page uses for an assertion the author did not name.
func unnamedExpectBlock(command, rendered string) []string {
	return []string{
		"```bash",
		command,
		"```",
		"",
		"<!-- docci expect-output -->",
		"",
		"```json",
		rendered,
		"```",
	}
}

// With the same number of unnamed assertions on both sides, an edit above a
// block must not change what it pairs with. Named assertions on the same page
// pair as usual.
func TestPairSurvivesEditAboveWhenUnnamedCountsMatch(t *testing.T) {
	named := []string{
		"<!-- docci name=\"mint\" output-contains=\"executed\" -->",
		"",
		"```bash",
		"mint-token",
		"```",
	}
	unnamed := unnamedExpectBlock("cat counter.txt", "{\"height\": \"3\"}")

	base := strings.Join(append(append([]string{"Intro.", ""}, named...), append([]string{""}, unnamed...)...), "\n")
	edited := strings.Join(append(append([]string{"Intro.", "", "A new paragraph above the blocks.", ""}, named...), append([]string{""}, unnamed...)...), "\n")

	before, err := Extract(base, "page.md")
	require.NoError(t, err)
	after, err := Extract(edited, "page.md")
	require.NoError(t, err)

	pairs, err := PairAssertions(before, after)
	require.NoError(t, err)
	require.Len(t, pairs, 2)
	for _, pair := range pairs {
		require.NotNil(t, pair.Before, "%s/%s", pair.Key, pair.Kind)
		require.NotNil(t, pair.After, "%s/%s", pair.Key, pair.Kind)
		require.Equal(t, pair.Before.Text, pair.After.Text)
	}
}

// Inserting an unnamed assertion above another shifts the other's ordinal, so
// the old #1 would pair with a different #1 and nothing would report it. The
// comparison must refuse instead.
func TestPairRefusesInsertedUnnamedAssertionAbove(t *testing.T) {
	original := unnamedExpectBlock("cat counter.txt", "{\"height\": \"3\"}")
	inserted := unnamedExpectBlock("cat other.txt", "{\"height\": \"9\"}")

	base := strings.Join(append([]string{"Intro.", ""}, original...), "\n")
	edited := strings.Join(append(append([]string{"Intro.", ""}, inserted...), append([]string{""}, original...)...), "\n")

	before, err := Extract(base, "page.md")
	require.NoError(t, err)
	after, err := Extract(edited, "page.md")
	require.NoError(t, err)
	require.Len(t, before, 1)
	require.Len(t, after, 2)

	_, err = PairAssertions(before, after)
	require.Error(t, err)
	require.Contains(t, err.Error(), "the unnamed assertions changed (1 in the base version, 2 in the head version)")
	require.Contains(t, err.Error(), "give each block a name")
}

// namedBlock is a bash block whose contains assertion is named, so it can be
// tracked across versions.
func namedBlock(name, contains string) []string {
	return []string{
		"<!-- docci name=\"" + name + "\" output-contains=\"" + contains + "\" -->",
		"",
		"```bash",
		"mint-token",
		"```",
	}
}

// The guard judges edits to named assertions. When every unnamed assertion is
// unchanged, a named one that was edited must pair normally, with no error.
func TestPairNamedEditWithUnnamedUnchangedPairs(t *testing.T) {
	unnamed := unnamedExpectBlock("cat counter.txt", "{\"height\": \"3\"}")

	base := strings.Join(append(append(namedBlock("mint", "executed"), ""), append(unnamed, "")...)[:], "\n")
	head := strings.Join(append(append(namedBlock("mint", "txHash"), ""), append(unnamed, "")...)[:], "\n")

	before, err := Extract(base, "page.md")
	require.NoError(t, err)
	after, err := Extract(head, "page.md")
	require.NoError(t, err)

	pairs, err := PairAssertions(before, after)
	require.NoError(t, err)
	require.Len(t, pairs, 2)

	byKey := map[string]Pair{}
	for _, pair := range pairs {
		byKey[pair.Key+"/"+pair.Kind] = pair
	}
	mint := byKey["mint/contains"]
	require.NotNil(t, mint.Before)
	require.NotNil(t, mint.After)
	require.Equal(t, "executed", mint.Before.Text)
	require.Equal(t, "txHash", mint.After.Text)

	unnamedPair := byKey["assertion #1/expect"]
	require.NotNil(t, unnamedPair.Before, "the unchanged unnamed assertion should still pair")
	require.NotNil(t, unnamedPair.After)
}

// An unnamed assertion edited in place cannot be told apart from a different
// assertion, so the comparison must refuse.
func TestPairRefusesUnnamedEditedInPlace(t *testing.T) {
	base := strings.Join(unnamedExpectBlock("cat counter.txt", "{\"height\": \"3\"}"), "\n")
	head := strings.Join(unnamedExpectBlock("cat counter.txt", "{\"height\": \"4\"}"), "\n")

	before, err := Extract(base, "page.md")
	require.NoError(t, err)
	after, err := Extract(head, "page.md")
	require.NoError(t, err)

	_, err = PairAssertions(before, after)
	require.Error(t, err)
	require.Contains(t, err.Error(), "give each block a name")
}

// Replace one unnamed assertion and append another. The count stays at two, so
// a count check alone would pass, and the ordinal keys would pair the old
// assertion with the wrong one: base [A, B] against head [B, C] would pair #1
// A with B and #2 B with C. Only comparing the unnamed assertions in order
// refuses this case.
func TestPairRefusesReplaceOneAppendOne(t *testing.T) {
	a := unnamedExpectBlock("cat a.txt", "{\"a\": \"1\"}")
	b := unnamedExpectBlock("cat b.txt", "{\"b\": \"2\"}")
	c := unnamedExpectBlock("cat c.txt", "{\"c\": \"3\"}")

	base := strings.Join(append(append(append([]string{"Intro.", ""}, a...), ""), append(b, "")...), "\n")
	head := strings.Join(append(append(append([]string{"Intro.", ""}, b...), ""), append(c, "")...), "\n")

	before, err := Extract(base, "page.md")
	require.NoError(t, err)
	after, err := Extract(head, "page.md")
	require.NoError(t, err)
	require.Len(t, before, 2)
	require.Len(t, after, 2)

	_, err = PairAssertions(before, after)
	require.Error(t, err, "same count, different assertions: the comparison must refuse")
	require.Contains(t, err.Error(), "give each block a name")
}

// A block a human named "assertion #3" is named, not an ordinal fallback. It
// must not be counted as unnamed, so editing it pairs normally. If it were
// treated as ordinal, its unchanged unnamed neighbour would be compared
// against an edited one and the comparison would refuse.
func TestNamedAssertionLookingLikeOrdinalIsNamed(t *testing.T) {
	unnamed := unnamedExpectBlock("cat counter.txt", "{\"height\": \"3\"}")
	base := strings.Join(append(append(namedBlock("assertion #3", "executed"), ""), append(unnamed, "")...), "\n")
	head := strings.Join(append(append(namedBlock("assertion #3", "txHash"), ""), append(unnamed, "")...), "\n")

	before, err := Extract(base, "page.md")
	require.NoError(t, err)
	require.True(t, before[0].Named(), "a block named by the author is named, whatever its name looks like")
	require.Equal(t, "assertion #3", before[0].Key)

	after, err := Extract(head, "page.md")
	require.NoError(t, err)

	pairs, err := PairAssertions(before, after)
	require.NoError(t, err)
	for _, pair := range pairs {
		if pair.Key == "assertion #3" {
			require.Equal(t, "executed", pair.Before.Text)
			require.Equal(t, "txHash", pair.After.Text)
			return
		}
	}
	t.Fatal("the named assertion did not pair")
}

// A named assertion inserted above unnamed ones must not renumber them. The
// unnamed sequence is unchanged, so the sequence check passes, and the ordinals
// are the only thing left to keep A and B on their own counterparts. The
// ordinal therefore counts unnamed assertions only.
func TestPairNamedInsertionAboveUnnamedKeepsThemPaired(t *testing.T) {
	a := unnamedExpectBlock("cat a.txt", "{\"a\": \"1\"}")
	b := unnamedExpectBlock("cat b.txt", "{\"b\": \"2\"}")
	inserted := namedBlock("mint", "executed")

	base := strings.Join(append(append([]string{"Intro.", ""}, a...), append([]string{""}, b...)...), "\n")
	head := strings.Join(append(append(append([]string{"Intro.", ""}, inserted...), ""), append(append(a, ""), b...)...), "\n")

	before, err := Extract(base, "page.md")
	require.NoError(t, err)
	after, err := Extract(head, "page.md")
	require.NoError(t, err)
	require.Len(t, before, 2)
	require.Len(t, after, 3)

	pairs, err := PairAssertions(before, after)
	require.NoError(t, err)

	byKey := map[string]Pair{}
	for _, pair := range pairs {
		byKey[pair.Key] = pair
	}
	for _, key := range []string{"assertion #1", "assertion #2"} {
		pair, ok := byKey[key]
		require.True(t, ok, key)
		require.NotNil(t, pair.Before, key)
		require.NotNil(t, pair.After, key)
		require.Equal(t, pair.Before.Text, pair.After.Text, "%s must pair with its own counterpart", key)
	}
	require.Nil(t, byKey["mint"].Before, "the inserted named assertion is new, not a reduction")
}

// A block a human named "assertion #1" and an unnamed assertion whose ordinal
// is also "assertion #1" share a Key. They are different assertions, so
// pairing must keep them apart. The identity includes Named so that this holds
// on its own, without depending on the sequence check.
func TestPairNamedAndOrdinalKeysDoNotCollide(t *testing.T) {
	// Both are "contains" assertions, so Kind cannot tell them apart and only
	// Named can.
	page := strings.Join([]string{
		"<!-- docci name=\"assertion #1\" output-contains=\"executed\" -->",
		"",
		"```bash",
		"mint-token",
		"```",
		"",
		"<!-- docci output-contains=\"ready\" -->",
		"",
		"```bash",
		"cat counter.txt",
		"```",
	}, "\n")

	before, err := Extract(page, "page.md")
	require.NoError(t, err)
	require.Len(t, before, 2)
	require.Equal(t, before[0].Key, before[1].Key, "the collision this test depends on")
	require.Equal(t, before[0].Kind, before[1].Kind, "the collision must not be separated by Kind")
	require.NotEqual(t, before[0].Named(), before[1].Named())

	after, err := Extract(page, "page.md")
	require.NoError(t, err)

	pairs, err := PairAssertions(before, after)
	require.NoError(t, err, "two distinct assertions must not be refused as duplicates")
	require.Len(t, pairs, 2)
	for _, pair := range pairs {
		require.NotNil(t, pair.Before)
		require.NotNil(t, pair.After)
		require.Equal(t, pair.Before.Named(), pair.After.Named())
	}
}

// A hand-built Assertion has a zero identity. It must be refused, not read as
// an unnamed assertion. Before identity was made constructor-only, a struct
// literal with no Named field was exactly that.
func TestPairRefusesHandBuiltAssertionWithNoIdentity(t *testing.T) {
	handBuilt := []Assertion{{Key: "mint", Kind: "contains", Text: "executed"}}
	extracted := []Assertion{NamedAssertion("mint", "contains", "executed")}

	_, err := PairAssertions(extracted, handBuilt)
	require.Error(t, err)
	require.Contains(t, err.Error(), "has no identity")

	_, err = PairAssertions(handBuilt, extracted)
	require.Error(t, err)
	require.Contains(t, err.Error(), "has no identity")
}

// Regression for the fifth route, found by the property test (seed 347). Under
// a heading "assertion #1", an unnamed block asserts both ways. The block is
// removed, and a block named "assertion #1" is inserted under the same heading.
// The name and the heading are different sources, so no pair may join the two
// blocks, even though both keys read "assertion #1". Before the identity
// carried its source, the guard paired them.
func TestPairNameAndHeadingWithSameTextDoNotJoin(t *testing.T) {
	base := strings.Join([]string{
		"## assertion #1",
		"",
		"<!-- docci output-contains=\"out-5-0\" -->",
		"",
		"```bash",
		"echo block-5",
		"```",
		"",
		"<!-- docci expect-output -->",
		"",
		"```json",
		"{\"id\": 5, \"v\": 0}",
		"```",
	}, "\n")
	head := strings.Join([]string{
		"## assertion #1",
		"",
		"<!-- docci name=\"assertion #1\" -->",
		"",
		"```bash",
		"echo block-6",
		"```",
		"",
		"<!-- docci expect-output -->",
		"",
		"```json",
		"{\"id\": 6, \"v\": 0}",
		"```",
	}, "\n")

	before, err := Extract(base, "page.md")
	require.NoError(t, err)
	after, err := Extract(head, "page.md")
	require.NoError(t, err)

	pairs, err := PairAssertions(before, after)
	require.NoError(t, err)
	for _, pair := range pairs {
		require.False(t, pair.Before != nil && pair.After != nil,
			"%s/%s joins the base block to a different head block", pair.Key, pair.Kind)
	}
	require.Len(t, pairs, 3, "the removed block gives two one-sided pairs (contains and expect), and the new named block one more")
}
