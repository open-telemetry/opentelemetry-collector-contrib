package publisher

import (
"context"
"errors"
"golang.org/x/sync/semaphore"
"log"
"sync"
"time"
)

// Publisher is go-routing safe
type Publisher struct {
	publishQueue Queue
	publisherSem *semaphore.Weighted
	concurrency  int64
	publishFn    func(req interface{})
	drainTimeout time.Duration
	wg           sync.WaitGroup
	// After close is set to true, subsequent calling Publish will be a no-op
	sync.RWMutex
	closed bool
}

// Queue is go-routing safe
type Queue interface {
	Enqueue(req interface{})
	Dequeue() (interface{}, bool)
}

// Create a publisher with parameters:
// queue: specify the underlining queue
// concurrency: specify the worker thread to consume the queue
// drainTimeout: time to wait for draining the on-hold requests when calling Close()
// fn: specify the publishing method to call
func NewPublisher(queue Queue, concurrency int64, drainTimeout time.Duration, fn func(req interface{})) (*Publisher, error) {
	if concurrency < 1 {
		// concurrency cannot be less than 1
		return nil, errors.New("concurrency cannot be less than 1 when creating Publisher!")
	}
	publisher := &Publisher{publishQueue: queue, publisherSem: semaphore.NewWeighted(concurrency), drainTimeout: drainTimeout, concurrency: concurrency,
		publishFn: fn}
	publisher.start()
	return publisher, nil
}

func (p *Publisher) Publish(req interface{}) {
	p.publishQueue.Enqueue(req)
}

func (p *Publisher) isClosed() bool {
	p.RLock()
	defer p.RUnlock()
	return p.closed
}

func (p *Publisher) start() {
	p.wg.Add(1)
	go p.startRouting()
}

func (p *Publisher) Close() {
	p.Lock()
	p.closed = true
	p.Unlock()

	// wait for draining the queue
	if waitWithTimeout(&p.wg, p.drainTimeout) {
		log.Printf("D! Publisher Close, draining publisher queue timeout")
	}
	// wait 1 second for publishing the on fly request
	ctx, _ := context.WithTimeout(context.TODO(), time.Second)
	if err := p.publisherSem.Acquire(ctx, p.concurrency); err != nil {
		log.Printf("D! Publisher Close, publisher work is not fully complete: %v", err)
	}
}

func (p *Publisher) startRouting() {
	for {
		// This for-loop is to do dequeue and publishing. Never do p.publishQueue.Enqueue() in this loop
		// which will cause deadlock if the underlining queue is a blockingQueue
		req, ok := p.publishQueue.Dequeue()
		if ok {
			if err := p.publisherSem.Acquire(context.TODO(), 1); err != nil {
				// The semaphore Acquire should never return error in this case.
				// The logic here is for debug/protection purpose.
				log.Printf("E! Publisher Acquire semaphore fail: %v", err)
				continue
			}
			go func() {
				p.publishFn(req)
				p.publisherSem.Release(1)
			}()
		} else {
			// if the publihser is closed along with no req returned from the queue, it means the queue has drained.
			if p.isClosed() {
				p.wg.Done()
				return
			}
			// wait for 50ms then continue
			time.Sleep(50 * time.Millisecond)
		}
	}
}

// return true if timeout happen before wg is done
func waitWithTimeout(wg *sync.WaitGroup, timeout time.Duration) bool {
	c := make(chan struct{})
	go func() {
		defer close(c)
		wg.Wait()
	}()
	select {
	case <-c:
		return false
	case <-time.After(timeout):
		return true
	}
}
