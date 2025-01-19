// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"
)

func Test_exemplarsFromConfig(t *testing.T) {
	traceID, err := hex.DecodeString("ae87dadd90e9935a4bc9660628efd569")
	require.NoError(t, err)

	spanID, err := hex.DecodeString("5828fa4960140870")
	require.NoError(t, err)

	tests := []struct {
		name         string
		c            *Config
		validateFunc func(t *testing.T, got []metricdata.Exemplar[int64])
	}{
		{
			name: "no exemplars",
			c:    &Config{},
			validateFunc: func(t *testing.T, got []metricdata.Exemplar[int64]) {
				assert.Nil(t, got)
			},
		},
		{
			name: "both-traceID-and-spanID",
			c: &Config{
				TraceID: "ae87dadd90e9935a4bc9660628efd569",
				SpanID:  "5828fa4960140870",
			},
			validateFunc: func(t *testing.T, got []metricdata.Exemplar[int64]) {
				require.Len(t, got, 1)
				metricdatatest.AssertEqual[metricdata.Exemplar[int64]](t, got[0], metricdata.Exemplar[int64]{
					TraceID: traceID,
					SpanID:  spanID,
				}, metricdatatest.IgnoreTimestamp(), metricdatatest.IgnoreValue())
			},
		},
		{
			name: "only-traceID",
			c: &Config{
				TraceID: "ae87dadd90e9935a4bc9660628efd569",
			},
			validateFunc: func(t *testing.T, got []metricdata.Exemplar[int64]) {
				require.Len(t, got, 1)
				metricdatatest.AssertEqual[metricdata.Exemplar[int64]](t, got[0], metricdata.Exemplar[int64]{
					TraceID: traceID,
					SpanID:  nil,
				}, metricdatatest.IgnoreTimestamp(), metricdatatest.IgnoreValue())
			},
		},
		{
			name: "only-spanID",
			c: &Config{
				SpanID: "5828fa4960140870",
			},
			validateFunc: func(t *testing.T, got []metricdata.Exemplar[int64]) {
				require.Len(t, got, 1)
				metricdatatest.AssertEqual[metricdata.Exemplar[int64]](t, got[0], metricdata.Exemplar[int64]{
					TraceID: nil,
					SpanID:  spanID,
				}, metricdatatest.IgnoreTimestamp(), metricdatatest.IgnoreValue())
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.validateFunc(t, exemplarsFromConfig(tt.c))
		})
	}
}
