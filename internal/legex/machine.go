package legex

import (
	"math"
	"regexp/syntax"
	"slices"
	"unicode/utf8"
)

const endOfChunk rune = utf8.MaxRune + 1

var _ Machine = (*machineDefault)(nil)

// machineDefault is a resumable adaptation of Go's regexp NFA machine. Each
// call replays the retained candidate bytes so matching stays independent of
// how the stream was chunked. Bytes that cannot begin a future match are
// released to the caller.
type machineDefault struct {
	re       *Regexp
	p        *syntax.Prog
	q0, q1   queue
	pool     []*thread
	matched  bool
	matchcap []int

	final    bool
	frontier int
}

func (m *machineDefault) Reset() {
	m.clear(&m.q0)
	m.clear(&m.q1)
	m.matched = false
	for i := range m.matchcap {
		m.matchcap[i] = -1
	}
}

func (m *machineDefault) Close() {
	pool := &defaultPool[m.re.mpool]
	m.clear(&m.q0)
	m.clear(&m.q1)
	m.re, m.p = nil, nil
	pool.Put(m)
}

func (m *machineDefault) MatchCap() []int {
	return slices.Clone(m.matchcap)
}

func (m *machineDefault) Match(ctx StreamContext, buf []byte) (int, int, bool) {
	return m.execute(ctx, buf, false)
}

func (m *machineDefault) Finish(ctx StreamContext, buf []byte) (int, int, bool) {
	return m.execute(ctx, buf, true)
}

func (m *machineDefault) execute(ctx StreamContext, buf []byte, final bool) (int, int, bool) {
	limit := 0
	for limit < len(buf) && (final || utf8.FullRune(buf[limit:])) {
		_, width := utf8.DecodeRune(buf[limit:])
		limit += width
	}

	m.clear(&m.q0)
	m.clear(&m.q1)
	m.matched = false
	for i := range m.matchcap {
		m.matchcap[i] = -1
	}
	m.final = final
	m.frontier = math.MaxInt

	m.match(ctx, &inputBytes{str: buf[:limit]})
	if m.matched {
		return m.matchcap[0], m.matchcap[1] - m.matchcap[0], true
	}

	if final || m.frontier == math.MaxInt {
		m.frontier = limit
	}
	return m.frontier, len(buf) - m.frontier, false
}

// match follows the standard library's ordered NFA execution. At the current
// end of available input it also records the earliest live thread, which is
// the first byte that must remain buffered for the next stream chunk.
func (m *machineDefault) match(ctx StreamContext, i *inputBytes) {
	startCond := m.re.cond
	if startCond == ^syntax.EmptyOp(0) {
		return
	}

	runq, nextq := &m.q0, &m.q1
	boundary := endOfChunk
	if m.final {
		boundary = endOfText
	}
	pos := 0
	r, r1 := boundary, boundary
	width, width1 := 0, 0
	r, width = i.step(pos)
	if r != endOfText {
		r1, width1 = i.step(pos + width)
		if r1 == endOfText {
			r1 = boundary
		}
	} else {
		r = boundary
	}
	flag := newLazyFlag(ctx.previous, r)
	if ctx.begin {
		flag = newLazyFlag(endOfText, r)
	}

	for {
		if len(runq.dense) == 0 {
			if startCond&syntax.EmptyBeginText != 0 && (!ctx.begin || pos != 0) {
				break
			}
			if m.matched {
				break
			}
		}
		if !m.matched {
			if len(m.matchcap) > 0 {
				m.matchcap[0] = pos
			}
			m.add(runq, uint32(m.p.Start), pos, m.matchcap, &flag, nil)
		}
		if width == 0 {
			for _, entry := range runq.dense {
				if entry.t != nil && len(entry.t.cap) > 0 {
					m.frontier = min(m.frontier, entry.t.cap[0])
				}
			}
			if m.final {
				flag = newLazyFlag(r, r1)
				m.step(runq, nextq, pos, pos, endOfText, &flag)
			} else {
				m.acceptSoftBoundary(runq, pos)
			}
			break
		}
		flag = newLazyFlag(r, r1)
		m.step(runq, nextq, pos, pos+width, r, &flag)
		if len(m.matchcap) == 0 && m.matched {
			break
		}
		pos += width
		r, width = r1, width1
		if width == 0 {
			r = boundary
		}
		r1, width1 = boundary, 0
		if width > 0 {
			r1, width1 = i.step(pos + width)
			if r1 == endOfText {
				r1 = boundary
			}
		}
		runq, nextq = nextq, runq
	}
	m.clear(runq)
	m.clear(nextq)
}

func (m *machineDefault) acceptSoftBoundary(q *queue, pos int) {
	for j, entry := range q.dense {
		if entry.t == nil {
			continue
		}
		if entry.t.inst.Op != syntax.InstMatch {
			m.matched = false
			return
		}
		if m.re.longest {
			start := entry.t.cap[0]
			for _, extension := range q.dense[j+1:] {
				if extension.t != nil && extension.t.inst.Op != syntax.InstMatch &&
					len(extension.t.cap) > 0 && extension.t.cap[0] == start {
					m.matched = false
					return
				}
			}
		}
		entry.t.cap[1] = pos
		copy(m.matchcap, entry.t.cap)
		m.matched = true
		return
	}
}

func (m *machineDefault) clear(q *queue) {
	for _, entry := range q.dense {
		if entry.t != nil {
			m.pool = append(m.pool, entry.t)
		}
	}
	q.dense = q.dense[:0]
}

