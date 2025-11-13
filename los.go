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

func NewMatcher(pair *Pair) Matcher {
	var patHead, parTail pattern
	if pair.headRegex == 0 {
		patHead = newKmpPattern(pair.head)
	} else {
		patHead = newRegexPattern(pair.head, pair.headRegex)
	}

	if pair.tailRegex == 0 {
		parTail = newKmpPattern(pair.tail)
	} else {
		parTail = newRegexPattern(pair.tail, pair.tailRegex)
	}
	return &matcher{STATE_NONE, 0, 0, bytes.NewBuffer(nil), [2]pattern{patHead, parTail}}
}

type Matcher interface {
	// Drain return the remaining unmatched string in the buffer of
	// matcher and reset the internal state, this should only be
	// called after matching is done.
	Drain() string
	// Match takes a string as input and return a sequence of
	// Result against the input. There could be 0 or more Result.
	Match(string) Results

	// Close must be called for each matcher. It act as nop for
	// kmpPattern. For regexPattern, however, Close will restore
	// machine in regexPattern, thus to reduce the memory alloc
	// pressure. It throws error if there is still data in buffer.
	//
	// WARN: Matcher should never be further used after Close.
	Close() error
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

var _ Matcher = (*matcher)(nil)

type matcher struct {
	state    State
	index    int
	offset   int
	buffer   *bytes.Buffer
	patterns [2]pattern
}

func (m *matcher) Drain() string {
	defer m.buffer.Reset()
	m.index, m.offset, m.state = 0, 0, STATE_NONE
	return m.buffer.String()
}

func (m *matcher) Match(s string) Results {
	return func(yield func(Result) bool) {
		m.buffer.WriteString(s)
	encore:
		pattern, buffer := m.patterns[m.state>>1], m.buffer.Bytes()
		index, offset, ok := pattern.Match(m.index, m.offset, buffer)
		if ok {
			m.index, m.offset = 0, offset
			if index > 0 &&
				!yield(textResult{m.state, m.buffer.Next(index)}) {
				return
			}
			m.offset = 0
			if !yield(textResult{m.state + 1, m.buffer.Next(offset)}) {
				return
			}
			m.state = m.state ^ 0b10 // transfer state
			goto encore
		}
		m.index, m.offset = index, offset
		if m.index == 0 {
			return
		}
		yield(textResult{m.state, m.buffer.Next(m.index)})
		m.index = 0
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
	Match(index int, offset int, s []byte) (newIndex int, newOffset int, ok bool)

	// Clear clean up the inner state of pattern
	Clear()
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

func (pat *kmpPattern) Match(index int, offset int, buffer []byte) (int, int, bool) {
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

// Implemented with regular expression VM for forward search.
//
// - https://swtch.com/~rsc/regexp/regexp2.html
type regexPattern struct {
	*legex.Machine
	clearFunc func()
}

// legex.Machine implement pattern
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
	return &regexPattern{re.Get(), func() { re.Put(re.Get()) }}
}

func (pat *regexPattern) Clear() {
	pat.clearFunc()
}
