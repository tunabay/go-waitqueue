// Copyright (c) 2021 Hirotsuna Mizuno. All rights reserved.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package waitqueue

import (
	"errors"
	"fmt"
)

// TaskError is the error thrown by WaitWithTask when TaskFunc returns an error.
type TaskError struct{ err error }

func (e TaskError) Error() string {
	return fmt.Sprintf("task failed: %s", e.err)
}
func (e TaskError) String() string { return e.Error() }
func (e TaskError) Unwrap() error  { return e.err }

// CanceledError is the error thrown by Wait or WaitWithTask when passed context
// is canceled while waiting in the queue or executing the TaskFunc.
type CanceledError struct {
	err     error
	inQueue bool // or in task
}

func (e CanceledError) Error() string {
	if e.inQueue {
		return fmt.Sprintf("canceled in queue: %s", e.err)
	}
	return fmt.Sprintf("canceled in task: %s", e.err)
}
func (e CanceledError) String() string { return e.Error() }
func (e CanceledError) Unwrap() error  { return e.err }

// NewCanceledError creates a CanceledError with underlying err. A TaskFunc is
// recommended to return a CanceledError wrapped by this when the context is
// canceled before the task is completed.
//
// It allows to handle cases where the context is canceled before the task is
// completed, before or after the task starts.
//
// 	task := func(ctx context.Context, gate int) error {
// 		select {
// 		case <-ctx.Done():
// 			// canceled before completion
// 			return waitqueue.NewCanceledError(ctx.Err())
// 		case <-myTaskDoneChan:
// 			// completed
// 		case err := <-myTaskFailedChan:
// 			// normal failure
// 			return err
// 		}
// 		return nil
// 	}
// 	if err := queue.WaitWithTask(ctx, task); err != nil {
// 		var canceledErr *waitqueue.CanceledError
// 		if errors.As(err, &canceledErr) {
// 			// canceled in queue before task is started or,
// 			// canceled in taskFunc before task is completed
// 		} else {
// 			// task failed with other error
// 		}
// 	}
func NewCanceledError(err error) error {
	return &CanceledError{err: err}
}

// ErrInvalidConfig is an error thrown when a config parameter is invalid.
var ErrInvalidConfig = errors.New("invalid config")

// ErrCordonedOff is the default error returned by the newly called Wait or
// WaitWithTask while the queue is cordoned off.
var ErrCordonedOff = errors.New("cordoned off")
