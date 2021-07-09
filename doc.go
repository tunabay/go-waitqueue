// Copyright (c) 2021 Hirotsuna Mizuno. All rights reserved.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

/*
Package waitqueue provides simple blocking functions for go-routine to wait in a
FIFO queue. Multiple go-routines can be queued and wait asynchronously, blocked
until their turn.
*/
package waitqueue
