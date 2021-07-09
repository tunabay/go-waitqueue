// Copyright (c) 2021 Hirotsuna Mizuno. All rights reserved.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package waitqueue

import (
	"time"
)

type gateSet struct {
	g           []*gate
	next, empty int
}

type gate struct {
	id        int // [0, NumGates)
	waitUntil time.Time
}

func newGateSet(numGates int) *gateSet {
	switch {
	case numGates < 1:
		panic("invalid numGates")
	case numGates+1 < 0: // n + 1 should not overflow
		panic("numGates overflow")
	}
	gs := &gateSet{
		g: make([]*gate, numGates+1),
	}
	for i := 0; i < numGates; i++ {
		gs.g[i] = &gate{id: i}
	}
	gs.empty = numGates

	return gs
}

func (gs *gateSet) check() (bool, time.Time) {
	if gs.next == gs.empty {
		return false, time.Time{}
	}

	return true, gs.g[gs.next].waitUntil
}

func (gs *gateSet) get() *gate {
	if gs.next == gs.empty {
		return nil
	}
	g := gs.g[gs.next]
	gs.next = (gs.next + 1) % len(gs.g)

	return g
}

func (gs *gateSet) add(g *gate) {
	gs.g[gs.empty] = g
	gs.empty = (gs.empty + 1) % len(gs.g)
}
