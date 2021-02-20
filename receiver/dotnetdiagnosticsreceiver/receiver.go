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
	"io"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dotnetdiagnosticsreceiver/dotnet"
)

type receiver struct {
	nextConsumer consumer.MetricsConsumer
	connect      connectionSupplier
	counters     []string
	intervalSec  int
	logger       *zap.Logger
}

type connectionSupplier func() (io.ReadWriter, error)

// NewReceiver creates a new receiver. connectionSupplier is swappable for
// testing.
func NewReceiver(
	_ context.Context,
	mc consumer.MetricsConsumer,
	connect connectionSupplier,
	counters []string,
	intervalSec int,
	logger *zap.Logger,
) (component.MetricsReceiver, error) {
	return &receiver{
		nextConsumer: mc,
		connect:      connect,
		counters:     counters,
		intervalSec:  intervalSec,
		logger:       logger,
	}, nil
}

func (r *receiver) Start(ctx context.Context, host component.Host) error {
	conn, err := r.connect()
	if err != nil {
		return err
	}

	w := dotnet.NewRequestWriter(conn, r.intervalSec, r.counters...)
	err = w.SendRequest()
	if err != nil {
		return err
	}

	return nil
}

func (r *receiver) Shutdown(context.Context) error {
	return nil
}
