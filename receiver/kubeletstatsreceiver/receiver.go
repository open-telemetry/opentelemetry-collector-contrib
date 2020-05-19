// Copyright 2020, OpenTelemetry Authors
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

package kubeletstatsreceiver

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver/interval"
)

var _ component.MetricsReceiver = (*receiver)(nil)

type receiver struct {
	logger   *zap.Logger
	cfg      configmodels.Receiver
	consumer consumer.MetricsConsumerOld
	runner   *interval.Runner
}

// Creates and starts the kubelet stats runnable.
func (r *receiver) Start(ctx context.Context, host component.Host) error {
	cfg := r.cfg.(*Config)
	runnable := newRunnable(ctx, r.consumer, cfg, r.logger)
	runner := interval.NewRunner(cfg.CollectionInterval, runnable)
	go func() {
		if err := runner.Start(); err != nil {
			host.ReportFatalError(err)
		}
	}()
	return nil
}

// Stops the kubelet stats runner.
func (r *receiver) Shutdown(ctx context.Context) error {
	r.runner.Stop()
	return nil
}
