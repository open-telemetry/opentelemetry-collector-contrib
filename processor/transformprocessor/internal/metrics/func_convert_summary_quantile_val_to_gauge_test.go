// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
)

func Test_ConvertSummaryQuantileValToGauge(t *testing.T) {
	tests := []summaryTestCase{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualMetrics := pmetric.NewMetricSlice()
			tt.input.CopyTo(actualMetrics.AppendEmpty())

			evaluate, err := convertSummaryQuantileValToGauge(ottl.Optional[string]{})
			assert.NoError(t, err)

			_, err = evaluate(nil, ottldatapoint.NewTransformContext(pmetric.NewNumberDataPoint(), tt.input, actualMetrics, pcommon.NewInstrumentationScope(), pcommon.NewResource(), pmetric.NewScopeMetrics(), pmetric.NewResourceMetrics()))
			assert.NoError(t, err)

			expected := pmetric.NewMetricSlice()
			tt.want(expected)
			assert.Equal(t, expected, actualMetrics)
		})
	}
}
