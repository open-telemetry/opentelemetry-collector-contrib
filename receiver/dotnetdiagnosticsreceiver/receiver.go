// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dotnetdiagnosticsreceiver

import (
	"context"
	"net"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"
)

type receiver struct {
	logger             *zap.Logger
	nextConsumer       consumer.MetricsConsumer
	counters           []string
	collectionInterval time.Duration
	connect            connectionSupplier
}

var _ = (component.MetricsReceiver)(nil)

func NewReceiver(
	_ context.Context,
	logger *zap.Logger,
	cfg *Config,
	mc consumer.MetricsConsumer,
	connect connectionSupplier,
) (component.MetricsReceiver, error) {
	return &receiver{
		logger:             logger,
		nextConsumer:       mc,
		counters:           cfg.Counters,
		collectionInterval: cfg.CollectionInterval,
		connect:            connect,
	}, nil
}

type connectionSupplier func() (net.Conn, error)

func (r *receiver) Start(ctx context.Context, host component.Host) error {
	return nil
}

func (r *receiver) Shutdown(context.Context) error {
	return nil
}
