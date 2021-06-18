package server

import (
	"context"
	"sync"
)

// 定义一个事件的结构体，本质上是一个channel，用于锁与释放
type Event struct {
	once sync.Once
	C    chan struct{}
}

func NewEvent() *Event {
	return &Event{
		C: make(chan struct{}),
	}
}

// 关闭阻塞的channel，用于给第一次payload 加载使用
func (e *Event) Set() {
	e.once.Do(func() {
		close(e.C)
	})
}

func (e *Event) Wait(ctx context.Context) {
	// 只有在执行set的时候，才会出发channel的关闭，从而退出wait方法
	select {
	case <-ctx.Done():
	case <-e.C:
	}
}
