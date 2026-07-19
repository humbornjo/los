# LOS - Life Of Speech

[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
![Alpha](https://img.shields.io/badge/status-alpha-orange.svg)

_LOS_ is actually a name derived from _POS_ (Part Of Speech). However, In the context of _LOS_, speech has a **Start
(head)** and an **End (tail)**. What this library does is: given **a stream of string contents**, tell you what part(s)
current content (in the stream) would match against.

Imagine you encounter a requirement that asks you to find a pattern `<los type=[TYPE] data=[DATA]>[Content]</los>` and
extract the `[TYPE]`, `[DATA]` and `[CONTENT]` from a stream of data (It would be the response stream of LLM nowadays if
you ask me). If you think first matching `<los type=`, then `data=` and `>` and finally `</los>` is too ugly. You might
want to try this lib.

**ATTENTION: This lib does not strictly outperform the Brute-Force matching. It just provide an elegant way to match a
specific pattern in a continuous stream**

Life of Speech support,

- **Fixed pattern match**: Implemented with Knuth-Morris-Pratt (KMP) algorithm.
- **Regex pattern match**: Implemented with the virtual machine used in Go STD lib `regexp`.

And you can use different pattern schema for head and tail (like fixed pattern for head and regex pattern for tail).

> STD `regexp` lib could actually support streaming data with some extension. See `match` function of `machine`, but
> `regexp` did not exploit this feature, It is more like they omit the need for resumable regex matching.

## Implementation status

The regex path uses one ordered NFA execution engine for all patterns. It supports Go regexp syntax and observable
matching semantics, including Perl leftmost-first and POSIX leftmost-longest priority, captures, anchors, word
boundaries, invalid UTF-8, and matches split at arbitrary byte boundaries.

When a matcher has multiple pairs, one Aho-Corasick automaton scans all fixed heads while each regex head keeps its own
streaming NFA. The matcher merges their candidates without changing streaming priority semantics.

Go's one-pass and bit-state backtracking engines are performance optimizations with the same observable semantics; they
are not separate LOS feature paths. Optional specialized execution engines and avoiding replay of long retained
candidates remain performance work, not regex correctness gaps.

## Explain

Say, match against a pattern `berserk` with a stream of string `["ber", "serg", "inrelevent", "[b", "erserk]"]`

```plain
MATCH : "ber"
OUTPUT: NONE                // "ber" partially match the pattern

MATCH : "serg"
OUTPUT: ["berserg"]         // match failed, all the inrelevent content is returned

MATCH : "inrelevent"
OUTPUT: ["inrelevent"]      // match failed, all the inrelevent content is returned

MATCH : "[b"
OUTPUT: ["["]               // "b" partially match the pattern, prefixing "[" is returned

MATCH : "erserk]"
OUTPUT: ["berserk", "]"]    // matched, and the suffixing "]" is also returned as non-matched content
```

It should be noted that "anchored" pattern will always match from the start of the stream to maintain the completeness of
regular expression semantics.

## Quickstart

```go
matcher := los.NewMatcher(
  los.NewPair(
    "ab(.*?)c",
    "xyz",
    los.WithRegexHead(los.REGEX_MODE_PERL),
    los.WithValue("block"),
  ),
)
defer matcher.Close()

contents := []string{"ab_c1", "23xyz", "ab"}

for _, content := range contents {
  for result := range matcher.Match(content) {
    switch result.State() {
    case los.STATE_HEAD:
      _ = result.Value() // "block"
      // ["ab_c", "_"]
      for match := range result.Matches() {
        // do something with submatch
      }
    case los.STATE_TAIL:
      _ = result.String() // "xyz"
    case los.STATE_NONE, los.STATE_BODY:
      _ = result.String() // "123"
    }
  }
}

for result := range matcher.Finish() {
  // Handle matches that depend on the end of the stream, plus any
  // remaining unmatched content.
  _ = result
}
```

## Multiple pairs

Pass more than one pair to scan several possible heads in the same stream:

```go
matcher := los.NewMatcher(
  los.NewPair("<think>", "</think>", los.WithValue("think")),
  los.NewPair(`<tool name="([^"]+)">`, "</tool>",
    los.WithRegexHead(los.REGEX_MODE_PERL),
    los.WithValue("tool"),
  ),
)
```

The earliest head wins. If heads start at the same byte, pair registration order breaks the tie, including equivalent
or duplicate heads. Once a head wins, only that pair's tail is active; other heads are body text, so matches do not
nest.

`Result.Value` returns the selected pair's snapshotted value for `STATE_HEAD`, `STATE_BODY`, and `STATE_TAIL` results.
It returns `nil` for `STATE_NONE`. Changing a pair's value after constructing a matcher does not change that matcher.

## Streaming semantics

`Match` treats the end of each input chunk as a soft boundary. A match is emitted only when later input cannot change
the result selected by Go's leftmost-first regexp semantics. For example, greedy repetitions, `$`, `\z`, and word
boundaries may remain buffered until lookahead arrives.

Call `Finish` after the final chunk. It resolves end-of-text assertions, emits the final matched or unmatched results,
and resets the matcher for a new logical stream. Use `Drain` instead when abandoning a stream: it returns the buffered
text without applying final regex matching and resets the matcher.

Head and tail patterns must consume at least one byte. `NewMatcher` rejects fixed or regex patterns that can produce a
zero-length match in any text context because they cannot make progress in a streaming state machine.

## References

- [Go STD lib `regexp`](https://pkg.go.dev/regexp)
- [Regular Expression Matching: the Virtual Machine Approach](https://swtch.com/~rsc/regexp/regexp2.html)

## Alternatives

- Intel [hyperscan](https://github.com/intel/hyperscan) and its Go binding [gohs](https://github.com/flier/gohs)
