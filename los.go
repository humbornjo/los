package los

import (
	"bytes"
	"iter"
)

type State = int

const (
	STATE_NONE State = iota
	STATE_HEAD
	STATE_BODY
	STATE_TAIL
)

type Pair struct {
	head      string
	headRegex bool
	tail      string
	tailRegex bool
}

type pairOption func(*Pair) *Pair

func WithRegexHead() pairOption {
	return func(pair *Pair) *Pair {
		pair.headRegex = true
		return pair
	}
}

func WithRegexTail() pairOption {
	return func(pair *Pair) *Pair {
		pair.tailRegex = true
		return pair
	}
}

func NewPair(head, tail string, opts ...pairOption) *Pair {
	pair := &Pair{head: head, tail: tail}
	for _, opt := range opts {
		pair = opt(pair)
	}
	return pair
}

func NewMatcher(pair *Pair) Matcher {
	enableRegex := pair.headRegex || pair.tailRegex
	if enableRegex {
		return nil // losRegex(pair)
	}
	return losDefault(pair)
}

type Matcher interface {
	// Drain return the remaining unmatched string in the buffer of
	// matcher, this should only be called after matching is done.
	Drain() string
	// Match takes a string as input and return a sequence of
	// Result against the input. There could be 0 or more Result.
	Match(string) Results
	// MatchReader(rx io.RuneReader) iter.Seq[Result]
}

// Results is a iterator of Result
type Results iter.Seq[Result]

// Result is the result of match, every Result must not be empty
// (len(Result.Raw()) > 0), String() and Raw() return the content
// of the matched string in state attached.
type Result interface {
	// Raw returns the content of the matched string in state
	Raw() []byte
	// State returns the state of the result content
	State() State
	// String is a shortcut for string(Raw())
	String() string
	// Matches returns a sequence of matched stringi
	//
	// For normal pair matches, the returned iterator should be of
	// length 1 and the value should be the same as String().
	//
	// For regex pair matches, the returned iterator will return
	// all the submatch in the compiled regular expression.
	Matches() iter.Seq[string]
}

var _ Result = textResult{}

type textResult struct {
	state State
	raw   []byte
}

func (r textResult) Raw() []byte {
	return r.raw
}

func (r textResult) String() string {
	return string(r.raw)
}

func (r textResult) State() State {
	return r.state
}

func (r textResult) Matches() iter.Seq[string] {
	return func(yield func(string) bool) {
		yield(r.String())
	}
}

// Default Implementation ---------------------------------------

var _ Matcher = (*defaultMatcher)(nil)

// losDefault takes a Pair as input and return a matcher that
// matches it.
func losDefault(pair *Pair) Matcher {
	return &defaultMatcher{
		state:    STATE_NONE,
		buffer:   bytes.NewBuffer(nil),
		patterns: [2]pattern{newDefaultPattern(pair.head), newDefaultPattern(pair.tail)},
	}
}

type defaultMatcher struct {
	state    State
	index    int
	offset   int
	buffer   *bytes.Buffer
	patterns [2]pattern
}

func (dm *defaultMatcher) Drain() string {
	defer dm.buffer.Reset()
	dm.index, dm.offset, dm.state = 0, 0, STATE_NONE
	return dm.buffer.String()
}

func (dm *defaultMatcher) Match(s string) Results {
	return func(yield func(Result) bool) {
		dm.buffer.WriteString(s)
	encore:
		pattern, buffer := dm.patterns[dm.state>>1], dm.buffer.Bytes()
		index, offset, ok := pattern.match(dm.index, dm.offset, buffer)
		if ok {
			dm.index, dm.offset = 0, offset
			if index > 0 &&
				!yield(textResult{dm.state, dm.buffer.Next(index)}) {
				return
			}
			dm.offset = 0
			if !yield(textResult{dm.state + 1, dm.buffer.Next(offset)}) {
				return
			}
			dm.state = dm.state ^ 0b10 // transfer state
			goto encore
		}
		dm.index, dm.offset = index, offset
		if dm.index == 0 {
			return
		}
		yield(textResult{dm.state, dm.buffer.Next(dm.index)})
		dm.index = 0
	}
}
