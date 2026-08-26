// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package maps_test

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/maps"
)

func TestLimit(t *testing.T) {
	ctx := maps.Limit(100)
	m := maps.New[int, pmetric.NumberDataPoint](ctx)

	v := pmetric.NewNumberDataPoint()

	var (
		loads  = new(atomic.Int64)
		stores = new(atomic.Int64)
		fails  = new(atomic.Int64)
	)

	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			for i := range 110 {
				o, loaded := m.LoadOrStore(i, v)
				switch {
				case maps.Exceeded(o, loaded):
					fails.Add(1)
				case loaded:
					loads.Add(1)
				case !loaded:
					stores.Add(1)
				}
			}
		})
	}
	wg.Wait()

	require.Equal(t, int64(100), stores.Load())
	require.Equal(t, int64(900), loads.Load())
	require.Equal(t, int64(100), fails.Load())
}
