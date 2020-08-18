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

package jmxmetricsreceiver

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"
)

type jmxMetricsReceiver struct {
	logger   *zap.Logger
	config   *config
	consumer consumer.MetricsConsumer
}

func newJmxMetricsReceiver(
	logger *zap.Logger,
	config *config,
	consumer consumer.MetricsConsumer,
) *jmxMetricsReceiver {
	return &jmxMetricsReceiver{
		logger:   logger,
		config:   config,
		consumer: consumer,
	}
}

func (jmx *jmxMetricsReceiver) Start(ctx context.Context, host component.Host) error {
	return nil
}

func (jmx *jmxMetricsReceiver) Shutdown(ctx context.Context) error {
	return nil
}
