package legex

import (
	"bytes"
	"math"
	"regexp/syntax"
)

func (m *Machine) Match(index int, offset int, buf []byte) (int, int, bool) {
	input := &inputBytes{bytes.NewBuffer(buf)}
	// Machine will continue to match from index+offset, where the previous match stopped
	//
	// INFO: If match the full pattern,
	// - true boolean value will be returned.
	// - offset will be the length of the pattern matched.
	// - content in buf before index will be the out-of-pattern string.
	//
	// INFO: If not match the full pattern,
	// - false boolean value will be returned.
	// - content in buf before index will be the out-of-pattern string.
	// - machine will remember the new index, if the index changed in the next match, the collected match index will be
	//   decreased by the difference as well.
	idx, off, ok := m.match(input, index, offset)

	if !ok {
		shift := math.MaxInt
		for _, e := range m.q0.dense {
			if e.t != nil {
				shift = min(shift, e.t.cap[0]-m.accum)
			}
		}
		if shift == math.MaxInt {
			m.accum += idx
			return idx, off, false
		}
		m.accum += shift
		return index + shift, len(buf) - (index + shift), false
	}
	m.accum = 0
	m.matched = false
	return m.matchcap[0], m.matchcap[1] - m.matchcap[0], true
}

func (m *Machine) Reset() {
	m.clear(&m.q0)
	m.clear(&m.q1)
}

// A queue is a 'sparse array' holding pending threads of execution.
// See https://research.swtch.com/2008/03/using-uninitialized-memory-for-fun-and.html
type queue struct {
	sparse []uint32
	dense  []entry
}

// An entry is an entry on a queue.
// It holds both the instruction pc and the actual thread.
// Some queue entries are just place holders so that the machine
// knows it has considered that pc. Such entries have t == nil.
type entry struct {
	pc uint32
	t  *thread
}

// A thread is the state of a single path through the machine:
// an instruction and a corresponding capture array.
// See https://swtch.com/~rsc/regexp/regexp2.html
type thread struct {
	inst *syntax.Inst
	cap  []int
}

// A Machine holds all the state during an NFA simulation for p.
type Machine struct {
	re       *Regexp      // corresponding Regexp
	p        *syntax.Prog // compiled program
	q0, q1   queue        // two queues for runq, nextq
	pool     []*thread    // pool of available threads
	matched  bool         // whether a match was found
	matchcap []int        // capture information for the match

	accum int
}

func (m *Machine) init(ncap int) {
	for _, t := range m.pool {
		t.cap = t.cap[:ncap]
	}
	m.matchcap = m.matchcap[:ncap]
}

// alloc allocates a new thread with the given instruction.
// It uses the free pool if possible.
func (m *Machine) alloc(i *syntax.Inst) *thread {
	var t *thread
	if n := len(m.pool); n > 0 {
		t = m.pool[n-1]
		m.pool = m.pool[:n-1]
	} else {
		t = new(thread)
		t.cap = make([]int, len(m.matchcap), cap(m.matchcap))
	}
	t.inst = i
	return t
}

