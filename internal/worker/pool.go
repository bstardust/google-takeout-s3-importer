// internal/worker/pool.go
package worker

import (
	"sync"
)

// Pool represents a worker pool for concurrent operations
type Pool struct {
	wg      sync.WaitGroup
	workers chan struct{}
}

// NewPool creates a new worker pool with the specified number of workers
func NewPool(size int) *Pool {
	return &Pool{
		workers: make(chan struct{}, size),
	}
}

// Submit submits a task to the worker pool
func (p *Pool) Submit(task func()) {
	p.workers <- struct{}{} // Acquire a worker

	go func() {
		defer func() {
			<-p.workers // Release the worker
		}()

		task()
	}()
}

// Wait waits for all tasks to complete
func (p *Pool) Wait() {
	p.wg.Wait()
}
