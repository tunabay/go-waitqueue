// Copyright (c) 2021 Hirotsuna Mizuno. All rights reserved.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package waitqueue_test

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tunabay/go-waitqueue"
)

const testTimeout = time.Second * 30

func TestMain(m *testing.M) {
	rand.Seed(time.Now().UnixNano())
	m.Run()
}

func TestQueue_simple(t *testing.T) {
	const numEntries = 5000

	q := waitqueue.New()

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	var wg sync.WaitGroup
	for i := 0; i < numEntries; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			delay := time.Second / 100 * time.Duration(rand.Intn(100))
			time.Sleep(delay)
			if err := q.Wait(ctx); err != nil {
				t.Errorf("n=%d: Wait() failed: %v", n, err)
			}
		}(i)
	}
	wg.Wait()
}

func TestQueue_simpleTask(t *testing.T) {
	const numEntries = 5000

	q := waitqueue.New()

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	var counter int // without lock
	counterTask := func(_ context.Context, _ int) error {
		counter++
		return nil
	}

	var wg sync.WaitGroup
	for i := 0; i < numEntries; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			delay := time.Second / 100 * time.Duration(rand.Intn(100))
			time.Sleep(delay)
			if err := q.WaitWithTask(ctx, counterTask); err != nil {
				t.Errorf("n=%d: WaitWithTask() failed: %v", n, err)
			}
		}(i)
	}
	wg.Wait()

	if counter != numEntries {
		t.Errorf("unexpected counter value: got %d, want %d.", counter, numEntries)
	}
}

func TestQueue_interval(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
		return
	}

	const numEntries = 500
	const entriesPerSec = numEntries / 20 // takes 20s

	conf := &waitqueue.Config{
		Interval: time.Second / entriesPerSec,
	}
	q, err := waitqueue.NewWithConfig(conf)
	if err != nil {
		t.Fatalf("waitqueue.NewWithConfig(): %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	var numDone, numCanceled uint32
	secMap := make(map[int64]uint32, 16)
	var secMu sync.Mutex
	secAdd := func() {
		tm := time.Now().Unix()
		secMu.Lock()
		if sc, ok := secMap[tm]; ok {
			secMap[tm] = sc + 1
		} else {
			secMap[tm] = 1
		}
		secMu.Unlock()
	}

	var wg sync.WaitGroup
	for i := 0; i < numEntries; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()

			delay := time.Second / 100 * time.Duration(rand.Intn(100*5))
			tout := time.Second / 100 * time.Duration(rand.Intn(100*8))
			time.Sleep(delay)
			waitCtx, waitCancel := context.WithTimeout(ctx, tout)
			defer waitCancel()
			if err := q.Wait(waitCtx); err != nil {
				if waitCtx.Err() != nil {
					_ = atomic.AddUint32(&numCanceled, 1)
				} else {
					t.Errorf("n=%d: Wait() failed: %v", n, err)
				}
				return
			}
			secAdd()
			_ = atomic.AddUint32(&numDone, 1)
		}(i)
	}
	wg.Wait()

	t.Logf("done:    %5d", numDone)
	t.Logf("canceled:%5d", numCanceled)
	numTotal := numDone + numCanceled
	t.Logf("total:   %5d", numTotal)
	if numTotal != numEntries {
		t.Errorf("unexpected counter: got %d, want %d", numTotal, numEntries)
	}

	if len(secMap) == 0 {
		t.Fatalf("no secmap entry.")
	}

	minT := int64(^uint64(0) >> 1)
	maxT := minT + 1
	for tm := range secMap {
		if tm < minT {
			minT = tm
		}
		if maxT < tm {
			maxT = tm
		}
	}
	for tm := minT; tm <= maxT; tm++ {
		tms := time.Unix(tm, 0).Format("15:04:05")
		cnt, ok := secMap[tm]
		if !ok {
			cnt = 0
		}
		t.Logf("%s:%5d", tms, cnt)
		switch {
		case cnt == 0:
			t.Errorf("%s: count=0", tms)
		case entriesPerSec < cnt:
			t.Errorf("%s: count %d > %d per sec", tms, cnt, entriesPerSec)
		}
	}
}

