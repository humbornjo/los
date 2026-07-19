package los

import (
	"bytes"
	"sort"

	"github.com/humbornjo/los/internal/legex"
)

var _ Matcher = (*automationMatcher)(nil)

type automationMatcher struct {
	state      State
	active     int
	tailIndex  int
	tailOffset int
	headFed    int
	consumed   int64
	buffer     *bytes.Buffer
	context    legex.StreamContext
	selector   *headSelector
	pairs      []automationPair
}

type automationPair struct {
	tail  pattern
	value any
}

type headCandidate struct {
	start    int64
	end      int64
	pair     int
	matchcap []int
}

type headPending struct {
	start int64
	pair  int
	ok    bool
}

func newAutomationMatcher(pairs []*Pair) Matcher {
	fixed := make([]acPattern, 0, len(pairs))
	selector := &headSelector{}
	entries := make([]automationPair, len(pairs))
	for i, pair := range pairs {
		if pair.headRegex == _REGEX_MODE_NONE {
			if pair.head == "" {
				panic("los: pattern must not match empty input")
			}
			fixed = append(fixed, acPattern{pair: i, text: pair.head})
		} else {
			selector.regex = append(selector.regex, newRegexHead(i, pair.head, pair.headRegex))
		}

		var tail pattern
		if pair.tailRegex == _REGEX_MODE_NONE {
			tail = newKmpPattern(pair.tail)
		} else {
			tail = newRegexPattern(pair.tail, pair.tailRegex)
		}
		entries[i] = automationPair{tail: tail, value: pair.value}
	}
	if len(fixed) > 0 {
		selector.fixed = newAcHead(fixed)
	}

	ctx := legex.NewStreamContext()
	selector.Reset(0, ctx)
	return &automationMatcher{
		active:   -1,
		buffer:   bytes.NewBuffer(nil),
		context:  ctx,
		selector: selector,
		pairs:    entries,
	}
}

func (m *automationMatcher) Match(s string) Results {
	m.buffer.WriteString(s)
	return func(yield func(Result) bool) {
	encore:
		if m.state == STATE_NONE {
			buffer := m.buffer.Bytes()
			m.selector.Feed(buffer[m.headFed:])
			m.headFed = len(buffer)
			candidate, safe, ok := m.selector.Decide(false)
			if !ok {
				n := int(safe - m.consumed)
				if n > 0 {
					m.headFed -= n
					yield(m.buildText(n, STATE_NONE, nil))
				}
				return
			}

			index := int(candidate.start - m.consumed)
			if index > 0 {
				m.headFed -= index
				if !yield(m.buildText(index, STATE_NONE, nil)) {
					return
				}
				goto encore
			}

			length := int(candidate.end - candidate.start)
			if length == 0 {
				panic("los: pattern matched empty input")
			}
			m.active = candidate.pair
			m.state = STATE_BODY
			value := m.pairs[m.active].value
			var result Result
			if candidate.matchcap == nil {
				result = m.buildText(length, STATE_HEAD, value)
			} else {
				result = m.buildRegex(length, STATE_HEAD, candidate.matchcap, value)
			}
			m.headFed = 0
			m.selector.Reset(m.consumed, m.context)
			if !yield(result) {
				return
			}
			goto encore
		}

		entry := &m.pairs[m.active]
		index, length, ok := entry.tail.Match(m.context, m.tailIndex, m.tailOffset, m.buffer.Bytes())
		if ok {
			if length == 0 {
				panic("los: pattern matched empty input")
			}
			m.tailIndex, m.tailOffset = 0, length
			if index > 0 && !yield(m.buildPattern(entry.tail, index, STATE_BODY, entry.value)) {
				return
			}

			m.tailOffset = 0
			m.state = STATE_NONE
			result := m.buildPattern(entry.tail, length, STATE_TAIL, entry.value)
			m.active = -1
			m.selector.Reset(m.consumed, m.context)
			if !yield(result) {
				return
			}
			goto encore
		}

		m.tailIndex, m.tailOffset = index, length
		if index > 0 {
			yield(m.buildPattern(entry.tail, index, STATE_BODY, entry.value))
			m.tailIndex = 0
		}
	}
}

