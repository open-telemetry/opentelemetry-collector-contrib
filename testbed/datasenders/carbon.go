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

package datasenders // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/datasenders"

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/carbonexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

// CarbonDataSender implements MetricDataSender for Carbon metrics protocol.
type CarbonDataSender struct {
	testbed.DataSenderBase
	consumer.Metrics
}

// Ensure CarbonDataSender implements MetricDataSenderOld.
var _ testbed.MetricDataSender = (*CarbonDataSender)(nil)

// NewCarbonDataSender creates a new Carbon metric protocol sender that will send
// to the specified port after Start is called.
func NewCarbonDataSender(port int) *CarbonDataSender {
	return &CarbonDataSender{
		DataSenderBase: testbed.DataSenderBase{
			Port: port,
			Host: testbed.DefaultHost,
		},
	}
}

// Start the sender.
func (cs *CarbonDataSender) Start() error {
	factory := carbonexporter.NewFactory()
	cfg := &carbonexporter.Config{
		Endpoint: cs.GetEndpoint().String(),
		Timeout:  5 * time.Second,
	}
	params := exportertest.NewNopCreateSettings()
	params.Logger = zap.L()

	exporter, err := factory.CreateMetricsExporter(context.Background(), params, cfg)
	if err != nil {
		return err
	}

	cs.Metrics = exporter
	return nil
}

// GenConfigYAMLStr returns receiver config for the agent.
func (cs *CarbonDataSender) GenConfigYAMLStr() string {
	// Note that this generates a receiver config for agent.
	return fmt.Sprintf(`
  carbon:
    endpoint: %s`, cs.GetEndpoint())
}

// ProtocolName returns protocol name as it is specified in Collector config.
func (cs *CarbonDataSender) ProtocolName() string {
	return "carbon"
}
