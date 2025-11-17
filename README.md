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

## TODOs

- [x] Avoid duplicate thread add during multiple match on Machine.
- [ ] correct anchored pattern matching behavior.
- [ ] Add test case for submatch and gnarly regular expressions.
- [ ] Implement Onepass Machine and Backtrace Machine for stream matching.
- [ ] improve performance on index advancing by integrating index calculation in thread walk.

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
    "ab(.*)c",
    "xyz",
    los.WithRegexHead(REGEX_MODE_PERL),
  ),
)
defer matcher.Close()

contents := []string{"ab_c1", "23xyz", "ab"}

for _, content := range contents {
  for result := range matcher.Match(content) {
    switch result.State() {
    case los.STATE_HEAD:
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

remains := matcher.Drain() // "ab"
```

## References

- [Go STD lib `regexp`](https://pkg.go.dev/regexp)
- [Regular Expression Matching: the Virtual Machine Approach](https://swtch.com/~rsc/regexp/regexp2.html)

## Alternatives

- Intel [hyperscan](https://github.com/intel/hyperscan) and its Go binding [gohs](https://github.com/flier/gohs)
