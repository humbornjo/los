package legex_test

import (
	"regexp"
	"testing"

	"github.com/humbornjo/los/internal/legex"
	"github.com/stretchr/testify/require"
)

func TestLegex_StreamingMachineDifferential(t *testing.T) {
	patterns := []string{
		`a`, `ab`, `a|ab`, `ab|a`,
		`a*`, `a*?`, `a+`, `a+?`, `a?`, `a??`, `a{2,3}`,
		`(a|b)+`, `(ab)*c`, `a.*b`, `a.*?b`, `[ab]+c`,
		`.`, `.*`, `.*?`, `^a`, `a$`, `\Aa\z`, `(?m)^a$`,
		`\ba\b`, `\Ba\B`, `foo$|fo`, `foo\b|fo`, `a?\b`, `a??\b`, `\B(foo|fo)\B`,
		`(a)?(b*)`, `(|a)*`, `((a|b)*)(c?)`,
		`[^\n]+`, `(?s:.+)`, `(?:ab|a)c`, `a(?:b|)`, `\w+`, `\W+`,
	}
	inputs := generatedInputs([]byte{'a', 'b', ' ', '\n'}, 4)
	inputs = append(inputs,
		"日本", "a日本b", "foo", "xfooo", string([]byte{0xff}), string([]byte{0xc2, 0x00}),
	)

	for _, expr := range patterns {
		standard := regexp.MustCompile(expr)
		for _, input := range inputs {
			partitions := [][]string{
				{input},
				byteChunks(input),
			}
			for split := 0; split <= len(input); split++ {
				partitions = append(partitions, []string{input[:split], input[split:]})
			}
			for _, chunks := range partitions {
				assertStreamingMatch(t, expr, input, chunks, standard.FindStringSubmatchIndex(input), false)
			}
		}
	}
}

func TestLegex_StreamingPosixMachineDifferential(t *testing.T) {
	patterns := []string{
		`a|ab`, `(a|aa)+`, `a+`, `a{1,3}`, `(ab|a)b`, `[ab]+`, `(a*)(a*)`, `^a+$`, `foo$|fo`,
	}
	inputs := generatedInputs([]byte{'a', 'b', '\n'}, 4)
	inputs = append(inputs, "foo")

	for _, expr := range patterns {
		standard := regexp.MustCompilePOSIX(expr)
		for _, input := range inputs {
			partitions := [][]string{
				{input},
				byteChunks(input),
			}
			for _, chunks := range partitions {
				assertStreamingMatch(t, expr, input, chunks, standard.FindStringSubmatchIndex(input), true)
			}
		}
	}
}

func assertStreamingMatch(t *testing.T, expr, input string, chunks []string, expected []int, posix bool) {
	t.Helper()
	var re *legex.Regexp
	var err error
	if posix {
		re, err = legex.CompilePOSIX(expr)
	} else {
		re, err = legex.Compile(expr)
	}
	require.NoError(t, err)
	machine := re.Get()
	defer machine.Close()
	ctx := legex.NewStreamContext()

	retained := make([]byte, 0, len(input))
	released := 0
	for _, chunk := range chunks {
		retained = append(retained, chunk...)
		index, length, ok := machine.Match(ctx, retained)
		require.GreaterOrEqual(t, index, 0, "expr=%q input=%q chunks=%q", expr, input, chunks)
		require.GreaterOrEqual(t, length, 0, "expr=%q input=%q chunks=%q", expr, input, chunks)
		require.LessOrEqual(t, index+length, len(retained), "expr=%q input=%q chunks=%q", expr, input, chunks)
		if ok {
			require.Equal(t, expected, globalCaptures(machine.MatchCap(), released),
				"expr=%q input=%q chunks=%q", expr, input, chunks)
			return
		}
		require.Equal(t, len(retained), index+length, "expr=%q input=%q chunks=%q", expr, input, chunks)
		ctx = ctx.Advance(retained[:index])
		retained = retained[index:]
		released += index
	}

	index, length, ok := machine.Finish(ctx, retained)
	if expected == nil {
		require.False(t, ok, "expr=%q input=%q chunks=%q", expr, input, chunks)
		require.Equal(t, len(retained), index, "expr=%q input=%q chunks=%q", expr, input, chunks)
		require.Zero(t, length, "expr=%q input=%q chunks=%q", expr, input, chunks)
		return
	}
	require.True(t, ok, "expr=%q input=%q chunks=%q", expr, input, chunks)
	require.Equal(t, expected, globalCaptures(machine.MatchCap(), released),
		"expr=%q input=%q chunks=%q", expr, input, chunks)
}

func globalCaptures(captures []int, released int) []int {
	for i, pos := range captures {
		if pos >= 0 {
			captures[i] = pos + released
		}
	}
	return captures
}

func generatedInputs(alphabet []byte, maxLength int) []string {
	inputs := []string{""}
	var appendInputs func([]byte, int)
	appendInputs = func(prefix []byte, remaining int) {
		if remaining == 0 {
			return
		}
		for _, b := range alphabet {
			input := append(append([]byte(nil), prefix...), b)
			inputs = append(inputs, string(input))
			appendInputs(input, remaining-1)
		}
	}
	appendInputs(nil, maxLength)
	return inputs
}

func byteChunks(input string) []string {
	chunks := make([]string, len(input))
	for i := range len(input) {
		chunks[i] = input[i : i+1]
	}
	return chunks
}
