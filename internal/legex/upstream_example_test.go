// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package legex_test

import (
	"testing"

	"github.com/humbornjo/los/internal/legex"
	"github.com/stretchr/testify/require"
)

type upstreamExampleInput struct {
	source  string
	pattern string
	input   string
}

type upstreamExampleCase struct {
	upstreamExampleInput
	expected []upstreamMatchBlock
}

func withUpstreamExampleExpected(inputs []upstreamExampleInput) []upstreamExampleCase {
	cases := make([]upstreamExampleCase, 0, len(inputs))
	for _, input := range inputs {
		cases = append(cases, upstreamExampleCase{
			upstreamExampleInput: input,
			expected:             newUpstreamStreamCase(input.pattern, input.input).expected,
		})
	}
	return cases
}

var _UPSTREAM_EXAMPLE_CASES = withUpstreamExampleExpected([]upstreamExampleInput{
	{"Example", `^[a-z]+\[[0-9]+\]$`, "adam[23]"},
	{"Example", `^[a-z]+\[[0-9]+\]$`, "eve[7]"},
	{"Example", `^[a-z]+\[[0-9]+\]$`, "Job[48]"},
	{"Example", `^[a-z]+\[[0-9]+\]$`, "snakey"},
	{"ExampleMatch", `foo.*`, `seafood`},
	{"ExampleMatch", `bar.*`, `seafood`},
	{"ExampleMatchString", `foo.*`, `seafood`},
	{"ExampleMatchString", `bar.*`, `seafood`},
	{"ExampleRegexp_Find", `foo.?`, `seafood fool`},
	{"ExampleRegexp_FindAll", `foo.?`, `seafood fool`},
	{"ExampleRegexp_FindAllSubmatch", `foo(.?)`, `seafood fool`},
	{"ExampleRegexp_FindSubmatch", `foo(.?)`, `seafood fool`},
	{"ExampleRegexp_Match", `foo.?`, `seafood fool`},
	{"ExampleRegexp_Match", `foo.?`, `something else`},
	{"ExampleRegexp_FindString", `foo.?`, `seafood fool`},
	{"ExampleRegexp_FindString", `foo.?`, `meat`},
	{"ExampleRegexp_FindStringIndex", `ab?`, `tablett`},
	{"ExampleRegexp_FindStringIndex", `ab?`, `foo`},
	{"ExampleRegexp_FindStringSubmatch", `a(x*)b(y|z)c`, `-axxxbyc-`},
	{"ExampleRegexp_FindStringSubmatch", `a(x*)b(y|z)c`, `-abzc-`},
	{"ExampleRegexp_FindAllString", `a.`, `paranormal`},
	{"ExampleRegexp_FindAllString", `a.`, `paranormal`},
	{"ExampleRegexp_FindAllString", `a.`, `graal`},
	{"ExampleRegexp_FindAllString", `a.`, `none`},
	{"ExampleRegexp_FindAllStringSubmatch", `a(x*)b`, `-ab-`},
	{"ExampleRegexp_FindAllStringSubmatch", `a(x*)b`, `-axxb-`},
	{"ExampleRegexp_FindAllStringSubmatch", `a(x*)b`, `-ab-axb-`},
	{"ExampleRegexp_FindAllStringSubmatch", `a(x*)b`, `-axxb-ab-`},
	{"ExampleRegexp_FindAllStringSubmatchIndex", `a(x*)b`, `-ab-`},
	{"ExampleRegexp_FindAllStringSubmatchIndex", `a(x*)b`, `-axxb-`},
	{"ExampleRegexp_FindAllStringSubmatchIndex", `a(x*)b`, `-ab-axb-`},
	{"ExampleRegexp_FindAllStringSubmatchIndex", `a(x*)b`, `-axxb-ab-`},
	{"ExampleRegexp_FindAllStringSubmatchIndex", `a(x*)b`, `-foo-`},
	{"ExampleRegexp_FindSubmatchIndex", `a(x*)b`, `-ab-`},
	{"ExampleRegexp_FindSubmatchIndex", `a(x*)b`, `-axxb-`},
	{"ExampleRegexp_FindSubmatchIndex", `a(x*)b`, `-ab-axb-`},
	{"ExampleRegexp_FindSubmatchIndex", `a(x*)b`, `-axxb-ab-`},
	{"ExampleRegexp_FindSubmatchIndex", `a(x*)b`, `-foo-`},
	{"ExampleRegexp_MatchString", `(gopher){2}`, `gopher`},
	{"ExampleRegexp_MatchString", `(gopher){2}`, `gophergopher`},
	{"ExampleRegexp_MatchString", `(gopher){2}`, `gophergophergopher`},
	{"ExampleRegexp_ReplaceAll", `a(x*)b`, `-ab-axxb-`},
	{"ExampleRegexp_ReplaceAll", `a(?P<1W>x*)b`, `-ab-axxb-`},
	{"ExampleRegexp_ReplaceAllLiteralString", `a(x*)b`, `-ab-axxb-`},
	{"ExampleRegexp_ReplaceAllString", `a(x*)b`, `-ab-axxb-`},
	{"ExampleRegexp_ReplaceAllString", `a(?P<1W>x*)b`, `-ab-axxb-`},
	{"ExampleRegexp_ReplaceAllStringFunc", `[^aeiou]`, `seafood fool`},
	{"ExampleRegexp_SubexpNames", `(?P<first>[a-zA-Z]+) (?P<last>[a-zA-Z]+)`, `Alan Turing`},
	{"ExampleRegexp_SubexpIndex", `(?P<first>[a-zA-Z]+) (?P<last>[a-zA-Z]+)`, `Alan Turing`},
	{"ExampleRegexp_Split", `a`, `banana`},
	{"ExampleRegexp_Split", `z+`, `pizza`},
	{"ExampleRegexp_Expand", `(?m)(?P<key>\w+):\s+(?P<value>\w+)$`, upstreamExampleContent()},
	{"ExampleRegexp_ExpandString", `(?m)(?P<key>\w+):\s+(?P<value>\w+)$`, upstreamExampleContent()},
	{"ExampleRegexp_FindIndex", `(?m)(?P<key>\w+):\s+(?P<value>\w+)$`, upstreamExampleShortContent()},
	{"ExampleRegexp_FindAllSubmatchIndex", `(?m)(?P<key>\w+):\s+(?P<value>\w+)$`, upstreamExampleShortContent()},
	{"ExampleRegexp_FindAllIndex", `o.`, `London`},
})

