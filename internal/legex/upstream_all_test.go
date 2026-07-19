// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package legex_test

import (
	"reflect"
	"regexp/syntax"
	"slices"
	"strings"
	"testing"

	"github.com/humbornjo/los/internal/legex"
	"github.com/stretchr/testify/require"
)

var _UPSTREAM_GOOD_RE = []string{
	``,
	`.`,
	`^.$`,
	`a`,
	`a*`,
	`a+`,
	`a?`,
	`a|b`,
	`a*|b*`,
	`(a*|b)(c*|d)`,
	`[a-z]`,
	`[a-abc-c\-\]\[]`,
	`[a-z]+`,
	`[abc]`,
	`[^1234]`,
	`[^\n]`,
	`\!\\`,
}

var _UPSTREAM_BAD_RE = []struct {
	expr string
	err  string
}{
	{`*`, "missing argument to repetition operator: `*`"},
	{`+`, "missing argument to repetition operator: `+`"},
	{`?`, "missing argument to repetition operator: `?`"},
	{`(abc`, "missing closing ): `(abc`"},
	{`abc)`, "unexpected ): `abc)`"},
	{`x[a-z`, "missing closing ]: `[a-z`"},
	{`[z-a]`, "invalid character class range: `z-a`"},
	{`abc\`, "trailing backslash at end of expression"},
	{`a**`, "invalid nested repetition operator: `**`"},
	{`a*+`, "invalid nested repetition operator: `*+`"},
	{`\x`, "invalid escape sequence: `\\x`"},
	{strings.Repeat(`\pL`, 27000), "expression too large"},
}

var _UPSTREAM_META_TESTS = []struct {
	pattern, quoted, literal string
	complete                 bool
}{
	{``, ``, ``, true},
	{`foo`, `foo`, `foo`, true},
	{`日本語+`, `日本語\+`, `日本語`, false},
	{`foo\.\$`, `foo\\\.\\\$`, `foo.$`, true},
	{`foo.\$`, `foo\.\\\$`, `foo`, false},
	{`!@#$%^&*()_+-=[{]}\|,<.>/?~`, `!@#\$%\^&\*\(\)_\+-=\[\{\]\}\\\|,<\.>/\?~`, `!@#`, false},
}

var _UPSTREAM_LITERAL_PREFIX_TESTS = []struct {
	pattern, literal string
	complete         bool
}{
	{`^0^0$`, `0`, false},
	{`^0^`, ``, false},
	{`^0$`, `0`, true},
	{`$0^`, ``, false},
	{`$0$`, ``, false},
	{`^^0$$`, ``, false},
	{`^$^$`, ``, false},
	{`$$0^^`, ``, false},
	{`a\x{fffd}b`, `a`, false},
	{`\x{fffd}b`, ``, false},
	{"\ufffd", ``, false},
}

var _UPSTREAM_SUBEXP_CASES = []struct {
	expr    string
	num     int
	names   []string
	indices map[string]int
}{
	{``, 0, nil, map[string]int{"": -1, "missing": -1}},
	{`.*`, 0, nil, map[string]int{"": -1, "missing": -1}},
	{`abba`, 0, nil, map[string]int{"": -1, "missing": -1}},
	{`ab(b)a`, 1, []string{"", ""}, map[string]int{"": -1, "missing": -1}},
	{`ab(.*)a`, 1, []string{"", ""}, map[string]int{"": -1, "missing": -1}},
	{`(.*)ab(.*)a`, 2, []string{"", "", ""}, map[string]int{"": -1, "missing": -1}},
	{`(.*)(ab)(.*)a`, 3, []string{"", "", "", ""}, map[string]int{"": -1, "missing": -1}},
	{`(.*)((a)b)(.*)a`, 4, []string{"", "", "", "", ""}, map[string]int{"": -1, "missing": -1}},
	{`(.*)(\(ab)(.*)a`, 3, []string{"", "", "", ""}, map[string]int{"": -1, "missing": -1}},
	{`(.*)(\(a\)b)(.*)a`, 3, []string{"", "", "", ""}, map[string]int{"": -1, "missing": -1}},
	{
		`(?P<foo>.*)(?P<bar>(a)b)(?P<foo>.*)a`,
		4,
		[]string{"", "foo", "bar", "", "foo"},
		map[string]int{"": -1, "missing": -1, "foo": 1, "bar": 2},
	},
}

// _UPSTREAM_REPLACE_CASES projects every row from replaceTests,
// replaceLiteralTests, and replaceFuncTests onto the matching operation that
// a streaming regex engine exposes. Duplicate pattern/input pairs are retained
// so the projection remains auditable against the upstream tables.
var _UPSTREAM_REPLACE_CASES = newUpstreamStreamCases([]upstreamPatternInput{
	{``, ``},
	{``, ``},
	{``, `abc`},
	{``, `abc`},
	{`b`, ``},
	{`b`, ``},
	{`b`, `abc`},
	{`b`, `abc`},
	{`y`, ``},
	{`y`, ``},
	{`y`, `abc`},
	{`y`, `abc`},
	{`[a-c]*`, "\u65e5"},
	{`[^日]`, "abc\u65e5def"},
	{`^[a-c]*`, `abcdabc`},
	{`[a-c]*$`, `abcdabc`},
	{`^[a-c]*$`, `abcdabc`},
	{`^[a-c]*`, `abc`},
	{`[a-c]*$`, `abc`},
	{`^[a-c]*$`, `abc`},
	{`^[a-c]*`, `dabce`},
	{`[a-c]*$`, `dabce`},
	{`^[a-c]*$`, `dabce`},
	{`^[a-c]*`, ``},
	{`[a-c]*$`, ``},
	{`^[a-c]*$`, ``},
	{`^[a-c]+`, `abcdabc`},
	{`[a-c]+$`, `abcdabc`},
	{`^[a-c]+$`, `abcdabc`},
	{`^[a-c]+`, `abc`},
	{`[a-c]+$`, `abc`},
	{`^[a-c]+$`, `abc`},
	{`^[a-c]+`, `dabce`},
	{`[a-c]+$`, `dabce`},
	{`^[a-c]+$`, `dabce`},
	{`^[a-c]+`, ``},
	{`[a-c]+$`, ``},
	{`^[a-c]+$`, ``},
	{`abc`, `abcdefg`},
	{`bc`, `abcbcdcdedef`},
	{`abc`, `abcdabc`},
	{`x`, `xxxXxxx`},
	{`abc`, ``},
	{`abc`, `abc`},
	{`.+`, `abc`},
	{`[a-c]*`, `def`},
	{`[a-c]+`, `abcbcdcdedef`},
	{`[a-c]*`, `abcbcdcdedef`},
	{`a+`, `banana`},
	{`a+`, `banana`},
	{`a+`, `banana`},
	{`a+`, `banana`},
	{`hello, (.+)`, `hello, world`},
	{`hello, (.+)`, `hello, world`},
	{`hello, (.+)`, `hello, world`},
	{`hello, (.+)`, `hello, world`},
	{`hello, (?P<noun>.+)`, `hello, world`},
	{`hello, (?P<noun>.+)`, `hello, world`},
	{`(?P<x>hi)|(?P<x>bye)`, `hi`},
	{`(?P<x>hi)|(?P<x>bye)`, `bye`},
	{`(?P<x>hi)|(?P<x>bye)`, `hi`},
	{`(?P<x>hi)|(?P<x>bye)`, `hi`},
	{`(?P<x>hi)|(?P<x>bye)`, `hi`},
	{`a+`, `aaa`},
	{`a+`, `aaa`},
	{`a+`, `aaa`},
	{`(x)?`, `123`},
	{`abc`, `123`},
	{`(a)(b){0}(c)`, `xacxacx`},
	{`(a)(((b))){0}c`, `xacxacx`},
	{`((a(b){0}){3}){5}(h)`, `say aaaaaaaaaaaaaaaah`},
	{`((a(b){0}){3}){5}h`, `say aaaaaaaaaaaaaaaah`},

	{`a+`, `banana`},
	{`a+`, `banana`},
	{`a+`, `banana`},
	{`a+`, `banana`},
	{`hello, (.+)`, `hello, world`},
	{`hello, (?P<noun>.+)`, `hello, world`},
	{`hello, (?P<noun>.+)`, `hello, world`},
	{`(?P<x>hi)|(?P<x>bye)`, `hi`},
	{`(?P<x>hi)|(?P<x>bye)`, `bye`},
	{`(?P<x>hi)|(?P<x>bye)`, `hi`},
	{`(?P<x>hi)|(?P<x>bye)`, `hi`},
	{`(?P<x>hi)|(?P<x>bye)`, `hi`},
	{`a+`, `aaa`},
	{`a+`, `aaa`},
	{`a+`, `aaa`},

	{`[a-c]`, `defabcdef`},
	{`[a-c]+`, `defabcdef`},
	{`[a-c]*`, `defabcdef`},
})

var _UPSTREAM_SPLIT_CASES = newUpstreamStreamCases([]upstreamPatternInput{
	{`:`, `foo:and:bar`},
	{`:`, `foo:and:bar`},
	{`:`, `foo:and:bar`},
	{`foo`, `foo:and:bar`},
	{`bar`, `foo:and:bar`},
	{`baz`, `foo:and:bar`},
	{`a`, `baabaab`},
	{`a*`, `baabaab`},
	{`ba*`, `baabaab`},
	{`f*b*`, `foobar`},
	{`f+.*b+`, `foobar`},
	{`o{2}`, `foobooboar`},
	{`,`, `a,b,c,d,e,f`},
	{`,`, `a,b,c,d,e,f`},
	{`,`, `,`},
	{`,`, `,,,`},
	{`,`, ``},
	{`.*`, ``},
	{`.+`, ``},
	{``, ``},
	{``, `foobar`},
	{`a*`, `abaabaccadaaae`},
	{`:`, `:x:y:z:`},
})

var _UPSTREAM_MIN_INPUT_LEN_CASES = []struct {
	pattern string
	input   string
}{
	{``, ``},
	{`a`, `a`},
	{`aa`, `aa`},
	{`(aa)a`, `aaa`},
	{`(?:aa)a`, `aaa`},
	{`a?a`, `a`},
	{`(aaa)|(aa)`, `aa`},
	{`(aa)+a`, `aaa`},
	{`(aa)*a`, `a`},
	{`(aa){3,5}`, `aaaaaa`},
	{`[a-z]`, `a`},
	{`日`, `日`},
}

func TestLegex_UpstreamCompileTables(t *testing.T) {
	for i, expr := range _UPSTREAM_GOOD_RE {
		_, err := legex.Compile(expr)
		require.NoError(t, err, "case=%d expr=%q", i, expr)
		assertUpstreamStreamCase(t, i, newUpstreamStreamCase(expr, "abc"+expr+"def"))
	}
	for i, tt := range _UPSTREAM_BAD_RE {
		_, err := legex.Compile(tt.expr)
		require.ErrorContains(t, err, tt.err, "case=%d expr=%q", i, tt.expr)
	}
}

func TestLegex_UpstreamInlineTableCounts(t *testing.T) {
	require.Len(t, _UPSTREAM_GOOD_RE, 17)
	require.Len(t, _UPSTREAM_BAD_RE, 12)
	require.Len(t, _UPSTREAM_FIND_TESTS, 87)
	require.Len(t, _UPSTREAM_REPLACE_CASES, 90)
	require.Len(t, _UPSTREAM_SPLIT_CASES, 23)
	require.Len(t, _UPSTREAM_META_TESTS, 6)
	require.Len(t, _UPSTREAM_LITERAL_PREFIX_TESTS, 11)
	require.Len(t, _UPSTREAM_SUBEXP_CASES, 11)
	require.Len(t, _UPSTREAM_MIN_INPUT_LEN_CASES, 12)
	require.Len(t, _UPSTREAM_ONE_PASS_TESTS, 37)
	require.Len(t, _UPSTREAM_EXAMPLE_CASES, 56)
}

func TestLegex_UpstreamAllSemanticTablesStreaming(t *testing.T) {
	cases := slices.Concat(_UPSTREAM_REPLACE_CASES, _UPSTREAM_SPLIT_CASES)
	for _, tt := range _UPSTREAM_MIN_INPUT_LEN_CASES {
		cases = append(cases, newUpstreamStreamCase(tt.pattern, tt.input))
	}
	cases = append(cases,
		newUpstreamStreamCase(`^x{1,1000}y{1,1000}$`, strings.Repeat("x", 1000)+strings.Repeat("y", 1000)),
		newUpstreamStreamCase(`a|b`, string(make([]byte, 256))),
		newUpstreamStreamCase(`a|b`, "\x00"),
		newUpstreamStreamCase(`a.*b.*c.*d`, `abcdefghijklmn`),
	)

	for i, tt := range cases {
		assertUpstreamStreamCase(t, i, tt)
	}
}

func TestLegex_UpstreamQuoteMetaAndLiteralPrefix(t *testing.T) {
	for i, tt := range _UPSTREAM_META_TESTS {
		quoted := legex.QuoteMeta(tt.pattern)
		require.Equal(t, tt.quoted, quoted, "quote case=%d pattern=%q", i, tt.pattern)
		if tt.pattern != "" {
			re := legex.MustCompile(quoted)
			input := "abc" + tt.pattern + "def"
			expected := []upstreamMatchBlock{{block: tt.pattern}}
			require.Equal(t, expected, streamFirstMatchBlocks(re, input),
				"stream quote case=%d pattern=%q", i, tt.pattern)
		}

		re := legex.MustCompile(tt.pattern)
		literal, complete := re.LiteralPrefix()
		require.Equal(t, tt.literal, literal, "prefix case=%d pattern=%q", i, tt.pattern)
		require.Equal(t, tt.complete, complete, "prefix case=%d pattern=%q", i, tt.pattern)
	}

	for i, tt := range _UPSTREAM_LITERAL_PREFIX_TESTS {
		re := legex.MustCompile(tt.pattern)
		literal, complete := re.LiteralPrefix()
		require.Equal(t, tt.literal, literal, "case=%d pattern=%q", i, tt.pattern)
		require.Equal(t, tt.complete, complete, "case=%d pattern=%q", i, tt.pattern)
	}
}

func TestLegex_UpstreamSubexpMetadata(t *testing.T) {
	for i, tt := range _UPSTREAM_SUBEXP_CASES {
		re := legex.MustCompile(tt.expr)
		require.Equal(t, tt.num, re.NumSubexp(), "case=%d expr=%q", i, tt.expr)
		if tt.names != nil {
			require.Equal(t, tt.names, re.SubexpNames(), "case=%d expr=%q", i, tt.expr)
		} else {
			require.Len(t, re.SubexpNames(), tt.num+1, "case=%d expr=%q", i, tt.expr)
		}
		for name, index := range tt.indices {
			require.Equal(t, index, re.SubexpIndex(name), "case=%d expr=%q name=%q", i, tt.expr, name)
		}
	}
}

func TestLegex_UpstreamParseAndCompileStreaming(t *testing.T) {
	for i, tt := range []struct {
		flags syntax.Flags
		match bool
	}{
		{syntax.Perl | syntax.OneLine, false},
		{syntax.Perl &^ syntax.OneLine, true},
	} {
		parsed, err := syntax.Parse("a$", tt.flags)
		require.NoError(t, err, "case=%d", i)
		re := legex.MustCompile(parsed.String())
		var expected []upstreamMatchBlock
		if tt.match {
			expected = []upstreamMatchBlock{{block: "a"}}
		}
		require.Equal(t, expected, streamFirstMatchBlocks(re, "a\nb"), "case=%d", i)
	}
}

func TestLegex_UpstreamDeepEqual(t *testing.T) {
	re1 := legex.MustCompile("a.*b.*c.*d")
	re2 := legex.MustCompile("a.*b.*c.*d")
	require.True(t, reflect.DeepEqual(re1, re2))

	expected := []upstreamMatchBlock{{block: "abcd"}}
	require.Equal(t, expected, streamFirstMatchBlocks(re1, "abcdefghijklmn"))
	require.True(t, reflect.DeepEqual(re1, re2))
}

func TestLegex_UpstreamUnmarshalText(t *testing.T) {
	unmarshaled := new(legex.Regexp)
	for i, expr := range _UPSTREAM_GOOD_RE {
		re := legex.MustCompile(expr)
		marshaled, err := re.MarshalText()
		require.NoError(t, err, "case=%d expr=%q", i, expr)
		require.NoError(t, unmarshaled.UnmarshalText(marshaled), "case=%d expr=%q", i, expr)
		require.Equal(t, expr, unmarshaled.String(), "case=%d expr=%q", i, expr)
		input := "abc" + expr + "def"
		require.Equal(t, newUpstreamStreamCase(expr, input).expected, streamAllMatchBlocks(unmarshaled, input),
			"stream case=%d expr=%q", i, expr)

		buf := make([]byte, 4, 32)
		appended, err := re.AppendText(buf)
		require.NoError(t, err, "case=%d expr=%q", i, expr)
		require.NoError(t, unmarshaled.UnmarshalText(appended[4:]), "case=%d expr=%q", i, expr)
		require.Equal(t, expr, unmarshaled.String(), "case=%d expr=%q", i, expr)
	}
}
