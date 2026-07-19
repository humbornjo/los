package legex_test

import (
	"regexp"
	"testing"

	"github.com/humbornjo/los/internal/legex"
	"github.com/stretchr/testify/require"
)

func TestLegex_MachineMatchesRegexp(t *testing.T) {
	tests := []struct {
		name  string
		expr  string
		input string
	}{
		{name: "literal", expr: `needle`, input: "hay needle stack"},
		{name: "alternation priority", expr: `a|ab`, input: "ab"},
		{name: "greedy repetition", expr: `a+`, input: "baaab"},
		{name: "non-greedy repetition", expr: `a+?`, input: "baaab"},
		{name: "nested captures", expr: `((a|b)+)(c)`, input: "xxabbczz"},
		{name: "optional capture", expr: `a(b)?c`, input: "ac"},
		{name: "counted repetition", expr: `[[:alpha:]]{2,4}[[:digit:]]{2}`, input: "--abcd42--"},
		{name: "unicode", expr: `(日本|中国)+`, input: "x日本中国y"},
		{name: "word boundary", expr: `\bword\b`, input: "a word!"},
		{name: "no word boundary", expr: `\Boo\B`, input: "foobar"},
		{name: "line anchors", expr: `(?m)^second$`, input: "first\nsecond\nthird"},
		{name: "text anchors", expr: `\Awhole text\z`, input: "whole text"},
		{name: "case folding", expr: `(?i:hello)`, input: "say HeLLo"},
		{name: "dot newline", expr: `(?s:a.*z)`, input: "a\n\nz"},
		{name: "empty match", expr: `a*`, input: "bbb"},
		{name: "nested empty capture", expr: `a*(|(b))c*`, input: "aacc"},
		{name: "adjacent greedy repetitions", expr: `(.*).*`, input: "ab"},
		{name: "repeated nested captures", expr: `((a|b|c)*(d))`, input: "abcd"},
		{name: "nested alternative priority", expr: `(?:A(?:A|a))`, input: "Aa"},
		{name: "empty alternative repetition", expr: `(|a)*`, input: "aa"},
		{name: "empty counted capture", expr: `(a){0}`, input: "x"},
		{name: "begin line excludes newline", expr: `(?-s)(?:(?:^).)`, input: "\n"},
		{name: "begin line with dot newline", expr: `(?s)(?:(?:^).)`, input: "\n"},
		{name: "ascii controls", expr: `\a\f\n\r\t\v`, input: "\a\f\n\r\t\v"},
		{name: "escaped punctuation", expr: `[\!\"\#\$\%\&\'\(\)\*\+\,\-\.\/\:\;\<\=\>\?\@\[\\\]\^\_\{\|\}\~]+`, input: `!"#$%&'()*+,-./:;<=>?@[\]^_{|}~`},
		{name: "invalid utf8 rune", expr: `\x{FFFD}`, input: string([]byte{0xff})},
		{name: "invalid utf8 sequence", expr: `\x{FFFD}`, input: string([]byte{0xc2, 0x00})},
		{name: "no match", expr: `xyz`, input: "abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			standard := regexp.MustCompile(tt.expr).FindStringSubmatchIndex(tt.input)
			re, err := legex.Compile(tt.expr)
			require.NoError(t, err)
			machine := re.Get()
			t.Cleanup(machine.Close)

			index, length, ok := machine.Finish([]byte(tt.input))
			if standard == nil {
				require.False(t, ok)
				return
			}

			require.True(t, ok)
			require.Equal(t, standard[0], index)
			require.Equal(t, standard[1]-standard[0], length)
			require.Equal(t, standard, machine.MatchCap())
		})
	}
}

func TestLegex_PosixMachineMatchesRegexp(t *testing.T) {
	tests := []struct {
		name  string
		expr  string
		input string
	}{
		{name: "longest alternative", expr: `a|ab`, input: "ab"},
		{name: "longest repetition", expr: `(a|aa)+`, input: "aaaa"},
		{name: "leftmost match", expr: `a+`, input: "baaabaaaa"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			standard := regexp.MustCompilePOSIX(tt.expr).FindStringSubmatchIndex(tt.input)
			re, err := legex.CompilePOSIX(tt.expr)
			require.NoError(t, err)
			machine := re.Get()
			t.Cleanup(machine.Close)

			index, length, ok := machine.Finish([]byte(tt.input))
			require.True(t, ok)
			require.Equal(t, standard[0], index)
			require.Equal(t, standard[1]-standard[0], length)
			require.Equal(t, standard, machine.MatchCap())
		})
	}
}