func TestLegex_UpstreamExamplePatternsStreaming(t *testing.T) {
	for i, tt := range _UPSTREAM_EXAMPLE_CASES {
		re, err := legex.Compile(tt.pattern)
		require.NoError(t, err, "case=%d source=%s pattern=%q", i, tt.source, tt.pattern)
		require.Equal(t, tt.expected, streamAllMatchBlocks(re, tt.input),
			"case=%d source=%s pattern=%q input=%q", i, tt.source, tt.pattern, tt.input)
	}
}

func TestLegex_UpstreamExampleCompileAndMetadata(t *testing.T) {
	for _, source := range []string{"ExampleMatch", "ExampleMatchString"} {
		_, err := legex.Compile(`a(b`)
		require.ErrorContains(t, err, "missing closing )", source)
	}
	require.Equal(t, `Escaping symbols like: \.\+\*\?\(\)\|\[\]\{\}\^\$`,
		legex.QuoteMeta(`Escaping symbols like: .+*?()|[]{}^$`))

	re := legex.MustCompile(`(?P<first>[a-zA-Z]+) (?P<last>[a-zA-Z]+)`)
	require.Equal(t, []string{"", "first", "last"}, re.SubexpNames())
	require.Equal(t, 2, re.SubexpIndex("last"))
	require.Zero(t, legex.MustCompile(`a.`).NumSubexp())
	require.Equal(t, 4, legex.MustCompile(`(.*)((a)b)(.*)a`).NumSubexp())
}

func upstreamExampleContent() string {
	return `
	# comment line
	option1: value1
	option2: value2

	# another comment line
	option3: value3
`
}

func upstreamExampleShortContent() string {
	return `
	# comment line
	option1: value1
	option2: value2
`
}
