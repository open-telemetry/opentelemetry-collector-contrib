// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package exceptionsconnector

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/pdatautil"
)

func TestNewConnector(t *testing.T) {
	defaultMethod := http.MethodGet
	defaultMethodValue := pcommon.NewValueStr(defaultMethod)
	for _, tc := range []struct {
		name           string
		dimensions     []Dimension
		wantDimensions []pdatautil.Dimension
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
			wantDimensions: []pdatautil.Dimension{
				{Name: "http.method", Value: &defaultMethodValue},
				{Name: "http.status_code", Value: nil},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Prepare
			factory := NewFactory()

			creationParams := connectortest.NewNopSettings()
			cfg := factory.CreateDefaultConfig().(*Config)
			cfg.Dimensions = tc.dimensions

			// Test Metrics
			traceMetricsConnector, err := factory.CreateTracesToMetrics(context.Background(), creationParams, cfg, consumertest.NewNop())
			smc := traceMetricsConnector.(*metricsConnector)

			assert.NoError(t, err)
			assert.NotNil(t, smc)
			assert.Equal(t, tc.wantDimensions, smc.dimensions)

			// Test Logs
			traceLogsConnector, err := factory.CreateTracesToLogs(context.Background(), creationParams, cfg, consumertest.NewNop())
			slc := traceLogsConnector.(*logsConnector)

			assert.NoError(t, err)
			assert.NotNil(t, slc)
			assert.Equal(t, tc.wantDimensions, smc.dimensions)
		})
	}
}
