// Package legex derived from `regexp`
package legex

import "sync"

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
	// Reset reset the inner state of Virtual Machine, it should be
	// called after got a machine.
	Reset()
	// Close restore the Virtual Machine resource back to the pool
	// to improve memory allocation.
	Close()
	// Accum returns a pivot that records the total shift since
	// last match. It is used to calibrate the index in matchcap.
	Accum() int
	// MatchCap return a clone slice with matched positions.
	MatchCap() []int
	// Match try to match the compiled pattern against buf from
	// index and with a prior knowledge of already matched offset.
	Match(index int, offset int, buf []byte) (int, int, bool)
}

func (re *Regexp) Get() Machine {
	numCap := re.prog.NumCap

	// Use Default Machine
	m, ok := defaultPool[re.mpool].Get().(*machineDefault)
	if !ok {
		m = new(machineDefault)
	}
	m.re, m.p = re, re.prog
	m.accum, m.matched, m.strict = 0, false, re.strict
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
	return m
}
