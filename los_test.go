package los

import (
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLos_Matcher_Kmp(t *testing.T) {
	matcher := NewMatcher(NewPair("prologue", "epilogue"))
	defer matcher.Close() // nolint: errcheck

	tests := []struct {
		name            string
		contents        []string
		expectedResults [][]Result
		drainedContent  string
	}{
		{
			name:            "pass through empty content",
			contents:        []string{"test"},
			expectedResults: [][]Result{{textResult{state: STATE_NONE, raw: []byte("test")}}},
			drainedContent:  "", // Remaining unmatched content
		},
		{
			name:            "single partial match 'pro'",
			contents:        []string{"pro"},
			expectedResults: [][]Result{nil}, // No complete match, results should be empty
			drainedContent:  "pro",           // Remaining unmatched content
		},
		{
			name:            "single complete prologue",
			contents:        []string{"prologue"},
			expectedResults: [][]Result{{textResult{state: STATE_HEAD, raw: []byte("prologue")}}},
			drainedContent:  "", // All content matched
		},
		{
			name:     "multiple contents with complete matches",
			contents: []string{"prologue", "content", "epilogue"},
			expectedResults: [][]Result{{
				textResult{state: STATE_HEAD, raw: []byte("prologue")},
			}, {
				textResult{state: STATE_BODY, raw: []byte("content")},
			}, {
				textResult{state: STATE_TAIL, raw: []byte("epilogue")},
			}},
			drainedContent: "", // All content matched across calls
		},
		{
			name:     "combined content with both prologue and epilogue",
			contents: []string{"prologue middle content epilogue"},
			expectedResults: [][]Result{{
				textResult{state: STATE_HEAD, raw: []byte("prologue")},
				textResult{state: STATE_BODY, raw: []byte(" middle content ")},
				textResult{state: STATE_TAIL, raw: []byte("epilogue")},
			}},
			drainedContent: "", // All content matched
		},
		{
			name:     "complete prologue and partial epilogue",
			contents: []string{"prologuedata", "epilo"},
			expectedResults: [][]Result{{
				textResult{state: STATE_HEAD, raw: []byte("prologue")},
				textResult{state: STATE_BODY, raw: []byte("data")},
			}, nil},
			drainedContent: "epilo", // All content matched
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i, content := range tt.contents {
				expected := tt.expectedResults[i]
				got := slices.Collect(matcher.Match(content))
				require.Equal(t, expected, got)
			}

			drainedContent := matcher.Drain()
			require.Equal(t, tt.drainedContent, drainedContent)
		})
	}
}

