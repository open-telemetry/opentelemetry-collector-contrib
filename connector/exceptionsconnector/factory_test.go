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

package exceptionsconnector

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestNewConnector(t *testing.T) {
	defaultMethod := "GET"
	defaultMethodValue := pcommon.NewValueStr(defaultMethod)
	for _, tc := range []struct {
		name           string
		dimensions     []Dimension
		wantDimensions []dimension
	}{
		{
			name: "simplest config (use defaults)",
		},
		{
			name: "configured dimensions",
			dimensions: []Dimension{
				{Name: "http.method", Default: &defaultMethod},
				{Name: "http.status_code"},
			},
			wantDimensions: []dimension{
				{name: "http.method", value: &defaultMethodValue},
				{"http.status_code", nil},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Prepare
			factory := NewFactory()

			creationParams := connectortest.NewNopCreateSettings()
			cfg := factory.CreateDefaultConfig().(*Config)
			cfg.Dimensions = tc.dimensions

			// Test Metrics
			traceMetricsConnector, err := factory.CreateTracesToMetrics(context.Background(), creationParams, cfg, consumertest.NewNop())
			smc := traceMetricsConnector.(*metricsConnector)

			assert.Nil(t, err)
			assert.NotNil(t, smc)
			assert.Equal(t, tc.wantDimensions, smc.dimensions)

			// Test Logs
			traceLogsConnector, err := factory.CreateTracesToLogs(context.Background(), creationParams, cfg, consumertest.NewNop())
			slc := traceLogsConnector.(*logsConnector)

			assert.Nil(t, err)
			assert.NotNil(t, slc)
			assert.Equal(t, tc.wantDimensions, smc.dimensions)
		})
	}
}
