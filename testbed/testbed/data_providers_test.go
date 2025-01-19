// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testbed

import (
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

const metricsPictPairsFile = "../../internal/goldendataset/testdata/generated_pict_pairs_metrics.txt"

func TestGoldenDataProvider(t *testing.T) {
	dp := NewGoldenDataProvider("", "", metricsPictPairsFile)
	dp.SetLoadGeneratorCounters(&atomic.Uint64{})
	var ms []pmetric.Metrics
	for {
		m, done := dp.GenerateMetrics()
		if done {
			break
		}
		ms = append(ms, m)
	}
	require.Equal(t, len(dp.(*goldenDataProvider).metricsGenerated), len(ms))
}
