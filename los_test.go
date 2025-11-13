package los

import (
	"iter"
	"slices"
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
			expectedResults: [][]Result{{textResult{STATE_NONE, []byte("test")}}},
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
			expectedResults: [][]Result{{textResult{STATE_HEAD, []byte("prologue")}}},
			drainedContent:  "", // All content matched
		},
		{
			name:     "multiple contents with complete matches",
			contents: []string{"prologue", "content", "epilogue"},
			expectedResults: [][]Result{{
				textResult{STATE_HEAD, []byte("prologue")},
			}, {
				textResult{STATE_BODY, []byte("content")},
			}, {
				textResult{STATE_TAIL, []byte("epilogue")},
			}},
			drainedContent: "", // All content matched across calls
		},
		{
			name:     "combined content with both prologue and epilogue",
			contents: []string{"prologue middle content epilogue"},
			expectedResults: [][]Result{{
				textResult{STATE_HEAD, []byte("prologue")},
				textResult{STATE_BODY, []byte(" middle content ")},
				textResult{STATE_TAIL, []byte("epilogue")},
			}},
			drainedContent: "", // All content matched
		},
		{
			name:     "complete prologue and partial epilogue",
			contents: []string{"prologuedata", "epilo"},
			expectedResults: [][]Result{{
				textResult{STATE_HEAD, []byte("prologue")},
				textResult{STATE_BODY, []byte("data")},
			}, nil},
			drainedContent: "epilo", // All content matched
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i, content := range tt.contents {
				expected := tt.expectedResults[i]
				got := slices.Collect(iter.Seq[Result](matcher.Match(content)))
				require.Equal(t, expected, got)
			}

			drainedContent := matcher.Drain()
			require.Equal(t, tt.drainedContent, drainedContent)
		})
	}
}
