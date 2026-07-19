// Package legex derived from `regexp`
package legex

import (
	"sync"
	"unicode/utf8"
)

// Pools of *machineDefault for use when calling (*Regexp).Get,
// split up by the size of the execution queues. defaultPool[i]
// machines have queue size defaultSize[i]. On a 64-bit system
// each queue entry is 16 bytes, so defaultPool[0] has
// 16*2*128 = 4kB queues, etc. The final defaultPool is a
// catch-all for very large queues.
var (
	// Sync Pool for Default Machine
	defaultSize = [...]int{128, 512, 2048, 16384, 0}
	defaultPool [len(defaultSize)]sync.Pool
)

type Machine interface {
	// Reset clears the current match state without changing stream context.
	Reset()
	// Close returns the machine resources to the pool.
	Close()
	// MatchCap returns a copy of the most recent submatch positions.
	MatchCap() []int
	// Match searches the retained stream buffer. When no match is complete,
	// index is the number of bytes safe to release and length is the number
	// of bytes that must remain buffered.
	Match(ctx StreamContext, buf []byte) (index int, length int, ok bool)
	// Finish performs the final search with a real end-of-text boundary.
	Finish(ctx StreamContext, buf []byte) (index int, length int, ok bool)
}

// StreamContext describes the text immediately before a retained stream
// buffer. Its fields are intentionally private so callers advance it only
// through consumed bytes.
type StreamContext struct {
	begin    bool
	previous rune
}

func NewStreamContext() StreamContext {
	return StreamContext{begin: true, previous: endOfText}
}

func (ctx StreamContext) Advance(consumed []byte) StreamContext {
	if len(consumed) == 0 {
		return ctx
	}
	ctx.begin = false
	ctx.previous, _ = utf8.DecodeLastRune(consumed)
	return ctx
}

func (re *Regexp) Get() Machine {
	numCap := re.matchcap

	// Use Default Machine
	m, ok := defaultPool[re.mpool].Get().(*machineDefault)
	if !ok {
		m = new(machineDefault)
	}
	m.re, m.p = re, re.prog
	if cap(m.matchcap) < re.matchcap {
		m.matchcap = make([]int, re.matchcap)
		for _, t := range m.pool {
			t.cap = make([]int, re.matchcap)
		}
	}
	for _, t := range m.pool {
		t.cap = t.cap[:numCap]
	}
	m.matchcap = m.matchcap[:numCap]
	// Allocate queues if needed.
	// Or reallocate, for "large" match pool.
	n := defaultSize[re.mpool]
	if n == 0 { // large pool
		n = len(re.prog.Inst)
	}
	if len(m.q0.sparse) < n {
		m.q0 = queue{make([]uint32, n), make([]entry, 0, n)}
		m.q1 = queue{make([]uint32, n), make([]entry, 0, n)}
	}
	m.Reset()
	return m
}
