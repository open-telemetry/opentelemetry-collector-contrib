// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package spanmetricsconnector

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/connector/connectortest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/pdatautil"
)

func TestNewConnector(t *testing.T) {
	defaultMethod := http.MethodGet
	defaultMethodValue := pcommon.NewValueStr(defaultMethod)
	for _, tc := range []struct {
		name                         string
		durationHistogramBuckets     []time.Duration
		dimensions                   []Dimension
		wantDurationHistogramBuckets []float64
		wantDimensions               []pdatautil.Dimension
	}{
		{
			name: "simplest config (use defaults)",
		},
		{
			name:                     "1 configured duration histogram bucket should result in 1 explicit duration bucket (+1 implicit +Inf bucket)",
			durationHistogramBuckets: []time.Duration{2 * time.Millisecond},
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

			creationParams := connectortest.NewNopSettings(metadata.Type)
			cfg := factory.CreateDefaultConfig().(*Config)
			cfg.Histogram.Explicit = &ExplicitHistogramConfig{
				Buckets: tc.durationHistogramBuckets,
			}
			cfg.Dimensions = tc.dimensions

			// Test
			traceConnector, err := factory.CreateTracesToMetrics(context.Background(), creationParams, cfg, consumertest.NewNop())
			smc := traceConnector.(*connectorImp)

			// Verify
			assert.NoError(t, err)
			assert.NotNil(t, smc)

			assert.Equal(t, tc.wantDimensions, smc.dimensions)
		})
	}
}