func TestQueue_expiredCtx(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
		return
	}

	const numEntries = 5000

	conf := &waitqueue.Config{
		Interval:        time.Second * 5,
		IntervalPerGate: time.Second * 5,
	}
	q, err := waitqueue.NewWithConfig(conf)
	if err != nil {
		t.Fatalf("waitqueue.NewWithConfig(): %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	expiredCtx, expireNow := context.WithCancel(ctx)
	expireNow()

	var numDone, numCanceled uint32

	var wg sync.WaitGroup
	for i := 0; i < numEntries; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()

			delay := time.Second / 100 * time.Duration(rand.Intn(100*2))
			time.Sleep(delay)
			if err := q.Wait(expiredCtx); err != nil {
				var cancelErr *waitqueue.CanceledError
				if errors.As(err, &cancelErr) {
					_ = atomic.AddUint32(&numCanceled, 1)
				} else {
					t.Errorf("n=%d: Wait() failed: %v", n, err)
				}
				return
			}
			_ = atomic.AddUint32(&numDone, 1)
		}(i)
	}
	wg.Wait()

	t.Logf("done:    %5d", numDone)
	t.Logf("canceled:%5d", numCanceled)
	numTotal := numDone + numCanceled
	t.Logf("total:   %5d", numTotal)
	if numTotal != numEntries {
		t.Errorf("unexpected counter: got %d, want %d", numTotal, numEntries)
	}
	if numDone != 0 {
		t.Errorf("unexpected done counter: got %d, want 0", numDone)
	}
}

func TestQueue_shortTimeout(t *testing.T) {
	const numEntries = 5000

	conf := &waitqueue.Config{
		Interval:        time.Second * 3 / numEntries,
		IntervalPerGate: 0,
	}
	q, err := waitqueue.NewWithConfig(conf)
	if err != nil {
		t.Fatalf("waitqueue.NewWithConfig(): %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	var numDone, numCanceled uint32

	var wg sync.WaitGroup
	for i := 0; i < numEntries; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()

			tout := time.Second / 1000 * time.Duration(rand.Intn(1500)) // 0..1.5s
			jobCtx, jobCancel := context.WithTimeout(ctx, tout)
			defer jobCancel()
			if err := q.Wait(jobCtx); err != nil {
				var cancelErr *waitqueue.CanceledError
				if errors.As(err, &cancelErr) {
					_ = atomic.AddUint32(&numCanceled, 1)
				} else {
					t.Errorf("n=%d: Wait() failed: %v", n, err)
				}
				return
			}
			_ = atomic.AddUint32(&numDone, 1)
		}(i)
	}
	wg.Wait()

	t.Logf("done:    %5d", numDone)
	t.Logf("canceled:%5d", numCanceled)
	numTotal := numDone + numCanceled
	t.Logf("total:   %5d", numTotal)
	if numTotal != numEntries {
		t.Errorf("unexpected counter: got %d, want %d", numTotal, numEntries)
	}
}

func TestQueue_multiGates(t *testing.T) {
	const (
		// short time and large entries make too short interval, and causes failure
		numEntries = 500 // must be multiple of numGates
		numGates   = 50
		testTime   = time.Second * 3
	)

	conf := &waitqueue.Config{
		NumGates:        numGates,
		Interval:        0,
		IntervalPerGate: testTime / time.Duration(numEntries/numGates-1),
	}
	q, err := waitqueue.NewWithConfig(conf)
	if err != nil {
		t.Fatalf("waitqueue.NewWithConfig(): %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	var numDone, numCanceled uint32
	numPerGate := make([]uint32, numGates)

	testStarted := time.Now()

	var wg sync.WaitGroup
	for i := 0; i < numEntries; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			task := func(_ context.Context, g int) error {
				atomic.AddUint32(&numPerGate[g], 1)
				return nil
			}
			if err := q.WaitWithTask(ctx, task); err != nil {
				var cancelErr *waitqueue.CanceledError
				if errors.As(err, &cancelErr) {
					_ = atomic.AddUint32(&numCanceled, 1)
				} else {
					t.Errorf("n=%d: WaitWithTask() failed: %v", n, err)
				}
				return
			}
			_ = atomic.AddUint32(&numDone, 1)
		}(i)
	}
	wg.Wait()

	elapsed := time.Since(testStarted)

	t.Logf("done:    %5d", numDone)
	t.Logf("canceled:%5d", numCanceled)
	numTotal := numDone + numCanceled
	t.Logf("total:   %5d", numTotal)
	t.Logf("elapsed: %v", elapsed)
	if elapsed.Round(time.Second/10) != testTime {
		t.Errorf("unexpected test time: got %v, want %v", elapsed, testTime)
	}
	t.Logf("gates:   %+v", numPerGate)
	for i, c := range numPerGate {
		if c != numEntries/numGates {
			t.Errorf("unexpected gate #%d count: got %d, want %v", i, c, numEntries/numGates)
		}
	}
	if numDone != numEntries {
		t.Errorf("unexpected done counter: got %d, want %d", numDone, numEntries)
	}
}
