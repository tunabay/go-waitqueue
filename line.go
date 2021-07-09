// Copyright (c) 2021 Hirotsuna Mizuno. All rights reserved.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package waitqueue

import (
	"time"
)

type line struct {
	head, tail *entry
	n          int
	sid        int // non-empty sequence id
	waitUntil  time.Time
}

type entry struct {
	c          chan struct{}
	g          *gate
	prev, next *entry
	done       bool
}

func (l *line) add() (*entry, bool) {
	isFirst := false
	e := &entry{c: make(chan struct{})}
	if l.head == nil {
		l.head = e
		l.tail = e
		l.sid++
		isFirst = true
	} else {
		l.tail.next = e
		e.prev = l.tail
		l.tail = e
	}
	l.n++

	return e, isFirst
}

func (l *line) pop() *entry {
	e := l.head
	l.head = e.next
	if l.head == nil {
		l.tail = nil
	} else {
		l.head.prev = nil
	}
	l.n--

	e.done = true

	return e
}

func (l *line) remove(e *entry) {
	if e.prev == nil {
		l.head = e.next
	} else {
		e.prev.next = e.next
	}
	if e.next == nil {
		l.tail = e.prev
	} else {
		e.next.prev = e.prev
	}
	l.n--
}
