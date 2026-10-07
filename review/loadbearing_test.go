package review

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadBearingWhenProseNamesTheValue(t *testing.T) {
	prose := "Confirm the tokens arrived: you should see 10 tokens on the destination chain."
	require.True(t, LoadBearing("10", prose))
}

func TestNotLoadBearingWhenProseIsSilent(t *testing.T) {
	prose := "The relayer prints a readiness line once both chains are connected."
	require.False(t, LoadBearing("41001", prose))
}

func TestBooleansDistinguishingSuccessAreAlwaysLoadBearing(t *testing.T) {
	require.True(t, LoadBearing("false", "no mention"), "a success/failure flag is load-bearing on its own")
	require.True(t, LoadBearing("SUCCEEDED", "no mention"))
}

func TestKnownNoiseIsNeverLoadBearing(t *testing.T) {
	prose := "It prints at 12:34:56 and takes 1m4s."
	require.False(t, LoadBearing("12:34:56", prose), "a timestamp is noise even when the prose mentions it")
	require.False(t, LoadBearing("1m4s", prose))
	require.False(t, LoadBearing("/home/you/.ibc/ibc.yml", prose))
}

func TestLiteralsSkipsWildcards(t *testing.T) {
	literals := Literals("{\"height\": \"<...>\", \"ready\": true}")
	require.Contains(t, literals, "height")
	require.Contains(t, literals, "true")
	require.NotContains(t, literals, "<...>")
}

func TestLiteralsKeepValuesWhole(t *testing.T) {
	literals := Literals(`{"height": "12:34:56", "config": "/home/you/.ibc/ibc.yml"}`)
	require.ElementsMatch(t, []string{"height", "12:34:56", "config", "/home/you/.ibc/ibc.yml"}, literals)
}

func TestWholeTimestampAndPathAreNotLoadBearing(t *testing.T) {
	prose := "At 12:34:56 the relayer reads /home/you/.ibc/ibc.yml."
	for _, literal := range Literals(`{"time": "12:34:56", "config": "/home/you/.ibc/ibc.yml"}`) {
		if literal == "time" || literal == "config" {
			continue
		}
		require.False(t, LoadBearing(literal, prose), literal)
	}
}

func TestDigitInsideLargerNumberIsNotNamed(t *testing.T) {
	require.False(t, LoadBearing("1", "You should see 100 tokens."))
	require.False(t, LoadBearing("1", "The year is 2026."))
	// "ok" is a terminal state now, so the substring check uses a neutral word.
	require.False(t, LoadBearing("boo", "Read the book."))
}

func TestProseMatchIsCaseInsensitive(t *testing.T) {
	require.True(t, LoadBearing("height", "Height is reported once synced."))
}

func TestProseUnitSuffixStillNamesTheNumber(t *testing.T) {
	require.True(t, LoadBearing("10", "You should see 10% free."))
	require.True(t, LoadBearing("10", "The pool scales 10x."))
}

func TestLiteralWithWildcardIsLoadBearingWhenAnyFragmentIsNamed(t *testing.T) {
	require.True(t, LoadBearing("a<...>b", "The output starts with a."))
	require.False(t, LoadBearing("a<...>b", "Nothing relevant here."))
}

func TestNoiseStillWinsAfterAggressiveProseSplitting(t *testing.T) {
	prose := "It prints 12:34:56 then 12 34 56 and reads /home/you/.ibc/ibc.yml in 1m4s."
	require.False(t, LoadBearing("12:34:56", prose))
	require.False(t, LoadBearing("/home/you/.ibc/ibc.yml", prose))
	require.False(t, LoadBearing("1m4s", prose))
}
