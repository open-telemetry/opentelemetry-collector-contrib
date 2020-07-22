package publisher

import (
	"container/list"
	"log"
	"sync"
)

// It is a FIFO queue with the functionality that dropping the front if the queue size reaches to the maxSize
type NonBlockingFifoQueue struct {
	queue   *list.List
	maxSize int
	sync.Mutex
}

func NewNonBlockingFifoQueue(size int) *NonBlockingFifoQueue {
	if size <= 0 {
		panic("Queue Size should be larger than 0!")
	}
	return &NonBlockingFifoQueue{
		queue:   list.New(),
		maxSize: size,
	}
}

func (u *NonBlockingFifoQueue) Dequeue() (interface{}, bool) {
	u.Lock()
	defer u.Unlock()

	if u.queue.Len() == 0 {
		return nil, false
	}

	return u.queue.Remove(u.queue.Front()), true
}

func (u *NonBlockingFifoQueue) Enqueue(value interface{}) {
	u.Lock()
	defer u.Unlock()

	if u.queue.Len() == u.maxSize {
		log.Printf("W! message is dropped due to nonblocking fifo queue is full")
		u.queue.Remove(u.queue.Front())
	}
	u.queue.PushBack(value)
}