func TestLegex_MachineResumesAcrossChunks(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		chunks   []string
		expected []streamResult
		finish   *streamResult
		matchcap []int
	}{
		{
			name:   "literal split at every byte",
			expr:   `abc`,
			chunks: []string{"x", "a", "b", "c"},
			expected: []streamResult{
				{index: 1},
				{length: 1},
				{length: 2},
				{length: 3, ok: true},
			},
			matchcap: []int{0, 3},
		},
		{
			name:   "begin text anchor spans chunks",
			expr:   `^abc`,
			chunks: []string{"a", "b", "c"},
			expected: []streamResult{
				{length: 1},
				{length: 2},
				{length: 3, ok: true},
			},
			matchcap: []int{0, 3},
		},
		{
			name:   "begin text anchor is not restarted after discard",
			expr:   `^abc`,
			chunks: []string{"x", "abc"},
			expected: []streamResult{
				{index: 1},
				{index: 3},
			},
		},
		{
			name:   "line anchor remembers discarded newline",
			expr:   `(?m)^abc`,
			chunks: []string{"x\n", "ab", "c"},
			expected: []streamResult{
				{index: 2},
				{length: 2},
				{length: 3, ok: true},
			},
			matchcap: []int{0, 3},
		},
		{
			name:   "nested capture spans chunks",
			expr:   `ab(.*)c`,
			chunks: []string{"xxa", "b日", "本c"},
			expected: []streamResult{
				{index: 2, length: 1},
				{length: 5},
				{length: 9},
			},
			finish:   &streamResult{length: 9, ok: true},
			matchcap: []int{0, 9, 2, 8},
		},
		{
			name: "utf8 rune split between chunks",
			expr: `日本`,
			chunks: []string{
				string([]byte{0xe6}),
				string([]byte{0x97, 0xa5, 0xe6}),
				string([]byte{0x9c, 0xac}),
			},
			expected: []streamResult{
				{length: 1},
				{length: 4},
				{length: 6, ok: true},
			},
			matchcap: []int{0, 6},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			re, err := legex.Compile(tt.expr)
			require.NoError(t, err)
			machine := re.Get()
			t.Cleanup(machine.Close)

			var input []byte
			for i, chunk := range tt.chunks {
				input = append(input, chunk...)
				index, length, ok := machine.Match(input)
				require.Equal(t, tt.expected[i], streamResult{index: index, length: length, ok: ok})
				input = input[index:]
			}
			if tt.finish != nil {
				index, length, ok := machine.Finish(input)
				require.Equal(t, *tt.finish, streamResult{index: index, length: length, ok: ok})
			}
			if tt.matchcap != nil {
				require.Equal(t, tt.matchcap, machine.MatchCap())
			}
		})
	}
}

func TestLegex_MachineMatchesAcrossEveryByteBoundary(t *testing.T) {
	tests := []struct {
		name  string
		expr  string
		input string
	}{
		{name: "literal", expr: `abc`, input: "xxabc!"},
		{name: "first alternative", expr: `a|ab`, input: "ab"},
		{name: "longer preferred alternative", expr: `ab|a`, input: "ab"},
		{name: "greedy with terminator", expr: `a+z`, input: "xaaaaz!"},
		{name: "non-greedy", expr: `a+?`, input: "aaaa"},
		{name: "nested unicode capture", expr: `ab(.*)cZ`, input: "xxab日本cZ!"},
		{name: "optional capture", expr: `a(b)?c`, input: "xxac!"},
		{name: "word boundary", expr: `\bword\b`, input: "x word!"},
		{name: "no word boundary", expr: `\Boo\B`, input: "foobar"},
		{name: "line anchors", expr: `(?m)^abc$`, input: "x\nabc\n"},
		{name: "text anchors", expr: `\Aabc\z`, input: "abc"},
		{name: "invalid utf8", expr: `\x{FFFD}`, input: string([]byte{0xc2, 0x00})},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expected := regexp.MustCompile(tt.expr).FindStringSubmatchIndex(tt.input)
			require.NotNil(t, expected)

			re, err := legex.Compile(tt.expr)
			require.NoError(t, err)
			machine := re.Get()
			t.Cleanup(machine.Close)

			var retained []byte
			released := 0
			matched := false
			for _, b := range []byte(tt.input) {
				retained = append(retained, b)
				index, _, ok := machine.Match(retained)
				if ok {
					matched = true
					break
				}
				retained = retained[index:]
				released += index
			}
			if !matched {
				_, _, matched = machine.Finish(retained)
			}
			require.True(t, matched)

			actual := machine.MatchCap()
			for i, pos := range actual {
				if pos >= 0 {
					actual[i] = pos + released
				}
			}
			require.Equal(t, expected, actual)
		})
	}
}

func TestLegex_MachineDefersBoundaryDependentMatches(t *testing.T) {
	tests := []struct {
		name   string
		expr   string
		chunks []string
		final  streamResult
	}{
		{name: "greedy repetition", expr: `a+`, chunks: []string{"a", "aa"}, final: streamResult{length: 3, ok: true}},
		{name: "end text anchor", expr: `abc$`, chunks: []string{"abc"}, final: streamResult{length: 3, ok: true}},
		{name: "absolute end anchor", expr: `\Aabc\z`, chunks: []string{"ab", "c"}, final: streamResult{length: 3, ok: true}},
		{name: "word boundary", expr: `foo\b`, chunks: []string{"foo"}, final: streamResult{length: 3, ok: true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			re, err := legex.Compile(tt.expr)
			require.NoError(t, err)
			machine := re.Get()
			t.Cleanup(machine.Close)

			var retained []byte
			for _, chunk := range tt.chunks {
				retained = append(retained, chunk...)
				index, _, ok := machine.Match(retained)
				require.False(t, ok)
				retained = retained[index:]
			}
			index, length, ok := machine.Finish(retained)
			require.Equal(t, tt.final, streamResult{index: index, length: length, ok: ok})
		})
	}
}

type streamResult struct {
	index  int
	length int
	ok     bool
}
