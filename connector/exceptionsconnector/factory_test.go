// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
