// Copyright (c) 2021 Hirotsuna Mizuno. All rights reserved.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package waitqueue

import (
	"time"
)

// Config is the set of configuration parameters for Queue.
type Config struct {
	// Number of exit gates of the queue. This is meaningful only when
	// executing a TaskFunc while occupying an exit gate, or when
	// 0 < IntervalPerGate. Must be greater than 0, but treat 0 as 1 to
	// accept zero-value Config.
	NumGates int

	// Minimum interval that the next entry should wait in the queue after
	// the previous entry leaves the queue and passes through an exit gate.
	Interval time.Duration

	// Minimum interval that an exit gate should wait before accepting the
	// next entry after the previous entry passes through the gate.
	IntervalPerGate time.Duration
}