func (m *automationMatcher) Finish() Results {
	return func(yield func(Result) bool) {
		defer m.reset()
		for m.buffer.Len() > 0 {
			if m.state == STATE_NONE {
				buffer := m.buffer.Bytes()
				m.selector.Feed(buffer[m.headFed:])
				m.headFed = len(buffer)
				m.selector.Finish()
				candidate, safe, ok := m.selector.Decide(true)
				if !ok {
					n := int(safe - m.consumed)
					if n > 0 {
						m.headFed -= n
						yield(m.buildText(n, STATE_NONE, nil))
					}
					return
				}

				index := int(candidate.start - m.consumed)
				if index > 0 && !yield(m.buildText(index, STATE_NONE, nil)) {
					return
				}
				length := int(candidate.end - candidate.start)
				if length == 0 {
					panic("los: pattern matched empty input")
				}
				m.active = candidate.pair
				m.state = STATE_BODY
				value := m.pairs[m.active].value
				var result Result
				if candidate.matchcap == nil {
					result = m.buildText(length, STATE_HEAD, value)
				} else {
					result = m.buildRegex(length, STATE_HEAD, candidate.matchcap, value)
				}
				m.headFed = 0
				m.selector.Reset(m.consumed, m.context)
				if !yield(result) {
					return
				}
				continue
			}

			entry := &m.pairs[m.active]
			index, length, ok := entry.tail.Finish(m.context, m.buffer.Bytes())
			if ok && length == 0 {
				panic("los: pattern matched empty input")
			}
			if index > 0 && !yield(m.buildPattern(entry.tail, index, STATE_BODY, entry.value)) {
				return
			}
			if !ok {
				return
			}

			m.state = STATE_NONE
			if !yield(m.buildPattern(entry.tail, length, STATE_TAIL, entry.value)) {
				return
			}
			m.active = -1
			m.selector.Reset(m.consumed, m.context)
		}
	}
}

func (m *automationMatcher) Drain() string {
	buffer := m.buffer.String()
	m.reset()
	return buffer
}

func (m *automationMatcher) Close() error {
	m.selector.Close()
	for i := range m.pairs {
		m.pairs[i].tail.Clear()
	}
	if m.buffer.Len() > 0 {
		return ErrBufferNotDrained
	}
	return nil
}

func (m *automationMatcher) buildText(n int, state State, value any) Result {
	result := textResult{state: state, raw: m.buffer.Next(n), value: value}
	m.advance(result.raw)
	return result
}

func (m *automationMatcher) buildRegex(n int, state State, matchcap []int, value any) Result {
	result := regexResult{state: state, raw: m.buffer.Next(n), matchcap: matchcap, value: value}
	m.advance(result.raw)
	return result
}

func (m *automationMatcher) buildPattern(pat pattern, n int, state State, value any) Result {
	result := pat.Build(m.buffer, n, state, value)
	m.advance(result.Raw())
	return result
}

func (m *automationMatcher) advance(raw []byte) {
	m.context = m.context.Advance(raw)
	m.consumed += int64(len(raw))
}

func (m *automationMatcher) reset() {
	m.state = STATE_NONE
	m.active = -1
	m.tailIndex = 0
	m.tailOffset = 0
	m.headFed = 0
	m.consumed = 0
	m.buffer.Reset()
	m.context = legex.NewStreamContext()
	m.selector.Reset(0, m.context)
	for i := range m.pairs {
		m.pairs[i].tail.Reset()
	}
}

type headSelector struct {
	fixed *acHead
	regex []*regexHead
	end   int64
}

func (s *headSelector) Feed(input []byte) {
	if len(input) == 0 {
		return
	}
	if s.fixed != nil {
		s.fixed.Feed(input)
	}
	for _, head := range s.regex {
		head.Feed(input)
	}
	s.end += int64(len(input))
}

func (s *headSelector) Finish() {
	for _, head := range s.regex {
		head.Finish()
	}
}

