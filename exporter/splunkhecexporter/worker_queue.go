package splunkhecexporter

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

var errSendingQueueIsFull = errors.New("worker_queue is full")

type request struct {
	bytes              []byte
	localHeaders       map[string]string
	compressionEnabled bool
}

type WorkerQueue interface {
	Start() error
	Stop() error
	Produce(*bufferState, map[string]string) error
}

type workerQueue struct {
	config  *Config
	workers []hecWorker
	queue   chan *request
	wg      sync.WaitGroup
	logger  *zap.Logger
}

func newWorkerQueue(config *Config, logger *zap.Logger, clientFunc func() *http.Client) *workerQueue {

	workers := make([]hecWorker, 0, config.WorkerQueueConfig.Worker)
	for i := 0; i < config.WorkerQueueConfig.Worker; i++ {
		workers = append(workers, newHecWorkerWithoutAck(config, clientFunc))
	}
	queue := make(chan *request, config.WorkerQueueConfig.Size)

	return &workerQueue{
		config:  config,
		workers: workers,
		queue:   queue,
	}
}

func (l *workerQueue) Start() error {
	if l == nil {
		return fmt.Errorf("uninitialised worker queue")
	}
	l.wg.Add(l.config.WorkerQueueConfig.Worker)
	for _, worker := range l.workers {
		go l.consume(worker)
	}
	return nil
}

func (l *workerQueue) consume(worker hecWorker) {
	defer l.wg.Done()
CONSUMER_LOOP:
	for request := range l.queue {
		if !l.config.RetrySettings.Enabled {
			if err := worker.Send(context.Background(), request); err != nil {
				l.logger.Error("Failed to send data to splunk", zap.Error(err))
				continue
			}
		}

		var err error
		for i := 0; i < l.config.WorkerQueueConfig.Retries; i++ {
			err = worker.Send(context.Background(), request)
			if err == nil {
				continue CONSUMER_LOOP
			}
			l.logger.Error("Unable to send data to Splunk", zap.Error(err), zap.Int("Retry attempt", i+1))
			if i < l.config.WorkerQueueConfig.Retries {
				time.Sleep(l.config.WorkerQueueConfig.Interval)
			}
		}
		l.logger.Error("Finished retrying and dropping events", zap.Error(err))

	}
}

func requestFrom(bufState *bufferState, headers map[string]string) (*request, error) {
	if err := bufState.Close(); err != nil {
		return nil, err
	}

	return &request{
		bytes:              bufState.copyBuffer(),
		localHeaders:       headers,
		compressionEnabled: bufState.compressionEnabled,
	}, nil

}

func (l *workerQueue) Produce(bufState *bufferState, headers map[string]string) error {
	request, err := requestFrom(bufState, headers)
	if err != nil {
		return err
	}
	select {
	case l.queue <- request:
	default:
		err = errSendingQueueIsFull
	}
	return err
}

func (l *workerQueue) Stop() error {
	close(l.queue)
	l.wg.Wait()
	return nil
}

var _ *workerQueue = &workerQueue{}
