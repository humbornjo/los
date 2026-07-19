// WARN: This package is not thread safe
package los

import (
	"bytes"
	"errors"
	"iter"

	"github.com/humbornjo/los/internal/legex"
)

var (
	ErrBufferNotDrained = errors.New("matcher closed without drained")
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
	headRegex regexMode
	tail      string
	tailRegex regexMode
	value     any
}

type pairOption func(*Pair) *Pair

type regexMode int

const (
	_REGEX_MODE_NONE regexMode = iota
	REGEX_MODE_PERL
	REGEX_MODE_POSIX
)

func WithRegexHead(mode ...regexMode) pairOption {
	m := _REGEX_MODE_NONE
	if len(mode) > 0 {
		m = mode[0]
	}
	return func(pair *Pair) *Pair {
		pair.headRegex = m
		return pair
	}
}

func WithRegexTail(mode ...regexMode) pairOption {
	m := _REGEX_MODE_NONE
	if len(mode) > 0 {
		m = mode[0]
	}
	return func(pair *Pair) *Pair {
		pair.tailRegex = m
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

// WithValue labels results produced while this pair is active.
func (p *Pair) WithValue(value any) *Pair {
	p.value = value
	return p
}

// NewMatcher builds a streaming matcher. Multiple pairs are selected by the
// earliest head, with registration order breaking ties at the same position.
func NewMatcher(first *Pair, rest ...*Pair) Matcher {
	if first == nil {
		panic("los: pair must not be nil")
	}
	if len(rest) > 0 {
		pairs := make([]*Pair, 1, len(rest)+1)
		pairs[0] = first
		for _, pair := range rest {
			if pair == nil {
				panic("los: pair must not be nil")
			}
			pairs = append(pairs, pair)
		}
		return newAutomationMatcher(pairs)
	}
	return newMatcher(first)
}

func newMatcher(pair *Pair) Matcher {
	var patHead, parTail pattern
	if pair.headRegex == _REGEX_MODE_NONE {
		patHead = newKmpPattern(pair.head)
	} else {
		patHead = newRegexPattern(pair.head, pair.headRegex)
	}

	if pair.tailRegex == _REGEX_MODE_NONE {
		parTail = newKmpPattern(pair.tail)
	} else {
		parTail = newRegexPattern(pair.tail, pair.tailRegex)
	}
	return &matcher{
		state:    STATE_NONE,
		buffer:   bytes.NewBuffer(nil),
		patterns: [2]pattern{patHead, parTail},
		context:  legex.NewStreamContext(),
		value:    pair.value,
	}
}

type Matcher interface {
	// Drain return the remaining unmatched string in the buffer of
	// matcher and reset the internal state, this should only be
	// called after matching is done.
	Drain() string
	// Match takes a string as input and return a sequence of
	// Result against the input. There could be 0 or more Result.
	Match(string) Results
	// Finish resolves end-of-text assertions and returns the final results.
	// It resets the matcher so it can process a new logical stream.
	Finish() Results

	// Close must be called for each matcher. It act as nop for
	// kmpPattern. For regexPattern, however, Close will restore
	// machine in regexPattern, thus to reduce the memory alloc
	// pressure. It throws error if there is still data in buffer.
	//
	// WARN: Matcher should never be further used after Close.
	Close() error
}

// Results is a iterator of Result
type Results = iter.Seq[Result]

// Result is the result of match, every Result must not be empty
// (len(Result.Raw()) > 0), String() and Raw() return the content
// of the matched string in state attached.
type Result interface {
	// Raw returns the content of the matched string in state
	Raw() []byte
	// State returns the state of the result content
	State() State
	// Value returns the value configured on the active pair. Unmatched text
	// outside a pair returns nil.
	Value() any
	// String is a shortcut for string(Raw())
	String() string
	// Matches returns a sequence of matched string
	//
	// For normal pair matches, the returned iterator should be of
	// length 1 and the value should be the same as String().
	//
	// For regex pair matches, the returned iterator will yield all
	// the submatch in the compiled regular expression.
	Matches() iter.Seq[string]
}

var _ Result = textResult{}

type textResult struct {
	state State
	raw   []byte
	value any
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

func (r textResult) Value() any {
	return r.value
}

func (r textResult) Matches() iter.Seq[string] {
	return func(yield func(string) bool) {
		yield(r.String())
	}
}

var _ Result = regexResult{}

type regexResult struct {
	state    State
	raw      []byte
	matchcap []int
	value    any
}

func (r regexResult) Raw() []byte {
	return r.raw
}

func (r regexResult) String() string {
	return string(r.raw)
}

func (r regexResult) State() State {
	return r.state
}

func (r regexResult) Value() any {
	return r.value
}

func (r regexResult) Matches() iter.Seq[string] {
	return func(yield func(string) bool) {
		for i := 0; i < len(r.matchcap); i += 2 {
			s, e := r.matchcap[i], r.matchcap[i+1]
			match := ""
			if s >= 0 && e >= 0 {
				match = string(r.raw[s:e])
			}
			if !yield(match) {
				return
			}
		}
	}
}

// Default Implementation ---------------------------------------

var _ Matcher = (*matcher)(nil)

type matcher struct {
	state    State
	index    int
	offset   int
	buffer   *bytes.Buffer
	patterns [2]pattern
	context  legex.StreamContext
	value    any
}

func (m *matcher) Drain() string {
	defer m.buffer.Reset()
	m.index, m.offset, m.state = 0, 0, STATE_NONE
	m.context = legex.NewStreamContext()
	for _, pattern := range m.patterns {
		pattern.Reset()
	}
	return m.buffer.String()
}

func (m *matcher) Match(s string) Results {
	m.buffer.WriteString(s)

	return func(yield func(Result) bool) {
	encore:
		pattern, buffer := m.patterns[m.state>>1], m.buffer.Bytes()
		index, offset, ok := pattern.Match(m.context, m.index, m.offset, buffer)
		if ok {
			if offset == 0 {
				panic("los: pattern matched empty input")
			}
			state := m.state
			m.index, m.offset = 0, offset
			if index > 0 &&
				!yield(m.build(pattern, index, state)) {
				return
			}
			m.offset, m.state = 0, state^0b10
			if !yield(m.build(pattern, offset, state+1)) {
				return
			}
			goto encore
		}
		m.index, m.offset = index, offset
		if m.index == 0 {
			return
		}
		yield(m.build(pattern, index, m.state))
		m.index = 0
	}
}

func (m *matcher) build(pattern pattern, n int, state State) Result {
	var value any
	if state != STATE_NONE {
		value = m.value
	}
	result := pattern.Build(m.buffer, n, state, value)
	m.context = m.context.Advance(result.Raw())
	return result
}

func (m *matcher) Finish() Results {
	return func(yield func(Result) bool) {
		defer func() {
			m.index, m.offset, m.state = 0, 0, STATE_NONE
			m.buffer.Reset()
			m.context = legex.NewStreamContext()
			for _, pattern := range m.patterns {
				pattern.Reset()
			}
		}()

		for m.buffer.Len() > 0 {
			state := m.state
			pattern := m.patterns[state>>1]
			index, length, ok := pattern.Finish(m.context, m.buffer.Bytes())
			if ok && length == 0 {
				panic("los: pattern matched empty input")
			}
			if index > 0 && !yield(m.build(pattern, index, state)) {
				return
			}
			if !ok {
				return
			}
			m.state = state ^ 0b10
			if !yield(m.build(pattern, length, state+1)) {
				return
			}
		}
	}
}

func (m *matcher) Close() error {
	m.patterns[0].Clear()
	m.patterns[1].Clear()

	if m.buffer.Len() > 0 {
		return ErrBufferNotDrained
	}
	return nil
}

// Pattern ------------------------------------------------------

type pattern interface {
	// Match advance the Match index and offset to release the
	// unmatched string in buffer ASAP.
	Match(ctx legex.StreamContext, index int, offset int, s []byte) (newIndex int, newOffset int, ok bool)
	Finish(ctx legex.StreamContext, s []byte) (index int, length int, ok bool)
	Reset()

	// Clear clean up the inner state of pattern
	Clear()

	// Build return a Result
	Build(buffer *bytes.Buffer, n int, state State, value any) Result
}

// Implemented with Knuth-Morris-Pratt algorithm for forward
// search.
type kmpPattern struct {
	lps    []int
	length int
	source string
}

var _ pattern = (*kmpPattern)(nil)

func newKmpPattern(source string) *kmpPattern {
	if source == "" {
		panic("los: pattern must not match empty input")
	}
	computeLpsArray := func(pattern string) []int {
		n := len(pattern)
		array := make([]int, n)
		for i, j := 1, 0; i < n; {
			if pattern[i] == pattern[j] {
				j++
				array[i], i = j, i+1
			} else {
				if j != 0 {
					j = array[j-1]
				} else {
					array[i], i = 0, i+1
				}
			}
		}
		return array
	}
	return &kmpPattern{computeLpsArray(source), len(source), source}
}

func (pat *kmpPattern) Finish(ctx legex.StreamContext, buffer []byte) (int, int, bool) {
	index, length, ok := pat.Match(ctx, 0, 0, buffer)
	if !ok {
		return len(buffer), 0, false
	}
	return index, length, true
}

func (pat *kmpPattern) Reset() {}

func (pat *kmpPattern) Match(_ legex.StreamContext, index int, offset int, buffer []byte) (int, int, bool) {
	if offset == pat.length {
		return index, offset, true
	}
	n, m := len(buffer), pat.length
	i, j := index+offset, offset // start match index with offset
	for i < n {
		if buffer[i] == pat.source[j] {
			i, j = i+1, j+1
			if j == m {
				return i - j, j, true
			}
		} else {
			if j != 0 {
				j = pat.lps[j-1]
			} else {
				i++
			}
		}
	}
	return i - j, j, false
}

func (pat *kmpPattern) Clear() {}

func (pat *kmpPattern) Build(buffer *bytes.Buffer, n int, state State, value any) Result {
	return textResult{state: state, raw: buffer.Next(n), value: value}
}

// Implemented with regular expression VM for forward search.
//
// - https://swtch.com/~rsc/regexp/regexp2.html
type regexPattern struct {
	legex.Machine
}

var _ pattern = (*regexPattern)(nil)

func newRegexPattern(pattern string, mode regexMode) *regexPattern {
	var re *legex.Regexp
	switch mode {
	case REGEX_MODE_PERL:
		re = legex.MustCompile(pattern)
	case REGEX_MODE_POSIX:
		re = legex.MustCompilePOSIX(pattern)
	default:
		panic("unreachable")
	}
	if re.CanMatchEmpty() {
		panic("los: pattern must not match empty input")
	}
	return &regexPattern{re.Get()}
}

func (pat *regexPattern) Clear() {
	pat.Close()
}

func (pat *regexPattern) Match(ctx legex.StreamContext, _, _ int, buffer []byte) (int, int, bool) {
	return pat.Machine.Match(ctx, buffer)
}

func (pat *regexPattern) Finish(ctx legex.StreamContext, buffer []byte) (int, int, bool) {
	return pat.Machine.Finish(ctx, buffer)
}

func (pat *regexPattern) Build(buffer *bytes.Buffer, n int, state State, value any) Result {
	if state%2 == 0 {
		return textResult{state: state, raw: buffer.Next(n), value: value}
	}

	matchcap := pat.MatchCap()
	start := matchcap[0]
	for i, pos := range matchcap {
		if pos >= 0 {
			matchcap[i] = pos - start
		}
	}
	pat.Reset()
	return regexResult{state: state, raw: buffer.Next(n), matchcap: matchcap, value: value}
}