// match runs the machine over the input starting at pos.
// It reports whether a match was found.
// If so, m.matchcap holds the submatch information.
func (m *Machine) match(i input, index int, offset int) (int, int, bool) {
	startCond := m.re.cond

	// Start Op is InstFail startCond is ^EmptyOp(0)
	if startCond == ^syntax.EmptyOp(0) {
		return index, offset, false
	}

	// State reset is not needed since machine can be reused
	// m.matched = false
	// for i := range m.matchcap {
	// 	m.matchcap[i] = -1
	// }

	// This block is fine
	runq, nextq := &m.q0, &m.q1

	r, r1 := endOfText, endOfText // nolint: ineffassign
	width, width1 := 0, 0
	r, width = i.step(index + offset)
	if r != endOfText {
		r1, width1 = i.step(index + offset + width)
	}

	// Trying to figure out what flag is
	var flag lazyFlag
	if offset == 0 {
		flag = newLazyFlag(-1, r)
	} else {
		flag = i.context(index + offset)
	}

	for {
		// If the curr queue has no pending threads, then,
		//
		// 1. All thread failed
		// 2. Just start the first match
		//
		// Either way, we need to match from the beginning.
		//
		// INFO: Here will derive a change from the std lib. when
		// matching from the beginning, we always try to match the
		// full prefix before add any thread. So the logic here is
		// pretty easy, just record the position of the matching
		// progress against the prefix. If the prefix can be matched,
		// thread will be added to the queue so that the following
		// content can be matched.
		//
		// WARN: Currently this if branch wont work because onepass
		// is disabled. `m.re.prefix` is always empty.
		if len(runq.dense) == 0 {
			// What is needed here is a offset, which corresponds to
			// the one in the outie package los, indicating the matched
			// length from the match start point.
			//
			// E.g. with pattern "abc", if the match is "aab", then the
			// offset is 2. Since it match the "ab".

			// Have match; finished exploring alternatives.
			if m.matched {
				break
			}

			// m.add(runq, uint32(m.p.Start), index, m.matchcap, &flag, nil)

			// When prefix is already been matched, just goto weave
			if len(m.re.prefix) == 0 || offset == len(m.re.prefix) {
				goto weave // time to add some threads
			}
			index, offset := m.matchPrefix(i, index, offset)
			// TODO: advance r, width and r1, width1
			if offset == len(m.re.prefix) {
				goto weave // time to add some threads
			}

			// Dude you are so fucked, not even finish prefix matching. Maybe next time.
			return index, offset, false

			// INFO: useless block, we dont focus on pos here
			//
			// if startCond&syntax.EmptyBeginText != 0 && pos != 0 {
			// 	// Anchored match, past beginning of text.
			// 	break
			// }
		}

	weave: // When reaching here, sure its in the middle of matching.
		if !m.matched {
			if len(m.matchcap) > 0 {
				m.matchcap[0] = index + offset
			}
			m.add(runq, uint32(m.p.Start), index+offset, m.matchcap, &flag, nil)
		}
		flag = newLazyFlag(r, r1)
		if width == 0 {
			break
		}

		m.step(runq, nextq, index+offset, index+offset+width, r, &flag)
		offset += width
		if m.matched {
			// Found a match and not paying attention to where it is, so any match will do.
			break
		}
		runq, nextq = nextq, runq

		if len(runq.dense) == 0 {
			index, offset = index+offset, 0
			r, width = i.step(index)
			if r != endOfText {
				r1, width1 = i.step(index + width)
			}
			flag = newLazyFlag(-1, r)
			// m.add(runq, uint32(m.p.Start), index, m.matchcap, &flag, nil)
			continue
		}

		r, width = r1, width1
		if r != endOfText {
			r1, width1 = i.step(index + offset + width)
		}
	}

	m.q0, m.q1 = *runq, *nextq
	return index, offset, m.matched
}

func (m *Machine) matchPrefix(i input, index int, offset int) (int, int) {
	n0, n1 := len(m.re.prefix), len(i.inner())
	i0, i1 := offset, index+offset
	for i0 < n0 && i1 < n1 {
		if m.re.prefix[i0] != i.inner()[i1] {
			i0, i1 = 0, i1+1
			continue
		}
		i0, i1 = i0+1, i1+1
	}
	return i1 - i0, i0
}

// clear frees all threads on the thread queue.
func (m *Machine) clear(q *queue) {
	for _, d := range q.dense {
		if d.t != nil {
			m.pool = append(m.pool, d.t)
		}
	}
	q.dense = q.dense[:0]
}

