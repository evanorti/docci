package executor

import "testing"

func TestCheckpointMatching(t *testing.T) {
	cases := []struct {
		name     string
		actual   string
		expected string
		want     bool
	}{
		{
			name:     "wildcard covers a value that changes every run",
			actual:   `{"latest_block_height": "1482", "catching_up": false}`,
			expected: `{"latest_block_height": "<...>", "catching_up": false}`,
			want:     true,
		},
		{
			name:     "a changed value still fails",
			actual:   `{"latest_block_height": "1482", "catching_up": true}`,
			expected: `{"latest_block_height": "<...>", "catching_up": false}`,
			want:     false,
		},
		{
			name:     "surrounding log noise does not matter",
			actual:   "INFO starting\nheight: 12\nINFO done",
			expected: "height: 12",
			want:     true,
		},
		{
			name:     "re-indenting a block to fit the page does not break it",
			actual:   "{\n\t\"ok\": true\n}",
			expected: "{\n      \"ok\": true\n}",
			want:     true,
		},
		{
			name:     "regex metacharacters in output are literal",
			actual:   "cost: $1.50 (approx)",
			expected: "cost: $1.50 (approx)",
			want:     true,
		},
		{
			name:     "an escaped placeholder is matched literally",
			actual:   "the template reads <...> here",
			expected: `the template reads \<...> here`,
			want:     true,
		},
		{
			name:     "an escaped placeholder does not match something else",
			actual:   "the template reads 42 here",
			expected: `the template reads \<...> here`,
			want:     false,
		},
		{
			name:     "missing output fails",
			actual:   "",
			expected: "height: <...>",
			want:     false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := matchExpected(tc.actual, tc.expected); got != tc.want {
				t.Errorf("matchExpected() = %v, want %v\nactual:   %q\nexpected: %q", got, tc.want, tc.actual, tc.expected)
			}
		})
	}
}

func TestDescribeMismatchNamesTheFirstMissingLine(t *testing.T) {
	actual := "height: 12\nready: false"
	expected := "height: <...>\nready: true"

	got := describeMismatch(actual, expected)
	want := "first line that did not appear: ready: true"
	if got != want {
		t.Errorf("describeMismatch() = %q, want %q", got, want)
	}
}
