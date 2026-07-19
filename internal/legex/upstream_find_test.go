// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package legex_test

import (
	"regexp"
	"slices"
	"testing"
	"unicode/utf8"

	"github.com/humbornjo/los/internal/legex"
	"github.com/stretchr/testify/require"
)

// upstreamFindTests mirrors every pattern and input in Go's regexp/find_test.go.
// The expected indices come from the public regexp implementation; LOS runs
// each search one byte at a time and preserves context between searches.
var upstreamFindTests = []struct {
	expr  string
	input string
}{
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
}

func TestLegex_UpstreamFindCasesStreaming(t *testing.T) {
	for i, tt := range upstreamFindTests {
		standard := regexp.MustCompile(tt.expr)
		re, err := legex.Compile(tt.expr)
		require.NoError(t, err, "case=%d expr=%q", i, tt.expr)
		expected := standard.FindAllStringSubmatchIndex(tt.input, -1)
		actual := streamAllSubmatchIndices(re, tt.input)
		require.True(t, slices.EqualFunc(expected, actual, slices.Equal),
			"case=%d expr=%q input=%q\nexpected=%v\nactual=%v", i, tt.expr, tt.input, expected, actual)
	}
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
