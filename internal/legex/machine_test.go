package legex_test

import (
	"fmt"
	"testing"

	"github.com/humbornjo/los/internal/legex"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// var findTests = []FindTest{
// 	{`^abcd$`, "abcd", build(1, 0, 4)},
// 	{`^bcd'`, "abcdef", nil},
// 	{`^abcd$`, "abcde", nil},
// 	{`a+`, "baaab", build(1, 1, 4)},
// 	{`a*`, "baaab", build(3, 0, 0, 1, 4, 5, 5)},
// 	{`[a-z]+`, "abcd", build(1, 0, 4)},
// 	{`[^a-z]+`, "ab1234cd", build(1, 2, 6)},
// 	{`[a\-\]z]+`, "az]-bcz", build(2, 0, 4, 6, 7)},
// 	{`[^\n]+`, "abcd\n", build(1, 0, 4)},
// 	{`[日本語]+`, "日本語日本語", build(1, 0, 18)},
// 	{`日本語+`, "日本語", build(1, 0, 9)},
// 	{`日本語+`, "日本語語語語", build(1, 0, 18)},
// 	{`()`, "", build(1, 0, 0, 0, 0)},
// 	{`(a)`, "a", build(1, 0, 1, 0, 1)},
// 	{`(.)(.)`, "日a", build(1, 0, 4, 0, 3, 3, 4)},
// 	{`(.*)`, "", build(1, 0, 0, 0, 0)},
// 	{`(.*)`, "abcd", build(1, 0, 4, 0, 4)},
// 	{`(..)(..)`, "abcd", build(1, 0, 4, 0, 2, 2, 4)},
// 	{`(([^xyz]*)(d))`, "abcd", build(1, 0, 4, 0, 4, 0, 3, 3, 4)},
// 	{`((a|b|c)*(d))`, "abcd", build(1, 0, 4, 0, 4, 2, 3, 3, 4)},
// 	{`(((a|b|c)*)(d))`, "abcd", build(1, 0, 4, 0, 4, 0, 3, 2, 3, 3, 4)},
// 	{`\a\f\n\r\t\v`, "\a\f\n\r\t\v", build(1, 0, 6)},
// 	{`[\a\f\n\r\t\v]+`, "\a\f\n\r\t\v", build(1, 0, 6)},
//
// 	{`a*(|(b))c*`, "aacc", build(1, 0, 4, 2, 2, -1, -1)},
// 	{`(.*).*`, "ab", build(1, 0, 2, 0, 2)},
// 	{`[.]`, ".", build(1, 0, 1)},
// 	{`/$`, "/abc/", build(1, 4, 5)},
// 	{`/$`, "/abc", nil},
//
// 	// multiple matches
// 	{`.`, "abc", build(3, 0, 1, 1, 2, 2, 3)},
// 	{`(.)`, "abc", build(3, 0, 1, 0, 1, 1, 2, 1, 2, 2, 3, 2, 3)},
// 	{`.(.)`, "abcd", build(2, 0, 2, 1, 2, 2, 4, 3, 4)},
// 	{`ab*`, "abbaab", build(3, 0, 3, 3, 4, 4, 6)},
// 	{`a(b*)`, "abbaab", build(3, 0, 3, 1, 3, 3, 4, 4, 4, 4, 6, 5, 6)},
//
// 	// fixed bugs
// 	{`ab$`, "cab", build(1, 1, 3)},
// 	{`axxb$`, "axxcb", nil},
// 	{`data`, "daXY data", build(1, 5, 9)},
// 	{`da(.)a$`, "daXY data", build(1, 5, 9, 7, 8)},
// 	{`zx+`, "zzx", build(1, 1, 3)},
// 	{`ab$`, "abcab", build(1, 3, 5)},
// 	{`(aa)*$`, "a", build(1, 1, 1, -1, -1)},
// 	{`(?:.|(?:.a))`, "", nil},
// 	{`(?:A(?:A|a))`, "Aa", build(1, 0, 2)},
// 	{`(?:A|(?:A|a))`, "a", build(1, 0, 1)},
// 	{`(a){0}`, "", build(1, 0, 0, -1, -1)},
// 	{`(?-s)(?:(?:^).)`, "\n", nil},
// 	{`(?s)(?:(?:^).)`, "\n", build(1, 0, 1)},
// 	{`(?:(?:^).)`, "\n", nil},
// 	{`\b`, "x", build(2, 0, 0, 1, 1)},
// 	{`\b`, "xx", build(2, 0, 0, 2, 2)},
// 	{`\b`, "x y", build(4, 0, 0, 1, 1, 2, 2, 3, 3)},
// 	{`\b`, "xx yy", build(4, 0, 0, 2, 2, 3, 3, 5, 5)},
// 	{`\B`, "x", nil},
// 	{`\B`, "xx", build(1, 1, 1)},
// 	{`\B`, "x y", nil},
// 	{`\B`, "xx yy", build(2, 1, 1, 4, 4)},
// 	{`(|a)*`, "aa", build(3, 0, 0, 0, 0, 1, 1, 1, 1, 2, 2, 2, 2)},
//
// 	// RE2 tests
// 	{`[^\S\s]`, "abcd", nil},
// 	{`[^\S[:space:]]`, "abcd", nil},
// 	{`[^\D\d]`, "abcd", nil},
// 	{`[^\D[:digit:]]`, "abcd", nil},
// 	{`(?i)\W`, "x", nil},
// 	{`(?i)\W`, "k", nil},
// 	{`(?i)\W`, "s", nil},
//
// 	// can backslash-escape any punctuation
// 	{`\!\"\#\$\%\&\'\(\)\*\+\,\-\.\/\:\;\<\=\>\?\@\[\\\]\^\_\{\|\}\~`,
// 		`!"#$%&'()*+,-./:;<=>?@[\]^_{|}~`, build(1, 0, 31)},
// 	{`[\!\"\#\$\%\&\'\(\)\*\+\,\-\.\/\:\;\<\=\>\?\@\[\\\]\^\_\{\|\}\~]+`,
// 		`!"#$%&'()*+,-./:;<=>?@[\]^_{|}~`, build(1, 0, 31)},
// 	{"\\`", "`", build(1, 0, 1)},
// 	{"[\\`]+", "`", build(1, 0, 1)},
//
// 	{"\ufffd", "\xff", build(1, 0, 1)},
// 	{"\ufffd", "hello\xffworld", build(1, 5, 6)},
// 	{`.*`, "hello\xffworld", build(1, 0, 11)},
// 	{`\x{fffd}`, "\xc2\x00", build(1, 0, 1)},
// 	{"[\ufffd]", "\xff", build(1, 0, 1)},
// 	{`[\x{fffd}]`, "\xc2\x00", build(1, 0, 1)},
//
// 	// long set of matches (longer than startSize)
// 	{
// 		".",
// 		"qwertyuiopasdfghjklzxcvbnm1234567890",
// 		build(36, 0, 1, 1, 2, 2, 3, 3, 4, 4, 5, 5, 6, 6, 7, 7, 8, 8, 9, 9, 10,
// 			10, 11, 11, 12, 12, 13, 13, 14, 14, 15, 15, 16, 16, 17, 17, 18, 18, 19, 19, 20,
// 			20, 21, 21, 22, 22, 23, 23, 24, 24, 25, 25, 26, 26, 27, 27, 28, 28, 29, 29, 30,
// 			30, 31, 31, 32, 32, 33, 33, 34, 34, 35, 35, 36),
// 	},
// }

var testCases = []testCase{
	{
		name:   "empty regex with empty input - should match at start",
		expr:   "",
		inputs: []string{"-"},
		expected: []expectedResult{
			{0, 0, true}, // Empty regex should match empty string at position 0, length 0
		},
	},
	{
		name:   "anchored pattern ^abcdefg matches exactly",
		expr:   "^abcdefg",
		inputs: []string{"abcdefg"},
		expected: []expectedResult{
			{0, 7, true}, // ^abcdefg should match "abcdefg" at position 0, length 7
		},
	},
	{
		name:   "a+ pattern matches consecutive a's in baaab",
		expr:   "a+",
		inputs: []string{"baaab"},
		expected: []expectedResult{
			{1, 3, true}, // a+ should match "aaab" starting at position 1, length 4
		},
	},
	{
		name:   "abcd.. pattern matches abcdef with any 2 chars",
		expr:   "abcd..",
		inputs: []string{"abcdef"},
		expected: []expectedResult{
			{0, 6, true}, // abcd.. should match "abcdef" at position 0, length 6 (.. matches ef)
		},
	},
	{
		name:   "simple 'a' pattern matches single 'a'",
		expr:   "a",
		inputs: []string{"a"},
		expected: []expectedResult{
			{0, 1, true}, // a should match "a" at position 0, length 1
		},
	},
	{
		name:   "'x' pattern does not match 'y' input",
		expr:   "x",
		inputs: []string{"y"},
		expected: []expectedResult{
			{1, 0, false}, // x should not match "y", advance by 1 with no match
		},
	},
	{
		name:   "'b' pattern matches 'b' in 'abc' at position 1",
		expr:   "b",
		inputs: []string{"abc"},
		expected: []expectedResult{
			{1, 1, true}, // b should match "b" in "abc" at position 1, length 1
		},
	},
	{
		name:   ". pattern matches any single character 'a'",
		expr:   ".",
		inputs: []string{"a"},
		expected: []expectedResult{
			{0, 1, true}, // . should match "a" at position 0, length 1
		},
	},
	{
		name:   ".* pattern matches entire string abcdef",
		expr:   ".*",
		inputs: []string{"abcdef"},
		expected: []expectedResult{
			{0, 6, true}, // .* should match "abcdef" at position 0, length 6
		},
	},
	{
		name:   "^ pattern matches start of string abcde",
		expr:   "^",
		inputs: []string{"abcde"},
		expected: []expectedResult{
			{0, 0, true}, // ^ should match start of "abcde" at position 0, length 0
		},
	},
	{
		name:   "$ pattern matches end of string abcde",
		expr:   "$",
		inputs: []string{"abcde"},
		expected: []expectedResult{
			{5, 0, true}, // $ should match end of "abcde" at position 5, length 0
		},
	},
}

func TestMachine_Std(t *testing.T) {
	performTest(t, testCases)
}

func TestMachine_Default(t *testing.T) {
	t.Run("Base test", func(t *testing.T) {
		testCases := []testCase{
			{
				name:   "anchored simple pattern >> partial match end >> match start",
				expr:   "abc",
				inputs: []string{"aaa", "bcd"},
				expected: []expectedResult{
					{2, 1, false}, // "aaa" - partial match "a"
					{0, 3, true},  // "abcd" - should match "abc"
				},
			},
			{
				name:   "anchored simple pattern >> partial match >> match start",
				expr:   "abc",
				inputs: []string{"ab", "cdef"},
				expected: []expectedResult{
					{0, 2, false}, // "ab" - partial, no match
					{0, 3, true},  // "abcdef" - should match "abc"
				},
			},
			{
				name:   "simple pattern >> match end >> no match",
				expr:   "abc",
				inputs: []string{"xababc", "def"},
				expected: []expectedResult{
					{3, 3, true},  // "xabc" - match "abc" starting at index 3
					{3, 0, false}, // "def" - no match, adcance all
				},
			},
			{
				name: "simple pattern >> multi match",
				expr: "error|warn|info",
				inputs: []string{
					"where there is a info",
					"there is a warning",
					"when there is a warning",
					"you dont give a fuck",
					"and suddenly an error come up",
					"warned you had been",
					"and you dont give a fuck",
				},
				expected: []expectedResult{
					{17, 4, true},  // 01: match "info" at end
					{11, 4, true},  // 02: match "warn" at end
					{19, 4, true},  // 03: match "warn" at end
					{23, 0, false}, // 04: non-match, just advance all
					{16, 5, true},  // 05: match "error" in the middle
					{8, 4, true},   // 06: match "warn" at start
					{39, 0, false}, // 07: match none, advance all
				},
			},
		}
		performTest(t, testCases)
	})

	t.Run("Test with Wildcard and various features", func(t *testing.T) {
		testCases := []testCase{
			{
				name:   "wildcard pattern ab.*c - partial then match",
				expr:   "ab.*c",
				inputs: []string{"aaa", "bkkkkkkkkca"},
				expected: []expectedResult{
					{2, 1, false}, // "aaa" - no match, advance by 2 with offset 1
					{0, 11, true}, // "bkkkkkkkkkca" - matches "ab.*c" pattern
				},
			},
			{
				name:   "wildcard pattern with immediate match",
				expr:   "ab.*c",
				inputs: []string{"abc", "xyz"},
				expected: []expectedResult{
					{0, 3, true},  // "abc" - matches "abc" (.* matches empty)
					{3, 0, false}, // "xyz" - no match
				},
			},
			{
				name:   "wildcard pattern with middle characters",
				expr:   "ab.*c",
				inputs: []string{"ab123c", "def"},
				expected: []expectedResult{
					{0, 6, true},  // "ab123c" - matches "ab.*c"
					{3, 0, false}, // "def" - no match
				},
			},
			{
				name: "long stream with prefix wildcard",
				expr: "[a-z]+114514",
				inputs: []string{
					"ABCD abcd1",
					"14514 yeah",
					" 114514 abcd",
					"114514",
				},
				expected: []expectedResult{
					{5, 5, false},  // 01: partial match "abcd1" at end
					{0, 10, true},  // 02: matched the rest "14514"
					{13, 4, false}, // 03: must be alphabet before "114514", partial match at the end
					{0, 10, true},  // 04: matched
				},
			},
		}
		performTest(t, testCases)
	})

	t.Run("Submatch test with various regex features", func(t *testing.T) {
		testCases := []testCase{
			{
				name:   "submatch pattern ab(.*)c - partial then match",
				expr:   "ab(.*)c",
				inputs: []string{"aaa", "bkkkkkkkkca"},
				expected: []expectedResult{
					{2, 1, false}, // "aaa" - no match, advance by 2 with offset 1
					{0, 11, true}, // "bkkkkkkkkkca" - matches "ab.*c" pattern
				},
			},
			{
				name:   "wildcard pattern with immediate match",
				expr:   "ab.*c",
				inputs: []string{"abc", "xyz"},
				expected: []expectedResult{
					{0, 3, true},  // "abc" - matches "abc" (.* matches empty)
					{3, 0, false}, // "xyz" - no match
				},
			},
			{
				name:   "wildcard pattern with middle characters",
				expr:   "ab.*c",
				inputs: []string{"ab123c", "def"},
				expected: []expectedResult{
					{0, 6, true},  // "ab123c" - matches "ab.*c"
					{3, 0, false}, // "def" - no match
				},
			},
			{
				name: "long stream with prefix wildcard",
				expr: "[a-z]+114514",
				inputs: []string{
					"ABCD abcd1",
					"14514 yeah",
					" 114514 abcd",
					"114514",
				},
				expected: []expectedResult{
					{5, 5, false},  // 01: partial match "abcd1" at end
					{0, 10, true},  // 02: matched the rest "14514"
					{13, 4, false}, // 03: must be alphabet before "114514", partial match at the end
					{0, 10, true},  // 04: matched
				},
			},
		}
		performTest(t, testCases)
	})
}

type (
	expectedResult struct {
		index  int
		offset int
		ok     bool
	}

	testCase struct {
		name     string
		expr     string
		inputs   []string
		expected []expectedResult
	}
)

func performTest(t *testing.T, tcs []testCase) {
	for _, tc := range tcs {
		if tc.expr == "$" {
			fmt.Println("q")
		}
		re, err := legex.Compile(tc.expr)
		require.NoError(t, err)

		machine := re.Get()

		var index, offset int
		var input []byte

		for i, inputStr := range tc.inputs {
			input = append(input, []byte(inputStr)...)

			idx, off, ok := machine.Match(index, offset, input)
			expected := tc.expected[i]
			assert.Equal(
				t, expected, expectedResult{idx, off, ok},
				tc.name+" | "+"index mismatch for input %d (%s)", i, inputStr,
			)

			if ok { // If match, advance input by the whole pattern and set offset to 0
				machine.Reset()
				input, index, offset = input[idx+off:], 0, 0
			} else { // If not match, advance input by idx and update offset
				input, index, offset = input[idx:], 0, off
			}
		}

		machine.Close()
	}
}
