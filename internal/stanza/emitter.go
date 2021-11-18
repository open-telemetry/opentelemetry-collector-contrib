// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package stanza // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/stanza"

import (
	"context"
	"sync"
	"time"

	"github.com/open-telemetry/opentelemetry-log-collection/entry"
	"github.com/open-telemetry/opentelemetry-log-collection/operator"
	"github.com/open-telemetry/opentelemetry-log-collection/operator/helper"
	"go.uber.org/zap"
)

// LogEmitter is a stanza operator that emits log entries to a channel
type LogEmitter struct {
	helper.OutputOperator
	logChan       chan []*entry.Entry
	stopOnce      sync.Once
	cancel        context.CancelFunc
	batchMux      sync.Mutex
	batch         []*entry.Entry
	wg            sync.WaitGroup
	maxBatchSize  uint
	flushInterval time.Duration
}

type LogEmitterOption interface {
	Apply(*LogEmitter)
}

type logEmitterOptionFunc func(*LogEmitter)

func (leo logEmitterOptionFunc) Apply(le *LogEmitter) {
	leo(le)
}

var (
	defaultFlushInterval      = 100 * time.Millisecond
	defaultMaxBatchSize  uint = 100
)

// LogEmitterWithMaxBatchSize returns an option that makes the LogEmitter use the specified max batch size
func LogEmitterWithMaxBatchSize(maxBatchSize uint) LogEmitterOption {
	return logEmitterOptionFunc(func(le *LogEmitter) {
		le.maxBatchSize = maxBatchSize
		le.batch = make([]*entry.Entry, 0, maxBatchSize)
	})
}

// LogEmitterWithFlushInterval returns an option that makes the LogEmitter use the specified flush interval
func LogEmitterWithFlushInterval(flushInterval time.Duration) LogEmitterOption {
	return logEmitterOptionFunc(func(le *LogEmitter) {
		le.flushInterval = flushInterval
	})
}

// LogEmitterWithLogger returns an option that makes the LogEmitter use the specified logger
func LogEmitterWithLogger(logger *zap.SugaredLogger) LogEmitterOption {
	return logEmitterOptionFunc(func(le *LogEmitter) {
		le.OutputOperator.BasicOperator.SugaredLogger = logger
	})
}

// NewLogEmitter creates a new receiver output
func NewLogEmitter(opts ...LogEmitterOption) *LogEmitter {
	le := &LogEmitter{
		OutputOperator: helper.OutputOperator{
			BasicOperator: helper.BasicOperator{
				OperatorID:    "log_emitter",
				OperatorType:  "log_emitter",
				SugaredLogger: zap.NewNop().Sugar(),
			},
		},
		logChan:       make(chan []*entry.Entry),
		maxBatchSize:  defaultMaxBatchSize,
		batch:         make([]*entry.Entry, 0, defaultMaxBatchSize),
		flushInterval: defaultFlushInterval,
	}

	for _, opt := range opts {
		opt.Apply(le)
	}

	return le
}

func (e *LogEmitter) Start(_ operator.Persister) error {
	ctx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel

	e.wg.Add(1)
	go e.flusher(ctx)
	return nil
}

// Process will emit an entry to the output channel
func (e *LogEmitter) Process(ctx context.Context, ent *entry.Entry) error {
	e.batchMux.Lock()

	e.batch = append(e.batch, ent)
	if uint(len(e.batch)) >= e.maxBatchSize {
		// flushTriggerAmount triggers a blocking flush
		e.batchMux.Unlock()
		e.flush(ctx)
		return nil
	}

	e.batchMux.Unlock()
	return nil
}

func (e *LogEmitter) flusher(ctx context.Context) {
	defer e.wg.Done()

	ticker := time.NewTicker(e.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			e.flush(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (e *LogEmitter) flush(ctx context.Context) {
	var batch []*entry.Entry
	e.batchMux.Lock()

	if len(e.batch) == 0 {
		e.batchMux.Unlock()
		return
	}
	batch = e.batch
	e.batch = make([]*entry.Entry, 0, e.maxBatchSize)

	e.batchMux.Unlock()

	select {
	case e.logChan <- batch:
	case <-ctx.Done():
	}
}

// Stop will close the log channel
func (e *LogEmitter) Stop() error {
	e.stopOnce.Do(func() {
		close(e.logChan)
	})

	if e.cancel != nil {
		e.cancel()
		e.cancel = nil
	}

	e.wg.Wait()
	return nil
}