func (s *headSelector) Decide(finished bool) (headCandidate, int64, bool) {
	// A completed candidate cannot win while another engine can still produce
	// an earlier start or a higher-priority match at the same start.
	var winner headCandidate
	hasWinner := false
	considerCandidate := func(candidate headCandidate, ok bool) {
		if ok && (!hasWinner || candidateBefore(candidate, winner)) {
			winner = candidate
			hasWinner = true
		}
	}
	if s.fixed != nil {
		considerCandidate(s.fixed.candidate, s.fixed.hasCandidate)
	}
	for _, head := range s.regex {
		considerCandidate(head.candidate, head.hasCandidate)
	}

	safe := s.end
	hasContender := false
	considerStart := func(start int64) {
		if !hasContender || start < safe {
			safe = start
			hasContender = true
		}
	}
	if hasWinner {
		considerStart(winner.start)
	}
	if s.fixed != nil && s.fixed.hasCandidate {
		considerStart(s.fixed.candidate.start)
	}
	for _, head := range s.regex {
		if head.hasCandidate {
			considerStart(head.candidate.start)
		}
	}

	blocked := false
	considerPending := func(pending headPending) {
		if !pending.ok || finished {
			return
		}
		considerStart(pending.start)
		if hasWinner && (pending.start < winner.start || pending.start == winner.start && pending.pair < winner.pair) {
			blocked = true
		}
	}
	if s.fixed != nil {
		considerPending(s.fixed.Pending())
	}
	for _, head := range s.regex {
		considerPending(head.Pending())
	}

	if hasWinner && !blocked {
		return winner, safe, true
	}
	if !hasContender {
		safe = s.end
	}
	return headCandidate{}, safe, false
}

func (s *headSelector) Reset(base int64, ctx legex.StreamContext) {
	s.end = base
	if s.fixed != nil {
		s.fixed.Reset(base)
	}
	for _, head := range s.regex {
		head.Reset(base, ctx)
	}
}

func (s *headSelector) Close() {
	for _, head := range s.regex {
		head.pattern.Clear()
	}
}

func candidateBefore(left, right headCandidate) bool {
	return left.start < right.start || left.start == right.start && left.pair < right.pair
}

type regexHead struct {
	pair         int
	pattern      *regexPattern
	buffer       bytes.Buffer
	context      legex.StreamContext
	base         int64
	candidate    headCandidate
	hasCandidate bool
	finished     bool
}

func newRegexHead(pair int, expr string, mode regexMode) *regexHead {
	return &regexHead{pair: pair, pattern: newRegexPattern(expr, mode)}
}

func (h *regexHead) Feed(input []byte) {
	if h.hasCandidate || h.finished {
		return
	}
	h.buffer.Write(input)
	index, length, ok := h.pattern.Match(h.context, 0, 0, h.buffer.Bytes())
	if ok {
		h.storeCandidate(index, length)
		return
	}
	if index > 0 {
		h.context = h.context.Advance(h.buffer.Next(index))
		h.base += int64(index)
	}
}

func (h *regexHead) Finish() {
	if h.hasCandidate || h.finished {
		return
	}
	index, length, ok := h.pattern.Finish(h.context, h.buffer.Bytes())
	if ok {
		h.storeCandidate(index, length)
		return
	}
	h.finished = true
}

func (h *regexHead) Pending() headPending {
	return headPending{start: h.base, pair: h.pair, ok: !h.hasCandidate && !h.finished}
}

func (h *regexHead) Reset(base int64, ctx legex.StreamContext) {
	h.buffer.Reset()
	h.context = ctx
	h.base = base
	h.candidate = headCandidate{}
	h.hasCandidate = false
	h.finished = false
	h.pattern.Reset()
}

func (h *regexHead) storeCandidate(index, length int) {
	if length == 0 {
		panic("los: pattern matched empty input")
	}
	matchcap := h.pattern.MatchCap()
	for i, position := range matchcap {
		if position >= 0 {
			matchcap[i] = position - index
		}
	}
	h.candidate = headCandidate{
		start:    h.base + int64(index),
		end:      h.base + int64(index+length),
		pair:     h.pair,
		matchcap: matchcap,
	}
	h.hasCandidate = true
	h.pattern.Reset()
	h.buffer.Reset()
}

type acPattern struct {
	pair int
	text string
}

