// This file Contains modified code from the Go standard library
package regexp

import "slices"

func (r *Regexp) Get() *machine {
	m := r.get()
	m.inputs.init(nil, nil, "")
	return m
}

func (r *Regexp) Put(m *machine) {
	r.put(m)
}

func (m *machine) Match(s string) []int {
	i := m.inputs.extend(s)
	if !m.match(i, m.index) {
		return nil
	}
	return slices.Clone(m.matchcap)
}
