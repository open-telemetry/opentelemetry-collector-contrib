// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build goexperiment.synctest

package deltatocumulativeprocessor

import (
	"context"
	"fmt"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestStaleness(t *testing.T) {
	synctest.Run(func() {
		ctx := context.Background()
		iface, _ := setup(t, &Config{MaxStale: 5 * time.Minute, MaxStreams: 50}, &CountingSink{})
		proc := iface.(*Processor)
		err := proc.Start(ctx, nil)
		time.Sleep(1 * time.Second) // ticker startup
		require.NoError(t, err)
		defer proc.Shutdown(ctx)

		data := func(n int) (pmetric.Metrics, pmetric.MetricSlice) {
			md := pmetric.NewMetrics()
			rm := md.ResourceMetrics().AppendEmpty()
			rm.Resource().Attributes().PutInt("n", int64(n))
			return md, rm.ScopeMetrics().AppendEmpty().Metrics()
		}

		md1, ms1 := data(1)
		md2, ms2 := data(2)

		for i := range 10 {
			m := pmetric.NewMetric()
			m.SetName(fmt.Sprintf("metric%d", i))
			sum := m.SetEmptySum()
			sum.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
			dp := sum.DataPoints().AppendEmpty()
			dp.SetIntValue(1)
			dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))

			m.CopyTo(ms1.AppendEmpty())
			m.CopyTo(ms2.AppendEmpty())
		}

		// every func in at is executed at minute i
		at := []func(){
			0: func() {
				err = proc.ConsumeMetrics(ctx, md1)
				require.NoError(t, err)
				err = proc.ConsumeMetrics(ctx, md2)
				require.NoError(t, err)
				require.Equal(t, 20, proc.last.Size())
			},
			4: func() {
				for i := range ms2.Len() {
					sum := ms2.At(i).Sum()
					sum.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
					dps := sum.DataPoints()
					for i := range dps.Len() {
						dps.At(i).SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
					}
				}
				err = proc.ConsumeMetrics(ctx, md2)
				require.NoError(t, err)
				require.Equal(t, 20, proc.last.Size())
			},
			6: func() {
				require.Equal(t, 10, proc.last.Size())
			},
			11: func() {
				require.Equal(t, 0, proc.last.Size())
			},
		}

		for _, do := range at {
			if do != nil {
				do()
			}
			time.Sleep(1 * time.Minute)
		}
	})
}
