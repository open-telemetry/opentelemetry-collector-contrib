// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package slowsqlconnector

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/slowsqlconnector/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/pdatautil"
)

func TestNewConnector(t *testing.T) {
	defaultMethod := "GET"
	defaultMethodValue := pcommon.NewValueStr(defaultMethod)
	for _, tc := range []struct {
		name           string
		dimensions     []Dimension
		dBSystem       []string
		wantDimensions []pdatautil.Dimension
		wantDBSystem   []string
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
			dBSystem: []string{"h2", "mysql"},
			wantDimensions: []pdatautil.Dimension{
				{Name: "http.method", Value: &defaultMethodValue},
				{Name: "http.status_code", Value: nil},
			},
			wantDBSystem: []string{"h2", "mysql"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Prepare
			factory := NewFactory()

			creationParams := connectortest.NewNopSettings(metadata.Type)
			cfg := factory.CreateDefaultConfig().(*Config)
			cfg.Dimensions = tc.dimensions

			// Test Logs
			traceLogsConnector, err := factory.CreateTracesToLogs(t.Context(), creationParams, cfg, consumertest.NewNop())
			slc := traceLogsConnector.(*logsConnector)

			assert.NoError(t, err)
			assert.NotNil(t, slc)
			assert.Equal(t, tc.wantDimensions, slc.dimensions)
		})
	}
}
