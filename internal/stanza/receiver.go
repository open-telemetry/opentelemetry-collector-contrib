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

package stanza

import (
	"context"
	"fmt"
	"sync"

	"github.com/open-telemetry/opentelemetry-log-collection/agent"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"
)

type receiver struct {
	sync.Mutex
	startOnce sync.Once
	stopOnce  sync.Once
	wg        sync.WaitGroup
	cancel    context.CancelFunc

	agent     *agent.LogAgent
	emitter   *LogEmitter
	consumer  consumer.Logs
	converter *Converter
	logger    *zap.Logger
}

// Ensure this receiver adheres to required interface
var _ component.LogsReceiver = (*receiver)(nil)

// Start tells the receiver to start
func (r *receiver) Start(ctx context.Context, host component.Host) error {
	r.Lock()
	defer r.Unlock()
	err := componenterror.ErrAlreadyStarted
	r.startOnce.Do(func() {
		err = nil
		rctx, cancel := context.WithCancel(ctx)
		r.cancel = cancel
		r.logger.Info("Starting stanza receiver")

		if obsErr := r.agent.Start(); obsErr != nil {
			err = fmt.Errorf("start stanza: %s", err)
			return
		}

		r.converter.Start()

		r.wg.Add(1)
		go func(ctx context.Context) {
			defer r.wg.Done()

			// Don't create done channel on every iteration.
			doneChan := ctx.Done()
			for {
				select {
				case <-doneChan:
					r.logger.Debug("Receive loop stopped")
					return

				case e, ok := <-r.emitter.logChan:
					if !ok {
						continue
					}

					r.converter.Batch(e)
				}
			}
		}(rctx)

		r.wg.Add(1)
		go func(ctx context.Context) {
			defer r.wg.Done()

			// Don't create done channel on every iteration.
			doneChan := ctx.Done()
			pLogsChan := r.converter.OutChannel()

			for {
				select {
				case <-doneChan:
					r.logger.Debug("Flush loop stopped")
					return

				case pLogs, ok := <-pLogsChan:
					if !ok {
						r.logger.Debug("Converter channel got closed")
						continue
					}
					if cErr := r.consumer.ConsumeLogs(ctx, pLogs); cErr != nil {
						r.logger.Error("ConsumeLogs() failed", zap.Error(cErr))
					}
				}
			}
		}(rctx)
	})

	return err
}

// Shutdown is invoked during service shutdown
func (r *receiver) Shutdown(context.Context) error {
	r.Lock()
	defer r.Unlock()

	err := componenterror.ErrAlreadyStopped
	r.stopOnce.Do(func() {
		r.logger.Info("Stopping stanza receiver")
		err = r.agent.Stop()
		r.converter.Stop()
		r.cancel()
		r.wg.Wait()
	})
	return err
}
