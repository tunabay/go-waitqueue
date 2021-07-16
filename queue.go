// Copyright (c) 2021 Hirotsuna Mizuno. All rights reserved.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package waitqueue

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Queue is a FIFO queue in which go-routines can be queued and blocked.
type Queue struct {
	line            *line
	gate            *gateSet
	interval        time.Duration
	intervalPerGate time.Duration
	paused          bool
	cordonedOff     error
	cond            *sync.Cond
	mu              sync.Mutex
}

// New creates a new Queue with the default configuration, with a single exit
// gate and zero interval durations.
func New() *Queue {
	q, err := NewWithConfig(nil)
	if err != nil {
		panic(fmt.Sprintf("invalid default conf: %s", err))
	}

	return q
}

// NewWithConfig creates a new Queue with the specified configuration. If conf
// is nil, the default configuration will be used.
func NewWithConfig(conf *Config) (*Queue, error) {
	if conf == nil {
		conf = &Config{}
	}
	numGates := conf.NumGates
	switch {
	case numGates == 0:
		numGates = 1
	case numGates < 0:
		return nil, fmt.Errorf("%w: negative NumGates %d", ErrInvalidConfig, numGates)
	}
	if conf.Interval < 0 {
		return nil, fmt.Errorf("%w: negative Interval %v", ErrInvalidConfig, conf.Interval)
	}
	if conf.IntervalPerGate < 0 {
		return nil, fmt.Errorf("%w: negative IntervalPerGate %v", ErrInvalidConfig, conf.IntervalPerGate)
	}
	q := &Queue{
		line:            &line{},
		gate:            newGateSet(numGates),
		interval:        conf.Interval,
		intervalPerGate: conf.IntervalPerGate,
		paused:          conf.Paused,
		cordonedOff:     conf.CordonedOff,
	}
	q.cond = sync.NewCond(&q.mu)

	return q, nil
}

// Wait appends the calling go-routine to the end of the queue and blocks until
// its turn. When it is its turn, it leaves the queue, passes through one of
// exit gates that can currently accept a new entry, and then returns.
// It respects Config.Interval when leaving the queue, and
// Config.IntervalPerGate when enters an exit gate.
func (q *Queue) Wait(ctx context.Context) error {
	return q.WaitWithTask(ctx, nil)
}

// WaitWithTask is the same as Wait, except that it executes task() while
// occupying an exit gate. In other words, it occupies an exit gate until the
// task completes or canceled, and the next entry waits for it.
func (q *Queue) WaitWithTask(ctx context.Context, task TaskFunc) error {
	q.mu.Lock()
	if err := q.cordonedOff; err != nil {
		q.mu.Unlock()
		return err
	}
	q.mu.Unlock()
	if ctx.Err() != nil {
		return &CanceledError{err: ctx.Err(), inQueue: true}
	}
	q.mu.Lock()
	e, isFirst := q.line.add()
	if isFirst {
		go q.serve(q.line.sid)
	}
	q.cond.Broadcast()
	q.mu.Unlock()

	select {
	case <-e.c:
	case <-ctx.Done():
		q.mu.Lock()
		if !e.done {
			q.line.remove(e)
			close(e.c)
		}
		q.cond.Broadcast()
		q.mu.Unlock()

		return &CanceledError{err: ctx.Err(), inQueue: true}
	}

	var taskErr error
	if task != nil {
		if err := task(ctx, e.g.id); err != nil {
			var canceledErr *CanceledError
			if errors.As(err, &canceledErr) {
				taskErr = canceledErr
			} else {
				taskErr = &TaskError{err: err}
			}
		}
	}

	q.mu.Lock()
	e.g.waitUntil = time.Now().Add(q.intervalPerGate)
	// fmt.Printf("gate %d freed (wait until %s)\n", e.g.id, e.g.waitUntil.Round(time.Second/10).Format("15:04:05.0"))
	q.gate.add(e.g)
	q.cond.Broadcast()
	q.mu.Unlock()

	return taskErr
}

