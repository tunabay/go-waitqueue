// Copyright (c) 2021 Hirotsuna Mizuno. All rights reserved.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package waitqueue

import (
	"context"
)

// TaskFunc represents a task performed while occupying an exit gate. Called
// when it is its turn. The ID number [0, NumGates) of the occupied exit gate is
// passed.
type TaskFunc func(context.Context, int) error
