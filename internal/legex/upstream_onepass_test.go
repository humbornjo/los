// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package legex_test

import (
	"strings"
	"testing"

	"github.com/humbornjo/los/internal/legex"
	"github.com/stretchr/testify/require"
)

// Go's onePassTests classify an optional execution backend. LOS deliberately
// uses one ordered NFA, so every pattern is instead checked for observable
// matching parity over a streamed input corpus.
var _UPSTREAM_ONE_PASS_TESTS = []string{
	`^(?:a|(?:a*))$`,
	`^(?:(a)|(?:a*))$`,
	`^(?:(?:(?:.(?:$))?))$`,
	`^abcd$`,
	`^abcd`,
	`^(?:(?:a{0,})*?)$`,
	`^(?:(?:a+)*)$`,
	`^(?:(?:a|(?:aa)))$`,
	`^(?:[^\s\S])$`,
	`^(?:(?:a{3,4}){0,})$`,
	`^(?:(?:(?:a*)+))$`,
	`^[a-c]+$`,
	`^[a-c]*$`,
	`^(?:a*)$`,
	`^(?:(?:aa)|a)$`,
	`^[a-c]*`,
	`^...$`,
	`^...`,
	`^(?:a|(?:aa))$`,
	`^a((b))c$`,
	`^a.[l-nA-Cg-j]?e$`,
	`^a((b))$`,
	`^a(?:(b)|(c))c$`,
	`^a(?:(b*)|(c))c$`,
	`^a(?:b|c)$`,
	`^a(?:b?|c)$`,
	`^a(?:b?|c?)$`,
	`^a(?:b?|c+)$`,
	`^a(?:b+|(bc))d$`,
	`^a(?:bc)+$`,
	`^a(?:[bcd])+$`,
	`^a((?:[bcd])+)$`,
	`^a(:?b|c)*d$`,
	`^.bc(d|e)*$`,
	`^(?:(?:aa)|.)$`,
	`^(?:(?:a{1,2}){1,2})$`,
	`^l` + strings.Repeat("o", 2<<8) + `ng$`,
}

func TestLegex_UpstreamOnePassPatternsStreaming(t *testing.T) {
	inputs := generatedInputs([]byte{'a', 'b', 'c', 'd', 'e', '\n'}, 4)
	longInput := "l" + strings.Repeat("o", 2<<8) + "ng"
	inputs = append(inputs, "abcd", "abcc", "a/b#c", longInput, longInput+"x")

	for i, expr := range _UPSTREAM_ONE_PASS_TESTS {
		for _, input := range inputs {
			streamCase := newUpstreamStreamCase(expr, input)
			re, err := legex.Compile(expr)
			require.NoError(t, err, "case=%d expr=%q", i, expr)
			require.Equal(t, streamCase.expected, streamAllMatchBlocks(re, input),
				"case=%d expr=%q input=%q", i, expr, input)
		}
	}
}

func TestLegex_UpstreamRunOnePassCaseStreaming(t *testing.T) {
	expr, input := `^a(/b+(#c+)*)*$`, "a/b#c"
	streamCase := newUpstreamStreamCase(expr, input)
	re := legex.MustCompile(expr)
	require.Equal(t, streamCase.expected, streamAllMatchBlocks(re, input))
}
