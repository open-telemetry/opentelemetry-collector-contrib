// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package maps_test

import (
	"errors"
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
				_, loaded, err := m.LoadOrStore(t.Context(), i, v)
				switch {
				case errors.Is(err, maps.ErrLimitExceeded):
					fails.Add(1)
				case err != nil:
					t.Error(err)
					return
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

func TestLimitDistinguishesZeroValue(t *testing.T) {
	m := maps.New[int, int](maps.Limit(1))
	value, loaded, err := m.LoadOrStore(t.Context(), 1, 0)
	require.NoError(t, err)
	require.Zero(t, value)
	require.False(t, loaded)
	value, loaded, err = m.LoadOrStore(t.Context(), 1, 42)
	require.NoError(t, err)
	require.Zero(t, value)
	require.True(t, loaded)
	_, loaded, err = m.LoadOrStore(t.Context(), 2, 0)
	require.ErrorIs(t, err, maps.ErrLimitExceeded)
	require.False(t, loaded)
	_, loaded, err = m.LoadAndDelete(t.Context(), 1)
	require.NoError(t, err)
	require.True(t, loaded)
	_, loaded, err = m.LoadOrStore(t.Context(), 2, 0)
	require.NoError(t, err)
	require.False(t, loaded)
}
