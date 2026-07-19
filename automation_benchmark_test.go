package los

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
)

func BenchmarkLos_AutomationMatcherLiteralHeads(b *testing.B) {
	for _, count := range []int{2, 8, 64, 256} {
		patterns := make([]acPattern, count)
		for i := range patterns {
			patterns[i] = acPattern{pair: i, text: fmt.Sprintf("literal-head-%03d-END", i)}
		}
		input := strings.Repeat("ordinary-stream-data;", 2048) + patterns[count-1].text

		b.Run(fmt.Sprintf("automation/%d", count), func(b *testing.B) {
			head := newAcHead(patterns)
			data := []byte(input)
			b.ReportAllocs()
			b.SetBytes(int64(len(data)))
			var candidate headCandidate
			for b.Loop() {
				head.Reset(0)
				head.Feed(data)
				candidate = head.candidate
			}
			if !head.hasCandidate {
				b.Fatal("literal head did not match")
			}
			runtime.KeepAlive(candidate)
		})

		b.Run(fmt.Sprintf("naive/%d", count), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(input)))
			var candidate headCandidate
			for b.Loop() {
				candidate = naiveLiteralHead(patterns, input)
			}
			if candidate.pair < 0 {
				b.Fatal("literal head did not match")
			}
			runtime.KeepAlive(candidate)
		})
	}
}

func naiveLiteralHead(patterns []acPattern, input string) headCandidate {
	winner := headCandidate{pair: -1}
	for _, pattern := range patterns {
		index := strings.Index(input, pattern.text)
		if index < 0 {
			continue
		}
		candidate := headCandidate{
			start: int64(index),
			end:   int64(index + len(pattern.text)),
			pair:  pattern.pair,
		}
		if winner.pair < 0 || candidateBefore(candidate, winner) {
			winner = candidate
		}
	}
	return winner
}
