package legex

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMachine_Match_Base(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		inputs   []string
		expected []struct {
			index  int
			offset int
			ok     bool
		}
	}{
		{
			name:   "simple anchored abc pattern",
			expr:   "^abc",
			inputs: []string{"aaa", "bcd"},
			expected: []struct {
				index  int
				offset int
				ok     bool
			}{
				{2, 1, false}, // "aaa" - partial match "a"
				{0, 3, true},  // "abcd" - should match "abc"
			},
		},
		{
			name:   "anchored pattern with partial match",
			expr:   "^abc",
			inputs: []string{"ab", "cdef"},
			expected: []struct {
				index  int
				offset int
				ok     bool
			}{
				{0, 2, false}, // "ab" - partial, no match
				{0, 3, true},  // "abcdef" - should match "abc"
			},
		},
		{
			name:   "pattern starting in middle of input",
			expr:   "abc",
			inputs: []string{"xababc", "def"},
			expected: []struct {
				index  int
				offset int
				ok     bool
			}{
				{3, 3, true},  // "xabc" - match "abc" starting at index 1
				{3, 0, false}, // "def" - no match, adcance all
			},
		},
		{
			name: "long stream with multiple keyword matches",
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
			expected: []struct {
				index  int
				offset int
				ok     bool
			}{
				{17, 4, true},  // First input - partial match "warning" at end
				{11, 4, true},  // Second input - partial match "warning" at start
				{19, 4, true},  // Third input - matches "warning"
				{23, 0, false}, // non-match, just advance all
				{16, 5, true},  // Fifth input - partial match "Error" at start, then partial "info"
				{8, 4, true},   // Fifth input - matches "info"
				{39, 0, false}, // Sixth input - partial match "warning" at end
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			re, err := Compile(tt.expr)
			require.NoError(t, err)

			machine := re.Get()
			defer re.Put(machine)

			var index, offset int
			var input []byte

			for i, inputStr := range tt.inputs {
				input = append(input, []byte(inputStr)...)

				idx, off, ok := machine.Match(index, offset, input)
				expected := tt.expected[i]
				assert.Equal(t, expected, struct {
					index  int
					offset int
					ok     bool
				}{idx, off, ok}, "index mismatch for input %d (%s)", i, inputStr)

				if ok { // If match, advance input by the whole pattern and set offset to 0
					input, index, offset = input[idx+off:], 0, 0
				} else { // If not match, advance input by idx and update offset
					input, index, offset = input[idx:], 0, off
				}
			}
		})
	}
}

func TestMachine_Match_Wildcard(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		inputs   []string
		expected []struct {
			index  int
			offset int
			ok     bool
		}
	}{
		{
			name:   "wildcard pattern ab.*c - partial then match",
			expr:   "ab.*c",
			inputs: []string{"aaa", "bkkkkkkkkca"},
			expected: []struct {
				index  int
				offset int
				ok     bool
			}{
				{2, 1, false}, // "aaa" - no match, advance by 2 with offset 1
				{0, 11, true}, // "bkkkkkkkkkca" - matches "ab.*c" pattern
			},
		},
		{
			name:   "wildcard pattern with immediate match",
			expr:   "ab.*c",
			inputs: []string{"abc", "xyz"},
			expected: []struct {
				index  int
				offset int
				ok     bool
			}{
				{0, 3, true},  // "abc" - matches "abc" (.* matches empty)
				{3, 0, false}, // "xyz" - no match
			},
		},
		{
			name:   "wildcard pattern with middle characters",
			expr:   "ab.*c",
			inputs: []string{"ab123c", "def"},
			expected: []struct {
				index  int
				offset int
				ok     bool
			}{
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
			expected: []struct {
				index  int
				offset int
				ok     bool
			}{
				{5, 5, false},  // First input - partial match "warning" at end
				{0, 10, true},  // Second input - partial match "warning" at start
				{13, 4, false}, // Second input - partial match "warning" at start
				{0, 10, true},  // Second input - partial match "warning" at start
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			re, err := Compile(tt.expr)
			require.NoError(t, err)

			machine := re.Get()
			defer re.Put(machine)

			var index, offset int
			var input []byte

			for i, inputStr := range tt.inputs {
				input = append(input, []byte(inputStr)...)

				idx, off, ok := machine.Match(index, offset, input)
				expected := tt.expected[i]
				assert.Equal(t, expected, struct {
					index  int
					offset int
					ok     bool
				}{idx, off, ok}, "index mismatch for input %d (%s)", i, inputStr)

				if ok { // If match, advance input by the whole pattern and set offset to 0
					input, index, offset = input[idx+off:], 0, 0
				} else { // If not match, advance input by idx and update offset
					input, index, offset = input[idx:], 0, off
				}
			}
		})
	}
}
