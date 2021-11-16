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

package prometheusoperatorreceiver

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
)

type prometheusOperatorReceiverWrapper struct {
	params             component.ReceiverCreateSettings
	config             *Config
	consumer           consumer.Metrics
	prometheusReceiver component.MetricsReceiver
}

// new returns a prometheusOperatorReceiverWrapper
func new(params component.ReceiverCreateSettings, cfg *Config, consumer consumer.Metrics) *prometheusOperatorReceiverWrapper {
	return &prometheusOperatorReceiverWrapper{params: params, config: cfg, consumer: consumer}
}

// Start creates and starts the prometheus receiver.
func (prw *prometheusOperatorReceiverWrapper) Start(ctx context.Context, host component.Host) error {
	return fmt.Errorf("receiver is not implemented")
}

// Shutdown stops the underlying Prometheus receiver.
func (prw *prometheusOperatorReceiverWrapper) Shutdown(ctx context.Context) error {
	return prw.prometheusReceiver.Shutdown(ctx)
}
