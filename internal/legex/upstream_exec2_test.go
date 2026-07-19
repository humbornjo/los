// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !race

package legex_test

import "testing"

func TestLegex_UpstreamRE2ExhaustiveStreaming(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping exhaustive RE2 corpus in short mode")
	}
	testUpstreamRE2Streaming(t, "testdata/re2-exhaustive.txt.bz2")
}