type acHead struct {
	nodes        []acNode
	lengths      map[int]int
	state        int
	position     int64
	candidate    headCandidate
	hasCandidate bool
}

type acNode struct {
	next   map[byte]int
	fail   int
	depth  int
	own    []int
	output []int
	// minFuture is the lowest pair reachable after consuming another byte.
	minFuture int
}

func newAcHead(patterns []acPattern) *acHead {
	head := &acHead{
		nodes:   []acNode{{next: make(map[byte]int), minFuture: -1}},
		lengths: make(map[int]int, len(patterns)),
	}
	for _, pattern := range patterns {
		state := 0
		for _, b := range []byte(pattern.text) {
			next, ok := head.nodes[state].next[b]
			if !ok {
				next = len(head.nodes)
				head.nodes = append(head.nodes, acNode{
					next:      make(map[byte]int),
					depth:     head.nodes[state].depth + 1,
					minFuture: -1,
				})
				head.nodes[state].next[b] = next
			}
			state = next
		}
		head.nodes[state].own = append(head.nodes[state].own, pattern.pair)
		head.lengths[pattern.pair] = len(pattern.text)
	}

	queue := make([]int, 0, len(head.nodes))
	for _, child := range head.nodes[0].next {
		queue = append(queue, child)
	}
	for len(queue) > 0 {
		state := queue[0]
		queue = queue[1:]
		head.nodes[state].output = append(head.nodes[state].output, head.nodes[state].own...)
		head.nodes[state].output = append(head.nodes[state].output, head.nodes[head.nodes[state].fail].output...)
		sort.Ints(head.nodes[state].output)
		for b, child := range head.nodes[state].next {
			fail := head.nodes[state].fail
			for fail != 0 {
				if _, ok := head.nodes[fail].next[b]; ok {
					break
				}
				fail = head.nodes[fail].fail
			}
			if target, ok := head.nodes[fail].next[b]; ok && target != child {
				head.nodes[child].fail = target
			}
			queue = append(queue, child)
		}
	}
	head.nodes[0].output = append(head.nodes[0].output, head.nodes[0].own...)

	var computeFuture func(int) int
	computeFuture = func(state int) int {
		includingSelf := -1
		for _, pair := range head.nodes[state].own {
			includingSelf = minPair(includingSelf, pair)
		}
		future := -1
		for _, child := range head.nodes[state].next {
			childMin := computeFuture(child)
			future = minPair(future, childMin)
			includingSelf = minPair(includingSelf, childMin)
		}
		head.nodes[state].minFuture = future
		return includingSelf
	}
	computeFuture(0)
	return head
}

func (h *acHead) Feed(input []byte) {
	for _, b := range input {
		for h.state != 0 {
			if _, ok := h.nodes[h.state].next[b]; ok {
				break
			}
			h.state = h.nodes[h.state].fail
		}
		if next, ok := h.nodes[h.state].next[b]; ok {
			h.state = next
		}
		h.position++
		for _, pair := range h.nodes[h.state].output {
			candidate := headCandidate{
				start: h.position - int64(h.lengths[pair]),
				end:   h.position,
				pair:  pair,
			}
			if !h.hasCandidate || candidateBefore(candidate, h.candidate) {
				h.candidate = candidate
				h.hasCandidate = true
			}
		}
	}
}

func (h *acHead) Pending() headPending {
	pending := headPending{}
	for state := h.state; ; state = h.nodes[state].fail {
		pair := h.nodes[state].minFuture
		if pair >= 0 {
			candidate := headPending{
				start: h.position - int64(h.nodes[state].depth),
				pair:  pair,
				ok:    true,
			}
			if !pending.ok || candidate.start < pending.start || candidate.start == pending.start && candidate.pair < pending.pair {
				pending = candidate
			}
		}
		if state == 0 {
			break
		}
	}
	return pending
}

func (h *acHead) Reset(base int64) {
	h.state = 0
	h.position = base
	h.candidate = headCandidate{}
	h.hasCandidate = false
}

func minPair(current, candidate int) int {
	if current < 0 || candidate < current {
		return candidate
	}
	return current
}
