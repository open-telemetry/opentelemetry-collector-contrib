// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awscloudwatchmetricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchmetricsreceiver"

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"
)

type metricReceiver struct {
	region        string
	profile       string
	pollInterval  time.Duration
	nextStartTime time.Time
	logger        *zap.Logger
	consumer      consumer.Logs
	wg            *sync.WaitGroup
	doneChan      chan bool
}

func (m *metricReceiver) Start(ctx context.Context, host component.Host) error {
	m.logger.Debug("Starting to poll for CloudWatch metrics")
	m.wg.Add(1)
	return nil
}

func (m *metricReceiver) Shutdown(ctx context.Context) error {
	m.logger.Debug("Shutting down awscloudwatchmetrics receiver")
	close(m.doneChan)
	m.wg.Wait()
	return nil
}