func TestLos_Matcher_Regex(t *testing.T) {
	invalidByte := string([]byte{0xc2})
	invalidMatch := invalidByte + "\x00-Go"

	tests := []struct {
		name           string
		pair           *Pair
		contents       []string
		expected       [][]resultExpectation
		finish         bool
		expectedFinal  []resultExpectation
		drainedContent string
	}{
		{
			name:     "multiple fixed-tail cycles across chunks",
			pair:     NewPair(`ab([A-Z]+)c`, `xyz`, WithRegexHead(REGEX_MODE_PERL)),
			contents: []string{"noisea", "bFO", "Ocbodyx", "yzgapabBARcinside", "xyzend"},
			expected: [][]resultExpectation{
				{{state: STATE_NONE, text: "noise", matches: []string{"noise"}}},
				nil,
				{
					{state: STATE_HEAD, text: "abFOOc", matches: []string{"abFOOc", "FOO"}},
					{state: STATE_BODY, text: "body", matches: []string{"body"}},
				},
				{
					{state: STATE_TAIL, text: "xyz", matches: []string{"xyz"}},
					{state: STATE_NONE, text: "gap", matches: []string{"gap"}},
					{state: STATE_HEAD, text: "abBARc", matches: []string{"abBARc", "BAR"}},
					{state: STATE_BODY, text: "inside", matches: []string{"inside"}},
				},
				{
					{state: STATE_TAIL, text: "xyz", matches: []string{"xyz"}},
					{state: STATE_NONE, text: "end", matches: []string{"end"}},
				},
			},
		},
		{
			name: "regex head and tail with optional capture and unicode body",
			pair: NewPair(
				`<([a-z]+)(?: id=([0-9]+))?>`, `</([a-z]+)>`,
				WithRegexHead(REGEX_MODE_PERL),
				WithRegexTail(REGEX_MODE_PERL),
			),
			contents: []string{"pre<item id=4", "2>日本</it", "em>mid<empty>", "日本</empty>post"},
			expected: [][]resultExpectation{
				{{state: STATE_NONE, text: "pre", matches: []string{"pre"}}},
				{
					{state: STATE_HEAD, text: "<item id=42>", matches: []string{"<item id=42>", "item", "42"}},
					{state: STATE_BODY, text: "日本", matches: []string{"日本"}},
				},
				{
					{state: STATE_TAIL, text: "</item>", matches: []string{"</item>", "item"}},
					{state: STATE_NONE, text: "mid", matches: []string{"mid"}},
					{state: STATE_HEAD, text: "<empty>", matches: []string{"<empty>", "empty", ""}},
				},
				{
					{state: STATE_BODY, text: "日本", matches: []string{"日本"}},
					{state: STATE_TAIL, text: "</empty>", matches: []string{"</empty>", "empty"}},
					{state: STATE_NONE, text: "post", matches: []string{"post"}},
				},
			},
			finish: true,
		},
		{
			name:     "finish resolves greedy head and releases partial tail",
			pair:     NewPair(`ab(.*)c`, `xyz`, WithRegexHead(REGEX_MODE_PERL)),
			contents: []string{"prefixabA", "BCcbodyxy"},
			expected: [][]resultExpectation{
				{{state: STATE_NONE, text: "prefix", matches: []string{"prefix"}}},
				nil,
			},
			finish: true,
			expectedFinal: []resultExpectation{
				{state: STATE_HEAD, text: "abABCc", matches: []string{"abABCc", "ABC"}},
				{state: STATE_BODY, text: "bodyxy", matches: []string{"bodyxy"}},
			},
		},
		{
			name:     "posix head chooses longest alternative",
			pair:     NewPair(`a|aa`, `!`, WithRegexHead(REGEX_MODE_POSIX)),
			contents: []string{"a", "a!rest"},
			expected: [][]resultExpectation{
				nil,
				{
					{state: STATE_HEAD, text: "aa", matches: []string{"aa"}},
					{state: STATE_TAIL, text: "!", matches: []string{"!"}},
					{state: STATE_NONE, text: "rest", matches: []string{"rest"}},
				},
			},
		},
		{
			name:     "preferred alternative waits for word-boundary lookahead",
			pair:     NewPair(`\B(foo|fo)\B`, `END`, WithRegexHead(REGEX_MODE_PERL)),
			contents: []string{"xfo", "oYEND"},
			expected: [][]resultExpectation{
				{{state: STATE_NONE, text: "x", matches: []string{"x"}}},
				{
					{state: STATE_HEAD, text: "foo", matches: []string{"foo", "foo"}},
					{state: STATE_BODY, text: "Y", matches: []string{"Y"}},
					{state: STATE_TAIL, text: "END", matches: []string{"END"}},
				},
			},
		},
		{
			name:     "invalid utf8 sequence and captures span chunks",
			pair:     NewPair(`(\x{FFFD})\x00-([[:alpha:]]+)`, `!`, WithRegexHead(REGEX_MODE_PERL)),
			contents: []string{"pre" + invalidByte, "\x00-Go!"},
			expected: [][]resultExpectation{
				{{state: STATE_NONE, text: "pre", matches: []string{"pre"}}},
				{
					{state: STATE_HEAD, text: invalidMatch, matches: []string{invalidMatch, invalidByte, "Go"}},
					{state: STATE_TAIL, text: "!", matches: []string{"!"}},
				},
			},
		},
		{
			name: "multiline cycles preserve asymmetric captures and context",
			pair: NewPair(
				`(?m)^([[:alpha:]]+):(?:(\d+)|([[:alpha:]]+))$`, `(?m)^END$`,
				WithRegexHead(REGEX_MODE_PERL),
				WithRegexTail(REGEX_MODE_PERL),
			),
			contents: []string{"junk\nname:va", "lue\nEN", "D\nnext\ncount:4", "2\nEND\n"},
			expected: [][]resultExpectation{
				{{state: STATE_NONE, text: "junk\n", matches: []string{"junk\n"}}},
				{
					{state: STATE_HEAD, text: "name:value", matches: []string{"name:value", "name", "", "value"}},
					{state: STATE_BODY, text: "\n", matches: []string{"\n"}},
				},
				{
					{state: STATE_TAIL, text: "END", matches: []string{"END"}},
					{state: STATE_NONE, text: "\nnext\n", matches: []string{"\nnext\n"}},
				},
				{
					{state: STATE_HEAD, text: "count:42", matches: []string{"count:42", "count", "42", ""}},
					{state: STATE_BODY, text: "\n", matches: []string{"\n"}},
					{state: STATE_TAIL, text: "END", matches: []string{"END"}},
					{state: STATE_NONE, text: "\n", matches: []string{"\n"}},
				},
			},
			finish: true,
		},
		{
			name:     "absolute end anchored regex tail resolves on finish",
			pair:     NewPair(`BEGIN`, `([a-z]+)\z`, WithRegexTail(REGEX_MODE_PERL)),
			contents: []string{"noiseBEG", "INpay", "load"},
			expected: [][]resultExpectation{
				{{state: STATE_NONE, text: "noise", matches: []string{"noise"}}},
				{{state: STATE_HEAD, text: "BEGIN", matches: []string{"BEGIN"}}},
				nil,
			},
			finish: true,
			expectedFinal: []resultExpectation{
				{state: STATE_TAIL, text: "payload", matches: []string{"payload", "payload"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher := NewMatcher(tt.pair)
			t.Cleanup(func() { require.NoError(t, matcher.Close()) })

			for i, content := range tt.contents {
				requireMatcherResults(t, tt.expected[i], matcher.Match(content))
			}

			if tt.finish {
				requireMatcherResults(t, tt.expectedFinal, matcher.Finish())
			} else {
				require.Equal(t, tt.drainedContent, matcher.Drain())
			}
		})
	}
}

func TestLos_Matcher_LongTextWithGnarlySnippet(t *testing.T) {
	head := "<<BEGIN kind=ALPHA id=2048>>"
	tail := "<<END kind=ALPHA>>"
	prefix := strings.Repeat("ordinary ", 700) +
		"<<BEGIN kind=ALPHA id=20x8>> " +
		strings.Repeat("preface ", 400)
	body := "\n" + strings.Repeat("payload日本語 ", 500) +
		"<<END kind=ALPH4>> " +
		strings.Repeat("continuation ", 300) + "\n"
	suffix := strings.Repeat("trailing ", 300)
	input := prefix + head + body + tail + suffix
	require.Greater(t, len(strings.Fields(input)), 1024)

	matcher := NewMatcher(NewPair(
		`<<BEGIN kind=([A-Z]+) id=([0-9]{4})>>`,
		`<<END kind=([A-Z]+)>>`,
		WithRegexHead(REGEX_MODE_PERL),
		WithRegexTail(REGEX_MODE_PERL),
	))
	t.Cleanup(func() { require.NoError(t, matcher.Close()) })

	var reconstructed, unmatched, matchedBody strings.Builder
	var boundaries []resultExpectation
	collect := func(results Results) {
		for result := range results {
			reconstructed.WriteString(result.String())
			switch result.State() {
			case STATE_NONE:
				unmatched.WriteString(result.String())
			case STATE_BODY:
				matchedBody.WriteString(result.String())
			case STATE_HEAD, STATE_TAIL:
				boundaries = append(boundaries, resultExpectation{
					state:   result.State(),
					text:    result.String(),
					matches: slices.Collect(result.Matches()),
				})
			}
		}
	}

	chunkSizes := []int{1, 2, 3, 5, 8, 13, 21, 34, 55, 89, 144, 233}
	for offset, chunk := 0, 0; offset < len(input); chunk++ {
		end := min(offset+chunkSizes[chunk%len(chunkSizes)], len(input))
		collect(matcher.Match(input[offset:end]))
		offset = end
	}
	collect(matcher.Finish())

	require.Equal(t, input, reconstructed.String())
	require.Equal(t, prefix+suffix, unmatched.String())
	require.Equal(t, body, matchedBody.String())
	require.Equal(t, []resultExpectation{
		{state: STATE_HEAD, text: head, matches: []string{head, "ALPHA", "2048"}},
		{state: STATE_TAIL, text: tail, matches: []string{tail, "ALPHA"}},
	}, boundaries)
}

type resultExpectation struct {
	state   State
	text    string
	matches []string
	value   any
}

func requireMatcherResults(t *testing.T, expected []resultExpectation, results Results) {
	t.Helper()
	actual := slices.Collect(results)
	require.Len(t, actual, len(expected))
	for i, result := range actual {
		require.Equal(t, expected[i].state, result.State(), "result %d state", i)
		require.Equal(t, expected[i].text, result.String(), "result %d text", i)
		require.Equal(t, expected[i].matches, slices.Collect(result.Matches()), "result %d matches", i)
		require.Equal(t, expected[i].value, result.Value(), "result %d value", i)
	}
}
