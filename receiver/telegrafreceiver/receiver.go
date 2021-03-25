// Copyright 2021, OpenTelemetry Authors
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

package telegrafreceiver

import (
	"context"
	"sync"

	"github.com/influxdata/telegraf"
	telegrafagent "github.com/influxdata/telegraf/agent"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

type telegrafreceiver struct {
	sync.Mutex
	startOnce sync.Once
	stopOnce  sync.Once
	wg        sync.WaitGroup
	cancel    context.CancelFunc

	agent           *telegrafagent.Agent
	consumer        consumer.Metrics
	logger          *zap.Logger
	metricConverter MetricConverter
}

// Ensure this receiver adheres to required interface.
var _ component.MetricsReceiver = (*telegrafreceiver)(nil)

// Start tells the receiver to start.
func (r *telegrafreceiver) Start(ctx context.Context, host component.Host) error {
	r.logger.Info("Starting telegraf receiver")

	r.Lock()
	defer r.Unlock()

	err := componenterror.ErrAlreadyStarted
	r.startOnce.Do(func() {
		err = nil
		rctx, cancel := context.WithCancel(ctx)
		r.cancel = cancel

		ch := make(chan telegraf.Metric)
		go func() {
			if rErr := r.agent.RunWithChannel(rctx, ch); rErr != nil {
				r.logger.Error("Problem starting receiver: %v", zap.Error(rErr))
			}
		}()

		r.wg.Add(1)
		go func() {
			var fErr error
			defer r.wg.Done()
			for {
				select {
				case <-rctx.Done():
					return

				case m, ok := <-ch:
					if !ok {
						r.logger.Info("channel closed")
						return
					}
					if m == nil {
						r.logger.Info("got nil from channel")
						break
					}

					var ms pdata.Metrics
					if ms, fErr = r.metricConverter.Convert(m); fErr != nil {
						r.logger.Error(
							"Error converting telegraf.Metric to pdata.Metrics",
							zap.Error(fErr),
						)
						continue
					}

					if fErr = r.consumer.ConsumeMetrics(ctx, ms); fErr != nil {
						r.logger.Error("ConsumeMetrics() error",
							zap.String("error", fErr.Error()),
						)
					}
				}
			}
		}()
	})

	return err
}

// Shutdown is invoked during service shutdown.
func (r *telegrafreceiver) Shutdown(context.Context) error {
	r.Lock()
	defer r.Unlock()

	err := componenterror.ErrAlreadyStopped
	r.stopOnce.Do(func() {
		r.logger.Info("Stopping telegraf receiver")
		r.cancel()
		r.wg.Wait()
		err = nil
	})
	return err
}
