package failoverconnector

import (
	"errors"
	"go.opentelemetry.io/collector/component"
	"sync"
	"time"
)

type consumerProvider[C any] func(...component.ID) (C, error)

type failoverRouter[C any] struct {
	consumerProvider consumerProvider[C]
	cfg              *Config
	pS               *pipelineSelector
	consumers        []C
	done             chan bool
	inRetry          bool
}

var (
	errNoValidPipeline = errors.New("All provided pipelines return errors")
	errConsumer        = errors.New("Error registering consumer")
)

func newFailoverRouter[C any](provider consumerProvider[C], cfg *Config) *failoverRouter[C] {
	return &failoverRouter[C]{
		consumerProvider: provider,
		cfg:              cfg,
		pS:               newPipelineSelector(cfg),
		done:             make(chan bool),
	}
}

func (f *failoverRouter[C]) getCurrentConsumer() C {
	return f.consumers[f.pS.currentIndex]
}

func (f *failoverRouter[C]) registerConsumers() error {
	consumers := make([]C, 0)
	for _, pipeline := range f.cfg.PipelinePriority {
		newConsumer, err := f.consumerProvider(pipeline...)
		if err == nil {
			consumers = append(consumers, newConsumer)
		} else {
			return errConsumer
		}
	}
	f.consumers = consumers
	return nil
}

// handlePipelineError evaluates if the error was due to a failed retry or a new failure and will resolve the error
// by adjusting the priority level accordingly
func (f *failoverRouter[C]) handlePipelineError() {
	if f.pS.currentIndexStable() {
		f.pS.nextPipeline()
		f.enableRetry()
	} else {
		f.pS.setStableIndex()
	}
}

// enableRetry is responsible for the lifecycle of the retry goroutine
func (f *failoverRouter[C]) enableRetry() {
	if f.inRetry {
		f.done <- true
	}

	ticker := time.NewTicker(f.cfg.RetryInterval)
	ch := make(chan bool)

	go func() {
		f.inRetry = true
		defer func() { f.inRetry = false }()
		go f.retryHighPriorityPipelines(f.pS.stableIndex, ch) // need to launch in goroutine
		for {
			select {
			case <-ticker.C:
				ch <- true
				go f.retryHighPriorityPipelines(f.pS.stableIndex, ch)
				if f.checkStopRetry(f.pS.stableIndex) {
					return
				}
			case <-f.done:
				ch <- true
				return
			}
		}
	}()
}

func (f *failoverRouter[C]) pipelineIsValid() bool {
	return f.pS.currentIndex < len(f.cfg.PipelinePriority)
}

// retryHighPriorityPipelines will iterate through all higher priority levels that have not exceeded their max
// retries and attempt to reestablish a stable data flow
func (f *failoverRouter[C]) retryHighPriorityPipelines(stableIndex int, ch chan bool) {
	ticker := time.NewTicker(f.cfg.RetryGap)

	defer ticker.Stop()

	for i := 0; i < stableIndex; i++ {
		if f.pS.maxRetriesUsed(i) {
			continue
		}
		select {
		case <-ch:
			return
		case <-ticker.C:
			f.pS.setToRetryIndex(i)
		}
	}
	<-ch
}

// checkStopRetry checks if retry should be suspended if all higher priority levels have exceeded their max retries
func (f *failoverRouter[C]) checkStopRetry(index int) bool {
	for i := 0; i < index; i++ {
		if f.pS.pipelineRetries[i] < f.cfg.MaxRetry {
			return false
		}
	}
	return true
}

// reportStable reports back to the failoverRouter that the current priority level that was called by Consume.SIGNAL was
// stable
func (f *failoverRouter[C]) reportStable() {
	if f.pS.currentIndexStable() {
		return
	}
	f.pS.setStable()
	f.done <- true
}

func (f *failoverRouter[C]) handleShutdown() {
	if f.inRetry {
		f.done <- true
	}
}

// pipelineSelector is meant to serve as the source of truth for the target priority level
type pipelineSelector struct {
	currentIndex    int
	stableIndex     int
	lock            sync.Mutex
	pipelineRetries []int
	maxRetry        int
}

func (p *pipelineSelector) nextPipeline() {
	p.lock.Lock()
	for ok := true; ok; ok = p.pipelineRetries[p.currentIndex] >= p.maxRetry {
		p.currentIndex++
	}
	p.stableIndex = p.currentIndex
	p.lock.Unlock()
}

func (p *pipelineSelector) setStableIndex() {
	p.lock.Lock()
	p.pipelineRetries[p.currentIndex]++
	p.currentIndex = p.stableIndex
	p.lock.Unlock()
}

func (p *pipelineSelector) setToRetryIndex(index int) {
	p.lock.Lock()
	p.currentIndex = index
	p.lock.Unlock()
}

// Potentially change mechanism to directly change elements in pipelines slice instead of tracking pipelines to skip
func (p *pipelineSelector) maxRetriesUsed(index int) bool {
	return p.pipelineRetries[index] >= p.maxRetry
}

func (p *pipelineSelector) setStable() {
	p.pipelineRetries[p.currentIndex] = 0
	p.stableIndex = p.currentIndex
}

func (p *pipelineSelector) currentIndexStable() bool {
	return p.currentIndex == p.stableIndex
}

func newPipelineSelector(cfg *Config) *pipelineSelector {
	return &pipelineSelector{
		currentIndex:    0,
		stableIndex:     0,
		lock:            sync.Mutex{},
		pipelineRetries: make([]int, len(cfg.PipelinePriority)),
		maxRetry:        cfg.MaxRetry,
	}
}