// Len returns the current length of the queue. Only entries waiting in the
// queue are counted. Entries that have already left the queue are not included
// in the count. It also does not include entries that are executing TaskFunc
// within an exit gate.
func (q *Queue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.line.n
}

// Length is identical to Len, deprecated.
//
// Deprecated: Use Len instead.
func (q *Queue) Length() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.line.n
}

// Pause pauses the queue. All waiting go-routines continue to wait
// unconditionally until the queue is resumed. Call Resume to resume the queue.
// Calling Wait or WaitWithTask while paused is allowed. Duplicate calling
// Pause while already paused has no effect.
func (q *Queue) Pause() {
	q.mu.Lock()
	q.paused = true
	q.mu.Unlock()
}

// Resume resumes the paused queue. All waiting go-routines go through again
// in order. Calling Resume when not in a paused state has no effect.
func (q *Queue) Resume() {
	q.mu.Lock()
	if q.paused {
		q.paused = false
		q.cond.Broadcast()
	}
	q.mu.Unlock()
}

// IsPaused reports whether q is currently paused.
func (q *Queue) IsPaused() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.paused
}

// Cordon temporarily blocks new entry into the queue. All new Wait and
// WaitWithTask calls fail immediately and return ErrCordonedOff. Calls that
// have already been started and are waiting for their turn in the queue are not
// affected or canceled, and continue to wait. In other words, this blocks the
// entrance to the queue. Call Uncordon to release this state. Duplicate Cordon
// calls have basically no effect, but if an error is set by CordonWithError,
// Cordon overwrites it with the default ErrCordonedOff.
func (q *Queue) Cordon() {
	q.CordonWithError(nil)
}

// CordonWithError is identical to Cordon except that it uses the provided err
// instead of the default ErrCordonedOff. If err is nil, it uses the default.
// Duplicate calls always updates the error.
func (q *Queue) CordonWithError(err error) {
	if err == nil {
		err = ErrCordonedOff
	}
	q.mu.Lock()
	q.cordonedOff = err
	q.mu.Unlock()
}

// Uncordon releases the cordoned off state of the queue. After this call, Wait
// and WaitWithTask calls return to normal operation.
func (q *Queue) Uncordon() {
	q.mu.Lock()
	q.cordonedOff = nil
	q.mu.Unlock()
}

// IsCordonedOff reports whether q is currently cordoned off.
func (q *Queue) IsCordonedOff() bool {
	return q.CordonedOffErr() != nil
}

// CordonedOffErr returns the cause error if the queue is cordoned off, nil
// otherwise.
func (q *Queue) CordonedOffErr() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.cordonedOff
}

func (q *Queue) serve(sid int) {
	// fmt.Printf("serve(%d) started.\n", sid)
	// defer fmt.Printf("serve(%d) finished.\n", sid)
	for {
		q.cond.L.Lock()
		var timer *time.Timer
		timerC := make(chan struct{})
		for {
			if q.line.n == 0 || q.line.sid != sid {
				q.cond.L.Unlock()
				close(timerC)
				return
			}
			if q.paused {
				q.cond.Wait()
				continue
			}
			gOK, gUntil := q.gate.check()
			if !gOK {
				q.cond.Wait()
				continue
			}
			wUntil := q.line.waitUntil
			if gUntil.After(wUntil) {
				wUntil = gUntil
			}
			if wd := time.Until(wUntil); 0 < wd {
				if timer == nil {
					timer = time.NewTimer(wd)
					go func(t *time.Timer) {
						select {
						case <-timerC:
							if !t.Stop() {
								<-t.C
							}
						case <-t.C:
							q.cond.L.Lock() // ensure it has reached q.cond.Wait() below
							q.cond.Broadcast()
							q.cond.L.Unlock()
						}
					}(timer)
				}
				q.cond.Wait()
				continue
			}
			break
		}
		close(timerC)
		e := q.line.pop()
		q.line.waitUntil = time.Now().Add(q.interval)
		e.g = q.gate.get()
		close(e.c)
		q.cond.L.Unlock()
	}
}
