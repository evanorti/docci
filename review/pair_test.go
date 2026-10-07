package review

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPairMatchesByKeyAndKind(t *testing.T) {
	before := []Assertion{{Key: "mint", Kind: "contains", Text: "executed"}}
	after := []Assertion{{Key: "mint", Kind: "contains", Text: "txHash"}}

	pairs, err := PairAssertions(before, after)
	require.NoError(t, err)
	require.Len(t, pairs, 1)
	require.Equal(t, "executed", pairs[0].Before.Text)
	require.Equal(t, "txHash", pairs[0].After.Text)
}

// A deleted assertion is the largest possible reduction, not an absent pair.
func TestPairKeepsDeletedAssertions(t *testing.T) {
	before := []Assertion{{Key: "balance", Kind: "expect", Text: "{\"balance\": \"10\"}"}}

	pairs, err := PairAssertions(before, nil)
	require.NoError(t, err)
	require.Len(t, pairs, 1)
	require.NotNil(t, pairs[0].Before)
	require.Nil(t, pairs[0].After, "a removed assertion must survive pairing so it can be judged")
}

// Ambiguous keys must refuse rather than pair arbitrarily.
func TestPairRefusesDuplicateKeys(t *testing.T) {
	before := []Assertion{
		{Key: "check", Kind: "contains", Text: "a"},
		{Key: "check", Kind: "contains", Text: "b"},
	}

	_, err := PairAssertions(before, before)
	require.Error(t, err)
	require.Contains(t, err.Error(), "appears more than once")
}

// A page with no assertions on one side cannot be compared.
func TestPairRefusesWhenBeforeIsEmpty(t *testing.T) {
	after := []Assertion{{Key: "check", Kind: "contains", Text: "a"}}

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
