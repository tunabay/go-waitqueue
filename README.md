# go-waitqueue: Simple FIFO queue for go-routines

[![Go Reference](https://pkg.go.dev/badge/github.com/tunabay/go-waitqueue.svg)](https://pkg.go.dev/github.com/tunabay/go-waitqueue)
[![MIT License](http://img.shields.io/badge/license-MIT-blue.svg?style=flat)](LICENSE)

Package waitqueue provides simple blocking functions for go-routine to wait in a FIFO queue.
Multiple go-routines can be queued and wait asynchronously, blocked until their turn.

## Features

* Cancellation while waiting for a turn or executing a task.
* Optional task function executed at the queue exit.
* Queue with multiple exit gates.
* Interval until the next go-routine's turn, for the queue itself and per exit gate.

## Usage

```
import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/tunabay/go-waitqueue"
)

func main() {
	q, _ := waitqueue.NewWithConfig(&waitqueue.Config{Interval: time.Second})

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			_ = q.Wait(context.Background())

			fmt.Println("it's my turn.")
		}()
	}
	wg.Wait()
}
```
[Run in Go Playground](https://play.golang.org/p/Ei9mnuvV-kE)

## Documentation

- https://pkg.go.dev/github.com/tunabay/go-waitqueue

## License

go-waitqueue is available under the MIT license. See the [LICENSE](LICENSE) file
for more information.
