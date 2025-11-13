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
				assert.Equal(t, expected.index, idx, "index mismatch for input %d (%s)", i, inputStr)
				assert.Equal(t, expected.offset, off, "offset mismatch for input %d (%s)", i, inputStr)
				assert.Equal(t, expected.ok, ok, "match result mismatch for input %d (%s)", i, inputStr)

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
			inputs: []string{"aaa", "bkkkkkkkkkca"},
			expected: []struct {
				index  int
				offset int
				ok     bool
			}{
				{2, 1, false}, // "aaa" - no match, advance by 2 with offset 1
				{0, 12, true}, // "bkkkkkkkkkca" - matches "ab.*c" pattern
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
				assert.Equal(t, expected.index, idx, "index mismatch for input %d (%s)", i, inputStr)
				assert.Equal(t, expected.offset, off, "offset mismatch for input %d (%s)", i, inputStr)
				assert.Equal(t, expected.ok, ok, "match result mismatch for input %d (%s)", i, inputStr)

				if ok { // If match, advance input by the whole pattern and set offset to 0
					input, index, offset = input[idx+off:], 0, 0
				} else { // If not match, advance input by idx and update offset
					input, index, offset = input[idx:], 0, off
				}
			}
		})
	}
}
