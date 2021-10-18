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

package redisreceiver

import (
	"context"

	"github.com/go-redis/redis/v7"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/interval"
)

type redisReceiver struct {
	settings       component.ReceiverCreateSettings
	config         *Config
	consumer       consumer.Metrics
	intervalRunner *interval.Runner
}

func newRedisReceiver(
	settings component.ReceiverCreateSettings,
	config *Config,
	consumer consumer.Metrics,
) *redisReceiver {
	return &redisReceiver{
		settings: settings,
		config:   config,
		consumer: consumer,
	}
}

// Set up and kick off the interval runner.
func (r *redisReceiver) Start(ctx context.Context, host component.Host) error {
	c := newRedisClient(&redis.Options{
		Addr:     r.config.Endpoint,
		Password: r.config.Password,
	})
	redisRunnable := newRedisRunnable(ctx, r.config.ID(), c, r.consumer, r.settings)
	r.intervalRunner = interval.NewRunner(r.config.CollectionInterval, redisRunnable)

	go func() {
		if err := r.intervalRunner.Start(); err != nil {
			host.ReportFatalError(err)
		}
	}()

	return nil
}

func (r *redisReceiver) Shutdown(ctx context.Context) error {
	r.intervalRunner.Stop()
	return nil
}
