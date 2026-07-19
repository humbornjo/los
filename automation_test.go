package los

import (
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLos_AutomationMatcherSelectsPairsAndValues(t *testing.T) {
	first := NewPair("<a>", "</a>").WithValue("a")
	second := NewPair("<b>", "</b>").WithValue("b")
	matcher := NewMatcher(first, second)
	t.Cleanup(func() { require.NoError(t, matcher.Close()) })
	first.WithValue("changed")
	second.WithValue("changed")

	requireMatcherResults(t, []resultExpectation{
		{state: STATE_NONE, text: "pre", matches: []string{"pre"}},
		{state: STATE_HEAD, text: "<a>", matches: []string{"<a>"}, value: "a"},
		{state: STATE_BODY, text: "one<b>nested</b>two", matches: []string{"one<b>nested</b>two"}, value: "a"},
		{state: STATE_TAIL, text: "</a>", matches: []string{"</a>"}, value: "a"},
		{state: STATE_NONE, text: "mid", matches: []string{"mid"}},
		{state: STATE_HEAD, text: "<b>", matches: []string{"<b>"}, value: "b"},
		{state: STATE_BODY, text: "three", matches: []string{"three"}, value: "b"},
		{state: STATE_TAIL, text: "</b>", matches: []string{"</b>"}, value: "b"},
		{state: STATE_NONE, text: "post", matches: []string{"post"}},
	}, matcher.Match("pre<a>one<b>nested</b>two</a>mid<b>three</b>post"))
	require.Empty(t, slices.Collect(matcher.Finish()))
}

func TestLos_MatcherSnapshotsSinglePairValue(t *testing.T) {
	pair := NewPair("HEAD", "TAIL").WithValue("original")
	matcher := NewMatcher(pair)
	t.Cleanup(func() { require.NoError(t, matcher.Close()) })
	pair.WithValue("changed")

	requireMatcherResults(t, []resultExpectation{
		{state: STATE_NONE, text: "pre", matches: []string{"pre"}},
		{state: STATE_HEAD, text: "HEAD", matches: []string{"HEAD"}, value: "original"},
		{state: STATE_BODY, text: "body", matches: []string{"body"}, value: "original"},
		{state: STATE_TAIL, text: "TAIL", matches: []string{"TAIL"}, value: "original"},
		{state: STATE_NONE, text: "post", matches: []string{"post"}},
	}, matcher.Match("preHEADbodyTAILpost"))
}

func TestLos_AutomationMatcherPreservesOrderedHeadPriority(t *testing.T) {
	tests := []struct {
		name     string
		pairs    []*Pair
		chunks   []string
		expected [][]resultExpectation
	}{
		{
			name: "higher priority longer literal waits",
			pairs: []*Pair{
				NewPair("foobar", "!").WithValue("long"),
				NewPair("foo", "?").WithValue("short"),
			},
			chunks: []string{"xfoo", "bar!"},
			expected: [][]resultExpectation{
				{{state: STATE_NONE, text: "x", matches: []string{"x"}}},
				{
					{state: STATE_HEAD, text: "foobar", matches: []string{"foobar"}, value: "long"},
					{state: STATE_TAIL, text: "!", matches: []string{"!"}, value: "long"},
				},
			},
		},
		{
			name: "higher priority shorter literal wins immediately",
			pairs: []*Pair{
				NewPair("foo", "?").WithValue("short"),
				NewPair("foobar", "!").WithValue("long"),
			},
			chunks: []string{"xfoobar?"},
			expected: [][]resultExpectation{{
				{state: STATE_NONE, text: "x", matches: []string{"x"}},
				{state: STATE_HEAD, text: "foo", matches: []string{"foo"}, value: "short"},
				{state: STATE_BODY, text: "bar", matches: []string{"bar"}, value: "short"},
				{state: STATE_TAIL, text: "?", matches: []string{"?"}, value: "short"},
			}},
		},
		{
			name: "earlier start beats registration order",
			pairs: []*Pair{
				NewPair("bar", "?").WithValue("bar"),
				NewPair("foo", "!").WithValue("foo"),
			},
			chunks: []string{"foobar!"},
			expected: [][]resultExpectation{{
				{state: STATE_HEAD, text: "foo", matches: []string{"foo"}, value: "foo"},
				{state: STATE_BODY, text: "bar", matches: []string{"bar"}, value: "foo"},
				{state: STATE_TAIL, text: "!", matches: []string{"!"}, value: "foo"},
			}},
		},
		{
			name: "duplicate head uses first pair",
			pairs: []*Pair{
				NewPair("tag", "A").WithValue("first"),
				NewPair("tag", "B").WithValue("second"),
			},
			chunks: []string{"tagA"},
			expected: [][]resultExpectation{{
				{state: STATE_HEAD, text: "tag", matches: []string{"tag"}, value: "first"},
				{state: STATE_TAIL, text: "A", matches: []string{"A"}, value: "first"},
			}},
		},
		{
			name: "failure link output keeps earliest start",
			pairs: []*Pair{
				NewPair("hers", "X").WithValue("hers"),
				NewPair("she", "!").WithValue("she"),
				NewPair("he", "?").WithValue("he"),
			},
			chunks: []string{"usher!"},
			expected: [][]resultExpectation{{
				{state: STATE_NONE, text: "u", matches: []string{"u"}},
				{state: STATE_HEAD, text: "she", matches: []string{"she"}, value: "she"},
				{state: STATE_BODY, text: "r", matches: []string{"r"}, value: "she"},
				{state: STATE_TAIL, text: "!", matches: []string{"!"}, value: "she"},
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher := NewMatcher(tt.pairs[0], tt.pairs[1:]...)
			t.Cleanup(func() { require.NoError(t, matcher.Close()) })
			for i, chunk := range tt.chunks {
				requireMatcherResults(t, tt.expected[i], matcher.Match(chunk))
			}
			require.Empty(t, slices.Collect(matcher.Finish()))
		})
	}
}

func TestLos_AutomationMatcherMergesFixedAndRegexHeads(t *testing.T) {
	tests := []struct {
		name     string
		pairs    []*Pair
		chunks   []string
		expected [][]resultExpectation
	}{
		{
			name: "pending higher priority perl head blocks fixed head",
			pairs: []*Pair{
				NewPair(`foo(bar)?`, `!`, WithRegexHead(REGEX_MODE_PERL)).WithValue("regex"),
				NewPair("foo", "?").WithValue("fixed"),
			},
			chunks: []string{"foo", "bar!"},
			expected: [][]resultExpectation{
				nil,
				{
					{state: STATE_HEAD, text: "foobar", matches: []string{"foobar", "bar"}, value: "regex"},
					{state: STATE_TAIL, text: "!", matches: []string{"!"}, value: "regex"},
				},
			},
		},
		{
			name: "failed higher priority boundary releases fixed head",
			pairs: []*Pair{
				NewPair(`foo\b`, `?`, WithRegexHead(REGEX_MODE_PERL)).WithValue("regex"),
				NewPair("foo", "!").WithValue("fixed"),
			},
			chunks: []string{"foo", "x!"},
			expected: [][]resultExpectation{
				nil,
				{
					{state: STATE_HEAD, text: "foo", matches: []string{"foo"}, value: "fixed"},
					{state: STATE_BODY, text: "x", matches: []string{"x"}, value: "fixed"},
					{state: STATE_TAIL, text: "!", matches: []string{"!"}, value: "fixed"},
				},
			},
		},
		{
			name: "higher priority fixed head beats pending regex",
			pairs: []*Pair{
				NewPair("foo", "?").WithValue("fixed"),
				NewPair(`foo(bar)?`, `!`, WithRegexHead(REGEX_MODE_PERL)).WithValue("regex"),
			},
			chunks: []string{"foobar?"},
			expected: [][]resultExpectation{{
				{state: STATE_HEAD, text: "foo", matches: []string{"foo"}, value: "fixed"},
				{state: STATE_BODY, text: "bar", matches: []string{"bar"}, value: "fixed"},
				{state: STATE_TAIL, text: "?", matches: []string{"?"}, value: "fixed"},
			}},
		},
		{
			name: "posix head keeps longest semantics and captures",
			pairs: []*Pair{
				NewPair(`(a|aa)+`, `!`, WithRegexHead(REGEX_MODE_POSIX)).WithValue("posix"),
				NewPair("aa", "?").WithValue("fixed"),
			},
			chunks: []string{"aa", "a!"},
			expected: [][]resultExpectation{
				nil,
				{
					{state: STATE_HEAD, text: "aaa", matches: []string{"aaa", "a"}, value: "posix"},
					{state: STATE_TAIL, text: "!", matches: []string{"!"}, value: "posix"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher := NewMatcher(tt.pairs[0], tt.pairs[1:]...)
			t.Cleanup(func() { require.NoError(t, matcher.Close()) })
			for i, chunk := range tt.chunks {
				requireMatcherResults(t, tt.expected[i], matcher.Match(chunk))
			}
			require.Empty(t, slices.Collect(matcher.Finish()))
		})
	}
}

func TestLos_AutomationMatcherSupportsMixedHeadAndTailModes(t *testing.T) {
	matcher := NewMatcher(
		NewPair("<fixed>", "</fixed>").WithValue("fixed"),
		NewPair(
			`P([0-9]+)`, `(a|aa)`,
			WithRegexHead(REGEX_MODE_PERL),
			WithRegexTail(REGEX_MODE_POSIX),
		).WithValue("perl"),
		NewPair(
			`(x|xx)`, `!+`,
			WithRegexHead(REGEX_MODE_POSIX),
			WithRegexTail(REGEX_MODE_PERL),
		).WithValue("posix"),
	)
	t.Cleanup(func() { require.NoError(t, matcher.Close()) })

	var actual []resultExpectation
	for _, chunk := range []string{"x", "x!!", "!P4", "2a", "a<fixed></fixed>"} {
		actual = appendResultExpectations(actual, matcher.Match(chunk))
	}
	actual = appendResultExpectations(actual, matcher.Finish())
	require.Equal(t, []resultExpectation{
		{state: STATE_HEAD, text: "xx", matches: []string{"xx", "xx"}, value: "posix"},
		{state: STATE_TAIL, text: "!!!", matches: []string{"!!!"}, value: "posix"},
		{state: STATE_HEAD, text: "P42", matches: []string{"P42", "42"}, value: "perl"},
		{state: STATE_TAIL, text: "aa", matches: []string{"aa", "aa"}, value: "perl"},
		{state: STATE_HEAD, text: "<fixed>", matches: []string{"<fixed>"}, value: "fixed"},
		{state: STATE_TAIL, text: "</fixed>", matches: []string{"</fixed>"}, value: "fixed"},
	}, actual)
}

func TestLos_AutomationMatcherResolvesPendingHeadsAtFinish(t *testing.T) {
	matcher := NewMatcher(
		NewPair("foobar", "?").WithValue("fixed"),
		NewPair(`foo`, `!`, WithRegexHead(REGEX_MODE_PERL)).WithValue("regex"),
	)
	t.Cleanup(func() { require.NoError(t, matcher.Close()) })

	require.Empty(t, slices.Collect(matcher.Match("foo")))
	requireMatcherResults(t, []resultExpectation{
		{state: STATE_HEAD, text: "foo", matches: []string{"foo"}, value: "regex"},
	}, matcher.Finish())
}

func TestLos_AutomationMatcherPreservesLogicalStreamContext(t *testing.T) {
	tests := []struct {
		name     string
		pairs    []*Pair
		chunks   []string
		expected []resultExpectation
	}{
		{
			name: "begin anchor applies only once",
			pairs: []*Pair{
				NewPair(`^H`, `T`, WithRegexHead(REGEX_MODE_PERL)).WithValue("anchored"),
				NewPair("unused", "!").WithValue("unused"),
			},
			chunks: []string{"H", "TH"},
			expected: []resultExpectation{
				{state: STATE_HEAD, text: "H", matches: []string{"H"}, value: "anchored"},
				{state: STATE_TAIL, text: "T", matches: []string{"T"}, value: "anchored"},
				{state: STATE_NONE, text: "H", matches: []string{"H"}},
			},
		},
		{
			name: "regex tail sees fixed head preceding rune",
			pairs: []*Pair{
				NewPair("x", `\Bfoo`, WithRegexTail(REGEX_MODE_PERL)).WithValue("selected"),
				NewPair("unused", "!").WithValue("unused"),
			},
			chunks: []string{"xfo", "o!"},
			expected: []resultExpectation{
				{state: STATE_HEAD, text: "x", matches: []string{"x"}, value: "selected"},
				{state: STATE_TAIL, text: "foo", matches: []string{"foo"}, value: "selected"},
				{state: STATE_NONE, text: "!", matches: []string{"!"}},
			},
		},
		{
			name: "next regex head sees newline consumed by prior pair",
			pairs: []*Pair{
				NewPair("A", "\n").WithValue("first"),
				NewPair(`(?m)^B`, `!`, WithRegexHead(REGEX_MODE_PERL)).WithValue("second"),
			},
			chunks: []string{"A\n", "B!"},
			expected: []resultExpectation{
				{state: STATE_HEAD, text: "A", matches: []string{"A"}, value: "first"},
				{state: STATE_TAIL, text: "\n", matches: []string{"\n"}, value: "first"},
				{state: STATE_HEAD, text: "B", matches: []string{"B"}, value: "second"},
				{state: STATE_TAIL, text: "!", matches: []string{"!"}, value: "second"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher := NewMatcher(tt.pairs[0], tt.pairs[1:]...)
			t.Cleanup(func() { require.NoError(t, matcher.Close()) })
			var actual []resultExpectation
			for _, chunk := range tt.chunks {
				actual = appendResultExpectations(actual, matcher.Match(chunk))
			}
			actual = appendResultExpectations(actual, matcher.Finish())
			require.Equal(t, tt.expected, actual)
		})
	}
}

func TestLos_AutomationMatcherHandlesInvalidUtf8AcrossChunks(t *testing.T) {
	invalid := string([]byte{0xc2})
	matcher := NewMatcher(
		NewPair("unused", "?").WithValue("fixed"),
		NewPair(`(\x{FFFD})x`, `!`, WithRegexHead(REGEX_MODE_PERL)).WithValue("regex"),
	)
	t.Cleanup(func() { require.NoError(t, matcher.Close()) })

	require.Empty(t, slices.Collect(matcher.Match(invalid)))
	requireMatcherResults(t, []resultExpectation{
		{state: STATE_HEAD, text: invalid + "x", matches: []string{invalid + "x", invalid}, value: "regex"},
		{state: STATE_TAIL, text: "!", matches: []string{"!"}, value: "regex"},
	}, matcher.Match("x!"))
}

func TestLos_AutomationMatcherMatchesAcrossEveryByteBoundary(t *testing.T) {
	tests := []struct {
		name     string
		pairs    func() []*Pair
		input    string
		expected []resultExpectation
	}{
		{
			name: "longer registered literal",
			pairs: func() []*Pair {
				return []*Pair{
					NewPair("foobar", "!").WithValue("long"),
					NewPair("foo", "?").WithValue("short"),
				}
			},
			input: "foobar!",
			expected: []resultExpectation{
				{state: STATE_HEAD, text: "foobar", matches: []string{"foobar"}, value: "long"},
				{state: STATE_TAIL, text: "!", matches: []string{"!"}, value: "long"},
			},
		},
		{
			name: "failure link candidate",
			pairs: func() []*Pair {
				return []*Pair{
					NewPair("hers", "?").WithValue("hers"),
					NewPair("she", "!").WithValue("she"),
					NewPair("he", "X").WithValue("he"),
				}
			},
			input: "usher!",
			expected: []resultExpectation{
				{state: STATE_NONE, text: "u", matches: []string{"u"}},
				{state: STATE_HEAD, text: "she", matches: []string{"she"}, value: "she"},
				{state: STATE_BODY, text: "r", matches: []string{"r"}, value: "she"},
				{state: STATE_TAIL, text: "!", matches: []string{"!"}, value: "she"},
			},
		},
		{
			name: "regex and fixed overlap",
			pairs: func() []*Pair {
				return []*Pair{
					NewPair(`foo(bar)?`, `!`, WithRegexHead(REGEX_MODE_PERL)).WithValue("regex"),
					NewPair("foo", "?").WithValue("fixed"),
				}
			},
			input: "foobar!",
			expected: []resultExpectation{
				{state: STATE_HEAD, text: "foobar", matches: []string{"foobar", "bar"}, value: "regex"},
				{state: STATE_TAIL, text: "!", matches: []string{"!"}, value: "regex"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			partitions := make([][]string, 0, len(tt.input))
			for split := 1; split < len(tt.input); split++ {
				partitions = append(partitions, []string{tt.input[:split], tt.input[split:]})
			}
			byteChunks := make([]string, len(tt.input))
			for i := range tt.input {
				byteChunks[i] = tt.input[i : i+1]
			}
			partitions = append(partitions, byteChunks)

			for i, chunks := range partitions {
				t.Run(string(rune('A'+i)), func(t *testing.T) {
					pairs := tt.pairs()
					matcher := NewMatcher(pairs[0], pairs[1:]...)
					var actual []resultExpectation
					for _, chunk := range chunks {
						actual = appendResultExpectations(actual, matcher.Match(chunk))
					}
					actual = appendResultExpectations(actual, matcher.Finish())
					require.Equal(t, tt.expected, actual, "chunks=%q", chunks)
					require.NoError(t, matcher.Close())
				})
			}
		})
	}
}

func TestLos_AutomationMatcherFinishDrainAndReset(t *testing.T) {
	matcher := NewMatcher(
		NewPair(`^BEGIN`, `([a-z]+)\z`, WithRegexHead(REGEX_MODE_PERL), WithRegexTail(REGEX_MODE_PERL)).WithValue("anchored"),
		NewPair("ALT", "!").WithValue("alt"),
	)
	t.Cleanup(func() { require.NoError(t, matcher.Close()) })

	requireMatcherResults(t, []resultExpectation{
		{state: STATE_HEAD, text: "BEGIN", matches: []string{"BEGIN"}, value: "anchored"},
	}, matcher.Match("BEGINpayload"))
	requireMatcherResults(t, []resultExpectation{
		{state: STATE_TAIL, text: "payload", matches: []string{"payload", "payload"}, value: "anchored"},
	}, matcher.Finish())

	require.Empty(t, slices.Collect(matcher.Match("AL")))
	require.Equal(t, "AL", matcher.Drain())
	requireMatcherResults(t, []resultExpectation{
		{state: STATE_HEAD, text: "ALT", matches: []string{"ALT"}, value: "alt"},
		{state: STATE_TAIL, text: "!", matches: []string{"!"}, value: "alt"},
	}, matcher.Match("ALT!"))
}

func TestLos_AutomationMatcherValidatesPairs(t *testing.T) {
	require.PanicsWithValue(t, "los: pair must not be nil", func() {
		NewMatcher(nil)
	})
	require.PanicsWithValue(t, "los: pair must not be nil", func() {
		NewMatcher(NewPair("head", "tail"), nil)
	})
	require.PanicsWithValue(t, "los: pattern must not match empty input", func() {
		NewMatcher(NewPair("head", "tail"), NewPair("", "end"))
	})
	require.PanicsWithValue(t, "los: pattern must not match empty input", func() {
		NewMatcher(NewPair("head", "tail"), NewPair(`a*`, "end", WithRegexHead(REGEX_MODE_PERL)))
	})
	require.PanicsWithValue(t, "los: pattern must not match empty input", func() {
		NewMatcher(NewPair("head", "tail"), NewPair("other", ""))
	})
	require.PanicsWithValue(t, "los: pattern must not match empty input", func() {
		NewMatcher(NewPair("head", "tail"), NewPair("other", `\b`, WithRegexTail(REGEX_MODE_PERL)))
	})
}

func TestLos_AutomationMatcherCloseRequiresDrain(t *testing.T) {
	matcher := NewMatcher(
		NewPair(`foo`, `!`, WithRegexHead(REGEX_MODE_PERL)),
		NewPair("bar", `x+`, WithRegexTail(REGEX_MODE_PERL)),
	)
	require.Empty(t, slices.Collect(matcher.Match("fo")))
	require.ErrorIs(t, matcher.Close(), ErrBufferNotDrained)
}

func TestLos_AutomationMatcherLongPartitionedStream(t *testing.T) {
	prefix := strings.Repeat("ordinary ", 1100) +
		"<fixeD>not-a-match</fixed> <regex id=x>also-not-a-match</regex> "
	fixedBody := strings.Repeat("fixed payload ", 120)
	middle := strings.Repeat("between ", 90)
	regexBody := strings.Repeat("payload日本語 ", 300)
	suffix := strings.Repeat("trailing ", 200)
	input := prefix + "<fixed>" + fixedBody + "</fixed>" + middle +
		"<regex id=42>" + regexBody + "</regex>" + suffix
	require.Greater(t, len(strings.Fields(input)), 1024)

	matcher := NewMatcher(
		NewPair("<fixed>", "</fixed>").WithValue("fixed"),
		NewPair(`<([a-z]+) id=([0-9]+)>`, `</([a-z]+)>`, WithRegexHead(REGEX_MODE_PERL), WithRegexTail(REGEX_MODE_PERL)).WithValue("regex"),
	)
	t.Cleanup(func() { require.NoError(t, matcher.Close()) })

	var results []resultExpectation
	collect := func(sequence Results) {
		for result := range sequence {
			results = append(results, resultExpectation{
				state:   result.State(),
				text:    result.String(),
				matches: slices.Collect(result.Matches()),
				value:   result.Value(),
			})
		}
	}
	for offset := 0; offset < len(input); {
		end := min(offset+1+(offset%127), len(input))
		collect(matcher.Match(input[offset:end]))
		offset = end
	}
	collect(matcher.Finish())

	var reconstructed, unmatched, matchedBody strings.Builder
	var boundaries []resultExpectation
	for _, result := range results {
		reconstructed.WriteString(result.text)
		switch result.state {
		case STATE_NONE:
			unmatched.WriteString(result.text)
		case STATE_BODY:
			matchedBody.WriteString(result.text)
		default:
			boundaries = append(boundaries, result)
		}
	}
	require.Equal(t, input, reconstructed.String())
	require.Equal(t, prefix+middle+suffix, unmatched.String())
	require.Equal(t, fixedBody+regexBody, matchedBody.String())
	require.Equal(t, []resultExpectation{
		{state: STATE_HEAD, text: "<fixed>", matches: []string{"<fixed>"}, value: "fixed"},
		{state: STATE_TAIL, text: "</fixed>", matches: []string{"</fixed>"}, value: "fixed"},
		{state: STATE_HEAD, text: "<regex id=42>", matches: []string{"<regex id=42>", "regex", "42"}, value: "regex"},
		{state: STATE_TAIL, text: "</regex>", matches: []string{"</regex>", "regex"}, value: "regex"},
	}, boundaries)
}

func appendResultExpectations(dst []resultExpectation, results Results) []resultExpectation {
	for result := range results {
		next := resultExpectation{
			state:   result.State(),
			text:    result.String(),
			matches: slices.Collect(result.Matches()),
			value:   result.Value(),
		}
		if len(dst) > 0 && next.state%2 == 0 && dst[len(dst)-1].state == next.state && dst[len(dst)-1].value == next.value {
			dst[len(dst)-1].text += next.text
			dst[len(dst)-1].matches[0] += next.matches[0]
			continue
		}
		dst = append(dst, next)
	}
	return dst
}
