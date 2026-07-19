// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package legex_test

import (
	"regexp"
	"testing"
	"unicode/utf8"

	"github.com/humbornjo/los/internal/legex"
	"github.com/stretchr/testify/require"
)

type upstreamMatchBlock struct {
	block    string
	captures []string
}

type upstreamStreamCase struct {
	pattern  string
	input    string
	expected []upstreamMatchBlock
}

type upstreamPatternInput struct {
	pattern string
	input   string
}

func newUpstreamStreamCase(pattern, input string) upstreamStreamCase {
	standard := regexp.MustCompile(pattern)
	return upstreamStreamCase{
		pattern:  pattern,
		input:    input,
		expected: upstreamMatchBlocks(input, standard.FindAllStringSubmatchIndex(input, -1)),
	}
}

func newUpstreamStreamCases(cases []upstreamPatternInput) []upstreamStreamCase {
	streamCases := make([]upstreamStreamCase, 0, len(cases))
	for _, tt := range cases {
		streamCases = append(streamCases, newUpstreamStreamCase(tt.pattern, tt.input))
	}
	return streamCases
}

// _UPSTREAM_FIND_TESTS mirrors every pattern and input in Go's regexp/find_test.go.
// Expected match blocks and capture strings come from the public regexp
// implementation; LOS runs each search one byte at a time and preserves
// context between searches.
var _UPSTREAM_FIND_TESTS = newUpstreamStreamCases([]upstreamPatternInput{
	{``, ``},
	{`^abcdefg`, "abcdefg"},
	{`a+`, "baaab"},
	{`abcd..`, "abcdef"},
	{`a`, "a"},
	{`x`, "y"},
	{`b`, "abc"},
	{`.`, "a"},
	{`.*`, "abcdef"},
	{`^`, "abcde"},
	{`$`, "abcde"},
	{`^abcd$`, "abcd"},
	{`^bcd'`, "abcdef"},
	{`^abcd$`, "abcde"},
	{`a+`, "baaab"},
	{`a*`, "baaab"},
	{`[a-z]+`, "abcd"},
	{`[^a-z]+`, "ab1234cd"},
	{`[a\-\]z]+`, "az]-bcz"},
	{`[^\n]+`, "abcd\n"},
	{`[日本語]+`, "日本語日本語"},
	{`日本語+`, "日本語"},
	{`日本語+`, "日本語語語語"},
	{`()`, ""},
	{`(a)`, "a"},
	{`(.)(.)`, "日a"},
	{`(.*)`, ""},
	{`(.*)`, "abcd"},
	{`(..)(..)`, "abcd"},
	{`(([^xyz]*)(d))`, "abcd"},
	{`((a|b|c)*(d))`, "abcd"},
	{`(((a|b|c)*)(d))`, "abcd"},
	{`\a\f\n\r\t\v`, "\a\f\n\r\t\v"},
	{`[\a\f\n\r\t\v]+`, "\a\f\n\r\t\v"},
	{`a*(|(b))c*`, "aacc"},
	{`(.*).*`, "ab"},
	{`[.]`, "."},
	{`/$`, "/abc/"},
	{`/$`, "/abc"},
	{`.`, "abc"},
	{`(.)`, "abc"},
	{`.(.)`, "abcd"},
	{`ab*`, "abbaab"},
	{`a(b*)`, "abbaab"},
	{`ab$`, "cab"},
	{`axxb$`, "axxcb"},
	{`data`, "daXY data"},
	{`da(.)a$`, "daXY data"},
	{`zx+`, "zzx"},
	{`ab$`, "abcab"},
	{`(aa)*$`, "a"},
	{`(?:.|(?:.a))`, ""},
	{`(?:A(?:A|a))`, "Aa"},
	{`(?:A|(?:A|a))`, "a"},
	{`(a){0}`, ""},
	{`(?-s)(?:(?:^).)`, "\n"},
	{`(?s)(?:(?:^).)`, "\n"},
	{`(?:(?:^).)`, "\n"},
	{`\b`, "x"},
	{`\b`, "xx"},
	{`\b`, "x y"},
	{`\b`, "xx yy"},
	{`\B`, "x"},
	{`\B`, "xx"},
	{`\B`, "x y"},
	{`\B`, "xx yy"},
	{`(|a)*`, "aa"},
	{`0A|0[aA]`, "0a"},
	{`0[aA]|0A`, "0a"},
	{`[^\S\s]`, "abcd"},
	{`[^\S[:space:]]`, "abcd"},
	{`[^\D\d]`, "abcd"},
	{`[^\D[:digit:]]`, "abcd"},
	{`(?i)\W`, "x"},
	{`(?i)\W`, "k"},
	{`(?i)\W`, "s"},
	{`\!\"\#\$\%\&\'\(\)\*\+\,\-\.\/\:\;\<\=\>\?\@\[\\\]\^\_\{\|\}\~`, `!"#$%&'()*+,-./:;<=>?@[\]^_{|}~`},
	{`[\!\"\#\$\%\&\'\(\)\*\+\,\-\.\/\:\;\<\=\>\?\@\[\\\]\^\_\{\|\}\~]+`, `!"#$%&'()*+,-./:;<=>?@[\]^_{|}~`},
	{"\\`", "`"},
	{"[\\`]+", "`"},
	{"\ufffd", "\xff"},
	{"\ufffd", "hello\xffworld"},
	{`.*`, "hello\xffworld"},
	{`\x{fffd}`, "\xc2\x00"},
	{"[\ufffd]", "\xff"},
	{`[\x{fffd}]`, "\xc2\x00"},
	{`.`, "qwertyuiopasdfghjklzxcvbnm1234567890"},
})

