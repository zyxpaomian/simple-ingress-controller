package server

import (
	"context"
	"sync"
)

// An Event is used to communicate that something has happened.
type Event struct {
	once sync.Once
	C    chan struct{}
}

// NewEvent creates a new Event.
func NewEvent() *Event {
	return &Event{
		C: make(chan struct{}),
	}
}

// Set sets the event by closing the C channel. After the first time, calls to set are a no-op.
func (e *Event) Set() {
	e.once.Do(func() {
		close(e.C)
	})
}

// Wait waits for the event to get set.
func (e *Event) Wait(ctx context.Context) {
	// 只有在执行set的时候，才会出发channel的关闭，从而推出wait方法
	select {
	case <-ctx.Done():
	case <-e.C:
	}
}
