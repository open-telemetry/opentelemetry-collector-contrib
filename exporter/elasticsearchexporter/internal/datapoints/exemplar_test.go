// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datapoints // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/datapoints"

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestExemplarValue(t *testing.T) {
	for _, tc := range []struct {
		name          string
		dp            pmetric.Exemplar
		expectedValue pcommon.Value
		expectedError string
	}{
		{
			name: "Exemplar returns value type of double",
			dp: func() pmetric.Exemplar {
				dp := pmetric.NewExemplar()
				dp.SetDoubleValue(10.0)
				return dp
			}(),
			expectedValue: pcommon.NewValueDouble(10.0),
		},
		{
			name: "Exemplar returns value type of int",
			dp: func() pmetric.Exemplar {
				dp := pmetric.NewExemplar()
				dp.SetIntValue(50)
				return dp
			}(),
			expectedValue: pcommon.NewValueInt(50),
		},
		{
			name: "return invalid exemplar when number is NaN",
			dp: func() pmetric.Exemplar {
				dp := pmetric.NewExemplar()
				dp.SetDoubleValue(math.NaN())
				return dp
			}(),
			expectedError: "invalid double value in Exemplar, either NaN or Inf",
		},
		{
			name: "return invalid exemplar when number is Inf",
			dp: func() pmetric.Exemplar {
				dp := pmetric.NewExemplar()
				dp.SetDoubleValue(math.Inf(0))
				return dp
			}(),
			expectedError: "invalid double value in Exemplar, either NaN or Inf",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := pmetric.NewMetric()
			m.SetName("test")

			actual, err := ExemplarValue(tc.dp)
			if tc.expectedError != "" {
				assert.ErrorContains(t, err, tc.expectedError)
				return
			}
			require.NoError(t, err)

			assert.Equal(t, tc.expectedValue.AsRaw(), actual.AsRaw())
		})
	}
}