func TestLegex_UpstreamFindCasesStreaming(t *testing.T) {
	for i, tt := range _UPSTREAM_FIND_TESTS {
		assertUpstreamStreamCase(t, i, tt)
	}
}

func assertUpstreamStreamCase(t *testing.T, index int, tt upstreamStreamCase) {
	t.Helper()
	re, err := legex.Compile(tt.pattern)
	require.NoError(t, err, "case=%d pattern=%q", index, tt.pattern)
	require.Equal(t, tt.expected, streamAllMatchBlocks(re, tt.input),
		"case=%d pattern=%q input=%q", index, tt.pattern, tt.input)
}

func streamAllMatchBlocks(re *legex.Regexp, input string) []upstreamMatchBlock {
	return upstreamMatchBlocks(input, streamAllSubmatchIndices(re, input))
}

func streamFirstMatchBlocks(re *legex.Regexp, input string) []upstreamMatchBlock {
	return upstreamSingleMatchBlocks(input, streamFirstSubmatchIndex(re, input, 0))
}

func upstreamSingleMatchBlocks(input string, match []int) []upstreamMatchBlock {
	if match == nil {
		return nil
	}
	return upstreamMatchBlocks(input, [][]int{match})
}

func upstreamMatchBlocks(input string, matches [][]int) []upstreamMatchBlock {
	if matches == nil {
		return nil
	}
	blocks := make([]upstreamMatchBlock, 0, len(matches))
	for _, match := range matches {
		block := upstreamMatchBlock{block: input[match[0]:match[1]]}
		for i := 2; i < len(match); i += 2 {
			capture := ""
			if match[i] >= 0 && match[i+1] >= 0 {
				capture = input[match[i]:match[i+1]]
			}
			block.captures = append(block.captures, capture)
		}
		blocks = append(blocks, block)
	}
	return blocks
}

func streamAllSubmatchIndices(re *legex.Regexp, input string) [][]int {
	var matches [][]int
	pos, previousEnd := 0, -1
	for pos <= len(input) {
		match := streamFirstSubmatchIndex(re, input, pos)
		if match == nil {
			break
		}

		accept := true
		if match[1] == pos {
			if match[0] == previousEnd {
				accept = false
			}
			_, width := utf8.DecodeRuneInString(input[pos:])
			if width > 0 {
				pos += width
			} else {
				pos = len(input) + 1
			}
		} else {
			pos = match[1]
		}
		previousEnd = match[1]
		if accept {
			matches = append(matches, match)
		}
	}
	return matches
}

func streamFirstSubmatchIndex(re *legex.Regexp, input string, start int) []int {
	machine := re.Get()
	defer machine.Close()
	ctx := legex.NewStreamContext().Advance([]byte(input[:start]))
	retained := make([]byte, 0, len(input)-start)
	released := start

	for i := start; i < len(input); i++ {
		retained = append(retained, input[i])
		index, _, ok := machine.Match(ctx, retained)
		if ok {
			return globalCaptures(machine.MatchCap(), released)
		}
		ctx = ctx.Advance(retained[:index])
		retained = retained[index:]
		released += index
	}
	if _, _, ok := machine.Finish(ctx, retained); !ok {
		return nil
	}
	return globalCaptures(machine.MatchCap(), released)
}