// step mirrors regexp.machine.step. Queue order preserves Perl leftmost-first
// priority; POSIX mode continues lower-priority threads to find the longest
// leftmost match.
func (m *machineDefault) step(runq, nextq *queue, pos, nextPos int, c rune, nextCond *lazyFlag) {
	longest := m.re.longest
	for j := 0; j < len(runq.dense); j++ {
		entry := &runq.dense[j]
		t := entry.t
		if t == nil {
			continue
		}
		if longest && m.matched && len(t.cap) > 0 && m.matchcap[0] < t.cap[0] {
			m.pool = append(m.pool, t)
			continue
		}

		inst := t.inst
		add := false
		switch inst.Op {
		default:
			panic("bad inst")
		case syntax.InstMatch:
			if len(t.cap) > 0 && (!longest || !m.matched || m.matchcap[1] < pos) {
				t.cap[1] = pos
				copy(m.matchcap, t.cap)
			}
			if !longest {
				for _, lower := range runq.dense[j+1:] {
					if lower.t != nil {
						m.pool = append(m.pool, lower.t)
					}
				}
				runq.dense = runq.dense[:0]
			}
			m.matched = true
		case syntax.InstRune:
			add = inst.MatchRune(c)
		case syntax.InstRune1:
			add = c == inst.Rune[0]
		case syntax.InstRuneAny:
			add = true
		case syntax.InstRuneAnyNotNL:
			add = c != '\n'
		}
		if add {
			t = m.add(nextq, inst.Out, nextPos, t.cap, nextCond, t)
		}
		if t != nil {
			m.pool = append(m.pool, t)
		}
	}
	runq.dense = runq.dense[:0]
}

// add mirrors regexp.machine.add. It follows epsilon transitions immediately
// and adds at most one thread for each instruction while preserving priority.
func (m *machineDefault) add(q *queue, pc uint32, pos int, cap []int, cond *lazyFlag, t *thread) *thread {
again:
	if pc == 0 {
		return t
	}
	if j := q.sparse[pc]; j < uint32(len(q.dense)) && q.dense[j].pc == pc {
		return t
	}

	j := len(q.dense)
	q.dense = q.dense[:j+1]
	entry := &q.dense[j]
	entry.t = nil
	entry.pc = pc
	q.sparse[pc] = uint32(j)

	inst := &m.p.Inst[pc]
	switch inst.Op {
	default:
		panic("unhandled")
	case syntax.InstFail:
	case syntax.InstAlt, syntax.InstAltMatch:
		t = m.add(q, inst.Out, pos, cap, cond, t)
		pc = inst.Arg
		goto again
	case syntax.InstEmptyWidth:
		op := syntax.EmptyOp(inst.Arg)
		if !m.final && rune(*cond) == endOfChunk &&
			op&(syntax.EmptyEndLine|syntax.EmptyEndText|syntax.EmptyWordBoundary|syntax.EmptyNoWordBoundary) != 0 {
			if t == nil {
				t = m.alloc(inst)
			} else {
				t.inst = inst
			}
			if len(cap) > 0 && &t.cap[0] != &cap[0] {
				copy(t.cap, cap)
			}
			entry.t = t
			t = nil
		} else if cond.match(op) {
			pc = inst.Out
			goto again
		}
	case syntax.InstNop:
		pc = inst.Out
		goto again
	case syntax.InstCapture:
		if int(inst.Arg) < len(cap) {
			old := cap[inst.Arg]
			cap[inst.Arg] = pos
			m.add(q, inst.Out, pos, cap, cond, nil)
			cap[inst.Arg] = old
		} else {
			pc = inst.Out
			goto again
		}
	case syntax.InstMatch, syntax.InstRune, syntax.InstRune1, syntax.InstRuneAny, syntax.InstRuneAnyNotNL:
		if t == nil {
			t = m.alloc(inst)
		} else {
			t.inst = inst
		}
		if len(cap) > 0 && &t.cap[0] != &cap[0] {
			copy(t.cap, cap)
		}
		entry.t = t
		t = nil
	}
	return t
}

type queue struct {
	sparse []uint32
	dense  []entry
}

type entry struct {
	pc uint32
	t  *thread
}

type thread struct {
	inst *syntax.Inst
	cap  []int
}

func (m *machineDefault) alloc(inst *syntax.Inst) *thread {
	var t *thread
	if n := len(m.pool); n > 0 {
		t = m.pool[n-1]
		m.pool = m.pool[:n-1]
	} else {
		t = new(thread)
		t.cap = make([]int, len(m.matchcap), cap(m.matchcap))
	}
	t.inst = inst
	return t
}

type lazyFlag uint64

func newLazyFlag(r1, r2 rune) lazyFlag {
	return lazyFlag(uint64(r1)<<32 | uint64(uint32(r2)))
}

func (f lazyFlag) match(op syntax.EmptyOp) bool {
	if op == 0 {
		return true
	}
	r1 := rune(f >> 32)
	if op&syntax.EmptyBeginLine != 0 {
		if r1 != '\n' && r1 >= 0 {
			return false
		}
		op &^= syntax.EmptyBeginLine
	}
	if op&syntax.EmptyBeginText != 0 {
		if r1 >= 0 {
			return false
		}
		op &^= syntax.EmptyBeginText
	}
	if op == 0 {
		return true
	}
	r2 := rune(f)
	if op&syntax.EmptyEndLine != 0 {
		if r2 != '\n' && r2 >= 0 {
			return false
		}
		op &^= syntax.EmptyEndLine
	}
	if op&syntax.EmptyEndText != 0 {
		if r2 >= 0 {
			return false
		}
		op &^= syntax.EmptyEndText
	}
	if op == 0 {
		return true
	}
	if syntax.IsWordChar(r1) != syntax.IsWordChar(r2) {
		op &^= syntax.EmptyWordBoundary
	} else {
		op &^= syntax.EmptyNoWordBoundary
	}
	return op == 0
}