// step executes one step of the machine, running each of the threads
// on runq and appending new threads to nextq.
// The step processes the rune c (which may be endOfText),
// which starts at position pos and ends at nextPos.
// nextCond gives the setting for the empty-width flags after c.
func (m *Machine) step(runq, nextq *queue, pos, nextPos int, c rune, nextCond *lazyFlag) {
	longest := m.re.longest
	for j := 0; j < len(runq.dense); j++ {
		d := &runq.dense[j]
		t := d.t
		if t == nil {
			continue
		}
		if longest && m.matched && len(t.cap) > 0 && m.matchcap[0] < t.cap[0] {
			m.pool = append(m.pool, t)
			continue
		}
		i := t.inst
		add := false
		switch i.Op {
		default:
			panic("bad inst")

		// case syntax.InstMatch:
		// 	if len(t.cap) > 0 && (!longest || !m.matched || m.matchcap[1] < pos) {
		// 		t.cap[1] = pos
		// 		copy(m.matchcap, t.cap)
		// 	}
		// 	if !longest {
		// 		// First-match mode: cut off all lower-priority threads.
		// 		for _, d := range runq.dense[j+1:] {
		// 			if d.t != nil {
		// 				m.pool = append(m.pool, d.t)
		// 			}
		// 		}
		// 		runq.dense = runq.dense[:0]
		// 	}
		// 	m.matched = true

		case syntax.InstRune:
			add = i.MatchRune(c)
		case syntax.InstRune1:
			add = c == i.Rune[0]
		case syntax.InstRuneAny:
			add = true
		case syntax.InstRuneAnyNotNL:
			add = c != '\n'
		}
		if add {
			t = m.add(nextq, i.Out, nextPos, t.cap, nextCond, t)
		}
		if t != nil {
			m.pool = append(m.pool, t)
		}
	}
	runq.dense = runq.dense[:0]
}

// add adds an entry to q for pc, unless the q already has such an entry.
// It also recursively adds an entry for all instructions reachable from pc by following
// empty-width conditions satisfied by cond.  pos gives the current position
// in the input.
func (m *Machine) add(q *queue, pc uint32, pos int, cap []int, cond *lazyFlag, t *thread) *thread {
again:
	if pc == 0 {
		return t
	}
	if j := q.sparse[pc]; j < uint32(len(q.dense)) && q.dense[j].pc == pc {
		return t
	}

	j := len(q.dense)
	q.dense = q.dense[:j+1]
	d := &q.dense[j]
	d.t = nil
	d.pc = pc
	q.sparse[pc] = uint32(j)

	i := &m.p.Inst[pc]
	switch i.Op {
	default:
		panic("unhandled")
	case syntax.InstFail:
		// nothing
	case syntax.InstAlt, syntax.InstAltMatch:
		t = m.add(q, i.Out, pos, cap, cond, t)
		pc = i.Arg
		goto again
	case syntax.InstEmptyWidth:
		if cond.match(syntax.EmptyOp(i.Arg)) {
			pc = i.Out
			goto again
		}
	case syntax.InstNop:
		pc = i.Out
		goto again
	case syntax.InstCapture:
		if int(i.Arg) < len(cap) {
			opos := cap[i.Arg]
			cap[i.Arg] = pos
			m.add(q, i.Out, pos, cap, cond, nil)
			cap[i.Arg] = opos
		} else {
			pc = i.Out
			goto again
		}
	case syntax.InstMatch:
		longest := m.re.longest
		if len(t.cap) > 0 && (!longest || !m.matched || m.matchcap[1] < pos) {
			t.cap[0], t.cap[1] = t.cap[0]-m.accum, pos
			copy(m.matchcap, t.cap)
		}
		if !longest {
			// First-match mode: cut off all lower-priority threads.
			for _, d := range q.dense[j+1:] {
				if d.t != nil {
					m.pool = append(m.pool, d.t)
				}
			}
			q.dense = q.dense[:0]
		}
		m.matched = true

	case syntax.InstRune, syntax.InstRune1, syntax.InstRuneAny, syntax.InstRuneAnyNotNL:
		if t == nil {
			t = m.alloc(i)
			copy(t.cap, cap)
		} else {
			t.inst = i
		}
		// if len(cap) > 0 && &t.cap[0] != &cap[0] {
		// 	copy(t.cap, cap)
		// }
		d.t = t
		t = nil
	}
	return t
}

// THE CODE BELOW RETAIN ----------------------------------------

// arrayNoInts is returned by doExecute match if nil dstCap is passed
// to it with ncap=0.
var arrayNoInts [0]int

// A lazyFlag is a lazily-evaluated syntax.EmptyOp,
// for checking zero-width flags like ^ $ \A \z \B \b.
// It records the pair of relevant runes and does not
// determine the implied flags until absolutely necessary
// (most of the time, that means never).
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
