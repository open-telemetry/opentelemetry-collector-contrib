// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package maps_test

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/puzpuzpuz/xsync/v3"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/maps"
)

func TestLimit(t *testing.T) {
	m := maps.Limit(xsync.NewMapOf[int, pmetric.NumberDataPoint](), 100, new(atomic.Int64))
	v := pmetric.NewNumberDataPoint()

	var (
		load  = new(atomic.Int64)
		store = new(atomic.Int64)
		fail  = new(atomic.Int64)
	)

	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			for i := range 110 {
				o, loaded := m.LoadOrStore(i, v)
				switch {
				case maps.Exceeded(o, loaded):
					fail.Add(1)
				case loaded:
					load.Add(1)
				case !loaded:
					store.Add(1)
				}
			}
			wg.Done()
		}()
	}
	wg.Wait()

	fmt.Println(load.Load(), store.Load(), fail.Load())

	require.Equal(t, int64(100), store.Load())
	require.GreaterOrEqual(t, int64(900), load.Load())
	require.LessOrEqual(t, int64(100), fail.Load())
}
