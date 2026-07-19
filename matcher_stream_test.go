package los

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLos_MatcherPreservesRegexSemanticsAcrossChunks(t *testing.T) {
	tests := []struct {
		name     string
		pair     *Pair
		chunks   []string
		expected [][]Result
		final    []Result
	}{
		{
			name:   "greedy head waits for disambiguation",
			pair:   NewPair(`a+`, `!`, WithRegexHead(REGEX_MODE_PERL)),
			chunks: []string{"a", "aa!"},
			expected: [][]Result{
				nil,
				{
					regexResult{state: STATE_HEAD, raw: []byte("aaa"), matchcap: []int{0, 3}},
					textResult{state: STATE_TAIL, raw: []byte("!")},
				},
			},
		},
		{
			name:   "word boundary waits for lookahead",
			pair:   NewPair(`foo\b`, `!`, WithRegexHead(REGEX_MODE_PERL)),
			chunks: []string{"foo", "!"},
			expected: [][]Result{
				nil,
				{
					regexResult{state: STATE_HEAD, raw: []byte("foo"), matchcap: []int{0, 3}},
					textResult{state: STATE_TAIL, raw: []byte("!")},
				},
			},
		},
		{
			name:     "end anchor resolves on finish",
			pair:     NewPair(`abc$`, `!`, WithRegexHead(REGEX_MODE_PERL)),
			chunks:   []string{"ab", "c"},
			expected: [][]Result{nil, nil},
			final: []Result{
				regexResult{state: STATE_HEAD, raw: []byte("abc"), matchcap: []int{0, 3}},
			},
		},
		{
			name:     "finish releases an unmatched partial",
			pair:     NewPair(`abc`, `!`, WithRegexHead(REGEX_MODE_PERL)),
			chunks:   []string{"ab"},
			expected: [][]Result{nil},
			final:    []Result{textResult{state: STATE_NONE, raw: []byte("ab")}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher := NewMatcher(tt.pair)
			t.Cleanup(func() { require.NoError(t, matcher.Close()) })

			for i, chunk := range tt.chunks {
				require.Equal(t, tt.expected[i], slices.Collect(matcher.Match(chunk)))
			}
			require.Equal(t, tt.final, slices.Collect(matcher.Finish()))
		})
	}
}

func TestLos_NewMatcherRejectsEmptyPatterns(t *testing.T) {
	tests := []struct {
		name string
		pair func() *Pair
	}{
		{name: "empty fixed head", pair: func() *Pair { return NewPair("", "tail") }},
		{name: "empty regex head", pair: func() *Pair { return NewPair(`a*`, "tail", WithRegexHead(REGEX_MODE_PERL)) }},
		{name: "empty regex tail", pair: func() *Pair { return NewPair("head", `(?:x?)`, WithRegexTail(REGEX_MODE_PERL)) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.PanicsWithValue(t, "los: pattern must not match empty input", func() {
				NewMatcher(tt.pair())
			})
		})
	}
}

func TestLos_RegexResultAdjustsCapturesAfterUnmatchedInput(t *testing.T) {
	matcher := NewMatcher(NewPair(`a(b)?c`, `!`, WithRegexHead(REGEX_MODE_PERL)))
	t.Cleanup(func() { require.NoError(t, matcher.Close()) })

	results := slices.Collect(matcher.Match("xxac!"))
	require.Equal(t, []Result{
		textResult{state: STATE_NONE, raw: []byte("xx")},
		regexResult{state: STATE_HEAD, raw: []byte("ac"), matchcap: []int{0, 2, -1, -1}},
		textResult{state: STATE_TAIL, raw: []byte("!")},
	}, results)
	require.Equal(t, []string{"ac", ""}, slices.Collect(results[1].Matches()))
}

func TestLos_MatcherAlternatesRegexHeadAndTailAcrossChunks(t *testing.T) {
	matcher := NewMatcher(NewPair(
		`<([a-z]+)>`, `</([a-z]+)>`,
		WithRegexHead(REGEX_MODE_PERL),
		WithRegexTail(REGEX_MODE_PERL),
	))
	t.Cleanup(func() { require.NoError(t, matcher.Close()) })

	chunks := []string{"pre<ta", "g>one</ta", "g>mid<x>two</x>post"}
	expected := [][]Result{
		{textResult{state: STATE_NONE, raw: []byte("pre")}},
		{
			regexResult{state: STATE_HEAD, raw: []byte("<tag>"), matchcap: []int{0, 5, 1, 4}},
			textResult{state: STATE_BODY, raw: []byte("one")},
		},
		{
			regexResult{state: STATE_TAIL, raw: []byte("</tag>"), matchcap: []int{0, 6, 2, 5}},
			textResult{state: STATE_NONE, raw: []byte("mid")},
			regexResult{state: STATE_HEAD, raw: []byte("<x>"), matchcap: []int{0, 3, 1, 2}},
			textResult{state: STATE_BODY, raw: []byte("two")},
			regexResult{state: STATE_TAIL, raw: []byte("</x>"), matchcap: []int{0, 4, 2, 3}},
			textResult{state: STATE_NONE, raw: []byte("post")},
		},
	}

	for i, chunk := range chunks {
		require.Equal(t, expected[i], slices.Collect(matcher.Match(chunk)))
	}
}

func TestLos_MatcherResetOperationsStartNewLogicalStream(t *testing.T) {
	tests := []struct {
		name   string
		finish bool
	}{
		{name: "drain"},
		{name: "finish", finish: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher := NewMatcher(NewPair(`^abc`, `!`, WithRegexHead(REGEX_MODE_PERL)))
			t.Cleanup(func() { require.NoError(t, matcher.Close()) })

			require.Equal(t, []Result{textResult{state: STATE_NONE, raw: []byte("x")}},
				slices.Collect(matcher.Match("x")))
			if tt.finish {
				require.Empty(t, slices.Collect(matcher.Finish()))
			} else {
				require.Empty(t, matcher.Drain())
			}
			require.Equal(t, []Result{
				regexResult{state: STATE_HEAD, raw: []byte("abc"), matchcap: []int{0, 3}},
				textResult{state: STATE_TAIL, raw: []byte("!")},
			}, slices.Collect(matcher.Match("abc!")))
		})
	}
}
