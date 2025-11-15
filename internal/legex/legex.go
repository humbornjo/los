// This file Contains modified code from the Go standard library
package legex

func (re *Regexp) Get() *Machine {
	m, ok := matchPool[re.mpool].Get().(*Machine)
	if !ok {
		m = new(Machine)
	}
	m.re = re
	m.accum = 0
	m.matched = false
	m.p = re.prog
	if cap(m.matchcap) < re.matchcap {
		m.matchcap = make([]int, re.matchcap)
		for _, t := range m.pool {
			t.cap = make([]int, re.matchcap)
		}
	}

	for _, t := range m.pool {
		t.cap = t.cap[:m.p.NumCap]
	}
	m.matchcap = m.matchcap[:m.p.NumCap]

	// Allocate queues if needed.
	// Or reallocate, for "large" match pool.
	n := matchSize[re.mpool]
	if n == 0 { // large pool
		n = len(re.prog.Inst)
	}
	if len(m.q0.sparse) < n {
		m.q0 = queue{make([]uint32, n), make([]entry, 0, n)}
		m.q1 = queue{make([]uint32, n), make([]entry, 0, n)}
	}

	return m
}

func (re *Regexp) Put(m *Machine) {
	m.clear(&m.q0)
	m.clear(&m.q1)
	m.re, m.p = nil, nil
	matchPool[re.mpool].Put(m)
}
