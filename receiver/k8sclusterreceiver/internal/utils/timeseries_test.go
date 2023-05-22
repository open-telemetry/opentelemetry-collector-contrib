// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"testing"

	v1 "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/stretchr/testify/require"
)

func TestGetInt64TimeSeries(t *testing.T) {
	dpVal := int64(10)
	ts := GetInt64TimeSeries(dpVal)

	require.Equal(t, dpVal, ts.Points[0].GetInt64Value())
}

func TestGetInt64TimeSeriesWithLabels(t *testing.T) {
	dpVal := int64(10)
	labelVals := []*v1.LabelValue{{Value: "value1"}, {Value: "value2"}}

	ts := GetInt64TimeSeriesWithLabels(dpVal, labelVals)

	require.Equal(t, dpVal, ts.Points[0].GetInt64Value())
	require.Equal(t, labelVals, ts.LabelValues)
}
