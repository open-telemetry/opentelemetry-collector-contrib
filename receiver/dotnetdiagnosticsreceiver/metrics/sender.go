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

package metrics

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dotnetdiagnosticsreceiver/dotnet"
)

// Sender wraps a consumer.Metrics, and has a Send method which
// conforms to dotnet.MetricsConsumer so it can be passed into a Parser.
type Sender struct {
	next         consumer.Metrics
	logger       *zap.Logger
	prevSendTime time.Time
}

func NewSender(next consumer.Metrics, logger *zap.Logger) *Sender {
	return &Sender{next: next, logger: logger}
}

// Send accepts a slice of dotnet.Metrics, converts them to pdata.Metrics, and
// sends them to the next pdata consumer. Conforms to dotnet.MetricsConsumer.
func (s *Sender) Send(rawMetrics []dotnet.Metric) {
	now := time.Now()
	pdm := rawMetricsToPdata(rawMetrics, s.prevSendTime, now)
	s.prevSendTime = now
	err := s.next.ConsumeMetrics(context.Background(), pdm)
	if err != nil {
		s.logger.Error(err.Error())
	}
}
