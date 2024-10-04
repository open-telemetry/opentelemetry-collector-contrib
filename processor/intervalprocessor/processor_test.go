// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package intervalprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/intervalprocessor"

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func TestAggregation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		passThrough bool
	}{
		{name: "basic_aggregation"},
		{name: "histograms_are_aggregated"},
		{name: "exp_histograms_are_aggregated"},
		{name: "gauges_are_aggregated"},
		{name: "summaries_are_aggregated"},
		{name: "all_delta_metrics_are_passed_through"},  // Deltas are passed through even when aggregation is enabled
		{name: "non_monotonic_sums_are_passed_through"}, // Non-monotonic sums are passed through even when aggregation is enabled
		{name: "gauges_are_passed_through", passThrough: true},
		{name: "summaries_are_passed_through", passThrough: true},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var config *Config
	for _, intervals := range []time.Duration{1, 2, 10, 60} {
		for _, tc := range testCases {
			config = &Config{Interval: time.Second * intervals, PassThrough: PassThrough{Summary: tc.passThrough, Gauge: tc.passThrough}}

			t.Run(fmt.Sprintf("%s/%d_partitions", tc.name, intervals), func(t *testing.T) {
				// next stores the results of the filter metric processor
				next := &consumertest.MetricsSink{}

				factory := NewFactory()
				mgp, err := factory.CreateMetricsProcessor(
					context.Background(),
					processortest.NewNopSettings(),
					config,
					next,
				)
				require.NoError(t, err)

				dir := filepath.Join("testdata", tc.name)

				md, err := golden.ReadMetrics(filepath.Join(dir, "input.yaml"))
				require.NoError(t, err)

				// Test that ConsumeMetrics works
				err = mgp.ConsumeMetrics(ctx, md)
				require.NoError(t, err)

				require.IsType(t, &Processor{}, mgp)
				processor := mgp.(*Processor)

				// Pretend we hit the interval timer and call export
				for i, p := range processor.partitions {
					processor.exportMetrics(i)
					// All the lookup tables should now be empty
					require.Empty(t, p.rmLookup)
					require.Empty(t, p.smLookup)
					require.Empty(t, p.mLookup)
					require.Empty(t, p.numberLookup)
					require.Empty(t, p.histogramLookup)
					require.Empty(t, p.expHistogramLookup)
					require.Empty(t, p.summaryLookup)

					// Exporting again should return nothing
					processor.exportMetrics(i)
				}

				// Next should contain:
				// 1. Anything left over from ConsumeMetrics()
				// Then for each partition:
				// 2. Anything exported from exportMetrics()
				// 3. An empty entry for the second call to exportMetrics()
				allMetrics := next.AllMetrics()
				require.Len(t, allMetrics, 1+(processor.numPartitions*2))

				nextData := allMetrics[0]
				expectedNextData, err := golden.ReadMetrics(filepath.Join(dir, "next.yaml"))
				require.NoError(t, err)
				require.NoError(t, pmetrictest.CompareMetrics(expectedNextData, nextData))

				expectedExportData, err := golden.ReadMetrics(filepath.Join(dir, "output.yaml"))
				require.NoError(t, err)
				partitionedExportedData := partitionedTestData(expectedExportData, processor.numPartitions)
				for i := 1; i < 1+(processor.numPartitions*2); i++ {
					if i%2 == 1 {
						require.NoError(t, pmetrictest.CompareMetrics(partitionedExportedData[i/2], allMetrics[i]), "the first export data should match the partitioned data")
						continue
					}

					require.NoError(t, pmetrictest.CompareMetrics(pmetric.NewMetrics(), allMetrics[i]), "the second export data should be empty")
				}
			})
		}
	}
}

// partitionedTestData returns the original test data, partitioned into the given number of partitions.
// The partitioning is done by hashing the stream ID and taking the modulo of the number of partitions.
func partitionedTestData(md pmetric.Metrics, partitions int) []pmetric.Metrics {
	out := make([]pmetric.Metrics, partitions)

	for i := 0; i < partitions; i++ {
		out[i] = pmetric.NewMetrics()
	}

	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		resID := identity.OfResource(rm.Resource())

		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			scopeID := identity.OfScope(resID, sm.Scope())

			for y := 0; y < sm.Metrics().Len(); y++ {
				m := sm.Metrics().At(y)
				metricID := identity.OfMetric(scopeID, m)

				// int -> uint64 theoretically can lead to overflow.
				// The only way we get an overflow here is if the interval is greater than 18446744073709551615 seconds.
				// That's 584942417355 years. I think we're safe.
				partition := metricID.Hash().Sum64() % uint64(partitions) //nolint
				rmClone := out[partition].ResourceMetrics().AppendEmpty()
				rm.CopyTo(rmClone)
			}
		}
	}

	return out
}
