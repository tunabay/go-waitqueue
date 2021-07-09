// Copyright (c) 2021 Hirotsuna Mizuno. All rights reserved.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package waitqueue_test

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/tunabay/go-waitqueue"
)

func ExampleQueue_interval() {
	// Create a Queue with 1s interval.
	q, _ := waitqueue.NewWithConfig(&waitqueue.Config{Interval: time.Second})

	// Start go-routines waiting their turn in the queue.
	started := time.Now()
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()

			// Wait in the queue for its turn.
			if err := q.Wait(context.Background()); err != nil {
				panic(err)
			}
			elapsed := time.Since(started).Round(time.Second)
			fmt.Printf("%s: %d\n", elapsed, n)
		}(i)
		time.Sleep(time.Second / 20) // Avoid getting out of order.
	}
	wg.Wait()

	// Output:
	// 0s: 0
	// 1s: 1
	// 2s: 2
	// 3s: 3
	// 4s: 4
}

func ExampleQueue_withTask() {
	// Create a simple Queue without interval.
	q := waitqueue.New()

	// Start go-routines waiting their turn in the queue.
	started := time.Now()
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()

			// A task performed while occupying the exit gate of the queue.
			// It sleeps 1s during which the next go-routine has to wait.
			myTask := func(_ context.Context, _ int) error {
				elapsed := time.Since(started).Round(time.Second)
				fmt.Printf("%s: %d\n", elapsed, n)
				time.Sleep(time.Second)
				return nil
			}

			// Wait in the queue for its turn, and when it's its
			// turn, run myTask at the exit gate of the queue.
			if err := q.WaitWithTask(context.Background(), myTask); err != nil {
				panic(err)
			}
		}(i)
		time.Sleep(time.Second / 20) // Avoid getting out of order.
	}
	wg.Wait()

	// Output:
	// 0s: 0
	// 1s: 1
	// 2s: 2
	// 3s: 3
	// 4s: 4
}

func ExampleQueue_canceled() {
	// Create a simple Queue without interval.
	q := waitqueue.New()

	// A cancelable 3s TaskFunc
	task := func(ctx context.Context, _ int) error {
		timer := time.NewTimer(time.Second * 3)
		select {
		case <-ctx.Done():
			// canceled before completion
			if !timer.Stop() {
				<-timer.C
			}
			return waitqueue.NewCanceledError(ctx.Err())
		case <-timer.C:
			// task is completed
		}
		return nil
	}

	started := time.Now()

	// Queue and run a task with the specified timeout.
	queueTask := func(jobNo int, tout time.Duration) {
		ctx, cancel := context.WithTimeout(context.Background(), tout)
		defer cancel()
		if err := q.WaitWithTask(ctx, task); err != nil {
			elapsed := time.Since(started).Round(time.Second)
			fmt.Printf("%s: job %d: %s\n", elapsed, jobNo, err)
			return
		}
		elapsed := time.Since(started).Round(time.Second)
		fmt.Printf("%s: job %d: completed.\n", elapsed, jobNo)
	}

	// Start 3 jobs with different timeouts.
	jobTimeouts := []time.Duration{
		time.Second * 10, // job 0: not canceled.
		time.Second * 2,  // job 1: canceled in the queue, while job 0 is running.
		time.Second * 5,  // job 2: canceled in the task, after job 0 is completed.
	}
	var wg sync.WaitGroup
	for jobNo, tout := range jobTimeouts {
		wg.Add(1)
		go func(jobNo int, tout time.Duration) {
			defer wg.Done()
			queueTask(jobNo, tout)
		}(jobNo, tout)
		time.Sleep(time.Second / 20) // Avoid getting out of order.
	}
	wg.Wait()

	// Output:
	// 2s: job 1: canceled in queue: context deadline exceeded
	// 3s: job 0: completed.
	// 5s: job 2: canceled in task: context deadline exceeded
}

func ExampleQueue_multipleGates() {
	// Create a Queue with 5 exit gates.
	q, _ := waitqueue.NewWithConfig(&waitqueue.Config{NumGates: 5})

	started := time.Now()

	// A TaskFunc that takes 1s
	task := func(_ context.Context, gid int) error {
		time.Sleep(time.Second)
		elapsed := time.Since(started).Round(time.Second)
		fmt.Printf("%s: task completed at exit gate %d.\n", elapsed, gid)
		return nil
	}

	// Start 10 tasks, each takes 1s.
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := q.WaitWithTask(context.Background(), task); err != nil {
				panic(err)
			}
		}()
	}
	wg.Wait()

	// Unordered output:
	// 1s: task completed at exit gate 0.
	// 1s: task completed at exit gate 1.
	// 1s: task completed at exit gate 2.
	// 1s: task completed at exit gate 3.
	// 1s: task completed at exit gate 4.
	// 2s: task completed at exit gate 0.
	// 2s: task completed at exit gate 1.
	// 2s: task completed at exit gate 2.
	// 2s: task completed at exit gate 3.
	// 2s: task completed at exit gate 4.
}
